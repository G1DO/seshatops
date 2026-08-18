package forecast_test

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/G1DO/seshatops/forecast"
)

func TestEvaluateCandidateArtifactRejectsInvalidPredictionContracts(t *testing.T) {
	ds, features := officialCandidateInputs(t)
	valid := candidateArtifact(ds, features, func(e forecast.Example) *float64 {
		return forecastFloat(0.5)
	})

	tests := []struct {
		name   string
		mutate func(*forecast.CandidateArtifact)
		want   string
	}{
		{
			name: "model version",
			mutate: func(artifact *forecast.CandidateArtifact) {
				artifact.ModelVersion = "other-model-v1"
			},
			want: "model version",
		},
		{
			name: "missing prediction",
			mutate: func(artifact *forecast.CandidateArtifact) {
				artifact.Predictions = artifact.Predictions[:len(artifact.Predictions)-1]
			},
			want: "prediction count",
		},
		{
			name: "risk without uncertainty",
			mutate: func(artifact *forecast.CandidateArtifact) {
				artifact.Predictions[0].Uncertainty = nil
			},
			want: "predicted state",
		},
		{
			name: "invalid uncertainty",
			mutate: func(artifact *forecast.CandidateArtifact) {
				artifact.Predictions[0].Uncertainty.Lower = 0.75
				artifact.Predictions[0].Uncertainty.Upper = 0.25
			},
			want: "uncertainty",
		},
		{
			name: "unknown abstention reason",
			mutate: func(artifact *forecast.CandidateArtifact) {
				artifact.Predictions[0].StockoutRisk = nil
				artifact.Predictions[0].Uncertainty = nil
				artifact.Predictions[0].Status = forecast.CandidatePredictionStatusAbstained
				artifact.Predictions[0].AbstentionReason = "made-up-reason"
			},
			want: "abstention state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := valid
			artifact.Predictions = append([]forecast.CandidatePrediction(nil), valid.Predictions...)
			tt.mutate(&artifact)
			_, err := forecast.EvaluateCandidateArtifact(ds, features, artifact)
			if !errors.Is(err, forecast.ErrInvalidInput) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err=%v, want invalid input containing %q", err, tt.want)
			}
		})
	}

	stale := forecast.NewNonCompleteFeatureSnapshot(
		forecast.SnapshotStatusStale,
		forecast.TenantNS001,
		forecast.SourceBoundary{},
		"pending source",
	)
	if _, err := forecast.EvaluateCandidateArtifact(ds, stale, valid); !errors.Is(err, forecast.ErrInvalidInput) {
		t.Fatalf("stale feature snapshot err=%v", err)
	}
}

func TestEvaluateCandidateArtifactFallsBackWhenCandidateAbstains(t *testing.T) {
	ds, features := officialCandidateInputs(t)
	abstaining := candidateArtifact(ds, features, func(forecast.Example) *float64 {
		return nil
	})
	for i := range abstaining.Predictions {
		abstaining.Predictions[i].Status = forecast.CandidatePredictionStatusAbstained
		abstaining.Predictions[i].AbstentionReason = forecast.CandidateAbstentionInsufficientSupport
	}

	evaluation, err := forecast.EvaluateCandidateArtifact(ds, features, abstaining)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.PromotionEligible || evaluation.Outcome == forecast.CandidateOutcomeCandidate {
		t.Fatalf("abstaining candidate outcome=%+v", evaluation)
	}
	if evaluation.PromotionSplit != forecast.SplitTest || len(evaluation.Splits) != 3 {
		t.Fatalf("evaluation shape=%+v", evaluation)
	}
	test := splitCandidateResult(evaluation.Splits, forecast.SplitTest)
	if test.Candidate.Defined || test.Candidate.Reason != "all abstain" {
		t.Fatalf("test candidate result=%+v", test.Candidate)
	}
	if evaluation.Reason == "" {
		t.Fatal("missing fallback reason")
	}
}

func TestEvaluateCandidateArtifactJSONIsStrictAndRecordsLineage(t *testing.T) {
	ds, features := officialCandidateInputs(t)
	artifact := candidateArtifact(ds, features, func(e forecast.Example) *float64 {
		if e.Label == 1 {
			return forecastFloat(0.99)
		}
		return forecastFloat(0.01)
	})
	raw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := forecast.EvaluateCandidateArtifactJSON(ds, features, raw)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.DatasetChecksum != forecast.Checksum(ds) ||
		evaluation.FeatureSnapshotID != features.SnapshotID ||
		evaluation.FeatureSnapshotChecksum != features.Checksum ||
		evaluation.EvaluationProtocolVersion != forecast.ProtocolID {
		t.Fatalf("lineage=%+v", evaluation)
	}

	unknown := append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"unexpected":true}`)...)
	if _, err := forecast.EvaluateCandidateArtifactJSON(ds, features, unknown); !errors.Is(err, forecast.ErrInvalidInput) {
		t.Fatalf("unknown field err=%v", err)
	}
}

func officialCandidateInputs(t *testing.T) (forecast.Dataset, forecast.FeatureSnapshot) {
	t.Helper()
	history := officialHistory(t)
	ds, err := forecast.BuildDataset(history, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	features, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{
		HistoryChecksum:    "history-candidate",
		ProjectionChecksum: "projection-candidate",
		MaxRecordedAt:      "2026-04-26T23:59:59Z",
		AppliedEventCount:  336,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ds, features
}

func candidateArtifact(ds forecast.Dataset, features forecast.FeatureSnapshot, scoreFn func(forecast.Example) *float64) forecast.CandidateArtifact {
	predictions := make([]forecast.CandidatePrediction, 0, len(ds.Examples))
	for _, example := range ds.Examples {
		score := scoreFn(example)
		prediction := forecast.CandidatePrediction{
			RowID:            example.RowID,
			Target:           forecast.CandidateTarget,
			HorizonDays:      forecast.HorizonDays,
			SourceCutoffDate: example.AsOfDate,
			Status:           forecast.CandidatePredictionStatusPredicted,
			StockoutRisk:     score,
			Uncertainty: &forecast.PredictionUncertainty{
				Method:      forecast.CandidateUncertaintyMethod,
				Lower:       0,
				Upper:       1,
				SampleCount: 5,
			},
		}
		if score == nil {
			prediction.Uncertainty = nil
		}
		predictions = append(predictions, prediction)
	}
	return forecast.CandidateArtifact{
		ArtifactVersion:           forecast.CandidateArtifactVersion,
		ModelVersion:              forecast.CandidateModelVersion,
		CodeVersion:               forecast.CandidateCodeVersion,
		EvaluationProtocolVersion: forecast.ProtocolID,
		DatasetVersion:            ds.ProtocolID,
		DatasetChecksum:           forecast.Checksum(ds),
		FeatureDefinitionVersion:  features.FeatureDefinitionVersion,
		FeatureSnapshotID:         features.SnapshotID,
		FeatureSnapshotChecksum:   features.Checksum,
		Target:                    forecast.CandidateTarget,
		HorizonDays:               forecast.HorizonDays,
		TrainingSplit:             forecast.CandidateTrainingSplit,
		TuningSplit:               forecast.CandidateTuningSplit,
		Predictions:               predictions,
	}
}

func splitCandidateResult(results []forecast.CandidateSplitEvaluation, split string) forecast.CandidateSplitEvaluation {
	for _, result := range results {
		if result.Split == split {
			return result
		}
	}
	return forecast.CandidateSplitEvaluation{}
}

func forecastFloat(value float64) *float64 {
	if math.IsNaN(value) {
		return nil
	}
	return &value
}
