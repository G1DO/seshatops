# Authorization

Default-deny authorization for implemented Identity HTTP. Go evaluates an
explicit allow-list ([ADR-0005](../adrs/0005-identity-tenant-policy-and-service-delegation.md)).
A session is identity only. The UI cannot authorize. Path `{tenant_id}` is an
assertion to validate, not authority. Client headers, query, body, and IdP
claims are not authorization.

Test-environment evidence: [EVIDENCE.md](../EVIDENCE.md). Not production
isolation. Service-identity credentials are not implemented. Residual risk:
library/test OIDC (not production revocation), process-local sessions and
assignments, no pentest.

## Invariants

- Missing, stale, or contradictory context is deny.
- No tenant may read or affect another tenant on these HTTP surfaces.
- Authentication does not imply authorization.
- Browser visibility never grants authority.
- Privileged audit rows are append-only and must persist before a privileged
  mutation; insert failure blocks the mutation.

Implemented Identity HTTP only. Not a production threat model.

| Threat | Control |
| --- | --- |
| Cross-tenant read or mutate | Path tenant is an assertion; allow-list match required; other-tenant payloads are not returned |
| Cookie or session forgery or expiry | Opaque httpOnly cookie; missing, expired, forged, or revoked → `401` before `/v1` |
| UI treated as authorization | Go evaluates the allow-list; the browser cannot authorize |
| Missing, stale, or contradictory context | Deny |
| Privileged mutate without audit | Audit insert must succeed before mutation; insert failure blocks the mutation |

Out of scope here: production IdP revocation, pentest, service-identity
credentials, and deployed network isolation.

## Authentication

Browser operators authenticate through Go (BFF). The browser never holds ID or
access tokens.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/auth/login` | Authorization Code + PKCE; redirect to the configured issuer |
| `GET` | `/auth/callback` | Validate assertion; set session cookie; redirect to `/` |
| `POST` | `/auth/logout` | Delete the server session and clear the cookie |
| `GET` | `/auth/session` | Presentation fields for the UI, or `401` |

Go validates issuer, audience, signature, subject, nonce, and expiry before
`identity.Store` creates a session. Constructor fields (`identity.Config` in
`identity/service.go`): `Issuer`, `ClientID`, `RedirectURL` required;
`Audience` defaults to `ClientID`; `ClientSecret` optional; `SessionTTL`;
`CookieSecure` (false for local HTTP); `CookieName` (default
`seshatops_session`). Cookie is httpOnly, SameSite=Lax, opaque. Sessions are
process-local memory — not a PostgreSQL table. Missing, expired, forged, or
revoked cookies return `401 {"error":"unauthenticated"}` before any `/v1` body
or SSE stream.

## Allow-list

Policy is an explicit allow-list. A role name alone is never a hit. Match
tenant + role assignment + resource + action.

Demo tenants:

| ID | UUID | Purpose |
| --- | --- | --- |
| `TENANT-NS-001` | `11111111-1111-4111-8111-111111111111` | Northstar fixture tenant |
| `TENANT-NS-002` | `22222222-2222-4222-8222-222222222222` | Cross-tenant negative tests; no allow-list rows |

Roles: `ROLE-OPS-READER` (same-tenant read), `ROLE-PLATFORM-OPERATOR`
(same-tenant privileged ops; does not imply inventory read).

| ID | Tenant | Role | Resource | Action |
| --- | --- | --- | --- | --- |
| `MX-001` | `TENANT-NS-001` | `ROLE-OPS-READER` | `RES-INVENTORY-PROJECTION` | `ACT-READ` |
| `MX-002` | `TENANT-NS-001` | `ROLE-OPS-READER` | `RES-OPS-VISIBILITY` | `ACT-READ` |
| `MX-003` | `TENANT-NS-001` | `ROLE-PLATFORM-OPERATOR` | `RES-OPS-VISIBILITY` | `ACT-READ` |
| `MX-004` | `TENANT-NS-001` | `ROLE-PLATFORM-OPERATOR` | `RES-QUARANTINE` | `ACT-QUARANTINE-RELEASE` |
| `MX-005` | `TENANT-NS-001` | `ROLE-PLATFORM-OPERATOR` | `RES-REPLAY` | `ACT-REPLAY` |
| `MX-006` | `TENANT-NS-001` | `ROLE-PLATFORM-OPERATOR` | `RES-REBUILD` | `ACT-REBUILD` |
| `MX-007` | `TENANT-NS-001` | `ROLE-PLATFORM-OPERATOR` | `RES-AUDIT` | `ACT-AUDIT-READ` |

IDs are also constants in `identity/matrix.go`. Deny when the tenant, role,
resource, or action is missing; when the path tenant is not the assigned
tenant; when a platform operator requests inventory read; when a reader
requests a privileged action; when a service principal is used as a user
role. Do not add bypass rows to make a demo easier.

Assignments are process-local memory, not a PostgreSQL policy schema.

## HTTP

| Method | Path | Allow |
| --- | --- | --- |
| `GET` | `/v1/tenants/{tenant_id}/inventory` and `.../inventory/stream` | `MX-001` |
| `GET` | `/v1/tenants/{tenant_id}/ops` | `MX-002` or `MX-003` |
| `POST` | `/v1/tenants/{tenant_id}/ops/quarantine/release` | `MX-004` |
| `POST` | `/v1/tenants/{tenant_id}/ops/replay` | `MX-005` |
| `POST` | `/v1/tenants/{tenant_id}/ops/rebuild` | `MX-006` |
| `GET` | `/v1/tenants/{tenant_id}/ops/audit` | `MX-007` |

After a valid session, Go evaluates `Allow` for the path tenant. Unmatched
membership, unassigned or service-like principals, nil policy, and
cross-tenant paths return `403 {"error":"forbidden"}` with no projection, ops,
or audit payload and without starting SSE. Open inventory SSE re-checks
session and `MX-001` on heartbeat and before each event.

Routes and SSE reconnect/catch-up: [openapi-projection.yaml](../architecture/openapi-projection.yaml).

Privileged POSTs: outbox `quarantined` rows return to `pending`; poison may be
re-driven only with retained same-tenant outbox bytes. Inbox gap/conflict/
stale rows are not force-applied (`409 not_releasable`). Replay is
same-tenant; rebuild resets that tenant's derived platform state only.
Wrong-tenant `event_id` after an allow is `404` and does not mutate the other
tenant.

## Audit

Authenticated allow/deny on `MX-004`–`MX-007` writes one append-only
`identity.authorization_decisions` row **before** a privileged mutation.
Insert failure is `500 {"error":"audit_failed"}` with no mutation.
Unauthenticated `401` creates no row. SQL `UPDATE`/`DELETE` are rejected by
trigger. Audit GET returns that path tenant's timeline only.

Sample (library tests, `TENANT-NS-001`): reader deny on quarantine release
(no mutation); operator allow recorded before release; operator audit-read
allow. A `TENANT-NS-002` row does not appear in the `TENANT-NS-001` GET.
