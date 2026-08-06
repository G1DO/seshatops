# M0 Architecture Review — Issue #3

Evidence that an architecture-boundary review **occurred** for SeshatOps Issue #3 (logical topology, language ownership, trust boundaries, storage responsibilities). Checked items are not proof that no undiscovered issue exists.

## Review record

| Field | Value |
| --- | --- |
| Reviewer | G1DO (maintainer; time-separated self-review planned at PR) |
| Date (UTC) | 2026-08-06 |
| Commit / branch reviewed | Branch `docs/3-architecture-boundaries`; base `219160b4ebc65b67978e0f2361d9f85f5faa6a2e` (matches `main` after PR #12). Architecture artifacts in this change set: `ARCHITECTURE.md`, this file, and the README discoverability link. |
| Scope | Issue #3 — Define architecture boundaries and language ownership |
| Result | Pass with recorded follow-ups |

## Reviewed sources

| Source | Role |
| --- | --- |
| GitHub Issue #3 brief (architecture boundaries and language ownership) | Acceptance criteria and required content |
| [PRODUCT.md](../../PRODUCT.md) | Product thesis, users, loop, non-goals (Issue #1 / PR #11) |
| [CLEAN_ROOM.md](../../CLEAN_ROOM.md) | Clean-room policy (Issue #2 / PR #12) |
| [docs/checklists/CLEAN_ROOM_REVIEW.md](../checklists/CLEAN_ROOM_REVIEW.md) | Review checklist pattern; baseline CRR-0001 |
| [README.md](../../README.md) | Public summary and status |
| [LICENSE](../../LICENSE) | Apache License 2.0 |
| Notion: [SeshatOps — Master Project Blueprint](https://app.notion.com/p/3b40a821b3cc813081f0ea44fd72692a) | Approved architecture intent |
| Notion: [Workflow — Notion → GitHub → Evidence](https://app.notion.com/p/3b40a821b3cc810eadebc2fc9a067000) | Source-of-truth boundaries |
| Notion: [M0 — Project Constitution](https://app.notion.com/p/3b40a821b3cc81f082c2e5e77d0499ee) | M0 scope, issue seed list, adversarial decisions |
| Merged PR #11 / Issue #1 | Product constitution outcome |
| Merged PR #12 / Issue #2 | Clean-room policy outcome |

No Ahoy repository, private source, schema, migration, log, screenshot, configuration, or production artifact was inspected.

## Contradiction and ambiguity matrix

| ID | Sources | Finding | Disposition |
| --- | --- | --- | --- |
| C1 | Master Blueprint “Exact stack” vs Issue #3 / M0 adversarial decisions | Blueprint names concrete frameworks and deploy tooling; M0 says this milestone fixes ownership and trust boundaries, not versions, schemas, or deployment topology | **Resolved for Issue #3:** `ARCHITECTURE.md` records logical paths and store roles only. Concrete stack and protocol choices are deferred (see Deferred decisions). |
| C2 | Master Blueprint communication table vs Issue #3 | Blueprint specifies particular browser and RPC mechanisms; Issue #3 forbids endpoint paths and concrete framework internals | **Resolved for Issue #3:** Allowed and prohibited paths are logical. Transports deferred to Issues #4 and #9. |
| C3 | PRODUCT.md language-agnostic hero steps vs Blueprint language-tagged steps | Different document layers, not conflicting requirements | **No conflict.** PRODUCT owns product narrative; ARCHITECTURE owns language tagging. |
| C4 | Blueprint progress tracker vs repository and M0 page | Blueprint still showed constitution “Not started” while Issues #1–#2 were merged | **Out of ARCHITECTURE scope.** Notion blueprint freshness only; no repository contradiction. |
| C5 | Blueprint SOP-001…010 vs GitHub Issues #1–#10 | Parallel numbering schemes | **Resolved:** GitHub Issues are the execution source of truth per Workflow. Follow #3–#10. |
| A1 | Blueprint Python→PostgreSQL read-only feature views vs Issue #3 “no broad transactional-database credentials” | Ambiguity on whether Python may read PostgreSQL at all | **Decided:** Narrow, non-transactional read of approved feature/read surfaces is allowed. Broad write/workflow credentials are prohibited. Exact database-role design belongs to Issue #5. |
| A2 | PRODUCT synthetic ERP capability vs architecture component detail | How much ERP detail belongs in ARCHITECTURE | **Decided:** Synthetic ERP is a Go-owned logical component and the public operational boundary. No schemas or endpoints in Issue #3. |
| — | Northstar Foods public scenario | Aligned across PRODUCT, CLEAN_ROOM, Blueprint, M0 | **None found** |
| — | Ahoy excluded from public dependencies | Aligned across PRODUCT, CLEAN_ROOM, Blueprint, M0 | **None found** |
| — | Human approval before execution; UI not authoritative for authorization | Aligned across PRODUCT and Blueprint | **None found** |
| — | Honest consistency stance (at-least-once with idempotent effects) | Product/Blueprint agree; detailed protocol is Issue #4 | **Aligned; detail deferred** |

No additional contradictions were invented to populate this table.

## Ownership-boundary review

- [x] TypeScript ownership and exclusions are unambiguous
- [x] Go ownership and exclusions are unambiguous
- [x] Python ownership and exclusions are unambiguous; output is advisory until Go-governed execution
- [x] PostgreSQL, Redpanda, and object-storage responsibilities are clear at a logical level
- [x] Required prohibited paths are explicit (browser→Python/DB/broker; Python→business writes and operational commands; UI-owned authorization; private adapter as required public dependency)
- [x] Allowed paths are documented without locking endpoint paths or schemas
- [x] Product languages are TypeScript, Go, and Python only; C is permanently excluded; Rust is not used unless a later measured ADR gate justifies one isolated component
- [x] Python failure is isolated from core transactional operations
- [x] Ahoy is absent from the public topology (named only as excluded)
- [x] Logical Mermaid diagrams cover topology, proposal-to-execution, and trust boundaries
- [x] High-level credential table does not claim to be the full authorization model
- [x] Logical architecture is distinguished from deployment architecture
- [x] No application code, package layouts, API routes, database schemas, event schemas, topic names, or dependency versions introduced

## Clean-room confirmation

- [x] No Ahoy or other private code, schemas, migrations, data, logs, traces, or production config used as a source
- [x] Ahoy appears only as an excluded private system
- [x] Public scenario remains Northstar Foods / synthetic ERP only
- [x] Content is safe for eventual public release
- [x] Uncertain private provenance excluded by construction (SeshatOps-only sources)

## Deferred decisions and owning issues

| Decision | Owning issue |
| --- | --- |
| Event envelope, commands, compatibility, inbox/deduplication, consistency, replay semantics, retries | #4 |
| Threat model, identity integration choice, authorization matrix, data classification, database-role detail | #5 |
| Forecast and governed-RAG evaluation protocols, promotion gates, adversarial corpora | #6 |
| Security, reliability, recovery, and performance evidence protocols; SLO and fault-campaign detail | #7 |
| Roadmap, milestone map, evidence ledger | #8 |
| Repository instructions, PR workflow, documentation CI, concrete local/runtime layout | #9 |
| Integrated constitution review across M0 documents | #10 |
| Concrete browser/intelligence transports and contract tooling | #4 / #9 |
| Deployment profiles and cloud topology | Later milestones (not Issue #3) |
| Rust admission measurement procedure | Later ADR when a gate is proposed |

## Findings

None that block Issue #3 acceptance. Follow-ups are the deferred decisions above.

## Remediation

None required for this review scope.

## Final result

**Pass with recorded follow-ups**

Follow-ups are explicitly owned by Issues #4–#10 and later ADRs. They do not leave Issue #3 architecture ownership ambiguous.
