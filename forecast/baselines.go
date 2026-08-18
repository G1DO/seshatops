package forecast

import (
	"sort"
	"strings"
)

const (
	// EvaluationCodeVersion identifies the implementation of the deterministic
	// baseline evaluation output. Bump it when the evaluator behavior changes.
	EvaluationCodeVersion = "m4-deterministic-baselines-v1"

	movingAverageWindowDays = HorizonDays
)

var declaredBaselines = []string{
	BaselineMovingAverage,
	BaselineSeasonalNaive,
}

var evaluationSplits = []string{
	SplitTrain,
	SplitValidation,
	SplitTest,
}

// BaselineEvaluation is the deterministic evaluation of all declared
// baselines over all frozen chronological splits.
type BaselineEvaluation struct {
	EvaluationProtocolVersion string              `json:"evaluation_protocol_version"`
	DatasetVersion            string              `json:"dataset_version"`
	DatasetChecksum           string              `json:"dataset_checksum"`
	FeatureDefinitionVersion  string              `json:"feature_definition_version"`
	FeatureSnapshotID         string              `json:"feature_snapshot_id"`
	FeatureSnapshotChecksum   string              `json:"feature_snapshot_checksum"`
	CodeVersion               string              `json:"code_version"`
	Baselines                 []BaselineRun       `json:"baselines"`
	Selections                []BaselineSelection `json:"selections"`
}

// BaselineRun contains canonical predictions and metrics for one baseline.
type BaselineRun struct {
	ID          string       `json:"id"`
	Predictions []Prediction `json:"predictions"`
	Results     []Result     `json:"results"`
}

// BaselineSelection identifies the qualifying baseline for one split. Found
// is false when neither declared baseline passes the frozen quality gates.
type BaselineSelection struct {
	Split      string `json:"split"`
	BaselineID string `json:"baseline_id,omitempty"`
	Found      bool   `json:"found"`
	Reason     string `json:"reason"`
}

type baselineKey struct {
	tenant string
	item   string
	date   string
}

type validatedBaselineInput struct {
	examples    []Example
	labels      map[baselineKey]Example
	featureRows map[baselineKey]FeatureRow
}

// BaselinePredictions returns one complete, canonical prediction set for a
// declared baseline. Missing baseline inputs are represented as nil scores.
func BaselinePredictions(ds Dataset, features FeatureSnapshot, baselineID string) ([]Prediction, error) {
	input, err := validateBaselineInput(ds, features, baselineID)
	if err != nil {
		return nil, err
	}
	return baselinePredictions(input, baselineID)
}

// EvaluateBaselines evaluates every declared baseline on train, validation,
// and test, then selects the qualifying baseline independently per split.
func EvaluateBaselines(ds Dataset, features FeatureSnapshot) (BaselineEvaluation, error) {
	input, err := validateBaselineInput(ds, features, "")
	if err != nil {
		return BaselineEvaluation{}, err
	}

	evaluation := BaselineEvaluation{
		EvaluationProtocolVersion: ProtocolID,
		DatasetVersion:            ds.ProtocolID,
		DatasetChecksum:           Checksum(ds),
		FeatureDefinitionVersion:  features.FeatureDefinitionVersion,
		FeatureSnapshotID:         features.SnapshotID,
		FeatureSnapshotChecksum:   features.Checksum,
		CodeVersion:               EvaluationCodeVersion,
		Baselines:                 make([]BaselineRun, 0, len(declaredBaselines)),
		Selections:                make([]BaselineSelection, 0, len(evaluationSplits)),
	}

	resultsBySplit := make(map[string]map[string]Result, len(evaluationSplits))
	canonicalDataset := ds
	canonicalDataset.Examples = input.examples
	for _, split := range evaluationSplits {
		resultsBySplit[split] = make(map[string]Result, len(declaredBaselines))
	}
	for _, baselineID := range declaredBaselines {
		predictions, err := baselinePredictions(input, baselineID)
		if err != nil {
			return BaselineEvaluation{}, err
		}
		run := BaselineRun{
			ID:          baselineID,
			Predictions: predictions,
			Results:     make([]Result, 0, len(evaluationSplits)),
		}
		for _, split := range evaluationSplits {
			result, err := Evaluate(canonicalDataset, split, predictionsForSplit(canonicalDataset, split, predictions))
			if err != nil {
				return BaselineEvaluation{}, err
			}
			run.Results = append(run.Results, result)
			resultsBySplit[split][baselineID] = result
		}
		evaluation.Baselines = append(evaluation.Baselines, run)
	}

	for _, split := range evaluationSplits {
		baselineID, found := QualifyingBaseline(resultsBySplit[split])
		selection := BaselineSelection{
			Split:  split,
			Found:  found,
			Reason: "no qualifying baseline",
		}
		if found {
			selection.BaselineID = baselineID
			selection.Reason = "qualifying baseline"
		}
		evaluation.Selections = append(evaluation.Selections, selection)
	}

	return evaluation, nil
}

