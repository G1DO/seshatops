# SeshatOps web — operations view

Minimal TypeScript screen for the committed Northstar Foods inventory
projection. Business authority stays in the Go `api` / `platform` packages.
Go `identity` owns login and session establishment; the UI only presents
session state and sends cookies.

## Prerequisites

- Node.js `24.14.0` and npm `11.9.0` (see [event-spine.md](../docs/design/specifications/event-spine.md) §9)

The Go runtime under `cmd/seshatops` mounts `identity.Service.Handler()` and
`api.NewServer(...).Handler()` on `:8080`. The local stack builds that runtime
alongside this web image; tests also compose the handlers directly
(`api/*_test.go`, `identity/*_test.go`).

## Setup

```bash
cd web
npm ci
```

## Configure

| Variable | Default | Purpose |
| --- | --- | --- |
| `VITE_API_BASE_URL` | empty (same origin) | Go API origin; empty uses the Vite `/auth` and `/v1` proxy |
| `VITE_API_PROXY_TARGET` | `http://127.0.0.1:8080` | Vite proxy target; the Compose web container sets `http://runtime:8080` |
| `VITE_CACHE_DIR` | `node_modules/.vite` | Vite cache directory; the Compose web container sets `/tmp/seshatops-vite` |
| `VITE_TENANT_ID` | Northstar fixture tenant | Demo tenant context only (not authorization) |
| `VITE_ITEM_ID` | Northstar flour item | Inventory resource whose stockout assessment is displayed; not authorization |

Local UI: leave `VITE_API_BASE_URL` unset so the browser talks to the Vite
origin. Vite proxies `/auth` and `/v1` to the configured
`VITE_API_PROXY_TARGET`, avoiding CORS (the Go packages do not advertise CORS
headers). Use `./scripts/local-stack.sh quickstart` for the combined Go and
web runtime.

Only set an absolute `VITE_API_BASE_URL` when the API origin allows that browser
origin via CORS.

The view talks only to Go (`/auth/*` and `/v1/tenants/{tenant_id}/…`). Allow-list
rows: [authorization.md](../docs/security/authorization.md). REST snapshots
include `quantity_on_hand`, checksum, `observed_at`, and `aggregate_version`.
`last_applied_event_id` arrives on SSE `inventory_projection.updated` frames
only. Connection states: `loading`, `live`, `stale`, `disconnected`, `error`.
On SSE loss the UI re-GETs the snapshot, replaces local state, then reopens
the stream. The stockout panel presents Go-owned prediction risk, uncertainty,
abstention, source freshness, and prediction lineage; stale or unavailable
freshness is never presented as a current confident forecast.

## Scripts

| Script | Purpose |
| --- | --- |
| `npm run dev` | Local Vite dev server (with `/auth` and `/v1` proxy) |
| `npm test` | Vitest unit/component/integration tests |
| `npm run typecheck` | `tsc --noEmit` |
| `npm run build` | Production build to `dist/` |
