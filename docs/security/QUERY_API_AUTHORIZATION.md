# Query API Authorization - SeshatOps

**Status:** Implemented for Issue #46 library/test scope. This note records
tenant-scoped default-deny on the inventory projection query APIs. It does
not claim that tenant isolation holds across all protected operations, that
privileged ops are enforced, that authorization is a production control, or
that `CAP-010`, `CLM-007`, or `CLM-008` are promoted.

**Owns:** Go evaluation of the frozen permission matrix against platform
assignments for `GET /v1/tenants/{tenant_id}/inventory` and
`.../inventory/stream` (`MX-001` / `RES-INVENTORY-PROJECTION` / `ACT-READ`).

**Does not own:** Ops-visibility HTTP/UI (Issue #47), quarantine/replay/rebuild
controls (Issue #48), audit recording (Issue #49), service-identity credentials
(CAP-011), a policy-engine product, a durable assignment schema, a deployment
service binary, or any `EVIDENCE.md` claim promotion.

## Authentication is not authorization

A fresh Go session is identity only ([OIDC_SESSION.md](OIDC_SESSION.md)).
Inventory reads also require a platform assignment whose tenant UUID, role,
resource, and action hit `MX-001`. Path `{tenant_id}` is an assertion to
validate; client `X-Tenant-ID`, query `tenant_id`, IdP claims, and UI tenant
context are not authority.

## Decision

1. `identity.RequireSession` still returns `401 {"error":"unauthenticated"}`
   when no fresh session exists.
2. After a valid lowercase UUIDv4 path tenant, Go loads the session principal
   and evaluates `Allow(principal, pathTenant, RES-INVENTORY-PROJECTION, ACT-READ)`.
3. Missing, empty, or unmatched membership, `ROLE-PLATFORM-OPERATOR` inventory
   read, unassigned or service-like principals, and cross-tenant paths return
   `403 {"error":"forbidden"}` with no projection body and without starting SSE.
4. A nil authorizer fails closed with 403.
5. Open SSE connections re-check session freshness and `MX-001` on heartbeat
   and before each event. Logout, expiry, or assignment revoke closes the stream.
6. Assignments are process-local memory, not a PostgreSQL policy schema.

The frozen allow-list is [PERMISSION_MATRIX.md](PERMISSION_MATRIX.md). Query
handlers do not expose `RES-OPS-VISIBILITY` or privileged actions.

## UI

TypeScript remains non-authoritative. The operations view may display a 403
as a request error; it cannot grant tenant access.

## Related documents

- [ADR-0005](../adrs/0005-identity-tenant-policy-and-service-delegation.md)
- [PERMISSION_MATRIX.md](PERMISSION_MATRIX.md)
- [OIDC_SESSION.md](OIDC_SESSION.md)
- [PROJECTION_READ_API.md](../architecture/PROJECTION_READ_API.md)
- [AUTHORIZATION_MODEL.md](AUTHORIZATION_MODEL.md)
