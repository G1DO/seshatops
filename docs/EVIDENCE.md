# Evidence

What the recorded test runs measured. Not production evidence.

## Event Spine

[Report](evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md).
Runtime commit `a4e5d47`; hosted CI on PR #41 `b59b760`.

On a single-host Testcontainers topology: accepted orders survive broker
outage; duplicate delivery does not duplicate inventory effects; schema v1
gap/stale/reorder/poison paths quarantine instead of applying; unchanged
history rebuilds to the same checksum. Not exactly-once delivery, not
backup/restore.

## Identity HTTP

[Report](evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md).
Runtime commit `173cedb`; hosted CI on PR #61 `1f19c58`. Operator = reviewer.

On implemented HTTP: tenant isolation, default-deny privileged routes,
authorized same-tenant ops visibility, and fail-closed quarantine/replay/rebuild
with audit-insert-before-mutate. Library/test OIDC; process-local sessions and
assignments. Not production revocation, pentest, or restore.

## M3 traceability and recovery

[Report](evaluation/M3_TRACEABILITY_RECOVERY_EXIT_GATE_EXPERIMENT_REPORT.md).
Runtime commit `a43e8a5`; hosted CI on PR #84 `a43e8a5`. Operator = reviewer.

On a single-host Testcontainers topology: an authorized same-tenant batch trace
returns upstream supplier/lot and downstream shipment/order hops with
provenance; duplicate delivery does not duplicate lineage effects; an
incompatible traceability event is quarantined while unrelated valid work
applies; unchanged history rebuilds to the same inventory and lineage
checksums. Tenant-negative trace HTTP is fail-closed. Not production, not
backup/restore.

## M4 forecast feature snapshots

[Specification](design/specifications/forecast-feature-snapshots.md).

The pure `forecast` builder has focused coverage for order-independent
deterministic rows, snapshot identity/checksum rebuilds, source-boundary
identity, cutoff-safe earlier rows, invalid quantities, label-free
serialization, and insufficient short history. The Go API and platform tests
also cover the intended tenant, method, authorization, replay, and explicit
non-complete response paths; database-backed integration cases require the
repository's Docker/Testcontainers environment.

The feature route is read-only by construction: one PostgreSQL
`READ ONLY`/`REPEATABLE READ` transaction reads retained outbox bytes, inbox
dispositions, and the inventory projection, then replays in memory. It has no
feature table, snapshot write, Python credential, label, raw event-byte, or
model-output path. The current sparse Event Spine history does not reproduce
the dense M4 restock fixture, so a live request may correctly return
`insufficient` or `incomplete`; this is an explicit limitation rather than a
forecast-quality result. These are repository/test claims, not production
isolation or forecasting-quality claims.

## M4 learned stockout candidate

The offline candidate producer has focused tests for deterministic train-only
fitting, typed risk and Wilson uncertainty output, insufficient-support
abstention, stale/malformed input rejection, and artifact-only CLI behavior.
The Go evaluator tests strict artifact lineage, prediction coverage,
abstention-aware fallback to the deterministic baseline, and JSON contract
rejection. These are implementation-contract claims only; no learned
candidate is recorded as promoted or as production forecasting evidence.

## M4 integrated stockout-intelligence exit gate

[Report](evaluation/M4_STOCKOUT_INTELLIGENCE_EXIT_GATE_EXPERIMENT_REPORT.md).

The integrated frozen test rebuilt the declared Northstar dataset and feature
snapshot, ran the offline Python candidate, evaluated all chronological splits
in Go, and selected `seasonal_naive` as the deterministic runtime predictor.
The candidate was not promoted because its test average precision was lower
than the qualifying baseline. The pure Go, Python, typed-boundary, and web
checks recorded in the report passed locally. PostgreSQL-backed persistence,
authorized read, and tenant-negative HTTP cases were explicitly skipped in
the local Windows environment because Docker was unavailable; no local exit
gate pass is claimed for those cases. This remains test-environment evidence,
not production quality, availability, or business-impact evidence.
