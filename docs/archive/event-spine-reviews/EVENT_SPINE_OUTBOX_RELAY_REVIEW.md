# Event Spine Outbox Relay Review - Issue #24

This document records the Issue #24 implementation review for the source-owned
outbox relay that publishes durable `erp.outbox` records to Redpanda. It does
not claim projection correctness, exactly-once delivery, security enforcement,
reliability SLOs, or production readiness.

## Review record

| Field | Value |
| --- | --- |
| Author | G1DO |
| Branch | `feat/24-redpanda-outbox-relay` |
| Date (UTC) | 2026-08-11 |
| Branch | `feat/24-redpanda-outbox-relay` |
| Scope | Issue #24 only: `relay` package, claim/lease/backoff/quarantine, franz-go publish, backlog inspect, Redpanda pin, integration tests, relay docs |
| Review type | At-least-once publication, duplicate-safe ambiguous window, clean-room, and verification honesty review |
| Runtime disposition | PostgreSQL + Redpanda library/integration tests; no projection consumer, inbox, or services |

## Acceptance matrix

| Issue #24 criterion | Evidence | Disposition |
| --- | --- | --- |
| Source transactions commit while Redpanda is unavailable | `TestAcceptSurvivesUnreachableBroker`, `TestRedpandaBrokerOutagePersistenceAndRecovery` | Covered |
| Pending outbox survives broker outage and relay restart | Outage + `TestRelayRestartAfterClaimWithoutPublish` | Covered |
| Eligible durable rows publish after recovery without silent loss | Outage recovery drain + consume assertion | Covered |
| Retries preserve event identity and content | Ambiguous-window and recovery identity checks | Covered |
| Ack-before-status crash may duplicate, never erases intent | `TestAmbiguousWindowAllowsDuplicatePublish`, `TestDrainOnceAmbiguousMarkPublishedFailure`, `TestRedpandaAmbiguousWindowDuplicate` | Covered |
| Unpublished/quarantine failures are inspectable | `InspectBacklog` tests | Covered |
| Topic/key policy only | `seshatops.m1.events` + aggregate key from parsed envelope | Covered |
| No exactly-once wording or transport guarantee | Package docs, `DisableIdempotentWrite`, `OUTBOX_RELAY.md` non-claims | Covered |

## Verification record

| Check | Result |
| --- | --- |
| `go test ./event ./northstar ./erp ./relay -count=1 -timeout 25m` | Passed locally on 2026-08-11 with Docker Desktop (pinned PostgreSQL + Redpanda images) |
| Pure unit tests (`TestBackoffAfterAttempts`, `TestAggregateKey`, `TestRedpandaImagePinDocumented`) | Pass without Docker |
| Hosted Go CI | Not claimed until a GitHub Actions run exists for the reviewed commit |
| Documentation CI | Not claimed until a hosted run exists |
| `EVIDENCE.md` claim promotion | None; `CLM-003` and `CLM-005` remain `Planned` |

## Clean-room review

- Topic, keys, identifiers, and fixture material remain fictional Northstar Foods /
  CONTRACTS.md artifacts.
- No Ahoy code, schemas, data, identifiers, logs, screenshots, or business rules
  were used.
- Broker client is `github.com/twmb/franz-go` for Kafka-compatible produce only;
  no CDC/Debezium or second broker.

## Residual risk and follow-ups

- Issue #25 (or later) must consume with inbox/projection idempotency for
  expected duplicates.
- Hosted Go CI must be observed green before citing CI success.
- Backpressure capacity limits remain undeclared (CONTRACTS deferred decisions).
- `CLM-003` / `CLM-005` stay Planned until formal evidence promotion with
  required artifacts.
