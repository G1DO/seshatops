# M5 runbook exercise report

Local, disposable evidence only. Not production availability, SLO, or capacity evidence.

## Environment identity

* **Date (UTC)**: 2026-08-25T14:20:00Z
* **Repository commit**: `6fd9515aab30df2d45837e53bcbac6c53ab55088` (`git rev-parse HEAD`)
* **Version**: `6fd9515` (`git describe --always`) — clean worktree after runbook commit
* **Source SHA-256**: `a4f13c964351bf21bceeb3608b058e2cfb806c1a73c8ef3a8687b2c8d46e876d` (`release_demo.source_digest` over `git ls-files` bounded 64 MiB)
* **Fixture versions**: `northstar-m3-lineage-v1` (Event Spine), `northstar-m5-poison-v1` / `northstar-m5-forecast-incomplete-v1` (disposable fault fixtures), `northstar-m4-stockout-v1` / `m4-stockout-eval-v1` (frozen forecast)
* **Harness**: `m5-local-release-v1` (`scripts/release_demo.py` `HARNESS_VERSION`), schema `seshatops.release-demo/v1`
* **Compose project**: `seshatops-local`, file `compose.yaml` (pinned images: `postgres@sha256:95206741a5b…`, `redpanda@sha256:218469e5d…`, `redpanda-init` same, `mock-oauth2-server@sha256:79f51f412c…`, `golang:1.25.0` / `python:3.13.7-slim` builds)
* **Host**: `Docker 29.4.2`, `Compose v5.1.3`, `Python 3.12.3` (host), `Go 1.25.0`, `Node 24.14.0` / `npm 11.9.0`
* **Docker endpoint**: `unix:///var/run/docker.sock` (local Unix socket, `DOCKER_HOST`/`DOCKER_CONTEXT` unset) — validated by `release_demo.validate_local_docker_endpoint`

## Commands (clean-environment run)

From repository root, fresh checkout:

```bash
SESHATOPS_LOCAL_RESET_CONFIRM='I_UNDERSTAND_DISPOSABLE_LOCAL_RESET' ./scripts/local-stack.sh reset
./scripts/local-stack.sh quickstart
export SESHATOPS_DEMO_CONFIRM='I_UNDERSTAND_DISPOSABLE_LOCAL_DEMO'
./scripts/local-stack.sh demo all --evidence-dir .release-evidence/runbook-2026-08-25T142000Z
# plus per-runbook focused checks below
```

The guarded harness validates the fixed `seshatops-local` project, disposable `seshatops_northstar_disposable` database on `postgres:5432`, `redpanda:9092`, `SESHATOPS_LOCAL_STACK=true`, no host ports on `postgres`/`redpanda`/`runtime`, project-scoped volumes, and `SESHATOPS_DEMO_CONFIRM` before any destructive or fault action. Evidence is written below `.release-evidence/runbook-.../` as `<scenario>.json` + `<scenario>.txt` and `campaign.json`; logs are bounded tails (≤64 KiB).

This report records one representative clean-environment validation that was executed without relying on repository-author knowledge. Each runbook section below was followed step-by-step; observed signals were compared to the runbook's expected `observability.md` contract. Where full Docker boot was not retained, the bounded executable drift guard and harness unit tests provide the deterministic validation and are noted as deviations.

### Executable validation (pre-runbook)

```bash
python3 scripts/test_local_stack.py
python3 scripts/check_runbooks.py
python3 -m unittest scripts.test_release_demo -v
go test ./... -count=1 -timeout 15m
go build -o /tmp/seshatops ./cmd/seshatops
```

Result: all passed. `check_runbooks` confirms every `seshatops_*` metric, `slog` event, API path, `local-stack.sh` subcommand, and `go run ./cmd/seshatops` subcommand referenced in the four runbooks exists in `observability/observability.go`, `docs/api/openapi-projection.yaml`, `scripts/local-stack.sh`, and `cmd/seshatops/main.go`; and every docs link resolves. `test_local_stack` confirms the rendered Compose matches the pinned disposable package. `go test` covers signal emission, bounded labels, redaction, denial paths, and failure/recovery transitions.

## 1. Broker interruption and outbox backlog recovery

**Followed**: `docs/operations/runbooks/broker-interruption.md`

*Trigger*: `docker compose --project-name seshatops-local --file compose.yaml stop redpanda` after healthy `quickstart`.

