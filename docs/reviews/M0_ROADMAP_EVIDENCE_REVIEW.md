# M0 Roadmap and Evidence Review — Issue #8

This document records a documentation-only review of the Issue #8 roadmap, capability ownership map, and evidence ledger. It does not claim application implementation, runtime correctness, security enforcement, performance, reliability, recovery, production readiness, or completed future experiments.

## 1. Review record

| Field | Value |
| --- | --- |
| Reviewer | Codex implementation/review pass; maintainer review remains a recorded follow-up |
| Date (UTC) | 2026-08-07 |
| Branch | `docs/8-roadmap-evidence-ledger` |
| Base commit | `13201d4` |
| Reviewed state | Issue #8 working tree after documentation implementation; final verification recorded below |
| Scope | `ROADMAP.md`, `EVIDENCE.md`, this review, and the minimal README discoverability links |
| Review type | Documentation implementation and static governance review |
| Documentation result | Pass with recorded follow-ups |
| Runtime result | Not run by design |

The change remains documentation-only. No application code, runtime configuration, infrastructure, schema, dashboard, script, CI workflow, model, dataset, experiment, or GitHub milestone mutation was introduced.

## 2. Sources reviewed

### GitHub and repository sources

- GitHub Issue #8 and its attached expanded request.
- Issues #1–#7 and merged PRs #11–#17.
- `README.md`, `PRODUCT.md`, `CLEAN_ROOM.md`, `ARCHITECTURE.md`, and `LICENSE`.
- `docs/architecture/EVENT_MODEL.md` and `docs/architecture/COMMAND_MODEL.md`.
- `docs/security/THREAT_MODEL.md` and `docs/security/AUTHORIZATION_MODEL.md`.
- `docs/evidence/CLAIM_STATUS_VOCABULARY.md`.
- All files under `docs/evaluation/`, including evidence protocols, schemas, templates, and the fault campaign matrix.
- Both ADRs under `docs/adrs/`.
- All existing M0 review documents under `docs/reviews/` and the clean-room checklist under `docs/checklists/`.

### Canonical planning sources

