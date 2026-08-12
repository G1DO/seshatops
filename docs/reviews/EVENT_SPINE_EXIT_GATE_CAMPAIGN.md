# Event-Spine Exit-Gate Campaign - Issue #30

This document records the Issue #30 integrated event-spine exit-gate campaign. It
evaluates the existing #21–#29 vertical slice under a declared test environment.
It does not claim production readiness, staging parity, exactly-once delivery,
Identity authorization/operations product scope, or Traceability & Recovery backup/restore.

## Campaign record

| Field | Value |
| --- | --- |
| Author | G1DO |
| Verified by | Local `go test ./...`, web Vitest/typecheck/build, hosted CI on PR #41 `b59b760` |
| Date (UTC) | 2026-08-12 |
| Branch | `test/30-m1-exit-gate` |
| Runtime commit verified | `a4e5d47f67f2b4ff1d97760c415c3ea28ad83e47` |
| Scope | Issue #30 only: execute event-spine exit-gate scenarios, record evidence, update `CLM-003`–`CLM-006` and Event Spine fault-matrix rows |
| Kind | Integration/fault evidence, clean-room, claim-status, and verification honesty |
| Runtime disposition | Pass in test environment (Testcontainers + Vitest) |

## Acceptance matrix

| Issue #30 criterion | Evidence | Disposition |
| --- | --- | --- |
| Clean environment reproduces seed → TypeScript path | Procedure + `go test ./...` + `web` Vitest/API suites | Covered (package-integrated; no compose demo binary) |
| Source atomicity/rollback | `TestAcceptOrderRollbackLeavesNoPartialState` | Covered |
| Broker outage preserves source/outbox; publication resumes | `TestRedpandaBrokerOutagePersistenceAndRecovery`, `TestAcceptSurvivesUnreachableBroker` | Covered |
| Ambiguous relay publish/retry remains safe | Relay ambiguous/restart/Redpanda duplicate tests | Covered |
| Duplicate delivery → one projection effect | Duplicate injection + redelivery + Redpanda duplicate tests | Covered |
| Crash before processing commit reprocesses safely | `TestCrashBeforeCommitThenRecover` | Covered |
| Crash after commit/before ack → harmless redelivery | `TestCrashAfterCommitBeforeAckIsDuplicateNoop` | Covered |
| Poison/unsupported/gapped input cannot mutate projection | Platform poison/unsupported/gap/reorder suite | Covered |
| TypeScript view uses Go committed state; converges after reconnect | `api` SSE reconnect + `projection.integration.test.tsx` | Covered |
| Baseline and rebuilt checksums match | `TestDeterministicRebuildChecksumEquality` | Covered |
| Applicable contract/unit/integration/TS/docs/secret/clean-room checks | Local suites pass; hosted CI green on PR #41 `b59b760` | Covered |
| Canonical docs honest; no overclaim | Status docs + non-claims in this review | Covered |
| `CLM-003`–`CLM-006` evidence-backed decisions | Experiment report + ledger update | Observed (test env) |
| No exactly-once delivery/business-effect claim | Exactly-once wording scan (prohibitions only) | Covered |
| Residual risks recorded | This review + experiment report | Covered |

## Verification record

| Check | Result |
| --- | --- |
| `go test ./... -count=1 -timeout 25m` | Passed 2026-08-12 (api/erp/event/northstar/platform/relay all `ok`) |
| `cd web && npm test` | Passed (15 tests) |
| `cd web && npm run typecheck` | Passed |
| `cd web && npm run build` | Passed |
| Exactly-once overclaim scan | Passed; remaining hits are prohibitions/qualifications |
| Ahoy / secret-like category search | Passed; Ahoy only as exclusion; test DB password `seshatops` and poison sanitization fixture are synthetic |
| Hosted Go / Web / Documentation CI | Green on PR #41 commit `b59b760`: Go CI [31588310097](https://github.com/G1DO/seshatops/actions/runs/31588310097), Web CI [31588310148](https://github.com/G1DO/seshatops/actions/runs/31588310148), Documentation CI [31588310157](https://github.com/G1DO/seshatops/actions/runs/31588310157) |
| `EVIDENCE.md` | `CLM-003`–`CLM-006` promoted to Observed with limitations |

## Scenario → fault row mapping

| Scenario cluster | Matrix rows | Claims |
| --- | --- | --- |
| Duplicate / crash windows | FC-001, FC-012, FC-013 | `CLM-004` |
| Broker outage | FC-007 | `CLM-003` |
| Poison / unsupported / gap | FC-009, FC-010, FC-011 | `CLM-005` |
| Deterministic rebuild | FC-014 | `CLM-006` |

Full procedure: [EVENT_SPINE_EXIT_GATE_PROCEDURE.md](../evaluation/EVENT_SPINE_EXIT_GATE_PROCEDURE.md).
Experiment report: [EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](../evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md).

## Clean-room check

- Fixture, topic, tenant, and schema material remain fictional Northstar Foods /
  CONTRACTS.md artifacts.
- No Ahoy code, schemas, data, identifiers, logs, screenshots, or business rules
  were used.
- Evidence artifacts contain no production secrets or private hostnames.
- Checklist record: `CRR-0010` in
  [CLEAN_ROOM_REVIEW.md](../checklists/CLEAN_ROOM_REVIEW.md).

## Residual risk and follow-ups

- Package-level suites ≠ one long-running multi-process demo service.
- Testcontainers topology ≠ staging/production multi-node evidence.
- Live browser demo remains optional; Vitest uses mocked EventSource.
- Operator Go `1.26.2` vs CI Go `1.25.0` difference is recorded.
- At-least-once delivery only; no exactly-once claim.
- Identity owns auth, cross-tenant negatives, operator quarantine UI, and `CLM-007`+.
- Traceability owns backup/restore and ADR-Q-005 recovery product behavior.
- Hosted CI run IDs are recorded for PR #41 commit `b59b760` above.
