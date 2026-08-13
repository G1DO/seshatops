# Permission Matrix - SeshatOps

**Status:** Frozen Identity & Operations demo allow-list for Issue #44 /
CAP-010 design. This document names tenants, roles, resources, actions, and
allow-list rows so later enforcement can cite stable IDs. Inventory and
ops-visibility query-API evaluation is recorded in
[QUERY_API_AUTHORIZATION.md](QUERY_API_AUTHORIZATION.md).
This matrix does not claim production tenant isolation or `CAP-010` evidence.
Privileged-ops HTTP evaluation is recorded in
[PRIVILEGED_OPS_AUTHORIZATION.md](PRIVILEGED_OPS_AUTHORIZATION.md).
Privileged-decision audit is recorded in
[AUDIT_AUTHORIZATION.md](AUDIT_AUTHORIZATION.md).

**Owns:** Northstar demo tenant identifiers, milestone role labels, resource
and action IDs, explicit allow-list rows mapped to the AUTHORIZATION_MODEL
tuple, privileged versus read classification, and default-deny gaps for this
milestone.

**Does not own:** Runtime enforcement (Issue #46 records inventory query-API
evaluation, Issue #47 records ops-visibility evaluation in
[QUERY_API_AUTHORIZATION.md](QUERY_API_AUTHORIZATION.md), and Issue #48
records privileged-ops evaluation in
[PRIVILEGED_OPS_AUTHORIZATION.md](PRIVILEGED_OPS_AUTHORIZATION.md)), OIDC
login or session runtime (Issue #45), audit recording (Issue #49, recorded in
[AUDIT_AUTHORIZATION.md](AUDIT_AUTHORIZATION.md)), a
policy-engine product, assignment schema, or any `EVIDENCE.md` claim
promotion.

## 1. How to read this matrix

Policy for Identity & Operations is an **explicit allow-list**
([ADR-0005](../adrs/0005-identity-tenant-policy-and-service-delegation.md)).
A missing, unknown, or incomplete entry is deny. UI labels are not authority.

