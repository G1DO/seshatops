# SeshatOps Roadmap

**Status:** Project Constitution is complete; Event Spine exit-gate evidence is recorded for
the Issue #30 test-environment campaign (`CLM-003`–`CLM-006` Observed).
Identity & Operations exit-gate evidence is recorded for the Issue #50
test-environment campaign (`CLM-007`–`CLM-010` Observed). Traceability & Recovery
through Portfolio Release remain Planned.

This document is the concise, repository-owned implementation sequence for SeshatOps. It maps each major capability to exactly one primary capability owner, records supported dependencies, and defines the governance rules for activating and completing capability sequences. It is a roadmap, not a task board and not evidence that future capabilities exist.

## Purpose and scope

SeshatOps is a clean-room, multi-tenant operations-intelligence platform built around the fictional Northstar Foods scenario. The intended product loop is:

> Observe → Reconstruct → Predict → Explain → Propose → Authorize → Approve → Execute → Audit → Replay

This roadmap covers the planned sequence from the Project Constitution through Portfolio Release. It does not select implementation packages, APIs, schemas, infrastructure products, model vendors, numerical targets, schedules, or staffing assumptions.

## Current state

| Capability sequence | Status | Repository state |
| --- | --- | --- |
| Project Constitution | Complete | Issues #1–#10 and PRs #11–#20 are complete. GitHub [milestone/1](https://github.com/G1DO/seshatops/milestone/1) and the Notion Project Constitution page are closed/Complete as of 2026-08-12. |
| Event Spine | Complete (test-environment exit gate) | Issues #21–#30 delivered the stack and exit-gate campaign (`CLM-003`–`CLM-006` Observed). GitHub [milestone/2](https://github.com/G1DO/seshatops/milestone/2) and the Notion Event Spine page are closed/Complete as of 2026-08-12. No production deployment. |
| Identity & Operations | Complete (test-environment exit gate, except `CAP-011`) | Issues #43–#50 delivered identity, default-deny HTTP, ops visibility, privileged controls, audit, and the exit-gate negative suite (`CLM-007`–`CLM-010` Observed). `CAP-011` remains Planned. No production deployment. |
| Traceability & Recovery through Portfolio Release | Planned | Outcome-and-exit-gate placeholders only; no detailed future implementation backlog is maintained here. Next activation: Traceability & Recovery. |

