# Identity & Operations privileged-ops note

Issue #48 binds Go-owned sessions to `MX-004`/`MX-005`/`MX-006` on
quarantine-release, replay, and rebuild POSTs. Controls reuse tenant-scoped
Event Spine helpers (`relay.ReleaseQuarantined`,
`platform.ReplayTenantHistory`, `platform.RebuildTenantFromHistory`) and
never call global `ResetDerivedState`. Terminal inbox quarantines stay
not-releasable.

This note records documentation and test review only. It does not promote
`CAP-013` or `CLM-010`, and it does not claim a production recovery console
or Traceability backup/restore.

## Acceptance

| Criterion | Disposition |
| --- | --- |
| Unauthorized quarantine/replay/rebuild attempts fail closed | Unauthenticated 401; reader, missing role, nil policy, unassigned principal, and `TENANT-NS-002` return 403 with no mutation |
| Authorized paths are explicit and documented | Operator `MX-004`/`MX-005`/`MX-006` POSTs documented in `PRIVILEGED_OPS_AUTHORIZATION.md` |
| Controls cannot apply another tenant's history | Path tenant plus envelope tenant must match; foreign `event_id` is 404; rebuild leaves other-tenant projection |
| Tests cover missing role, wrong tenant, and forged context | `X-Tenant-ID`, query `tenant_id`, and body `tenant_id` cannot swap the path tenant |

## Non-goals confirmed

No audit product (#49), exit-gate suite (#50), service-identity credentials,
durable assignment schema, demo default-deny bypass, deployment binary, or
`EVIDENCE.md` claim promotion.
