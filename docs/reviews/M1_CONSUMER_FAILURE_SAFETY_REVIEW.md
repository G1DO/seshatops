# M1 Consumer Failure Safety Review - Issue #26

This document records the Issue #26 implementation review for consumer poison,
gap, restart-window, sanitization, and bounded failure/backlog visibility. It
does not claim exactly-once delivery, production readiness, security
enforcement, reliability SLOs, operator recovery controls, or hosted CI success
without a recorded GitHub Actions run.

## Review record

| Field | Value |
| --- | --- |
| Reviewer | Implementation pass on branch `feat/26-consumer-failure-safety`; maintainer review remains a follow-up |
| Date (UTC) | 2026-08-12 |
| Branch | `feat/26-consumer-failure-safety` |
| Scope | Issue #26 only: handler-poison escalation via `ProcessRecord`, `InspectProcessing`, FC-009–012-style platform tests, consumer docs, matrix residual notes |
| Review type | Failure/quarantine safety, restart windows, sanitization, backlog visibility, clean-room, and verification honesty review |
| Runtime disposition | PostgreSQL + Redpanda library/integration tests; no HTTP/SSE service, metrics stack, or operator recovery UI |

## Acceptance matrix

| Issue #26 criterion | Evidence | Disposition |
| --- | --- | --- |
| Crash before PostgreSQL commit is safe reprocess with no partial state | `TestCrashBeforeCommitThenRecover`, `TestRollbackAtomicity` | Covered |
| Crash after commit / before ack is durable no-op on redelivery | `TestCrashAfterCommitBeforeAckIsDuplicateNoop` | Covered |
| Poison/malformed/unsupported input does not mutate projection | `TestExplicitPoisonAttemptEventuallyAcks`, `TestMalformedEnvelopeQuarantinedWithoutProjection`, `TestUnsupportedSchemaQuarantined` | Covered |
| Aggregate-version gap/reorder never silently skipped | `TestAggregateVersionGapAndRedrive`, `TestReorderAggregateVersionIsNotSilentlySkipped` | Covered |
| Durable failure records retain identity/lineage without unrestricted payload | `TestFailureRecordSanitization`; schema in `platform/migrate.sql` | Covered |
| Failure/backlog state inspectable through bounded signal | `InspectProcessing`, `TestInspectProcessingVisibility` | Covered |
| Unsafe event does not permanently stop unrelated aggregates when independent | `TestUnsafeEventDoesNotBlockUnrelatedAggregate` | Covered |
| Operator-controlled recovery deferred to M2 | Consumer docs operator-recovery boundary; no release/replay API | Covered |

## Verification record

| Check | Result |
| --- | --- |
| `go test ./platform -count=1 -timeout 25m` | Passed locally on 2026-08-12 with Docker (pinned PostgreSQL + Redpanda images) |
| Hosted Go CI | Not claimed until a GitHub Actions run exists for the reviewed commit |
| Documentation CI | Not claimed until a hosted run exists |
| `EVIDENCE.md` claim promotion | None; `CLM-004`, `CLM-005`, `CLM-006`, and `CLM-009` remain `Planned` with updated notes |
| FC-009–FC-012 campaign status | Remain Planned / Not executed; local FC-style tests are not hosted campaign results |

## Restart-window traces (FC-012 support note)

Local library/integration paths used for restart evidence:

1. Pre-commit: `testFailBeforeCommit` aborts before PostgreSQL commit; inbox and
   projection counts remain zero; redelivery applies once.
2. Post-commit/pre-ack: first consume commits inbox/projection then skips
   offset commit; republish/redelivery yields `duplicate_noop` with unchanged
   quantity/version.

These are local traces only. They do not promote any claim to Observed or
Reproduced.

## Sanitized failure-record example (synthetic)

Malformed delivery with a fake secret-like substring produces one
`processing_failures` row with:

- `failure_category` / `diagnostic_code` = `malformed_envelope`
- null identity fields when the envelope cannot be formed
- `received_bytes_hash` = SHA-256 of received bytes
- no payload substring retained in inspected columns

## Clean-room review

- Topic, keys, identifiers, and fixture material remain fictional Northstar Foods
  / CONTRACTS.md artifacts.
- No Ahoy code, schemas, data, identifiers, logs, screenshots, or business rules
  were used.
- Failure samples from `InspectProcessing` omit raw payloads and gap
  `event_bytes`.

## Residual risk and follow-ups

- Transient no-ack failures can still head-of-line block a partition until retry
  succeeds; unrelated-aggregate progress is guaranteed for durable quarantine
  decisions, not in-flight database outages.
- `InspectProcessing` is a library verification surface, not M2 operational
  monitoring (`CAP-012` / `CLM-009` remain Planned).
- Operator release-from-quarantine and privileged replay remain M2 (`CAP-013`).
- Issue #27 owns the Go-owned projection query/SSE read surface.
- Hosted Go CI must be observed green before citing CI success.
- Formal FC-009–FC-012 campaign execution remains later evidence work.
