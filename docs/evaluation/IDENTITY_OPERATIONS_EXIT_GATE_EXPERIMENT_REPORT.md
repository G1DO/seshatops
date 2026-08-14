# Identity & Operations Exit-Gate Experiment Report

> Issue #50 Identity & Operations exit-gate experiment. Results below are from an actual authorized campaign run.

## Experiment identity

| Field | Value/status |
| --- | --- |
| Experiment ID | `EXP-M2-EXIT-GATE-001` |
| Experiment name | Identity & Operations cross-tenant and privilege negative-test suite |
| Experiment type | Security / authorization negative-test evidence |
| Claim IDs under evaluation | `CLM-007`, `CLM-008`, `CLM-009`, `CLM-010` |
| Prior claim status | Planned |
| Report status | Complete for test-environment campaign; hosted CI recorded for PR #61 `1f19c58` |

## Operator and run timing

| Field | Value/status |
| --- | --- |
| Operator or operator role | G1DO on branch `test/50-identity-negative-suite` |
| Run start timestamp | 2026-08-13T05:08:20Z |
| Run end timestamp | 2026-08-13T05:15:25Z |
| Clock and time-zone assumptions | UTC wall clock on the operator host |
| Experiment duration | Approximately 7 minutes wall time for frozen filters + full Go suite + web checks |

Measured: forged/stale identity fail-closed, cross-tenant HTTP leakage
fail-closed, privilege escalation fail-closed, authorized same-tenant ops
visibility, and authorized same-tenant quarantine/replay/rebuild with
append-only privileged audit. Scoped to implemented HTTP in this Test
environment.

## Environment class and topology

| Field | Value/status |
| --- | --- |
| Environment class | Test environment |
| Environment description | Single Linux developer host with Docker Testcontainers; no staging or production topology |
| Topology and boundaries | Per-package integration: PostgreSQL 16.14 containers for `api`/persistence tests; in-process mock OIDC for session tests; Vitest mocks for browser presentation; no multi-node cluster |
| Operating system and runtime | Linux 7.0.0-28-generic x86_64; Go `go1.26.2` (module targets `1.25.0`); Node `v24.18.0`; Docker `29.4.2` |
| Environment differences and contention | Local Docker contention possible; not isolated CI hardware; Go version newer than CI pin |

## Repository, configuration, and tools

| Field | Value/status |
| --- | --- |
| Repository | `github.com/G1DO/seshatops` |
| Commit | Runtime packages verified at `173cedb82b2d7e4f6b8e25ae778d817811b9820a` (Issue #49 merged on `main`). Campaign docs added after the run on `test/50-identity-negative-suite`. Hosted CI recorded for PR #61 commit `1f19c58`. |
| Branch or tag | `test/50-identity-negative-suite` |
| Dirty-working-tree state | Procedure file present as untracked at suite start; remaining campaign artifacts added after the run |
| Configuration version and relevant values | Frozen matrix `MX-001`–`MX-007`; tenants `TENANT-NS-001` / `TENANT-NS-002`; seed `northstar-m1-order-line-v1` |
| Tool and dependency versions | `erp.PostgresImage` digest pin; Vite `7.3.6`; Vitest `3.2.7`; TypeScript `6.0.3` |

## Dataset, fixture, corpus, or workload provenance

| Field | Value/status |
| --- | --- |
| Dataset, fixture, corpus, or workload identity | Deterministic Northstar Foods Event Spine order-line fixture plus Identity demo tenants |
| Provenance and clean-room independence | Synthetic Northstar fixture and demo allow-list; [CLEAN_ROOM.md](../../CLEAN_ROOM.md), [AUTHORIZATION.md](../security/AUTHORIZATION.md) |
| Version or snapshot | Seed `northstar-m1-order-line-v1`; matrix Issue #44 |
| Generation or preparation method | `northstar.Generate` / package test helpers / in-memory platform assignments |
| Tenant distribution and data boundaries | `TENANT-NS-001` allow-list grants; `TENANT-NS-002` has no `MX-*` rows |
| Seed(s), or reason not applicable | `northstar-m1-order-line-v1` |

## Preconditions

- Docker available for Testcontainers.
- Candidate commit includes Issues #43–#49 merged stack.

## Method

Execute the Issue #50 scenario map via existing automated tests (injection
method = forged cookies/tokens, path vs header/query/body tenant swaps, missing
role, nil policy, unassigned/service principal, and cross-tenant `event_id`).
Capture package exit codes. Perform category searches for Ahoy and secret-like
strings. Do not invent hosted CI run IDs until GitHub Actions executes for the
PR head.

## Exact commands or automation entry points

```bash
go test ./... -count=1 -timeout 25m
cd web && npm test && npm run typecheck && npm run build
```

