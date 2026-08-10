# ADR-0004: M1 PostgreSQL Inbox and Projection Consistency

- **Status:** Accepted M1 decision; source/outbox persistence implemented in Issue #23; inbox/projection implementation pending
- **Date:** 2026-08-07
- **Scope:** Issue #21 source/outbox ownership, inbox identity, projection transactions, gaps, and failure records

## Context

M0 requires PostgreSQL authority, atomic source/outbox recording, at-least-once
delivery, stable identity, transactional deduplication, per-aggregate version
checks, quarantine, and deterministic replay. M1 needs concrete persistence
boundaries without introducing another database or implying a distributed
transaction with Redpanda.

## Decision

1. M1 uses one PostgreSQL database with logical erp and platform schemas and separate responsibilities.
2. The source transaction updates the accepted order, authoritative inventory, and immutable outbox row atomically.
3. The source-owned outbox relay publishes after commit and records published only after broker acknowledgement.
4. The relay preserves event bytes, event ID, content, and aggregate key across retries.
5. The platform inbox is unique on (consumer_name, event_id) and stores content hash, tenant, aggregate identity, version, and disposition. Structurally valid gap events retain canonical event bytes in a restricted field for re-drive; malformed raw input is not stored.
6. Matching duplicate content is a durable no-op only for an already applied or duplicate disposition; a matching `quarantined_gap` event remains eligible for re-drive. Conflicting content is quarantined.
7. Inbox state and inventory projection changes commit in one PostgreSQL transaction.
8. The inventory projection is keyed by (tenant_id, item_id) and applies only contiguous aggregate versions with valid payload arithmetic.
9. Missing versions are quarantined with expected and received versions and are not silently skipped. Contiguous `quarantined_gap` versions may be re-driven from retained canonical event bytes after the gap is filled.
10. Processing failures retain sanitized identity, category, source position, nullable canonical hash when available, attempt, and status information; raw payloads and secrets are excluded.
11. The consumer commits an offset only after the durable processing or quarantine transaction commits.

The concrete fields, topic, retry rules, failure categories, and checksum are
defined in CONTRACTS.md.

## Consequences

- Accepted source state cannot disappear because Redpanda is unavailable.
- Consumer restarts and uncertain acknowledgements may redeliver events safely.
- Duplicate and conflicting identities become explicit durable decisions.
- A partition may experience bounded delay during transient failures, while
  deterministic poison and gap cases become durable quarantine decisions.
- M1 does not provide an operator recovery UI or claim exactly-once behavior.

## Alternatives rejected

### Direct publish inside the source transaction

Rejected because broker availability cannot be part of the PostgreSQL source
commit without a distributed transaction.

### Separate database for source and platform state

Rejected for M1 because one PostgreSQL database with logical schemas and scoped
credentials is sufficient to demonstrate separate ownership without adding an
extra database dependency.

### Projection-only deduplication

Rejected because inbox identity and the local projection effect must commit as
one durable decision.

### Auto-commit or acknowledgement before commit

Rejected because a restart could lose the processing decision while the broker
believes the event was consumed.

### Silent gap skipping

Rejected because applying a later aggregate version without its predecessor can
produce an unsafe projection and breaks deterministic reconstruction.

## Verification route and limitations

The M1 contract review must verify transaction-boundary wording, uniqueness,
acknowledgement order, failure dispositions, checksum scope, and no extra
runtime infrastructure. Runtime integration and fault tests belong to later
M1 implementation issues.
