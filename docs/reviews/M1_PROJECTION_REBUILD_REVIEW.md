# M1 Projection Rebuild Review - Issue #29

This document records the Issue #29 implementation review for controlled
duplicate injection, isolated derived-state reset, deterministic projection
rebuild, and CONTRACTS.md §8 checksum equality proofs. It does not claim
exactly-once delivery, production readiness, hosted fault-campaign success,
Observed/Reproduced claim promotion, M2 operator recovery, or M3 backup/restore.

## Review record

| Field | Value |
| --- | --- |
| Reviewer | Implementation pass on branch `feat/29-projection-rebuild`; maintainer review remains a follow-up |
| Date (UTC) | 2026-08-12 |
| Branch | `feat/29-projection-rebuild` |
| Scope | Issue #29 only: `ResetDerivedState`, `RebuildFromHistory`, `HandlerVersion`, FC-001/FC-014-style platform tests, rebuild docs, evidence/matrix notes |
| Review type | Duplicate safety, deterministic rebuild, checksum, clean-room, and verification honesty review |
| Runtime disposition | PostgreSQL + Redpanda library/integration tests; no recovery UI or restore workflow |

## Acceptance matrix

| Issue #29 criterion | Evidence | Disposition |
| --- | --- | --- |
| Baseline deterministic workload produces checksum `A` with reproduction metadata | `TestDeterministicRebuildChecksumEquality`, `ReproductionMetadata` | Covered |
| Controlled identical duplicate does not create an additional inventory effect | `TestDuplicateInjectionLeavesProjectionUnchanged` | Covered |
| Isolated rebuild resets only derived platform state | `ResetDerivedState`, `TestResetDerivedStatePreservesERPAndBrokerInputs` | Covered |
| Replaying unchanged retained history yields checksum `B` with `A == B` | `TestDeterministicRebuildChecksumEquality` | Covered |
| Replay uses recorded event data (not wall clock / unrecorded inputs) | `RebuildFromHistory` → `ProcessRecord` on retained bytes | Covered |
| Gaps / unsupported versions / conflicting identities fail incomplete | `TestRebuildIncompleteOnGap`, `TestRebuildIncompleteOnUnsupportedVersion`, `TestRebuildIncompleteOnConflictingIdentity` | Covered |
| Replay cannot execute an external irreversible effect | `TestRebuildHasNoExternalSideEffectPath`; ProcessRecord writes only `platform` | Covered |
| Checksum computation is repeatable | `TestChecksumRepeatability` | Covered |

## Verification record

| Check | Result |
| --- | --- |
| `go test ./platform -count=1 -timeout 25m -run 'TestChecksumRepeatability\|TestResetDerivedState\|TestDuplicateInjection\|TestDeterministicRebuild\|TestRebuildIncomplete\|TestRebuildHasNoExternal'` | Passed locally on 2026-08-12 with Docker (pinned PostgreSQL + Redpanda images) |
| Hosted Go CI | Not claimed until a GitHub Actions run exists for the reviewed commit |
| Documentation CI | Not claimed until a hosted run exists |
| `EVIDENCE.md` claim promotion | None; `CLM-004` and `CLM-006` remain `Planned` with updated notes |
| FC-001 / FC-014 campaign status | Remain Planned / Not executed; local FC-style tests are not hosted campaign results |

## Reproduction metadata (local FC-014 support note)

| Field | Value |
| --- | --- |
| Seed | `northstar-m1-order-line-v1` |
| Event contract version | `1` (`event.SchemaVersionV1`) |
| Handler version | `m1-inventory-projection-v1` |
| Repository commit | Fill at PR merge with the reviewed commit SHA |
| Broker topic | `seshatops.m1.events` |
| History range (local unit path) | partition `0`, offset `7` for the retained baseline record in `TestDeterministicRebuildChecksumEquality` |
| Checksum inputs | tenant `inventory_projection` rows: `tenant_id`, `item_id`, `quantity_on_hand`, `aggregate_version` (CONTRACTS.md §8) |
| Checksum `A` / `B` | Equal for complete rebuild of the Northstar fixture (`quantity_on_hand=8`, `aggregate_version=1`) |
| Limitations | Local library/integration only; not hosted FC-014; not M2 recovery or M3 restore |

## Duplicate-injection trace (CLM-004 support note)

Local integration path used for duplicate-safety evidence:

1. Seed Northstar inventory and `erp.AcceptOrder`.
2. `relay.DrainOnce` publishes exact outbox bytes to `seshatops.m1.events`.
3. `platform.ConsumeOnce` applies projection and records checksum `A`.
4. Identical bytes are republished; consume yields inbox `duplicate_noop`.
5. Projection quantity/version and checksum remain equal to baseline.

This is local library/integration evidence only. It does not promote `CLM-004`
to Observed or Reproduced.

## Projection rebuild trace (CLM-006 support note)

1. Apply retained Northstar event bytes; record checksum `A` and metadata.
2. `ResetDerivedState` clears only platform inbox/projection/failures.
3. Confirm ERP inventory/orders/outbox bytes unchanged.
4. `RebuildFromHistory` replays the same retained bytes through `ProcessRecord`.
5. Status `complete` and checksum `B` with `A == B`.

Negative paths leave status `incomplete` with explicit reasons and must not be
cited as successful rebuild proofs.

## Clean-room review

- Topic, keys, identifiers, and fixture material remain fictional Northstar Foods
  / CONTRACTS.md artifacts.
- No Ahoy code, schemas, data, identifiers, logs, screenshots, or business rules
  were used.
- Rebuild and duplicate traces retain no secrets or private production data.

## Residual risk and follow-ups

- Issue #30 records test-environment FC-001 / FC-014 campaign evidence and
  Observed decisions for `CLM-004` / `CLM-006`; staging/production promotion
  remains out of scope.
- M2 owns authorized operator replay/quarantine release; M3 owns backup/restore
  and ADR-Q-005 recovery product behavior.
- Incomplete rebuilds may still emit a diagnostic checksum; callers must gate
  equality proofs on status `complete`.
- Hosted Go CI must be observed green for the Issue #30 PR head before citing
  hosted CI success.