The repository now includes Go packages for the event contract, Northstar
fixture, `erp` source/outbox, `relay` publication, `platform` projection
consumer (including Issue #29 rebuild helpers), `api` read surface, `identity/`
session and policy, and the `web/` TypeScript operations view. No long-running
deployment service, model, dataset experiment, or production environment exists
yet. Existing documents define intent and
reviewed truth; they do not prove observed production behavior, performance, or
production readiness.

## Source-of-truth boundaries

| System | Owns | Does not own |
| --- | --- | --- |
| Notion | Vision, product and architecture intent, capability-sequence purpose, high-level risks, exit gates, and capability-sequence summaries | Daily task state, duplicated GitHub checklists, or repository technical truth |
| GitHub milestones and issues | Active execution state, bounded deliverables, acceptance criteria, dependencies, and implementation progress | Long-form product strategy or repository evidence |
| Repository documents | Current reviewed technical truth, contracts, invariants, ADRs, runbooks, roadmap, and evidence ledger | Unreviewed brainstorming or duplicated daily task state |
| Pull requests and evidence artifacts | Actual changes, review discussion, verification performed, test/experiment artifacts, limitations, and claim support | Future-scope ideas or unsupported claims |

Ordinary issue status remains owned by GitHub. `ROADMAP.md` records capability-sequence intent and sequencing; it must not become a second task tracker.

## Capability sequence

| Capability sequence | Outcome | Primary capability ownership | Supported or prerequisite capabilities | Dependencies | Exit gate | Status |
| --- | --- | --- | --- | --- | --- | --- |
| **Project Constitution** | A reviewer understands what SeshatOps is, what it excludes, how it will be built, and what evidence is required before application code begins. | `CAP-001`–`CAP-003` | Supports governance, evidence, and clean-room review for every later capability sequence. | Master Project Blueprint and Issues #1–#10; no code or external service dependency. | Reviewed documentation is merged; boundaries, ownership, and exit gates are explicit; the Event Spine backlog can be created without inventing a new architecture. | **Complete** |
| **Event Spine** | A synthetic-ERP transaction travels through the transactional outbox, versioned event transport, Go projection, and TypeScript view. | `CAP-004`–`CAP-008` | Provides the event history, projections, and live view used by Identity & Operations through Approved Actions and the public demo. | Project Constitution architecture, correctness, security, and evidence boundaries. | The vertical slice is duplicate-safe and unchanged event history reconstructs the expected projection deterministically. | **Complete — test-environment exit gate (Issue #30)** |
| **Identity & Operations** | Users operate through default-deny tenant-aware HTTP boundaries with visibility into processing health and failures. Service-identity credentials remain Planned (`CAP-011`). | `CAP-009`–`CAP-013` | Extends Event Spine surfaces with identity, authorization, operational visibility, quarantine, and controlled replay. | Event Spine event and projection surfaces; Project Constitution security model. | The cross-tenant negative suite passes and privileged operations remain default-deny on implemented HTTP surfaces. | **Complete — test-environment exit gate (Issue #50), except `CAP-011`** |
| **Traceability & Recovery** | An authorized operator traces fictional operational lineage and demonstrates controlled reconstruction and recovery. | `CAP-014`–`CAP-017` | Reuses Event Spine event history/projections and Identity & Operations authorization and recovery controls. | Event Spine; Identity & Operations authorization and controls. | The traceability query, deterministic projection checksum, and documented restore experiment succeed with reproducible evidence. | **Planned** |
| **Stockout Intelligence** | The platform produces evaluated stockout-risk intelligence with uncertainty and honest abstention. | `CAP-018`–`CAP-021` | Uses Event Spine reconstructed state and Identity & Operations tenant boundaries; supplies intelligence to Approved Actions and Portfolio Release. | Event Spine replayable operational state; Identity & Operations tenant-safe access; Project Constitution evaluation protocols. | A frozen temporal evaluation is reproducible and every prediction is traceable to versioned inputs and evaluation artifacts. | **Planned** |
| **Approved Actions** | An authorized human approves a typed recommendation and retries produce one durable business effect with traceable receipts. | `CAP-022`–`CAP-026` | Uses Event Spine current state, Identity & Operations authorization, Stockout Intelligence recommendations, and Traceability & Recovery lineage/recovery context. | Event Spine operational state; Identity & Operations authorization; sequencing preference for Stockout Intelligence proposal output. | Authorized approval and repeated command delivery produce one durable business effect with traceable evidence. This is not an exactly-once network or broker-delivery claim. | **Planned** |
| **Governed RAG** | Authorized users receive evidence-backed explanations from fictional documents without cross-tenant leakage or model-owned authority. | `CAP-027`–`CAP-030` | Supports Stockout Intelligence explanations and Approved Actions typed proposals; uses Identity & Operations and tenant boundaries. | Identity & Operations permission context; Project Constitution governed-RAG protocol; sequencing preference for Stockout Intelligence outputs. | A frozen governed-RAG security and evaluation suite meets thresholds declared and justified during Governed RAG. Threshold values remain Planned from Project Constitution. | **Planned** |
| **Reliability & Cloud** | The platform is reproducibly deployed and reliability, recovery, performance, and cost claims are supported by repeatable artifacts. | `CAP-031`–`CAP-037` | Provides cross-cutting operational evidence for Event Spine through Governed RAG and release evidence for Portfolio Release. | Implemented surfaces requiring telemetry, recovery, and deployment evidence; Traceability & Recovery behavior is a prerequisite for restore campaigns. | Deployment is reproducible and repeated reliability evidence reports exist for the declared environment and conditions. | **Planned** |
| **Portfolio Release** | A reviewer can run the public project, understand its design, follow the demonstration, and verify every major public claim. | `CAP-038`–`CAP-040` | Integrates the supported product loop, evidence index, clean-room review, and public claim reconciliation. | Completed implementation capability sequences and applicable Reliability & Cloud evidence; all public dependencies remain fictional or public. | Every public claim links to evidence and the released repository runs without Ahoy or another private dependency. | **Planned** |

## Capability-to-primary-owner matrix

Each capability has exactly one primary capability owner. A supporting owner may extend, consume, or verify the capability, but it never becomes a second primary owner. Capability status is `Planned` until later implementation and evidence justify a change.

| Capability ID | Capability | Primary capability owner | Supporting owners | Required evidence route | Status |
| --- | --- | --- | --- | --- | --- |
| `CAP-001` | Project constitution and clean-room governance | Constitution | Portfolio | Constitution and clean-room review | Planned |
| `CAP-002` | Architecture, correctness, and security models | Constitution | Event Spine through Reliability & Cloud | Reviewed contracts, invariants, ADRs, and model reviews | Planned |
| `CAP-003` | Evidence and claim governance | Constitution | Event Spine through Portfolio Release | Roadmap/evidence review and claim-ledger records | Planned |
| `CAP-004` | Synthetic ERP and deterministic workload foundation | Event Spine | Traceability & Recovery through Approved Actions, Portfolio | Deterministic workload and integration evidence | Observed (test env; via CLM-003–006 campaign) |
| `CAP-005` | Transactional outbox | Event Spine | Reliability | Contract/integration test and outage evidence | Observed (test env; CLM-003) |
| `CAP-006` | Versioned event contracts and transport | Event Spine | Traceability, Reliability | Compatibility test and delivery trace | Observed (test env; CLM-005) |
| `CAP-007` | Deduplicated operational projections | Event Spine | Traceability, Reliability | Duplicate-delivery test and projection artifact | Observed (test env; CLM-004/006) |
| `CAP-008` | TypeScript live operations view | Event Spine | Portfolio | Authorized live-view demonstration and test evidence | Planned (implemented UI + Vitest/API reconnect evidence in Issue #30; authorized live-view claim route remains open) |
| `CAP-009` | Identity and sessions | Identity | Approved Actions through Portfolio Release | Identity/session integration and negative tests | Observed (test env; CLM-008 session negatives) |
| `CAP-010` | Tenant-aware authorization | Identity | Traceability, Approvals, Governed RAG, Reliability | Cross-tenant negative suite | Observed (test env; CLM-007/008) |
| `CAP-011` | Service identities and least privilege | Identity | Approved Actions through Reliability & Cloud | Service-identity authorization tests and audit evidence | Planned |
| `CAP-012` | Operational visibility | Identity | Reliability | Health, lag, failure, quarantine, and replay evidence | Observed (test env; CLM-009) |
| `CAP-013` | Quarantine and controlled replay operations | Identity | Traceability, Reliability | Authorized fault/replay campaign | Observed (test env; CLM-010 authorization subset) |
| `CAP-014` | Traceability and recall | Traceability | Approvals, Portfolio | Lineage query and recall report | Planned |
| `CAP-015` | Deterministic projection replay and checksum reconstruction | Traceability | Event Spine, Reliability | Deterministic checksum comparison | Planned |
| `CAP-016` | Backup and restore | Traceability | Reliability | Restore experiment with application-readable and integrity checks | Planned |
| `CAP-017` | Recovery and integrity verification | Traceability | Reliability | Recovery report and integrity verification artifacts | Planned |
| `CAP-018` | Forecast datasets and features | Stockout | Portfolio | Versioned dataset/feature manifest | Planned |
| `CAP-019` | Stockout prediction | Stockout | Approvals, Portfolio | Temporal evaluation report | Planned |
| `CAP-020` | Uncertainty and abstention | Stockout | Approvals, Governed RAG, Portfolio | Uncertainty/calibration and abstention evaluation | Planned |
| `CAP-021` | Forecast evaluation and model/data lineage | Stockout | Reliability, Portfolio | Reproducible evaluation and lineage artifacts | Planned |
| `CAP-022` | Typed proposals and authority boundary | Approvals | Stockout, Governed RAG, Portfolio | Proposal contract/security test | Planned |
| `CAP-023` | Policy and human approval workflow | Approvals | Identity, Governed RAG, Portfolio | Approval workflow and audit evidence | Planned |
| `CAP-024` | Execution-time authorization and approval freshness | Approvals | Identity, Reliability | Negative/integration tests across display, approval, and execution | Planned |
| `CAP-025` | Idempotent command execution | Approvals | Reliability | Retry/concurrency fault campaign | Planned |
| `CAP-026` | Durable receipts and uncertain-outcome reconciliation | Approvals | Traceability, Reliability | Receipt and reconciliation report | Planned |
| `CAP-027` | Permission-aware retrieval | Governed RAG | Identity, Portfolio | Governed retrieval evaluation | Planned |
| `CAP-028` | Citations, refusals, and conflicting evidence | Governed RAG | Stockout, Approvals, Portfolio | Citation/refusal/conflict evaluation report | Planned |
| `CAP-029` | Prompt-injection and retrieval-leakage defense | Governed RAG | Identity, Reliability, Portfolio | Adversarial and cross-tenant negative suite | Planned |
| `CAP-030` | Retrieval and answer evaluation | Governed RAG | Reliability, Portfolio | Frozen governed-RAG evaluation report | Planned |
| `CAP-031` | Correlated telemetry and distributed tracing | Reliability | Event Spine through Governed RAG | Trace bundle with correlated metrics and logs | Planned |
| `CAP-032` | SLI and SLO evidence | Reliability | Event Spine through Governed RAG | Environment-scoped SLO evidence report | Planned |
| `CAP-033` | Load and performance evidence | Reliability | Event Spine through Governed RAG | Distribution-aware performance report | Planned |
| `CAP-034` | Fault and degraded-mode campaigns | Reliability | Event Spine through Governed RAG | Fault-campaign report and raw artifacts | Planned |
| `CAP-035` | Restore and recovery evidence campaigns | Reliability | Traceability | Restore/recovery campaign report | Planned |
| `CAP-036` | Reproducible cloud deployment | Reliability | Portfolio | Deployment reproduction record | Planned |
| `CAP-037` | Cost evidence | Reliability | Portfolio | Declared-environment cost report | Planned |
| `CAP-038` | Deterministic public demo | Portfolio | Event Spine through Reliability & Cloud | Demonstration script and release artifact | Planned |
| `CAP-039` | Public release and evidence index | Portfolio | Constitution, Reliability | Release review and evidence index | Planned |
| `CAP-040` | Evidence-bounded README, CV, and portfolio claims | Portfolio | Constitution, Reliability | Public-claim reconciliation review | Planned |

## Dependencies and sequencing

Dependencies are recorded only where supported by the product constitution, architecture, correctness model, security model, and evidence protocols. No dates, durations, staffing assumptions, or delivery commitments are implied.

### Hard implementation dependencies

- Project Constitution precedes Event Spine because the event spine must follow the reviewed architecture, correctness, security, and evidence boundaries.
- Event Spine precedes Identity & Operations because identity and operations govern event-processing and projection surfaces.
- Event Spine precedes Traceability & Recovery because traceability and recovery reuse the event history and projections.
- Event Spine precedes Stockout Intelligence because forecasting features derive from replayable operational state.
- Event Spine and Identity & Operations precede Approved Actions because approved actions require current state and tenant-aware authorization.
- Identity & Operations precedes Governed RAG because retrieval and citation must receive an authoritative permission context.
- Traceability & Recovery precedes the Reliability & Cloud restore/recovery campaign because recovery behavior must exist before it can be evidenced.
- Applicable implementation and Reliability & Cloud evidence precede Portfolio Release claims.

### Sequencing preferences

- Stockout Intelligence output should be available before the complete Approved Actions proposal workflow is demonstrated.
- Approved Actions and Governed RAG should be integrated into the Portfolio Release demonstration only after their authority and evidence boundaries are reviewed.
- Identity & Operations controls should be available before Traceability & Recovery workflows are exposed to operators.

### Cross-cutting evidence dependencies

- Project Constitution defines the status, clean-room, source-of-truth, and claim-promotion rules used by every later evidence record.
- Identity & Operations authorization evidence supports tenant and privileged-operation claims in Traceability & Recovery through Governed RAG.
- Reliability & Cloud supplies operational, restore, performance, deployment, and cost evidence for earlier capabilities; this does not change their primary ownership.
- Portfolio Release reconciles public wording against `EVIDENCE.md` and retains superseded claims and evidence.

## Active capability-sequence decomposition rule

Event Spine exit-gate evidence is recorded (Issue #30). Identity & Operations
exit-gate evidence is recorded (Issue #50). Traceability & Recovery is the next
Planned capability sequence. Detailed GitHub issues remain bounded to
the active capability sequence. Traceability & Recovery through Portfolio Release remain outcome-and-exit-gate placeholders and must
not accumulate speculative implementation backlogs.

When a capability sequence begins:

1. Review its Notion page and the current repository truth.
2. Confirm purpose, user-visible outcome, invariants, trust boundaries, non-goals, dependencies, and required exit evidence.
3. Create only bounded GitHub issues for that active capability sequence.
4. Give each issue one observable outcome, acceptance criteria, failure/security cases, dependencies, evidence requirements, and non-goals.
5. Link the capability-sequence intent to the repository plan without duplicating daily issue status.

## Capability-sequence activation rules

- A capability sequence remains Planned until its dependencies, outcome, exit gate, and evidence route are reviewed.
- The Notion capability-sequence page is reviewed before GitHub issues are created.
- A capability sequence becomes active only when its first bounded issue begins in GitHub.
- Future capability sequences may have only placeholder purpose, outcome, dependencies, and exit-gate information before activation.
- GitHub owns issue and milestone execution state; this file records the stable sequence and governance rules.

## Capability-sequence completion rules

A capability sequence may be marked complete only when:

- Its user-visible outcome works end to end where applicable.
- Required acceptance criteria and relevant correctness, failure, and security cases pass.
- Canonical documentation matches the reviewed implementation.
- Required evidence artifacts exist, are linked to stable claim IDs, and include limitations.
- No README, CV, demo, or portfolio claim exceeds the ledger.
- Important decisions have ADRs where required.
- The deterministic demonstration path and repository checks applicable to the capability sequence are available.
- Residual risks and deferred work are recorded.
- The capability-sequence summary is published through the Notion/GitHub workflow.

Documentation existence alone cannot satisfy a future runtime capability-sequence exit gate.

## Scope-change and contradiction handling

- A scope change must identify the affected capability IDs, primary capability owner, supporting owners, claims, dependencies, and evidence routes.
- A capability may have only one primary owner. If ownership changes, the roadmap records the old and new owner and preserves claim traceability.
- Contradictions are recorded with the sources, impact, disposition, and owner before implementation scope changes.
- A later implementation detail must not silently rewrite a product, security, correctness, or evidence invariant.
- Scope changes that introduce a schedule, target, vendor, model, infrastructure product, or production claim require the owning capability sequence’s explicit decision and evidence route.
- Superseded claims and evidence remain in the ledger; they are never silently deleted or reused.

## Non-goals and deferred decisions

This roadmap does not:

- Create or modify GitHub milestones.
- Create detailed Event Spine through Portfolio Release issues.
- Add application code, runtime scaffolding, infrastructure, CI, schemas, dashboards, or scripts.
- Select identity providers, policy engines, databases, brokers, model vendors, vector stores, cloud products, or deployment sizes.
- Invent schedules, throughput, latency, accuracy, availability, SLO, RPO, RTO, cost, security, or evaluation targets.
- Claim exactly-once network or broker delivery, production readiness, or measured performance.
- Use Ahoy or any private system as a dependency, evidence source, or design input.

Deferred decisions are owned by the relevant capability sequence: completed Issue #9 established repository workflow and documentation CI; Issue #10 owns integrated Project Constitution review; Event Spine owns runtime event implementation; Identity & Operations owns identity and operational enforcement; Traceability & Recovery owns restore behavior; Stockout Intelligence owns forecasting evaluation decisions; Approved Actions owns command and reconciliation implementation; Governed RAG owns retrieval/evaluation implementation; Reliability & Cloud owns deployment and operational evidence decisions; and Portfolio Release owns public release and final claim reconciliation.

## Related documents

- [PRODUCT.md](PRODUCT.md)
- [CLEAN_ROOM.md](CLEAN_ROOM.md)
- [ARCHITECTURE.md](ARCHITECTURE.md)
- [EVENT_MODEL.md](docs/architecture/EVENT_MODEL.md)
- [CONTRACTS.md](CONTRACTS.md)
- [COMMAND_MODEL.md](docs/architecture/COMMAND_MODEL.md)
- [CLAIM_STATUS_VOCABULARY.md](docs/evidence/CLAIM_STATUS_VOCABULARY.md)
- [EVIDENCE.md](EVIDENCE.md)
- [Identity & Operations Exit-Gate Experiment Report](docs/evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md)
