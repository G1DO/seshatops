# Event Spine Source-Owned Outbox Relay

**Status:** Implemented for Issue #24 library/integration scope. Issue #25
implements the projection consumer; service runtime remains later Event Spine work.

**Owns:** Claiming durable `erp.outbox` rows, publishing exact stored event
bytes to the Event Spine Redpanda topic, recording publication status after broker
acknowledgement, transient backoff, durable quarantine, and minimum backlog
visibility.

**Does not own:** Inbox/projection consumption, metrics SLOs, retention,
Kubernetes scheduling, or a generic event-bus abstraction.

## Toolchain pin

| Item | Value |
| --- | --- |
| Redpanda version | `v25.2.1` (`relay.RedpandaVersionLabel`) |
| Immutable image | `docker.redpanda.com/redpandadata/redpanda@sha256:218469e5d088757bb2c3ff4c5e272f7eebdc4e94c933e6e15aff10b845cbcd07` (`relay.RedpandaImage`) |

Resolved from the Docker Hub multi-platform index digest for tag `v25.2.1` on
2026-08-11. Version strings alone are not sufficient; tests and local tooling
must use the digest pin. PostgreSQL continues to use `erp.PostgresImage`.

## Topic and key policy

| Item | Event Spine value |
| --- | --- |
| Topic | `seshatops.m1.events` |
| Key | `tenant_id/aggregate_type/aggregate_id` |
| Value | Exact UTF-8 JCS bytes from `erp.outbox.event_bytes` |

The key is never `event_id`. The relay does not use or claim Redpanda
exactly-once producer features.

## Publication flow

Package `relay` implements the source-owned relay:

1. `ClaimDue` selects due `pending` rows (or expired/`NULL`-lease `publishing`
   rows) with `FOR UPDATE SKIP LOCKED`, sets `publishing`, assigns
   `publish_lease_owner`, sets `publish_lease_expires_at`, and increments
   `publish_attempts`.
2. Stored bytes are validated with `event.Parse`. The Redpanda key is taken from
   the parsed envelope; column/bytes aggregate-key disagreement is quarantined.
   Malformed or unsupported contracts are `quarantined` with `last_error_code`;
   rows are never deleted.
3. Valid rows are published with the aggregate key and **exact** stored bytes.
4. `MarkPublished` runs **only after** broker acknowledgement.
5. Transient broker failures return the row to `pending` with exponential
   backoff stored in `publish_lease_expires_at` (1s base, cap 60s) and
   `last_error_code = broker_publish_failed`.

Default publishing lease TTL is 30 seconds. A live lease is not claimed by
another worker. An expired or `NULL` `publishing` lease is reclaimable for retry.

## At-least-once and the ambiguous window

Delivery is at least once. A crash after broker acknowledgement and before the
PostgreSQL `published` update leaves the row `publishing` until the lease
expires. Retry may duplicate publication with the same event identity and
content. Duplicate publication is expected and safe; silent loss is not.

## Backlog visibility

`InspectBacklog` exposes pending, publishing, published, and quarantined
counts, the oldest unpublished `created_at`, and a bounded quarantine sample.
This is the Event Spine verification surface for unpublished intent and terminal
publication failures. It is not an Identity & Operations metrics or alerting
stack. Issue #47 product reads use tenant-scoped `InspectBacklogForTenant`
through `GET /v1/tenants/{tenant_id}/ops`. Issue #48 may return a same-tenant
quarantined row to `pending` via `ReleaseQuarantined` after `MX-004`.

## Integration harness

- Preferred: Testcontainers using `erp.PostgresImage` and `relay.RedpandaImage`.
- Override: `SESHATOPS_TEST_DATABASE_URL` for PostgreSQL.
- When Docker is unavailable, broker/PostgreSQL-backed tests skip with an
  explicit reason. Pure unit tests (backoff, key, image pin) still run.

## Non-claims

This document does not claim exactly-once delivery, projection correctness,
tenant-isolation enforcement beyond stored tenant fields, performance,
availability SLOs, or production readiness. `CLM-003` and `CLM-005` remain
Planned in `EVIDENCE.md` until formal claim promotion with required evidence
artifacts.
