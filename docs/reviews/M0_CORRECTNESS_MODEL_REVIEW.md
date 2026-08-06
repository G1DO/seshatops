# M0 Correctness Model Review — Issue #4

Evidence that a documentation review occurred for SeshatOps Issue #4: event, command, consistency, idempotency, replay, and failure-handling principles. Checked items are not proof that a future implementation will satisfy the model.

## Review record

| Field | Value |
| --- | --- |
| Reviewer | Codex documentation implementation/review pass; maintainer review remains a recorded follow-up |
| Date (UTC) | 2026-08-06 |
| Branch / base | `docs/4-correctness-model`; base commit `32d6bb555883f1ff5c42fe3ecf3ed29ff066f46b`; working-tree changes reviewed before commit |
| Scope | Issue #4 correctness model, event and command documents, ADRs 0001–0002, discoverability links |
| Review type | Documentation pre-commit / pre-PR review |
| Result | Pass with recorded follow-ups |

This review evaluates the conceptual documentation only. It does not claim runtime implementation, production readiness, measured reliability, verified security, or exactly-once behavior.

## Reviewed sources

- [GitHub Issue #4](https://github.com/G1DO/seshatops/issues/4) and the supplied expanded Issue #4 brief.
- Issues #1–#3 and merged PRs [#11](https://github.com/G1DO/seshatops/pull/11), [#12](https://github.com/G1DO/seshatops/pull/12), and [#13](https://github.com/G1DO/seshatops/pull/13).
- Repository product, clean-room, architecture, checklist, review, and license documents.
- Notion: [SeshatOps — Master Project Blueprint](https://app.notion.com/p/3b40a821b3cc813081f0ea44fd72692a), [Workflow — Notion → GitHub → Evidence](https://app.notion.com/p/3b40a821b3cc810eadebc2fc9a067000), and [M0 — Project Constitution](https://app.notion.com/p/3b40a821b3cc81f082c2e5e77d0499ee).
- The Issue #4 artifacts in this working tree: `EVENT_MODEL.md`, `COMMAND_MODEL.md`, ADRs 0001–0002, and this review.

## Requirement-to-invariant traceability

| ID | Required invariant | Primary coverage | Review disposition |
| --- | --- | --- | --- |
| I-01 | Duplicate event delivery cannot duplicate business effects | `EVENT_MODEL.md` §5, EM-04; `COMMAND_MODEL.md` §4, CM-02–CM-04 | Covered conceptually |
| I-02 | Accepted source transactions cannot disappear during broker outage | `EVENT_MODEL.md` §4 and §8, EM-02; ADR-0001 | Covered conceptually |
| I-03 | Repeated equivalent commands produce one durable business effect | `COMMAND_MODEL.md` §4, §6, CM-02; ADR-0002 | Covered conceptually |
| I-04 | Same idempotency key with different intent is rejected | `COMMAND_MODEL.md` §2 and §4, CM-03; ADR-0002 | Covered conceptually |
| I-05 | Authorization is rechecked at command execution | `COMMAND_MODEL.md` §3 and §5, CM-05; ADR-0002 | Covered conceptually |
| I-06 | Stale approval cannot authorize changed intent or state | `COMMAND_MODEL.md` §2 and §5, CM-06; ADR-0002 | Covered conceptually |
| I-07 | Missing aggregate versions cannot be silently skipped | `EVENT_MODEL.md` §3 and §6, EM-05 and EM-08; ADR-0001 | Covered conceptually |
| I-08 | Replay cannot repeat irreversible external effects | `EVENT_MODEL.md` §7, EM-09; `COMMAND_MODEL.md` §8; ADR-0001/0002 | Covered conceptually |
| I-09 | Python failure cannot stop core transactional operations | `EVENT_MODEL.md` §8; `COMMAND_MODEL.md` §8 and CM-10 | Covered conceptually |
| I-10 | Unsupported exactly-once business-effect claims are rejected | `EVENT_MODEL.md` §5 and EM-10; ADR-0001 and ADR-0002 | Covered conceptually |

## Acceptance-criteria coverage

| Criterion | Coverage |
| --- | --- |
| Event and command model documents plus initial ADRs exist | `EVENT_MODEL.md`, `COMMAND_MODEL.md`, ADRs 0001–0002 |
| Canonical event envelope, aggregate identity, and monotonic versions are defined | `EVENT_MODEL.md` §§2–3 |
| Transactional outbox, at-least-once delivery, inbox/deduplication, quarantine, and deterministic replay are defined | `EVENT_MODEL.md` §§4–7 and ADR-0001 |
| Command idempotency, authorization rechecks, approval freshness, and durable receipts are defined | `COMMAND_MODEL.md` §§2–7 and ADR-0002 |
| Unsupported exactly-once claims are explicitly rejected | `EVENT_MODEL.md` §5 and EM-10; both ADRs |
| Broker and Python outage degradation is documented | `EVENT_MODEL.md` §8 and `COMMAND_MODEL.md` §8 |
| Requirement-to-invariant traceability and ADR review notes exist | This document §§3 and 6 |
| No runtime code, speculative infrastructure, concrete schemas, topics, SQL, or dependency decisions are introduced | Scope and clean-room checks below |

## Event-model review

- Events are defined as immutable facts after accepted state changes and are distinguished from commands.
- PostgreSQL is kept authoritative for transactional state; Redpanda is kept asynchronous transport and replay input.
- The outbox invariant, stable event identity, event schema version, aggregate version, and lack of global ordering are explicit.
- Duplicate, reordered, skipped, unsupported, malformed, cross-tenant, and conflicting-content cases have defined safe dispositions.
- Quarantine preserves investigation context without prescribing a queue, topic, retention period, or sensitive-data logging strategy.
- Replay is projection-focused, deterministic, version-aware, visible on failure, and prohibited from repeating irreversible effects.
- Event sourcing is explicitly not inferred from replayable projections.

## Command-model review

- Commands are controlled state-changing requests through Go, not direct intelligence or UI actions.
- The conceptual request fields include tenant, actor, target, expected version, normalized intent, timing, lineage, and approval reference.
- The lifecycle includes structural validation, current authorization, target-version checks, approval freshness, idempotency state, transactional execution, durable receipt, and repeated-outcome retrieval.
- Equivalent intent returns the durable outcome; conflicting intent under the same key is rejected.
- Authorization and approval are rechecked at execution and bound to current intent and state.
- Receipts distinguish the platform’s recorded decision from independently confirmed external completion.
- Downstream timeout and lost-response cases remain uncertain until reconciliation; blind irreversible retries are prohibited.

## Failure-case coverage

| Failure case | Required handling |
| --- | --- |
| Duplicate event | Stable-ID deduplication; no duplicate local effect |
| Reordered or skipped aggregate version | Detect and hold, quarantine, or reconcile; never silently skip |
| Poison or unsupported event | Quarantine with safe diagnostic context |
| Same event ID with conflicting content | Integrity violation and controlled investigation |
| Concurrent command retry | Scoped intent idempotency and durable outcome |
| Replayed command | Retrieve durable outcome; do not repeat the business effect |
| Stale or revoked approval | Reject or require renewed approval |
| Authorization changed after proposal | Recheck immediately before execution |
| Client timeout after successful execution | Retrieve the durable receipt |
| Downstream timeout with unknown outcome | Mark uncertain and reconcile before duplicate-capable retry |
| Partial downstream failure | Record local decision and attempt state; do not claim distributed atomicity |
| Broker outage | Preserve accepted transaction and outbox; allow stale async state and controlled backpressure |
| Python outage | Keep core Go-owned transactions available; mark intelligence unavailable or stale |
| Replay attempting external effects | Suppress, simulate, or separately reconcile effects |
| Cross-tenant context mismatch | Reject or quarantine; never apply under the wrong tenant |

## ADR review notes

| ADR | Decision reviewed | Alternatives and follow-up |
| --- | --- | --- |
| 0001 | PostgreSQL transactional outbox, asynchronous at-least-once delivery, stable IDs, consumer deduplication, quarantine, deterministic replay, and no unsupported exactly-once claim | Direct publish, broker authority, broker exactly-once, event sourcing, and dropping unsafe events are rejected. Concrete schemas, topics, retention, retries, and workers remain deferred. |
| 0002 | Business-intent idempotency, execution-time authorization, approval binding, durable receipts, and uncertain-outcome reconciliation | Request-only keys, blind retry, UI authorization, bearer approvals, distributed two-phase commit, and no receipts are rejected. Policy detail and external adapter protocols remain deferred. |

Both ADRs explicitly identify consequences, risks, and implementation choices that must not be invented by a later implementer.

## Consistency check against ARCHITECTURE.md

- PostgreSQL remains authoritative transactional and governance state; Redpanda remains asynchronous transport and replay input.
- Go remains owner of transactional behavior, authorization, commands, receipts, and replay coordination.
- Python remains advisory and cannot write business state, authorize, or execute commands.
- The product sequence remains validate/authorize → approve → recheck → execute.
- The documents do not add concrete API routes, schemas, topics, tables, credentials, libraries, or deployment topology.
- The logical publication rule does not assign a new process or broker credential and therefore does not rewrite the accepted architecture boundary.

## Clean-room confirmation

- No Ahoy repository or private Ahoy artifact was inspected or used.
- No private code, schema, migration, data, log, trace, screenshot, identifier, business rule, or production behavior was used.
- All terminology and examples are generic SeshatOps concepts or the existing fictional Northstar Foods context.
- No private identifier denylist was created.
- AI-assisted authoring is subject to maintainer review under `CLEAN_ROOM.md` §6; that review is recorded as a follow-up rather than implied by this document.

## Deferred decisions and owners

| Decision | Owner |
| --- | --- |
| Threat model, identity, roles/scopes, authorization matrix, data classification, and database roles | Issue #5 |
| Forecasting, RAG, proposal, and model/retrieval evaluation | Issue #6 |
| Reliability targets, capacity, fault campaigns, recovery, and measured evidence | Issue #7 |
| Roadmap, milestone integration, and evidence ledger | Issue #8 |
| Repository instructions, CI, concrete transports, schemas, code generation, dependencies, and deployment | Issue #9 |
| Integrated constitution review | Issue #10 |
| Concrete topic/partition/retention/retry choices, adapter contracts, archival policy, and recovery algorithms | Later implementation milestones |

## Residual risks

- No runtime implementation or failure campaign exists yet; these documents define required behavior only.
- The complete threat and authorization model is intentionally deferred to Issue #5.
- External systems may return uncertain outcomes that require adapter-specific reconciliation.
- Schema compatibility, retention, capacity, and operational thresholds remain unspecified by design.
- Manual documentation and clean-room review remain necessary until later repository governance work.

## Verification record

The following checks were run against the working tree:

| Check | Result |
| --- | --- |
| `git diff --check` | Pass; Git emitted only the existing LF-to-CRLF working-copy warnings for `README.md` and `ARCHITECTURE.md`. |
| Git status and changed-path allowlist | Pass after expanding untracked directories to individual files; exactly the five expected files plus the two allowed discoverability links were present. |
| Local Markdown-link target check | Pass for repository-relative links in the touched documents. |
| Markdown structure check | Pass; all five new documents have titles and substantive section structure. |
| Required event-field, command-field, and failure-case searches | Pass. |
| Unsupported-claim and clean-room category searches | Pass by manual review of all matches; rejection and deferral language was retained intentionally. |
| Runtime/build checks | Not applicable; no runtime code or services exist in this Issue #4 scope. |

An initial allowlist command incorrectly treated untracked directories as changed paths; the corrected check used Git's full untracked-file listing and passed. No commit, push, or pull request was created as part of Issue #4.
