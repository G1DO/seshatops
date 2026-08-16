# Event Spine Contracts

Wire, persistence, failure dispositions, checksum, and toolchain pins for the
implemented Event Spine. Why: [ADR-0001](../../decisions/0001-transactional-outbox-and-at-least-once-delivery.md)
and [ADR-0003](../../decisions/0003-event-envelope-and-schema-compatibility.md).
Executable helpers: `event`. Fixture: `northstar`. HTTP/SSE:
[openapi-projection.yaml](../../api/openapi-projection.yaml).

## 1. Event Spine vertical slice

The implemented Event Spine source path records Northstar M1 order/inventory
and M3 lineage hops for one tenant:

> accepted source hop -> PostgreSQL source transaction and outbox ->
> Redpanda -> Go consumer -> transactional inbox, inventory projection, and
> lineage projection ->
> Go-owned read surface for the TypeScript operations view

HTTP/SSE for that last step is
[openapi-projection.yaml](../../api/openapi-projection.yaml).
This document does not redefine routes or SSE frames.

The **accepted event contract** includes the M3 traceability families in
section 2. `event.Parse` / `event.Validate` accept those families. The
implemented ERP outbox path emits `supplier.registered`,
`ingredient_lot.received`, `production_batch.produced`,
`shipment.dispatched`, and `inventory.quantity_decremented`.
The projection consumer applies `inventory.quantity_decremented` to the
inventory projection and the four M3 traceability families to tenant-scoped
relational lineage tables. Those families are not coerced into inventory
projection. An accepted family this consumer does not apply is still
quarantined as an unsupported contract, with available event identity
(event id, tenant, aggregate, schema version, event type, and content hash)
recorded so tenant-scoped inspect can see it. The Northstar lineage fixture is
`northstar.GenerateLineage` with seed `northstar-m3-lineage-v1`.

Multi-line orders, commands, approvals, and intelligence remain outside this
contract. Identity HTTP is
[authorization.md](../../security/authorization.md).

## 2. Event envelope

The wire value is UTF-8 JSON. The v1 envelope contains exactly these fields:

| Field | Type and Event Spine rule |
| --- | --- |
| `event_id` | UUIDv4 string; stable identity of one logical event |
| `tenant_id` | Canonical lowercase UUID string |
| `event_type` | One of the accepted families in the table below |
| `event_schema_version` | Positive integer; Event Spine accepts exactly `1` for every accepted family |
| `aggregate_type` | Exact aggregate type required by that family |
| `aggregate_id` | Canonical lowercase identifier for that aggregate |
| `aggregate_version` | Positive integer; per-tenant, per-aggregate sequence beginning at `1` |
| `occurred_at` | RFC 3339 UTC timestamp for the source business occurrence |
| `recorded_at` | RFC 3339 UTC timestamp for durable source/outbox recording |
| `producer` | Exactly `synthetic-erp` |
| `correlation_id` | Stable UUID string for order/workflow lineage |
| `causation_id` | Nullable UUID string; `null` for the initial source transaction |
| `trace_id` | Stable opaque lineage string; it is not a distributed-tracing claim |
| `payload` | Strict payload for that family and schema version |

Event Spine does not permit free-form metadata. Future metadata requires a reviewed
schema version.

Accepted families, all at `event_schema_version` `1`:

| `event_type` | `aggregate_type` | Payload fields |
| --- | --- | --- |
| `inventory.quantity_decremented` | `inventory_item` | `order_id`, `item_id`, `quantity_decremented`, `quantity_before`, `quantity_after` |
| `supplier.registered` | `supplier` | `supplier_id` |
| `ingredient_lot.received` | `ingredient_lot` | `lot_id`, `supplier_id`, `item_id` |
| `production_batch.produced` | `production_batch` | `batch_id`, `lot_id` |
| `shipment.dispatched` | `shipment` | `shipment_id`, `batch_id`, `order_id` |

