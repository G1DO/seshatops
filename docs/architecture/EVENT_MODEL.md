# Event Model — SeshatOps Correctness Principles

**Status:** Planned conceptual contract from Issue #4. This document defines
principles that future implementations must preserve. The concrete M1 contract is in [`CONTRACTS.md`](../../CONTRACTS.md).
Executable parse, validation, JCS canonicalization, and content-hash helpers
for that contract live in the Go package `event`. Neither the documents nor the
library prove transport, projection, or production readiness.

**Owns:** Event meaning, authority boundaries, identity, aggregate ordering, publication and consumption guarantees, quarantine, replay, and event-related degraded behavior.

**Does not own:** Later event schemas, topics, partitions, retention, worker
algorithms, dependency choices, deployment topology, the full threat model, or
measured reliability targets. M1's first JSON event, topic, key, persistence,
retry, failure, and checksum decisions are owned by [`CONTRACTS.md`](../../CONTRACTS.md).

## 1. Purpose and scope

SeshatOps uses events to communicate accepted changes and to rebuild derived operational views. An event is an immutable fact about a state change; it is not an instruction to perform a state change.

This model deliberately does not redefine SeshatOps as an event-sourced system. PostgreSQL remains authoritative for transactional state. Events support projections, integrations, audit lineage, and controlled replay, while authoritative business state remains owned by its transactional boundary.

### Events and commands

| Concept | Meaning | Authority |
| --- | --- | --- |
| Event | An immutable fact emitted after an accepted transactional state change | Records what happened; does not grant permission to change state |
| Command | A controlled request to perform a state-changing action | Must be validated, authorized, approved where required, and executed through Go |
| Projection | Derived state built from accepted events | Useful for reads and workflows, but not a replacement for authoritative transactional state |
| Replay | Reprocessing retained events to rebuild or verify derived state | Must not silently repeat irreversible external effects |

### Authority and transport

- PostgreSQL is the authoritative transactional store for SeshatOps platform state.
- Redpanda is asynchronous event transport and replay input. It is not the transactional system of record and does not own authorization.
- A source transaction and its corresponding outbox intent are committed atomically in the same PostgreSQL transaction where that authoritative boundary uses PostgreSQL.
- Publication occurs asynchronously after the source transaction is accepted. No broker response is required to make an accepted source transaction durable.
- The logical owner of authoritative state owns the durable outbox intent. A publisher may be a separate asynchronous component; this document does not select a process, credential, topic, or framework.
- No distributed transaction across an ERP, PostgreSQL, and Redpanda is implied.

## 2. Conceptual event envelope

The following is a conceptual contract. It describes purpose and invariants without defining concrete serialization, validation syntax, or language types.

| Field | Purpose and invariants |
| --- | --- |
| `event_id` | Stable identity of one logical event. It is the basis for deduplication and remains unchanged across publication retries, redelivery, and controlled re-publication. Re-publication must not create a new logical identity. |
| `tenant_id` | Tenant context for the fact. Consumers must validate that the tenant context agrees with the aggregate, authorization boundary, and processing context. |
| `event_type` | Semantic name of the fact. It must be recognized or safely quarantined; it is not a command name. |
| `event_schema_version` | Version of the event contract and payload interpretation. It is distinct from `aggregate_version` and must not be used as a broker offset or business-state sequence. |
| `aggregate_type` | Stable conceptual type of the aggregate whose state changed. |
| `aggregate_id` | Identifier of the aggregate within the tenant and aggregate type. Together with tenant and type it forms aggregate identity. |
| `aggregate_version` | Monotonic business-state sequence for this aggregate. It expresses per-aggregate ordering, not global ordering, wall-clock order, or publication order. |
| `occurred_at` | Recorded time at which the business fact occurred or the accepted change took place. It is part of the event record and must not be silently replaced by the replay-time clock. |
| `recorded_at` | Recorded time at which the event or outbox intent was durably recorded. It is distinct from business occurrence time. |
| `producer` | Identity of the logical producer or authoritative boundary that recorded the fact. It supports provenance and validation; it does not by itself grant consumer authority. |
| `correlation_id` | Stable lineage identifier connecting related events, commands, workflow decisions, and traces. |
| `causation_id` | Identifier of the immediate command or event that caused this fact where applicable. It must remain stable when the fact is retried or republished. |
| `payload` | Immutable business data required to interpret the fact. Its meaning is governed by `event_type` and `event_schema_version`; no concrete payload schema is defined here. |
| `metadata` or trace context | Technical lineage and observability context needed for investigation. It must not become an unreviewed channel for sensitive data or executable instructions. |

An event's logical identity includes its stable identifier and the content needed to interpret that identity. The same `event_id` with materially different envelope or payload content is an integrity violation, not an ordinary duplicate.

## 3. Aggregate identity and ordering

An aggregate is the tenant-scoped unit whose state transitions are versioned together. Conceptually, its identity is the combination of `tenant_id`, `aggregate_type`, and `aggregate_id`.

- Each accepted state-changing fact for an aggregate advances its monotonic `aggregate_version` according to the aggregate owner's state transition rules.
- Ordering expectations apply within one aggregate. There is no implied total or global order across aggregates.
- A consumer may observe a duplicate version, a stale version, a missing version, or a later version before an earlier required version.
- A duplicate already durably accepted may be treated as a replay of the same logical fact, provided its content matches the recorded identity.
- A lower or repeated version with conflicting content is an integrity failure.
- A later version must not be silently applied when an earlier required version is missing. The consumer must hold, quarantine, or route the case for reconciliation according to an explicit operational decision.
- Reordered events must not be treated as valid merely because the broker delivered them.
- Detection and recovery algorithms, buffering periods, retry counts, and operational thresholds are implementation decisions outside this document.

