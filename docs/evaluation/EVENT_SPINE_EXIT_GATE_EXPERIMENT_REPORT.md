# Event Spine test campaign

Issue #30. Synthetic Northstar order → outbox → Redpanda → consumer →
projection → REST/SSE → TypeScript view, including rollback, broker outage,
ambiguous publish, duplicates, both consumer crash windows,
poison/unsupported/gap handling, deterministic rebuild checksum equality, and
UI reconnect. Test environment only.

Operator G1DO on `test/30-m1-exit-gate`, 2026-08-12. Runtime packages at
`a4e5d47`. Hosted CI on PR #41 `b59b760`. Seed `northstar-m1-order-line-v1`.
Single Linux host with Docker Testcontainers (PostgreSQL 16.14, Redpanda
v25.2.1). Vitest mocks browser EventSource. Operator Go `1.26.2` (module
targets `1.25.0`).

## Commands

```bash
go test ./... -count=1 -timeout 25m
cd web && npm test && npm run typecheck && npm run build
```

## Results

Pass.

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

```text
ok  github.com/G1DO/seshatops/api       43.938s
ok  github.com/G1DO/seshatops/erp       43.458s
ok  github.com/G1DO/seshatops/event      0.011s
ok  github.com/G1DO/seshatops/northstar  0.014s
ok  github.com/G1DO/seshatops/platform 171.963s
ok  github.com/G1DO/seshatops/relay    129.815s
```

Web: 4 files / 15 tests passed; typecheck passed; production-mode build passed.

Hosted CI on `b59b760`: Go
[31588310097](https://github.com/G1DO/seshatops/actions/runs/31588310097),
Web [31588310148](https://github.com/G1DO/seshatops/actions/runs/31588310148),
Docs [31588310157](https://github.com/G1DO/seshatops/actions/runs/31588310157).

## Limitations

- Single-host Testcontainers is not staging or production.
- Package suites are not one long-lived multi-process binary.
- Vitest covers reconnect with mocked EventSource; live browser walkthrough is
  optional.
- Rebuild proof is Event Spine checksum equality, not backup/restore.
- Does not establish exactly-once delivery, SLO, capacity, or Identity HTTP
  isolation (separate campaign).
