# Evidence

What the recorded test runs measured. Not production evidence.

## Event Spine

[Report](evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md).
Runtime commit `a4e5d47`; hosted CI on PR #41 `b59b760`.

On a single-host Testcontainers topology: accepted orders survive broker
outage; duplicate delivery does not duplicate inventory effects; schema v1
gap/stale/reorder/poison paths quarantine instead of applying; unchanged
history rebuilds to the same checksum. Not exactly-once delivery, not
backup/restore.

## Identity HTTP

[Report](evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md).
Runtime commit `173cedb`; hosted CI on PR #61 `1f19c58`. Operator = reviewer.

On implemented HTTP: tenant isolation, default-deny privileged routes,
authorized same-tenant ops visibility, and fail-closed quarantine/replay/rebuild
with audit-insert-before-mutate. Library/test OIDC; process-local sessions and
assignments. Not production revocation, pentest, or restore.

## M3 traceability and recovery

[Report](evaluation/M3_TRACEABILITY_RECOVERY_EXIT_GATE_EXPERIMENT_REPORT.md).
Runtime commit `a43e8a5`; hosted CI on PR #84 `a43e8a5`. Operator = reviewer.

On a single-host Testcontainers topology: an authorized same-tenant batch trace
returns upstream supplier/lot and downstream shipment/order hops with
provenance; duplicate delivery does not duplicate lineage effects; an
incompatible traceability event is quarantined while unrelated valid work
applies; unchanged history rebuilds to the same inventory and lineage
checksums. Tenant-negative trace HTTP is fail-closed. Not production, not
backup/restore.

## M4 forecast feature snapshots

[Specification](design/specifications/forecast-feature-snapshots.md).

The pure `forecast` builder has focused coverage for order-independent
deterministic rows, snapshot identity/checksum rebuilds, source-boundary
identity, cutoff-safe earlier rows, invalid quantities, label-free
serialization, and insufficient short history. The Go API and platform tests
also cover the intended tenant, method, authorization, replay, and explicit
non-complete response paths; database-backed integration cases require the
repository's Docker/Testcontainers environment.

The feature route is read-only by construction: one PostgreSQL
`READ ONLY`/`REPEATABLE READ` transaction reads retained outbox bytes, inbox
dispositions, and the inventory projection, then replays in memory. It has no
feature table, snapshot write, Python credential, label, raw event-byte, or
model-output path. The current sparse Event Spine history does not reproduce
the dense M4 restock fixture, so a live request may correctly return
`insufficient` or `incomplete`; this is an explicit limitation rather than a
forecast-quality result. These are repository/test claims, not production
isolation or forecasting-quality claims.

## M4 learned stockout candidate

The offline candidate producer has focused tests for deterministic train-only
fitting, typed risk and Wilson uncertainty output, insufficient-support
abstention, stale/malformed input rejection, and artifact-only CLI behavior.
The Go evaluator tests strict artifact lineage, prediction coverage,
abstention-aware fallback to the deterministic baseline, and JSON contract
rejection. These are implementation-contract claims only; no learned
candidate is recorded as promoted or as production forecasting evidence.

## M4 integrated stockout-intelligence exit gate

[Report](evaluation/M4_STOCKOUT_INTELLIGENCE_EXIT_GATE_EXPERIMENT_REPORT.md).

The integrated frozen test rebuilt the declared Northstar dataset and feature
snapshot, ran the offline Python candidate, evaluated all chronological splits
in Go, and selected `seasonal_naive` as the deterministic runtime predictor.
The candidate was not promoted because its test average precision was lower
than the qualifying baseline. The pure Go, Python, typed-boundary, and web
checks recorded in the report passed locally. PostgreSQL-backed persistence,
authorized read, and tenant-negative HTTP cases were explicitly skipped in
the local Windows environment because Docker was unavailable; no local exit
gate pass is claimed for those cases. This remains test-environment evidence,
not production quality, availability, or business-impact evidence.

