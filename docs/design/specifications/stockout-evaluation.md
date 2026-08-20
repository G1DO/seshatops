# M4 Stockout Evaluation Protocol

Frozen target, temporal dataset, and scorecard for Northstar Foods
stockout-risk evaluation. Executable helpers: package `forecast`, including
`BaselinePredictions`, `EvaluateBaselines`, `EvaluateCandidateArtifactJSON`,
`SelectRuntimePredictor`, and `BaselineRuntimeResponse`.
Protocol id: `m4-stockout-eval-v1`.

This contract is evaluation-only. It is not production demand-forecasting
evidence. Event Spine wire and persistence remain
[event-spine.md](event-spine.md). M3 traceability lineage is not a
forecasting input.

## 1. Problem

The observation unit is `(tenant_id, item_id, as_of_date)`. `as_of_date` is
a UTC calendar date. There is one end-of-day on-hand quantity per unit. There
is no location dimension.

The prediction horizon is 7 calendar days. Callers cannot override the
horizon, split bounds, metrics, or quality gates.

The positive label is `1` if any end-of-day on-hand in
`(as_of_date, as_of_date + 7 days]` equals `0`. Otherwise the label is `0`.
The observation day itself is not in the label window.

If `as_of_date + 7 days` is past the last history date for that item, the
example is unlabeled and is omitted from train, validation, and test. Labels
are not imputed.

## 2. History fixture

The official history is `forecast.GenerateHistory` with seed
`northstar-m4-stockout-v1`. It is a dense daily on-hand observation log for
fictional Northstar Foods, including restocks as scenario facts. It is not an
Event Spine family and does not add replenishment commands to `erp`.

The log covers 112 consecutive UTC days starting Monday `2026-01-05` and
ending Sunday `2026-04-26`.

| Tenant UUID | Purpose |
| --- | --- |
| `11111111-1111-4111-8111-111111111111` | Official dataset (`TENANT-NS-001`) |
| `22222222-2222-4222-8222-222222222222` | Isolation fixture only (`TENANT-NS-002`) |

Official items on `TENANT-NS-001`: `item-flour-001`, `item-yeast-001`,
`item-salt-001`. `TENANT-NS-002` carries `item-flour-001` with a distinct
series and is omitted from the official dataset.

Each item series is generated without a wall-clock RNG. On each date, a
weekday-specific restock (if any) is applied before demand. End-of-day
on-hand is `max(0, on_hand - demand)`. Weekend demand may differ from
weekday demand. Parameters are chosen so the official `TENANT-NS-001`
train, validation, and test splits each contain at least one positive and
one negative label.

Unsupported or empty seeds fail closed.

## 3. Source and history boundary

`forecast.BuildDataset` requires an explicit tenant. An empty tenant fails
closed. Observations for any other tenant are dropped. Negative on-hand,
duplicate `(tenant_id, item_id, as_of_date)`, non-`YYYY-MM-DD` UTC dates, and
per-item calendar gaps fail closed.

Each labeled example records:

- `source_cutoff` equal to `as_of_date` (inclusive). Features and
  `history_hash` may use observations with `as_of_date` less than or equal to
  that cutoff only.
- `history_hash`, the SHA-256 of that item's allowed observations
  (`as_of_date`, `quantity_on_hand`), sorted by date, encoded as in section 5.
- `row_id`, the SHA-256 of
  `m4-stockout-eval-v1\t{tenant_id}\t{item_id}\t{YYYY-MM-DD}\n`
  as lowercase hex.

Example rows do not store future quantities. Two histories that differ only
after the cutoff share `row_id` and `history_hash`. The label changes only
when a difference falls inside the 7-day label window.

## 4. Chronological splits

Splits are assigned by unique labeled `as_of_date` values, sorted
chronologically, shared across items. Random splitting is not an API.

On the official 112-day log the last 7 calendar dates are unlabeled. The
remaining 105 labeled dates partition as:

| Split | 1-based date index | Inclusive UTC dates |
| --- | --- | --- |
| `train` | 1–70 | `2026-01-05`–`2026-03-15` |
| `validation` | 71–84 | `2026-03-16`–`2026-03-29` |
| `test` | 85–105 | `2026-03-30`–`2026-04-19` |

