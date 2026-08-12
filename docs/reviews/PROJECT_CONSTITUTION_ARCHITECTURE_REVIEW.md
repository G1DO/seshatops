# Project Constitution Architecture Review â€” Issue #3

Evidence that an architecture-boundary review **occurred** for SeshatOps Issue #3 (logical topology, language ownership, trust boundaries, storage responsibilities). Checked items are not proof that no undiscovered issue exists.

## Review record

| Field | Value |
| --- | --- |
| Reviewer | G1DO (maintainer; time-separated self-review completed) |
| Date (UTC) | 2026-08-06 |
| First-pass tip | Architecture docs at `43be239cc98ead99cdfc0e00d73bd60e77eb1533`; review tip record at `4228ea91343c39cc59e4e5b12dd9f3559d5ab6fb` |
| Commit / branch reviewed | Branch `docs/3-architecture-boundaries`; tip updated after PR-review remediation (see tip SHA below). Base `219160b4ebc65b67978e0f2361d9f85f5faa6a2e` (`main` after PR #12). |
| Tip SHA (this remediation) | `4324f77d896df45dff7c8952ca818b62cc118ee4` |
| Scope | Issue #3 â€” Define architecture boundaries and language ownership; PR #13 review remediation |
| Result | Pass with recorded follow-ups |

### Time-separated self-review

| Pass | When (UTC) | What was reviewed |
| --- | --- | --- |
| 1 | 2026-08-06 (authoring) | Initial `ARCHITECTURE.md`, review matrix, README link against Issue #3, PRODUCT, CLEAN_ROOM, and Notion Constitution/Blueprint |
| 2 | 2026-08-06 (PR review remediation) | Separate pass after PR feedback: approve/validate order vs PRODUCT hero steps 9â€“11; Goâ†’Python one-way edges; Redpanda produce wording; Issue #4 vocabulary softening; naming; matrix and clean-room record |

Pass 2 was performed after a break from Pass 1 authoring and incorporates external PR review findings. Result below applies only after Pass 2 remediation is in the tip commit.

## Reviewed sources

| Source | Role |
| --- | --- |
| GitHub Issue #3 brief (architecture boundaries and language ownership) | Acceptance criteria and required content |
| GitHub PR #13 review feedback | Remediation items for this pass |
| [PRODUCT.md](../../PRODUCT.md) | Product thesis, users, loop, non-goals (Issue #1 / PR #11); hero steps 9â€“11 |
| [CLEAN_ROOM.md](../../CLEAN_ROOM.md) | Clean-room policy (Issue #2 / PR #12) |
| [docs/checklists/CLEAN_ROOM_REVIEW.md](../checklists/CLEAN_ROOM_REVIEW.md) | Review checklist pattern; baseline CRR-0001 |
| [README.md](../../README.md) | Public summary and status |
| [LICENSE](../../LICENSE) | Apache License 2.0 |
| Notion: [SeshatOps â€” Master Project Blueprint](https://app.notion.com/p/3b40a821b3cc813081f0ea44fd72692a) | Approved architecture intent |
| Notion: [Workflow â€” Notion â†’ GitHub â†’ Evidence](https://app.notion.com/p/3b40a821b3cc810eadebc2fc9a067000) | Source-of-truth boundaries |
| Notion: [Constitution â€” Project Constitution](https://app.notion.com/p/3b40a821b3cc81f082c2e5e77d0499ee) | Constitution scope, issue seed list, adversarial decisions |
| Merged PR #11 / Issue #1 | Product constitution outcome |
| Merged PR #12 / Issue #2 | Clean-room policy outcome |

No Ahoy repository, private source, schema, migration, log, screenshot, configuration, or production artifact was inspected.

## Contradiction and ambiguity matrix

| ID | Sources | Finding | Disposition |
| --- | --- | --- | --- |
| C1 | Master Blueprint â€œExact stackâ€ vs Issue #3 / Constitution adversarial decisions | Blueprint names concrete frameworks and deploy tooling; Constitution says this milestone fixes ownership and trust boundaries, not versions, schemas, or deployment topology | **Resolved for Issue #3:** `ARCHITECTURE.md` records logical paths and store roles only. Concrete stack and protocol choices are deferred (see Deferred decisions). |
| C2 | Master Blueprint communication table vs Issue #3 | Blueprint specifies particular browser and RPC mechanisms; Issue #3 forbids endpoint paths and concrete framework internals | **Resolved for Issue #3:** Allowed and prohibited paths are logical. Transports deferred to Issues #4 and #9. |
| C3 | PRODUCT.md language-agnostic hero steps vs Blueprint language-tagged steps | Different document layers, not conflicting requirements | **No conflict.** PRODUCT owns product narrative; ARCHITECTURE owns language tagging. |
| C4 | Blueprint progress tracker vs repository and Constitution page | Blueprint still showed constitution â€œNot startedâ€ while Issues #1â€“#2 were merged | **Out of ARCHITECTURE scope.** Notion blueprint freshness only; no repository contradiction. |
| C5 | Blueprint SOP-001â€¦010 vs GitHub Issues #1â€“#10 | Parallel numbering schemes | **Resolved:** GitHub Issues are the execution source of truth per Workflow. Follow #3â€“#10. |
| C6 | PRODUCT.md hero steps 9â€“11 vs ARCHITECTURE sequence diagram | First-pass diagram showed human approve before Go validate/authorize; PRODUCT requires platform validation then human approval then execute (with recheck before command) | **Fixed:** Sequence and prose now validate/authorize â†’ human approve â†’ short recheck â†’ ERP command. |
| A1 | Blueprint Pythonâ†’PostgreSQL read-only feature views vs Issue #3 â€œno broad transactional-database credentialsâ€ | Ambiguity on whether Python may read PostgreSQL at all | **Decided:** Narrow, non-transactional read of approved feature/read surfaces is allowed. Broad write/workflow credentials are prohibited. Exact database-role design belongs to Issue #5. |
| A2 | PRODUCT synthetic ERP capability vs architecture component detail | How much ERP detail belongs in ARCHITECTURE | **Decided:** Synthetic ERP is a Go-owned logical component and the public operational boundary. No schemas or endpoints in Issue #3. |
| A3 | ARCHITECTURE allowed paths vs Go credential â€œconsume/produceâ€ | Topology and allowed paths showed ERPâ†’Redpandaâ†’Go consume only; credentials still mentioned produce | **Fixed:** Removed produce from Go credentials until Issue #4 defines publication ownership. |
| â€” | Northstar Foods public scenario | Aligned across PRODUCT, CLEAN_ROOM, Blueprint, Constitution | **None found** |
| â€” | Ahoy excluded from public dependencies | Aligned across PRODUCT, CLEAN_ROOM, Blueprint, Constitution | **None found** |
| â€” | Human approval before execution; UI not authoritative for authorization | Aligned across PRODUCT and Blueprint | **None found** |
| â€” | Honest consistency stance (at-least-once with idempotent effects) | Product/Blueprint agree; detailed protocol is Issue #4 | **Aligned; detail deferred** |

No additional contradictions were invented to populate this table.

## Ownership-boundary review

- [x] TypeScript ownership and exclusions are unambiguous
- [x] Go ownership and exclusions are unambiguous
- [x] Python ownership and exclusions are unambiguous; output is advisory until Go-governed execution
- [x] PostgreSQL, Redpanda, and object-storage responsibilities are clear at a logical level
- [x] Required prohibited paths are explicit (browserâ†’Python/DB/broker; Pythonâ†’business writes and operational commands; UI-owned authorization; private adapter as required public dependency)
- [x] Allowed paths are documented without locking endpoint paths or schemas
- [x] Goâ†’Python edges are one-way initiation (Go initiates; Python responds on that invocation)
- [x] Proposal flow order matches PRODUCT hero steps 9â€“11 (validate/authorize â†’ approve â†’ recheck â†’ execute)
- [x] Redpanda path and credential wording are consistent (consume only until Issue #4)
- [x] Product languages are TypeScript, Go, and Python only; C is permanently excluded; Rust is not used unless a later measured ADR gate justifies one isolated component
- [x] Python failure is isolated from core transactional operations
- [x] Ahoy is absent from the public topology (named only as excluded)
- [x] Logical Mermaid diagrams cover topology, proposal-to-execution, and trust boundaries
- [x] High-level credential table does not claim to be the full authorization model
- [x] Logical architecture is distinguished from deployment architecture
- [x] Outbox/inbox/dedup protocol details are not frozen; pointed at Issue #4
- [x] No application code, package layouts, API routes, database schemas, event schemas, topic names, or dependency versions introduced

## Clean-room confirmation

- [x] No Ahoy or other private code, schemas, migrations, data, logs, traces, or production config used as a source
- [x] Ahoy appears only as an excluded private system
- [x] Public scenario remains Northstar Foods / synthetic ERP only
- [x] Content is safe for eventual public release
- [x] Uncertain private provenance excluded by construction (SeshatOps-only sources)

### CRR-0002 â€” Architecture boundaries clean-room record (Issue #3)

| Field | Value |
| --- | --- |
| Reviewer | G1DO (maintainer; time-separated self-review) |
| Date (UTC) | 2026-08-06 |
| Commit / tip | First-pass architecture at `43be239cc98ead99cdfc0e00d73bd60e77eb1533`; remediation tip `4324f77d896df45dff7c8952ca818b62cc118ee4` |
| Scope (paths / PR) | `ARCHITECTURE.md`, `docs/reviews/PROJECT_CONSTITUTION_ARCHITECTURE_REVIEW.md`, `README.md`, `PRODUCT.md` (Â§10 ownership link); PR #13 |
| Review type | pre-PR |
| Result | Pass with remediation |

#### Checks

- [x] No Ahoy or other private code, schemas, migrations, data, logs, traces, or production config
- [x] No private screenshots, recordings, or exports
- [x] No production identifiers, hostnames, internal URLs, or private account/tenant IDs
- [x] No private business-specific rules, recipes, prices, customers, suppliers, or process knowledge
- [x] No secrets, credentials, tokens, or private environment files
- [x] No raw AI conversations or prompt histories containing private context
- [x] All new material has a permitted source or recorded synthetic provenance
- [x] Synthetic data (if any) records origin, generation method, license, reproducibility, and independence
- [x] AI-assisted work (if any) used clean-room inputs only; output human-reviewed
- [x] Screenshots, examples, fixtures, names, schemas, and terminology are fictional/generic and independently explainable
- [x] Uncertain provenance excluded (not sanitized and kept)
- [x] Category repository search run over scope for exclusion terms and secret-like strings
- [x] Public artifacts remain independently explainable and reproducible without private systems
- [x] No private denylist of real identifiers was added

#### Findings

1. Sequence diagram order disagreed with PRODUCT hero steps 9â€“11 (approve before validate).
2. Topology/trust diagrams used bidirectional Goâ†”Python edges while prose said Go initiates.
3. Go credentials mentioned Redpanda produce without an allowed produce path.
4. Some wording foreshadowed outbox/inbox/dedup as if already decided (Issue #4).

No private Ahoy material, secrets, or application code found. `Ahoy` appears only as an excluded private system.

#### Remediation

1. Reordered sequence and prose: validate/authorize â†’ human approve â†’ recheck â†’ ERP command.
2. Changed Goâ†’Python edges to one-way initiation in both Mermaid diagrams and the allowed-path table.
3. Removed Redpanda produce from Go credentials until Issue #4 defines publication ownership.
4. Softened outbox/inbox/dedup vocabulary; pointed protocols at Issue #4.
5. Renamed policy-engine credentials â†’ policy credentials; IntelligenceServices â†’ Intelligence.
6. Updated PRODUCT.md Â§10 to point at `ARCHITECTURE.md` and `CLEAN_ROOM.md`.

#### Notes

- Category search covered the scoped files for `Ahoy`, secret-like patterns, and confirmation that customer/supplier language remains fictional/generic.
- No Ahoy repository or private Ahoy artifact was inspected or used for this review.
- Checked boxes record that this review occurred; they do not prove absence of undiscovered issues.

## Deferred decisions and owning issues

| Decision | Owning issue |
| --- | --- |
| Event envelope, commands, compatibility, consumption/idempotency mechanisms, consistency, replay semantics, retries, broker produce/publication ownership | #4 |
| Threat model, identity integration choice, authorization matrix, data classification, database-role detail | #5 |
| Forecast and governed-RAG evaluation protocols, promotion gates, adversarial corpora | #6 |
| Security, reliability, recovery, and performance evidence protocols; SLO and fault-campaign detail | #7 |
| Roadmap, milestone map, evidence ledger | #8 |
| Repository instructions, PR workflow, documentation CI, concrete local/runtime layout | #9 |
| Integrated constitution review across Project Constitution documents | #10 |
| Concrete browser/intelligence transports and contract tooling | #4 / #9 |
| Deployment profiles and cloud topology | Later milestones (not Issue #3) |
| Rust admission measurement procedure | Later ADR when a gate is proposed |

## Findings

PR-review findings (C6, A3, diagram direction, Issue #4 vocabulary) are remediated in this change set. Remaining follow-ups are the deferred decisions above.

## Remediation

See CRR-0002 remediation list. Tip SHA recorded as `4324f77d896df45dff7c8952ca818b62cc118ee4`.

## Final result

> Pass with recorded follow-ups

Follow-ups are explicitly owned by Issues #4â€“#10 and later ADRs. They do not leave Issue #3 architecture ownership ambiguous. Pass applies to the tip that includes Pass 2 remediation.
