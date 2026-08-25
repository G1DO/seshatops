# Poison and incompatible event isolation

Local, disposable release evidence only. Not a production triage runbook or SLO.

## Trigger and observable symptoms

* One event with an incompatible contract reaches the consumer: `event_schema_version` `2` (accepted families accept exactly `1`), stale version gap, invalid payload, or aggregate mismatch. In the disposable environment this is produced only by the packaged `demo-fixture` poison path (`northstar-m5-poison-v1`, event `318f5d78-...4011`) or the `poison-isolation` demo scenario. No public API accepts raw event writes.
* `seshatops_processing_failures_quarantined` increments (or `seshatops_processing_failures_retrying` briefly then quaratined). `seshatops_processing_oldest_failure_age_seconds` becomes non-zero.
* `seshatops_processing_quarantined_gap` may increment for residual inbox gap rows.
* Relay counters remain unaffected (poison is a consumer quarantine, not an outbox `quarantined` unless the same incompatible bytes were durably recorded in `erp.outbox` — distinguish via diagnosis).
* Structured logs: `consumer.cycle.completed` with bounded fields; `authorization.denied` does not appear (this is not an auth failure). Logs never contain raw event bodies.
* Ops visibility:

  ```bash
  curl -b /tmp/cookies.txt \
    http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops | jq .processing
  ```

  shows `failures_quarantined >= 1`, `failures[]` with `failure_category`, `diagnostic_code`, `quarantine_status`, `event_id`, `event_type`, `aggregate_id`, all tenant-scoped. `gaps[]` may contain residual `quarantined_gap`.
* Core runtime stays ready: `GET /readyz` `200 ready` and `seshatops_runtime_ready 1` (poison is isolated, not a full outage). Unrelated valid source work (`northstar-m3-lineage-v1` supplier→lot→batch→shipment→order) continues to project and remains visible via `GET /v1/tenants/11111111-1111-4111-8111-111111111111/inventory` and SSE `inventory_projection.updated`.

Distinguish: quarantined poison = `seshatops_processing_failures_quarantined` / `quarantined_gap` and `platform.inbox` history. Retryable transport = `seshatops_relay_publish_outcomes_total{outcome="transient"}` + `seshatops_runtime_ready 0` (see `broker-interruption.md`). Immutable conflict = `409 not_releasable` on release attempt. Authorization denial = `401 {"error":"unauthenticated"}` or `403 {"error":"forbidden"}` + `seshatops_auth_denials_total`. Forecast-only degradation never moves `seshatops_runtime_ready` (see `forecast-degradation.md`).

## Scope and safety assumptions

* Tenant-scoped quarantine only. Path tenant (`11111111-1111-4111-8111-111111111111` for `TENANT-NS-001`) is an assertion; Go evaluates allow-list row `MX-004` (`ROLE-PLATFORM-OPERATOR` on `RES-QUARANTINE` `ACT-QUARANTINE-RELEASE`) and `MX-005` (`RES-REPLAY`). Wrong-tenant `event_id` is `404` without mutation (`ErrTenantMismatch`).
* Audit-before-mutate enforced. Every `POST /v1/tenants/{tenant_id}/ops/quarantine/release` or `POST .../replay` first inserts one `identity.authorization_decisions` row; `audit_failed` (`500 {"error":"audit_failed"}`) blocks mutation.
* No direct projection edit, no `UPDATE platform.inventory_projection`, no `DELETE FROM erp.outbox`, no manual Redpanda offset commit. All recovery is via public `POST` controls or `POST .../rebuild`.
* No authorization bypass. Missing/expired/forged session is `401`; reader-only `ROLE-OPS-READER` on `TENANT-NS-001` is `403` for release/replay/rebuild. Use the deterministic local assignment:

  ```text
  northstar-demo-operator|11111111-1111-4111-8111-111111111111|ROLE-OPS-READER
  northstar-demo-operator|11111111-1111-4111-8111-111111111111|ROLE-PLATFORM-OPERATOR
  ```

  Without `ROLE-PLATFORM-OPERATOR`, controls are deny.

## Diagnosis commands and expected signals

