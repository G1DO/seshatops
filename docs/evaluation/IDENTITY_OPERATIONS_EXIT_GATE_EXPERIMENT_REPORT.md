# Identity HTTP test campaign

Issue #50. Forged/stale identity, cross-tenant HTTP, privilege escalation,
authorized same-tenant ops visibility, and authorized same-tenant
quarantine/replay/rebuild with append-only privileged audit. Implemented HTTP
in a test environment only.

Operator G1DO on `test/50-identity-negative-suite`, 2026-08-13. Runtime
packages at `173cedb`. Hosted CI on PR #61 `1f19c58`. Matrix `MX-001`–`MX-007`;
tenants `TENANT-NS-001` / `TENANT-NS-002`; seed `northstar-m1-order-line-v1`.
Single Linux host with Docker Testcontainers (PostgreSQL 16.14) and in-process
mock OIDC. Operator = reviewer. Operator Go `1.26.2` (module targets `1.25.0`).

## Commands

```bash
go test ./... -count=1 -timeout 25m
cd web && npm test && npm run typecheck && npm run build
```

## Denials

Fail-closed HTTP outcomes asserted by `api` tests:

| Case | Representative test | Status | Body / leak check |
| --- | --- | --- | --- |
| Unauthenticated inventory/SSE | `TestUnauthenticatedRefusedOnRESTAndSSE` | 401 | `unauthenticated`; no projection |
| Unauthenticated privileged POST | `TestUnauthenticatedControlFailsClosed` | 401 | no outbox mutation; no audit row |
| Unauthenticated audit GET | `TestUnauthenticatedAuditReadFailsClosed` | 401 | no records body |
| Cross-tenant inventory REST/SSE | `TestCrossTenantInventoryReadDenied` | 403 | `forbidden`; no `"items"` / `"checksum"` |
| Cross-tenant ops GET | `TestCrossTenantOpsReadDenied` | 403 | `forbidden`; no `"backlog"` / `"quarantines"` |
| Cross-tenant / forged-context control | `TestCrossTenantAndForgedContextControlDenied` | 403 or 404 | 403 on `TENANT-NS-002` path; 404 foreign `event_id`; outbox unchanged |
| Cross-tenant audit GET | `TestCrossTenantAndNilPolicyAuditReadDenied` | 403 | `forbidden`; no `"records"` / `"principal_id"` |
| Reader privileged POST | `TestReaderCannotReleaseQuarantine` | 403 | deny audit row; outbox remains quarantined |
| Reader audit GET | `TestReaderCannotReadAudit` | 403 | no records body |
| Nil policy inventory | `TestNilPolicyFailsClosed` | 403 | no projection |
| Nil policy ops | `TestNilPolicyOpsFailsClosed` | 403 | no ops snapshot |
| Forged tenant header/query | `TestForgedTenantHeaderAndQueryDoNotAuthorize` | 403 | header/query not authority |
| Client-supplied audit actor/tenant | `TestClientSuppliedActorAndTenantAreIgnored` | 403 or allow with session actor | body `principal_id`/`tenant_id` ignored |
| Audit insert failure | `TestAuditInsertFailureBlocksPrivilegedMutation` | 500 | `audit_failed`; outbox remains quarantined |
| Policy missing/ambiguous membership | `TestPolicyDeniesMissingOrAmbiguousMembership` | `ErrForbidden` | no HTTP surface; library deny |

Policy denials in `./identity` return `identity.ErrForbidden` without an HTTP body.

## Results

Pass.

| # | Scenario | Result |
| --- | --- | --- |
| 1 | Forged or stale identity | Pass (`identity` session/token negatives + `TestDirectoryClearRevokesMembership`) |
| 2 | Unauthenticated HTTP and revoked session/assignment | Pass (`api` 401 REST/SSE/control/audit; SSE stop after session and assignment revoke) |
| 3 | Cross-tenant and ambiguous-membership policy deny | Pass (`identity` policy tests including `TestPolicyDeniesMissingOrAmbiguousMembership`) |
| 4 | Cross-tenant HTTP leakage | Pass (inventory, ops, controls, audit, rebuild) |
| 5 | Forged tenant or actor assertion | Pass (header/query not authority; client actor/tenant ignored) |
| 6 | Privilege escalation | Pass (reader/operator mismatch, nil policy on inventory and ops, unassigned principals, service principal) |
| 7 | Authorized ops visibility | Pass (`TestOpsReaderCanReadSameTenantOps`) |
| 8 | Authorized quarantine/replay; gap not releasable | Pass |
| 9 | Privileged allow/deny audit | Pass (append-only; deny without mutation; insert failure blocks mutation; operator MX-007 GET) |
| 10 | UI 403 is presentation | Pass (Vitest; not isolation evidence) |

```text
ok  github.com/G1DO/seshatops/api       273.798s
ok  github.com/G1DO/seshatops/erp        43.580s
ok  github.com/G1DO/seshatops/event       0.044s
ok  github.com/G1DO/seshatops/identity    3.443s
ok  github.com/G1DO/seshatops/northstar   0.004s
ok  github.com/G1DO/seshatops/platform  261.993s
ok  github.com/G1DO/seshatops/relay     173.692s
```

Web: 8 files / 36 tests passed; typecheck passed; Vite production-mode
`npm run build` passed. That build is not a production environment.

Hosted CI on `1f19c58`: Go
[31671557442](https://github.com/G1DO/seshatops/actions/runs/31671557442),
Web [31671557513](https://github.com/G1DO/seshatops/actions/runs/31671557513),
Docs [31671557450](https://github.com/G1DO/seshatops/actions/runs/31671557450).

## Limitations

- Single-host Testcontainers is not staging or production.
- Package suites are not one long-lived multi-process binary.
- Vitest covers 403 presentation only.
- Operator and reviewer are the same person.
- Library/test OIDC; process-local sessions and assignments.
- Service-identity credentials are not implemented; tests deny
  unassigned/service principals on user paths.
- Isolation is not claimed for retrieval, citations, approvals, commands,
  exports, or backup/restore; those surfaces do not exist.
