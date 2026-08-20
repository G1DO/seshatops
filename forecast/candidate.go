package forecast

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
)

const (
	// CandidateArtifactVersion identifies the JSON contract produced by the
	// offline learned candidate.
	CandidateArtifactVersion = 1

	// CandidateModelVersion identifies the fixed candidate model family.
	CandidateModelVersion = "m4-onhand-bucket-rate-v1"

	// CandidateCodeVersion identifies the Python artifact producer contract.
	CandidateCodeVersion = "m4-python-stockout-candidate-v1"

	CandidateTarget = "stockout-within-horizon"

	CandidateTrainingSplit  = SplitTrain
	CandidateTuningSplit    = SplitValidation
	CandidatePromotionSplit = SplitTest

	CandidatePredictionStatusPredicted = "predicted"
	CandidatePredictionStatusAbstained = "abstained"

	CandidateUncertaintyMethod = "wilson-95"

	CandidateAbstentionInsufficientSupport = "insufficient_training_support"
	CandidateAbstentionUnsupportedInput    = "unsupported_feature_input"

	CandidateOutcomeCandidate            = "candidate"
	CandidateOutcomeBaseline             = "baseline"
	CandidateOutcomeNoQualifyingBaseline = "no_qualifying_baseline"
)

// CandidateInput is the read-only JSON input accepted by the offline Python
// candidate. It contains no credentials or mutable platform state.
type CandidateInput struct {
	Dataset         Dataset         `json:"dataset"`
	DatasetChecksum string          `json:"dataset_checksum"`
	Features        FeatureSnapshot `json:"features"`
}

// CandidateArtifact is the typed prediction artifact emitted by the offline
// candidate. It is not an authorization or workflow decision.
type CandidateArtifact struct {
	ArtifactVersion           int                   `json:"artifact_version"`
	ModelVersion              string                `json:"model_version"`
	CodeVersion               string                `json:"code_version"`
	EvaluationProtocolVersion string                `json:"evaluation_protocol_version"`
	DatasetVersion            string                `json:"dataset_version"`
	DatasetChecksum           string                `json:"dataset_checksum"`
	FeatureDefinitionVersion  string                `json:"feature_definition_version"`
	FeatureSnapshotID         string                `json:"feature_snapshot_id"`
	FeatureSnapshotChecksum   string                `json:"feature_snapshot_checksum"`
	Target                    string                `json:"target"`
	HorizonDays               int                   `json:"horizon_days"`
	TrainingSplit             string                `json:"training_split"`
	TuningSplit               string                `json:"tuning_split"`
	Predictions               []CandidatePrediction `json:"predictions"`
}

// CandidatePrediction is one typed risk result. A nil StockoutRisk is an
// explicit abstention and must carry a reason.
type CandidatePrediction struct {
	RowID            string                 `json:"row_id"`
	Target           string                 `json:"target"`
	HorizonDays      int                    `json:"horizon_days"`
	SourceCutoffDate string                 `json:"source_cutoff_date"`
	Status           string                 `json:"status"`
	StockoutRisk     *float64               `json:"stockout_risk,omitempty"`
	Uncertainty      *PredictionUncertainty `json:"uncertainty,omitempty"`
	AbstentionReason string                 `json:"abstention_reason,omitempty"`
}

// PredictionUncertainty records the interval method and support behind a
// predicted score.
type PredictionUncertainty struct {
	Method      string  `json:"method"`
	Lower       float64 `json:"lower"`
	Upper       float64 `json:"upper"`
	SampleCount int     `json:"sample_count"`
}

// CandidateSplitEvaluation compares the candidate with the selected baseline
// for one frozen split.
type CandidateSplitEvaluation struct {
	Split                  string  `json:"split"`
	Candidate              Result  `json:"candidate"`
	BaselineID             string  `json:"baseline_id,omitempty"`
	Baseline               *Result `json:"baseline,omitempty"`
	CandidateBeatsBaseline bool    `json:"candidate_beats_baseline"`
	ComparisonReason       string  `json:"comparison_reason"`
}

// CandidateEvaluation is the Go-owned, abstention-aware evaluation result.
// PromotionEligible and Outcome refer only to the frozen test split.
type CandidateEvaluation struct {
	ArtifactVersion           int                        `json:"artifact_version"`
	ModelVersion              string                     `json:"model_version"`
	CodeVersion               string                     `json:"code_version"`
	EvaluationProtocolVersion string                     `json:"evaluation_protocol_version"`
	DatasetVersion            string                     `json:"dataset_version"`
	DatasetChecksum           string                     `json:"dataset_checksum"`
	FeatureDefinitionVersion  string                     `json:"feature_definition_version"`
	FeatureSnapshotID         string                     `json:"feature_snapshot_id"`
	FeatureSnapshotChecksum   string                     `json:"feature_snapshot_checksum"`
	PromotionSplit            string                     `json:"promotion_split"`
	PromotionEligible         bool                       `json:"promotion_eligible"`
	Outcome                   string                     `json:"outcome"`
	Reason                    string                     `json:"reason"`
	BaselineEvaluation        BaselineEvaluation         `json:"baseline_evaluation"`
	Splits                    []CandidateSplitEvaluation `json:"splits"`
}

