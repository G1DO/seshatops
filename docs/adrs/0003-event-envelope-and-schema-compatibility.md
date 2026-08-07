# ADR-0003: M1 Event Envelope and Schema Compatibility

- **Status:** Accepted M1 decision; JSON/JCS envelope library implemented in Issue #22 (`event`, `northstar`); outbox/broker/projection runtime remains pending
- **Date:** 2026-08-07
- **Scope:** Issue #21 event identity, first event family, JSON representation, and schema compatibility

## Context

M0 defines the required event identity, tenant context, aggregate ordering,
lineage, at-least-once delivery, quarantine, and no-exactly-once claim. It does
not define a wire representation or compatibility policy. M1 needs one narrow
contract that can be implemented without a schema registry or future-domain
breadth.

## Decision

1. M1 uses strict UTF-8 JSON and the repository-owned contract in CONTRACTS.md. Duplicate object member names, non-JCS-compatible values, and non-integer or out-of-range v1 numeric forms are rejected before canonicalization.
2. The first event family is inventory.quantity_decremented for a one-line synthetic order.
3. The envelope contains stable event, tenant, event type, schema version, aggregate, time, producer, lineage, trace, and payload fields defined in CONTRACTS.md.
4. M1 accepts exactly event schema version 1; unknown fields, missing fields, invalid values, unknown event types, and unsupported versions are quarantined.
5. Event content identity is the SHA-256 of the JCS canonical JSON envelope without transport and processing metadata. The stored event bytes and Redpanda value are those canonical UTF-8 bytes.
6. The same event ID with different canonical content is an integrity failure, not a duplicate.
7. Any semantic or required-field change requires a new schema version and handler. No implicit compatibility or schema-registry service is introduced.

## Consequences

- The first implementation is inspectable and dependency-light.
- Go and TypeScript can share human-readable contract examples and test vectors.
- Strict versioning makes incompatibility visible instead of silently changing interpretation.
- Later schemas require explicit review and handler support.
- JSON canonicalization and validation must be tested when runtime work begins.

## Alternatives rejected

### Protobuf or Avro for the first slice

Rejected for M1 because code generation and compatibility tooling add immediate
scope without a demonstrated requirement.

### A schema-registry service

Rejected because M1 has one event family and one supported version; a registry
would add an extra service and operational dependency.

### Permissive additive compatibility inside version 1

Rejected because unknown fields and implicit defaults would make producer and
consumer interpretation less auditable for the first contract.

### Event ID as content identity

Rejected because a stable identifier must still be checked against immutable
event content to detect tampering or conflicting publication.

## Verification route and limitations

The M1 contract review must verify required-field coverage, canonicalization
rules, rejection dispositions, and absence of unsupported exactly-once claims.
No runtime compatibility result is claimed by this ADR.
