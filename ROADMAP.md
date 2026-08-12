# SeshatOps Roadmap

**Status:** M0 is complete; M1 Event Spine is active with an executable event
contract, Northstar fixture (Issue #22), source/outbox persistence (Issue #23),
Redpanda publication (Issue #24), inventory projection consumer (Issue #25), and
consumer failure/restart safety (Issue #26). Service runtime and TypeScript view
remain later M1 work; M2–M8 are Planned.

This document is the concise, repository-owned implementation sequence for SeshatOps. It maps each major capability to exactly one primary milestone, records supported dependencies, and defines the governance rules for activating and completing milestones. It is a roadmap, not a task board and not evidence that future capabilities exist.

## Purpose and scope

SeshatOps is a clean-room, multi-tenant operations-intelligence platform built around the fictional Northstar Foods scenario. The intended product loop is:

> Observe → Reconstruct → Predict → Explain → Propose → Authorize → Approve → Execute → Audit → Replay

This roadmap covers the planned sequence from the M0 constitution through the M8 portfolio release. It does not select implementation packages, APIs, schemas, infrastructure products, model vendors, numerical targets, schedules, or staffing assumptions.

## Current state

| Milestone | Status | Repository state |
| --- | --- | --- |
| M0 — Project Constitution | Complete | Issues #1–#10 and PRs #11–#20 are complete. Notion remains `In Progress` until its normal workflow is updated; that external state is not silently changed here. |
| M1 — Event Spine | Active implementation | Issue #21 accepted the contract. Issues #22–#26 add event libraries, source/outbox, Redpanda relay, inventory projection consumer, and bounded consumer failure/restart safety. Service runtime and TypeScript view remain later M1 issues. |
| M2–M8 | Planned | Outcome-and-exit-gate placeholders only; no detailed future implementation backlog is maintained here. |

The repository now includes Go packages for the event contract, Northstar
fixture, `erp` source/outbox, `relay` publication, and `platform` projection
consumer. No service runtime, model, dataset experiment, deployment, or
production environment exists yet. Existing documents define intent and
reviewed truth; they do not prove observed production behavior, performance, or
production readiness.

## Source-of-truth boundaries

| System | Owns | Does not own |
| --- | --- | --- |
| Notion | Vision, product and architecture intent, milestone purpose, high-level risks, exit gates, and milestone summaries | Daily task state, duplicated GitHub checklists, or repository technical truth |
| GitHub milestones and issues | Active execution state, bounded deliverables, acceptance criteria, dependencies, and implementation progress | Long-form product strategy or repository evidence |
| Repository documents | Current reviewed technical truth, contracts, invariants, ADRs, runbooks, roadmap, and evidence ledger | Unreviewed brainstorming or duplicated daily task state |
| Pull requests and evidence artifacts | Actual changes, review discussion, verification performed, test/experiment artifacts, limitations, and claim support | Future-scope ideas or unsupported claims |

Ordinary issue status remains owned by GitHub. `ROADMAP.md` records milestone intent and sequencing; it must not become a second task tracker.

## Milestone sequence

| Milestone | Outcome | Primary capability ownership | Supported or prerequisite capabilities | Dependencies | Exit gate | Status |
| --- | --- | --- | --- | --- | --- | --- |
| **M0 — Project Constitution** | A reviewer understands what SeshatOps is, what it excludes, how it will be built, and what evidence is required before application code begins. | `CAP-001`–`CAP-003` | Supports governance, evidence, and clean-room review for every later milestone. | Master Project Blueprint and Issues #1–#10; no code or external service dependency. | Reviewed documentation is merged; boundaries, ownership, and exit gates are explicit; the M1 backlog can be created without inventing a new architecture. | **Complete** |
| **M1 — Event Spine** | A synthetic-ERP transaction travels through the transactional outbox, versioned event transport, Go projection, and TypeScript view. | `CAP-004`–`CAP-008` | Provides the event history, projections, and live view used by M2–M5 and the public demo. | M0 architecture, correctness, security, and evidence boundaries. | The vertical slice is duplicate-safe and unchanged event history reconstructs the expected projection deterministically. | **Active — failure/restart safety** |
| **M2 — Identity & Operations** | Users and service identities operate through default-deny tenant-aware boundaries with visibility into processing health and failures. | `CAP-009`–`CAP-013` | Extends M1 surfaces with identity, authorization, operational visibility, quarantine, and controlled replay. | M1 event and projection surfaces; M0 security model. | The future cross-tenant negative suite passes and privileged operations remain default-deny. | **Planned** |
| **M3 — Traceability & Recovery** | An authorized operator traces fictional operational lineage and demonstrates controlled reconstruction and recovery. | `CAP-014`–`CAP-017` | Reuses M1 event history/projections and M2 authorization and recovery controls. | M1 event spine; M2 operational authorization and controls. | The traceability query, deterministic projection checksum, and documented restore experiment succeed with reproducible evidence. | **Planned** |
| **M4 — Stockout Intelligence** | The platform produces evaluated stockout-risk intelligence with uncertainty and honest abstention. | `CAP-018`–`CAP-021` | Uses M1 reconstructed state and M2 tenant boundaries; supplies intelligence to M5 and M8. | M1 replayable operational state; M2 tenant-safe access; M0 evaluation protocols. | A frozen temporal evaluation is reproducible and every prediction is traceable to versioned inputs and evaluation artifacts. | **Planned** |
| **M5 — Approved Actions** | An authorized human approves a typed recommendation and retries produce one durable business effect with traceable receipts. | `CAP-022`–`CAP-026` | Uses M1 current state, M2 authorization, M4 recommendations, and M3 lineage/recovery context. | M1 operational state; M2 authorization; sequencing preference for M4 proposal output. | Authorized approval and repeated command delivery produce one durable business effect with traceable evidence. This is not an exactly-once network or broker-delivery claim. | **Planned** |
| **M6 — Governed RAG** | Authorized users receive evidence-backed explanations from fictional documents without cross-tenant leakage or model-owned authority. | `CAP-027`–`CAP-030` | Supports M4 explanations and M5 typed proposals; uses M2 identity and tenant boundaries. | M2 permission context; M0 governed-RAG protocol; sequencing preference for M4 outputs. | A frozen governed-RAG security and evaluation suite meets thresholds declared and justified during M6. Threshold values remain Planned in M0. | **Planned** |
| **M7 — Reliability & Cloud** | The platform is reproducibly deployed and reliability, recovery, performance, and cost claims are supported by repeatable artifacts. | `CAP-031`–`CAP-037` | Provides cross-cutting operational evidence for M1–M6 and release evidence for M8. | Implemented surfaces requiring telemetry, recovery, and deployment evidence; M3 recovery behavior is a prerequisite for restore campaigns. | Deployment is reproducible and repeated reliability evidence reports exist for the declared environment and conditions. | **Planned** |
| **M8 — Portfolio Release** | A reviewer can run the public project, understand its design, follow the demonstration, and verify every major public claim. | `CAP-038`–`CAP-040` | Integrates the supported product loop, evidence index, clean-room review, and public claim reconciliation. | Completed implementation milestones and applicable M7 evidence; all public dependencies remain fictional or public. | Every public claim links to evidence and the released repository runs without Ahoy or another private dependency. | **Planned** |

## Capability-to-primary-milestone matrix

Each capability has exactly one primary milestone. A supporting milestone may extend, consume, or verify the capability, but it never becomes a second primary owner. Capability status is `Planned` until later implementation and evidence justify a change.

| Capability ID | Capability | Primary milestone | Supporting milestones | Required evidence route | Status |
| --- | --- | --- | --- | --- | --- |
| `CAP-001` | Project constitution and clean-room governance | M0 | M8 | Constitution and clean-room review | Planned |
| `CAP-002` | Architecture, correctness, and security models | M0 | M1–M7 | Reviewed contracts, invariants, ADRs, and model reviews | Planned |
| `CAP-003` | Evidence and claim governance | M0 | M1–M8 | Roadmap/evidence review and claim-ledger records | Planned |
| `CAP-004` | Synthetic ERP and deterministic workload foundation | M1 | M3–M5, M8 | Deterministic workload and integration evidence | Planned |
| `CAP-005` | Transactional outbox | M1 | M7 | Contract/integration test and outage evidence | Planned |
| `CAP-006` | Versioned event contracts and transport | M1 | M3, M7 | Compatibility test and delivery trace | Planned |
| `CAP-007` | Deduplicated operational projections | M1 | M3, M7 | Duplicate-delivery test and projection artifact | Planned |
| `CAP-008` | TypeScript live operations view | M1 | M8 | Authorized live-view demonstration and test evidence | Planned |
| `CAP-009` | Identity and sessions | M2 | M5–M8 | Identity/session integration and negative tests | Planned |
| `CAP-010` | Tenant-aware authorization | M2 | M3, M5, M6, M7 | Cross-tenant negative suite | Planned |
| `CAP-011` | Service identities and least privilege | M2 | M5–M7 | Service-identity authorization tests and audit evidence | Planned |
| `CAP-012` | Operational visibility | M2 | M7 | Health, lag, failure, quarantine, and replay evidence | Planned |
| `CAP-013` | Quarantine and controlled replay operations | M2 | M3, M7 | Authorized fault/replay campaign | Planned |
| `CAP-014` | Traceability and recall | M3 | M5, M8 | Lineage query and recall report | Planned |
| `CAP-015` | Deterministic projection replay and checksum reconstruction | M3 | M1, M7 | Deterministic checksum comparison | Planned |
| `CAP-016` | Backup and restore | M3 | M7 | Restore experiment with application-readable and integrity checks | Planned |
| `CAP-017` | Recovery and integrity verification | M3 | M7 | Recovery report and integrity verification artifacts | Planned |
| `CAP-018` | Forecast datasets and features | M4 | M8 | Versioned dataset/feature manifest | Planned |
| `CAP-019` | Stockout prediction | M4 | M5, M8 | Temporal evaluation report | Planned |
| `CAP-020` | Uncertainty and abstention | M4 | M5, M6, M8 | Uncertainty/calibration and abstention evaluation | Planned |
| `CAP-021` | Forecast evaluation and model/data lineage | M4 | M7, M8 | Reproducible evaluation and lineage artifacts | Planned |
| `CAP-022` | Typed proposals and authority boundary | M5 | M4, M6, M8 | Proposal contract/security test | Planned |
| `CAP-023` | Policy and human approval workflow | M5 | M2, M6, M8 | Approval workflow and audit evidence | Planned |
| `CAP-024` | Execution-time authorization and approval freshness | M5 | M2, M7 | Negative/integration tests across display, approval, and execution | Planned |
| `CAP-025` | Idempotent command execution | M5 | M7 | Retry/concurrency fault campaign | Planned |
| `CAP-026` | Durable receipts and uncertain-outcome reconciliation | M5 | M3, M7 | Receipt and reconciliation report | Planned |
| `CAP-027` | Permission-aware retrieval | M6 | M2, M8 | Governed retrieval evaluation | Planned |
| `CAP-028` | Citations, refusals, and conflicting evidence | M6 | M4, M5, M8 | Citation/refusal/conflict evaluation report | Planned |
| `CAP-029` | Prompt-injection and retrieval-leakage defense | M6 | M2, M7, M8 | Adversarial and cross-tenant negative suite | Planned |
| `CAP-030` | Retrieval and answer evaluation | M6 | M7, M8 | Frozen governed-RAG evaluation report | Planned |
| `CAP-031` | Correlated telemetry and distributed tracing | M7 | M1–M6 | Trace bundle with correlated metrics and logs | Planned |
| `CAP-032` | SLI and SLO evidence | M7 | M1–M6 | Environment-scoped SLO evidence report | Planned |
| `CAP-033` | Load and performance evidence | M7 | M1–M6 | Distribution-aware performance report | Planned |
| `CAP-034` | Fault and degraded-mode campaigns | M7 | M1–M6 | Fault-campaign report and raw artifacts | Planned |
| `CAP-035` | Restore and recovery evidence campaigns | M7 | M3 | Restore/recovery campaign report | Planned |
| `CAP-036` | Reproducible cloud deployment | M7 | M8 | Deployment reproduction record | Planned |
| `CAP-037` | Cost evidence | M7 | M8 | Declared-environment cost report | Planned |
| `CAP-038` | Deterministic public demo | M8 | M1–M7 | Demonstration script and release artifact | Planned |
| `CAP-039` | Public release and evidence index | M8 | M0, M7 | Release review and evidence index | Planned |
| `CAP-040` | Evidence-bounded README, CV, and portfolio claims | M8 | M0, M7 | Public-claim reconciliation review | Planned |

## Dependencies and sequencing

Dependencies are recorded only where supported by the product constitution, architecture, correctness model, security model, and evidence protocols. No dates, durations, staffing assumptions, or delivery commitments are implied.

### Hard implementation dependencies

- M0 precedes M1 because the event spine must follow the reviewed architecture, correctness, security, and evidence boundaries.
- M1 precedes M2 because identity and operations govern event-processing and projection surfaces.
- M1 precedes M3 because traceability and recovery reuse the event history and projections.
- M1 precedes M4 because forecasting features derive from replayable operational state.
- M1 and M2 precede M5 because approved actions require current state and tenant-aware authorization.
- M2 precedes M6 because retrieval and citation must receive an authoritative permission context.
- M3 precedes the M7 restore/recovery campaign because recovery behavior must exist before it can be evidenced.
- Applicable implementation and M7 evidence precede M8 release claims.

### Sequencing preferences

- M4 output should be available before the complete M5 proposal workflow is demonstrated.
- M5 and M6 should be integrated into the M8 demonstration only after their authority and evidence boundaries are reviewed.
- M2 operational controls should be available before M3 recovery workflows are exposed to operators.

### Cross-cutting evidence dependencies

- M0 defines the status, clean-room, source-of-truth, and claim-promotion rules used by every later evidence record.
- M2 authorization evidence supports tenant and privileged-operation claims in M3–M6.
- M7 supplies operational, restore, performance, deployment, and cost evidence for earlier capabilities; this does not change their primary ownership.
- M8 reconciles public wording against `EVIDENCE.md` and retains superseded claims and evidence.

## Active-milestone decomposition rule

M1 is the active milestone. Issue #21 accepted the event-spine contract,
Issue #22 adds the executable event and Northstar fixture libraries, and
Issue #23 adds the synthetic ERP source transaction with pending outbox
persistence. Detailed GitHub issues remain bounded to the active milestone.
M2–M8 remain outcome-and-exit-gate placeholders and must not accumulate
speculative implementation backlogs.

When a milestone begins:

1. Review its Notion page and the current repository truth.
2. Confirm purpose, user-visible outcome, invariants, trust boundaries, non-goals, dependencies, and required exit evidence.
3. Create only bounded GitHub issues for that active milestone.
4. Give each issue one observable outcome, acceptance criteria, failure/security cases, dependencies, evidence requirements, and non-goals.
5. Link the milestone-level intent to the repository plan without duplicating daily issue status.

## Milestone activation rules

- A milestone remains Planned until its dependencies, outcome, exit gate, and evidence route are reviewed.
- The Notion milestone page is reviewed before GitHub issues are created.
- A milestone becomes active only when its first bounded issue begins in GitHub.
- Future milestones may have only placeholder purpose, outcome, dependencies, and exit-gate information before activation.
- GitHub owns issue and milestone execution state; this file records the stable sequence and governance rules.

## Milestone completion rules

A milestone may be marked complete only when:

- Its user-visible outcome works end to end where applicable.
- Required acceptance criteria and relevant correctness, failure, and security cases pass.
- Canonical documentation matches the reviewed implementation.
- Required evidence artifacts exist, are linked to stable claim IDs, and include limitations.
- No README, CV, demo, or portfolio claim exceeds the ledger.
- Important decisions have ADRs where required.
- The deterministic demonstration path and repository checks applicable to the milestone are available.
- Residual risks and deferred work are recorded.
- The milestone summary is published through the Notion/GitHub workflow.

Documentation existence alone cannot satisfy a future runtime milestone exit gate.

## Scope-change and contradiction handling

- A scope change must identify the affected capability IDs, primary milestone, supporting milestones, claims, dependencies, and evidence routes.
- A capability may have only one primary owner. If ownership changes, the roadmap records the old and new owner and preserves claim traceability.
- Contradictions are recorded with the sources, impact, disposition, and owner before implementation scope changes.
- A later implementation detail must not silently rewrite a product, security, correctness, or evidence invariant.
- Scope changes that introduce a schedule, target, vendor, model, infrastructure product, or production claim require the owning milestone’s explicit decision and evidence route.
- Superseded claims and evidence remain in the ledger; they are never silently deleted or reused.

## Non-goals and deferred decisions

This roadmap does not:

- Create or modify GitHub milestones.
- Create detailed M1–M8 issues.
- Add application code, runtime scaffolding, infrastructure, CI, schemas, dashboards, or scripts.
- Select identity providers, policy engines, databases, brokers, model vendors, vector stores, cloud products, or deployment sizes.
- Invent schedules, throughput, latency, accuracy, availability, SLO, RPO, RTO, cost, security, or evaluation targets.
- Claim exactly-once network or broker delivery, production readiness, or measured performance.
- Use Ahoy or any private system as a dependency, evidence source, or design input.

Deferred decisions are owned by the relevant milestone: completed Issue #9 established repository workflow and documentation CI; Issue #10 owns integrated M0 review; M1 owns runtime event implementation; M2 owns identity and operational enforcement; M3 owns restore behavior; M4 owns forecasting evaluation decisions; M5 owns command and reconciliation implementation; M6 owns retrieval/evaluation implementation; M7 owns deployment and operational evidence decisions; and M8 owns public release and final claim reconciliation.

## Related documents

- [PRODUCT.md](PRODUCT.md)
- [CLEAN_ROOM.md](CLEAN_ROOM.md)
- [ARCHITECTURE.md](ARCHITECTURE.md)
- [EVENT_MODEL.md](docs/architecture/EVENT_MODEL.md)
- [CONTRACTS.md](CONTRACTS.md)
- [COMMAND_MODEL.md](docs/architecture/COMMAND_MODEL.md)
- [CLAIM_STATUS_VOCABULARY.md](docs/evidence/CLAIM_STATUS_VOCABULARY.md)
- [EVIDENCE.md](EVIDENCE.md)
