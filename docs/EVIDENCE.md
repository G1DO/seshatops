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

Hosted CI: Go
[31588310097](https://github.com/G1DO/seshatops/actions/runs/31588310097),
Web [31588310148](https://github.com/G1DO/seshatops/actions/runs/31588310148),
Docs [31588310157](https://github.com/G1DO/seshatops/actions/runs/31588310157).

## Identity HTTP

[Report](evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md).
Runtime commit `173cedb`; hosted CI on PR #61 `1f19c58`. Operator = reviewer.

On implemented HTTP: tenant isolation, default-deny privileged routes,
authorized same-tenant ops visibility, and fail-closed quarantine/replay/rebuild
with audit-insert-before-mutate. Library/test OIDC; process-local sessions and
assignments. Not production revocation, pentest, or restore.

Hosted CI: Go
[31671557442](https://github.com/G1DO/seshatops/actions/runs/31671557442),
Web [31671557513](https://github.com/G1DO/seshatops/actions/runs/31671557513),
Docs [31671557450](https://github.com/G1DO/seshatops/actions/runs/31671557450).
