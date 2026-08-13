# SeshatOps Evidence Ledger

**Status:** Active ledger. `CLM-003`–`CLM-006` are `Observed` for the Issue #30
test-environment event-spine exit-gate campaign. All other claims remain `Planned`.

This document is the canonical repository-owned claim ledger for SeshatOps. It records what a claim means, which capability and milestone own it, how it must be verified, and what limitations apply. The existence of this ledger or any other documentation does not prove production readiness.

## Claim-status vocabulary

The canonical statuses are defined in [docs/evidence/CLAIM_STATUS_VOCABULARY.md](docs/evidence/CLAIM_STATUS_VOCABULARY.md). This ledger references that vocabulary and does not redefine it:

`Planned` · `Implemented` · `Observed` · `Reproduced` · `Superseded`

`Not evaluated`, `Not executed`, and `TBD — evidence required` are record or result dispositions used by protocols and templates; they are not additional claim statuses.

## Ledger field definitions

| Field | Meaning and control |
| --- | --- |
| Claim ID | Stable identifier such as `CLM-001`; never silently reused. |
| Claim | The bounded technical or portfolio statement that may eventually be made. |
| Status | One canonical claim status from the vocabulary. |
| Capability ID | Exactly one stable capability ID from `ROADMAP.md`. |
| Owning capability sequence | The capability sequence primarily responsible for the capability and claim boundary. |
| Verification method | The future test, experiment, review, reproduction, or demonstration that can support the claim. |
| Required artifact (Artifact) | The artifact category required before promotion, without inventing a future filename. |
| Evidence artifact | The actual evidence reference after it exists; `—` means none exists. |
| Environment class (Environment) | Declared evidence class from the canonical vocabulary; `Planned` means not evaluated. |
| Repository commit or release (Commit/release) | Exact repository state supporting the evidence; `—` means none exists. |
| Observation or decision date (Date) | Date of the recorded observation or claim decision; `—` means none exists. |
| Reviewer | Person or role that reviewed the evidence and status decision. |
| Limitations | Conditions, exclusions, and uncertainties that bound the claim. |
| Supersedes | Prior claim or evidence record replaced by this record, if any. |
| Superseded by | Later claim or evidence record replacing this record, if any. |
| Notes/follow-up | Required remediation, reproduction, withdrawal, or owner follow-up. |

## Stable claim-ID rules

- Claim IDs are unique within the ledger and stable across revisions.
- Rewording preserves an ID when the meaning and evidence boundary remain materially unchanged.
- A materially different claim receives a new ID.
- IDs are never silently reused after withdrawal or supersession.
- Superseded claims and evidence remain traceable.
- Pull requests, reports, experiments, reviews, and release notes reference claim IDs.
- Claim promotion is blocked when the stable ID or required evidence route is missing.

## Initial Planned claims

Rows below start from the Issue #8 Planned ledger. `CLM-003`–`CLM-006` were
promoted to `Observed` by Issue #30 with linked experiment evidence. Other rows
remain intentionally `Planned` without evidence artifacts.

