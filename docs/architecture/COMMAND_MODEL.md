# Command Model — SeshatOps Correctness Principles

**Status:** Planned conceptual contract for Issue #4. This document defines principles that future implementations must preserve. It is not an API schema, database schema, authorization matrix, runtime implementation, or production evidence.

**Owns:** Command intent, lifecycle, idempotency, execution-time authorization and approval checks, durable receipts, and uncertain downstream outcomes.

**Does not own:** Concrete API routes, payload types, database indexes, locking algorithms, libraries, external adapter contracts, the complete threat model, or role and scope policy details owned by Issue #5.

## 1. Purpose and scope

A command is a controlled request to perform a state-changing action. It is not an event, a prediction, a UI action, or a free-form instruction emitted directly by intelligence.

The Go-owned transactional boundary is authoritative for validation, authorization, state transitions, command execution, durable receipts, and the decision about whether an action is complete, failed, or uncertain.

| Concept | Meaning |
| --- | --- |
| Command | A request to attempt a controlled state change |
| Event | An immutable fact emitted after an accepted state change; see [EVENT_MODEL.md](EVENT_MODEL.md) |
| Proposal | Advisory typed output that may inform a command but cannot authorize or execute one |
| Receipt | Durable evidence of the platform's recorded decision and observed result |

## 2. Conceptual command request

The following fields define conceptual meaning and invariants only. No concrete wire representation is prescribed.

| Field | Purpose and invariants |
| --- | --- |
| `command_id` | Stable identity of the command record or logical execution decision. It must remain attributable across retries and receipt retrieval. |
| `idempotency_key` | Identity of one business intent. It is scoped with tenant and relevant command context; it is not merely a transport or HTTP request identifier. |
| `command_type` | Semantic action being requested. It must be recognized and allowed by current policy. |
| `tenant_id` | Tenant context in which the command is evaluated and executed. Cross-tenant context mismatch is rejected. |
| `actor` or principal reference | Identity of the requesting or approving principal, or an authorized service principal where applicable. Possession of a reference does not establish authorization. |
| target type | Conceptual kind of resource or aggregate to change. |
| target identifier | Identifier of the target within the tenant and target type. |
| expected target version | Version the caller believes it is acting on. A changed current version can invalidate the command, proposal, or approval and requires renewed validation. |
| normalized intent or input digest | Canonical representation or digest of the material business intent used for idempotency conflict detection. Equivalent intent must normalize equivalently; changed intent must not. |
| `requested_at` | Recorded time at which the command was requested. It is evidence, not a substitute for current authorization or state checks. |
| `expires_at` where applicable | Freshness boundary after which the command or approval cannot be used without renewed validation. |
| `correlation_id` | Lineage identifier connecting the command to the originating workflow, event, proposal, approval, and receipt. |
| `causation_id` | Identifier of the immediate event, proposal, or workflow action that caused the command request where applicable. |
| approval reference where applicable | Reference to the approval decision. It must be bound to the intended command, target, important parameters, tenant, and relevant target version. |

Sensitive credentials and unbounded free-form instructions are not implied by this conceptual envelope.

## 3. Command lifecycle

The logical lifecycle is:

1. **Receive** the request and establish its request context.
2. **Validate structurally** that the command can be interpreted safely.
3. **Resolve tenant and actor context** from trusted server-side context.
4. **Check current authorization** for the actor, tenant, action, target, and scope.
5. **Check target state and expected version** so stale state is not silently overwritten.
6. **Validate required approval and freshness** against the intended command and current target state.
7. **Claim or retrieve idempotency state** for the scoped business intent.
8. **Execute through the Go-owned transactional boundary**, including the final authorization and state checks immediately before the transition.
9. **Persist the resulting state and durable receipt** together where the local effect requires atomic recording.
10. **Return or retrieve the same outcome** for valid repetitions of the same intent.

Display-time authorization, an earlier proposal check, or an earlier approval does not replace execution-time authorization. A command that cannot establish the required durable decision remains failed or uncertain and must not be reported as silently successful.

## 4. Business-intent idempotency

The idempotency key represents one business intent, not one network attempt.

- Its scope includes tenant and the relevant command context, such as command type and target where required to distinguish intents safely.
- The normalized intent or input digest is part of the conflict check.
- The same scoped key with equivalent normalized intent returns the original durable outcome or its current durable status.
- The same scoped key with different intent is rejected as an idempotency conflict. It must not overwrite or reuse the original command record.
- Concurrent repetitions of an equivalent intent cannot create multiple business effects.
- Idempotency state and the durable outcome survive process restart and client disconnect.
- A client timeout after successful execution is an uncertain client observation, not permission to execute blindly again. The client retrieves the durable receipt using the same intent.
- Repeated valid requests produce one logical durable business effect even though transport attempts may occur more than once.
- This does not claim exactly-once network delivery, external execution, or distributed atomicity.