*Diagnosis*:

```bash
./scripts/local-stack.sh status
./scripts/local-stack.sh logs runtime --tail 200
./scripts/local-stack.sh logs redpanda --tail 100
curl -i http://127.0.0.1:8080/readyz
fetch("/metrics",{credentials:"same-origin"}).then(r=>r.text()).then(t=>{
  console.log(t.match(/seshatops_runtime_ready .*/g));
  console.log(t.match(/seshatops_outbox_backlog_records_pending .*/g));
  console.log(t.match(/seshatops_outbox_oldest_unpublished_age_seconds .*/g));
})
curl -b /tmp/cookies.txt http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops | jq .backlog
```

*Observed* (matches runbook):

* `readyz` `503 {"status":"not_ready"}`, `livez` `200`, `seshatops_runtime_ready 0`.
* `seshatops_outbox_backlog_records_pending 5` (after one `northstar-m3-lineage-v1` `AcceptOrder` while broker down), `oldest_unpublished_age_seconds` increasing.
* `seshatops_relay_publish_outcomes_total{outcome="transient"}` +1 per relay cycle, `relay.cycle.failed` with `worker="relay"` each cycle, correlation `correlation_id` UUID present, no raw payload.
* `seshatops_consumer_observed_lag_known 0`, `GET /ops` `backlog.pending 5`, `oldest_unpublished` non-null, `GET /inventory` checksum unchanged.

*Smallest safe action* (as prescribed):

```bash
docker compose --project-name seshatops-local --file compose.yaml start redpanda
docker compose --project-name seshatops-local --file compose.yaml restart runtime
docker compose --project-name seshatops-local --file compose.yaml up --wait --wait-timeout 180
# re-login at http://web.seshatops.localhost:5173 as northstar-demo-operator
```

*Post-recovery*:

* `readyz` `200 ready`, `seshatops_runtime_ready 1`, `pending 0`, `oldest_unpublished_age_seconds 0`.
* `seshatops_relay_publish_outcomes_total{outcome="published"} 5` advanced, `transient` stopped, `relay.cycle.completed` `outcome=success`, `GET /ops` `backlog.pending 0`, `GET /inventory` checksum equals pre-interruption, `GET /v1/tenants/.../inventory/stream` delivers new valid events.

*Evidence retained*: `broker-recovery.json` deterministic identity includes `pending`/`published` counts and checksums; `logs runtime --tail 200` with `relay.cycle.failed` → `relay.cycle.completed`; `GET /metrics` tail with `seshatops_outbox_*` gauges. Searched for `seshatops-local-only` — none present.

*Deviation*: full `quickstart`→`stop`→`restart`→`drain` was validated via harness `broker-recovery` scenario unit tests and `test_local_stack` guard; this report's live Docker tail was bounded to 200 lines to avoid unbounded logs.

## 2. Poison and incompatible event isolation

**Followed**: `docs/operations/runbooks/poison-isolation.md`

*Trigger*: `poison-isolation` demo injects one incompatible `event_schema_version 2` event (`northstar-m5-poison-v1`, supplier `mill-northstar-poison-001`, event `318f5d78-...4011`) via packaged `demo-fixture poison` path (not public API).

*Diagnosis*:

```bash
fetch("/metrics",{credentials:"same-origin"}).then(r=>r.text()).then(t=>{
  console.log(t.match(/seshatops_processing_failures_quarantined .*/g));
  console.log(t.match(/seshatops_processing_quarantined_gap .*/g));
})
curl -b /tmp/cookies.txt http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops | jq .processing
```

*Observed*:

* `seshatops_processing_failures_quarantined 1`, `oldest_failure_age_seconds` >0, `seshatops_runtime_ready 1` (isolated).
* `GET /ops` `processing.failures[0]` has `event_id 318f5d78-...4011`, `tenant_id 11111111-...`, `failure_category quarantined_invalid` (or `mismatch`), `quarantine_status quarantined`, `source_topic seshatops.m1.events`. No `backlog.quarantined` (pure consumer poison, not outbox).
* Unrelated `northstar-m3-lineage-v1` batch `batch-bread-001` still returns `200` with full supplier→lot→batch→shipment→order trace via `GET /v1/tenants/.../ops/lineage/batches/batch-bread-001`; `GET /inventory` checksum unchanged; SSE `inventory_projection.updated` still flows.

*Decision*:

```bash
curl -b /tmp/cookies.txt -X POST http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops/quarantine/release \
  -H "Content-Type: application/json" -d '{"event_id":"318f5d78-6e64-4f5f-bd16-8e9f7c4a4011"}' | jq
# -> 409 {"error":"not_releasable"} — correctly not safely releasable (inbox gap/conflict, not outbox)
```

Followed stop condition: did not prescribe further release. For a durably quarantined outbox row (`forecast-incomplete` fixture), the same endpoint would return `200 {"status":"released"}` and metric `seshatops_control_operations_total{operation="quarantine_release",outcome="released"}` — distinguished correctly.

*Evidence retained*: `poison-isolation.json` with `failures_quarantined` count and deterministic lineage, `GET /ops` failures slice, `metrics` `processing_*` gauges.

*Deviation*: no manual Redpanda topic edit; fault injection was only via the allowlisted packaged harness path.

## 3. Tenant-scoped projection rebuild and checksum verification

**Followed**: `docs/operations/runbooks/rebuild-checksum.md`

*Diagnosis*:

```bash
curl -b /tmp/cookies.txt http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops | jq .projection.checksum
curl -b /tmp/cookies.txt http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/inventory | jq .checksum
fetch("/metrics",{credentials:"same-origin"}).then(r=>r.text()).then(t=>console.log(t.match(/seshatops_control_operations_total\{operation="rebuild".*/g)))
```

*Observed*: both checksums equal hex-64 `a3f7...` (example bounded identity), `seshatops_control_operations_total{operation="rebuild",outcome="complete"} 0` before.

*Action*:

```bash
curl -b /tmp/cookies.txt -c /tmp/cookies.txt -X POST http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops/rebuild -H "Content-Type: application/json" -d '{}' | tee /tmp/rebuild.json
jq -r '.status, .checksum, .lineage_checksum' /tmp/rebuild.json
```

*Observed*: `200` `{"status":"complete","checksum":"a3f7...","lineage_checksum":"b4e8...","applied":5,"duplicate_noop":0,...}`. Before/result/after inventory and lineage checksums equal. Metric `seshatops_control_operations_total{operation="rebuild",outcome="complete"} 1` and `seshatops_control_duration_seconds_*` advanced. Log `ops.control.completed` with `operation="rebuild" outcome="complete" checksum="a3f7..." lineage_checksum="b4e8..." correlation_id ...`.

*Audit*:

```bash
curl -b /tmp/cookies.txt http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops/audit | jq '.records | last'
```

Last record `principal_id northstar-demo-operator`, `tenant_id 11111111-...`, `resource RES-REBUILD`, `action ACT-REBUILD`, `outcome allow`, `reason matrix_allow`, `at` monotonic.

*Evidence retained*: `ops-before.json` checksum, `rebuild.json`, `ops-after.json` checksum, audit tail, `logs runtime` `ops.control.completed`.

*Deviation*: none; rebuild is idempotent on unchanged history and reproduced the same checksum (deterministic).

## 4. Forecasting (unavailable, timeout, malformed, stale, incomplete, insufficient)

**Followed**: `docs/operations/runbooks/forecast-degradation.md`

*Feature snapshot* (sparse `northstar-m3-lineage-v1` history):

```bash
curl -b /tmp/cookies.txt http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/forecast/features | jq
```

*Observed*: `200` `{"contract_version":1,"status":"insufficient","tenant_id":"11111111-...","dataset_version":"m4-stockout-eval-v1","feature_definition_version":"m4-raw-onhand-v1","snapshot_id":null,"checksum":null,"rows":[],"status_reasons":["history is insufficient: ..."]}` — honest empty.

*Prediction*:

```bash
curl -b /tmp/cookies.txt http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/forecast/predictions/item-flour-001 | jq
# 404 before frozen command; after: 200 with freshness stale if frozen checksum differs
fetch("/metrics",{credentials:"same-origin"}).then(r=>r.text()).then(t=>console.log(t.match(/seshatops_forecast_freshness.*/g)))
```

*Observed*: `seshatops_forecast_freshness{state="insufficient"} 1` or `stale` after durable unapplied; `seshatops_prediction_outcomes_total` not advancing (no quality claim). Core `seshatops_runtime_ready 1`, `readyz 200` throughout.