| Claim ID | Claim | Status | Capability ID | Owning capability sequence | Verification method | Required artifact (Artifact) | Evidence artifact | Environment | Commit/release | Date | Reviewer | Limitations | Supersedes | Superseded by | Notes/follow-up |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `CLM-001` | Project Constitution integrated exit gate is satisfied. | Planned | `CAP-003` | Constitution | Documentation and integrated constitution review | Review record | — | Planned | — | — | — | Documentation review cannot prove future runtime capabilities. | — | — | Issue #10 remains the integrated review owner. |
| `CLM-002` | Clean-room governance keeps public artifacts independent from Ahoy. | Planned | `CAP-001` | Constitution | Clean-room review and category search | Clean-room review record | — | Planned | — | — | — | Policy and review do not prove that every future artifact is clean. | — | — | Portfolio Release review must repeat the check. |
| `CLM-003` | Transactional outbox handling preserves accepted source intent during broker outage. | Observed | `CAP-005` | Event Spine | Outbox integration test and outage fault campaign | Test report and raw delivery trace | [EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md); FC-007 | Test environment | `a4e5d47` (runtime); PR #41 `b59b760` | 2026-08-12 | G1DO (exit-gate suites + hosted CI) | Single-host Testcontainers Redpanda stop/start only; not staging/production outage or capacity evidence. | — | — | Issue #30 EXP-M1-EXIT-GATE-001. Hosted CI on `b59b760`: Go [31588310097](https://github.com/G1DO/seshatops/actions/runs/31588310097), Web [31588310148](https://github.com/G1DO/seshatops/actions/runs/31588310148), Docs [31588310157](https://github.com/G1DO/seshatops/actions/runs/31588310157). |
| `CLM-004` | Event processing is duplicate-safe. | Observed | `CAP-007` | Event Spine | Duplicate-delivery integration test and fault report | Test report and deduplication trace | [EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md); FC-001/012/013 | Test environment | `a4e5d47` (runtime); PR #41 `b59b760` | 2026-08-12 | G1DO (exit-gate suites + hosted CI) | One durable inventory-projection effect under tested duplicates/crash windows; **not** exactly-once delivery or multi-node evidence. | — | — | Must remain distinct from exactly-once delivery claims. |
| `CLM-005` | Versioned event contracts and ordering compatibility are enforced. | Observed | `CAP-006` | Event Spine | Contract and compatibility tests | Compatibility report and event trace | [EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md); FC-009/010/011 | Test environment | `a4e5d47` (runtime); PR #41 `b59b760` | 2026-08-12 | G1DO (exit-gate suites + hosted CI) | Exact schema v1 + Event Spine gap/stale/reorder/poison dispositions only; not a general migration or operator quarantine product. | — | — | Unsupported versions and ordering gaps require explicit dispositions. |
| `CLM-006` | Unchanged history deterministically rebuilds projections. | Observed | `CAP-007` | Event Spine | Deterministic projection rebuild test and expected-state comparison | Projection rebuild report and expected-state artifact | [EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md); FC-014 | Test environment | `a4e5d47` (runtime); PR #41 `b59b760` | 2026-08-12 | G1DO (exit-gate suites + hosted CI) | Platform `ResetDerivedState`/`RebuildFromHistory` A==B for complete retained history; not Traceability & Recovery backup/restore (ADR-Q-005). | — | — | Traceability & Recovery requires a separate claim for authorized recovery replay/restore. |
| `CLM-007` | Tenant isolation holds across protected operations. | Planned | `CAP-010` | Identity | Cross-tenant negative suite | Security test report and raw denial results | — | Planned | — | — | — | Issue #46 adds inventory query-API default-deny in library/test scope (`MX-001`). Issue #49 adds tenant-scoped privileged-audit read (`MX-007`) in library/test scope. Isolation is not claimed for retrieval, citations, approvals, commands, exports, replay, or recovery. | — | — | Scope must include reads, retrieval, citations, approvals, commands, exports, audit, replay, and recovery. |
| `CLM-008` | Privileged operations are default-deny. | Planned | `CAP-010` | Identity | Privileged-operation negative suite | Security test report and audit evidence | — | Planned | — | — | — | Issue #48 adds privileged HTTP controls in library/test scope (`MX-004`/`MX-005`/`MX-006`) with default-deny negatives. Issue #49 adds append-only privileged-decision persistence and `MX-007` read in library/test scope. Issue #50 exit-gate suite is not claimed. Claim is not promoted. | — | — | Missing, stale, ambiguous, or unauthorized context must fail closed. |
| `CLM-009` | Operational health, lag, and failure visibility is available. | Planned | `CAP-012` | Identity | Operational visibility integration test | Operational report and trace bundle | — | Planned | — | — | — | Event Spine keeps library-only `relay.InspectBacklog` and `platform.InspectProcessing` for verification. Issue #47 adds authorized tenant-scoped `GET /v1/tenants/{tenant_id}/ops` in library/test scope (`MX-002`/`MX-003`). This is not an SLO/alerting platform. Claim is not promoted. | — | — | Visibility does not itself establish availability or SLO compliance. |
| `CLM-010` | Quarantine and controlled replay are safe and authorized. | Planned | `CAP-013` | Identity | Quarantine/replay fault campaign and authorization tests | Fault report, quarantine record, and audit trace | — | Planned | — | — | — | Issue #48 adds authorized quarantine-release/replay/rebuild POSTs in library/test scope. Issue #49 adds append-only privileged-decision audit in library/test scope. Hosted fault campaign, Issue #50 exit-gate suite, and claim promotion are not recorded. Global `ResetDerivedState` is not exposed. | — | — | Replay must not repeat irreversible external effects. |
| `CLM-011` | Fictional operational lineage and recall are traceable. | Planned | `CAP-014` | Traceability | Authorized lineage query and recall scenario | Traceability report and lineage artifact | — | Planned | — | — | — | No synthetic ERP lineage or query implementation exists. | — | — | Scope remains the fictional Northstar Foods domain. |
| `CLM-012` | Backup restoration preserves readable, bounded, integrity-checked state. | Planned | `CAP-016` | Traceability | Restore experiment with application-readable and integrity checks | Restore report and integrity results | — | Planned | — | — | — | No backup, restore target, or integrity checker exists. | — | — | No RPO or RTO target is claimed. |
| `CLM-013` | Stockout evaluation is reproducible over a frozen temporal split. | Planned | `CAP-021` | Stockout | Temporal evaluation report and independent reproduction | Evaluation report and reproduction manifest | — | Planned | — | — | — | No dataset, feature view, evaluator, or model exists. | — | — | Thresholds and metric applicability remain undecided. |
| `CLM-014` | Forecasts expose uncertainty and abstain when appropriate. | Planned | `CAP-020` | Stockout | Uncertainty, calibration, and abstention evaluation | Forecast evaluation report | — | Planned | — | — | — | No forecast output or abstention implementation exists. | — | — | No accuracy, calibration, or coverage target is invented. |
| `CLM-015` | Dataset, feature, model, and evaluator lineage is reproducible. | Planned | `CAP-021` | Stockout | Lineage-manifest review and evaluation reproduction | Versioned lineage manifest and evaluation report | — | Planned | — | — | — | No data, model, evaluator, or artifact registry exists. | — | — | Provenance and clean-room independence must be recorded for each future run. |
| `CLM-016` | Typed proposals cannot directly authorize or execute business changes. | Planned | `CAP-022` | Approvals | Proposal contract and authority-boundary security test | Proposal test report and audit trace | — | Planned | — | — | — | No proposal or command implementation exists. | — | — | Python and retrieved content remain advisory and non-authoritative. |
| `CLM-017` | Authorization and approval freshness are rechecked at execution. | Planned | `CAP-024` | Approvals | Display/approval/execution negative and integration suite | Security test report and decision timeline | — | Planned | — | — | — | No workflow or execution runtime exists. | — | — | Changed intent, target, role, scope, session, policy, or state must invalidate execution. |
| `CLM-018` | Equivalent command retries produce one durable business effect. | Planned | `CAP-025` | Approvals | Retry and concurrency fault campaign | Command report, effect lineage, and receipt | — | Planned | — | — | — | No command path or external adapter exists. | — | — | This is not an exactly-once network or broker-delivery claim. |
| `CLM-019` | Durable receipts distinguish confirmed, failed, and uncertain outcomes and support reconciliation. | Planned | `CAP-026` | Approvals | Timeout, partial-failure, and reconciliation campaign | Receipt/reconciliation report and audit trace | — | Planned | — | — | — | No receipt or external reconciliation implementation exists. | — | — | External outcome confirmation remains adapter-specific. |
| `CLM-020` | Retrieval is permission-aware. | Planned | `CAP-027` | Governed RAG | Governed retrieval security evaluation | Retrieval evaluation report and authorized-result trace | — | Planned | — | — | — | No index, corpus, or retrieval implementation exists. | — | — | Authorization must be applied before retrieved content becomes model context. |
| `CLM-021` | Citations are authorized and supported by the returned evidence. | Planned | `CAP-028` | Governed RAG | Citation authorization, freshness, and support evaluation | Governed-RAG evaluation report and citation artifact | — | Planned | — | — | — | No corpus, citation validator, or result exists. | — | — | Screenshots or unsupported citations cannot promote the claim. |
| `CLM-022` | Governed-RAG retrieval and answer evaluation is reproducible. | Planned | `CAP-030` | Governed RAG | Frozen evaluation suite and independent reproduction | Governed-RAG evaluation report and reproduction manifest | — | Planned | — | — | — | No RAG system, evaluation corpus, or thresholds exist. | — | — | Threshold values must be declared and justified during Governed RAG. |
| `CLM-023` | Prompt-injection resistance is evaluated. | Planned | `CAP-029` | Governed RAG | Adversarial prompt-injection evaluation | Security/evaluation report and raw cases | — | Planned | — | — | — | No prompt, retrieval, or proposal runtime exists. | — | — | Retrieved content cannot become an executable instruction or authority source. |
| `CLM-024` | Cross-tenant retrieval leakage is prevented. | Planned | `CAP-029` | Governed RAG | Cross-tenant retrieval, cache, and index negative suite | Security test report and raw denial/leakage results | — | Planned | — | — | — | No tenant-aware index or cache exists. | — | — | A single local demonstration cannot support this claim. |
| `CLM-025` | Governed-RAG refusal behavior is evaluated for unsafe, unsupported, stale, or inaccessible requests. | Planned | `CAP-028` | Governed RAG | Refusal and abstention evaluation | Governed-RAG evaluation report | — | Planned | — | — | — | No refusal or abstention behavior exists. | — | — | Refusal quality remains scoped to declared cases and conditions. |
| `CLM-026` | Telemetry is correlated across traces, metrics, and logs. | Planned | `CAP-031` | Reliability | Traceability integration test and trace-bundle review | Trace bundle and telemetry completeness report | — | Planned | — | — | — | No telemetry pipeline or runtime exists. | — | — | Correlation does not itself establish an SLO or production claim. |
| `CLM-027` | SLO evidence is reproducible and environment-scoped. | Planned | `CAP-032` | Reliability | SLI/SLO evidence review and reproduction | SLO evidence report and query/configuration record | — | Planned | — | — | — | No SLI, SLO, telemetry window, or environment exists. | — | — | No SLO target or availability result is invented. |
| `CLM-028` | Load and tail-latency evidence is reproducible and distribution-aware. | Planned | `CAP-033` | Reliability | Controlled load experiment and independent reproduction | Performance report, raw results, and workload manifest | — | Planned | — | — | — | No workload, topology, measurement system, or target exists. | — | — | Averages alone cannot support latency claims. |
| `CLM-029` | Fault and degraded-mode evidence is reproducible. | Planned | `CAP-034` | Reliability | Fault campaign with safety, integrity, and recovery checks | Fault-campaign report and raw artifacts | — | Planned | — | — | — | No runtime or fault-injection environment exists. | — | — | Results must distinguish unavailable, degraded, recovered, and uncertain states. |
| `CLM-030` | Restore and recovery campaigns produce scoped evidence. | Planned | `CAP-035` | Reliability | Restore/recovery campaign and integrity review | Restore/recovery report and integrity artifacts | — | Planned | — | — | — | No backup system, restore environment, or recovery procedure exists. | — | — | A backup artifact alone is not evidence of restorability. |
| `CLM-031` | Cloud deployment is reproducible for the declared environment. | Planned | `CAP-036` | Reliability | Deployment reproduction | Deployment reproduction record and release artifact | — | Planned | — | — | — | No deployment configuration or cloud environment exists. | — | — | No provider, size, topology, or production claim is selected in Project Constitution. |
| `CLM-032` | Cost evidence is scoped to a declared environment and conditions. | Planned | `CAP-037` | Reliability | Cost measurement and evidence review | Cost report and environment manifest | — | Planned | — | — | — | No deployment, workload, billing context, or cost measurement exists. | — | — | No cost target or generalization beyond the declared environment is claimed. |
| `CLM-033` | The public demonstration is deterministic. | Planned | `CAP-038` | Portfolio | Demonstration script and repeatability review | Demonstration script, release artifact, and run record | — | Planned | — | — | — | No application or demo path exists. | — | — | A screenshot alone is insufficient evidence. |
| `CLM-034` | The released repository runs independently of Ahoy. | Planned | `CAP-039` | Portfolio | Clean-room release review and public reproduction | Release artifact, reproduction record, and clean-room review | — | Planned | — | — | — | No release or runtime exists. | — | — | Project Constitution policy is not proof of the final repository state. |
| `CLM-035` | README, CV, demo, and portfolio claims are evidence-backed. | Planned | `CAP-040` | Portfolio | Public-claim reconciliation review | Release review and claim-to-evidence index | — | Planned | — | — | — | Current public wording remains bounded by the initial Planned state. | — | — | Promotion requires stable claim IDs and linked supporting evidence. |

