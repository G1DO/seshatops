# SeshatOps Evidence Ledger

**Status:** `CLM-003`–`CLM-006` are `Observed` for the Issue #30 test-environment
Event Spine campaign. `CLM-007`–`CLM-010` are `Observed` for the Issue #50
test-environment Identity & Operations campaign. All other claims remain
`Planned`. This ledger is not production evidence.

Public README, demo, CV, and interview language may not exceed this file.

## Vocabulary

Canonical statuses: [docs/evidence/CLAIM_STATUS_VOCABULARY.md](docs/evidence/CLAIM_STATUS_VOCABULARY.md).

`Planned` · `Implemented` · `Observed` · `Reproduced` · `Superseded`

- `Planned` means intended, not built.
- `Implemented` means code exists, not that it is correct or secure.
- `Observed` applies only to the recorded commit, environment, and limitations.
- Promotion requires a linked experiment record. Screenshots, design docs, and
  documentation CI alone cannot promote a claim.
- Claim IDs are stable. Do not silently reuse an ID.

## Observed claims

| Claim ID | Claim | Status | Capability | Sequence | Evidence | Environment | Commit/release | Date | Limitations |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `CLM-003` | Transactional outbox handling preserves accepted source intent during broker outage. | Observed | `CAP-005` | Event Spine | [EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md); Redpanda stop/start | Test environment | `a4e5d47` (runtime); PR #41 `b59b760` | 2026-08-12 | Single-host Testcontainers Redpanda stop/start only. |
| `CLM-004` | Event processing is duplicate-safe. | Observed | `CAP-007` | Event Spine | [EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md); duplicate delivery and consumer crash windows | Test environment | `a4e5d47` (runtime); PR #41 `b59b760` | 2026-08-12 | One durable inventory-projection effect under tested duplicates. Not exactly-once delivery. |
| `CLM-005` | Versioned event contracts and ordering compatibility are enforced. | Observed | `CAP-006` | Event Spine | [EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md); poison, unsupported version, aggregate-version gap | Test environment | `a4e5d47` (runtime); PR #41 `b59b760` | 2026-08-12 | Schema v1 and Event Spine gap/stale/reorder/poison dispositions only. |
| `CLM-006` | Unchanged history deterministically rebuilds projections. | Observed | `CAP-007` | Event Spine | [EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md); retained-history rebuild checksum | Test environment | `a4e5d47` (runtime); PR #41 `b59b760` | 2026-08-12 | Complete retained history checksum A==B. Not backup/restore (ADR-Q-005). |
| `CLM-007` | Tenant isolation holds on Identity HTTP surfaces (inventory, ops, quarantine, replay, rebuild, audit). | Observed | `CAP-010` | Identity | [IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md) | Test environment | `173cedb` (runtime); PR #61 `1f19c58` | 2026-08-13 | Implemented HTTP only. Not retrieval, approvals, commands, exports, or production isolation. Operator=reviewer. |
| `CLM-008` | Privileged Identity HTTP operations are default-deny. | Observed | `CAP-010` | Identity | [IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md) | Test environment | `173cedb` (runtime); PR #61 `1f19c58` | 2026-08-13 | `MX-004`–`MX-007` plus forged/stale session in library/test scope. Not production IdP revocation. |
| `CLM-009` | Authorized tenant-scoped ops visibility is available. | Observed | `CAP-012` | Identity | [IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md) | Test environment | `173cedb` (runtime); PR #61 `1f19c58` | 2026-08-13 | `GET /v1/tenants/{tenant_id}/ops` (`MX-002`/`MX-003`). Not an SLO platform. |
| `CLM-010` | Quarantine release, replay, and rebuild are authorized and fail closed on Identity HTTP. | Observed | `CAP-013` | Identity | [IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md) | Test environment | `173cedb` (runtime); PR #61 `1f19c58` | 2026-08-13 | Authorization subset only. Not Traceability restore. Irreversible-effect replay and quarantine-recovery campaigns remain Planned. |

