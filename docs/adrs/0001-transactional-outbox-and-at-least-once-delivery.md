# ADR-0001: Transactional Outbox and At-Least-Once Delivery

- **Status:** Accepted design principle with accepted M1 amendment; source/outbox persistence implemented in Issue #23; source-owned relay publication implemented for M1 library/integration scope in Issue #24; projection consumer pending
- **Date:** 2026-08-06
- **Scope:** Issue #4 event publication, consumption, quarantine, and replay correctness

## Context

SeshatOps must preserve accepted business state during broker outages, tolerate duplicate delivery, detect unsafe aggregate-version gaps, and support replayable derived state. The repository architecture establishes PostgreSQL as authoritative transactional state and Redpanda as asynchronous transport and replay input. The product constitution requires at-least-once delivery with idempotent business effects and explicitly rejects exactly-once delivery marketing.

The system must also remain a clean-room public platform. This ADR therefore records principles without introducing a broker configuration, event schema, database table, topic, process layout, or dependency choice.

## Decision

1. An authoritative PostgreSQL state change and its corresponding outbox intent are recorded atomically in one transaction.
2. Broker publication is asynchronous. An accepted source transaction is not considered lost because Redpanda is unavailable.
3. Outbox publication and downstream consumption are at least once. Retries and redelivery preserve the same logical event identity and content.
4. Consumers use stable event identity for inbox or equivalent deduplication. Deduplication state is committed atomically with the required local effect where applicable.
5. Same event identity with different content is an integrity violation, not a normal duplicate.
6. Missing aggregate versions, unsupported schemas, malformed envelopes, impossible transitions, and repeated processing failures are quarantined or routed for controlled reconciliation. Later versions are not silently applied across a required gap.
7. Replay is deterministic for replay-safe projections and must suppress, simulate, or separately reconcile irreversible external effects.
8. PostgreSQL remains transactional authority. Redpanda remains asynchronous transport and replay input; SeshatOps is not silently redefined as event sourced.
9. The platform makes no unsupported claim of exactly-once delivery or exactly-once business effects.

The full conceptual contract is in [EVENT_MODEL.md](../architecture/EVENT_MODEL.md).
The concrete M1 contract is in [CONTRACTS.md](../../CONTRACTS.md).

## M1 concrete amendment from ADR-Q-002

For the first M1 event family, the source-owned relay publishes to the
`seshatops.m1.events` topic using the canonical aggregate key
`tenant_id/aggregate_type/aggregate_id`. It publishes the exact immutable JSON
bytes recorded in the `erp` outbox and marks the row `published` only after a
broker acknowledgement.

Outbox states are `pending`, `publishing`, `published`, and `quarantined`.
Transient publication failures use a 1-second exponential backoff capped at 60
seconds. A crash after broker acknowledgement and before the PostgreSQL update
may publish a duplicate with the same event identity and content.
An expired `publishing` lease is reclaimable for retry; a live lease
is not claimed by another relay worker.

The Go consumer uses manual offset commits and acknowledges only after the
required `platform` inbox, projection, or quarantine transaction commits.
Deterministic poison, schema, tenant, content-conflict, and version-gap cases
become durable quarantine decisions rather than silent skips. These choices do
not change the at-least-once delivery model or introduce an exactly-once claim.

## Consequences

### Benefits

- Accepted source transactions remain durable when the broker is down.
- Duplicate publication and delivery become expected, testable behaviors rather than data-loss assumptions.
- Consumers can protect local business effects with stable event identity.
- Unsafe events become visible investigation and reconciliation cases.
- Projection rebuilds can be deterministic without treating the transport as the system of record.
- The consistency claim stays honest and evidence-compatible.

### Costs and trade-offs

- Outbox records and quarantine cases require durable storage, monitoring, and eventual operational decisions.
- Consumers must implement idempotent behavior and aggregate-version checks.
- Publication lag can make projections, notifications, and intelligence inputs stale.
- Replay requires versioned handler logic and identifiable reference inputs.
- External side effects need separate suppression or reconciliation paths during replay.
- Backpressure or temporary rejection may be necessary when safe durable capacity is exhausted.

## Alternatives considered

### Direct broker publication in the source transaction

Rejected as the primary correctness model because a broker or network failure could leave accepted business state without durable publication intent. A broker acknowledgement is not a substitute for the authoritative transaction.

### Independent database and broker writes without an outbox

Rejected because a process crash between the two writes creates an untracked publication gap that cannot be resolved reliably from the accepted state alone.

### Broker as the transactional system of record

Rejected. Redpanda is transport and replay input; PostgreSQL owns authoritative transactional state and governance records.

### Broker-provided exactly-once features

Rejected as an unsupported business guarantee. Transport-level features cannot by themselves prove exactly-once effects across consumers, databases, external systems, retries, and human workflows.

### Event sourcing for all business state

Rejected for Issue #4. Events support replayable projections and integrations, but authoritative business state is not silently reconstructed solely from the event stream.

### Dropping events during outage or poison handling

Rejected. Controlled backpressure, quarantine, and reconciliation are preferable to silent loss or silent version skipping.

## Risks

- Outbox growth or broker lag can create stale projections and operational pressure.
- Quarantine can become a data graveyard without ownership and reconciliation procedures.
- Incorrect deduplication can suppress legitimate events or apply duplicates.
- Handler changes or mutable reference data can undermine replay determinism.
- An external side effect may be uncertain even when local event processing succeeds.
- Event contract evolution can create unsupported versions if later compatibility governance is weak.

These risks require later operational evidence and implementation-specific controls; this ADR does not invent thresholds or algorithms.

## Deferred implementation choices

Retention and archival, partition sizing, publisher/consumer process layout,
alerting, credentials, libraries, deployment topology, and later event families
remain open. Issue #7 owns reliability evidence; completed Issue #9 established
repository workflow and documentation CI; M1 and later milestones own the
corresponding runtime choices.
