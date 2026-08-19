package forecast_test

import (
	"errors"
	"testing"
	"time"

	"github.com/G1DO/seshatops/forecast"
)

func TestSelectRuntimePredictorHonorsFrozenOutcome(t *testing.T) {
	base := forecast.CandidateEvaluation{
		EvaluationProtocolVersion: forecast.ProtocolID,
		PromotionSplit:            forecast.SplitTest,
		BaselineEvaluation: forecast.BaselineEvaluation{
			CodeVersion: forecast.EvaluationCodeVersion,
		},
		Splits: []forecast.CandidateSplitEvaluation{{
			Split:                  forecast.SplitTest,
			BaselineID:             forecast.BaselineMovingAverage,
			CandidateBeatsBaseline: false,
		}},
	}

	baseline := base
	baseline.Outcome = forecast.CandidateOutcomeBaseline
	selection, err := forecast.SelectRuntimePredictor(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if selection.Predictor != forecast.RuntimePredictorBaseline || selection.BaselineID != forecast.BaselineMovingAverage {
		t.Fatalf("selection=%+v", selection)
	}

	candidate := base
	candidate.Outcome = forecast.CandidateOutcomeCandidate
	candidate.PromotionEligible = true
	candidate.ModelVersion = forecast.CandidateModelVersion
	candidate.CodeVersion = forecast.CandidateCodeVersion
	candidate.Splits[0].CandidateBeatsBaseline = true
	selection, err = forecast.SelectRuntimePredictor(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if selection.Predictor != forecast.RuntimePredictorCandidate || selection.ModelVersion != forecast.CandidateModelVersion {
		t.Fatalf("selection=%+v", selection)
	}

	candidate.PromotionEligible = false
	if _, err := forecast.SelectRuntimePredictor(candidate); !errors.Is(err, forecast.ErrInvalidInput) {
		t.Fatalf("contradictory candidate err=%v", err)
	}
}

func TestRuntimeContractValidatesLineageAndBaselineOutput(t *testing.T) {
	snapshot := runtimeSnapshot(t)
	selection := forecast.RuntimeSelection{
		Predictor:    forecast.RuntimePredictorCandidate,
		ModelVersion: forecast.CandidateModelVersion,
		CodeVersion:  forecast.CandidateCodeVersion,
	}
	request, err := forecast.NewRuntimeRequest(snapshot, forecast.TenantNS001, forecast.ItemFlour, "2026-01-12", selection, nil)
	if err != nil {
		t.Fatal(err)
	}
	score := 0.4
	response := forecast.RuntimeResponse{
		ContractVersion:          forecast.RuntimeContractVersion,
		Predictor:                forecast.RuntimePredictorCandidate,
		TenantID:                 request.TenantID,
		ItemID:                   request.ItemID,
		RowID:                    request.RowID,
		ObservationDate:          request.ObservationDate,
		SourceCutoffDate:         request.ObservationDate,
		Target:                   request.Target,
		HorizonDays:              request.HorizonDays,
		FeatureDefinitionVersion: request.FeatureDefinitionVersion,
		FeatureSnapshotID:        request.FeatureSnapshotID,
		FeatureSnapshotChecksum:  request.FeatureSnapshotChecksum,
		ModelVersion:             request.ModelVersion,
		CodeVersion:              request.CodeVersion,
		Status:                   forecast.CandidatePredictionStatusPredicted,
		StockoutRisk:             &score,
		Uncertainty:              &forecast.PredictionUncertainty{Method: forecast.CandidateUncertaintyMethod, Lower: 0.1, Upper: 0.8, SampleCount: 10},
	}
	if err := forecast.ValidateRuntimeResponse(request, response); err != nil {
		t.Fatal(err)
	}
	response.TenantID = forecast.TenantNS002
	if err := forecast.ValidateRuntimeResponse(request, response); !errors.Is(err, forecast.ErrInvalidInput) {
		t.Fatalf("tenant mismatch err=%v", err)
	}

	baselineRequest, err := forecast.NewRuntimeRequest(snapshot, forecast.TenantNS001, forecast.ItemFlour, "2026-01-12", forecast.RuntimeSelection{
		Predictor:    forecast.RuntimePredictorBaseline,
		BaselineID:   forecast.BaselineMovingAverage,
		ModelVersion: forecast.BaselineMovingAverage,
		CodeVersion:  forecast.EvaluationCodeVersion,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	baselineResponse, err := forecast.BaselineRuntimeResponse(snapshot, baselineRequest)
	if err != nil {
		t.Fatal(err)
	}
	if baselineResponse.Status != forecast.CandidatePredictionStatusPredicted || baselineResponse.Uncertainty == nil || baselineResponse.Uncertainty.Method != forecast.RuntimeUncertaintyDeterministic {
		t.Fatalf("baseline response=%+v", baselineResponse)
	}
}

func runtimeSnapshot(t *testing.T) forecast.FeatureSnapshot {
	t.Helper()
	start, err := time.Parse("2006-01-02", "2026-01-05")
	if err != nil {
		t.Fatal(err)
	}
	observations := make([]forecast.Observation, 21)
	for i := range observations {
		observations[i] = forecast.Observation{
			TenantID:       forecast.TenantNS001,
			ItemID:         forecast.ItemFlour,
			AsOfDate:       start.AddDate(0, 0, i).Format("2006-01-02"),
			QuantityOnHand: int64(i % 4),
		}
	}
	snapshot, err := forecast.BuildFeatureSnapshot(forecast.History{Observations: observations}, forecast.TenantNS001, forecast.SourceBoundary{
		HistoryChecksum: "history-v1",
		MaxRecordedAt:   "2026-01-25T09:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