Hosted CI for Event Spine PR #41 `b59b760`: Go
[31588310097](https://github.com/G1DO/seshatops/actions/runs/31588310097),
Web [31588310148](https://github.com/G1DO/seshatops/actions/runs/31588310148),
Docs [31588310157](https://github.com/G1DO/seshatops/actions/runs/31588310157).

Hosted CI for Identity PR #61 `1f19c58`: Go
[31671557442](https://github.com/G1DO/seshatops/actions/runs/31671557442),
Web [31671557513](https://github.com/G1DO/seshatops/actions/runs/31671557513),
Docs [31671557450](https://github.com/G1DO/seshatops/actions/runs/31671557450).

## Planned claims

| Claim ID | Claim | Capability | Sequence |
| --- | --- | --- | --- |
| `CLM-001` | Project Constitution integrated exit gate is satisfied. | `CAP-003` | Constitution |
| `CLM-002` | Clean-room governance keeps public artifacts independent from Ahoy. | `CAP-001` | Constitution |
| `CLM-011` | Fictional operational lineage and recall are traceable. | `CAP-014` | Traceability |
| `CLM-012` | Backup restoration preserves readable, bounded, integrity-checked state. | `CAP-016` | Traceability |
| `CLM-013` | Stockout evaluation is reproducible over a frozen temporal split. | `CAP-021` | Stockout |
| `CLM-014` | Forecasts expose uncertainty and abstain when appropriate. | `CAP-020` | Stockout |
| `CLM-015` | Dataset, feature, model, and evaluator lineage is reproducible. | `CAP-021` | Stockout |
| `CLM-016` | Typed proposals cannot directly authorize or execute business changes. | `CAP-022` | Approvals |
| `CLM-017` | Authorization and approval freshness are rechecked at execution. | `CAP-024` | Approvals |
| `CLM-018` | Equivalent command retries produce one durable business effect. | `CAP-025` | Approvals |
| `CLM-019` | Durable receipts distinguish confirmed, failed, and uncertain outcomes. | `CAP-026` | Approvals |
| `CLM-020` | Retrieval is permission-aware. | `CAP-027` | Governed RAG |
| `CLM-021` | Citations are authorized and supported by the returned evidence. | `CAP-028` | Governed RAG |
| `CLM-022` | Governed-RAG retrieval and answer evaluation is reproducible. | `CAP-030` | Governed RAG |
| `CLM-023` | Prompt-injection resistance is evaluated. | `CAP-029` | Governed RAG |
| `CLM-024` | Cross-tenant retrieval leakage is prevented. | `CAP-029` | Governed RAG |
| `CLM-025` | Governed-RAG refusal behavior is evaluated. | `CAP-028` | Governed RAG |
| `CLM-026` | Telemetry is correlated across traces, metrics, and logs. | `CAP-031` | Reliability |
| `CLM-027` | SLO evidence is reproducible and environment-scoped. | `CAP-032` | Reliability |
| `CLM-028` | Load and tail-latency evidence is reproducible. | `CAP-033` | Reliability |
| `CLM-029` | Fault and degraded-mode evidence is reproducible. | `CAP-034` | Reliability |
| `CLM-030` | Restore and recovery campaigns produce scoped evidence. | `CAP-035` | Reliability |
| `CLM-031` | Cloud deployment is reproducible for the declared environment. | `CAP-036` | Reliability |
| `CLM-032` | Cost evidence is scoped to a declared environment. | `CAP-037` | Reliability |
| `CLM-033` | The public demonstration is deterministic. | `CAP-038` | Portfolio |
| `CLM-034` | The released repository runs independently of Ahoy. | `CAP-039` | Portfolio |
| `CLM-035` | README, CV, demo, and portfolio claims are evidence-backed. | `CAP-040` | Portfolio |

## Capability IDs

Stable join keys for claims. Status here is evidence status, not GitHub
milestone state. Roadmap intent lives in Notion.

| ID | Capability | Sequence | Status |
| --- | --- | --- | --- |
| `CAP-001` | Project constitution and clean-room governance | Constitution | Planned |
| `CAP-002` | Architecture, correctness, and security models | Constitution | Planned |
| `CAP-003` | Evidence and claim governance | Constitution | Planned |
| `CAP-004` | Synthetic ERP and deterministic workload | Event Spine | Observed (test env; via `CLM-003`–`006`) |
| `CAP-005` | Transactional outbox | Event Spine | Observed (`CLM-003`) |
| `CAP-006` | Versioned event contracts and transport | Event Spine | Observed (`CLM-005`) |
| `CAP-007` | Deduplicated operational projections | Event Spine | Observed (`CLM-004`/`006`) |
| `CAP-008` | TypeScript live operations view | Event Spine | Planned (UI exists; authorized live-view claim route open) |
| `CAP-009` | Identity and sessions | Identity | Observed (`CLM-008`) |
| `CAP-010` | Tenant-aware authorization | Identity | Observed (`CLM-007`/`008`) |
| `CAP-011` | Service identities and least privilege | Identity | Planned |
| `CAP-012` | Operational visibility | Identity | Observed (`CLM-009`) |
| `CAP-013` | Quarantine and controlled replay | Identity | Observed (`CLM-010` authorization subset) |
| `CAP-014` | Traceability and recall | Traceability | Planned |
| `CAP-015` | Deterministic projection replay and checksum reconstruction | Traceability | Planned |
| `CAP-016` | Backup and restore | Traceability | Planned |
| `CAP-017` | Recovery and integrity verification | Traceability | Planned |
| `CAP-018` | Forecast datasets and features | Stockout | Planned |
| `CAP-019` | Stockout prediction | Stockout | Planned |
| `CAP-020` | Uncertainty and abstention | Stockout | Planned |
| `CAP-021` | Forecast evaluation and lineage | Stockout | Planned |
| `CAP-022` | Typed proposals and authority boundary | Approvals | Planned |
| `CAP-023` | Policy and human approval workflow | Approvals | Planned |
| `CAP-024` | Execution-time authorization and approval freshness | Approvals | Planned |
| `CAP-025` | Idempotent command execution | Approvals | Planned |
| `CAP-026` | Durable receipts and reconciliation | Approvals | Planned |
| `CAP-027` | Permission-aware retrieval | Governed RAG | Planned |
| `CAP-028` | Citations, refusals, and conflicting evidence | Governed RAG | Planned |
| `CAP-029` | Prompt-injection and retrieval-leakage defense | Governed RAG | Planned |
| `CAP-030` | Retrieval and answer evaluation | Governed RAG | Planned |
| `CAP-031` | Correlated telemetry | Reliability | Planned |
| `CAP-032` | SLI and SLO evidence | Reliability | Planned |
| `CAP-033` | Load and performance evidence | Reliability | Planned |
| `CAP-034` | Fault and degraded-mode campaigns | Reliability | Planned |
| `CAP-035` | Restore and recovery evidence | Reliability | Planned |
| `CAP-036` | Reproducible cloud deployment | Reliability | Planned |
| `CAP-037` | Cost evidence | Reliability | Planned |
| `CAP-038` | Deterministic public demo | Portfolio | Planned |
| `CAP-039` | Public release and evidence index | Portfolio | Planned |
| `CAP-040` | Evidence-bounded public claims | Portfolio | Planned |

## Related documents

- [Claim status vocabulary](docs/evidence/CLAIM_STATUS_VOCABULARY.md)
- [Event Spine experiment report](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md)
- [Identity & Operations experiment report](docs/evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md)
