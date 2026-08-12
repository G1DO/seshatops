# Event Spine Inventory Projection Consumer Review - Issue #25

This document records the Issue #25 implementation review for the transactional
Redpanda consumer that commits inbox/deduplication state with the inventory
projection. It does not claim exactly-once delivery, production readiness,
security enforcement, reliability SLOs, or hosted CI success without a recorded
GitHub Actions run.

## Review record

| Field | Value |
| --- | --- |
| Reviewer | Implementation pass on branch `feat/25-inventory-projection-consumer`; maintainer review remains a follow-up |
| Date (UTC) | 2026-08-12 |
| Branch | `feat/25-inventory-projection-consumer` |
| Scope | Issue #25 only: `platform` package, inbox/projection/failure schema, ProcessRecord dispositions, manual offset commit, checksum helper, integration tests, consumer docs |
| Review type | At-least-once consumption, duplicate-safe inbox, aggregate-version rules, ack-after-commit, clean-room, and verification honesty review |
| Runtime disposition | PostgreSQL + Redpanda library/integration tests; no HTTP/SSE service or operator recovery UI |

## Acceptance matrix

| Issue #25 criterion | Evidence | Disposition |
| --- | --- | --- |
| First valid event creates one inbox record and one inventory effect | `TestFirstValidEventAppliesProjection`, `TestRedpandaFirstDeliveryAndDuplicate` | Covered |
| Identical redelivery does not create a second inventory effect | `TestIdenticalRedeliveryIsDuplicateNoop`, Redpanda duplicate path | Covered |
| Inbox and projection commit in one PostgreSQL transaction | `ProcessRecord` / `applyProjection`; `TestRollbackAtomicity` | Covered |
| Transaction failure commits neither inbox nor projection | `TestRollbackAtomicity` | Covered |
| Same `event_id` with different content is integrity failure | `TestConflictingEventIDRejected` | Covered |
| Aggregate-version rules from #21 enforced | `TestAggregateVersionGapAndRedrive`, `TestStaleAggregateVersionQuarantined` | Covered |
| Tenant/schema/event validation cannot mutate projection | `TestKeyMismatchQuarantined`, `TestMalformedEnvelopeQuarantinedWithoutProjection`, `TestUnsupportedSchemaQuarantined` | Covered |
| Offset commit only after durable processing succeeds | `ConsumeOnce` / `ProcessAndMaybeAck`; crash-window tests | Covered |
| No exactly-once transport claim | Package docs, consumer docs, review non-claims | Covered |

## Verification record

| Check | Result |
| --- | --- |
| `go test ./event ./northstar ./erp ./relay ./platform -count=1 -timeout 25m` | Passed locally on 2026-08-12 with Docker (pinned PostgreSQL + Redpanda images) |
| `go test ./platform -count=1 -timeout 25m` | Passed locally on 2026-08-12 |
| Hosted Go CI | Not claimed until a GitHub Actions run exists for the reviewed commit |
| Documentation CI | Not claimed until a hosted run exists |
| `EVIDENCE.md` claim promotion | None; `CLM-004`, `CLM-005`, and `CLM-006` remain `Planned` with updated notes |

## Duplicate-delivery trace (CLM-004 support note)

Local integration path used for duplicate-safety evidence:

1. Seed Northstar inventory and `erp.AcceptOrder`.
2. `relay.DrainOnce` publishes exact outbox bytes to `seshatops.m1.events`.
3. `platform.ConsumeOnce` applies projection (`quantity_on_hand=8`, `aggregate_version=1`) and inbox `applied`.
4. Identical bytes are republished; consume yields inbox `duplicate_noop` with unchanged projection.

This is local library/integration evidence only. It does not promote `CLM-004`
to Observed or Reproduced.

## Clean-room review

- Topic, keys, identifiers, and fixture material remain fictional Northstar Foods /
  CONTRACTS.md artifacts.
- No Ahoy code, schemas, data, identifiers, logs, screenshots, or business rules
  were used.
- Failure records store sanitized diagnostic codes and optional received-bytes
  hashes; raw payloads are not retained except canonical gap `event_bytes` for
  structurally valid `quarantined_gap` rows.

## Residual risk and follow-ups

- Issue #26 completed bounded failure/backlog observability and handler-poison
  escalation; see
  [EVENT_SPINE_CONSUMER_FAILURE_SAFETY_REVIEW.md](EVENT_SPINE_CONSUMER_FAILURE_SAFETY_REVIEW.md).
- Issue #27 owns the Go-owned projection query/SSE read surface.
- Hosted Go CI must be observed green before citing CI success.
- `CLM-004` / `CLM-005` / `CLM-006` stay Planned until formal evidence promotion
  with required artifacts.
