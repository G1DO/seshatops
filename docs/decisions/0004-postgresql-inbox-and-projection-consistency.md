# ADR-0004: Event Spine PostgreSQL Inbox and Projection Consistency

- **Status:** Accepted; Event Spine implements this for inbox and inventory projection
- **Date:** 2026-08-07

## Context

[ADR-0001](0001-transactional-outbox-and-at-least-once-delivery.md) requires
PostgreSQL authority, atomic source/outbox recording, at-least-once delivery,
transactional deduplication, per-aggregate version checks, and quarantine.
Event Spine needs persistence boundaries without a second database or a
distributed transaction with Redpanda.

## Decision

1. One PostgreSQL database with logical `erp` and `platform` schemas.
2. The source transaction records the accepted source hop (lineage or one-line order), any required inventory update, and the immutable outbox row atomically.
3. The source-owned outbox relay publishes after commit and records published only after broker acknowledgement.
4. The platform inbox is unique on `(consumer_name, event_id)`. Inbox state and inventory projection changes commit in one transaction.
5. Matching duplicate content is a durable no-op. Conflicting content and missing aggregate versions are quarantined, not silently skipped.
6. The consumer commits an offset only after the durable processing or quarantine transaction commits.

Concrete fields, topic, retry rules, failure categories, and checksum:
[event-spine.md](../design/specifications/event-spine.md).

## Alternatives rejected

### Direct publish inside the source transaction

Rejected because broker availability cannot be part of the PostgreSQL source
commit without a distributed transaction.

### Separate database for source and platform state

Rejected for Event Spine; one database with logical schemas is enough.

### Projection-only deduplication

Rejected because inbox identity and the local projection effect must commit as
one durable decision.

### Auto-commit or acknowledgement before commit

Rejected because a restart could lose the processing decision while the broker
believes the event was consumed.

### Silent gap skipping

Rejected because applying a later aggregate version without its predecessor
produces an unsafe projection.
