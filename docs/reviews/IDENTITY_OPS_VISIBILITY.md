# Identity & Operations ops-visibility note

Issue #47 binds Go-owned sessions to `MX-002`/`MX-003` on
`GET /v1/tenants/{tenant_id}/ops`. The route reuses tenant-scoped
`InspectBacklogForTenant` and `InspectProcessingForTenant`. Global Event Spine
`InspectBacklog` / `InspectProcessing` remain library verification helpers.

This note records documentation and test review only. It does not promote
`CAP-012` or `CLM-009`, and it does not claim an SLO/alerting platform or
privileged-ops enforcement.

## Acceptance

| Criterion | Disposition |
| --- | --- |
| Authorized viewers can read lag/poison/freshness for their tenant/scope | `ROLE-OPS-READER` and `ROLE-PLATFORM-OPERATOR` on `TENANT-NS-001` receive a 200 ops snapshot |
| Unauthorized callers are denied | Unauthenticated 401; missing role, nil policy, unassigned principal, and `TENANT-NS-002` path (reader and platform operator) return 403 with no snapshot body |
| Docs distinguish library inspect helpers from this product surface | `OPERATIONS_VIEW.md`, `QUERY_API_AUTHORIZATION.md`, outbox/consumer docs |

## Non-goals confirmed

No quarantine/replay mutations (#48), audit product (#49), exit-gate suite
(#50), service-identity credentials, durable assignment schema, demo
default-deny bypass, deployment binary, or `EVIDENCE.md` claim promotion.
