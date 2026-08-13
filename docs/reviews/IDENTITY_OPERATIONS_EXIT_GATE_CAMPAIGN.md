# Identity & Operations Exit-Gate Campaign - Issue #50

This document records the Issue #50 cross-tenant and privilege negative-test
suite. It evaluates the existing #43–#49 Identity & Operations surfaces under a
declared test environment. It does not claim production readiness, staging
parity, production IdP/revocation, service-identity credentials, Traceability
& Recovery backup/restore, forecasting, RAG, or approval/command execution.

## Campaign record

| Field | Value |
| --- | --- |
| Author | G1DO |
| Verified by | Local `go test ./...`, frozen `identity`/`api` filters, web Vitest/typecheck/build |
| Date (UTC) | 2026-08-13 |
| Branch | `test/50-identity-negative-suite` |
| Runtime commit verified | `173cedb82b2d7e4f6b8e25ae778d817811b9820a` |
| Scope | Issue #50 only: freeze and execute identity negatives, record evidence, update `CLM-007`–`CLM-010` |
| Kind | Authorization negative-test evidence, clean-room, claim-status, and verification honesty |
| Runtime disposition | Pass in test environment (Testcontainers + mock OIDC + Vitest) |

## Acceptance matrix

| Issue #50 criterion | Evidence | Disposition |
| --- | --- | --- |
| Suite is listed and reproducible | Fenced anchored `-run` blocks in the procedure (not table-escaped pipes) | Covered |
| Cross-tenant leakage cases fail closed | Policy + HTTP inventory/ops/control/audit/rebuild tests | Covered |
| Privileged ops without permission fail closed | Reader/missing-role/nil-policy/unassigned/service-principal tests | Covered |
| Forged/stale identity fails closed | Session cookie, expiry, forged/swapped/expired ID token, assignment revoke, client principal header | Covered |
| Evidence artifacts linked; claim promotions only with required fields | Experiment report + `EVIDENCE.md` | Observed (test env) |
| Suite does not pass only because UI hid buttons | Go HTTP 401/403 is the control; Vitest records 403 as a request error | Covered |
| No production/staging, forecast/RAG/approval/command, backup/restore, or demo bypass | Explicit non-goals in procedure and this review | Covered |
| Residual risks recorded | This review + experiment report + threat-model §9 | Covered |

## Verification record

| Check | Result |
| --- | --- |
| Frozen `go test ./identity` filters | Passed 2026-08-13; copy-paste anchored `-run` filters re-checked 2026-08-13 including `TestPolicyDeniesMissingOrAmbiguousMembership` and `TestDirectoryClearRevokesMembership` |
| Frozen `go test ./api` added fail-closed tests | Passed 2026-08-13 (`ok` 26.089s): assignment-revoke SSE, unassigned inventory/ops, nil-policy ops, client actor ignore, audit insert failure, operator audit GET |
| Frozen `go test ./api` original filters | Passed (`ok` 112.530s; scenarios 2, 4–9 as first recorded) |
| `go test ./... -count=1 -timeout 25m` | Passed 2026-08-13 (api/erp/event/identity/northstar/platform/relay all `ok`) |
| `cd web && npm test` | Passed (36 tests) |
| `cd web && npm run typecheck` | Passed |
| `cd web && npm run build` | Passed |
| Ahoy / secret-like category search | Passed; Ahoy only as exclusion; test DB password `seshatops` and mock OIDC tokens are synthetic |
| Hosted Go / Web / Documentation CI | Pass on PR #61 `1f19c58`: Go [31671557442](https://github.com/G1DO/seshatops/actions/runs/31671557442), Web [31671557513](https://github.com/G1DO/seshatops/actions/runs/31671557513), Docs [31671557450](https://github.com/G1DO/seshatops/actions/runs/31671557450) |
| `EVIDENCE.md` | `CLM-007`–`CLM-010` promoted to Observed with limitations |

## Scenario → claim mapping

| Scenario cluster | Claims | Fault matrix |
| --- | --- | --- |
| Cross-tenant HTTP / policy deny | `CLM-007` | None (Event Spine FC rows unchanged) |
| Forged/stale identity and privilege default-deny | `CLM-008` | None |
| Authorized ops visibility | `CLM-009` | None |
| Authorized quarantine/replay/rebuild + audit | `CLM-010` | `FC-015`/`FC-016` remain Planned |

Full procedure: [IDENTITY_OPERATIONS_EXIT_GATE_PROCEDURE.md](../evaluation/IDENTITY_OPERATIONS_EXIT_GATE_PROCEDURE.md).
Experiment report: [IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md](../evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md).
Audit timeline citation: [IDENTITY_AUDIT.md](IDENTITY_AUDIT.md).

## Contradiction disposition (`CLM-007` constitution-era scope)

[EVIDENCE.md](../../EVIDENCE.md) originally noted that tenant-isolation scope
must include reads, retrieval, citations, approvals, commands, exports, audit,
replay, and recovery. Issue #50 out-of-scope excludes forecasting, RAG,
approval/command execution, and backup/restore.

Disposition: this campaign evaluates Identity & Operations implemented HTTP
surfaces (inventory, ops, quarantine, replay, rebuild, audit, sessions). Later
surfaces are recorded as Not applicable with reasons in the procedure. The
ledger limitation column states that bound; the constitution-era remaining
categories are not silently rewritten as covered.

## Contradiction disposition (Notion console demo)

The Notion Identity & Operations page lists an operational-console demo as
required evidence. GitHub Issue #50 does not. GitHub owns execution: the live
walkthrough is optional supporting evidence, same as Issue #30.

## Clean-room check

- Tenant UUIDs, matrix IDs, and fixtures remain fictional Northstar Foods /
  CONTRACTS.md artifacts.
- No Ahoy code, schemas, data, identifiers, logs, screenshots, or business rules
  were used.
- Evidence artifacts contain no production secrets or private hostnames.
- Checklist record: `CRR-0011` in
  [CLEAN_ROOM_REVIEW.md](../checklists/CLEAN_ROOM_REVIEW.md).

## Residual risk and follow-ups

- Package-level suites ≠ one long-running multi-process demo service.
- Testcontainers topology ≠ staging/production multi-node evidence.
- Live browser demo remains optional (GitHub #50 vs Notion required-evidence
  contradiction recorded above); Vitest uses mocked EventSource/fetch.
- Operator and reviewer are the same person (`G1DO`); Observed, not Reproduced.
- Operator Go `1.26.2` vs CI Go `1.25.0` difference is recorded.
- Process-local assignments ≠ durable policy schema.
- `CAP-011` service-identity credentials remain unimplemented.
- Production authentication, pentest, and policy-engine verification do not exist.
- Traceability owns backup/restore and ADR-Q-005 recovery product behavior.
- Hosted CI run IDs are not recorded for this campaign yet.
