# Identity & Operations privileged-audit note

Issue #49 persists authenticated privileged allows and denies into
append-only `identity.authorization_decisions` and serves
`GET /v1/tenants/{tenant_id}/ops/audit` after `MX-007`. Actor is the
Go-owned session principal. Path tenant is the tuple tenant. Client-supplied
actor or tenant fields are not authority.

This note records documentation and test review plus a sample timeline for
later Issue #50 citation. It does not promote `CAP-010`, `CAP-013`,
`CLM-007`, `CLM-008`, or `CLM-010`, and it does not claim a production audit
or SIEM product.

## Acceptance

| Criterion | Disposition |
| --- | --- |
| Privileged allows/denies create durable audit records | Operator `MX-004` allow and reader `MX-004` deny persist before mutation; unauthenticated `401` creates no row; insert failure is `500` with no outbox mutation |
| Audit access itself is authorized | Operator `MX-007` GET returns 200; reader, nil policy, and `TENANT-NS-002` return 403 with no records body |
| Records are sufficient for an exit-gate timeline example | Principal, action, tenant/resource, reason, timestamp, and allow/deny are present; see sample below |

## Sample audit timeline

Reconstructed from Issue #49 library/test cases on `TENANT-NS-001`
(`11111111-1111-4111-8111-111111111111`). Not hosted-CI or production
evidence.

| Sequence | Principal | Action | Resource | Outcome | Reason | Target |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `operator-northstar` | `ACT-QUARANTINE-RELEASE` | `RES-QUARANTINE` | deny | forbidden | quarantined `event_id` |
| 2 | `platform-operator` | `ACT-QUARANTINE-RELEASE` | `RES-QUARANTINE` | allow | matrix_allow | same `event_id` |
| 3 | `platform-operator` | `ACT-AUDIT-READ` | `RES-AUDIT` | allow | matrix_allow | (none) |

Row 1 does not mutate outbox. Row 2 is recorded before release. Row 3 is the
authorized inspection of that tenant's timeline. A seeded `TENANT-NS-002` row
does not appear in the `TENANT-NS-001` GET.

## Non-goals confirmed

No exit-gate suite (#50), TypeScript audit UI, SIEM/export, inventory or
ops-visibility read auditing, service-identity credentials, durable assignment
schema, demo default-deny bypass, deployment binary, or `EVIDENCE.md` claim
promotion.
