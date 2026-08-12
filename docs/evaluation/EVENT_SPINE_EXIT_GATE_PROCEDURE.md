# Event-Spine Exit-Gate Campaign Procedure

**Status:** Executable procedure for Issue #30. This document maps each required
exit-gate scenario to existing package tests. It does not introduce a new
runtime architecture, compose stack, or demo binary.

**Environment class:** Test environment (Testcontainers PostgreSQL + Redpanda;
Vitest for the TypeScript view).

**Non-claims:** This procedure does not claim production readiness, staging
parity, exactly-once delivery, hosted multi-node topology, Identity & Operations operator recovery,
or Traceability & Recovery backup/restore.

## Preconditions

1. Clean checkout of the candidate Event Spine commit on a focused branch.
2. Docker available for Testcontainers.
3. Go `1.25.0` or compatible toolchain used by repository CI; Node.js for `web/`.
4. No secrets or private Ahoy material in the working tree.

Pinned images:

| Dependency | Constant | Label |
| --- | --- | --- |
| PostgreSQL | `erp.PostgresImage` | `16.14` |
| Redpanda | `relay.RedpandaImage` | `v25.2.1` |

Fixture seed: `northstar-m1-order-line-v1`. Handler version:
`m1-inventory-projection-v1`. Topic: `seshatops.m1.events`.

## Full suite gate

From the repository root:

```bash
go test ./... -count=1 -timeout 25m
cd web && npm test && npm run typecheck && npm run build
```

Hosted GitHub Actions (`go-ci`, `web-ci`, `documentation-ci`) must pass for the
reviewed PR head before hosted-green claims are recorded.

## Scenario map

| # | Required scenario | Exact command / tests |
| --- | --- | --- |
| 1 | Normal deterministic order path | `go test ./erp ./relay ./platform -count=1 -timeout 25m -run 'TestAcceptOrderCommitsSourceAndOutboxAtomically\|TestDrainOncePublishesExactBytes\|TestRedpandaNormalPublication\|TestRedpandaFirstDeliveryAndDuplicate\|TestFirstValidEventAppliesProjection'` |
| 2 | Source transaction forced rollback | `go test ./erp -count=1 -timeout 15m -run 'TestAcceptOrderRollbackLeavesNoPartialState'` |
| 3 | Redpanda unavailable then recovered | `go test ./relay -count=1 -timeout 25m -run 'TestRedpandaBrokerOutagePersistenceAndRecovery\|TestAcceptSurvivesUnreachableBroker'` |
| 4 | Ambiguous relay publish/retry | `go test ./relay -count=1 -timeout 25m -run 'TestAmbiguousWindowAllowsDuplicatePublish\|TestDrainOnceAmbiguousMarkPublishedFailure\|TestRedpandaAmbiguousWindowDuplicate\|TestRelayRestartAfterClaimWithoutPublish'` |
| 5 | Duplicate publication/delivery | `go test ./platform -count=1 -timeout 25m -run 'TestIdenticalRedeliveryIsDuplicateNoop\|TestDuplicateInjectionLeavesProjectionUnchanged\|TestRedpandaFirstDeliveryAndDuplicate'` |
| 6 | Consumer crash before DB commit | `go test ./platform -count=1 -timeout 25m -run 'TestCrashBeforeCommitThenRecover'` |
| 7 | Consumer crash after commit / before ack | `go test ./platform -count=1 -timeout 25m -run 'TestCrashAfterCommitBeforeAckIsDuplicateNoop'` |
| 8 | Unsupported/poison event | `go test ./platform -count=1 -timeout 25m -run 'TestExplicitPoisonAttemptEventuallyAcks\|TestUnsupportedSchemaQuarantined\|TestUnsafeEventDoesNotBlockUnrelatedAggregate\|TestMalformedEnvelopeQuarantinedWithoutProjection'` |
| 9 | Aggregate-version gap/reorder | `go test ./platform -count=1 -timeout 25m -run 'TestAggregateVersionGapAndRedrive\|TestReorderAggregateVersionIsNotSilentlySkipped\|TestStaleAggregateVersionQuarantined'` |
| 10 | Projection reset + deterministic rebuild | `go test ./platform -count=1 -timeout 25m -run 'TestDeterministicRebuildChecksumEquality\|TestResetDerivedStatePreservesERPAndBrokerInputs\|TestChecksumRepeatability\|TestRebuildIncomplete'` |
| 11 | REST/SSE disconnect and TypeScript convergence | `go test ./api -count=1 -timeout 25m -run 'TestRESTReturnsCommittedProjection\|TestSSEEmitsOnlyAfterCommit\|TestSSEDisconnectReconnectRESTConverge'`; `cd web && npm test -- src/test/projection.integration.test.tsx` |
| 12 | Contract, docs, secret, clean-room | `go test ./event ./northstar -count=1`; hosted Documentation CI; clean-room category search per `docs/checklists/CLEAN_ROOM_REVIEW.md`; exactly-once wording scan |

## Related claims and fault rows

| Scenario cluster | Fault matrix | Claim IDs |
| --- | --- | --- |
| Duplicate delivery / crash windows | FC-001, FC-012, FC-013 | `CLM-004` |
| Broker outage / outbox durability | FC-007 | `CLM-003` |
| Poison / unsupported / gap / reorder | FC-009, FC-010, FC-011 | `CLM-005` |
| Deterministic rebuild checksum | FC-014 | `CLM-006` |

## Evidence outputs

After execution, record results in:

- [EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md)
- [EVENT_SPINE_EXIT_GATE_CAMPAIGN.md](../reviews/EVENT_SPINE_EXIT_GATE_CAMPAIGN.md)
- [FAULT_CAMPAIGN_MATRIX.md](FAULT_CAMPAIGN_MATRIX.md) (Event Spine rows only)
- [EVIDENCE.md](../../EVIDENCE.md) (`CLM-003`–`CLM-006` only when evidence supports promotion)

## Optional live UI walkthrough

Documented in [OPERATIONS_VIEW.md](../architecture/OPERATIONS_VIEW.md) and
`web/README.md`. Serve `api.NewServer(...).Handler()` on `:8080` and run
`npm run dev`. Synthetic Northstar data only. Optional supporting evidence; not
required when API + Vitest reconnect tests pass.
