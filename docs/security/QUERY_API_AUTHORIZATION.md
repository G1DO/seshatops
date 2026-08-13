# Query API Authorization - SeshatOps

**Status:** Implemented for Issue #46 inventory and Issue #47 ops-visibility
library/test scope. This note records tenant-scoped default-deny on those
query APIs. It does not claim that tenant isolation holds across all protected
operations, that privileged ops are enforced, that authorization is a
production control, or that `CAP-010`, `CAP-012`, `CLM-007`, `CLM-008`, or
`CLM-009` are promoted.

**Owns:** Go evaluation of the frozen permission matrix against platform
assignments for:

- `GET /v1/tenants/{tenant_id}/inventory` and `.../inventory/stream`
  (`MX-001` / `RES-INVENTORY-PROJECTION` / `ACT-READ`)
- `GET /v1/tenants/{tenant_id}/ops` (`MX-002` / `MX-003` /
  `RES-OPS-VISIBILITY` / `ACT-READ`)

**Does not own:** Quarantine/replay/rebuild controls (Issue #48), audit
recording (Issue #49), service-identity credentials (CAP-011), a
policy-engine product, a durable assignment schema, a deployment service
binary, SLO/alerting, or any `EVIDENCE.md` claim promotion.

## Authentication is not authorization

A fresh Go session is identity only ([OIDC_SESSION.md](OIDC_SESSION.md)).
Inventory reads require a platform assignment that hits `MX-001`. Ops
visibility requires `MX-002` (`ROLE-OPS-READER`) or `MX-003`
(`ROLE-PLATFORM-OPERATOR`). Path `{tenant_id}` is an assertion to validate;
client `X-Tenant-ID`, query `tenant_id`, IdP claims, and UI tenant context
are not authority.

## Decision

1. `identity.RequireSession` still returns `401 {"error":"unauthenticated"}`
   when no fresh session exists.
2. After a valid lowercase UUIDv4 path tenant, Go loads the session principal
   and evaluates `Allow` for the requested resource and `ACT-READ`.
3. Missing, empty, or unmatched membership, unassigned or service-like
   principals, and cross-tenant paths return `403 {"error":"forbidden"}` with
   no snapshot body and without starting SSE.
4. `ROLE-PLATFORM-OPERATOR` may read ops (`MX-003`) and is denied inventory
   (`MX-001`).
5. A nil authorizer fails closed with 403.
6. Open SSE connections re-check session freshness and `MX-001` on heartbeat
   and before each event. Logout, expiry, or assignment revoke closes the
   stream. Ops visibility is REST-only.
7. Assignments are process-local memory, not a PostgreSQL policy schema.
8. `GET .../ops` reads tenant-scoped `InspectBacklogForTenant` and
   `InspectProcessingForTenant`. Global Event Spine `InspectBacklog` /
   `InspectProcessing` remain library verification helpers, not the product
   surface. Unattributed `processing_failures` rows (`tenant_id` NULL) are
   omitted from the tenant view.

The frozen allow-list is [PERMISSION_MATRIX.md](PERMISSION_MATRIX.md). Query
handlers do not expose privileged actions.

## UI

TypeScript remains non-authoritative. The operations view may display a 403
as a request error; it cannot grant tenant access. Inventory and ops fetches
are independent.

## Related documents

- [ADR-0005](../adrs/0005-identity-tenant-policy-and-service-delegation.md)
- [PERMISSION_MATRIX.md](PERMISSION_MATRIX.md)
- [OIDC_SESSION.md](OIDC_SESSION.md)
- [PROJECTION_READ_API.md](../architecture/PROJECTION_READ_API.md)
- [OPERATIONS_VIEW.md](../architecture/OPERATIONS_VIEW.md)
- [AUTHORIZATION_MODEL.md](AUTHORIZATION_MODEL.md)
