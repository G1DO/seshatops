# Broker interruption and outbox backlog recovery

Local, disposable release evidence only. Not a production incident procedure, SLO, or recovery-time objective. Transport remains at-least-once; duplicate delivery is expected behavior.

## Trigger and observable symptoms

* Redpanda `redpanda` service stopped, process killed, or host `docker compose` network partition. In the disposable stack this is only the declared `redpanda:9092` service (`compose.yaml` pinned `redpanda:v25.2.1`).
* `GET /readyz` returns `503 {"status":"not_ready"}` while `GET /livez` remains `200 {"status":"alive"}`.
* `seshatops_runtime_ready 0` (process-local gauge; resets on restart) via authenticated `GET /metrics` (`MX-010` `ROLE-RELEASE-OBSERVER` on `SCOPE-RUNTIME`). Core `/readyz` and this gauge intentionally exclude forecast/Python.
* Durable backlog grows: `seshatops_outbox_backlog_records_pending` increases, `seshatops_outbox_oldest_unpublished_age_seconds` increases. `seshatops_outbox_backlog_records_publishing` may show leased rows.
* Relay degradation: `seshatops_relay_publish_outcomes_total{outcome="transient"}` increments, structured logs `relay.cycle.failed` with `worker="relay"` and bounded `duration_ms` (logged after the idle/transient broker probe so the event reflects the outage and flips `seshatops_runtime_ready`). The relay worker probes the broker only when idle (`claimed == 0`) or on transient failures, caching healthy successes for one `CycleTimeout` (default `10s`) to bound load while re-probing immediately on failure — so a broker outage flips readiness within one interval even with no backlog. Consumer idle: `consumer.cycle.completed` stops emitting `processed`, `seshatops_consumer_observed_lag_known 0` (consumer also probes on `DeadlineExceeded` idle polls).
* Tenant read path stays authorized but shows stale projection: `GET /v1/tenants/11111111-1111-4111-8111-111111111111/ops` (`MX-002`/`MX-003`) shows `backlog.pending > 0`, `GET /v1/tenants/11111111-1111-4111-8111-111111111111/inventory` checksum unchanged.

Distinguish: `transient` relay outcome is a retryable transport failure (broker unavailable). It is not `quarantined` (poison), not `409 not_releasable` (immutable inbox conflict), not `401/403` (authorization denial), and not forecast-only degradation.

## Scope and safety assumptions

* Single-host disposable Compose project `seshatops-local` (`compose.yaml` + `SESHATOPS_LOCAL_STACK=true`). No remote host, no shared volume, no external broker.
* PostgreSQL remains transactional authority; outbox rows are durable in `erp.outbox`. Do not delete `postgres-data` or edit projections directly. Manual `psql` surgery is prohibited.
* Tenant isolation preserved. Path tenant is an assertion; recovery requires fresh Go-owned OIDC session (`northstar-demo-operator` with `ROLE-OPS-READER` + `ROLE-PLATFORM-OPERATOR` + `ROLE-RELEASE-OBSERVER` on `SCOPE-RUNTIME`). No bypass of allow-list or invariant checks.
* Sessions are process-local memory; restarting `runtime` invalidates cookies and requires re-login via Authorization Code + PKCE. This is expected.
* Redpanda history is retained across restart. The harness `broker-recovery` scenario uses `demo` guard; this runbook uses only `scripts/local-stack.sh` and `docker compose` on the declared local endpoint.

## Diagnosis commands and expected signals

All commands below are implemented in `#99`-`#104`. Run from repository root. No aspirational tooling.

```bash
./scripts/local-stack.sh status
./scripts/local-stack.sh logs runtime --tail 200
./scripts/local-stack.sh logs redpanda --tail 100
```

Expected healthy baseline (before fault, after `quickstart`):

* `status` shows `seshatops-local-runtime` healthy, `postgres` healthy, `redpanda` healthy, `web` healthy.
* `logs runtime` contains `http.request.completed` with `route="health"` and `status_code 200` for `/readyz`, periodic `relay.cycle.completed` `outcome="success"` and `consumer.cycle.completed`.

After broker interruption (`docker compose` stop or `kill`):

```bash
docker compose --project-name seshatops-local --file compose.yaml ps
curl -i http://127.0.0.1:8080/readyz  # via runtime directly, or via Vite proxy
curl -s http://127.0.0.1:8080/livez
```

* `ps` shows `redpanda` `exited` or `unhealthy`.
* `GET /readyz` `503 not_ready`; `GET /livez` `200 alive`.

Authenticated metrics (browser console after `http://web.seshatops.localhost:5173` login as `northstar-demo-operator`):

```js
fetch("/metrics", { credentials: "same-origin" }).then(r=>r.text()).then(t=>{
  console.log(t.match(/seshatops_runtime_ready .*/g));
  console.log(t.match(/seshatops_outbox_backlog_records_pending .*/g));
  console.log(t.match(/seshatops_outbox_oldest_unpublished_age_seconds .*/g));
  console.log(t.match(/seshatops_relay_publish_outcomes_total.*/g));
})
```

* `seshatops_runtime_ready 0`
* `seshatops_outbox_backlog_records_pending` > `0` and increasing with each new ERP acceptance
* `seshatops_outbox_oldest_unpublished_age_seconds` > `0` and increasing

Ops visibility (same session):

```bash
curl -b /tmp/cookies.txt -c /tmp/cookies.txt \
  http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops \
  -H "Accept: application/json" | jq .backlog
```