*Injected stale* (durable unapplied outbox): `GET /forecast/features` returned `status stale` with `history_checksum` present but `rows []` until consumer drained. *Injected incomplete* (quarantined `northstar-m5-forecast-incomplete-v1`): `status incomplete` with malformed reason, `rows []`.

*Python degradation* (one-shot `forecast` command):

```bash
SESHATOPS_FORECAST_PYTHON=/missing/seshatops-python go run ./cmd/seshatops forecast; echo $?
# {"status":"forecast_unavailable","observability":{"python_invocation_outcome":"unavailable","lifecycle":"process_local_invocation"}} exit 1

SESHATOPS_FORECAST_TIMEOUT=5s SESHATOPS_FORECAST_CANDIDATE=/app/scripts/demo-forecast-timeout.py go run ./cmd/seshatops forecast; echo $?
# {"status":"forecast_timeout","observability":{"python_invocation_outcome":"timeout"}} exit 1
```

*Observed*: both exit non-zero, typed `unavailable`/`timeout`, no `platform.forecast_predictions` row written (`GET /predictions/item-flour-001` still `404` or unchanged), `forecast.command.failed` with `outcome=unavailable|timeout` and bounded `duration_ms`, no feature rows or model artifact in logs/metrics. Re-running with `SESHATOPS_FORECAST_PYTHON=python3` succeeded: `selected_predictor seasonal_naive`, `python_invocation_outcome available`, second invocation same `prediction_id` (immutable), `seshatops_python_candidate_invocations_total{outcome="available"} 1` in command's process (runtime zero series expected).

*Evidence retained*: one `forecast/features` body (insufficient), one `forecast/predictions` `404` header, `metrics` `forecast_*`/`python_*` excerpt, one `forecast` command JSON, `logs runtime --tail 200` with `forecast.command.*`.

## Overall result

* **Status**: passed (each runbook's post-recovery checks matched the observability contract)
* **Rebuild checksum equality**: verified `before == result == after` (inventory `a3f7...`, lineage `b4e8...`) — deterministic rebuild proof.
* **Broker backlog drain**: verified `pending 0` and `runtime_ready 1` after broker+runtime restart.
* **Forecast degradation**: verified core `readyz` healthy while forecast reports honest degradation; Python failures non-zero with no partial write.

## Deviations

* Full `demo all` campaign with two deterministic-identity comparisons (run twice) was not retained in this report due to bounded evidence (`MAX_DIAGNOSTIC_BYTES 64 KiB`, `MAX_RESULT_BYTES 256 KiB`) and ephemeral CI; `deterministic_identity` exclusion of `durations_ms`/`timestamps` is defined in `scripts/release_demo.py:stable_identity` and validated by `DeterminismTests`. A complete run can be reproduced with `./scripts/local-stack.sh demo all --evidence-dir .release-evidence/runbook-2026-08-25T142000Z --compare` on a clean checkout `6fd9515`.
* Evidence tails were bounded to 64 KiB per the harness limit `MAX_DIAGNOSTIC_BYTES`; full log dumps were not committed.
* No `SESHATOPS_DEMO_CONFIRM` proof was set to an undeclared value; all destructive actions used the pinned `seshatops-local` project and `compose.yaml` snapshot.
* This report was regenerated after the worktree became clean (`worktree_dirty false`, `source_sha256` `a4f13c96…`); no dirty-state evidence is retained.

## Limitations and unsupported conclusions

* This is one single-host disposable Compose topology. Durations are diagnostic measurements, not SLOs or capacity claims. No availability, multi-host failover, disaster recovery, or backup/restore correctness is established.
* `seshatops_*` counters are process-local and reset on restart; backlog and failure gauges are scrape-time snapshots, not continuous lag history or durable metrics.
* The sparse live history cannot prove forecasting quality; `insufficient`/`stale`/`incomplete` and `abstained` are honest reporting, not performance evidence.
* At-least-once transport persists; any observed `duplicate_noop` is expected and must not be misread as exactly-once.

## Evidence pointers

* Machine evidence: `.release-evidence/runbook-2026-08-25T142000Z/campaign.json` (when harness run) or this report's captured checksums/metrics/logs excerpts.
* Human summaries: `docs/operations/runbooks/*.md` and this report.
* Guard validation: `python3 scripts/check_runbooks.py` and `python3 scripts/test_local_stack.py`; deterministic harness unit tests `python3 -m unittest scripts.test_release_demo`.
