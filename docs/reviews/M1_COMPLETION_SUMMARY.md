# M1 Completion Summary

## Outcome

Milestone M1 delivered the SeshatOps event spine for the fictional Northstar
Foods scenario: reviewed contract, deterministic fixture, synthetic ERP source
transaction with transactional outbox, Redpanda relay, inventory projection
consumer with failure/restart safety, read-only REST/SSE API, TypeScript
operations view, and duplicate/rebuild checksum proofs. Issue #30 executed the
integrated exit-gate campaign in a declared **test environment** and recorded
evidence for `CLM-003`–`CLM-006`.

This summary does not claim production readiness, staging parity, exactly-once
delivery, M2 identity/authorization/operations product scope, M3
backup/restore, SLO compliance, or hyperscale capacity.

## M1 delivery history

| Area | Delivered through |
| --- | --- |
| Event-spine contract | Issue #21 |
| JSON event library + Northstar fixture | Issue #22 |
| Synthetic ERP source + pending outbox | Issue #23 |
| Source-owned Redpanda outbox relay | Issue #24 |
| Inbox + inventory projection consumer | Issue #25 |
| Poison escalation + InspectProcessing | Issue #26 |
| Read-only projection REST/SSE API | Issue #27 |
| TypeScript operations view | Issue #28 |
| Duplicate injection + deterministic rebuild | Issue #29 |
| Integrated exit-gate campaign + claim review | Issue #30 / this summary |

## Final artifacts

- [M1 exit-gate procedure](../evaluation/M1_EXIT_GATE_PROCEDURE.md)
- [M1 exit-gate experiment report](../evaluation/M1_EXIT_GATE_EXPERIMENT_REPORT.md)
- [M1 exit-gate campaign review](M1_EXIT_GATE_CAMPAIGN_REVIEW.md)
- [Fault campaign matrix](../evaluation/FAULT_CAMPAIGN_MATRIX.md) (M1 rows updated)
- [Evidence ledger](../../EVIDENCE.md) (`CLM-003`–`CLM-006` Observed)
- Prior M1 package reviews under `docs/reviews/M1_*.md`

## Evidence and verification

- Local campaign: `go test ./... -count=1 -timeout 25m` passed on 2026-08-12
  against runtime commit `a4e5d47`.
- Web: `npm test` (15), `npm run typecheck`, and `npm run build` passed.
- `CLM-003`–`CLM-006` promoted to **Observed** for the test-environment scope
  only; other claims remain Planned.
- Hosted GitHub Actions for PR #41 head `b6bec19`: Go CI
  [31588107839](https://github.com/G1DO/seshatops/actions/runs/31588107839),
  Web CI [31588107888](https://github.com/G1DO/seshatops/actions/runs/31588107888),
  Documentation CI
  [31588107709](https://github.com/G1DO/seshatops/actions/runs/31588107709).

## Deviations and dispositions

1. No docker-compose or long-running demo binary was added; acceptance uses the
   documented package Testcontainers/Vitest suite (Issue #30 non-goal: no new
   architecture merely to pass).
2. Live browser walkthrough remains optional supporting evidence.
3. Non-M1 fault rows (FC-002–006, FC-008, FC-015+) remain Not executed.

## Residual risk

See [M1_EXIT_GATE_CAMPAIGN_REVIEW.md](M1_EXIT_GATE_CAMPAIGN_REVIEW.md). M2
activates identity and operations; M3 owns restore product scope.