- [SeshatOps — Master Project Blueprint](https://app.notion.com/p/3b40a821b3cc813081f0ea44fd72692a).
- [Workflow — Notion → GitHub → Evidence](https://app.notion.com/p/3b40a821b3cc810eadebc2fc9a067000).
- [M0 — Project Constitution](https://app.notion.com/p/3b40a821b3cc81f082c2e5e77d0499ee).
- The `SeshatOps Milestones` database and its M0–M8 rows.

The live GitHub repository metadata confirmed that the repository is private and `main` is the default branch. The connected GitHub issue data confirmed Issue #8’s M0 association. A direct anonymous GitHub API request could not enumerate the private milestone collection, so no complete live milestone inventory is claimed here; no milestone was created or modified.

## 3. Issue #8 acceptance-criteria coverage

| Criterion | Coverage | Result |
| --- | --- | --- |
| Create `ROADMAP.md` and `EVIDENCE.md` | Both files exist with the required sections and content. | Covered |
| Define M0–M8 outcomes and testable exit gates | The roadmap contains all nine canonical names, outcomes, gates, and statuses. | Covered |
| Keep detailed implementation issues limited to the active milestone | M0 is the only active milestone; M1–M8 contain only outcome, dependency, capability, and exit-gate placeholders. | Covered |
| Map every major capability to exactly one primary milestone | `CAP-001`–`CAP-040` are unique and each appears once as a primary owner. | Covered |
| Define required evidence-ledger fields | The field-definition section and ledger identify claim, status, owner, verification, artifact, environment, commit/release, date, reviewer, and limitations. | Covered |
| Mark every initial claim Planned | `CLM-001`–`CLM-035` all have exactly `Planned` status. | Covered |
| State source-of-truth boundaries | Notion, GitHub, repository documents, pull requests, and evidence artifacts are explicitly separated. | Covered |
| Preserve the invariants | No orphan capability, no duplicate primary owner, no unsupported promotion, no invented schedule/target, and no Ahoy dependency are documented. | Covered |

## 4. M0–M8 milestone-name review

| ID | Canonical name | Roadmap status |
| --- | --- | --- |
| M0 | Project Constitution | Active |
| M1 | Event Spine | Planned |
| M2 | Identity & Operations | Planned |
| M3 | Traceability & Recovery | Planned |
| M4 | Stockout Intelligence | Planned |
| M5 | Approved Actions | Planned |
| M6 | Governed RAG | Planned |
| M7 | Reliability & Cloud | Planned |
| M8 | Portfolio Release | Planned |

All nine names match the canonical map. M5 wording is intentionally expressed as one durable business effect for equivalent intent under retries, not exactly-once network or broker delivery. M6 thresholds remain Planned and no numerical target is introduced.

## 5. Milestone outcome and exit-gate review

- M0 describes the integrated documentation outcome and the gate required before M1 issue creation.
- M1 covers the synthetic-ERP-to-live-view vertical slice, duplicate safety, and deterministic reconstruction.
- M2 covers identity, tenant authorization, service identities, operational visibility, quarantine, and controlled replay.
- M3 covers lineage, deterministic recovery replay, checksum comparison, backup/restore, and integrity verification.
- M4 covers temporal forecasting evaluation, uncertainty, abstention, and lineage.
- M5 preserves the proposal → policy → human approval → execution-time recheck → idempotent command → receipt boundary.
- M6 preserves permission-first retrieval, citations, refusals, conflicting evidence, prompt-injection resistance, and evaluation.
- M7 preserves telemetry, SLO, performance, fault, restore, deployment, and cost evidence without targets.
- M8 reconciles the deterministic demo, public release, evidence index, clean-room independence, and portfolio claims.

Every milestone has an observable outcome and a testable exit gate. No milestone has a calendar date, duration, staffing assumption, or invented threshold.

## 6. Capability inventory and ownership checks

The roadmap contains exactly 40 stable capability IDs, `CAP-001` through `CAP-040`:

| Milestone | Capability IDs | Coverage |
| --- | --- | --- |
| M0 | `CAP-001`–`CAP-003` | Constitution/clean room; architecture/correctness/security; evidence/claims |
| M1 | `CAP-004`–`CAP-008` | Synthetic ERP/workload; outbox; event contracts/transport; projections; live view |
| M2 | `CAP-009`–`CAP-013` | Identity; tenant authorization; service identities; visibility; quarantine/replay |
| M3 | `CAP-014`–`CAP-017` | Traceability; deterministic replay/checksum; backup/restore; recovery/integrity |
| M4 | `CAP-018`–`CAP-021` | Forecast data/features; prediction; uncertainty/abstention; evaluation/lineage |
| M5 | `CAP-022`–`CAP-026` | Typed proposals; approval; execution rechecks; idempotency; receipts/reconciliation |
| M6 | `CAP-027`–`CAP-030` | Permission-aware retrieval; citations/refusals; injection/leakage defense; evaluation |
| M7 | `CAP-031`–`CAP-037` | Telemetry; SLO; performance; fault; restore/recovery evidence; deployment; cost |
| M8 | `CAP-038`–`CAP-040` | Deterministic demo; release/evidence index; public claim bounds |

The primary-ownership check is satisfied:

- No capability is orphaned.
- No capability has two primary milestones.
- Supporting milestones are explicitly separate from primary ownership.
- M1’s initial projection rebuild and M3’s authorized recovery replay/checksum evidence are intentionally distinct capability boundaries.
- M2 owns quarantine/replay operations; M3 owns reconstruction/recovery behavior; M7 owns campaign evidence.

## 7. Capability-to-evidence-route check

Every capability row includes a future verification route. The routes are limited to evidence types already supported by the canonical documents: documentation review, contract/compatibility test, integration test, cross-tenant negative suite, deterministic checksum comparison, lineage query, restore experiment, temporal evaluation, governed-RAG evaluation, fault campaign, trace bundle, SLO report, performance report, deployment reproduction, demonstration script, and release review.

No route is represented as already executed. No planned filename is presented as an evidence artifact.

## 8. Milestone dependency review

The roadmap differentiates hard implementation dependencies, sequencing preferences, and cross-cutting evidence dependencies.

- M0 precedes the event spine and all later milestone planning.
- M1 supplies the operational history and projections used by M2–M5.
- M2 supplies authorization and permission context used by M3, M5, and M6.
- M3 supplies recovery behavior required before M7 restore/recovery campaigns.
- M4 supplies intelligence output for the M5 proposal flow.
- M7 supplies operational evidence for earlier capabilities without becoming their primary owner.
- M8 consumes completed capability evidence and reconciles public claims.

No dependency introduces a vendor, schema, target, schedule, or staffing assumption.

## 9. Active-milestone decomposition review

M0 is the only active milestone. The roadmap explicitly requires Notion review, dependency and evidence confirmation, and bounded GitHub issues before a milestone begins. M1–M8 remain placeholders and have no speculative issue backlog. GitHub remains the execution-status owner.

## 10. Evidence-ledger field review

`EVIDENCE.md` defines all required fields:

- Claim ID
- Claim
- Status
- Capability ID
- Owning milestone
- Verification method
- Required artifact / Artifact
- Evidence artifact
- Environment class / Environment
- Repository commit or release / Commit/release
- Observation or decision date / Date
- Reviewer
- Limitations
- Supersedes
- Superseded by
- Notes/follow-up

The required Issue #8 fields remain directly identifiable in the ledger. Initial evidence artifacts, environments, commits, dates, and reviewers are empty or `Planned` as required.

## 11. Stable claim-ID review

The initial ledger contains exactly 35 unique stable IDs, `CLM-001` through `CLM-035`. Each claim has one capability ID, one owning milestone, one future verification method, and one required artifact category. No evidence artifact link is fabricated.

## 12. Initial claim coverage

The initial claims cover:

- M0 constitution and clean-room independence.
- Outbox durability, duplicate-safe processing, event compatibility, and deterministic rebuild.
- Tenant isolation, default-deny operations, operational visibility, and controlled replay.
- Traceability, backup/restore, recovery, and integrity.
- Forecast evaluation, uncertainty, abstention, and lineage.
- Typed proposals, authority boundaries, execution rechecks, retries, receipts, and reconciliation.
- Permission-aware retrieval, citations, evaluation, prompt injection, tenant leakage, and refusal.
- Telemetry, SLO, performance, fault, restore, deployment, and cost evidence.
- Deterministic demo, Ahoy-independent release, and evidence-bounded public claims.

## 13. Verification that every initial claim is Planned

All 35 initial ledger rows have `Status` exactly equal to `Planned`. No row is `Implemented`, `Observed`, `Reproduced`, `Superseded`, `Passed`, or `Production ready`. `Not evaluated` and `Not executed` are not used as claim statuses.

## 14. Unsupported-claim scan

The implementation does not introduce:

- Delivery, accuracy, latency, throughput, availability, cost, security, SLO, RPO, RTO, or production targets.
- Invented schedules, durations, staffing, repetition counts, or pass thresholds.
- Runtime results, benchmark values, experiment outcomes, or production observations.
- Selected vendors, infrastructure products, identity providers, policy engines, models, libraries, schemas, topics, or deployment sizes.
- Evidence links to artifacts that do not exist.

## 15. Source-of-truth boundary review

The roadmap assigns Notion to intent and milestone summaries, GitHub to execution state, repository documents to reviewed technical truth, and pull requests/evidence artifacts to actual changes and claim support. The ledger does not duplicate daily issue state or redefine the canonical claim vocabulary.

## 16. Consistency with Issues #1–#7

| Prior issue | Preserved boundary |
| --- | --- |
| #1 | Northstar Foods, complete product loop, planned status, and explicit non-goals remain intact. |
| #2 | Clean-room independence, public-safe provenance, and no private source material remain explicit. |
| #3 | TypeScript/Go/Python ownership and logical architecture boundaries remain unchanged. |
| #4 | At-least-once delivery, idempotent business effects, durable receipts, replay safety, and no exactly-once marketing remain intact. |
| #5 | Default-deny authorization, tenant isolation, service identity, approval/execution separation, and Python’s non-authoritative role remain intact. |
| #6 | Temporal evaluation, uncertainty, abstention, lineage, permission-aware RAG, citations, refusals, and typed proposals remain Planned. |
| #7 | Claim vocabulary, evidence environments, reproducibility fields, fault/recovery/performance protocols, and absence of targets remain intact. |

## 17. Clean-room confirmation

- No Ahoy repository or private Ahoy artifact was accessed.
- No private code, schema, migration, data, log, trace, screenshot, identifier, workload, incident, metric, model output, or production behavior was used.
- No private identifier denylist was created.
- The roadmap and ledger use the existing fictional Northstar Foods scenario and generic public-safe terminology.
- Ahoy is named only as an excluded dependency and release-review boundary.

## 18. Contradictions, assumptions, and unresolved decisions

| Finding | Disposition |
| --- | --- |
| Notion mentions M0–M8 GitHub milestones while Issue #8 forbids milestone mutation. | Existing state was treated as read-only; the repository roadmap is the canonical map for this change. |
| Deterministic rebuild/replay appears in M1 and M3. | M1 owns the initial projection invariant; M3 owns recovery replay and checksum evidence. |
| Quarantine/replay spans M2, M3, and M7. | M2 owns controls, M3 owns recovery behavior, and M7 owns evidence campaigns. |
| M5’s exactly-one wording can be misread as exactly-once delivery. | Roadmap wording is bounded to one durable business effect for equivalent intent under retries. |
| Notion M0 lists `AGENTS.md`, but Issue #8 allows only the expected files and Issue #9 owns repository instructions. | No `AGENTS.md` was added; defer to Issue #9. |
| The private GitHub milestone collection could not be enumerated anonymously. | Record as an inspection limitation; no milestone mutation was attempted. |

## 19. Deferred work and owners

| Work | Owner |
| --- | --- |
| Repository instructions, PR workflow, documentation CI | Issue #9 |
| Integrated adversarial M0 review | Issue #10 |
| Runtime event spine implementation | M1 |
| Identity/session and operational enforcement | M2 |
| Recovery implementation and restore behavior | M3 |
| Forecast evaluation decisions and thresholds | M4 |
| Command execution and reconciliation implementation | M5 |
| Retrieval implementation and governed-RAG thresholds | M6 |
| Deployment, operational evidence, and cost methodology | M7 |
| Public release and final claim reconciliation | M8 |

## 20. Residual risks

- The roadmap and ledger are governance artifacts and do not prove future implementation.
- Capability grouping and evidence routes may require controlled revision as active milestones make implementation decisions.
- No runtime test harness, identity system, retrieval system, telemetry pipeline, backup system, deployment environment, or production environment exists.
- Future evidence may be incomplete, non-reproducible, invalidated, or too broadly interpreted; withdrawal and supersession must remain available.
- Maintainer review and the integrated Issue #10 constitution review remain follow-ups.

## 21. Verification record

| Check | Result |
| --- | --- |
| Changed-path allowlist | Pass; exactly `README.md`, `ROADMAP.md`, `EVIDENCE.md`, and this review are changed. |
| `git diff --check` and untracked-file whitespace scan | Pass; no whitespace errors or trailing spaces found. |
| Markdown link target validation | Pass; all checked repository-relative links resolve. |
| Nine milestone names, outcomes, and exit gates | Pass; all nine canonical entries are present. |
| Unique capability IDs and exactly one primary owner | Pass; 40 capability rows and 40 unique primary IDs are present. |
| Orphan-capability search | Pass; all roadmap capability IDs are represented in the primary matrix. |
| Unique claim IDs and 35-row inventory | Pass; 35 claim rows and 35 unique IDs are present. |
| Every initial claim status exactly `Planned` | Pass; all 35 rows are `Planned` and none use another claim status. |
| Empty initial evidence/artifact/date/reviewer fields | Pass; every initial row has evidence `—`, environment `Planned`, and commit/date/reviewer `—`. |
| Unsupported schedule/target/result/vendor scan | Pass; no fabricated numeric targets, schedules, runtime results, vendors, or secret-like literals found. |
| Source-of-truth boundary review | Pass by document inspection |
| Clean-room category review | Pass by document inspection; no private Ahoy material accessed |
| Runtime, infrastructure, security-control, reliability, performance, recovery, and production tests | Not run by design |

## Final result

**Pass with recorded follow-ups** for documentation coverage and static governance review only. This result does not claim runtime implementation, observed behavior, reproduced behavior, production readiness, verified security, reliability, restorability, performance, availability, or any SLO.
