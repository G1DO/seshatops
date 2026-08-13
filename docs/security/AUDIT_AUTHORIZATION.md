# Audit Authorization - SeshatOps

**Status:** Implemented for Issue #49 library/test scope. This note records
append-only privileged authorization-decision audit and `MX-007` read.
It does not claim that audit protection is a production control or that a
SIEM/export platform exists. Issue #50 records the Identity & Operations
exit-gate suite in
[IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md](../evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md).

**Owns:** Go persistence and evaluation of:

- Append-only `identity.authorization_decisions` rows for authenticated
  allow/deny on `MX-004`, `MX-005`, `MX-006`, and `MX-007`
- `GET /v1/tenants/{tenant_id}/ops/audit` (`MX-007` / `RES-AUDIT` /
  `ACT-AUDIT-READ`)

**Does not own:** Inventory or ops-visibility read auditing (`MX-001` /
`MX-002` / `MX-003`), SIEM/export, retention, cryptographic audit logs,
backup/restore, a TypeScript audit UI, service-identity credentials, a
policy-engine product, or the Issue #50 exit-gate campaign itself.

## Authentication is not authorization

A fresh Go session is identity only. Audit read requires a platform assignment
that hits `MX-007` for the path tenant. `ROLE-OPS-READER` is deny. Path
`{tenant_id}` is an assertion to validate; client `X-Tenant-ID`, query
`tenant_id`, body `principal_id` or `tenant_id`, IdP claims, and UI buttons
are not authority. AUTH-10: the actor is the Go-owned session principal.

## Write path

Authenticated privileged allow and deny on quarantine-release, replay, rebuild,
and audit-read persist one append-only row **before** a privileged mutation
runs. Insert failure is `500 {"error":"audit_failed"}` with no mutation.
Unauthenticated `401` is not an authorization decision and creates no row.

Each row stores principal, tenant, resource, action, outcome (`allow` or
`deny`), reason, optional target, and timestamp. There is no public POST that
creates audit rows. SQL `UPDATE` and `DELETE` are rejected by trigger.

`api.Server.OnDecision` may still observe the persisted decision in process.

## Read path

1. Sign in through OIDC. A session does not grant audit read.
2. As `ROLE-PLATFORM-OPERATOR` on `TENANT-NS-001`, `GET`
   `/v1/tenants/{tenant_id}/ops/audit`.
3. The response is that path tenant's timeline only. `TENANT-NS-002` is deny
   and does not leak.

The TypeScript operations view does not consume this route.

## Fail closed

Missing session is `401`. Missing role, unassigned/service principal, nil
policy, `TENANT-NS-002`, and forged tenant headers or body fields are `403`
with no `records` body. Cross-tenant rows are never returned on an allow.

TypeScript cannot authorize.
