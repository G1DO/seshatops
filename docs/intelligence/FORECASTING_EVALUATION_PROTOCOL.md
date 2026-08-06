# Forecasting Evaluation Protocol

**Status:** Planned. This document defines a future evaluation protocol for forecasting capabilities. It is not a model, dataset, production series, measured result, promotion decision, or reliability claim.

**Owns:** Forecast-evaluation units, temporal separation, leakage review, baseline comparison, metric categories, uncertainty and abstention evaluation, lineage, reproducibility, claim discipline, and evaluation invalidation.

**Does not own:** Forecasting implementation, model or vendor selection, feature-store design, executable schemas, deployment, production thresholds, or the security/reliability evidence protocol owned by Issue #7.

## 1. Purpose and boundaries

The protocol defines how a future SeshatOps forecasting capability may be evaluated without allowing future information into historical inputs and without presenting unmeasured capability as established. It applies to point forecasts, interval forecasts, ranked risk outputs, and explicit unavailable or abstained results.

The protocol must preserve the product and architecture boundaries:

- Python may produce advisory forecasts and uncertainty information.
- Go remains responsible for validation, authorization, approval, workflow state, and command execution.
- A forecast or proposal cannot authorize, approve, or execute a business change.
- Evaluation evidence must remain tenant-safe, clean-room compliant, and reproducible from versioned artifacts.

This document defines categories and required records. It intentionally does not select a production model, forecasting library, data source, vendor, numerical target, or promotion threshold.

## 2. Conceptual evaluation unit

An evaluation unit is a future record of one forecasting task evaluated under a declared run. The record should identify, conceptually:

| Element | Meaning | Initial status |
| --- | --- | --- |
| Forecasting task | The question being evaluated and the intended decision context | Planned |
| Target variable | The value or event to forecast | Planned |
| Entity or series | The tenant-safe series or cohort being evaluated | Planned |
| Forecast origin | The historical point at which the forecast would have been made | Planned |
| Forecast horizon | The future interval or steps being predicted | Planned |
| Time granularity | The declared temporal resolution | Planned |
| Dataset snapshot | The immutable version of inputs available for the run | Planned |
| Segment or cohort | A declared slice used to identify heterogeneous behavior | Planned |
| Model or baseline | The versioned candidate or comparison condition | Planned |
| Evaluation run | The reproducible execution joining inputs, configuration, evaluator, and outputs | Planned |

No production series, private identifiers, actual observations, or fabricated values belong in this protocol.

## 3. Temporal separation and evaluation lifecycle

Every future run must declare how chronological evidence is separated:

1. Training evidence is earlier than validation evidence, and validation evidence is earlier than final test evidence where all three are applicable.
2. Every test observation is strictly later than the training observations used for that forecast.
3. Rolling-origin or expanding-window evaluation is used when the task requires repeated historical forecast origins; the choice and rationale are recorded.
4. Feature availability is reconstructed as of each forecast origin. A feature is not eligible merely because it exists in a later revised snapshot.
5. Model or hyperparameter selection uses development and validation evidence. The final holdout is protected from repeated optimization.
6. Preprocessing, imputation, scaling, feature selection, and learned transformations are fitted only on the evidence permitted for the relevant forecast origin.
7. Any use of revised, backfilled, delayed, or externally sourced data records whether historical availability can be reconstructed.
8. Irregular, sparse, newly introduced, discontinued, and insufficient-history series receive an explicit handling decision rather than being silently excluded.

The evaluation record must distinguish development evidence, validation evidence, and final evaluation evidence. An anecdotal example may illustrate behavior but cannot establish a capability claim or replace the declared evaluation.

## 4. Leakage review

A leakage review is required before a result can be accepted into an evaluation report. The review must consider at least:

| Leakage class | Required question |
| --- | --- |
| Target leakage | Does an input encode the target or a post-outcome consequence of the target? |
| Temporal leakage | Was information from after the forecast origin available to the historical input? |
| Future-derived feature | Was an aggregate, window, label, or transformation calculated using future observations? |
| Revised or backfilled data | Does the snapshot contain a later correction that was not historically available? |
| Cross-series leakage | Can another series reveal future information about the evaluated series without an allowed historical relationship? |
| Tenant leakage | Could an input, feature, cache, artifact, or slice contain another tenant's information? |
| Duplicate records | Do equivalent records occur across training, validation, or final evaluation splits? |
| Preprocessing leakage | Was a learned transformation fitted using evidence outside the permitted window? |
| Post-origin human label | Was a human label or adjudication created after the forecast origin then used as an input? |
| External availability | Can the historical availability and version of external data be reconstructed? |

Each review records the examined evidence, the disposition, the reviewer, and any unresolved limitation. If leakage cannot be ruled out, the affected result remains `Not evaluated` or `TBD — evidence required`.

## 5. Baseline comparison

The evaluation must compare a candidate against documented baselines appropriate to the task. Possible categories include:

- Last observed value.
- Seasonal naive behavior where seasonality is meaningful and supported by the task.
- Historical mean or median where that summary is appropriate.
- Simple trend where a trend assumption is justified.
- An existing operational heuristic only when it is independently documented, safe to disclose, and comparable under the same temporal protocol.

No baseline is mandatory for every series. Applicability, implementation version, input availability, and known limitations must be recorded. A baseline comparison does not select a production model or vendor.

## 6. Metrics and slices

The evaluator may report metric categories that fit the target and decision context:

| Category | Examples of what it describes | Required limitation note |
| --- | --- | --- |
| Absolute error | Magnitude of point-forecast error | Consider scale and outliers. |
| Squared error | Greater weighting of large errors | Explain sensitivity to skew and extreme values. |
| Percentage or scaled error | Relative or scale-normalized error where mathematically appropriate | Address zero, near-zero, intermittent, and low-volume targets. |
| Directional or ranking quality | Whether direction or ordering is useful for the task | Do not treat ranking quality as proof of calibrated magnitude. |
| Bias | Systematic over- or under-forecasting | Report by relevant horizon and segment. |
| Horizon behavior | Error and uncertainty as the horizon changes | Do not collapse all horizons into one unsupported conclusion. |
| Segment behavior | Error, calibration, and availability by cohort | Record unsupported or underrepresented segments. |
| Prediction intervals | Coverage of stated intervals | Evaluate calibration separately from point accuracy. |
| Interval width or sharpness | Informativeness of uncertainty ranges | Narrow intervals are not useful if coverage is unreliable. |
| Calibration | Agreement between stated uncertainty and observed outcomes | State the calibration method and limitations. |
| Abstention or unavailable rate | Frequency and distribution of withheld results | Assess both safety and operational effect. |
| Operationally material errors | Error categories tied to a declared decision risk | Do not invent materiality thresholds in M0. |

Reports must include distributions and declared slices, not only one aggregate average. Metric applicability and interpretation remain task-specific. Every target, result, benchmark value, and pass threshold is `Planned`, `Not evaluated`, or `TBD — evidence required` until supported by repository evidence.

## 7. Uncertainty, freshness, and abstention

Every future non-abstained forecasting output must represent uncertainty explicitly. If a capability cannot provide a supported uncertainty representation, it must return an unavailable, unsupported, or abstained result; lack of uncertainty support is not an automatic pass condition. The evaluation must distinguish:

- Point accuracy from uncertainty calibration.
- Low confidence from high confidence.
- Insufficient history from ordinary error.
- Out-of-distribution or unsupported conditions from evaluated conditions.
- Stale inputs or stale forecasts from current evidence.
- An unavailable or abstained result from a forecasted value.

For any capability that returns an uncertainty representation, calibration must be evaluated separately from point accuracy. Until calibration evidence exists, calibration and related capability claims remain `Not evaluated`.

The evaluator must allow abstention for low-confidence, out-of-distribution, insufficient-history, stale-data, or data-quality conditions. Unsupported precision is not a quality signal. Forecasts must never be presented as guaranteed outcomes, and downstream proposals must retain uncertainty and source lineage.

## 8. Lineage and reproducibility

An evaluation record must identify, where applicable:

- Evaluation-run ID.
- Capability and version.
- Dataset and snapshot version.
- Chronological split definition.
- Feature, preprocessing, and transformation versions.
- Model or baseline identifier and version.
- Evaluator version and code commit where applicable.
- Configuration and declared environment.
- Deterministic seed where relevant.
- Evaluation timestamp and time basis.
- Metrics and slices produced.
- Input, output, and report artifacts.
- Leakage-review record.
- Known limitations and exclusions.
- Reviewer and disposition.

Reproduction requires the same versioned inputs, evaluator, configuration, permitted reference data, and declared environment. If any required lineage is lost or cannot be reconstructed, the affected result or claim is not established.

## 9. Claim discipline, withdrawal, and rollback

Promotion or public description must not rely only on anecdotal examples. A future claim must identify the evidence supporting it and must not exceed the evaluated task, segments, horizons, data snapshot, and uncertainty behavior.

An evaluation result or forecasting claim must be withdrawn, marked unavailable, or returned to a prior supported state when evidence is invalidated by:

- Discovered temporal, target, tenant, cross-series, or preprocessing leakage.
- Lost or non-reproducible dataset or artifact lineage.
- Reproduction failure.
- Material regression against a documented comparison condition.
- Calibration or abstention failure for the declared use.
- Unsupported segment or horizon behavior.
- Security or tenant-isolation failure.
- Corrupted, altered, or incomplete evaluation artifacts.

Rollback here means withdrawing or reverting a capability, configuration, or claim. It does not define deployment mechanisms, traffic shifting, or infrastructure operations.

## 10. Required invariants

1. Forecast evaluation preserves historical temporal ordering.
2. Future-derived information cannot enter historical forecast features.
3. Final test evidence is not repeatedly optimized against.
4. Every reported result identifies its dataset, evaluator, configuration, and version lineage.
5. Missing or unreliable evidence may require abstention or an unavailable result.
6. Intelligence output cannot authorize, approve, or execute a business change.
7. Evaluation evidence remains tenant-safe and clean-room compliant.
8. No threshold or result is presented as established without repository evidence.
9. Evaluation or claim rollback is required when supporting evidence becomes invalid.

## 11. Deferred decisions

The following remain open: exact dataset and series definitions, date ranges, feature contracts, model families, forecasting libraries, evaluation tooling, numerical targets, promotion thresholds, production integration, and the broader security, reliability, recovery, and performance evidence protocol owned by Issue #7.

## 12. Related documents

- [Product constitution](../../PRODUCT.md)
- [Logical architecture](../../ARCHITECTURE.md)
- [Event model](../architecture/EVENT_MODEL.md)
- [Command model](../architecture/COMMAND_MODEL.md)
- [Threat model](../security/THREAT_MODEL.md)
- [Authorization model](../security/AUTHORIZATION_MODEL.md)
- [Governed-RAG evaluation protocol](GOVERNED_RAG_EVALUATION_PROTOCOL.md)
- [Forecasting evaluation schema](../evaluation/schemas/FORECASTING_EVALUATION_SCHEMA.md)
- [Forecasting evaluation report template](../evaluation/templates/FORECASTING_EVALUATION_REPORT.md)