Hosted PR #98 implementation evidence: [Go CI passed](https://github.com/G1DO/seshatops/actions/runs/32181303886)
on implementation commit `bad5073` (including the Docker-backed API gate) and
[Web CI passed](https://github.com/G1DO/seshatops/actions/runs/32181304013);
all Documentation CI jobs passed on that implementation commit, including
[link check](https://github.com/G1DO/seshatops/actions/runs/32181303898/job/95854713070).
The merged PR head was the later docs-only commit `39911bc` (no code change
beyond documentation). An earlier Documentation run on implementation commit
`8b53501` had [failed](https://github.com/G1DO/seshatops/actions/runs/32180186848/job/95851340432)
only on the pre-existing Redpanda URL in `docs/design/specifications/event-spine.md`
returning HTTP 500; the docs-only follow-ups (`bad5073` → `39911bc`) passed
without changing that unrelated link. This preserves the distinction between
implementation evidence and documentation-only follow-up.

## M5 one-shot forecast runner

The explicit `cmd/seshatops forecast` command now rebuilds the declared M4
Northstar fixture, runs the offline Python artifact producer through a bounded
typed process boundary, evaluates the artifact and frozen checksums/metrics in
Go, selects `seasonal_naive`, and persists through
`platform.ForecastService`. Its result is bounded to protocol/lineage,
split-metric, selection, prediction identity/status, and limitation fields; it
does not emit feature rows or model internals. Focused tests cover successful
selection, immutable rerun identity, Python unavailable/timeout/malformed
responses, no-persist failure ordering, tenant-scoped persistence, and live
freshness mismatch behavior. The frozen source remains separate from the live
Event Spine source, so a successful command is not a claim that the persisted
result is currently fresh.

## M5 packaged release demonstrations

The guarded `./scripts/local-stack.sh demo` harness can run eight named
scenarios or the complete campaign against the disposable packaged Compose
environment. It drives the real Go process, OIDC/public HTTP routes,
PostgreSQL, and at-least-once Redpanda boundary; local-only fixture, broker, and
Python-process faults are not public API controls. See
[Getting started](getting-started.md#release-demonstration-campaign) for the
commands, scenario scope, evidence schema, cleanup, and deterministic
comparison procedure.

A passing scenario file is evidence about that single disposable, single-host
local run at its recorded commit and fixture version. The machine result and
human summary record the source fingerprint, observed statuses, counts,
diagnostic durations, checksums, telemetry, deterministic identity, cleanup,
and limitations. Timing is retained for diagnosis but excluded from repeat-run
identity comparison.
The harness also makes failures and failed cleanup non-zero rather than
converting them into successful evidence.

These artifacts do not establish production availability, an SLO or recovery
objective, capacity or sustained-load behavior, multi-host failover, disaster
recovery, backup/restore correctness, forecasting quality, or business impact.
They do not change the honest at-least-once transport claim. Repository-local
evidence belongs under the ignored `.release-evidence/` directory; explicitly
selected paths outside the checkout are also supported. Evidence must not
include secrets, private data, or unbounded logs. No specific campaign result
is recorded in this document.

## M5 runbook exercise

[Runbook exercise report](evaluation/RUNBOOK_EXERCISE_REPORT.md).

Each recovery runbook ([broker interruption and outbox backlog recovery](operations/runbooks/broker-interruption.md), [poison isolation](operations/runbooks/poison-isolation.md), [tenant rebuild](operations/runbooks/rebuild-checksum.md), [forecast degradation](operations/runbooks/forecast-degradation.md)) was followed from a clean disposable Compose environment without repository-author knowledge. The report records environment identity, exact commands, observed `/readyz`/`/metrics`/log/API signals, checksums, duration, result, and deviations. It preserves the tenant-scoped, audit-before-mutate, and bounded-evidence invariants and does not establish production availability, SLO, or capacity claims.