Do **not** copy `-run` strings from markdown tables: table `\|` is a literal
pipe in the shell. Each pattern below is anchored `^(A|B|…)$`.

```bash
go test ./identity -count=1 -timeout 15m -run '^(TestForgedSessionCookieRejected|TestExpiredSessionRejected|TestForgedIDTokenRejected|TestSwappedAudienceRejected|TestSwappedIssuerRejected|TestExpiredIDTokenRejected|TestClientSuppliedPrincipalIgnored|TestLogoutRevokesSession|TestDirectoryClearRevokesMembership)$'
go test ./api -count=1 -timeout 25m -run '^(TestUnauthenticatedRefusedOnRESTAndSSE|TestSSEStopsAfterSessionRevoked|TestSSEStopsAfterAssignmentRevoked|TestClientPrincipalHeaderDoesNotAuthenticate|TestUnauthenticatedControlFailsClosed|TestUnauthenticatedAuditReadFailsClosed)$'
go test ./identity -count=1 -timeout 5m -run '^(TestPolicyDeniesCrossTenantInventoryRead|TestPolicyDeniesCrossTenantOpsVisibility|TestPolicyDeniesCrossTenantPrivilegedOps|TestPolicyDeniesMissingOrAmbiguousMembership)$'
go test ./api -count=1 -timeout 25m -run '^(TestCrossTenantInventoryReadDenied|TestCrossTenantOpsReadDenied|TestPlatformOperatorCrossTenantOpsDenied|TestCrossTenantAndForgedContextControlDenied|TestOperatorRebuildLeavesOtherTenant|TestCrossTenantAndNilPolicyAuditReadDenied|TestAuditReadDoesNotLeakOtherTenant)$'
go test ./api -count=1 -timeout 25m -run '^(TestForgedTenantHeaderAndQueryDoNotAuthorize|TestForgedTenantHeaderDoesNotAuthorizeOps|TestClientSuppliedActorAndTenantAreIgnored)$'
go test ./identity -count=1 -timeout 5m -run '^(TestPolicyDeniesPlatformOperatorInventoryRead|TestPolicyDeniesPrivilegedActionsForOpsReader|TestPolicyDeniesUnassignedAndServicePrincipal)$'
go test ./api -count=1 -timeout 25m -run '^(TestPlatformOperatorInventoryReadDenied|TestReaderCannotReleaseQuarantine|TestReaderCannotReplayOrRebuild|TestReaderCannotReadAudit|TestMissingRoleInventoryReadDenied|TestMissingRoleOpsReadDenied|TestMissingRoleControlDenied|TestUnassignedPrincipalInventoryReadDenied|TestUnassignedPrincipalOpsReadDenied|TestUnassignedAndNilPolicyControlDenied|TestNilPolicyFailsClosed|TestNilPolicyOpsFailsClosed)$'
go test ./api -count=1 -timeout 25m -run '^(TestOpsReaderCanReadSameTenantOps|TestOpsSnapshotExcludesOtherTenantAndPayloadFragments)$'
go test ./api -count=1 -timeout 25m -run '^(TestOperatorCanReleaseSameTenantQuarantine|TestReleaseGapIsNotReleasable|TestOperatorReplayIsDuplicateNoop)$'
go test ./api -count=1 -timeout 25m -run '^(TestOperatorReleaseAllowPersistsAudit|TestReaderDenyPersistsAuditWithoutMutation|TestAuditInsertFailureBlocksPrivilegedMutation|TestAuthorizationDecisionsAreAppendOnly|TestOperatorCanReadSameTenantAudit)$'
cd web && npm test -- src/ui/OperationsView.test.tsx src/state/useOpsControls.test.tsx src/state/useOpsVisibility.test.tsx src/api/client.test.ts
```

## Safety and termination criteria

Stop on cross-tenant snapshot or audit-record leakage, unauthorized privileged
mutation, fail-open nil policy, or discovery of private/Ahoy/secret material.
No production systems were targeted.

## Raw artifact references and checksums

| Artifact | Reference | Checksum/status |
| --- | --- | --- |
| Identity filter log | Operator host (not committed) | `ok github.com/G1DO/seshatops/identity` for scenarios 1, 3, 6a |
| API filter log | Operator host (not committed) | `ok github.com/G1DO/seshatops/api 112.530s` for scenarios 2, 4–9 |
| Go suite log | Operator host (not committed) | All packages `ok` |
| Web test log | Operator host (not committed) | 8 files / 36 tests passed |
| Typecheck/build logs | Operator host (not committed) | Passed |
| Audit timeline citation | [AUTHORIZATION.md](../security/AUTHORIZATION.md) | Sample timeline from library tests |

