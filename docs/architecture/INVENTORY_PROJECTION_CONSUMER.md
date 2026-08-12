# M1 Inventory Projection Consumer

**Status:** Implemented for Issues #25 and #26 library/integration scope.
Issue #27 adds post-commit `AppliedNotifier` hooks used by the HTTP/SSE read
surface in [PROJECTION_READ_API.md](PROJECTION_READ_API.md). Issue #29 adds
isolated derived-state reset and retained-history rebuild proofs in
[PROJECTION_REBUILD.md](PROJECTION_REBUILD.md).

**Owns:** Consuming the M1 Redpanda topic, durable inbox/deduplication,
aggregate-version validation, PostgreSQL inventory projection updates, atomic
inbox+projection commits, acknowledgement-after-commit ordering, minimal
sanitized processing-failure records for unparseable/poison deliveries,
bounded failure/gap inspection via `InspectProcessing`, the projection
checksum helper, tenant projection listing, optional post-commit applied
notifications for the Go read API, and (with Issue #29) test-scoped derived
state reset plus retained-history rebuild helpers.

**Does not own:** Generic CQRS frameworks, order projections, operator recovery
UI, release-from-quarantine workflows, HTTP/SSE route design (Issue #27 /
[PROJECTION_READ_API.md](PROJECTION_READ_API.md)), metrics SLOs, or
exactly-once transport claims.

## Toolchain pins

| Item | Value |
| --- | --- |
| PostgreSQL | `erp.PostgresImage` (`16.14`) |
| Redpanda | `relay.RedpandaImage` (`v25.2.1`) |
| Topic | `seshatops.m1.events` (`relay.Topic`) |
| Consumer group / inbox name | `seshatops-m1-inventory-projection` |

## Schema (`platform`)

Migration source: [`platform/migrate.sql`](../../platform/migrate.sql).

| Table | Responsibility |
| --- | --- |
| `platform.inbox` | Unique `(consumer_name, event_id)` deduplication and disposition; gap rows may retain canonical `event_bytes` |
| `platform.inventory_projection` | Committed `(tenant_id, item_id)` quantity and aggregate version |
| `platform.processing_failures` | Sanitized durable decisions for malformed/unsupported/poison deliveries (no raw payloads) |

Inbox dispositions: `applied`, `duplicate_noop`, `quarantined_conflict`,
`quarantined_gap`, `quarantined_stale`, `quarantined_invalid`,
`quarantined_mismatch`, `quarantined_transition`.

## Consume flow

Package `platform` implements:

1. Parse strict UTF-8 JSON with `event.Parse` and reject unsupported contracts.
2. Require Redpanda key byte-equality with
   `tenant_id/aggregate_type/aggregate_id` and `payload.item_id == aggregate_id`.
3. In one PostgreSQL transaction: resolve inbox identity/content, validate
   aggregate version and inventory arithmetic, then insert/update inbox and
   projection (or quarantine) together.
4. Commit the Redpanda consumer offset **only after** that transaction commits
   (`ShouldAck`).
5. After an `applied` decision, automatically re-drive contiguous
   `quarantined_gap` successors for the same aggregate from retained canonical
   bytes.

Identical redelivery with matching content and disposition `applied` or
`duplicate_noop` is a durable no-op. Same `event_id` with a different content
hash is `quarantined_conflict` and does not mutate the projection.

## Acknowledgement semantics

| Decision | Offset commit |
| --- | --- |
| Applied / duplicate no-op / durable quarantine | Yes, after DB commit |
| Applied / duplicate no-op when gap re-drive fails | No; redelivery retries re-drive |
| Transient PostgreSQL/processing failure (including begin/commit) | No; does not consume poison budget |
| Poison after five explicit handler-poison attempts | Yes, after sanitized failure record commits |

Delivery remains at least once. A crash after DB commit and before offset
acknowledgement may redeliver; redelivery must not duplicate the inventory
effect. M1 does not use or claim Redpanda exactly-once features.

## Failure and quarantine

Disposition policy follows [CONTRACTS.md](../../CONTRACTS.md) §7.

| Failure | Durable surface | Projection mutated? |
| --- | --- | --- |
| Malformed envelope | `processing_failures` (`malformed_envelope`) | No |
| Unsupported schema/type | `processing_failures` (`unsupported_contract`) | No |
| Tenant/key/aggregate mismatch | inbox `quarantined_mismatch` | No |
| Same `event_id`, conflicting content | inbox `quarantined_conflict` | No |
| Aggregate-version gap / reorder | inbox `quarantined_gap` (+ canonical bytes) | No until contiguous re-drive |
| Stale aggregate version | inbox `quarantined_stale` | No |
| Impossible inventory transition | inbox `quarantined_transition` / `quarantined_invalid` | No |
| Explicit handler poison | `processing_failures` (`handler_poison`), ack after five attempts | No |
| Transient PostgreSQL/handler retry | no durable failure row; withhold ack | No |

Raw payloads, secrets, and unrestricted error text are not stored.
`diagnostic_code` is a short fixed code. `received_bytes_hash` may record
SHA-256 of received bytes under a distinct field and is never content identity.
`content_hash` is nullable when a canonical envelope cannot be formed.

### Synthetic failure-record example

Northstar-safe illustration only (not a production observation):

| Field | Example value |
| --- | --- |
| `failure_category` | `malformed_envelope` |
| `diagnostic_code` | `malformed_envelope` |
| `quarantine_status` | `quarantined` |
| `event_id` | null (unparseable) |
| `received_bytes_hash` | lowercase SHA-256 hex of received bytes |
| `source_topic` | `seshatops.m1.events` |
| `source_partition` / `source_offset` | broker position used for attribution |

## Restart windows

| Crash window | Expected outcome |
| --- | --- |
| Before PostgreSQL processing transaction commits | No partial inbox/projection effect; redelivery reprocesses safely |
| After PostgreSQL commit, before Redpanda acknowledgement | Redelivery expected; matching content is durable `duplicate_noop` with no second inventory effect |

## Failure and backlog visibility

`InspectProcessing` exposes inbox disposition counts, `processing_failures`
retrying/quarantined counts, oldest gap and failure timestamps, and bounded
sanitized samples (`LIMIT 20`) for M1 verification. Samples omit `event_bytes`
and raw payloads. This is not an M2 metrics, alerting, or operator console
surface.

Durable quarantine decisions acknowledge offsets so unrelated aggregates on the
same consumer path can continue. Transient no-ack failures may still head-of-line
block a partition until retry succeeds.

## Operator recovery boundary (M2)

M1 does not provide release-from-quarantine, privileged replay, authentication,
or RBAC for recovery. Operator-controlled recovery remains M2 scope
(`CAP-013`).

## Projection checksum

`platform.ChecksumTenant` implements [CONTRACTS.md](../../CONTRACTS.md) §8:
lowercase identifiers, base-10 integers, rows sorted by tenant then item,
tab-delimited UTF-8 lines ending in `\n`, SHA-256 hex. Empty projection hashes
the empty byte sequence.

Issue #29 documents how checksum `A`/`B` comparison is used after
`ResetDerivedState` and `RebuildFromHistory` in
[PROJECTION_REBUILD.md](PROJECTION_REBUILD.md).

## Integration harness

- Preferred: Testcontainers using `erp.PostgresImage` and `relay.RedpandaImage`.
- Override: `SESHATOPS_TEST_DATABASE_URL` for PostgreSQL.
- When Docker is unavailable, broker/PostgreSQL-backed tests skip with an
  explicit reason.

## Non-claims

This document does not claim exactly-once delivery, production readiness,
tenant-isolation security enforcement beyond the M1 contract checks, SLO
compliance, or hosted CI success without a recorded GitHub Actions run.
