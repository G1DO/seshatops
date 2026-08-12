# Event Spine Source Transaction and Outbox Persistence

**Status:** Implemented for Issue #23 library/integration scope. Issue #24
implements the source-owned outbox relay; Issue #25 implements the projection
consumer.

**Owns:** The minimum `erp` PostgreSQL schema, synthetic one-line order
acceptance transaction, immutable outbox insert, Northstar inventory seed, and
PostgreSQL image pin used by integration tests.

**Does not own:** Outbox publication, broker configuration, inbox/projection,
HTTP APIs, or a general ERP catalogue.

## Toolchain pin

| Item | Value |
| --- | --- |
| PostgreSQL version | `16.14` (`erp.PostgresVersionLabel`) |
| Immutable image | `postgres@sha256:95206741a5b214807675e14165369d05b93a9cf692223b616d07cca227e74b0b` (`erp.PostgresImage`) |

Resolved from Docker Hub tag `postgres:16.14` multi-platform index digest on
2026-08-10. Version strings alone are not sufficient; tests and local tooling
must use the digest pin.

## Schema (`erp`)

Migration source: [`erp/migrate.sql`](../../erp/migrate.sql).

| Table | Responsibility |
| --- | --- |
| `erp.inventory_items` | Authoritative `(tenant_id, item_id)` quantity and aggregate version |
| `erp.orders` | Accepted one-line synthetic orders |
| `erp.outbox` | Immutable logical event bytes, content hash, and publication status |

Outbox status values match ADR-0001: `pending`, `publishing`, `published`,
`quarantined`. Issue #23 inserts `pending` only. Nullable lease columns exist so
Issue #24 can claim rows without rewriting event identity fields.

## Source transaction

`erp.AcceptOrder` performs one PostgreSQL transaction:

1. Validate quantity and identity/context fields.
2. Lock the inventory row with `SELECT … FOR UPDATE`.
3. Reject unknown items, tenant/context mismatches, and transitions that would
   make `quantity_on_hand` negative.
4. Update inventory quantity and advance `aggregate_version` by 1.
5. Insert the accepted order.
6. Build the `inventory.quantity_decremented` v1 envelope, validate it, and store
   exact JCS bytes plus SHA-256 content hash in `erp.outbox` with status
   `pending`.
7. Commit. No Redpanda or broker call occurs in this path.

A failure before commit rolls back order, inventory, and outbox together
(M1-INV-01).

## Fixture seed

Integration tests use `northstar.Generate(northstar.DefaultSeed)` and
`erp.SeedNorthstarInventory`:

- tenant `11111111-1111-4111-8111-111111111111`
- item `item-flour-001`
- `quantity_on_hand = 10`
- `aggregate_version = 0`

Accepting `erp.OrderCommandFromFixture` yields the exact Issue #22 event
identity and content (`aggregate_version = 1`, quantity 10→8).

## Integration test harness

- Preferred: Testcontainers using `erp.PostgresImage`.
- Override: set `SESHATOPS_TEST_DATABASE_URL` to an existing PostgreSQL DSN.
- When neither Docker nor the override is available, PostgreSQL-backed tests
  skip with an explicit reason. Validation-only tests still run.

## Non-claims

This document does not claim broker-outage recovery, at-least-once publication,
projection correctness, tenant-isolation enforcement beyond the source row key,
performance, or production readiness. `CLM-003` remains Planned until broker
outage evidence exists.
