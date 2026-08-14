# Evidence

What may be claimed about this repository. Not production evidence.

Statuses: **Planned** (intended, not built), **Implemented** (code exists),
**Observed** (measured in a named experiment with recorded limitations),
**Reproduced** (prior observation repeated), **Superseded** (no longer the
basis for claims). Promotion to Observed requires the experiment report.
Screenshots and documentation CI cannot promote a claim. Future claim IDs are
assigned when an experiment is planned.

## Observed

| ID | Claim | Evidence | Environment | Commit | Limitations |
| --- | --- | --- | --- | --- | --- |
| `CLM-003` | Outbox preserves accepted intent during broker outage | [Event Spine report](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md) | Test | `a4e5d47`; PR #41 `b59b760` | Single-host Testcontainers Redpanda stop/start |
| `CLM-004` | Duplicate delivery does not duplicate inventory effects | same | Test | same | Not exactly-once delivery |
| `CLM-005` | Versioned contracts and ordering compatibility | same | Test | same | Schema v1; gap/stale/reorder/poison only |
| `CLM-006` | Unchanged history rebuilds to the same checksum | same | Test | same | Not backup/restore |
| `CLM-007` | Tenant isolation on Identity HTTP | [Identity report](docs/evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md) | Test | `173cedb`; PR #61 `1f19c58` | Implemented HTTP only. Operator=reviewer |
| `CLM-008` | Privileged HTTP is default-deny | same | Test | same | Library/test OIDC. Not production revocation |
| `CLM-009` | Authorized tenant-scoped ops visibility | same | Test | same | `GET .../ops`. Not an SLO platform |
| `CLM-010` | Quarantine, replay, rebuild fail closed | same | Test | same | Authorization subset. Not Traceability restore |

Hosted CI, Event Spine PR #41: Go
[31588310097](https://github.com/G1DO/seshatops/actions/runs/31588310097),
Web [31588310148](https://github.com/G1DO/seshatops/actions/runs/31588310148),
Docs [31588310157](https://github.com/G1DO/seshatops/actions/runs/31588310157).

Hosted CI, Identity PR #61: Go
[31671557442](https://github.com/G1DO/seshatops/actions/runs/31671557442),
Web [31671557513](https://github.com/G1DO/seshatops/actions/runs/31671557513),
Docs [31671557450](https://github.com/G1DO/seshatops/actions/runs/31671557450).