## Claim promotion, withdrawal, and supersession

- `Planned` means intended; it does not imply implementation, observation, reproduction, or readiness.
- `Implemented` may state that a reviewed implementation exists; it does not establish correct behavior or measured quality.
- `Observed` applies only to the exact commit, environment, configuration, workload, data, duration, and conditions recorded in the evidence artifact.
- `Reproduced` requires the documented reproduction context and materially consistent results; it does not broaden the claim beyond the recorded scope.
- `Superseded` retires a claim or evidence record as the current basis for decision-making without deleting its history.
- A status-changing pull request must reference the stable claim ID and supporting evidence.
- Missing reproducibility context blocks promotion.
- Screenshots, README wording, averages, design documents, and dependency-reported success are insufficient by themselves.
- Evidence that becomes invalid, incomplete, contradicted, or out of scope must be withdrawn, demoted, or superseded.

## Public-claim guardrails

- README, CV, portfolio, demo, release-note, and interview language may not exceed this ledger.
- Planned claims use future or design language.
- Implemented claims state existence, not measured quality.
- Observed claims state the environment and limitations.
- Reproduced claims link the reproduction record.
- Production claims require production-class evidence and explicit review.
- Local demonstrations cannot support distributed, recovery, capacity, production-security, availability, or production-performance claims.
- The Portfolio Release review reconciles every public claim against this ledger.

## Evidence record expectations

Future evidence records must identify the claim IDs, prior and new status, repository state, environment class, topology, operating system/runtime, tools and dependencies, configuration, data or workload provenance, seed or reason not applicable, exact commands, timestamps, operator, reviewer, raw artifacts, checksums where relevant, results, anomalies, limitations, reproduction instructions, and claim-status decision.

No planned filename is treated as an evidence artifact. A required artifact becomes evidence only when the actual record exists, is linked, and is reviewed.

## Related documents

- [ROADMAP.md](ROADMAP.md)
- [Claim Status Vocabulary](docs/evidence/CLAIM_STATUS_VOCABULARY.md)
- [Security Evidence Protocol](docs/evaluation/SECURITY_EVIDENCE_PROTOCOL.md)
- [Reliability and Recovery Evidence Protocol](docs/evaluation/RELIABILITY_RECOVERY_EVIDENCE_PROTOCOL.md)
- [Performance Evidence Protocol](docs/evaluation/PERFORMANCE_EVIDENCE_PROTOCOL.md)
- [Fault Campaign Matrix](docs/evaluation/FAULT_CAMPAIGN_MATRIX.md)
- [Experiment Report Template](docs/evaluation/templates/EXPERIMENT_REPORT.md)