func validateBaselineInput(ds Dataset, features FeatureSnapshot, baselineID string) (validatedBaselineInput, error) {
	if baselineID != "" && !isDeclaredBaseline(baselineID) {
		return validatedBaselineInput{}, wrapInvalid("unsupported baseline %s", baselineID)
	}
	if ds.ProtocolID != ProtocolID {
		return validatedBaselineInput{}, wrapInvalid("dataset protocol %q", ds.ProtocolID)
	}
	tenantID := strings.ToLower(strings.TrimSpace(ds.TenantID))
	if tenantID == "" {
		return validatedBaselineInput{}, wrapInvalid("empty dataset tenant")
	}
	if features.Status != SnapshotStatusComplete {
		reason := strings.Join(features.StatusReasons, "; ")
		if reason == "" {
			reason = "no usable feature rows"
		}
		return validatedBaselineInput{}, wrapInvalid("feature snapshot status %q: %s", features.Status, reason)
	}
	if features.ContractVersion != SnapshotContractVersion {
		return validatedBaselineInput{}, wrapInvalid("feature contract version %d", features.ContractVersion)
	}
	if strings.ToLower(strings.TrimSpace(features.TenantID)) != tenantID {
		return validatedBaselineInput{}, wrapInvalid("feature tenant %q does not match dataset tenant %q", features.TenantID, tenantID)
	}
	if features.DatasetVersion != ProtocolID {
		return validatedBaselineInput{}, wrapInvalid("feature dataset version %q", features.DatasetVersion)
	}
	if features.FeatureDefinitionVersion != FeatureDefinitionVersion {
		return validatedBaselineInput{}, wrapInvalid("feature definition version %q", features.FeatureDefinitionVersion)
	}
	if features.Checksum == "" || features.Checksum != FeatureChecksum(features) {
		return validatedBaselineInput{}, wrapInvalid("feature snapshot checksum")
	}
	if features.SnapshotID == "" || features.SnapshotID != snapshotIdentity(features) {
		return validatedBaselineInput{}, wrapInvalid("feature snapshot identity")
	}

	examples := append([]Example(nil), ds.Examples...)
	sort.Slice(examples, func(i, j int) bool { return exampleLess(examples[i], examples[j]) })
	labels := make(map[baselineKey]Example, len(examples))
	dateSplits := make(map[string]string)
	for _, example := range examples {
		if example.ProtocolID != ProtocolID {
			return validatedBaselineInput{}, wrapInvalid("example protocol %q", example.ProtocolID)
		}
		if example.TenantID != tenantID {
			return validatedBaselineInput{}, wrapInvalid("example tenant %q", example.TenantID)
		}
		if example.ItemID == "" {
			return validatedBaselineInput{}, wrapInvalid("empty example item_id")
		}
		if example.Label != 0 && example.Label != 1 {
			return validatedBaselineInput{}, wrapInvalid("example label %d", example.Label)
		}
		if example.SourceCutoffDate != example.AsOfDate {
			return validatedBaselineInput{}, wrapInvalid("example cutoff %q for %s", example.SourceCutoffDate, example.AsOfDate)
		}
		if _, err := parseDate(example.AsOfDate); err != nil {
			return validatedBaselineInput{}, err
		}
		if !isEvaluationSplit(example.Split) {
			return validatedBaselineInput{}, wrapInvalid("example split %q", example.Split)
		}
		if example.RowID != RowID(example.TenantID, example.ItemID, example.AsOfDate) {
			return validatedBaselineInput{}, wrapInvalid("example row_id %s", example.RowID)
		}
		key := baselineKey{tenant: example.TenantID, item: example.ItemID, date: example.AsOfDate}
		if _, exists := labels[key]; exists {
			return validatedBaselineInput{}, wrapInvalid("duplicate example %s", example.RowID)
		}
		labels[key] = example
		if prior, exists := dateSplits[example.AsOfDate]; exists && prior != example.Split {
			return validatedBaselineInput{}, wrapInvalid("non-chronological date %s crosses splits", example.AsOfDate)
		}
		dateSplits[example.AsOfDate] = example.Split
	}
	if err := validateSplitChronology(dateSplits); err != nil {
		return validatedBaselineInput{}, err
	}
	expectedSplits := assignSplits(sortedStringKeys(dateSplits))
	for _, date := range sortedStringKeys(dateSplits) {
		if expected, exists := expectedSplits[date]; !exists || expected != dateSplits[date] {
			return validatedBaselineInput{}, wrapInvalid("frozen split assignment for date %s", date)
		}
	}

	featureRows := make(map[baselineKey]FeatureRow, len(features.Rows))
	for _, row := range features.Rows {
		if row.TenantID != tenantID {
			return validatedBaselineInput{}, wrapInvalid("feature row tenant %q", row.TenantID)
		}
		if row.ItemID == "" {
			return validatedBaselineInput{}, wrapInvalid("empty feature item_id")
		}
		if _, err := parseDate(row.AsOfDate); err != nil {
			return validatedBaselineInput{}, err
		}
		if row.SourceCutoffDate != row.AsOfDate {
			return validatedBaselineInput{}, wrapInvalid("feature cutoff %q for %s", row.SourceCutoffDate, row.AsOfDate)
		}
		if row.QuantityOnHand < 0 {
			return validatedBaselineInput{}, wrapInvalid("negative feature quantity for %s", row.RowID)
		}
		if row.RowID != RowID(row.TenantID, row.ItemID, row.AsOfDate) {
			return validatedBaselineInput{}, wrapInvalid("feature row_id %s", row.RowID)
		}
		key := baselineKey{tenant: row.TenantID, item: row.ItemID, date: row.AsOfDate}
		example, exists := labels[key]
		if !exists {
			return validatedBaselineInput{}, wrapInvalid("extra feature row %s", row.RowID)
		}
		if row.Split != example.Split || row.HistoryHash != example.HistoryHash {
			return validatedBaselineInput{}, wrapInvalid("feature lineage mismatch for %s", row.RowID)
		}
		if _, exists := featureRows[key]; exists {
			return validatedBaselineInput{}, wrapInvalid("duplicate feature row %s", row.RowID)
		}
		featureRows[key] = row
	}
	if len(featureRows) != len(labels) {
		return validatedBaselineInput{}, wrapInvalid("feature row count %d, want %d", len(featureRows), len(labels))
	}

	return validatedBaselineInput{
		examples:    examples,
		labels:      labels,
		featureRows: featureRows,
	}, nil
}

