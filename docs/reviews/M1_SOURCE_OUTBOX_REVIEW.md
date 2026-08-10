# M1 Source Transaction and Outbox Review - Issue #23

This document records the Issue #23 implementation review for the synthetic ERP
source transaction and transactional outbox persistence. It does not claim
broker publication, projection correctness, security enforcement, reliability,
or production readiness.

## Review record

| Field | Value |
| --- | --- |
| Reviewer | Implementation pass on branch `feat/23-transactional-outbox`; maintainer review remains a follow-up |
| Date (UTC) | 2026-08-10 |
| Branch | `feat/23-transactional-outbox` |
| Scope | Issue #23 only: `erp` schema, AcceptOrder transaction, outbox insert, Northstar seed, PostgreSQL integration tests, persistence docs |
| Review type | Source correctness, atomicity, clean-room, and verification honesty review |
| Runtime disposition | PostgreSQL-backed library/integration tests only; no Redpanda, relay, inbox, or services |

## Acceptance matrix

| Issue #23 criterion | Evidence | Disposition |
| --- | --- | --- |
| Valid order commits source change and one outbox intent atomically | `TestAcceptOrderCommitsSourceAndOutboxAtomically` | Covered |
| Forced failure before commit leaves neither side committed | `TestAcceptOrderRollbackLeavesNoPartialState` | Covered |
| Partial writes before commit leave no committed partial state | Same rollback test after inventory/order/outbox writes | Covered |
| Source transaction has no Redpanda dependency | `TestAcceptOrderHasNoBrokerDependency`, `TestNoBrokerImports` | Covered |
| Outbox preserves Issue #22 event identity/content | Golden hash/bytes compare against `northstar` fixture | Covered |
| Invalid quantity / unknown item / tenant mismatch / bad transition rejected | `TestAcceptOrderDomainValidation` | Covered |
| Concurrent updates preserve inventory invariant | `TestAcceptOrderConcurrentInventoryIntegrity` | Covered |

## Verification record

| Check | Result |
| --- | --- |
| `go test ./event ./northstar ./erp -count=1 -timeout 15m` | Passed locally on 2026-08-10 (erp used Testcontainers + pinned PostgreSQL image) |
| `gofmt` on `erp` | Applied before submission |
| Hosted Go CI | Not claimed until a GitHub Actions run exists for the reviewed commit |
| Documentation CI | Not run locally in this pass; hosted Documentation CI remains required |
| `EVIDENCE.md` claim promotion | None; `CLM-003` remains `Planned` (atomicity tests are not broker-outage evidence) |

## Clean-room review

- Schema names, identifiers, and seed values are fictional Northstar Foods /
  CONTRACTS.md material.
- No Ahoy code, schemas, data, identifiers, logs, screenshots, or business rules
  were used.
- Provenance for the Issue #22 fixture remains authoritative; this issue only
  seeds inventory from that fixture.

## Residual risk and follow-ups

- Issue #24 must publish exact stored outbox bytes and mark `published` only
  after broker acknowledgement.
- Hosted Go CI must be observed green before citing CI success.
- Concurrent integrity is demonstrated for one contended inventory row via
  `SELECT FOR UPDATE`; broader load testing is out of scope.
- `CLM-003` stays Planned until broker-outage recovery evidence exists.