// EvaluateCandidateArtifactJSON decodes one strict artifact and evaluates it
// against the frozen Go-owned protocol and deterministic baselines.
func EvaluateCandidateArtifactJSON(ds Dataset, features FeatureSnapshot, raw []byte) (CandidateEvaluation, error) {
	artifact, err := DecodeCandidateArtifactJSON(raw)
	if err != nil {
		return CandidateEvaluation{}, err
	}
	return EvaluateCandidateArtifact(ds, features, artifact)
}

// DecodeCandidateArtifactJSON decodes one strict artifact emitted by the
// offline candidate producer. Unknown and trailing JSON are rejected before
// the artifact can enter evaluation or runtime persistence.
func DecodeCandidateArtifactJSON(raw []byte) (CandidateArtifact, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var artifact CandidateArtifact
	if err := decoder.Decode(&artifact); err != nil {
		return CandidateArtifact{}, wrapInvalid("candidate artifact: %v", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return CandidateArtifact{}, wrapInvalid("candidate artifact has trailing JSON")
		}
		return CandidateArtifact{}, wrapInvalid("candidate artifact trailing data: %v", err)
	}
	return artifact, nil
}

// EvaluateCandidateArtifact validates and evaluates one offline candidate
// artifact on every frozen split. Only the test comparison can make the
// candidate promotion-eligible.
func EvaluateCandidateArtifact(ds Dataset, features FeatureSnapshot, artifact CandidateArtifact) (CandidateEvaluation, error) {
	input, err := validateBaselineInput(ds, features, "")
	if err != nil {
		return CandidateEvaluation{}, err
	}
	predictions, err := validateCandidateArtifact(input, ds, features, artifact)
	if err != nil {
		return CandidateEvaluation{}, err
	}
	baselines, err := EvaluateBaselines(ds, features)
	if err != nil {
		return CandidateEvaluation{}, err
	}

	evaluation := CandidateEvaluation{
		ArtifactVersion:           artifact.ArtifactVersion,
		ModelVersion:              artifact.ModelVersion,
		CodeVersion:               artifact.CodeVersion,
		EvaluationProtocolVersion: artifact.EvaluationProtocolVersion,
		DatasetVersion:            artifact.DatasetVersion,
		DatasetChecksum:           artifact.DatasetChecksum,
		FeatureDefinitionVersion:  artifact.FeatureDefinitionVersion,
		FeatureSnapshotID:         artifact.FeatureSnapshotID,
		FeatureSnapshotChecksum:   artifact.FeatureSnapshotChecksum,
		PromotionSplit:            CandidatePromotionSplit,
		BaselineEvaluation:        baselines,
		Splits:                    make([]CandidateSplitEvaluation, 0, len(evaluationSplits)),
	}

	for _, split := range evaluationSplits {
		candidateResult, err := Evaluate(ds, split, predictionsForSplit(ds, split, predictions))
		if err != nil {
			return CandidateEvaluation{}, err
		}
		comparison := CandidateSplitEvaluation{
			Split:     split,
			Candidate: candidateResult,
		}
		baselineID, baseline, found := baselineResultForSplit(baselines, split)
		if !found {
			comparison.ComparisonReason = "no qualifying baseline"
		} else {
			comparison.BaselineID = baselineID
			baselineCopy := baseline
			comparison.Baseline = &baselineCopy
			comparison.CandidateBeatsBaseline, comparison.ComparisonReason = CandidatePromoted(candidateResult, baselineID, baseline)
		}
		evaluation.Splits = append(evaluation.Splits, comparison)

		if split == CandidatePromotionSplit {
			evaluation.PromotionEligible = comparison.CandidateBeatsBaseline
			evaluation.Reason = comparison.ComparisonReason
			switch {
			case comparison.CandidateBeatsBaseline:
				evaluation.Outcome = CandidateOutcomeCandidate
			case comparison.BaselineID != "":
				evaluation.Outcome = CandidateOutcomeBaseline
			default:
				evaluation.Outcome = CandidateOutcomeNoQualifyingBaseline
			}
		}
	}
	return evaluation, nil
}

