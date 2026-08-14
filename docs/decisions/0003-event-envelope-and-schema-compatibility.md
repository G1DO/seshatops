# ADR-0003: Event Spine Event Envelope and Schema Compatibility

- **Status:** Accepted; JSON/JCS envelope is implemented in `event` and `northstar`
- **Date:** 2026-08-07

## Context

[ADR-0001](0001-transactional-outbox-and-at-least-once-delivery.md) requires
event identity, tenant context, aggregate ordering, lineage, at-least-once
delivery, and quarantine. It does not define a wire representation. Event Spine
needs one narrow contract without a schema registry.

## Decision

1. Event Spine uses strict UTF-8 JSON and the repository-owned contract in [event-spine.md](../design/specifications/event-spine.md). Duplicate object member names, non-JCS-compatible values, and non-integer or out-of-range v1 numeric forms are rejected before canonicalization.
2. The first event family is inventory.quantity_decremented for a one-line synthetic order.
3. The envelope contains the fields defined in [event-spine.md](../design/specifications/event-spine.md).
4. Event Spine accepts exactly event schema version 1; unknown fields, missing fields, invalid values, unknown event types, and unsupported versions are quarantined.
5. Event content identity is the SHA-256 of the JCS canonical JSON envelope without transport and processing metadata. The stored event bytes and Redpanda value are those canonical UTF-8 bytes.
6. The same event ID with different canonical content is an integrity failure, not a duplicate.
7. Any semantic or required-field change requires a new schema version and handler. No implicit compatibility or schema-registry service is introduced.

## Alternatives rejected

### Protobuf or Avro for the first slice

Rejected because code generation and compatibility tooling add scope without a
demonstrated requirement.

### A schema-registry service

Rejected because Event Spine has one event family and one supported version.

### Permissive additive compatibility inside version 1

Rejected because unknown fields and implicit defaults would make producer and
consumer interpretation less auditable.

### Event ID as content identity

Rejected because a stable identifier must still be checked against immutable
event content.
