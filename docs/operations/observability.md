# Release observability

SeshatOps exposes bounded local release evidence, not a monitoring platform,
production telemetry service, distributed tracing system, alert policy, or
SLO. PostgreSQL remains the transactional authority; Redpanda remains
at-least-once transport.

## Access and safe use

`GET /metrics` returns Prometheus text format (`text/plain; version=0.0.4`)
and requires both a fresh Go-owned session and `MX-010`:
`ROLE-RELEASE-OBSERVER` on Go-selected `SCOPE-RUNTIME`,
`RES-RELEASE-METRICS`, `ACT-READ`. The scope is not accepted from a path,
query, header, body, or IdP claim. `401` and `403` return only the existing
minimal JSON error; they never return metrics.

The local Compose identity `northstar-demo-operator` has this explicit role.
After logging in, inspect the same-origin endpoint from the browser console:

```js
fetch("/metrics", { credentials: "same-origin" })
  .then((response) => response.text())
  .then(console.log)
```

`/metrics` is proxied by Vite to Go, like `/auth` and `/v1`; it is not a
browser authorization shortcut. The response is aggregate release state and
has no tenant, user, event, resource, correlation, URL, error, or payload
labels. It must not be made unauthenticated.

## Metrics contract

All counters below are process-local memory and reset when the owning process
restarts. They are useful release evidence only; they are not durable history
or production metrics. Labels are exhaustive, fixed enums shown in braces.

| Metric | Unit and labels | Source and limitation |
| --- | --- | --- |
| `seshatops_relay_publish_outcomes_total` | events; `{outcome=published\|transient\|quarantined\|ambiguous}` | Relay cycles in the long-lived runtime. `ambiguous` means broker acknowledgement succeeded but durable outbox status did not. |
| `seshatops_consumer_processing_outcomes_total` | events; `{outcome=processed\|skipped}` | Consumer cycles in the long-lived runtime. |
| `seshatops_consumer_ack_outcomes_total` | events; `{outcome=acknowledged\|withheld\|commit_failed}` | `withheld` means no offset acknowledgement was attempted; it is not a broker lag value. |
| `seshatops_control_operations_total` and `seshatops_control_duration_seconds_{sum,count}` | operations and seconds; `{operation=quarantine_release\|replay\|rebuild,outcome=...}` | Authorized control request outcomes and wall-clock handler duration. Checksum identity is logged, never a label. |
| `seshatops_http_requests_total` | requests; `{route=auth\|inventory\|ops\|forecast\|metrics\|health\|other,outcome=success\|client_error\|server_error}` | Runtime HTTP responses. Route is a fixed class, not a raw URL. |
| `seshatops_auth_denials_total` | denials; `{route=...,reason=unauthenticated\|forbidden}` | Runtime `401` and `403` responses. It does not identify principals or tenants. |
| `seshatops_prediction_outcomes_total` | observations; `{predictor=candidate\|baseline,outcome=predicted\|abstained}` | Authorized forecast prediction reads and the separate forecast command. It does not measure forecast quality. |
| `seshatops_forecast_freshness` | one-hot gauge; `{state=fresh\|stale\|unavailable}` | Last authorized prediction-read freshness state in the long-lived runtime. `0` for every state means none observed since start. |
| `seshatops_python_candidate_invocations_total` | invocations; `{outcome=available\|unavailable\|timeout\|invalid_response}` | The one-shot `seshatops forecast` process only. The long-lived runtime's fixed zero series do not report CLI invocations; use that command's JSON evidence. |
| `seshatops_python_candidate_available` | one/zero | Last Python availability observed by that one forecast invocation. It is not core runtime readiness; the runtime scrape's initial zero is not a degradation signal. |

The following gauges are current scrape-time snapshots. They are refreshed by
the authorized endpoint from PostgreSQL and therefore are database-backed
observations, not process-local counters. They remain aggregate and do not
promise production monitoring durability or sampling frequency.

| Gauge | Unit | Meaning |
| --- | --- | --- |
| `seshatops_outbox_backlog_records_pending` | rows | Pending outbox rows. |
| `seshatops_outbox_backlog_records_publishing` | rows | Leased/publishing outbox rows. |
| `seshatops_outbox_backlog_records_quarantined` | rows | Quarantined outbox rows. |
| `seshatops_outbox_oldest_unpublished_age_seconds` | seconds | Age of the oldest pending or publishing row; zero means none or a sub-second age. |
| `seshatops_processing_failures_retrying` | rows | Consumer processing failures in retrying state. |
| `seshatops_processing_failures_quarantined` | rows | Quarantined consumer processing failures. |
| `seshatops_processing_quarantined_gap` | rows | Durable residual `quarantined_gap` inbox rows. |
| `seshatops_processing_oldest_failure_age_seconds` | seconds | Age of the oldest processing failure; zero means none or a sub-second age. |
| `seshatops_processing_oldest_gap_age_seconds` | seconds | Age of the oldest residual gap; zero means none or a sub-second age. |