A role name is never an allow-list hit by itself. Downstream work must match
**tenant + role assignment + resource + action** (and the remaining tuple
fields at runtime). Issues [#46](https://github.com/G1DO/seshatops/issues/46),
[#47](https://github.com/G1DO/seshatops/issues/47),
and [#48](https://github.com/G1DO/seshatops/issues/48) must cite the `MX-*`,
`RES-*`, and `ACT-*` IDs below rather than inventing names.

## 2. Tuple mapping

[AUTHORIZATION_MODEL.md](AUTHORIZATION_MODEL.md) §4 evaluates:

> Tenant, principal, resource type, resource identity, action, scope or
> contextual constraints, current resource state, relevant policy and
> assignment version, and current time or freshness context where required.

This matrix freezes the demo values for the first five fields. The rest remain
runtime checks owned by later issues.

| Tuple field | Matrix representation |
| --- | --- |
| Tenant | `TENANT-NS-*` identifier; platform membership, not an IdP or client field |
| Principal | Authenticated principal bound by platform assignment to a `ROLE-*` label in that tenant |
| Resource type | `RES-*` identifier |
| Resource identity | Tenant-scoped `*` for this demo (every resource of that type in the named tenant) |
| Action | `ACT-*` identifier |
| Scope or contextual constraints | Same-tenant only; no cross-tenant or platform-wide scope |
| Current resource state | Runtime; not a matrix row |
| Policy and assignment version | Runtime; not a matrix row |
| Freshness | Runtime session/decision freshness; not a matrix row |

Client-supplied tenant or role fields, IdP claims treated as authority, and
browser-visible buttons are not tuple inputs. A path or header tenant (for
example `{tenant_id}` on `GET /v1/tenants/{tenant_id}/inventory`) is an
assertion to validate against the platform-bound tuple tenant; mismatch is
deny.

## 3. Demo tenants

| ID | Tenant UUID | Seeded in Event Spine | Purpose |
| --- | --- | --- | --- |
| `TENANT-NS-001` | `11111111-1111-4111-8111-111111111111` | Yes (`northstar` fixture) | Northstar Foods demo tenant for allow-list grants |
| `TENANT-NS-002` | `22222222-2222-4222-8222-222222222222` | No | Named second tenant for cross-tenant negative tests |

`TENANT-NS-002` is documentation only. This issue does not add ERP, projection,
or fixture rows for it. There are **no** allow-list rows for `TENANT-NS-002`.

## 4. Roles

Roles are labels bound to tenant, resource, and action. They are not
platform-wide authority.

| ID | PRODUCT persona | Milestone meaning |
| --- | --- | --- |
| `ROLE-OPS-READER` | Operations manager | Same-tenant read path only |
| `ROLE-PLATFORM-OPERATOR` | Platform / security operator | Same-tenant privileged ops only; does not imply inventory read or other-tenant access |

This milestone does not define `tenant_admin`, `approver`, or `planner`. Those
surfaces are later capabilities. An unused or future role name is not an
allow-list hit.

Service identities in ADR-0005 (source/outbox, outbox relay, projection
consumer, ops API) are **not user roles**. They receive **no** `MX-*` rows in
this matrix and cannot substitute for a human role on query or privileged-op
paths (Issue #46 failure case: service identity bypasses resource checks).
Missing service rows mean **deny** on those paths; this table still applies.
It is not a license for internal callers to skip the matrix.

## 5. Resources

| ID | Meaning | Downstream owner |
| --- | --- | --- |
| `RES-INVENTORY-PROJECTION` | Committed inventory projection snapshot and SSE stream (`GET /v1/tenants/{tenant_id}/inventory` and `/stream`) | Issue #46 |
| `RES-OPS-VISIBILITY` | Lag, poison, quarantine, and freshness signals | Issue #47 binds `GET /v1/tenants/{tenant_id}/ops` |
| `RES-QUARANTINE` | Quarantine-release control | Issue #48 |
| `RES-REPLAY` | Controlled replay | Issue #48 |
| `RES-REBUILD` | Derived-state rebuild | Issue #48 |
| `RES-AUDIT` | Privileged authorization-decision audit read | Issue #49 binds `GET /v1/tenants/{tenant_id}/ops/audit` |

Unknown resource types are deny.

## 6. Actions

| ID | Class | Meaning |
| --- | --- | --- |
| `ACT-READ` | Read (non-privileged) | Inspect the named resource in the assigned tenant |
| `ACT-QUARANTINE-RELEASE` | Privileged | Release a quarantined event or processing record |
| `ACT-REPLAY` | Privileged | Execute controlled replay |
| `ACT-REBUILD` | Privileged | Rebuild derived projection state |
| `ACT-AUDIT-READ` | Privileged | Read privileged authorization-decision audit records |

Privileged actions remain default-deny without a matching `MX-*` row. Missing
or unknown actions are deny, never allow. Read is not a substitute for a
privileged action.

## 7. Frozen allow-list

Resource identity is tenant-scoped `*` for every row. Scope is same-tenant
only. No row grants `TENANT-NS-002`. No row grants
`ROLE-PLATFORM-OPERATOR` inventory read. No row grants forecast, RAG,
approval, command, or backup/restore actions.

| ID | Tenant | Role | Resource | Action | Class |
| --- | --- | --- | --- | --- | --- |
| `MX-001` | `TENANT-NS-001` | `ROLE-OPS-READER` | `RES-INVENTORY-PROJECTION` | `ACT-READ` | Read |
| `MX-002` | `TENANT-NS-001` | `ROLE-OPS-READER` | `RES-OPS-VISIBILITY` | `ACT-READ` | Read |
| `MX-003` | `TENANT-NS-001` | `ROLE-PLATFORM-OPERATOR` | `RES-OPS-VISIBILITY` | `ACT-READ` | Read |
| `MX-004` | `TENANT-NS-001` | `ROLE-PLATFORM-OPERATOR` | `RES-QUARANTINE` | `ACT-QUARANTINE-RELEASE` | Privileged |
| `MX-005` | `TENANT-NS-001` | `ROLE-PLATFORM-OPERATOR` | `RES-REPLAY` | `ACT-REPLAY` | Privileged |
| `MX-006` | `TENANT-NS-001` | `ROLE-PLATFORM-OPERATOR` | `RES-REBUILD` | `ACT-REBUILD` | Privileged |
| `MX-007` | `TENANT-NS-001` | `ROLE-PLATFORM-OPERATOR` | `RES-AUDIT` | `ACT-AUDIT-READ` | Privileged |

`MX-002` is same-tenant health visibility for the operations reader. It is not
quarantine release, replay, rebuild, or audit.

## 8. Default-deny gaps

Future enforcement must deny when any of the following is true. These gaps are
intentional; they are not omitted grants.

1. The tenant, role, resource, or action is absent from this matrix.
2. The tenant is missing, ambiguous, or not bound to the principal by platform
   membership.
3. A role name is presented without tenant, resource, and action.
4. A `TENANT-NS-001` principal requests `TENANT-NS-002`, or the reverse.
5. `ROLE-PLATFORM-OPERATOR` requests `RES-INVENTORY-PROJECTION` `ACT-READ`
   (`MX-001` is reader-only).
6. `ROLE-OPS-READER` requests any privileged action (`ACT-QUARANTINE-RELEASE`,
   `ACT-REPLAY`, `ACT-REBUILD`, `ACT-AUDIT-READ`).
7. A service identity is used as a user-role substitute on query or
   privileged-op paths.
8. The requested action is a later-milestone capability: approve, propose,
   retrieve, forecast, command execution, or restore.
9. Resource identity, scope, current state, policy version, or freshness
   cannot be established at runtime.
10. `ACT-REPLAY` or `ACT-REBUILD` would apply another tenant's event history
    or derived state, even when the caller's assigned tenant matches `MX-005`
    or `MX-006`.

A demo must not add bypass rows to make operator work easier.

## 9. Non-claims

Publishing this matrix does not implement authorization, does not itself
promote `CAP-010`, `CLM-007`, or `CLM-008`, and does not select a policy-engine
product. Inventory and ops-visibility query-API evaluation is recorded in
[QUERY_API_AUTHORIZATION.md](QUERY_API_AUTHORIZATION.md). Privileged-ops HTTP
surfaces are recorded in
[PRIVILEGED_OPS_AUTHORIZATION.md](PRIVILEGED_OPS_AUTHORIZATION.md).
Privileged-decision audit is recorded in
[AUDIT_AUTHORIZATION.md](AUDIT_AUTHORIZATION.md). Issue #50 records the
test-environment exit-gate suite in
[IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md](../evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md).

## 10. Related documents

- [ADR-0005](../adrs/0005-identity-tenant-policy-and-service-delegation.md)
- [IDENTITY_BOUNDARIES.md](IDENTITY_BOUNDARIES.md)
- [AUTHORIZATION_MODEL.md](AUTHORIZATION_MODEL.md)
- [THREAT_MODEL.md](THREAT_MODEL.md)
- [QUERY_API_AUTHORIZATION.md](QUERY_API_AUTHORIZATION.md)
- [PRIVILEGED_OPS_AUTHORIZATION.md](PRIVILEGED_OPS_AUTHORIZATION.md)
- [AUDIT_AUTHORIZATION.md](AUDIT_AUTHORIZATION.md)
- [PRODUCT.md](../../PRODUCT.md) (personas)
