# Forecast degradation

Local, disposable release evidence only. Not a production forecasting SLA or model-quality claim.

## Trigger and observable symptoms

The live Event Spine history is intentionally sparse (`northstar-m3-lineage-v1` — 5 events) and therefore does not reproduce the frozen M4 fixture (`northstar-m4-stockout-v1` with dense restock history). The Go forecast boundary reports this honestly rather than fabricating rows.

* `GET /v1/tenants/11111111-1111-4111-8111-111111111111/forecast/features` (`MX-008` `RES-FORECAST-FEATURES` `ACT-READ`) returns `200` with an explicit non-complete status and an empty `rows` array. One of:

  * `status == "insufficient"` — sparse history has too few days to form a snapshot (`history_checksum`/`projection_checksum` present but reasons indicate short history).
  * `status == "stale"` — durable `erp.outbox` rows remain unapplied (`inventory_projection` not yet consumed); `source_history_boundary.max_recorded_at` lags behind durable source.
  * `status == "incomplete"` — malformed retained event bytes (e.g., `northstar-m5-forecast-incomplete-v1` injected via disposable source) — `status_reasons` includes the malformed marker, no usable rows.

  In each case `snapshot_id == null` or hash-derived, `checksum == null`, `rows == []`. This is expected local behavior, not an error to hide.

* `GET /v1/tenants/11111111-1111-4111-8111-111111111111/forecast/predictions/{resource_id}` (`MX-009`) returns either:

  * `404 {"error":"not_found"}` — no prediction persisted for that `resource_id`/tenant (fresh stack).
  * `200` with `status == "abstained"` or `stale`/`unavailable` freshness (`freshness.status`), `stockout_risk == null`, explicit `abstention_reason` and `freshness.fresh_at`. Example: after `seshatops forecast` selecting `seasonal_naive`, a later sparse-history request correctly reports `freshness.status == "stale"` and the prediction's `lineage.feature_snapshot_checksum` differs from current complete source snapshot.

* Python degradation (one-shot `go run ./cmd/seshatops forecast`):

  * `python3` missing or `SESHATOPS_FORECAST_PYTHON=/missing/seshatops-python` → `outcome == "unavailable"` in command JSON `observability.python_invocation_outcome`, stderr contains `forecast.command.failed` with `outcome="unavailable"`, exit non-zero, no prediction row persisted.
  * `SESHATOPS_FORECAST_TIMEOUT=5s` + `SESHATOPS_FORECAST_CANDIDATE=/app/scripts/demo-forecast-timeout.py` (sleep 12s) → `outcome == "timeout"`, non-zero, no write.
  * Malformed Python output → `outcome == "invalid_response"`, non-zero, no write.

* Core health stays healthy during any above state:

  * `GET /readyz` `200 {"status":"ready"}`, `GET /livez` `200`, `seshatops_runtime_ready 1`. Forecast/Python availability never changes `/readyz` or the runtime-ready gauge (`docs/operations/observability.md`).
  * `seshatops_forecast_freshness{state="fresh" 0|1, stale 0|1, unavailable 0|1}` reflects the last prediction-read freshness (authorized reads only); initial all-zero means no observation yet.
  * `seshatops_prediction_outcomes_total{predictor="baseline"|"candidate",outcome="predicted"|"abstained"}` advances only on authorized `GET /forecast/predictions/*` or successful `forecast` command; it does not measure quality.

Distinguish: forecast degradation = `GET /forecast/*` `200` with explicit `stale`/`incomplete`/`insufficient` or `abstained`/`stale` freshness and core `ready=1`. It is not `seshatops_runtime_ready 0` (broker), not `processing_quarantined` (poison), not `409 not_releasable` (immutable conflict), not `401/403` (authorization denial — forecast reads use distinct `MX-008`/`MX-009`).

## Scope and safety assumptions

* Forecast feature route is read-only by construction: single `READ ONLY`/`REPEATABLE READ` PostgreSQL transaction over retained `erp.outbox`, `platform.inbox`, and `platform.inventory_projection` rows, replays in memory, never writes `feature` rows/snapshots, never calls a projection mutator, never holds Python credentials beyond the bounded subprocess deadline.
* `platform.ForecastService` selects the runtime predictor only from the frozen test outcome (`m4-stockout-eval-v1` → `seasonal_naive`); it invokes a candidate only through the bounded typed `forecast_candidate/stockout_candidate.py` subprocess boundary and persists via validated `platform.forecast_predictions` row.
* Tenant-scoped persistence: prediction reads validate `path tenant_id == assignment tenant` (`MX-009`), non-`northstar-demo-operator` tenant `22222222-2222-4222-8222-222222222222` remains `403` without leakage.
* Do not change freshness by disabling auth or by editing `platform.forecast_predictions` directly — direct DB edits are prohibited.
* Python failures are bounded and ordered: failure is typed (`unavailable`/`timeout`/`invalid_response`) and fails before prediction persistence.