Dates after index 105 are unlabeled. A shorter hand-built history assigns
the same index rule to its own labeled dates (so a tiny fixture may lie
entirely in `train`). Official `TENANT-NS-001` labeled row count is
3 items × 105 dates.

## 5. Dataset checksum

`forecast.Checksum` hashes one tenant's labeled examples at protocol
`m4-stockout-eval-v1`. The empty dataset hashes the empty byte sequence.

Each example contributes:

`row_id`, `tenant_id`, `item_id`, `as_of_date`, `label` (`0` or `1`),
`split`, `source_cutoff`, `history_hash`.

Canonicalization is:

1. Normalize identifiers (`tenant_id`, `item_id`, `split`) to lowercase
   canonical strings.
2. Normalize the integer label to base-10 with no leading zeros except `0`.
3. Sort rows bytewise by `tenant_id`, then `item_id`, then `as_of_date`.
4. Encode each row as tab-delimited UTF-8 text ending in `\n`.
5. Hash the resulting bytes with SHA-256 and emit lowercase hexadecimal.

Rebuilding from the same declared observations and tenant yields the same
rows, splits, identifiers, and checksum. Input observation order does not
matter.

`history_hash` uses the same integer and tab/`\n` rules over that example's
allowed `(as_of_date, quantity_on_hand)` pairs, sorted by date.

## 6. Predictions and abstention

`forecast.Evaluate` scores one named split. Every labeled row in that split
must appear exactly once, keyed by `row_id`. An omitted row, a duplicate
`row_id`, or an extra `row_id` fails closed. Prediction-set and score
validation run before metric outcomes. An empty split with no predictions is
undefined. Extra predictions on an empty split fail closed. A class-degenerate
split still requires a complete valid prediction list, then reports undefined
metrics.

A nil score is an explicit abstention. A score outside `[0, 1]`, NaN, or
infinity fails closed.

Let `S` be the labeled examples in the split and `P` the subset with a
non-nil score. Coverage is `|P| / |S|`.

## 7. Metrics

Primary metric: binary average precision on `P`. Sort by score descending;
ties break by `row_id` ascending. Average precision is the mean of precision
at each positive rank (sklearn-style uninterpolated AP).

Secondary metric: Brier score `mean((p - y)^2)` on `P`. Lower is better.

A result is undefined (not a win) when any of the following hold:

- `S` is empty
- `S` has no positive label or no negative label
- `P` is empty (all abstain)
- average precision cannot be computed because `P` has no positive label
- Brier cannot be computed because `P` is empty

Undefined metrics are reported as such. They do not pass quality gates.

## 8. Quality gates, baselines, and promotion

A result qualifies when coverage is at least `0.80` and both average
precision and Brier are defined. Coverage uses integer comparison
`|P| * 100 >= |S| * 80`.

Declared deterministic baseline ids and their implemented predictors:

| Id | Prediction at `as_of_date` `T` | Abstain when |
| --- | --- | --- |
| `seasonal_naive` | Copy the realized label at `T - 7 days` (`1.0` or `0.0`) | That lookback date has no realized label for the item |
| `moving_average` | Mean of the last 7 daily indicators `1[on_hand == 0]` through `T` inclusive | Fewer than 7 daily observations exist in `[T - 6 days, T]` |

Both baselines use only observations at or before `T`. The realized label at
`T - 7` uses window `(T - 7, T]`, which does not require data after `T`.

`forecast.EvaluateBaselines` evaluates both predictors on `train`, `validation`,
and `test` independently. It requires a complete feature snapshot whose rows
match the declared dataset exactly. Stale, incomplete, or insufficient feature
snapshots fail closed; missing per-row lookback inputs become explicit nil-score
abstentions and are accounted for by `forecast.Evaluate`.

The deterministic evaluation output records:

- `evaluation_protocol_version`: `m4-stockout-eval-v1`;
- `dataset_version` and the canonical dataset checksum;
- `feature_definition_version`: `m4-raw-onhand-v1`;
- the feature snapshot ID and checksum;
- `code_version`: `m4-deterministic-baselines-v1`;
- canonical predictions and abstention-aware metrics for each baseline and
  frozen split; and
- the qualifying baseline, or an explicit no-qualifying-baseline outcome, for
  each split.

Baseline selection is per split. A learned candidate must use the qualifying
baseline from the same split when applying the existing promotion rule; no
baseline or candidate is selected by tuning on another split.