## 5. Authorization and approval rechecks

- Authorization is enforced by the Go-owned transactional boundary.
- Authorization is rechecked immediately before execution using current tenant, actor, action, target, scope, policy, and state context.
- An approval must be bound to the intended command, target, tenant, important parameters, normalized intent, and relevant target version.
- Expired, revoked, altered, or stale approvals cannot authorize execution.
- A changed target version may require renewed validation or approval; the command must not silently act on an obsolete decision.
- Python output cannot authorize a command or approval.
- UI state cannot authorize a command or approval.
- Possession of an idempotency key, command identifier, approval reference, or receipt reference cannot authorize a command.

Issue #5 owns the detailed threat model, identity integration, role model, scopes, policy language, and database-role design. This document fixes timing and binding principles only.

## 6. Durable command receipts

A durable receipt is evidence of the platform's recorded decision and observed result. It is not proof that every independent external system completed successfully unless that result was independently confirmed.

The conceptual receipt can identify:

| Information | Purpose |
| --- | --- |
| command identity | `command_id` and a safe reference to the idempotency key |
| tenant and command | tenant, command type, target type, and target identifier |
| intent | normalized intent digest used for conflict detection |
| execution status | final or current status, including failure or uncertainty categories |
| result | resulting aggregate or effect reference and resulting version where applicable |
| authority | actor and approval references, without treating references as authority themselves |
| lineage | correlation and causation identifiers |
| timing | request, decision, execution, observation, and receipt timestamps as applicable |
| reconciliation | required reconciliation state and outcome when downstream completion is uncertain |

Repeated valid requests retrieve the same durable business outcome or current durable status. A receipt must not hide an unresolved uncertainty behind a generic success label.

## 7. Partial downstream failure and uncertain outcomes

Independent systems do not share a claimed distributed atomic transaction merely because one command initiated the work.

- Record the local decision and the external-attempt state durably.
- Use downstream idempotency support where available, preserving the same logical command intent across retries.
- Treat timeouts, lost responses, connection failures after send, and ambiguous acknowledgements as uncertain outcomes until reconciled.
- Reconcile the downstream state before issuing a retry that could duplicate an irreversible effect.
- Never silently report success without durable evidence of the result being claimed.
- Never silently issue repeated irreversible external effects because a client or network response was lost.
- A terminal failure, uncertainty, or reconciliation requirement remains visible in the durable receipt and audit lineage.

The specific adapter protocol, reconciliation query, retry schedule, and external retention policy are deferred implementation choices.

## 8. Replay and degraded behavior

Replay of event projections must not reissue commands, approvals, notifications, or other irreversible external operations. Side effects are suppressed, simulated, or separately reconciled.

During a broker outage, command execution must not pretend that asynchronous projections are current. It may continue only when the required authoritative state and policy inputs are available and the command can be durably recorded. Controlled rejection is preferable to an unrecorded decision.

During a Python outage, existing Go-owned authorization and core transactional operations remain available. New forecasts, explanations, retrieval results, and proposals may be unavailable or explicitly stale. Go must not require a synchronous Python response to complete a core transaction, and Python failure cannot grant command authority.

## 9. Command-model invariants

| ID | Invariant |
| --- | --- |
| CM-01 | A command is a controlled request; proposals, UI state, and events cannot execute directly. |
| CM-02 | Equivalent repetitions of one scoped business intent return one durable business outcome. |
| CM-03 | The same idempotency key with different normalized intent is rejected as a conflict. |
| CM-04 | Concurrent retries and process restarts cannot create multiple effects for one equivalent intent. |
| CM-05 | Authorization is rechecked immediately before execution by Go. |
| CM-06 | Expired, revoked, altered, or stale approval cannot authorize changed intent or target state. |
| CM-07 | A durable receipt records the platform decision and observed result without overclaiming external completion. |
| CM-08 | Timeouts and lost downstream responses remain uncertain until reconciled. |
| CM-09 | Reconciliation precedes any retry that could duplicate an irreversible downstream effect. |
| CM-10 | Python failure cannot stop core transactional operations or grant authority. |

## 10. Deferred implementation choices

Concrete API routes, payload schemas, indexes, unique constraints, lock or claim algorithms, receipt storage, downstream adapter contracts, retry and reconciliation algorithms, approval policy details, and observability thresholds remain open. They must preserve the invariants above and the full authorization model defined by Issue #5.
