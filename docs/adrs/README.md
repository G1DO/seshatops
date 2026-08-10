# Architecture Decision Records

This directory contains accepted technical decisions and a bounded queue of
decisions that later milestones must resolve. An ADR records a reviewed
decision; an `ADR-Q-*` entry records an unresolved decision without selecting
an implementation.

## Accepted ADRs

| ADR | Decision | Status | Owner |
| --- | --- | --- | --- |
| [ADR-0001](0001-transactional-outbox-and-at-least-once-delivery.md) | Transactional outbox and at-least-once delivery | Accepted design principle; Issue #23 source/outbox persistence implemented; relay pending | M1 / M7 evidence |
| [ADR-0002](0002-idempotent-command-execution.md) | Idempotent command execution and durable receipts | Accepted design principle; implementation pending | M5 / M7 evidence |
| [ADR-0003](0003-event-envelope-and-schema-compatibility.md) | M1 event envelope and schema compatibility | Accepted M1 decision; Issue #22 library implemented; runtime pending | M1 / M7 evidence |
| [ADR-0004](0004-postgresql-inbox-and-projection-consistency.md) | M1 PostgreSQL inbox and projection consistency | Accepted M1 decision; Issue #23 source/outbox implemented; inbox/projection pending | M1 / M7 evidence |

Accepted ADRs preserve the M0 boundaries: PostgreSQL remains transactional
authority, asynchronous delivery is at least once, replay must be safe for
irreversible effects, and no exactly-once network or broker claim is made.

## Deferred decision queue

Queue entries are not decisions until their disposition is recorded. Each open
entry must be converted into a reviewed ADR by its owning milestone before the
relevant implementation or evidence campaign begins. Resolved entries remain
visible for traceability.

| Queue ID | Decision area | Earliest owner | Trigger | Fixed M0 constraints | Disposition |
| --- | --- | --- | --- | --- | --- |
| ADR-Q-001 | Event envelope and schema compatibility | M1 | Before the first event contract is implemented | Stable event identity, tenant, aggregate/version, schema, time, and trace context are required. | Resolved by ADR-0003 |
| ADR-Q-002 | Outbox, transport, delivery, and consumer configuration | M1 | Before the outbox relay and consumer are implemented | Durable outbox, asynchronous transport, at-least-once delivery, deduplication, quarantine, and replay remain required. | Resolved by the M1 amendment to ADR-0001 |
| ADR-Q-003 | Persistence, inbox, deduplication, and projection constraints | M1 | Before event, inbox, or projection storage is implemented | Transactional authority, deterministic projection rebuilds, version checks, and no silent gap skipping remain required. | Resolved by ADR-0004 |
| ADR-Q-004 | Identity, tenant visibility, policy, and service delegation | M2 | Before identity or operational enforcement is implemented | Default deny, tenant isolation, least privilege, and Go-owned authorization remain required. | Open |
| ADR-Q-005 | Replay, recovery, restore, and checksum behavior | M3 | Before recovery implementation or restore evidence begins | Replay must be deterministic where applicable, preserve lineage, and suppress or reconcile irreversible effects. | Open |
| ADR-Q-006 | Forecasting features, models, thresholds, and temporal evaluation | M4 | Before forecasting implementation or evaluation begins | Temporal leakage controls, uncertainty, abstention, lineage, and reproducibility remain required. | Open |
| ADR-Q-007 | Command, approval, adapter, retry, reconciliation, and receipt contracts | M5 | Before approved-action execution is implemented | Human approval, execution-time authorization, idempotent effects, durable receipts, and explicit uncertainty remain required. | Open |
| ADR-Q-008 | Governed retrieval, citation, refusal, and output validation | M6 | Before governed retrieval is implemented or evaluated | Permission-aware retrieval, citation, refusal, injection resistance, and typed non-authoritative proposals remain required. | Open |
| ADR-Q-009 | Telemetry, SLOs, fault campaigns, deployment, backup, and cost evidence | M7 | Before reliability, deployment, or performance claims are made | Evidence must be environment-scoped, reproducible, limitation-aware, and free of invented M0 targets. | Open |
| ADR-Q-010 | Rust admission and measurement gate | M7 | When a measured performance or specialized-component need is demonstrated | Rust remains measurement-gated; no speculative M0 component or workspace is authorized. | Open |
| ADR-Q-011 | Release, claim reconciliation, public evidence, and clean-room release review | M8 | Before public release or portfolio claims | Public claims must link to evidence, remain within the ledger, and use no Ahoy or private dependency. | Open |

## Queue governance

- The earliest owning milestone resolves a queue item; supporting milestones
  may contribute constraints or evidence.
- A queue item may not be promoted to an accepted ADR by implication from a
  planning page, vendor suggestion, or implementation convenience.
- New decisions must preserve the product, architecture, correctness,
  security, clean-room, and evidence invariants already reviewed for M0.
- Deferred items remain visible until an accepted ADR links the decision,
  consequences, verification route, and remaining limitations.