## Diagnosis commands and expected signals

All commands below are implemented (`#99`-`#104`). No hidden helpers or aspirational dashboards.

```bash
./scripts/local-stack.sh status
./scripts/local-stack.sh logs runtime --tail 200
```

Authenticated diagnosis (login at `http://web.seshatops.localhost:5173` as `northstar-demo-operator`; requires `ROLE-OPS-READER` for `MX-008`/`MX-009`):

```js
fetch("/metrics", {credentials:"same-origin"}).then(r=>r.text()).then(t=>{
  console.log(t.match(/seshatops_runtime_ready .*/g));
  console.log(t.match(/seshatops_forecast_freshness.*/g));
  console.log(t.match(/seshatops_prediction_outcomes_total.*/g));
  console.log(t.match(/seshatops_python_candidate.*/g));
})
```

Feature snapshot:

```bash
curl -b /tmp/cookies.txt \
  http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/forecast/features | jq
```

Expected non-complete shape:

```json
{"contract_version":1,"status":"insufficient","tenant_id":"11111111-...","dataset_version":"m4-stockout-eval-v1","feature_definition_version":"m4-raw-onhand-v1","snapshot_id":null,"checksum":null,"rows":[],"status_reasons":["history is insufficient: ..."]}
```

Prediction read:

```bash
curl -b /tmp/cookies.txt \
  http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/forecast/predictions/item-flour-001 | jq .freshness
```

* `404` if no prediction yet, or `200` with `freshness.status == "stale"|"unavailable"` after frozen command vs sparse live history mismatch.

One-shot frozen forecast command (disposable DB only, requires `postgres://` disposable name):

```bash
export SESHATOPS_DATABASE_URL='postgres://seshatops:seshatops-local-only@postgres:5432/seshatops_northstar_disposable?sslmode=disable'
export SESHATOPS_FORECAST_PYTHON='python3'
export SESHATOPS_FORECAST_CONFIRM='I_UNDERSTAND_FROZEN_M4_FORECAST_WRITE'
go run ./cmd/seshatops forecast | jq .observability
```

Through Compose (local stack):

```bash
docker compose --project-name seshatops-local --file compose.yaml run --rm \
  -e SESHATOPS_FORECAST_CONFIRM='I_UNDERSTAND_FROZEN_M4_FORECAST_WRITE' runtime forecast | jq
```

Expected successful result excerpt:

```json
{"protocol":"m4-stockout-eval-v1","dataset_version":"m4-stockout-v1","selected_predictor":"seasonal_naive","prediction_status":"predicted","observability":{"correlation_id":"...","python_invocation_outcome":"available","lifecycle":"process_local_invocation"}}
```

Failure shapes (typed, non-zero, no row written):

```bash
SESHATOPS_FORECAST_PYTHON=/missing/seshatops-python go run ./cmd/seshatops forecast; echo $?
# {"status":"forecast_unavailable","observability":{"python_invocation_outcome":"unavailable"}}  exit 1

SESHATOPS_FORECAST_TIMEOUT=5s SESHATOPS_FORECAST_CANDIDATE=/app/scripts/demo-forecast-timeout.py \
  go run ./cmd/seshatops forecast; echo $?
# {"status":"forecast_timeout","observability":{"python_invocation_outcome":"timeout"}}  exit 1
```

Logs (bounded redaction):

```json
{"time":"2026-08-25T12:03:00.000Z","level":"INFO","msg":"forecast.command.failed","correlation_id":"d4e5f6g7-...","outcome":"timeout","duration_ms":5012}
```

No log contains raw feature rows, model artifacts, error strings, or event bodies.

Controls that are *not* forecast degradation:

```bash
curl -i http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/forecast/features?horizon=999
# 400 {"error":"unsupported_forecast_query"} — caller-supplied controls are rejected

curl -X POST http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/forecast/features
# 405 {"error":"method_not_allowed"} Allow: GET
```

## Smallest safe action sequence

1. **Confirm core health is unaffected** — verify this is forecast-only:

   ```bash
   curl -s http://127.0.0.1:8080/readyz | grep ready
   curl -b /tmp/cookies.txt http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/inventory | jq .checksum
   ./scripts/local-stack.sh logs runtime --tail 50
   ```

   If `/readyz` is `503` or `seshatops_runtime_ready 0`, follow `broker-interruption.md` — not this runbook.

