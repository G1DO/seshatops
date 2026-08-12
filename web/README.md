# SeshatOps web — Event Spine operations view

Minimal TypeScript screen for the committed Northstar Foods inventory
projection. Business authority stays in the Go `api` / `platform` packages.

## Prerequisites

- Node.js `24.14.0` and npm `11.9.0` (see `CONTRACTS.md` §9)
- A process serving `github.com/G1DO/seshatops/api` `NewServer(...).Handler()`
  on `http://127.0.0.1:8080` (no deployment binary is shipped in this issue)

## Setup

```bash
cd web
npm ci
```

## Configure

| Variable | Default | Purpose |
| --- | --- | --- |
| `VITE_API_BASE_URL` | empty (same origin) | Go API origin; empty uses the Vite `/v1` proxy |
| `VITE_TENANT_ID` | Northstar fixture tenant | Demo tenant context only (not Identity auth) |

Local demo (recommended): leave `VITE_API_BASE_URL` unset so the browser talks
to the Vite origin. Vite proxies `/v1` → `http://127.0.0.1:8080`, avoiding CORS
(the Go `api` package does not advertise CORS headers).

```bash
# Terminal A: serve api.NewServer(...).Handler() on :8080
# Terminal B:
npm run dev
```

Only set an absolute `VITE_API_BASE_URL` when the API origin allows that browser
origin via CORS.

## Demo narrative

1. Serve the Go read API over committed projection state on `:8080`.
2. Run `npm run dev` and open the Vite URL; the view loads
   `GET /v1/tenants/{tenant}/inventory` (via proxy).
3. Process the deterministic Northstar synthetic order through the Event Spine spine.
4. The view applies `inventory_projection.updated` SSE frames and shows
   presentation before → after quantities (for example `item-flour-001`:
   `10` → `8`) plus checksum / observed_at / aggregate_version /
   last_applied_event_id from the API.
5. If the stream drops, the banner shows disconnected/stale (not “live
   current”), then REST catch-up reconnects.

## Explicit deferrals

- No authentication or identity screens (Identity+)
- No recovery/quarantine controls
- No dashboard suite or design system

## Scripts

| Script | Purpose |
| --- | --- |
| `npm run dev` | Local Vite dev server (with `/v1` proxy) |
| `npm test` | Vitest unit/component/integration tests |
| `npm run typecheck` | `tsc --noEmit` |
| `npm run build` | Production build to `dist/` |

## Docs

- [OPERATIONS_VIEW.md](../docs/architecture/OPERATIONS_VIEW.md)
- [PROJECTION_READ_API.md](../docs/architecture/PROJECTION_READ_API.md)
- [EVENT_SPINE_COMPLETION_SUMMARY.md](../docs/reviews/EVENT_SPINE_COMPLETION_SUMMARY.md)
