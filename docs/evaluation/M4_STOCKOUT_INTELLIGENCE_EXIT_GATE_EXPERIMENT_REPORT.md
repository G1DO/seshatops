# M4 stockout-intelligence exit-gate experiment

Issue #91. Integrated frozen Northstar Foods stockout evaluation, selected
predictor runtime lineage, forecast read behavior, abstention, and Python
failure handling. This is test-environment evidence only; it is not a
production forecasting, availability, or business-impact claim.

## Scope and inputs

The gate used protocol `m4-stockout-eval-v1`, history seed
`northstar-m4-stockout-v1`, and tenant `TENANT-NS-001`. Go rebuilt the
chronological dataset and raw feature snapshot before invoking the offline
Python candidate. The rebuilt dataset has 315 labeled examples and the
feature snapshot has 315 label-free rows.

The observed lineage was:

| Field | Value |
| --- | --- |
| Dataset checksum | `b29e795cbacd0a40ee2b7c15c0d52200a867fa24554f50b7f77989302c8e116a` |
| Feature snapshot ID | `2d4121bb6804b9dbe9ee0d3d68989e9271e348a3c3ddf0727df5d53428035fc6` |
| Feature snapshot checksum | `808980035fd123badb12df34224a8b510683d93dffbc1d8fde3df28590be4d78` |
| Feature definition | `m4-raw-onhand-v1` |
| Source boundary | `m4-official-history-v1`, `m4-official-projection-v1`, `2026-04-26T23:59:59Z`, 448 observations |

## Environment and commands

Local operator environment on 2026-08-18:

- Windows `amd64`; Go `1.25.0`; Python `3.13.7`; Node `24.14.0`; npm
  `11.9.0`.
- Docker was unavailable. PostgreSQL Testcontainers tests therefore reported
  an explicit skip; no database-backed result below is claimed as locally
  observed.
- The final implementation commit under test was `8b53501`. An independent
  review identified and the follow-up commits resolved missing end-to-end read
  coverage, non-mandatory Python setup, and unasserted evidence values.

Commands run:

```text
go test ./forecast -run TestM4ExitGateRebuildsAndEvaluatesTheFrozenCandidate -count=1 -v
go test ./api -run TestM4ExitGatePersistsSelectedForecastForAuthorizedRead -count=1 -v
go test ./forecast -count=1 -timeout 15m
go test ./... -count=1 -timeout 15m
python -m unittest discover -s forecast_candidate -p 'test_*.py'
go test ./platform -run TestCommandCandidateInvokerUsesTypedPythonBoundary -count=1 -timeout 15m
cd web && npm ci && npm run typecheck && npm test && npm run build
```

The forecast gate test created a temporary `CandidateInput` JSON document,
ran `forecast_candidate/stockout_candidate.py`, evaluated its artifact with
the Go-owned evaluator, asserted the frozen checksums and metrics, and passed
the selected outcome through `forecast.SelectRuntimePredictor`. The API gate
test carries that actual artifact and evaluation through
`platform.ForecastService`, Go-owned persistence, the authorized forecast GET,
and a cross-tenant denial check.

## Results

The learned candidate was not promoted. The frozen test split selected the
qualifying deterministic `seasonal_naive` baseline because candidate average
precision was lower; full candidate coverage did not override that gate.

| Split | Selected baseline | Candidate AP | Candidate Brier | Candidate coverage | Baseline AP | Baseline Brier | Candidate promoted |
| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |
| Train | `moving_average` | 0.957489696104 | 0.036397378241 | 1.000000 | 0.962987425654 | 0.170493197279 | No |
| Validation | `seasonal_naive` | 0.963827614379 | 0.126833502348 | 1.000000 | 1.000000000000 | 0.000000000000 | No |
| Test | `seasonal_naive` | 0.953728663154 | 0.126833502348 | 1.000000 | 1.000000000000 | 0.000000000000 | No |

The runtime selection was `predictor=baseline`,
`baseline_id=seasonal_naive`, `model_version=seasonal_naive`, and
`code_version=m4-deterministic-baselines-v1`. The candidate was not selected
by caller input or by post-hoc reinterpretation.

The pure frozen evaluator and the Python candidate suite passed. The full Go
command passed, but the API gate and other PostgreSQL-backed service tests
explicitly skipped because Docker was unavailable. The web suite also passed:
10 test files, 52 tests, typecheck, and Vite build. The typed command boundary
test passed malformed-response, timeout, and unavailable-command cases.

Hosted CI for PR #98 — implementation commit `bad5073` recorded
[Go CI success](https://github.com/G1DO/seshatops/actions/runs/32181303886)
(including the Docker-backed API gate), [Web CI success](https://github.com/G1DO/seshatops/actions/runs/32181304013),
and success for all Documentation CI jobs, including
[link check](https://github.com/G1DO/seshatops/actions/runs/32181303898/job/95854713070).
The merged PR head was the later docs-only commit `39911bc`; it carries the same
implementation as `bad5073` with no code change. An earlier Documentation run on
implementation commit `8b53501` had
[failed](https://github.com/G1DO/seshatops/actions/runs/32180186848/job/95851340432)
only on the pre-existing Redpanda URL in `docs/design/specifications/event-spine.md`
returning HTTP 500; the docs-only follow-ups (`bad5073` → `39911bc`) passed without
changing that unrelated link. This preserves the distinction between implementation
evidence and documentation-only follow-up.

## Limitations and failed cases

- This local run does not establish the PostgreSQL-backed authorized forecast
  read, persistence, tenant-negative authorization, or core-flow continuity
  cases because Docker was unavailable. Those cases remain required for the
  hosted CI/Testcontainers run.
- The Python process is a one-shot offline candidate adapter, not a deployed
  service. No production SLO, availability, forecast quality, or business
  impact is claimed.
- The test uses a synthetic fixture and process-local test collaborators; it
  is not a backup/restore, cloud deployment, pentest, or capacity campaign.
- No candidate promotion failure was treated as a test failure: baseline
  selection is the declared successful outcome for this frozen dataset.