The self-id in each payload (`item_id`, `supplier_id`, `lot_id`, `batch_id`,
`shipment_id`) must equal `aggregate_id`. Canonical lowercase identifiers use
the existing `aggregate_id` rule. `order_id` is a UUID string.

The M3 chain is 1:1. Arrays are not permitted in the event contract, so a
production batch references one lot and a shipment references one batch.
Shipment joins to the order hop by `shipment.payload.order_id` equal to
`inventory.quantity_decremented` `payload.order_id`.

The normative `inventory.quantity_decremented` v1 payload is:

| Field | Type and invariant |
| --- | --- |
| `order_id` | UUID string for the accepted synthetic order |
| `item_id` | Canonical inventory-item identifier; equals `aggregate_id` |
| `quantity_decremented` | Positive integer |
| `quantity_before` | Non-negative integer |
| `quantity_after` | Non-negative integer |

The inventory payload must satisfy:

`quantity_before - quantity_decremented = quantity_after`.

Traceability payload invariants:

| Field | Type and invariant |
| --- | --- |
| `supplier_id` | Canonical lowercase identifier; equals `aggregate_id` on `supplier.registered` |
| `lot_id` | Canonical lowercase identifier; equals `aggregate_id` on `ingredient_lot.received` |
| `batch_id` | Canonical lowercase identifier; equals `aggregate_id` on `production_batch.produced` |
| `shipment_id` | Canonical lowercase identifier; equals `aggregate_id` on `shipment.dispatched` |
| `item_id` on `ingredient_lot.received` | Canonical inventory-item identifier |
| `order_id` on `shipment.dispatched` | UUID string for the synthetic order |

Example inventory shape:

```json
{
  "event_id": "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1",
  "tenant_id": "11111111-1111-4111-8111-111111111111",
  "event_type": "inventory.quantity_decremented",
  "event_schema_version": 1,
  "aggregate_type": "inventory_item",
  "aggregate_id": "item-flour-001",
  "aggregate_version": 1,
  "occurred_at": "2026-08-07T09:00:00Z",
  "recorded_at": "2026-08-07T09:00:00Z",
  "producer": "synthetic-erp",
  "correlation_id": "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a2",
  "causation_id": null,
  "trace_id": "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a3",
  "payload": {
    "order_id": "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a4",
    "item_id": "item-flour-001",
    "quantity_decremented": 2,
    "quantity_before": 10,
    "quantity_after": 8
  }
}
```

The identifiers in this example are synthetic and illustrative only.

## 3. Schema compatibility and content identity

Version compatibility is exact and explicit:

- The parser accepts only the families and schema version `1` listed in section 2.
- Unknown event types, unknown schema versions, missing fields, unknown fields, duplicate object member names, invalid types, and invalid values are rejected. An unknown event type fails as unsupported even when the payload matches a known family.
- The implemented ERP producer emits the accepted families in section 2 at schema version `1` through `erp.RegisterSupplier`, `erp.ReceiveIngredientLot`, `erp.ProduceProductionBatch`, `erp.DispatchShipment`, and `erp.AcceptOrder`.
- The implemented inventory consumer applies `inventory.quantity_decremented` to inventory projection. The four M3 traceability families are applied to tenant-scoped relational lineage tables and are not coerced into inventory projection. An accepted family this consumer does not apply is quarantined as an unsupported contract. The failure record keeps the identity Parse recovered; it is not an unattributed parse failure.
- There is no implicit coercion, defaulting, fallback handler, or schema-registry service.
- Any semantic, required-field, or interpretation change requires a new schema version and handler.

The content hash is:

`SHA-256(JCS(canonical envelope without transport metadata))`

