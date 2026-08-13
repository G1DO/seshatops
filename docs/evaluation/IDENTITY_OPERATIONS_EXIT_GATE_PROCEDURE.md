# Identity & Operations Exit-Gate Campaign Procedure

**Status:** Executable procedure for Issue #50. This document maps each required
exit-gate scenario to existing package tests from Issues #43–#49. It does not
introduce a new runtime architecture, compose stack, demo binary, policy
engine, or HTTP route.

**Environment class:** Test environment (Testcontainers PostgreSQL for `api`
and identity persistence tests; in-process mock OIDC for session tests;
Vitest for TypeScript presentation).

**Non-claims:** This procedure does not claim production readiness, staging
parity, hosted multi-node topology, production IdP/revocation, service-identity
credentials (`CAP-011`), Traceability & Recovery backup/restore, forecasting,
RAG, approval/command execution, or that hidden UI controls are authorization.

## Preconditions

1. Clean checkout of the candidate Identity & Operations commit on a focused
   branch. Issues #43–#49 must already be present (do not re-implement them).
2. Docker available for Testcontainers.
3. Go `1.25.0` or compatible toolchain used by repository CI; Node.js for `web/`.
4. No secrets or private Ahoy material in the working tree.

Pinned images:

| Dependency | Constant | Label |
| --- | --- | --- |
| PostgreSQL | `erp.PostgresImage` | `16.14` |
| Redpanda | `relay.RedpandaImage` | `v25.2.1` (full `go test ./...` only) |

Frozen matrix: [PERMISSION_MATRIX.md](../security/PERMISSION_MATRIX.md)
(`MX-001`–`MX-007`). Demo tenants: `TENANT-NS-001`
(`11111111-1111-4111-8111-111111111111`) and `TENANT-NS-002`
(`22222222-2222-4222-8222-222222222222`). Fixture seed used by authorized
control tests: `northstar-m1-order-line-v1`.

## Full suite gate

From the repository root:

```bash
go test ./... -count=1 -timeout 25m
cd web && npm test && npm run typecheck && npm run build
```

Hosted GitHub Actions (`go-ci`, `web-ci`, `documentation-ci`) must pass for the
reviewed PR head before hosted-green claims are recorded.

Isolation evidence is **Go HTTP 401/403** (and policy `ErrForbidden`). TypeScript
tests record that a 403 is presented as a request error; they do not prove
tenant isolation.

## Scenario map

Copy commands from the fenced blocks below. Do **not** copy `-run` strings from a
markdown table: table `\|` is a literal pipe in the shell and matches zero tests
while still exiting 0. Each `-run` pattern is anchored `^(A|B|…)$`.

| # | Required scenario | Packages |
| --- | --- | --- |
| 1 | Forged or stale identity fails closed | `./identity` |
| 2 | Unauthenticated HTTP and revoked session/assignment fail closed | `./api` |
| 3 | Cross-tenant and ambiguous-membership policy deny | `./identity` |
| 4 | Cross-tenant HTTP leakage fails closed | `./api` |
| 5 | Forged tenant or actor assertion is not authority | `./api` |
| 6 | Privilege escalation fails closed | `./identity`, `./api` |
| 7 | Authorized same-tenant ops visibility | `./api` |
| 8 | Authorized same-tenant quarantine/replay; gap not releasable | `./api` |
| 9 | Privileged allow/deny audit, insert-fail closed, append-only | `./api` |
| 10 | UI 403 is presentation, not a grant | `web/` Vitest |
| 11 | Docs, secret, clean-room | Category search now; hosted Documentation CI when the PR head exists |

### 1. Forged or stale identity

```bash
go test ./identity -count=1 -timeout 15m -run '^(TestForgedSessionCookieRejected|TestExpiredSessionRejected|TestForgedIDTokenRejected|TestSwappedAudienceRejected|TestSwappedIssuerRejected|TestExpiredIDTokenRejected|TestClientSuppliedPrincipalIgnored|TestLogoutRevokesSession|TestDirectoryClearRevokesMembership)$'
```

### 2. Unauthenticated HTTP and revoked session/assignment

```bash
go test ./api -count=1 -timeout 25m -run '^(TestUnauthenticatedRefusedOnRESTAndSSE|TestSSEStopsAfterSessionRevoked|TestSSEStopsAfterAssignmentRevoked|TestClientPrincipalHeaderDoesNotAuthenticate|TestUnauthenticatedControlFailsClosed|TestUnauthenticatedAuditReadFailsClosed)$'
```

