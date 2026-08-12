# Event-Spine Exit-Gate Experiment Report

> Filled from [templates/EXPERIMENT_REPORT.md](templates/EXPERIMENT_REPORT.md) for
> Issue #30. Results below are from an actual authorized campaign run.

## Experiment identity

| Field | Value/status |
| --- | --- |
| Experiment ID | `EXP-M1-EXIT-GATE-001` |
| Experiment name | Event Spine integrated exit-gate campaign |
| Experiment type | Reliability / fault / integration evidence |
| Claim IDs under evaluation | `CLM-003`, `CLM-004`, `CLM-005`, `CLM-006` |
| Prior claim status | Planned |
| Report status | Complete for test-environment campaign |

## Operator and run timing

| Field | Value/status |
| --- | --- |
| Operator or operator role | Implementation campaign pass on branch `test/30-m1-exit-gate` |
| Run start timestamp | 2026-08-12T10:29:10Z |
| Run end timestamp | 2026-08-12T10:32:25Z |
| Clock and time-zone assumptions | UTC wall clock on the operator host |
| Experiment duration | Approximately 3 minutes wall time for Go suite + web checks |

## Objective and hypothesis

Decide whether the implemented Event Spine vertical slice satisfies the Issue #30 exit
gate under a declared **Test environment**: synthetic Northstar order → outbox →
Redpanda → consumer → projection → REST/SSE → TypeScript view, including
rollback, broker outage, ambiguous publish, duplicates, both consumer crash
windows, poison/unsupported/gap handling, deterministic rebuild checksum
equality, and UI reconnect convergence.

Hypothesis: existing package-level Go Testcontainers and Vitest suites cover the
required scenarios without new architecture, and evidence supports promoting
`CLM-003`–`CLM-006` to **Observed** scoped to this environment.

## Environment class and topology

| Field | Value/status |
| --- | --- |
| Environment class | Test environment |
| Environment description | Single Linux developer host with Docker Testcontainers; no staging or production topology |
| Topology and boundaries | Per-package integration: PostgreSQL 16.14 + Redpanda v25.2.1 containers for Go packages; Vitest mocks for browser EventSource; no multi-node broker cluster |
| Operating system and runtime | Linux 7.0.0-28-generic x86_64; Go `go1.26.2` (module targets `1.25.0`); Node `v24.18.0`; Docker `29.4.2` |
| Environment differences and contention | Local Docker contention possible; not isolated CI hardware; Go version newer than CI pin |

## Repository, configuration, and tools

| Field | Value/status |
| --- | --- |
| Repository | `github.com/G1DO/seshatops` |
| Commit | Runtime packages verified at `a4e5d47f67f2b4ff1d97760c415c3ea28ad83e47`. Hosted CI verified on PR #41 commit `b59b760af69bbc2754068f5e35b7e0c1682e713b` (prior evidence commit `b6bec19`). |
| Branch or tag | `test/30-m1-exit-gate` |
| Dirty-working-tree state | Clean at suite start; subsequent docs-only campaign artifacts added after the run |
| Configuration version and relevant values | Topic `seshatops.m1.events`; handler `m1-inventory-projection-v1`; schema v1 |
| Tool and dependency versions | `erp.PostgresImage` digest pin; `relay.RedpandaImage` digest pin; Vite `7.3.6`; Vitest `3.2.7`; TypeScript `6.0.3` |

## Dataset, fixture, corpus, or workload provenance

| Field | Value/status |
| --- | --- |
| Dataset, fixture, corpus, or workload identity | Deterministic Northstar Foods Event Spine order-line fixture |
| Provenance and clean-room independence | Synthetic; see `docs/events/SYNTHETIC_DATA_PROVENANCE.md` |
| Version or snapshot | Seed `northstar-m1-order-line-v1` |
| Generation or preparation method | `northstar.Generate` / package test helpers |
| Tenant distribution and data boundaries | Single fictional tenant used by Event Spine fixtures |
| Seed(s), or reason not applicable | `northstar-m1-order-line-v1` |

