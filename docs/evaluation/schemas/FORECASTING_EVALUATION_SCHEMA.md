# Forecasting Evaluation Schema

**Status:** Planned. This is a conceptual Markdown field catalog for future evaluation records. It is not JSON Schema, Protobuf, a database schema, an API contract, or a source-code type.

**Evidence rule:** Every result, target, benchmark value, threshold, and pass status remains `Planned`, `Not evaluated`, or `TBD — evidence required` until supported by repository evidence. Empty fields are intentional.

## Record catalog

| Field | Meaning | Initial value/status |
| --- | --- | --- |
| Evaluation run ID | Stable identity for one reproducible run | Planned |
| Case-set or task identity | Identity and version of the evaluated task definition | Planned |
| Capability name and version | Forecasting capability under evaluation | Planned |
| Dataset identity and snapshot | Versioned data lineage available to the run | Planned |
| Dataset provenance | Origin, license, generation method, and clean-room independence | Planned |
| Series or cohort definition | Tenant-safe entity, series, or segment definition | Planned |
| Split definition | Chronological training, validation, and final-evaluation rules | Planned |
| Forecast-origin policy | How historical availability is reconstructed | Planned |
| Horizon and granularity | Declared forecast horizon and time resolution | Planned |
| Feature/preprocessing version | Version of transformations and feature availability logic | Planned |
| Model or baseline identifier/version | Candidate or comparison condition, if applicable | Planned |
| Evaluator version | Version of evaluation logic and adjudication rules | Planned |
| Code commit | Repository commit used where applicable | Planned |
| Configuration | Non-secret configuration affecting the run | Planned |
| Environment | Declared execution environment and dependencies | Planned |
| Deterministic seed | Seed where relevant; otherwise reason not applicable | Planned |
| Run timestamp | Time basis for the evaluation record | Planned |
| Leakage review | Review identity, findings, and disposition | Planned |
| Metrics | Metric categories and produced values | Not evaluated |
| Metric limitations | Known mathematical or interpretive limitations | Planned |
| Slices/categories | Horizons, segments, cohorts, and support status | Planned |
| Uncertainty/calibration checks | Interval, calibration, and uncertainty evidence | Not evaluated |
| Abstention/unavailable checks | Conditions and observed abstention behavior | Not evaluated |
| Failures | Failed, incomplete, invalid, or inconclusive cases | Planned |
| Artifacts | Versioned inputs, outputs, reports, and manifests | Planned |
| Reviewer | Reviewer identity or role | Planned |
| Claim status | Status of any proposed capability claim | Not evaluated |
| Disposition | Review disposition and follow-up state | Planned |
| Limitations and exclusions | Known boundaries, unsupported segments, and exclusions | Planned |
| Reproduction information | Instructions and dependencies required to reproduce | Planned |
| Rollback/invalidation record | Withdrawal or invalidation information if applicable | Planned |

## Record rules

- Values must be traceable to versioned evidence or remain empty.
- A missing required lineage field prevents the affected result from being established.
- Development, validation, and final-evaluation evidence must remain distinguishable.
- Every non-abstained point forecast must carry an uncertainty representation or an unavailable/unsupported/abstained disposition; uncertainty support cannot be silently omitted.
- Calibration must be recorded separately from point accuracy and remains `Not evaluated` until supported by evidence.
- A reviewer disposition does not establish forecasting quality, security, reliability, or production readiness.
- No field authorizes, approves, or executes a business action.

## Related documents

- [Forecasting evaluation protocol](../../intelligence/FORECASTING_EVALUATION_PROTOCOL.md)
- [Forecasting evaluation report template](../templates/FORECASTING_EVALUATION_REPORT.md)
- [Governed-RAG evaluation schema](GOVERNED_RAG_EVALUATION_SCHEMA.md)
