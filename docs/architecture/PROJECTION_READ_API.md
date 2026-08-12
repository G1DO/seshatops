# M1 Projection Read API (REST + SSE)

**Status:** Implemented for Issue #27 library/integration scope. No deployment
service binary, authentication, or TypeScript client is claimed here.

**Owns:** The minimum Go-owned read-only HTTP surface for the committed M1
inventory projection, including REST snapshot DTOs, one SSE stream for
post-commit projection updates, reconnect/catch-up semantics, and the OpenAPI
fragment in [`openapi-m1-projection.yaml`](openapi-m1-projection.yaml).

**Does not own:** Authentication/RBAC, write/command endpoints, GraphQL,
WebSockets, generic subscription gateways, operator recovery UI, Redpanda
access from the browser, or the TypeScript operations view (Issue #28).

## Browser isolation

Per [ARCHITECTURE.md](../../ARCHITECTURE.md):

- Allowed: Browser ↔ Go public APIs.
- Prohibited: Browser → PostgreSQL; Browser → Redpanda.

Package `api` reads committed `platform.inventory_projection` rows through
PostgreSQL only. It does not import or contact the broker. UI data for M1 must
flow exclusively through this Go surface.

## Routes

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/v1/tenants/{tenant_id}/inventory` | Authoritative committed snapshot |
| `GET` | `/v1/tenants/{tenant_id}/inventory/stream` | SSE live updates after projection commit |

`POST`, `PUT`, `PATCH`, and `DELETE` on these paths return `405 Method Not Allowed`
and do not mutate projection state. Malformed or non-lowercase UUIDv4
`tenant_id` values return `400` (or `404` for empty path segments) without
mutating state.

`tenant_id` validation follows the M1 lowercase UUIDv4 identifier rule. This is
context validation only; it is **not** M2 authentication or tenant
authorization enforcement.

## Snapshot DTO

```json
{
  "tenant_id": "11111111-1111-4111-8111-111111111111",
  "items": [
    {
      "item_id": "item-flour-001",
      "quantity_on_hand": 8,
      "aggregate_version": 1
    }
  ],
  "checksum": "<CONTRACTS.md §8 hex>",
  "observed_at": "2026-08-12T07:00:00Z"
}
```

| Field | Meaning |
| --- | --- |
| `items[].aggregate_version` | Committed business aggregate version from the projection |
| `checksum` | `platform.ChecksumTenant` over the tenant's committed projection |
| `observed_at` | Server UTC RFC 3339 timestamp when the snapshot was read (connection/as-of freshness). The projection table does not store wall-clock freshness columns. |

Empty tenants return `items: []` and the empty-input checksum.

## SSE semantics

- Content type: `text/event-stream`
- Event name: `inventory_projection.updated`
- Payload fields: `tenant_id`, `item_id`, `quantity_on_hand`, `aggregate_version`,
  `last_applied_event_id`, `checksum`
- Emission occurs only after the corresponding inbox+projection PostgreSQL
  transaction commits with disposition `applied` (including gap redrive applies).
- Duplicate no-ops, quarantines, and rolled-back transactions do not emit.
- Delivery is at least once for connected subscribers. Slow/full subscriber
  buffers may drop updates without blocking projection commits.
- Periodic SSE comment heartbeats may be sent; they are not projection updates.
- SSE is **not** a complete event history and does not claim exactly-once or
  gapless delivery (`M1-INV-14`).

### Example SSE frame

```text
event: inventory_projection.updated
data: {"tenant_id":"11111111-1111-4111-8111-111111111111","item_id":"item-flour-001","quantity_on_hand":8,"aggregate_version":1,"last_applied_event_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1","checksum":"..."}

```

## Reconnect / catch-up

REST is the authoritative snapshot path.

1. On initial load: `GET .../inventory`, render items + version/checksum metadata.
2. Open `GET .../inventory/stream` for subsequent committed updates.
3. On SSE disconnect, process restart, suspected drop, or stale/disconnected UI
   state: re-fetch `GET .../inventory`, replace local state from the snapshot,
   then reopen the SSE stream.
4. Do not invent business state from missed SSE frames. Treat SSE as a live
   hint that a committed change exists.

This keeps stale/disconnected behavior visible (`M1-INV-11`) rather than
silently presenting a partial live feed as complete.

## Wiring

Library package: `github.com/G1DO/seshatops/api`.

1. Construct `hub := api.NewHub()`.
2. `platform.SetAppliedNotifier(hub)` before consuming/applying events.
3. Serve `api.NewServer(db, hub).Handler()`.

No long-running deployment binary is provided in this issue.

## Authentication boundary (M2)

Authentication, session identity, and tenant authorization enforcement remain
M2 scope. M1 carries `tenant_id` for demo context and identifier validation
only.

## Non-claims

This document does not claim production readiness, hosted CI success without a
recorded GitHub Actions run, exactly-once SSE delivery, or M2 security
enforcement.