All commands below are implemented (`#99`-`#104`). No hidden helpers.

```bash
./scripts/local-stack.sh status
./scripts/local-stack.sh logs runtime --tail 200
```

Authenticated diagnosis (after login at `http://web.seshatops.localhost:5173` as `northstar-demo-operator`):

```js
// metrics (same-origin, MX-010)
fetch("/metrics", {credentials:"same-origin"}).then(r=>r.text()).then(t=>{
  console.log(t.match(/seshatops_processing_failures_quarantined .*/g));
  console.log(t.match(/seshatops_processing_failures_retrying .*/g));
  console.log(t.match(/seshatops_processing_quarantined_gap .*/g));
  console.log(t.match(/seshatops_runtime_ready .*/g));
})
```

Ops and inventory:

```bash
curl -b /tmp/cookies.txt \
  http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops | jq
curl -b /tmp/cookies.txt \
  http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/inventory | jq .checksum
# SSE (proves unrelated work still flows):
curl -b /tmp/cookies.txt -H "Accept: text/event-stream" \
  http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/inventory/stream
```

Expected:

* `/ops` `processing.failures_quarantined >=1` and `failures[0].quarantine_status == "quarantined"` with `failure_category` in `conflict|gap|stale|invalid|mismatch|transition`. `gaps[]` entries have `reason == "quarantined_gap"` if present.
* `/ops` `backlog.quarantined` reflects outbox `quarantined` rows if the poison was durably recorded; for pure consumer poison (incompatible broker bytes not from outbox) this gauge stays `0` — this distinction drives the release decision.
* `/metrics` `seshatops_processing_failures_quarantined` matches `/ops` count; `seshatops_runtime_ready 1`.
* `logs runtime` contains `consumer.cycle.completed` and later `ops.control.completed` after any authorized control; never raw `event_bytes`.

Redacted log example:

```json
{"time":"2026-08-25T12:01:00.000Z","level":"INFO","msg":"ops.control.completed","correlation_id":"b2c3d4e5-...","operation":"quarantine_release","outcome":"released","duration_ms":12}
```

## Smallest safe action sequence

1. **Inspect** — record `failures[]` `event_id`/`tenant_id`/`event_type`/`failure_category`/`diagnostic_code`/`quarantine_status` and `gaps[]`. Verify tenant id matches path tenant `11111111-1111-4111-8111-111111111111`. If tenant mismatch, stop (wrong-tenant event, `404` expected).
2. **Check if releasable**:

   * If `backlog.quarantined > 0` and `failures[]` entry has `quarantine_status == "quarantined"` and `failure_category` is outbox-owned (e.g., `quarantined` outbox row), the row is a durably retained outbox quarantine that can be retried via the same-tenant outbox path.
   * If `quarantine_status` is `quarantined_gap`, `quarantined_conflict`, `quarantined_stale`, `quarantined_invalid`, `quarantined_mismatch`, or `quarantined_transition` and resides in `platform.inbox`/inventory lineage as a gap/conflict, it is **not releasable**. The control returns `409 not_releasable` and must not be force-applied.

3. **Attempt release only when safely releasable** — authorized `POST` with explicit matrix row:

   ```bash
   curl -b /tmp/cookies.txt -c /tmp/cookies.txt \
     -X POST http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops/quarantine/release \
     -H "Content-Type: application/json" \
     -d '{"event_id":"318f5d78-6e64-4f5f-bd16-8e9f7c4a4011"}' | jq
   ```

   * `200 {"status":"released"}` → outbox row returned to `pending` and will be re-driven through normal relay→consumer path; metric `seshatops_control_operations_total{operation="quarantine_release",outcome="released"}` increments; `ops.control.completed` logged with `outcome=released`.
   * `404 {"error":"not_found"}` → no same-tenant quarantined target for that `event_id` (or wrong tenant). Do not retry with a different tenant.
   * `409 {"error":"not_releasable"}` → terminal inbox quarantine; follow stop condition below.