func validateCandidateArtifact(input validatedBaselineInput, ds Dataset, features FeatureSnapshot, artifact CandidateArtifact) ([]Prediction, error) {
	if artifact.ArtifactVersion != CandidateArtifactVersion {
		return nil, wrapInvalid("candidate artifact version %d", artifact.ArtifactVersion)
	}
	if artifact.ModelVersion != CandidateModelVersion {
		return nil, wrapInvalid("candidate model version %q", artifact.ModelVersion)
	}
	if artifact.CodeVersion != CandidateCodeVersion {
		return nil, wrapInvalid("candidate code version %q", artifact.CodeVersion)
	}
	if artifact.EvaluationProtocolVersion != ProtocolID {
		return nil, wrapInvalid("candidate evaluation protocol %q", artifact.EvaluationProtocolVersion)
	}
	if artifact.DatasetVersion != ds.ProtocolID || artifact.DatasetChecksum != Checksum(ds) {
		return nil, wrapInvalid("candidate dataset lineage")
	}
	if artifact.FeatureDefinitionVersion != features.FeatureDefinitionVersion ||
		artifact.FeatureSnapshotID != features.SnapshotID ||
		artifact.FeatureSnapshotChecksum != features.Checksum {
		return nil, wrapInvalid("candidate feature lineage")
	}
	if artifact.Target != CandidateTarget || artifact.HorizonDays != HorizonDays {
		return nil, wrapInvalid("candidate target or horizon")
	}
	if artifact.TrainingSplit != CandidateTrainingSplit || artifact.TuningSplit != CandidateTuningSplit {
		return nil, wrapInvalid("candidate split declaration")
	}
	if len(artifact.Predictions) != len(input.examples) {
		return nil, wrapInvalid("candidate prediction count %d, want %d", len(artifact.Predictions), len(input.examples))
	}

	examples := make(map[string]Example, len(input.examples))
	for _, example := range input.examples {
		examples[example.RowID] = example
	}
	byID := make(map[string]CandidatePrediction, len(artifact.Predictions))
	for _, prediction := range artifact.Predictions {
		if prediction.RowID == "" {
			return nil, wrapInvalid("candidate empty row_id")
		}
		if _, duplicate := byID[prediction.RowID]; duplicate {
			return nil, wrapInvalid("candidate duplicate row_id %s", prediction.RowID)
		}
		example, ok := examples[prediction.RowID]
		if !ok {
			return nil, wrapInvalid("candidate extra row_id %s", prediction.RowID)
		}
		if prediction.Target != CandidateTarget || prediction.HorizonDays != HorizonDays {
			return nil, wrapInvalid("candidate prediction target or horizon for %s", prediction.RowID)
		}
		if prediction.SourceCutoffDate != example.AsOfDate {
			return nil, wrapInvalid("candidate cutoff for %s", prediction.RowID)
		}
		switch prediction.Status {
		case CandidatePredictionStatusPredicted:
			if prediction.StockoutRisk == nil || prediction.Uncertainty == nil || prediction.AbstentionReason != "" {
				return nil, wrapInvalid("candidate predicted state for %s", prediction.RowID)
			}
			if err := validateCandidateScore(*prediction.StockoutRisk); err != nil {
				return nil, wrapInvalid("candidate score for %s: %v", prediction.RowID, err)
			}
			if err := validateCandidateUncertainty(*prediction.Uncertainty, *prediction.StockoutRisk); err != nil {
				return nil, wrapInvalid("candidate uncertainty for %s: %v", prediction.RowID, err)
			}
		case CandidatePredictionStatusAbstained:
			if prediction.StockoutRisk != nil || prediction.Uncertainty != nil || !isCandidateAbstentionReason(prediction.AbstentionReason) {
				return nil, wrapInvalid("candidate abstention state for %s", prediction.RowID)
			}
		default:
			return nil, wrapInvalid("candidate prediction status %q for %s", prediction.Status, prediction.RowID)
		}
		byID[prediction.RowID] = prediction
	}

	predictions := make([]Prediction, 0, len(input.examples))
	for _, example := range input.examples {
		prediction, ok := byID[example.RowID]
		if !ok {
			return nil, wrapInvalid("candidate omitted row_id %s", example.RowID)
		}
		predictions = append(predictions, Prediction{RowID: prediction.RowID, Score: prediction.StockoutRisk})
	}
	return predictions, nil
}

func validateCandidateScore(score float64) error {
	if math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 1 {
		return wrapInvalid("score %v", score)
	}
	return nil
}

func validateCandidateUncertainty(uncertainty PredictionUncertainty, risk float64) error {
	if uncertainty.Method != CandidateUncertaintyMethod || uncertainty.SampleCount <= 0 {
		return wrapInvalid("method or sample count")
	}
	if err := validateCandidateScore(uncertainty.Lower); err != nil {
		return err
	}
	if err := validateCandidateScore(uncertainty.Upper); err != nil {
		return err
	}
	if uncertainty.Lower > uncertainty.Upper || risk < uncertainty.Lower || risk > uncertainty.Upper {
		return wrapInvalid("interval bounds")
	}
	return nil
}

func isCandidateAbstentionReason(reason string) bool {
	switch reason {
	case CandidateAbstentionInsufficientSupport, CandidateAbstentionUnsupportedInput:
		return true
	default:
		return false
	}
}

func baselineResultForSplit(evaluation BaselineEvaluation, split string) (string, Result, bool) {
	var baselineID string
	for _, selection := range evaluation.Selections {
		if selection.Split == split && selection.Found {
			baselineID = selection.BaselineID
			break
		}
	}
	if baselineID == "" {
		return "", Result{}, false
	}
	for _, run := range evaluation.Baselines {
		if run.ID != baselineID {
			continue
		}
		for _, result := range run.Results {
			if result.Split == split {
				return baselineID, result, true
			}
		}
	}
	return "", Result{}, false
}