Committed raw machine logs are omitted to avoid noisy Testcontainers output;
package names, commands, and exit outcomes are the reproducible references.
HTTP denial outcomes below are taken from the frozen test assertions (status,
`forbidden` body, no snapshot/records/control leak), not from uncommitted
container logs.

## Denial result table

These are the fail-closed HTTP outcomes the frozen `api` tests assert. They are
the Issue #50 raw denial results for `CLM-007`/`CLM-008`.

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

**Observed result:** Pass (test environment)

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
| 11 | Docs/secret/clean-room | Pass (local category search). Hosted Documentation CI [31671557450](https://github.com/G1DO/seshatops/actions/runs/31671557450) on `1f19c58` |

Package summary (`go test ./... -count=1 -timeout 25m`):

```text
ok  github.com/G1DO/seshatops/api       273.798s
ok  github.com/G1DO/seshatops/erp        43.580s
ok  github.com/G1DO/seshatops/event       0.044s
ok  github.com/G1DO/seshatops/identity    3.443s
ok  github.com/G1DO/seshatops/northstar   0.004s
ok  github.com/G1DO/seshatops/platform  261.993s
ok  github.com/G1DO/seshatops/relay     173.692s
```

Web: 8 files / 36 tests passed; typecheck passed; Vite production-mode `npm run build` passed. That build is not a production environment.

## Distributions and slices

Not applicable: authorization negative suite, not a latency or capacity experiment.

## Failures and anomalies

None observed in the recorded suite. Hosted GitHub Actions for PR #61 commit
`1f19c58`: Go CI
[31671557442](https://github.com/G1DO/seshatops/actions/runs/31671557442),
Web CI [31671557513](https://github.com/G1DO/seshatops/actions/runs/31671557513),
Documentation CI
[31671557450](https://github.com/G1DO/seshatops/actions/runs/31671557450).

## Data-integrity checks

- Cross-tenant inventory, ops, audit, and control paths return 403 (or 404 for
  foreign `event_id`) with no unauthorized snapshot, records body, or mutation.
- Forged `X-Tenant-ID`, query `tenant_id`, and body `tenant_id`/`principal_id`
  are not authority.
- Reader deny of quarantine release persists an audit row and leaves outbox
  quarantined.
- Rebuild leaves other-tenant projection unchanged.
- Gap quarantine is not releasable.
- Operator replay of already-applied history is duplicate-noop.
- Append-only: privileged decision rows are not updated or deleted by the
  product path.

## Recovery observations

Not a reliability recovery campaign. Replay of irreversible external effects
and quarantine-recovery failure campaigns remain Planned / Not executed.

## Limitations

- Testcontainers single-host topology is not staging or production.
- Package suites are not one long-lived multi-process demo binary.
- Live browser walkthrough is optional (GitHub #50 does not require the Notion
  console demo; Issue #30 used the same disposition). Vitest covers 403
  presentation only.
- Operator and reviewer are the same person (`G1DO`). This supports Observed,
  not Reproduced.
- Operator Go toolchain (`1.26.2`) differs from CI Go `1.25.0`.
- `TestSSEStopsAfterSessionRevoked` uses a short sleep plus a two-second close
  window; it is not a load/timing SLO.
- Isolation is **not** claimed for retrieval, citations, approvals, commands,
  exports, caches, indexes, or backup/restore; those surfaces do not exist.
- Service-identity credentials are not implemented; tests only deny
  unassigned/service principals on user paths.
- Assignments are process-local memory, not a durable policy schema.
- Claims do **not** establish production authentication, pentest coverage, SLO,
  or Traceability restore.

## Reproduction instructions

1. Check out the reviewed campaign commit.
2. Ensure Docker is available.
3. Run the full suite gate commands above.
4. Compare outcomes to this report; attach hosted CI run IDs when present.

## Reviewer decision

| Field | Value/status |
| --- | --- |
| Reviewer | G1DO |
| Review date | 2026-08-13 |
| Evidence completeness | Complete for declared test-environment scope; hosted CI recorded for PR #61 `1f19c58` |
| Documentation disposition | Pass with recorded limitations |
| Runtime result disposition | Pass |

## New claim status

| Field | Value/status |
| --- | --- |
| New claim status | `CLM-007`–`CLM-010` → **Observed** (test environment) |
| Decision rationale | Named experiment, exact commands, package pass outcomes, and limitations recorded; unimplemented surfaces remain out of scope |
| Evidence links | This report; [EVIDENCE.md](../../EVIDENCE.md) |

## Superseded evidence

Prior Issue #45–#49 review notes that left `CLM-007`–`CLM-010` Planned are
historical support notes; this experiment is the promotion evidence for those
four claims only, scoped to Identity & Operations HTTP surfaces.