* `backlog.pending` > `0`, `oldest_unpublished` non-null.

Log correlation: every runtime log carries `correlation_id` (UUIDv4, never caller-supplied). Example redacted line:

```json
{"time":"2026-08-25T12:00:00.000Z","level":"INFO","msg":"relay.cycle.failed","correlation_id":"a1b2c3d4-...","worker":"relay","duration_ms":45}
```

No log contains cookies, tokens, OIDC assertions, credentials, raw event bodies, or private identifiers.

## Smallest safe action sequence

1. **Confirm durability** — verify backlog is captured in metrics/outbox, not lost. If `seshatops_outbox_backlog_records_pending == 0` after new ERP work, stop: source transaction failed, not a broker backlog.
2. **Restart broker** — only the declared local service:

   ```bash
   docker compose --project-name seshatops-local --file compose.yaml start redpanda
   # or if stopped via scripts harness:
   docker compose --project-name seshatops-local --file compose.yaml up --detach --wait redpanda
   ./scripts/local-stack.sh status
   ```

   Wait up to 30s for `redpanda` health `healthy` (see `compose.yaml` healthcheck `rpk cluster health`).

3. **Restart packaged runtime** — the bounded local recovery restarts the Go process after Redpanda (sessions are lost). This is the only prescribed recovery for this environment:

   ```bash
   docker compose --project-name seshatops-local --file compose.yaml restart runtime
   docker compose --project-name seshatops-local --file compose.yaml up --wait --wait-timeout 180
   ./scripts/local-stack.sh status
   ```

4. **Re-establish OIDC session** — open `http://web.seshatops.localhost:5173`, log in as `northstar-demo-operator` (Authorization Code + PKCE, `S256`). Old cookies are invalid.
5. **Verify drain** — poll until healthy:

   ```bash
   curl -s http://127.0.0.1:8080/readyz | grep ready
   # then authenticated:
   fetch("/metrics", {credentials:"same-origin"}).then(r=>r.text()).then(t=>console.log(
     t.match(/seshatops_runtime_ready 1/),
     t.match(/seshatops_outbox_backlog_records_pending 0/)
   ))
   curl -b /tmp/cookies.txt \
     http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops | jq .backlog.pending
   ```

   Success is `readyz 200 ready`, `seshatops_runtime_ready 1`, `pending == 0`, `oldest_unpublished_age_seconds == 0`, and `relay.cycle.completed` resuming with `published` counts.

Do not: delete `postgres-data`/`redpanda-data` volumes, run a broad volume prune, edit `platform.inventory_projection`, inject `ROLLBACK`, or use `--break-expectation` (harness self-test only).

## Stop / rollback / escalation conditions

* Stop if `GET /readyz` returns `401/403` — you are unauthenticated, not observing broker state. Re-login.
* Stop if `seshatops_outbox_backlog_records_quarantined > 0` appears instead of `pending` — this is poison, not a retryable transport failure. Follow `poison-isolation.md`.
* Stop if `POST /v1/tenants/.../ops/quarantine/release` returns `409 not_releasable` — follow that runbook; do not force.
* Stop if `platform` migration or `postgres` health fails (`./scripts/local-stack.sh logs postgres`). This is not a broker fault.
* Rollback: the environment is disposable. If readiness does not return within 180s after both restarts, capture bounded evidence (below) and reset only the declared project:

  ```bash
  SESHATOPS_LOCAL_RESET_CONFIRM='I_UNDERSTAND_DISPOSABLE_LOCAL_RESET' ./scripts/local-stack.sh reset
  ./scripts/local-stack.sh quickstart
  ```

  Never run `down --volumes` with an undeclared project name or external volume.

* Escalate (local only): retain evidence and re-run `./scripts/local-stack.sh demo broker-recovery --evidence-dir .release-evidence/broker-$(date -u +%Y%m%dT%H%M%SZ)` to get deterministic harness comparison.

## Post-recovery checks and evidence to retain

* `GET /readyz` `200 {"status":"ready"}` and `seshatops_runtime_ready 1`.
* `seshatops_outbox_backlog_records_pending == 0` and `seshatops_outbox_oldest_unpublished_age_seconds == 0`.
* `GET /v1/tenants/11111111-1111-4111-8111-111111111111/inventory` checksum equals pre-interruption value (or equals bootstrap summary checksum) and `GET /v1/tenants/11111111-1111-4111-8111-111111111111/ops` `projection.checksum` matches.
* `seshatops_relay_publish_outcomes_total{outcome="published"}` advanced after restart; `transient` counter stopped advancing.
* `GET /metrics` still shows bounded label sets only (no event ID/correlation/tenant labels).
* Retain (redacted, bounded ≤64 KiB): `docker compose ps` snapshot, `logs runtime --tail 200`, `logs redpanda --tail 100`, `GET /metrics` tail, and `GET /ops` `backlog` snapshot. Search tails for `seshatops-local-only` or fixture markers before retaining; do not commit full dumps or secrets.

## Limitations and unsupported production conclusions

* Evidence is from one single-host Compose topology with pinned images (`postgres:16.14`, `redpanda:v25.2.1`). Not production latency, availability, or recovery-time objective.
* Outbox backlog gauge is a scrape-time snapshot; not a durable SLO metric or continuous lag history.
* At-least-once transport persists; duplicate redelivery after recovery is expected and must show `duplicate_noop` (see `duplicate-delivery` demo). Do not claim exactly-once.
* No backup/restore, multi-region failover, or capacity claim is established by this procedure.