## 4. Transactional outbox

The core invariant is:

> A transaction that changes authoritative business state records its corresponding outbox entry atomically in the same PostgreSQL transaction.

The outbox entry represents durable publication intent for the logical event. It does not mean that the broker has accepted or delivered the event.

- A committed source transaction and its outbox intent succeed or fail together within the authoritative PostgreSQL boundary.
- Broker unavailability must not erase an accepted source transaction or its outbox intent.
- Unpublished outbox records remain durable until publication succeeds or an explicit operational decision records how the condition is handled.
- The platform may apply controlled backpressure or temporarily reject new work when safe durable capacity is exhausted. Silent loss is not an acceptable degradation strategy.
- Publication is asynchronous and at least once. A publisher retry retains the same event identity and event content.
- PostgreSQL remains authoritative transactional storage. Redpanda remains asynchronous transport and replay input.

This does not prescribe an outbox table, polling interval, relay framework, retry schedule, partition key, retention period, or capacity target.

## 5. At-least-once delivery and consumer deduplication

Exactly-once delivery and exactly-once business effects are not claimed without evidence that does not currently exist. Duplicate publication, duplicate delivery, consumer restarts, and acknowledgement uncertainty are expected failure modes.

- Consumers deduplicate using stable `event_id`, within the relevant tenant and processing context.
- Inbox or equivalent deduplication state must be committed atomically with the resulting local effect where a durable local effect is required.
- A duplicate delivery must not duplicate inventory, workflow, audit, notification, or other business effects.
- The same `event_id` with different content is an integrity violation and must be quarantined or escalated for reconciliation.
- A consumer acknowledges an event only after the required durable processing decision is committed. Acknowledgement before that decision may cause loss on restart.
- A failed or uncertain processing decision remains visible and attributable; it is not silently treated as success.

## 6. Quarantine and poison events

An event must be quarantined, rejected, or routed for controlled reconciliation when safe processing is unavailable. Examples include:

- malformed or incomplete envelope;
- unsupported `event_schema_version`;
- invalid, missing, or cross-tenant context;
- same `event_id` with conflicting content;
- missing required aggregate version;
- impossible state transition;
- repeated processing failure; or
- an unrecognized event type for which safe handling is unavailable.

Quarantine must preserve enough identity, version, lineage, failure category, and sanitized diagnostic information for investigation and controlled recovery. It must not copy sensitive payload data into unrestricted logs or create a new sensitive-data store. Quarantine is not permission to bypass authorization, version checks, or integrity checks.

A quarantined event must not block unrelated aggregates by default, but recovery must not silently skip the event or apply later dependent versions as if the gap did not exist.

## 7. Deterministic replay

Replay is the controlled reprocessing of events to rebuild or verify derived projections and other replay-safe outputs. It is not a license to reissue commands or reconstruct all authoritative business state solely from events.

Replayable handlers must:

- use stable event identity and the required per-aggregate ordering;
- produce equivalent derived state from equivalent inputs, versions, and handler logic;
- identify the version of handler logic and any required reference inputs;
- avoid silent dependence on the current wall clock, uncontrolled randomness, mutable external data, or unrecorded model output;
- make unsupported or non-replayable operations explicit; and
- make replay failures visible, attributable, and recoverable through an operational decision.

Replay of projections must not repeat irreversible external commands, notifications, approvals, or other external effects. Such effects are suppressed, simulated, or separately reconciled. If a handler cannot meet this rule, it is not replay-safe and that limitation must be documented.

## 8. Degraded behavior

### Broker outage

- Core transactional writes may continue while durable outbox capacity and safety limits permit.
- Accepted transactions and their outbox entries remain atomic and durable.
- Projections, notifications, and intelligence inputs fed by asynchronous publication may become stale.
- Publication resumes from durable outbox state after recovery.
- The platform must not claim that asynchronous consumers are current during the outage.
- Controlled backpressure or temporary rejection is preferable to silently accepting work that cannot be durably retained.

### Python outage

- Core Go-owned transactional operations, authorization, and existing business-state transitions remain available.
- New forecasts, retrieval results, explanations, and proposals may be unavailable or explicitly stale.
- Python failure cannot grant authority, bypass approval, or make a command executable.
- A core transaction must not depend on a synchronous Python call completing successfully.

## 9. Event-model invariants

| ID | Invariant |
| --- | --- |
| EM-01 | An event represents an accepted state change; it is not published as a substitute for committing that change. |
| EM-02 | The authoritative state change and its outbox intent are atomic, and an accepted transaction cannot disappear during broker outage. |
| EM-03 | Retries and re-publication retain the same logical event identity and content. |
| EM-04 | Duplicate delivery cannot duplicate a required durable business effect. |
| EM-05 | Aggregate versions are checked per aggregate; missing versions are not silently skipped. |
| EM-06 | Same event identity with conflicting content is treated as an integrity violation. |
| EM-07 | Acknowledgement follows the required durable processing decision. |
| EM-08 | Unsafe, poison, incompatible, and cross-tenant events are quarantined or reconciled without unsafe application. |
| EM-09 | Replay is deterministic and does not repeat irreversible external effects. |
| EM-10 | The platform makes no unsupported exactly-once delivery or business-effect claim. |

## 10. Deferred implementation choices

Issue #21 resolves the first M1 event serialization, schema compatibility,
topic/key policy, source/outbox boundary, inbox/projection transaction, retry
and acknowledgement contract, failure record, and checksum canonicalization in
[`CONTRACTS.md`](../../CONTRACTS.md).

The following remain deliberately open: later event families, retention and
archival, partition sizing, publisher and consumer process layout, metrics,
alerts and thresholds, deployment-specific credentials, HTTP routes, and
operator recovery controls. Those choices remain subject to the invariants in
this document and the M1 contract.
