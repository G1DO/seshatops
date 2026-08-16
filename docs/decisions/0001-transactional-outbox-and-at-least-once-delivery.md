# ADR-0001: Transactional Outbox and At-Least-Once Delivery

- **Status:** Accepted; Event Spine implements this for inventory and M3 lineage source hops
- **Date:** 2026-08-06

## Context

SeshatOps must preserve accepted business state during broker outages, tolerate
duplicate delivery, detect unsafe aggregate-version gaps, and support replayable
derived state. PostgreSQL is authoritative transactional state. Redpanda is
asynchronous transport and replay input.

## Decision

1. An authoritative PostgreSQL state change and its corresponding outbox intent are recorded atomically in one transaction.
2. Broker publication is asynchronous. An accepted source transaction is not considered lost because Redpanda is unavailable.
3. Outbox publication and downstream consumption are at least once. Retries and redelivery preserve the same logical event identity and content.
4. Consumers use stable event identity for inbox or equivalent deduplication. Deduplication state is committed atomically with the required local effect where applicable.
5. Same event identity with different content is an integrity violation, not a normal duplicate.
6. Missing aggregate versions, unsupported schemas, malformed envelopes, impossible transitions, and repeated processing failures are quarantined or routed for controlled reconciliation. Later versions are not silently applied across a required gap.
7. Replay is deterministic for replay-safe projections and must suppress, simulate, or separately reconcile irreversible external effects.
8. PostgreSQL remains transactional authority. Redpanda remains asynchronous transport and replay input; SeshatOps is not silently redefined as event sourced.
9. Exactly-once delivery and exactly-once business effects are not claimed.

Wire, checksum, and dispositions: [event-spine.md](../design/specifications/event-spine.md).

## Alternatives considered

### Direct broker publication in the source transaction

Rejected because a broker or network failure could leave accepted business
state without durable publication intent.

### Independent database and broker writes without an outbox

Rejected because a crash between the two writes creates an untracked
publication gap.

### Broker as the transactional system of record

Rejected. Redpanda is transport and replay input.

### Broker-provided exactly-once features

Rejected as a business guarantee. Transport-level features cannot prove
exactly-once effects across consumers, databases, retries, and external systems.

### Event sourcing for all business state

Rejected. Events support replayable projections; authoritative business state
is not reconstructed solely from the stream.

### Dropping events during outage or poison handling

Rejected. Quarantine and reconciliation are preferable to silent loss or silent
version skipping.
