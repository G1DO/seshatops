# ADR-0002: Idempotent Command Execution and Durable Receipts

- **Status:** Accepted design principle; implementation pending
- **Date:** 2026-08-06
- **Scope:** Issue #4 command intent, authorization rechecks, approvals, receipts, and downstream uncertainty

## Context

SeshatOps executes only authorized, human-approved actions through the Go-owned transactional boundary. Clients, workers, networks, and external systems can retry, restart, time out, or lose responses. A command may therefore be delivered more than once, and a response may be missing even when the requested action succeeded.

The platform needs one durable business outcome for equivalent repetitions without claiming distributed exactly-once execution. It must also prevent stale approvals, changed targets, Python output, UI state, or possession of an identifier from becoming authority.

## Decision

1. The idempotency key represents one scoped business intent, including tenant and relevant command context.
2. Equivalent normalized intent under the same scoped key returns the original durable outcome or current durable status.
3. The same scoped key with different normalized intent is rejected as an idempotency conflict.
4. Concurrent repetitions cannot create multiple business effects, and idempotency state survives process restart.
5. Go rechecks current authorization immediately before execution.
6. Required approvals are bound to the tenant, intended command, target, important parameters, normalized intent, and relevant target version. Expired, revoked, altered, or stale approvals cannot authorize execution.
7. The platform records a durable command receipt containing the decision, observed result, lineage, authority references, and uncertainty or reconciliation state.
8. External systems are not treated as part of a distributed atomic transaction. Timeouts and lost responses are uncertain until reconciled.
9. A retry that could duplicate an irreversible downstream effect is not issued blindly; reconciliation precedes that retry.
10. Python output, UI state, approval possession, and idempotency-key possession cannot authorize or execute a command.

This ADR is the command-execution decision. Implementation waits for
Approved Actions (ADR-Q-007). Do not treat this file as a current runtime
surface.

## Consequences

### Benefits

- Client retries and process restarts can retrieve a durable result without repeating the intended business effect.
- Changed intent under a reused key becomes an explicit conflict instead of an ambiguous mutation.
- Authorization and approval remain current at the point of execution.
- Operators can distinguish success, terminal failure, and uncertainty from a durable receipt.
- External ambiguity is handled through reconciliation rather than silent success or blind retry.

### Costs and trade-offs

- Idempotency state and receipts require durable storage and lifecycle management.
- Intent normalization must be stable enough to distinguish equivalent and changed requests.
- Authorization and approval checks occur again at execution, adding decision complexity.
- Downstream adapters need reconciliation capabilities or an explicit inability to confirm outcomes.
- A command may remain visibly uncertain until an operational decision resolves it.

## Alternatives considered

### Idempotency per HTTP request only

Rejected because one business intent can be represented by multiple requests, transports, or process attempts. Request identity alone does not protect the business effect.

### Blind retry after timeout

Rejected because a timeout does not establish that the external effect failed. Blind retry can repeat an irreversible action.

### UI-owned or proposal-time authorization

Rejected because display state and earlier checks can become stale. Go must enforce current authorization immediately before execution.

### Approval as an unbound bearer token

Rejected because an approval detached from target, parameters, intent, tenant, or version could authorize a changed action.

### Distributed two-phase commit across external systems

Rejected for the public correctness model. Independent systems do not provide a universal shared transaction boundary, and the adapters and failure semantics are not defined here.

### No durable receipt

Rejected because a client timeout or process restart would force an unsafe guess about whether the command already took effect.

### Exactly-once command execution as a product claim

Rejected without evidence across the Go boundary, retries, external systems, and reconciliation. The decision is one durable business outcome for equivalent intent, not a claim that every network or delivery layer executes once.

## Risks

- Incorrect intent normalization can create false equivalence or false conflicts.
- Poorly scoped keys can permit cross-tenant collisions or unintended reuse.
- Receipt retention and redaction require later governance decisions.
- External systems may not support idempotency or reliable reconciliation.
- A command can remain uncertain for an operationally significant period.
- Authorization, approval, and target-version policy remain incomplete until Issue #5.

## Deferred implementation choices

Concrete API shapes, storage schema, uniqueness constraints, locking or claim algorithms, receipt retention, approval policy details, identity integration, external adapter protocols, reconciliation queries, retry schedules, and operational thresholds remain open. Issue #5 owns the detailed authorization model; Issue #7 owns reliability evidence; completed Issue #9 established repository workflow and documentation CI; Approvals and later capability sequences own the corresponding runtime choices.
