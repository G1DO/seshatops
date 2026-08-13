# Event Spine Operations View (TypeScript)

**Status:** Implemented for Issue #28 package/test scope plus Issue #45 session
presentation, Issue #47 authorized ops-visibility presentation, and Issue #48
privileged-control presentation. The TypeScript view does not authorize. Go
inventory and ops-visibility query-API default-deny is recorded in
[QUERY_API_AUTHORIZATION.md](../security/QUERY_API_AUTHORIZATION.md). Privileged
POSTs are recorded in
[PRIVILEGED_OPS_AUTHORIZATION.md](../security/PRIVILEGED_OPS_AUTHORIZATION.md).
No deployment service binary, SLO/alerting platform, or `CAP-008` / `CAP-009` /
`CAP-010` / `CAP-012` / `CAP-013` claim promotion is asserted here.

**Owns:** The minimum browser TypeScript operations screen that consumes the
Issue #27 Go REST snapshot and SSE stream for the committed Northstar Foods
inventory projection, presents Go-owned session state, presents the
Issue #47 lag/poison/freshness snapshot, and POSTs Issue #48 controls.

**Does not own:** Dashboards, design systems, authorization decisions,
intelligence, commands, direct database or broker access, or authoritative
inventory/business rules (those remain in Go).

## Browser isolation

Per [ARCHITECTURE.md](../../ARCHITECTURE.md) and
[PROJECTION_READ_API.md](PROJECTION_READ_API.md):

- Allowed: Browser ↔ Go public APIs only.
- Prohibited: Browser → PostgreSQL; Browser → Redpanda.

Package `web/` talks exclusively to:

| Method | Path |
| --- | --- |
| `GET` | `/auth/login` |
| `GET` | `/auth/callback` (IdP redirect target; handled by Go) |
| `POST` | `/auth/logout` |
| `GET` | `/auth/session` |
| `GET` | `/v1/tenants/{tenant_id}/inventory` |
| `GET` | `/v1/tenants/{tenant_id}/inventory/stream` |
| `GET` | `/v1/tenants/{tenant_id}/ops` |
| `POST` | `/v1/tenants/{tenant_id}/ops/quarantine/release` |
| `POST` | `/v1/tenants/{tenant_id}/ops/replay` |
| `POST` | `/v1/tenants/{tenant_id}/ops/rebuild` |

Cookies are sent with REST and SSE. The UI presents principal and expiry; it
does not authorize. Unauthenticated callers see Sign in and are not shown
projection or ops-visibility data. Authentication is not authorization.
Inventory reads require `MX-001`. Ops visibility requires `MX-002` or
`MX-003`. Privileged POSTs require `MX-004`, `MX-005`, or `MX-006`. Those
fetches are independent so a 403 on one surface does not hide the other.

## Connection model

Presentation connection states are explicit (`M1-INV-11`):

| State | Meaning |
| --- | --- |
| `loading` | Initial REST snapshot in flight, or snapshot loaded and SSE not yet open |
| `live` | Snapshot loaded and SSE `open` (connected to the projection stream) |
| `stale` | Catch-up REST refetch after stream interruption |
| `disconnected` | SSE lost; quantities are not guaranteed current |
| `error` | Authoritative REST load failed |

Reconnect follows the #27 contract: on SSE loss, re-`GET` the snapshot, replace
local state (clearing any prior SSE `last_applied_event_id`), then reopen the
stream. SSE is a live hint, not a gapless history (`M1-INV-14`).

## Before/after presentation

The read API exposes current `quantity_on_hand` only. The UI retains the last
rendered quantity per `item_id` and shows **previous → current** as presentation
state when a newer authoritative value arrives. The UI does not compute
decrements, validate event arithmetic, or recompute checksums (`M1-INV-13`).

For the deterministic Northstar demo, observing the synthetic order apply yields
`item-flour-001` transitioning from `10` to `8` through successive API values.

## Freshness and version fields

Rendered without business reinterpretation:

- `aggregate_version`
- `checksum`
- `observed_at` (snapshot as-of time from the server)
- `last_applied_event_id` (from SSE when available; cleared on REST catch-up)

## Library inspect vs product surface

Event Spine library helpers `relay.InspectBacklog` and
`platform.InspectProcessing` remain global verification surfaces. They are not
the operator product. Issue #47 exposes tenant-scoped
`InspectBacklogForTenant` / `InspectProcessingForTenant` through
`GET /v1/tenants/{tenant_id}/ops` after Go evaluates `RES-OPS-VISIBILITY`
`ACT-READ`. The UI renders those counts and timestamps without inventing SLOs.
Issue #48 POSTs privileged controls through the same Go API; the browser does
not authorize.

## Local demo

See [web/README.md](../../web/README.md). Serve the Go `identity` `/auth/*`
handler and `api.NewServer(db, hub, auth, policy).Handler()` on
`http://127.0.0.1:8080` and run `npm run dev` with the default empty
`VITE_API_BASE_URL` so Vite proxies `/auth` and `/v1` same-origin (no CORS
required). Inventory, ops-visibility, and privileged-ops authorization is
recorded in
[QUERY_API_AUTHORIZATION.md](../security/QUERY_API_AUTHORIZATION.md) and
[PRIVILEGED_OPS_AUTHORIZATION.md](../security/PRIVILEGED_OPS_AUTHORIZATION.md).

## Non-claims

This document does not claim production readiness, hosted CI success without a
recorded GitHub Actions run, exactly-once SSE delivery, production
authorization, Identity authentication as a production control, or promotion of
`CAP-008` / `CAP-009` / `CAP-010` / `CAP-012` / `CAP-013` beyond Planned without the
required evidence route.
