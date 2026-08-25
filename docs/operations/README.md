# Operations

Implemented local release procedures only. Not a production runbook or SLO.

## Observability

* [Release observability](observability.md) — bounded `/metrics` and structured-log contract, access, lifecycles, and redaction.

## Recovery runbooks

Each runbook refers only to implemented CLI/API/Compose commands from the current release. Follow them without relying on repository-author knowledge.

* [Broker interruption and outbox backlog recovery](runbooks/broker-interruption.md) — retryable transport failure, durable `pending` backlog, `seshatops_runtime_ready 0`, restart and drain.
* [Poison and incompatible event isolation](runbooks/poison-isolation.md) — quarantined poison inspection, `409 not_releasable` vs safely releasable outbox quarantine, release/replay decision, unrelated-work verification.
* [Tenant-scoped projection rebuild](runbooks/rebuild-checksum.md) — authorized `POST .../ops/rebuild`, audit-before-mutate, before/result/after checksum equality.
* [Forecast degradation](runbooks/forecast-degradation.md) — `unavailable`/`timeout`/`invalid_response`, `stale`/`incomplete`/`insufficient` honest reporting, forecast-only vs core health.

Exercise evidence: [Runbook exercise report](../evaluation/RUNBOOK_EXERCISE_REPORT.md). Demo harness for the same failure classes: [Getting started — Release demonstration campaign](../getting-started.md#release-demonstration-campaign).

Distinguish retryable transport failure, quarantined poison, immutable conflict, authorization denial, and forecast-only degradation. Do not prescribe replay/release when the state is not safely releasable.