func baselinePredictions(input validatedBaselineInput, baselineID string) ([]Prediction, error) {
	predictions := make([]Prediction, 0, len(input.examples))
	for _, example := range input.examples {
		score, ok, err := baselineScore(input, example, baselineID)
		if err != nil {
			return nil, err
		}
		prediction := Prediction{RowID: example.RowID}
		if ok {
			value := score
			prediction.Score = &value
		}
		predictions = append(predictions, prediction)
	}
	return predictions, nil
}

func predictionsForSplit(ds Dataset, split string, predictions []Prediction) []Prediction {
	byID := make(map[string]Prediction, len(predictions))
	for _, prediction := range predictions {
		byID[prediction.RowID] = prediction
	}
	examples := append([]Example(nil), ds.Examples...)
	sort.Slice(examples, func(i, j int) bool { return exampleLess(examples[i], examples[j]) })
	out := make([]Prediction, 0, len(examples))
	for _, example := range examples {
		if example.Split != split {
			continue
		}
		out = append(out, byID[example.RowID])
	}
	return out
}

func baselineScore(input validatedBaselineInput, example Example, baselineID string) (float64, bool, error) {
	switch baselineID {
	case BaselineSeasonalNaive:
		lookback, ok := addDays(example.AsOfDate, -HorizonDays)
		if !ok {
			return 0, false, wrapInvalid("as_of_date %s", example.AsOfDate)
		}
		prior, exists := input.labels[baselineKey{tenant: example.TenantID, item: example.ItemID, date: lookback}]
		if !exists {
			return 0, false, nil
		}
		return float64(prior.Label), true, nil
	case BaselineMovingAverage:
		var zeroDays int
		for offset := 0; offset < movingAverageWindowDays; offset++ {
			date, ok := addDays(example.AsOfDate, -offset)
			if !ok {
				return 0, false, wrapInvalid("as_of_date %s", example.AsOfDate)
			}
			row, exists := input.featureRows[baselineKey{tenant: example.TenantID, item: example.ItemID, date: date}]
			if !exists {
				return 0, false, nil
			}
			if row.QuantityOnHand == 0 {
				zeroDays++
			}
		}
		return float64(zeroDays) / float64(movingAverageWindowDays), true, nil
	default:
		return 0, false, wrapInvalid("unsupported baseline %s", baselineID)
	}
}

func isDeclaredBaseline(id string) bool {
	for _, declared := range declaredBaselines {
		if id == declared {
			return true
		}
	}
	return false
}

func isEvaluationSplit(split string) bool {
	for _, declared := range evaluationSplits {
		if split == declared {
			return true
		}
	}
	return false
}

func validateSplitChronology(dateSplits map[string]string) error {
	lastSplit := ""
	for _, date := range sortedStringKeys(dateSplits) {
		split := dateSplits[date]
		if lastSplit != "" && splitRank(split) < splitRank(lastSplit) {
			return wrapInvalid("non-chronological split at %s", date)
		}
		lastSplit = split
	}
	return nil
}

func splitRank(split string) int {
	switch split {
	case SplitTrain:
		return 1
	case SplitValidation:
		return 2
	case SplitTest:
		return 3
	default:
		return 0
	}
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
