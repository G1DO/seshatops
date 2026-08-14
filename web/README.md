# SeshatOps web — operations view

Minimal TypeScript screen for the committed Northstar Foods inventory
projection. Business authority stays in the Go `api` / `platform` packages.
Go `identity` owns login and session establishment; the UI only presents
session state and sends cookies.

## Prerequisites

- Node.js `24.14.0` and npm `11.9.0` (see [event-spine.md](../docs/design/specifications/event-spine.md) §9)

There is no `cmd/` binary. `identity.Service.Handler()` serves `/auth/*` and
`api.NewServer(...).Handler()` serves `/v1/*`; they are separate `http.Handler`
values. Tests compose them (`api/*_test.go`, `identity/*_test.go`). A process
that mounts both on `:8080` is not in this repository. Vite still proxies
`/auth` and `/v1` to `http://127.0.0.1:8080` when `VITE_API_BASE_URL` is empty.

## Setup

```bash
cd web
npm ci
```

## Configure

| Variable | Default | Purpose |
| --- | --- | --- |
| `VITE_API_BASE_URL` | empty (same origin) | Go API origin; empty uses the Vite `/auth` and `/v1` proxy |
| `VITE_TENANT_ID` | Northstar fixture tenant | Demo tenant context only (not authorization) |

Local UI: leave `VITE_API_BASE_URL` unset so the browser talks to the Vite
origin. Vite proxies `/auth` and `/v1` → `http://127.0.0.1:8080`, avoiding
CORS (the Go packages do not advertise CORS headers). The UI is exercised by
Vitest; `npm run dev` needs a combined Go process that this repo does not
ship.

Only set an absolute `VITE_API_BASE_URL` when the API origin allows that browser
origin via CORS.

The view talks only to Go (`/auth/*` and `/v1/tenants/{tenant_id}/…`). Allow-list
rows: [authorization.md](../docs/security/authorization.md). REST snapshots
include `quantity_on_hand`, checksum, `observed_at`, and `aggregate_version`.
`last_applied_event_id` arrives on SSE `inventory_projection.updated` frames
only. Connection states: `loading`, `live`, `stale`, `disconnected`, `error`.
On SSE loss the UI re-GETs the snapshot, replaces local state, then reopens
the stream.

## Scripts

| Script | Purpose |
| --- | --- |
| `npm run dev` | Local Vite dev server (with `/auth` and `/v1` proxy) |
| `npm test` | Vitest unit/component/integration tests |
| `npm run typecheck` | `tsc --noEmit` |
| `npm run build` | Production build to `dist/` |