## Preconditions

- Docker available for Testcontainers.
- Procedure in [EVENT_SPINE_EXIT_GATE_PROCEDURE.md](EVENT_SPINE_EXIT_GATE_PROCEDURE.md).
- Candidate commit includes Issues #21–#29 merged stack.

## Method

Execute the Issue #30 scenario map via existing automated tests (injection
method = library/integration fault hooks and Testcontainers broker
stop/start). Capture package exit codes. Perform category searches for Ahoy,
secret-like strings, and unsupported exactly-once claims. Do not invent hosted
CI run IDs until GitHub Actions executes for the PR head.

## Exact commands or automation entry points

```bash
go test ./... -count=1 -timeout 25m
cd web && npm test && npm run typecheck && npm run build
```

Scenario-focused entry points are listed in
[EVENT_SPINE_EXIT_GATE_PROCEDURE.md](EVENT_SPINE_EXIT_GATE_PROCEDURE.md).

## Safety and termination criteria

Stop on unexpected second projection effect, silent source/outbox loss during
outage, unsupported input mutating projection, incomplete rebuild cited as
success, or discovery of private/Ahoy/secret material. No production systems
were targeted.

## Raw artifact references and checksums

| Artifact | Reference | Checksum/status |
| --- | --- | --- |
| Go suite log | Operator host `/tmp/m1-exit-gate-go-test.log` (not committed) | All packages `ok` |
| Web test log | `/tmp/m1-exit-gate-npm-test.log` | 15/15 Vitest passed |
| Typecheck/build logs | `/tmp/m1-exit-gate-typecheck.log`, `/tmp/m1-exit-gate-build.log` | Passed |
| Duplicate/rebuild traces | Test names in `platform/rebuild_test.go`, `platform/consume_test.go` | Contract §8 equality asserted in-process |
| Procedure | [EVENT_SPINE_EXIT_GATE_PROCEDURE.md](EVENT_SPINE_EXIT_GATE_PROCEDURE.md) | N/A |
| Review | [EVENT_SPINE_EXIT_GATE_CAMPAIGN_REVIEW.md](../reviews/EVENT_SPINE_EXIT_GATE_CAMPAIGN_REVIEW.md) | N/A |

Committed raw machine logs are omitted to avoid noisy binary/testcontainer
output; package names, commands, and exit outcomes are the reproducible
references.

## Results

**Observed result:** Pass (test environment)