The following gauges are process-local snapshots and reset with the runtime.
They are not durable metrics.

| Gauge | Unit | Meaning |
| --- | --- | --- |
| `seshatops_consumer_observed_lag_records` | records | Sum of `high_watermark - first_returned_offset` for partitions that returned records in the most recent poll. It is an observed fetched-batch snapshot, not continuous consumer-group lag. |
| `seshatops_consumer_observed_lag_known` | one/zero | One only when the latest poll returned at least one partition with records and a high watermark. |
| `seshatops_runtime_ready` | one/zero | One when database, migrations, broker, relay, and consumer readiness are all healthy. `database`/`migrations`/`broker` reflect startup `Ping` + `Migrate` success; runtime broker or DB outages after startup surface within one interval via relay/consumer worker failures (relay probes the broker only when idle `claimed == 0` or `transient > 0`, caching healthy successes for one `CycleTimeout` default `10s` and re-probing immediately on failure, ordered so `relay.cycle.failed` flips readiness). It intentionally excludes forecast/Python availability. |
| `seshatops_build_info` | one | `seshatops_build_info{version="v0.1.0",commit="<short SHA>"} 1` — immutable build identity for this process (also `GET /version` and `seshatops version`). Bounded fixed labels only; `0` with `unknown` when not stamped. |

No metric label is derived from an event ID, correlation ID, tenant, user,
resource, raw error, raw URL, cookie, token, OIDC assertion, credential, raw
event body, feature row, or model artifact. New metric dimensions require an
explicit fixed enum and documentation update.

## Structured logs and correlation

The executable configures `slog` JSON logs on stderr. Stable event names are
`http.request.completed`, `authorization.denied`, `relay.cycle.completed`,
`relay.cycle.failed`, `consumer.cycle.completed`, `consumer.cycle.failed`,
`ops.control.completed`, `forecast.command.completed`,
`forecast.command.failed`, and `worker.retrying`. `relay.cycle.failed` is logged after the idle/transient broker probe so the event reflects the outage and readiness flip.

HTTP receives a generated UUIDv4-style `X-Correlation-ID`; caller-provided
correlation headers are ignored. The ID is propagated through the HTTP
context. Relay and consumer worker cycles and the forecast command generate
their own IDs. Logs may contain only `correlation_id`, fixed route/operation/
worker/outcome/reason/predictor/freshness values, HTTP status, duration, and validated
rebuild inventory and lineage checksums. They never log raw requests, headers, cookies, credentials,
tokens, assertions, event bodies, private identifiers, raw feature rows, model
artifacts, or error strings.

Inspect bounded recent runtime evidence with:

```bash
./scripts/local-stack.sh logs runtime
```

To validate redaction for a local run, capture only the bounded tail needed
for diagnosis and search it for the deliberately supplied fixture markers or
credentials before retaining it. Do not commit log dumps.

## Forecast evidence

Run the frozen command through the packaged stack:

```bash
./scripts/local-stack.sh quickstart
```

The `forecast` command's existing JSON output now includes an `observability`
object with its generated correlation ID, Python invocation outcome, selected
predictor, prediction outcome, and lifecycle
`process_local_invocation`. A Python unavailable, timeout, or invalid-response
outcome fails before prediction persistence and is logged without the input,
artifact, or error text. Forecast degradation never changes `/readyz` or the
core runtime-ready gauge.

## Recovery runbooks

When a local release shows a failure signal, follow the release runbooks instead of ad-hoc database or broker edits:

* [Broker interruption and outbox backlog recovery](runbooks/broker-interruption.md)
* [Poison and incompatible event isolation](runbooks/poison-isolation.md)
* [Tenant-scoped projection rebuild](runbooks/rebuild-checksum.md)
* [Forecast degradation](runbooks/forecast-degradation.md)

These refer only to commands and signals that exist in this release; see the [operations index](README.md) and [Getting started](../getting-started.md#recovery-runbooks).

## Release campaign evidence

Run the guarded packaged campaign, or one of its eight named scenarios, with:

```bash
export SESHATOPS_DEMO_CONFIRM='I_UNDERSTAND_DISPOSABLE_LOCAL_DEMO'
./scripts/local-stack.sh demo all
./scripts/local-stack.sh demo broker-recovery
```

The procedures and evidence layout are documented in
[Getting started](../getting-started.md#release-demonstration-campaign). The
harness observes the aggregate readiness, relay/consumer, outbox backlog,
authorization-denial, and forecast failure signals defined here alongside
public responses and durable database state. It retains measured durations as
diagnostic observations, not SLOs or recovery objectives.

Use only this guarded path for broker interruption, incompatible fixture
events, rebuild, cross-tenant denial, forecast-source faults, and Python
degradation demonstrations. The target check fixes the Compose project,
services, disposable PostgreSQL database, broker, local marker, protected
ports, and confirmation before destructive or fault actions. Fault tooling is
packaged and local-only; this observability contract does not add a public
fault-injection endpoint or prescribe direct projection edits.