4. **Replay only for retained history** — if poison was not durably retained (pure broker bytes) and the check is duplicate-safety:

   ```bash
   curl -b /tmp/cookies.txt \
     -X POST http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops/replay \
     -H "Content-Type: application/json" -d '{}' | jq
   # or for a specific event:
   curl -b /tmp/cookies.txt \
     -X POST http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops/replay \
     -H "Content-Type: application/json" -d '{"event_id":"318f5d78-..."}' | jq
   ```

   Expected `200` with `status == "complete"` or `duplicate_noop` for already-applied events. A `quarantined` count in the replay result indicates the poison was re-quarantined as expected; do not loop.

5. **Verify unrelated work** — after any control, confirm core projection still advances:

   ```bash
   curl -b /tmp/cookies.txt \
     http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/inventory | jq .checksum
   curl -b /tmp/cookies.txt \
     http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops | jq .processing
   ```

   `inventory` checksum for unrelated items must be unchanged unless the poison was a valid-but-later-versioned event; `processing.applied` or `duplicate_noop` should have advanced while `failures_quarantined` stays bounded.

## Stop / rollback / escalation conditions

* **Stop and do not prescribe release/replay when**: `409 not_releasable`, `quarantined_gap`/`quarantined_conflict`/`stale`/`invalid`/`mismatch`/`transition` inbox row, or when `diagnostic_code` indicates an unsupported contract that this consumer does not apply. The stated incompatibility is immutable for this release.
* Stop if `401 unauthenticated` or `403 forbidden` — caller lacks `MX-004`/`MX-005` for this tenant. Re-login with the correct deterministic assignment; do not add a bypass row.
* Stop if `500 audit_failed` — audit insert failed, mutation blocked. Retain evidence, do not retry with `RESET`; check `postgres` health (`./scripts/local-stack.sh logs postgres`).
* Stop if cross-tenant `event_id` (`22222222-...` negative tenant) — the control is `404` by design; no foreign effect was created.
* No rollback deletes quarantined rows. If a mistaken `released` was issued for a truly poison contract, the next consumer cycle will re-quarantine it (`quarantined` outcome). Do not edit `erp.outbox` status manually.
* If unsure, escalate by capturing `GET /ops` + `GET /metrics` (bounded tail) + `GET /v1/tenants/.../ops/audit` and stopping the runbook. Never suggest disabling invariant checks.

## Post-recovery checks and evidence to retain

* After `200 released`: `backlog.quarantined` decremented, `seshatops_control_duration_seconds_*` shows one `quarantine_release` `released`, audit `GET /ops/audit` contains one `allow` record with `principal_id == northstar-demo-operator`, `tenant_id == 11111111-1111-4111-8111-111111111111`, `resource == RES-QUARANTINE`, `action == ACT-QUARANTINE-RELEASE`, `outcome == allow`.
* After `409 not_releasable`: metric `seshatops_control_operations_total{operation="quarantine_release",outcome="not_releasable"}` increments; no audit `allow` mutates projection.
* Unrelated-work check passes: `GET /inventory` checksum and `GET /ops/lineage/batches/{batch_id}` for a valid `northstar-m3-lineage-v1` batch still return `200` with provenance; SSE still yields `inventory_projection.updated` for new valid events.
* Retain (redacted, ≤64 KiB): `GET /ops` `processing` slice, `GET /metrics` counters for `processing_*` and `control_operations`, `logs runtime --tail 200` filtered to `consumer.cycle.*`/`ops.control.completed`, and the `POST` response body. Ensure no raw event bytes appear.

## Limitations and unsupported production conclusions

* Scope is one tenant, one incompatible event family, in-process quarantine. Not production dead-letter queue, not multi-tenant fan-out, not automatic replay policy.
* Duration measurements are diagnostic; no SLO or SLO-breach conclusion is supported.
* The fixed poison fixture (`event_schema_version 2`) is local-only fault tooling packaged inside the harness (`scripts/release_demo.py` `poison-isolation` scenario, `cmd/seshatops demo-fixture poison` guard). It is not a public API and does not prove resilience to arbitrary production schema drift outside the accepted families (`docs/design/specifications/event-spine.md` §2).
* No claim that `released` guarantees eventual apply; a valid event may still become `duplicate_noop` or `conflict` per Event Spine contracts.
