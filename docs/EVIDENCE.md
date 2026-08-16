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
Measured on a working tree based on `557d906`. Hosted CI is not claimed until
the pull request that lands this campaign has workflow runs.

On a single-host Testcontainers topology: an authorized same-tenant batch trace
returns upstream supplier/lot and downstream shipment/order hops with
provenance; duplicate delivery does not duplicate lineage effects; an
incompatible traceability event is quarantined while unrelated valid work
applies; unchanged history rebuilds to the same inventory and lineage
checksums. Tenant-negative trace HTTP is fail-closed. Not production, not
backup/restore.