The bounded learned candidate is an offline Python artifact producer at
`forecast_candidate/stockout_candidate.py`. It consumes serialized
`forecast.CandidateInput` data and has no database credentials, HTTP path, or
workflow authority. It pools the current on-hand quantity into fixed buckets
(`zero`, `low` for 1–2, `medium` for 3–7, and `high` for 8+) and learns a
Laplace-smoothed stockout rate from `train` only. A bucket needs at least five
training rows; otherwise its predictions abstain with
`insufficient_training_support`. Predicted rows include a 95% Wilson interval,
sample count, target, horizon, and source cutoff. No hyperparameter or
threshold tuning is performed, and validation/test labels cannot affect the
emitted predictions.

Each candidate artifact records its artifact, model, code, protocol, dataset,
feature-definition, feature-snapshot, target, horizon, and train/validation
lineage. Go strictly validates the artifact, evaluates all frozen splits, and
applies promotion only to the test comparison. Invalid lineage, stale or
incomplete features, malformed predictions, unsupported scores, and missing
rows fail closed.

Tie-break among qualifying baselines, in order:

1. Higher average precision
2. Lower Brier score
3. Higher coverage
4. Lexicographic id ascending (`moving_average` before `seasonal_naive`)

A learned candidate is promoted only if all of the following hold:

- a qualifying baseline exists
- the candidate qualifies
- candidate average precision is strictly greater than the qualifying baseline
- candidate Brier is less than or equal to the qualifying baseline Brier

Otherwise the qualifying baseline ships and that outcome is reported. Equal
average precision does not promote. A missing qualifying baseline does not
promote a candidate.

Reproduce the deterministic baseline evaluation with:

`go test ./forecast -run TestEvaluateBaselinesIsDeterministicAndSelectsPerSplit -count=1`

Produce a candidate artifact from a documented `CandidateInput` JSON file with:

`python3 forecast_candidate/stockout_candidate.py --input candidate-input.json --output candidate-artifact.json`

Evaluate that artifact with the Go-owned contract using:

`go test ./forecast -run TestEvaluateCandidateArtifact -count=1`

## 9. Runtime integration

`forecast.SelectRuntimePredictor` derives one runtime choice from the frozen
test outcome. A promoted candidate is the only outcome that permits the
learned predictor; every other qualifying outcome selects the reported
deterministic baseline. A missing qualifying baseline fails closed.

`platform.ForecastService` sends a one-row, label-free
`forecast.RuntimeRequest` to a configured short-lived Python command over
stdin/stdout. The command receives tenant/resource identity, the raw feature,
feature snapshot lineage, and selected model/code versions; it receives no
database credentials and has no write or authorization path. Go applies a
deadline, bounds output, rejects unknown or trailing JSON, and validates the
response against the exact request. Timeout, crash, unavailable Python, or a
malformed response returns a typed failure without inserting a prediction row,
so retry does not create an ambiguous current-state effect.

The selected baseline is computed in Go from the same cutoff-safe feature
snapshot. Both candidate and baseline results are persisted by Go in
`platform.forecast_predictions`. The deterministic prediction identity is
tenant, resource, observation date, horizon, dataset/feature versions, and
feature snapshot identity; reusing it with a different result is a conflict.
Persisted rows record predictor/model/code lineage, source freshness,
uncertainty or abstention state, and request correlation.

The explicit `go run ./cmd/seshatops forecast` entrypoint rebuilds the declared
Northstar M4 history, dataset, and feature snapshot, invokes the existing
artifact producer through a bounded typed command boundary, and evaluates the
artifact in Go before calling `platform.ForecastService`. It asserts the
recorded M4 checksums, split metrics, and non-promotion outcome so a changed
protocol result fails visibly until the evidence is intentionally reviewed.

## 10. Non-goals

- Long-running production model serving, persisted feature snapshots, or
  Python database credentials. The separate Go-owned read-only feature
  snapshot and one-shot runtime boundaries are specified in
  [forecast-feature-snapshots.md](forecast-feature-snapshots.md).
- New Event Spine replenishment families
- Making M3 lineage a forecasting dependency
- Production, SLO, or hosted-campaign claims

Golden checksum: `forecast/testdata/dataset.sha256`. Reproduce with
`go test ./forecast`.
