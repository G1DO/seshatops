# Identity & Operations query API default-deny note

Issue #46 binds Go-owned sessions to the frozen permission matrix on inventory
projection REST and SSE reads. Cross-tenant, missing/ambiguous membership,
operator-without-`MX-001`, unassigned/service principals, and forged tenant
headers fail closed. Authorized same-tenant `MX-001` reads still work.

This note records documentation and test review only. It does not promote
`CAP-010`, `CLM-007`, or `CLM-008`, and it does not claim tenant isolation
beyond these query APIs or privileged-ops enforcement.

## Acceptance

| Criterion | Disposition |
| --- | --- |
| Cross-tenant reads are denied | `TENANT-NS-001` reader against `TENANT-NS-002` returns `403 forbidden` with no projection body |
| Missing/ambiguous tenant or role fails closed | Empty assignment fields, nil policy, and unbound principals return 403 |
| Authorized same-tenant reads still work | `ROLE-OPS-READER` on `TENANT-NS-001` REST/SSE still 200 after session |
| Tests cover forged tenant headers / context swaps | `X-Tenant-ID` and query `tenant_id` cannot grant or swap the path tenant |

## Non-goals confirmed

No ops-visibility API (#47), quarantine/replay controls (#48), audit product
(#49), service-identity credentials, durable assignment schema, demo
default-deny bypass, deployment binary, or `EVIDENCE.md` claim promotion.
