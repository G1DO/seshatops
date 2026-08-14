# SeshatOps web — Event Spine operations view

Minimal TypeScript screen for the committed Northstar Foods inventory
projection. Business authority stays in the Go `api` / `platform` packages.
Go `identity` owns login and session establishment; the UI only presents
session state and sends cookies.

## Prerequisites

- Node.js `24.14.0` and npm `11.9.0` (see `CONTRACTS.md` §9)
- Optional: a process serving `identity.Service.Handler()` (`/auth/*`) and
  `api.NewServer(...).Handler()` (`/v1/*`) on `http://127.0.0.1:8080`

There is no `cmd/` binary. Packages are exercised by tests (`api.NewServer`
and `identity` constructors in `api/*_test.go` and `identity/*_test.go`).
Serving them together on `:8080` is optional for a local UI walkthrough.
A typical wiring is `api.NewHub()`, `platform.SetAppliedNotifier(hub)`, then
`api.NewServer(db, hub, auth, policy)` plus `identity.Service.Handler()`.

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

Local demo (recommended): leave `VITE_API_BASE_URL` unset so the browser talks
to the Vite origin. Vite proxies `/auth` and `/v1` → `http://127.0.0.1:8080`,
avoiding CORS (the Go packages do not advertise CORS headers).

```bash
# Terminal A: serve identity.Handler() and api.NewServer(...).Handler() on :8080
# Terminal B:
npm run dev
```

Only set an absolute `VITE_API_BASE_URL` when the API origin allows that browser
origin via CORS.

## Demo narrative

1. Serve the Go identity and read API over committed projection state on `:8080`.
2. Run `npm run dev` and open the Vite URL; Sign in redirects through
   `GET /auth/login` (OIDC Authorization Code + PKCE).
3. After a Go-owned session cookie is set, the view loads
   `GET /v1/tenants/{tenant}/inventory` and
   `GET /v1/tenants/{tenant}/ops` (via proxy).
4. Process the deterministic Northstar synthetic order through the Event Spine.
5. The view applies `inventory_projection.updated` SSE frames and shows
   presentation before → after quantities (for example `item-flour-001`:
   `10` → `8`) plus checksum / observed_at / aggregate_version /
   last_applied_event_id from the API.
6. If the stream drops, the banner shows disconnected/stale (not “live
   current”), then REST catch-up reconnects.

A session is identity only. Inventory reads also require `MX-001` for the
path tenant. Ops visibility requires `MX-002` or `MX-003` (Go-owned; the UI
cannot authorize). Privileged POSTs require `MX-004`, `MX-005`, or `MX-006`.

The view talks only to Go:

| Method | Path |
| --- | --- |
| `GET` | `/auth/login`, `/auth/callback`, `/auth/session` |
| `POST` | `/auth/logout` |
| `GET` | `/v1/tenants/{tenant_id}/inventory` and `.../inventory/stream` |
| `GET` | `/v1/tenants/{tenant_id}/ops` |
| `POST` | `/v1/tenants/{tenant_id}/ops/quarantine/release`, `.../replay`, `.../rebuild` |

Connection states: `loading`, `live`, `stale`, `disconnected`, `error`. On SSE
loss the UI re-GETs the snapshot, replaces local state, then reopens the
stream. SSE is a live hint, not a gapless history. The API exposes current
`quantity_on_hand`; previous → current is presentation state.

## Explicit deferrals

- No dashboard suite or design system
- No SLO/alerting platform

## Scripts

| Script | Purpose |
| --- | --- |
| `npm run dev` | Local Vite dev server (with `/auth` and `/v1` proxy) |
| `npm test` | Vitest unit/component/integration tests |
| `npm run typecheck` | `tsc --noEmit` |
| `npm run build` | Production build to `dist/` |

## Docs

- [AUTHORIZATION.md](../docs/security/AUTHORIZATION.md)
- [openapi-projection.yaml](../docs/architecture/openapi-projection.yaml)
