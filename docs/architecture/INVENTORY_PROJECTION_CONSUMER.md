# M1 Inventory Projection Consumer

**Status:** Implemented for Issue #25 library/integration scope. Bounded
failure/backlog observability remains Issue #26. HTTP/SSE read surface remains
Issue #27.

**Owns:** Consuming the M1 Redpanda topic, durable inbox/deduplication,
aggregate-version validation, PostgreSQL inventory projection updates, atomic
inbox+projection commits, acknowledgement-after-commit ordering, minimal
sanitized processing-failure records for unparseable/poison deliveries, and the
projection checksum helper.

**Does not own:** Generic CQRS frameworks, order projections, operator recovery
UI, HTTP/SSE APIs, metrics SLOs, or exactly-once transport claims.

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

## Projection checksum

`platform.ChecksumTenant` implements [CONTRACTS.md](../../CONTRACTS.md) §8:
lowercase identifiers, base-10 integers, rows sorted by tenant then item,
tab-delimited UTF-8 lines ending in `\n`, SHA-256 hex. Empty projection hashes
the empty byte sequence.

## Integration harness

- Preferred: Testcontainers using `erp.PostgresImage` and `relay.RedpandaImage`.
- Override: `SESHATOPS_TEST_DATABASE_URL` for PostgreSQL.
- When Docker is unavailable, broker/PostgreSQL-backed tests skip with an
  explicit reason.

## Non-claims

This document does not claim exactly-once delivery, production readiness,
tenant-isolation security enforcement beyond the M1 contract checks, SLO
compliance, or hosted CI success without a recorded GitHub Actions run.
