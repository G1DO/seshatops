# Privileged Ops Authorization - SeshatOps

**Status:** Implemented for Issue #48 library/test scope. This note records
tenant-scoped default-deny on quarantine release, replay, and rebuild
controls. Durable privileged-decision audit is recorded in
[AUDIT_AUTHORIZATION.md](AUDIT_AUTHORIZATION.md). It does not claim that
privileged ops are a production control, or that `CAP-013` or `CLM-010`
are promoted.

**Owns:** Go evaluation of the frozen permission matrix for:

- `POST /v1/tenants/{tenant_id}/ops/quarantine/release` (`MX-004` /
  `RES-QUARANTINE` / `ACT-QUARANTINE-RELEASE`)
- `POST /v1/tenants/{tenant_id}/ops/replay` (`MX-005` / `RES-REPLAY` /
  `ACT-REPLAY`)
- `POST /v1/tenants/{tenant_id}/ops/rebuild` (`MX-006` / `RES-REBUILD` /
  `ACT-REBUILD`)

**Does not own:** Audit recording or `MX-007` read (Issue #49,
[AUDIT_AUTHORIZATION.md](AUDIT_AUTHORIZATION.md)), the
Identity & Operations exit-gate suite (Issue #50), backup/restore, service
identity credentials, a policy-engine product, or any `EVIDENCE.md` claim
promotion.

## Authentication is not authorization

A fresh Go session is identity only. Privileged controls require a platform
assignment that hits `MX-004`, `MX-005`, or `MX-006` for the path tenant.
`ROLE-OPS-READER` is deny. Path `{tenant_id}` is an assertion to validate;
client `X-Tenant-ID`, query `tenant_id`, body `tenant_id`, IdP claims, and UI
buttons are not authority.

## Runbook

1. Sign in through OIDC. A session does not grant release/replay/rebuild.
2. Inspect same-tenant lag and quarantine samples on
   `GET /v1/tenants/{tenant_id}/ops` (`MX-002` or `MX-003`).
3. As `ROLE-PLATFORM-OPERATOR` on `TENANT-NS-001`, POST the control for that
   path tenant only.
4. Outbox `quarantined` rows return to `pending` so the relay may retry.
   Poison failures may be re-driven only through `ProcessRecord` using
   retained same-tenant `erp.outbox` bytes.
5. Inbox `quarantined_gap`, conflict, stale, invalid, mismatch, and
   transition rows are not force-applied (`409 not_releasable`).
6. Replay reprocesses retained same-tenant bytes. Already-applied events are
   `duplicate_noop`. Rebuild resets **that tenant's** derived platform state
   only, then replays. Incomplete rebuilds are not success. ERP source state
   is not mutated. Global `ResetDerivedState` is not used.

## Decision path

Each authenticated allow or deny produces a durable append-only
`identity.authorization_decisions` row (principal, tenant, resource, action,
outcome, reason, target, timestamp) before a privileged mutation runs.
`api.Server.OnDecision` may observe the persisted decision. Details are in
[AUDIT_AUTHORIZATION.md](AUDIT_AUTHORIZATION.md).

## Fail closed

Missing session is `401`. Missing role, unassigned/service principal, nil
policy, `TENANT-NS-002`, and forged tenant headers or body fields are `403`
with no mutation. Wrong-tenant `event_id` after an allow is `404` and does
not mutate the other tenant.

TypeScript may POST these routes and display `403`. It cannot authorize.