JCS means [JSON Canonicalization Scheme (RFC 8785)](https://www.rfc-editor.org/rfc/rfc8785.html).
The hash input includes all envelope fields and the payload. It excludes only
transport and processing metadata:
Redpanda topic, partition, offset, publish timestamp, inbox timestamp, retry
state, and database timestamps created after source recording.

Before JCS canonicalization, the parser must reject duplicate object member
names at every object level and accept only UTF-8 JSON values compatible with
JCS. The v1 numeric fields are JSON integer tokens in the inclusive range
`0` through `9007199254740991` (`2^53 - 1`); decimal, exponent, string, and
out-of-range forms are rejected. `event_schema_version` and
`aggregate_version` must still be positive, and `quantity_decremented` must
still be positive. UTC timestamps use the `Z` form; their exact source string,
including any permitted fractional seconds, is part of event content and is
not rewritten during retry or republishing.

The stored event bytes and Redpanda value are the UTF-8 JCS serialization of
the validated envelope. The example below shows shape only; it is not required
to use JCS property order as displayed.

The same `event_id` with a different content hash is an integrity failure, not
an ordinary duplicate.

## 4. PostgreSQL ownership and transaction boundaries

Event Spine uses one PostgreSQL database with separate logical schemas:

| Schema | Owns | Must not own |
| --- | --- | --- |
| `erp` | Source orders, lineage hops, authoritative inventory, and outbox rows | Platform projection state |
| `platform` | Inbox records, inventory projection, lineage projection, and processing failures | Source inventory or order state |

### Source transaction

Each source transaction records one accepted hop: supplier registration,
ingredient-lot receipt, production-batch production, shipment dispatch, or a
one-line order. It locks required parent source rows and, for the order hop,
the authoritative inventory row. It derives the event tenant from the command
and locked parent or inventory row, writes the source row, and inserts the
immutable outbox event and content hash. These changes commit or roll back
together. No Redpanda call occurs inside this transaction.

Shipment stores `order_id` without requiring `erp.orders` to exist; the M3
fixture dispatches before the order hop. `AcceptOrder` requires `causation_id`
to equal the shipment outbox event when a shipment already exists for that
order, and requires `causation_id` to be null otherwise.

Schema and image pins live in `erp/migrate.sql` / `erp.PostgresImage`,
`relay.RedpandaImage`, and `platform` migrations. `erp.outbox` is unique on
`event_id` and on `(tenant_id, aggregate_type, aggregate_id, aggregate_version)`.
Rebuild helpers are `platform.ResetDerivedState` and
`platform.RebuildFromHistory`.

### Outbox relay

The source-owned relay claims due outbox rows with a lease, publishes the exact
stored event bytes, and marks the row `published` only after broker acknowledgement.
A crash after broker acknowledgement and before the PostgreSQL update may
publish a duplicate with the same identity and content.
An expired `publishing` lease (`relay.DefaultLeaseTTL`, 30s) is reclaimable
for retry (claimed again as `publishing` under a new lease); a live lease is
not claimed by another relay worker.

Outbox states are `pending`, `publishing`, `published`, and `quarantined`.
Transient publication failures use a 1-second exponential backoff capped at
60 seconds. Non-retryable serialization or contract failures are durably
quarantined; rows are never silently deleted.

### Consumer transaction

The Go consumer validates the event before taking the duplicate no-op path:

1. Parse strict UTF-8 JSON and validate the exact v1 envelope and payload.
2. Recompute the expected key as the UTF-8 bytes of
   `tenant_id/aggregate_type/aggregate_id` and require byte-for-byte equality
   with the Redpanda key.
3. Require the family payload self-id to equal `aggregate_id`
   (`item_id`, `supplier_id`, `lot_id`, `batch_id`, or `shipment_id`).
   The consumer never infers a tenant from a resource identifier.
4. Check inbox identity and content, applying the disposition rules in section
   6.
5. Validate tenant and aggregate version rules, then insert or update the
   inbox and apply the projection in one `platform` transaction.

Any missing, ambiguous, or inconsistent tenant/key/aggregate context is
quarantined before a duplicate can be acknowledged. The Redpanda offset is
committed only after that transaction commits.

## 5. Redpanda delivery contract

| Item | Event Spine decision |
| --- | --- |
| Topic | `seshatops.m1.events` |
| Value | UTF-8 canonical JSON envelope |
| Key | `tenant_id/aggregate_type/aggregate_id` using canonical lowercase identifiers |
| Consumer group | `seshatops-m1-inventory-projection` |
| Acknowledgement | Manual offset commit after durable processing decision |
| Ordering | Per aggregate key only; no global ordering |
| Producer | Source-owned outbox relay only |
| Consumer | Go projection consumer; consume-only broker capability |

The key is never `event_id`. Event Spine does not use or claim Redpanda exactly-once
features.

## 6. Inbox, inventory projection, and lineage projection

The inbox has a unique constraint on `(consumer_name, event_id)` and stores
tenant, content hash, aggregate identity, aggregate version, and disposition.
For a structurally valid event quarantined only for an aggregate-version gap,
the inbox also retains the exact canonical event bytes in a restricted field
needed for automatic re-drive. Malformed or otherwise non-canonical input is
never retained as raw payload.

The inventory projection has primary key `(tenant_id, item_id)` and stores
`quantity_on_hand` and `aggregate_version`.

- Version `1` initializes the projection from `quantity_after`.
- Later events require incoming version `current + 1`.
- Later events require `quantity_before == current quantity_on_hand`.
- Every event must satisfy the payload arithmetic invariant.
- Matching event identity and content is a durable no-op only when the existing
  disposition is `applied` or `duplicate_noop`.
- Conflicting content, stale versions, and conflicting same-version events are quarantined.

The lineage projection is four tenant-scoped relational tables
(`lineage_suppliers`, `lineage_ingredient_lots`, `lineage_production_batches`,
`lineage_shipments`) sufficient for supplier → ingredient lot → production
batch → shipment → order traversal. Each row stores source event identity,
schema version, source timestamps, and correlation/causation/trace identifiers.
Parent identifiers are stored under the envelope tenant; they are never used
to infer tenant. Cross-tenant joins are not performed. Child hops may arrive
before parents because broker ordering is per aggregate only. Version `1`
inserts a hop; a later version for the same aggregate is quarantined rather
than silently skipped. Matching duplicate delivery is a durable no-op and
cannot duplicate lineage rows. Conflicting 1:1 parent edges in the same tenant
are quarantined.

Inbox insertion and projection update are one transaction. A later event is
never applied across a missing aggregate version. A gap is durably quarantined
with expected and received versions; once the missing event applies, the
consumer may automatically re-drive contiguous `quarantined_gap` inbox rows
from their retained canonical event bytes. Re-drive revalidates the content
hash and applies each event transactionally before changing its disposition.
Operator quarantine, replay, and rebuild HTTP plus the TypeScript operations
view: [authorization.md](../../security/authorization.md) and
[openapi-projection.yaml](../../api/openapi-projection.yaml).

## 7. Failure and quarantine contract

| Failure | Disposition |
| --- | --- |
| Malformed envelope | Durable quarantine; no projection effect |
| Unsupported schema/version | Durable quarantine; no downgrade |
| Unknown event family | Durable quarantine |
| Accepted family not applied by this consumer | Durable `unsupported_contract` quarantine; record available event identity; no inventory projection |
| Tenant, key, or aggregate mismatch | Durable quarantine; never reinterpret tenant |
| Same ID with conflicting content | Integrity quarantine |
| Aggregate-version gap | Quarantine and defer application |
| Stale or conflicting aggregate version | Integrity quarantine |
| Impossible inventory transition | Durable quarantine |
| Matching duplicate | Durable no-op; acknowledge after commit |
| Transient PostgreSQL or handler failure | Do not acknowledge; allow redelivery |
| Repeated handler poison failure | After five handler attempts, persist a sanitized failure record and acknowledge |
| Failure-record persistence unavailable | Do not acknowledge |
| Replay | Projection-only; no commands, notifications, or irreversible effects |

Traceability hops use this same contract. A quarantined M3 delivery has no
lineage effect. Replay and quarantine release revalidate through `ProcessRecord`
and never force-apply a terminal inbox quarantine.

The minimal `platform.processing_failures` record contains:

`failure_id`, `consumer_name`, nullable `event_id`, nullable tenant and
aggregate identity, available schema and event type, failure category, content
hash, source topic/partition/offset, attempt count, timestamps, sanitized
diagnostic code, and quarantine status. `content_hash` is nullable when a
canonical envelope cannot be formed. When Parse succeeded, event id, tenant,
aggregate, schema version, event type, and content hash are available and must
be recorded. When Parse failed but the JSON still carries those identity
fields, they may be recorded without a content hash. Completely unparseable
input leaves those fields null. A separate received-bytes hash may be
recorded only under a distinct field name and is never treated as event content
identity. Raw payloads, secrets, and unrestricted diagnostics are not stored.
Inspect failure samples expose stored event id, tenant, aggregate type/id, and
event type when those columns are populated. Correlation remains on retained
outbox bytes and applied lineage hops.

## 8. Projection checksum

The checksum covers one tenant's complete inventory projection at a declared
snapshot. Each row contributes:

`tenant_id`, `item_id`, `quantity_on_hand`, and `aggregate_version`.

Canonicalization is:

1. Normalize identifiers to lowercase canonical strings.
2. Normalize integers to base-10 with no leading zeros except `0`.
3. Sort rows bytewise by `tenant_id`, then `item_id`.
4. Encode each row as tab-delimited UTF-8 text ending in `\n`.
5. Hash the resulting bytes with SHA-256 and emit lowercase hexadecimal.

Timestamps, database row order, offsets, inbox IDs, failure records, and mutable
metadata are excluded. The empty projection hashes the empty byte sequence.

Rebuild uses `platform.ResetDerivedState` and `platform.RebuildFromHistory`.
The inventory checksum definition itself remains this section.

A second checksum covers one tenant's complete lineage projection. It is never
mixed into the inventory hash. Each applied hop contributes:

`kind`, `tenant_id`, `hop_id`, `parent_id`, `item_id`, `order_id`,
`aggregate_version`, `source_event_id`, and `event_schema_version`.

`kind` is the hop aggregate type (`supplier`, `ingredient_lot`,
`production_batch`, `shipment`). `parent_id` is empty for a supplier.
`item_id` is populated only for ingredient lots. `order_id` is populated only
for shipments. Missing optional identifiers are the empty string.

Canonicalization uses the same five steps as inventory, except rows sort
bytewise by `kind`, then `tenant_id`, then `hop_id`. Inbox, failures,
timestamps, correlation/causation/trace, and inventory rows are excluded.
The empty lineage projection hashes the empty byte sequence. Incomplete rebuild
status means neither checksum is a successful A==B proof.

## 9. Minimum local toolchain

- Go `1.25.0`
- Node.js `24.14.0`
- npm `11.9.0`
- TypeScript `6.0.3`
- PostgreSQL `16.14` pinned as
  `postgres@sha256:95206741a5b214807675e14165369d05b93a9cf692223b616d07cca227e74b0b`
  (`erp.PostgresImage`)
- Redpanda `v25.2.1` pinned as
  `docker.redpanda.com/redpandadata/redpanda@sha256:218469e5d088757bb2c3ff4c5e272f7eebdc4e94c933e6e15aff10b845cbcd07`
  (`relay.RedpandaImage`)

Reference release records: [PostgreSQL 16.14](https://www.postgresql.org/docs/release/16.14/),
[Redpanda 25.2 upgrade documentation](https://docs.redpanda.com/streaming/25.2/upgrade/rolling-upgrade/),
and [TypeScript releases](https://github.com/microsoft/TypeScript/releases).