| # | Scenario | Result |
| --- | --- | --- |
| 1 | Normal order path | Pass (`erp`/`relay`/`platform` suites) |
| 2 | Source rollback | Pass (`TestAcceptOrderRollbackLeavesNoPartialState`) |
| 3 | Broker outage/recovery | Pass (`TestRedpandaBrokerOutagePersistenceAndRecovery`, `TestAcceptSurvivesUnreachableBroker`) |
| 4 | Ambiguous relay retry | Pass (relay ambiguous/restart tests) |
| 5 | Duplicate delivery | Pass (platform duplicate + Redpanda duplicate) |
| 6 | Crash before commit | Pass (`TestCrashBeforeCommitThenRecover`) |
| 7 | Crash after commit/before ack | Pass (`TestCrashAfterCommitBeforeAckIsDuplicateNoop`) |
| 8 | Poison/unsupported | Pass (poison + unsupported + sanitization tests) |
| 9 | Gap/reorder | Pass (gap/redrive, reorder, stale) |
| 10 | Rebuild checksum A==B | Pass (`TestDeterministicRebuildChecksumEquality`) |
| 11 | REST/SSE + TS reconnect | Pass (`api` + Vitest integration) |
| 12 | Docs/secret/clean-room scans | Pass (category searches; hosted Doc CI [31588310157](https://github.com/G1DO/seshatops/actions/runs/31588310157) on `b59b760`) |

Package summary:

```text
ok  github.com/G1DO/seshatops/api       43.938s
ok  github.com/G1DO/seshatops/erp       43.458s
ok  github.com/G1DO/seshatops/event      0.011s
ok  github.com/G1DO/seshatops/northstar  0.014s
ok  github.com/G1DO/seshatops/platform 171.963s
ok  github.com/G1DO/seshatops/relay    129.815s
```

Web: 4 files / 15 tests passed; typecheck passed; production build passed.

## Distributions and slices

Not applicable: functional/fault campaign, not a latency or capacity experiment.

## Failures and anomalies

None observed in the recorded suite. Hosted GitHub Actions for PR #41 commit
`b59b760` succeeded: Go CI
[31588310097](https://github.com/G1DO/seshatops/actions/runs/31588310097),
Web CI [31588310148](https://github.com/G1DO/seshatops/actions/runs/31588310148),
Documentation CI
[31588310157](https://github.com/G1DO/seshatops/actions/runs/31588310157).

## Data-integrity checks

- Source + outbox atomic commit and rollback leave no partial ERP state.
- Duplicate delivery yields one inventory projection effect (`duplicate_noop`).
- Poison/unsupported/gap paths do not mutate projection; dispositions visible.
- `ResetDerivedState` preserves ERP inputs; complete rebuild checksum equals
  baseline under CONTRACTS.md §8 inputs.
- API/SSE expose only committed projection; web reconnect uses REST catch-up.

## Recovery observations

- Broker outage: accepted source/outbox remain durable; publication resumes after
  recovery.
- Consumer crash before commit: safe reprocess.
- Consumer crash after commit/before ack: harmless redelivery / duplicate noop.
- Ambiguous publish window: at-least-once duplicate publish remains attributable.

## Limitations

- Testcontainers single-host topology is not staging or production.
- Package suites are not one long-lived multi-process demo binary.
- Live browser walkthrough is optional; Vitest covers reconnect with mocked
  EventSource.
- Operator Go toolchain (`1.26.2`) differs from CI Go `1.25.0`.
- Claims do **not** establish exactly-once delivery, SLO, capacity, or
  multi-tenant authorization (Identity).
- `CLM-006` is Event Spine rebuild proof only; Traceability owns authorized recovery/restore
  product scope (ADR-Q-005).

## Reproduction instructions

1. Check out the reviewed campaign commit (PR head after Issue #30 docs land).
2. Ensure Docker is available.
3. Follow [EVENT_SPINE_EXIT_GATE_PROCEDURE.md](EVENT_SPINE_EXIT_GATE_PROCEDURE.md).
4. Run the full suite gate commands above.
5. Compare outcomes to this report; attach hosted CI run IDs when present.

## Reviewer decision

| Field | Value/status |
| --- | --- |
| Reviewer | Implementation campaign pass; maintainer review remains a follow-up |
| Review date | 2026-08-12 |
| Evidence completeness | Complete for declared test-environment scope; hosted CI recorded for PR #41 commit `b59b760` |
| Documentation disposition | Pass with recorded limitations |
| Runtime result disposition | Pass |

## New claim status

| Field | Value/status |
| --- | --- |
| New claim status | `CLM-003`–`CLM-006` → **Observed** (test environment) |
| Decision rationale | Named experiment, exact commands, package pass outcomes, and limitations recorded; distinct from exactly-once or production claims |
| Evidence links | This report; [EVENT_SPINE_EXIT_GATE_CAMPAIGN_REVIEW.md](../reviews/EVENT_SPINE_EXIT_GATE_CAMPAIGN_REVIEW.md); [FAULT_CAMPAIGN_MATRIX.md](FAULT_CAMPAIGN_MATRIX.md); [EVIDENCE.md](../../EVIDENCE.md) |

## Superseded evidence

Prior Issue #23–#29 review notes that left `CLM-003`–`CLM-006` Planned are
historical support notes; this experiment is the promotion evidence for those
four claims only.

## Follow-up work

- Maintainer review of claim promotion and clean-room record.
- Identity+ owns operator quarantine UI, auth, and operational health claims
  (`CLM-007`+).
- Traceability owns backup/restore and authorized recovery product claims.
