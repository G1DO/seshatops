# Event Spine Projection Rebuild and Duplicate Proof

**Status:** Implemented for Issue #29 library/integration scope. Issue #30
records test-environment Observed evidence for `CLM-004` and `CLM-006`.
Traceability & Recovery backup/restore is not introduced. Issue #48 operator
HTTP is recorded in
[PRIVILEGED_OPS_AUTHORIZATION.md](../security/PRIVILEGED_OPS_AUTHORIZATION.md).

**Owns:** Controlled duplicate-injection harness coverage, the documented use of
the CONTRACTS.md §8 inventory projection checksum, isolated reset of derived
Event Spine platform state, replay of retained event history through the replay-safe
projection handler, rebuild completeness gating, and reproduction metadata for
`CLM-004` / `CLM-006` support artifacts.

**Does not own:** Operator HTTP authorization (Issue #48 /
[PRIVILEGED_OPS_AUTHORIZATION.md](../security/PRIVILEGED_OPS_AUTHORIZATION.md)),
backup/restore (`CAP-015`–`CAP-017`), generalized event-sourcing frameworks,
or exactly-once delivery claims.

## Building blocks

| Symbol | Role |
| --- | --- |
| `platform.HandlerVersion` | `m1-inventory-projection-v1` handler identity for reproduction metadata |
| `platform.ChecksumTenant` | CONTRACTS.md §8 SHA-256 of one tenant's inventory projection |
| `platform.ResetDerivedState` | Deletes only `platform.inbox`, `platform.inventory_projection`, and `platform.processing_failures` |
| `platform.RebuildFromHistory` | Replays retained `HistoryRecord` key/value/position bytes through `ProcessRecord` |
| `platform.ResetDerivedStateForTenant` | Issue #48 tenant-scoped derived-state delete |
| `platform.ReplayTenantHistory` / `RebuildTenantFromHistory` | Issue #48 operator replay/rebuild through `ProcessRecord` |
| `platform.ReproductionMetadata` | Seed, contract version, handler version, commit, broker range, checksum inputs/result, status, limitations |

## Canonical checksum

Normalization remains exactly as defined in
[CONTRACTS.md](../../CONTRACTS.md) §8 and implemented by
`platform.ChecksumTenant`:

1. Lowercase `tenant_id` and `item_id`.
2. Base-10 integers with no leading zeros except `0`.
3. Sort rows by `tenant_id`, then `item_id`.
4. Encode tab-delimited UTF-8 lines ending in `\n`.
5. SHA-256 lowercase hex. Empty projection hashes the empty byte sequence.

Timestamps, offsets, inbox IDs, failure records, and mutable metadata are
excluded.

## Isolated derived-state reset

`ResetDerivedState` is a test/verification procedure that clears **only**
derived Event Spine platform processing state. It must not:

- mutate `erp.inventory_items`, `erp.orders`, or `erp.outbox`;
- delete or rewrite retained Redpanda topic history;
- regenerate events from current ERP state.

Authoritative synthetic ERP source state and broker history remain inputs to
the rebuild campaign.

## Projection rebuild procedure

1. Run a baseline deterministic workload (`northstar.DefaultSeed`) through
   accept → publish → consume (or direct `ProcessRecord` with retained bytes).
2. Record checksum `A` and `ReproductionMetadata` (seed, `event_schema_version`,
   `HandlerVersion`, commit, topic/partition/offset range, checksum inputs).
3. Optionally inject controlled identical duplicates; confirm no additional
   inventory effect and checksum unchanged.
4. Call `ResetDerivedState`.
5. Call `RebuildFromHistory` with the documented retained history range (exact
   retained key/value bytes). Do not use wall clock, uncontrolled randomness,
   mutable external data, or unrecorded intelligence output.
6. On status `complete`, checksum `B` must equal `A` for the declared commit,
   seed, contract version, handler version, and history range.
7. On gaps, unsupported versions, conflicting identities, or other unsafe
   history, status is `incomplete` with explicit reasons. An incomplete rebuild
   must not be treated as a successful `A == B` proof even if a diagnostic
   checksum is present.

## Duplicate-injection harness

The Issue #29 harness republishes exact retained event bytes to
`seshatops.m1.events` after a successful apply. The consumer must record
`duplicate_noop` and leave `quantity_on_hand` / `aggregate_version` / checksum
unchanged. This remains at-least-once + idempotent effects
(`M1-INV-04` / `M1-INV-14`), not exactly-once delivery.

## Safety boundary

`RebuildFromHistory` only calls `ProcessRecord`, which writes platform tables.
It does not accept ERP orders, publish to Redpanda, execute commands, send
notifications, or reach irreversible external adapters. `AppliedNotifier` may
emit an in-process read hint after an apply; that is not an external effect.

## Milestone boundary

| Later owner | Deferred behavior |
| --- | --- |
| Identity (`CAP-013`) | Authorized quarantine release and operator-controlled replay (Issue #48 HTTP; `CLM-010` Observed for authorization subset) |
| Traceability (`CAP-015`–`CAP-017`, ADR-Q-005) | Authorized recovery/restore product and broader checksum reconstruction campaigns |

## Non-claims

This document does not claim production readiness, staging parity, or
exactly-once transport. Issue #30 records test-environment duplicate-safety
and retained-history rebuild evidence and Observed decisions for
`CLM-004`/`CLM-006`; see
[EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](../evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md).
Traceability still owns authorized recovery/restore product scope.