2. **Classify forecast source state** — call the feature route once (it is deterministic and order-independent):

   ```bash
   curl -b /tmp/cookies.txt \
     http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/forecast/features | jq '{status, status_reasons, rows: (.rows|length), dataset_version, feature_definition_version}'
   ```

   * `insufficient` — do nothing; sparse `northstar-m3-lineage-v1` history is the intended live state. No manual backfill.
   * `stale` — durable outbox rows are pending apply. First ensure broker-consumer path is healthy (`./scripts/local-stack.sh status`, `seshatops_outbox_backlog_records_pending`), then re-read `features` after `consumer.cycle.completed`. The stale→complete transition is eventual, not immediate.
   * `incomplete` — retained `erp.outbox` bytes are malformed (`northstar-m5-forecast-incomplete-v1` demo). This diagnostic row is quarantined; `forecast` reads will remain `incomplete` until the source is reset via tenant-scoped `reset-northstar` path (not by editing the outbox row).

3. **For prediction freshness mismatch** — compare lineage:

   ```bash
   curl -b /tmp/cookies.txt \
     http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/forecast/predictions/item-flour-001 | jq '{status, freshness, lineage}'
   ```

   If `freshness.status == "stale"` and the frozen `seshatops forecast` result checksum differs from current `features` snapshot, the prediction is honestly reported as stale — correct behavior. No replay of the live sparse history will make it `fresh`.

4. **For Python unavailable/timeout/malformed** — only in the one-shot `forecast` command process:

   * Verify no prediction was persisted:

     ```bash
     curl -b /tmp/cookies.txt \
       http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/forecast/predictions/item-flour-001
     # 404 if stack was fresh; unchanged checksum if prior v1 existed
     ```

   * Re-run with valid Python once (disposable env only):

     ```bash
     export SESHATOPS_FORECAST_PYTHON='python3'
     go run ./cmd/seshatops forecast | jq .observability.python_invocation_outcome
     # "available"
     ```

   * Verify core operations continued through the failure window:

     ```bash
     curl -b /tmp/cookies.txt \
       http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/inventory | jq .checksum
     curl -s http://127.0.0.1:8080/readyz
     ```

## Stop / rollback / escalation conditions

* Stop if `401 unauthenticated` or `403 forbidden` on `GET /forecast/features` — caller lacks `MX-008`, not a `stale` condition. Re-login; do not broaden the allow-list.
* Stop if `405` on `POST` to the forecast routes — the route is read-only by design; do not retry with a write.
* Stop if `400` for query overrides (`horizon`, `cutoff`, `dataset`) — the server contract is fixed; caller cannot change `m4-stockout-eval-v1`.
* Do not prescribe `POST /ops/quarantine/release` for an `incomplete` forecast snapshot — that control targets outbox/inbox quarantine, not the forecast replay boundary. Use the smallest boundary that matches the signal.
* No rollback edits `platform.forecast_predictions`. If a Python failure left no row, the next successful `forecast` command will persist the deterministic `seasonal_naive` advisory again.
* Escalation (local only): capture `GET /forecast/features` body (≤64 KiB), `GET /metrics` `forecast_*`/`python_*` lines, and `logs runtime --tail 200` filtered to `forecast.command.*`, then stop.

## Post-recovery checks and evidence to retain

* `GET /readyz` `200 ready`, `seshatops_runtime_ready 1` throughout forecast degradation and after any retry.
* `GET /forecast/features` status correctly reflects current source state (`complete` only with dense history, otherwise the declared non-complete status with empty rows). For this release that means `insufficient` is a passing verification of honest reporting.
* If `seshatops forecast` was re-run successfully: JSON result `prediction_status == "predicted"`, `observability.python_invocation_outcome == "available"`, `lifecycle == "process_local_invocation"`, dual invocation yields same immutable `prediction_id` and checksums.
* If Python failure: command exited non-zero, no new `forecast_predictions` row (`GET /predictions/*` `404` or unchanged), `seshatops_python_candidate_invocations_total{outcome="timeout"|"unavailable"}` advanced only in the forecast process (runtime zero series expected), core inventory and SSE remain functional.
* Retain (redacted, ≤64 KiB): one `GET /forecast/features` body, one `GET /forecast/predictions/*` body or `404` header, one `GET /metrics` `forecast_*`/`python_*` excerpt, one `forecast` command JSON result, and `logs runtime --tail 200` with `forecast.command.*`. Verify no feature rows beyond empty, no model artifact, and no token/cookie appears.

## Limitations and unsupported production conclusions

* The spare Event Spine history cannot prove forecasting quality. `insufficient`/`stale`/`incomplete` are truth reporting, not model metrics.
* The frozen M4 fixture (`northstar-m4-stockout-v1`) and the live Northstar fixture (`northstar-m3-lineage-v1`) are separate datasets by design; a successful `forecast` command does not make live history `fresh`.
* Durations and `observed_at` are diagnostic; no SLO, availability, or business-impact claim is supported.
* Python is a one-shot artifact producer with a bounded deadline, not a service. `seshatops_python_candidate_available` is a single-process gauge, not readiness.