### 3. Cross-tenant and ambiguous-membership policy deny

```bash
go test ./identity -count=1 -timeout 5m -run '^(TestPolicyDeniesCrossTenantInventoryRead|TestPolicyDeniesCrossTenantOpsVisibility|TestPolicyDeniesCrossTenantPrivilegedOps|TestPolicyDeniesMissingOrAmbiguousMembership)$'
```

### 4. Cross-tenant HTTP leakage

```bash
go test ./api -count=1 -timeout 25m -run '^(TestCrossTenantInventoryReadDenied|TestCrossTenantOpsReadDenied|TestPlatformOperatorCrossTenantOpsDenied|TestCrossTenantAndForgedContextControlDenied|TestOperatorRebuildLeavesOtherTenant|TestCrossTenantAndNilPolicyAuditReadDenied|TestAuditReadDoesNotLeakOtherTenant)$'
```

### 5. Forged tenant or actor assertion

```bash
go test ./api -count=1 -timeout 25m -run '^(TestForgedTenantHeaderAndQueryDoNotAuthorize|TestForgedTenantHeaderDoesNotAuthorizeOps|TestClientSuppliedActorAndTenantAreIgnored)$'
```

### 6. Privilege escalation

```bash
go test ./identity -count=1 -timeout 5m -run '^(TestPolicyDeniesPlatformOperatorInventoryRead|TestPolicyDeniesPrivilegedActionsForOpsReader|TestPolicyDeniesUnassignedAndServicePrincipal)$'
go test ./api -count=1 -timeout 25m -run '^(TestPlatformOperatorInventoryReadDenied|TestReaderCannotReleaseQuarantine|TestReaderCannotReplayOrRebuild|TestReaderCannotReadAudit|TestMissingRoleInventoryReadDenied|TestMissingRoleOpsReadDenied|TestMissingRoleControlDenied|TestUnassignedPrincipalInventoryReadDenied|TestUnassignedPrincipalOpsReadDenied|TestUnassignedAndNilPolicyControlDenied|TestNilPolicyFailsClosed|TestNilPolicyOpsFailsClosed)$'
```

### 7. Authorized same-tenant ops visibility

```bash
go test ./api -count=1 -timeout 25m -run '^(TestOpsReaderCanReadSameTenantOps|TestOpsSnapshotExcludesOtherTenantAndPayloadFragments)$'
```

### 8. Authorized same-tenant quarantine/replay; gap not releasable

```bash
go test ./api -count=1 -timeout 25m -run '^(TestOperatorCanReleaseSameTenantQuarantine|TestReleaseGapIsNotReleasable|TestOperatorReplayIsDuplicateNoop)$'
```

### 9. Privileged allow/deny audit

```bash
go test ./api -count=1 -timeout 25m -run '^(TestOperatorReleaseAllowPersistsAudit|TestReaderDenyPersistsAuditWithoutMutation|TestAuditInsertFailureBlocksPrivilegedMutation|TestAuthorizationDecisionsAreAppendOnly|TestOperatorCanReadSameTenantAudit)$'
```

### 10. UI 403 is presentation, not a grant

```bash
cd web && npm test -- src/ui/OperationsView.test.tsx src/state/useOpsControls.test.tsx src/state/useOpsVisibility.test.tsx src/api/client.test.ts
```

These Vitest files do **not** prove isolation.

### 11. Docs, secret, clean-room

Local category search per [CLEAN_ROOM_REVIEW.md](../checklists/CLEAN_ROOM_REVIEW.md).
Hosted Documentation CI is **not executed** until a pull-request head exists;
do not record it as Pass before a green hosted run.

## Security-protocol category disposition

Categories from [SECURITY_EVIDENCE_PROTOCOL.md](SECURITY_EVIDENCE_PROTOCOL.md)
§2–§3. Omission is not coverage; each row is evaluated, unavailable, or not
applicable with a reason.

