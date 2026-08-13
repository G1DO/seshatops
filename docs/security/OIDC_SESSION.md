# OIDC Login and Session Runtime - SeshatOps

**Status:** Implemented for Issue #45 library/test scope. This note records the
OIDC Authorization Code + PKCE relying-party flow and Go-owned session gate.
It does not claim production authentication or production tenant isolation.
Issue #50 records the Identity & Operations exit-gate suite in
[IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md](../evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md).
Inventory query-API default-deny is recorded in
[QUERY_API_AUTHORIZATION.md](QUERY_API_AUTHORIZATION.md).

**Owns:** Configurable OIDC relying-party integration, opaque Go-owned
sessions, unauthenticated refusal on projection `/v1` routes, logout, and
session presentation for the operations UI.

**Does not own:** Tenant/role/resource/action default-deny (Issue #46,
recorded in [QUERY_API_AUTHORIZATION.md](QUERY_API_AUTHORIZATION.md)),
IdP vendor selection, durable session storage, secret management, a
deployment service binary, or the Issue #50 exit-gate campaign itself.

## Authentication is not authorization

A validated OIDC assertion and a fresh Go session establish **identity only**.
They do not grant tenant membership, role authority, approval authority, or
command authority (AUTH-05). Client-supplied principal, tenant, or role
headers are ignored. Inventory query authorization is recorded in
[QUERY_API_AUTHORIZATION.md](QUERY_API_AUTHORIZATION.md).

## Flow

Browser operators authenticate through the Go backend (BFF). The browser never
holds ID or access tokens.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/auth/login` | Start Authorization Code + PKCE; redirect to the configured issuer |
| `GET` | `/auth/callback` | Validate assertion; establish session cookie; redirect to `/` |
| `POST` | `/auth/logout` | Delete the server session and clear the cookie |
| `GET` | `/auth/session` | Presentation fields for the UI (or `401`) |

Go validates issuer, audience, signature, subject binding, nonce, and expiry
before `identity.Store` creates a session. Forged, swapped, stale, or
contradictory assertions are rejected and no session is issued.

Session cookie: `seshatops_session`, httpOnly, SameSite=Lax, opaque store key.
Login binds OIDC `state` to the initiating browser with `seshatops_login`
(httpOnly, SameSite=Lax). Callbacks without a matching cookie are rejected and
do not consume the pending PKCE verifier. Abandoned pending logins expire and
are evicted.

Minimum session fields: principal identifier, issuer+subject binding,
authentication time, expiry, correlation id.

Protected routes (`GET /v1/tenants/{tenant_id}/inventory` and
`.../inventory/stream`) require a fresh session. Missing, unknown, expired, or
revoked cookies return `401 {"error":"unauthenticated"}` before any projection
body or SSE stream is written. An already-open SSE connection is rechecked on
heartbeat and before each event; logout or expiry closes the stream.
Issue #46 also re-checks `MX-001` on those intervals.

## Configuration

IdP product remains configuration-time. Package `identity` takes issuer,
client id, redirect URL, optional audience (defaults to client id), session
TTL, and cookie Secure flag. Tests use an in-process mock OpenID provider.

The session store is process-local memory. There is no deployment binary and
no PostgreSQL session table in this issue.

## UI

The TypeScript operations view sends cookies (`credentials: "include"`),
presents principal and expiry from `GET /auth/session`, starts login via
`GET /auth/login`, and posts logout. Vite proxies `/auth` and `/v1` to the Go
listener. The UI is not an enforcement point.

## Related documents

- [ADR-0005](../adrs/0005-identity-tenant-policy-and-service-delegation.md)
- [IDENTITY_BOUNDARIES.md](IDENTITY_BOUNDARIES.md)
- [AUTHORIZATION_MODEL.md](AUTHORIZATION_MODEL.md)
- [PERMISSION_MATRIX.md](PERMISSION_MATRIX.md)
- [QUERY_API_AUTHORIZATION.md](QUERY_API_AUTHORIZATION.md)