| Category | Disposition | Reason |
| --- | --- | --- |
| Transactional reads | Evaluated | Inventory REST/SSE and ops GET cross-tenant / forged-header denials |
| Transactional writes | Evaluated (privileged controls only) | Quarantine release, replay, and rebuild POSTs; no general ERP write API |
| Identifier substitution | Evaluated | Foreign `event_id`, path vs header/query/body `tenant_id` swaps |
| Events and asynchronous consumers | Not applicable | Event Spine consumer isolation is Issue #30; this suite does not re-run broker campaigns |
| Commands and durable receipts | Not applicable | No approval/command/receipt surface in Identity & Operations |
| Retrieval candidates and model context | Not applicable | No RAG/retrieval implementation |
| Citations | Not applicable | No citation surface |
| Forecasts and typed proposals | Not applicable | No forecasting or proposal surface |
| Caches | Not applicable | No tenant-aware product cache |
| Search or retrieval indexes | Not applicable | No retrieval index |
| Logs and traces | Evaluated (deny bodies) | Forbidden responses have no snapshot/records/mutation; ops snapshot excludes other-tenant payload fragments |
| Exports and evidence artifacts | Not applicable | No export product |
| Replay | Evaluated | Same-tenant replay/rebuild; foreign history and other-tenant projection left unchanged |
| Backup and restore | Not applicable | Traceability owns restore (ADR-Q-005); `FC-015`/`FC-016` remain Planned |
| Administrative operations | Evaluated | Privileged quarantine/replay/rebuild/audit outside assigned tenant or role fail closed |
| Service-to-service access | Evaluated (deny on user paths) | Unassigned/service principal has no `MX-*` row; no service-identity credential runtime (`CAP-011` Planned) |
| Missing principal / session | Evaluated | Unauthenticated 401; forged/expired cookie and ID token rejected |
| Missing or ambiguous tenant / role | Evaluated | Empty assignment fields, nil policy, unbound principals return 403 |
| Unauthorized action / scope mismatch | Evaluated | Reader cannot privileged-act; operator cannot read inventory (`MX-001`) |
| Stale / revoked session or assignment | Evaluated | Logout, expiry, SSE stop after session revoke, SSE stop after assignment revoke, directory membership clear |
| Direct browser bypass | Evaluated | Go HTTP tests hit routes directly; UI hiding is not the control |
| Approval, receipt, prompt-injection, Python authority | Not applicable | Later capability sequences |

Constitution-era [EVIDENCE.md](../../EVIDENCE.md) `CLM-007` notes that isolation
scope “must include” retrieval, citations, approvals, commands, and exports.
Those surfaces are **not implemented** in Identity & Operations. This campaign
does not silently drop that requirement; it records those categories as Not
applicable and bounds `CLM-007` to the implemented HTTP surfaces.

## Related claims

| Scenario cluster | Claim IDs | Fault matrix |
| --- | --- | --- |
| Cross-tenant HTTP / policy deny | `CLM-007` | None (authorization suite; Event Spine FC rows unchanged) |
| Forged/stale identity and privilege default-deny | `CLM-008` | None |
| Authorized ops visibility | `CLM-009` | None |
| Authorized quarantine/replay/rebuild + audit | `CLM-010` | `FC-015`/`FC-016` remain Planned; not marked Observed |

## Evidence outputs

After execution, record results in:

- [IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md](IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md)
- [IDENTITY_OPERATIONS_EXIT_GATE_CAMPAIGN.md](../reviews/IDENTITY_OPERATIONS_EXIT_GATE_CAMPAIGN.md)
- [EVIDENCE.md](../../EVIDENCE.md) (`CLM-007`–`CLM-010` only when evidence supports promotion)
- [THREAT_MODEL.md](../security/THREAT_MODEL.md) residual risks for this campaign

Do not rewrite Event Spine fault-matrix rows. Do not mark `FC-015` or `FC-016`
Observed.

## Optional live UI walkthrough

The Notion Identity & Operations page lists an operational-console demo as
required evidence. GitHub Issue #50 does not. Disposition: GitHub owns
execution; the live walkthrough is **optional supporting evidence**, same as
Issue #30. Documented in [OPERATIONS_VIEW.md](../architecture/OPERATIONS_VIEW.md)
and `web/README.md`. Serve `api.NewServer(...).Handler()` on `:8080` and run
`npm run dev`. Synthetic Northstar data only. Not required when Go HTTP
negatives and Vitest 403 presentation tests pass.

## Audit timeline citation

Do not invent a new audit product. Cite the sample timeline already recorded in
[IDENTITY_AUDIT.md](../reviews/IDENTITY_AUDIT.md).
