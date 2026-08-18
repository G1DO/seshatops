package platform

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/G1DO/seshatops/forecast"
)

type runtimeCandidateStub struct {
	calls int
}

func (s *runtimeCandidateStub) Invoke(_ context.Context, request forecast.RuntimeRequest) (forecast.RuntimeResponse, error) {
	s.calls++
	score := 0.25
	return forecast.RuntimeResponse{
		ContractVersion:          forecast.RuntimeContractVersion,
		Predictor:                request.Predictor,
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
		Uncertainty:              &forecast.PredictionUncertainty{Method: forecast.CandidateUncertaintyMethod, Lower: 0, Upper: 0.5, SampleCount: 10},
	}, nil
}

func TestForecastServicePersistsBaselineIdempotentlyAndScopesTenant(t *testing.T) {
	db := openTestDB(t)
	snapshot := serviceSnapshot(t, forecast.TenantNS001)
	request := ForecastRequest{
		TenantID:        forecast.TenantNS001,
		ItemID:          forecast.ItemFlour,
		ObservationDate: "2026-01-12",
		CorrelationID:   "018f5d78-6e64-4f5f-bd16-8e9f7c4a20f1",
		Features:        snapshot,
		Evaluation:      baselineEvaluation(),
		CandidateArtifact: &forecast.CandidateArtifact{
			ModelVersion: "candidate-context-is-not-selected",
		},
	}
	service := NewForecastService(db, nil)
	first, err := service.PredictAndPersist(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.PredictAndPersist(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.PredictionID != second.PredictionID || first.StockoutRisk == nil || first.Uncertainty == nil {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.forecast_predictions WHERE tenant_id = $1`, forecast.TenantNS001).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("prediction count=%d", count)
	}
	if _, found, err := GetPredictionForTenant(context.Background(), db, forecast.TenantNS002, first.PredictionID); err != nil || found {
		t.Fatalf("cross-tenant lookup found=%v err=%v", found, err)
	}
	records, err := ListPredictionsForTenant(context.Background(), db, forecast.TenantNS001)
	if err != nil || len(records) != 1 || records[0].TenantID != forecast.TenantNS001 {
		t.Fatalf("records=%+v err=%v", records, err)
	}
}

func TestForecastServiceInvokesCandidateOnlyWhenPromoted(t *testing.T) {
	db := openTestDB(t)
	snapshot := serviceSnapshot(t, forecast.TenantNS001)
	stub := &runtimeCandidateStub{}
	service := NewForecastService(db, stub)
	request := ForecastRequest{
		TenantID:        forecast.TenantNS001,
		ItemID:          forecast.ItemFlour,
		ObservationDate: "2026-01-12",
		CorrelationID:   "018f5d78-6e64-4f5f-bd16-8e9f7c4a20f2",
		Features:        snapshot,
		Evaluation:      candidateEvaluation(),
	}
	record, err := service.PredictAndPersist(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if stub.calls != 1 || record.Predictor != forecast.RuntimePredictorCandidate || record.StockoutRisk == nil {
		t.Fatalf("calls=%d record=%+v", stub.calls, record)
	}

	service = NewForecastService(db, nil)
	request.Evaluation = baselineEvaluation()
	request.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20f3"
	if _, err := service.PredictAndPersist(context.Background(), request); !errors.Is(err, ErrPredictionConflict) {
		t.Fatalf("predictor change err=%v, want conflict", err)
	}
}

func baselineEvaluation() forecast.CandidateEvaluation {
	return forecast.CandidateEvaluation{
		EvaluationProtocolVersion: forecast.ProtocolID,
		DatasetVersion:            forecast.ProtocolID,
		PromotionSplit:            forecast.SplitTest,
		Outcome:                   forecast.CandidateOutcomeBaseline,
		BaselineEvaluation: forecast.BaselineEvaluation{
			CodeVersion: forecast.EvaluationCodeVersion,
		},
		Splits: []forecast.CandidateSplitEvaluation{{
			Split:                  forecast.SplitTest,
			BaselineID:             forecast.BaselineMovingAverage,
			CandidateBeatsBaseline: false,
		}},
	}
}

func candidateEvaluation() forecast.CandidateEvaluation {
	evaluation := baselineEvaluation()
	evaluation.Outcome = forecast.CandidateOutcomeCandidate
	evaluation.PromotionEligible = true
	evaluation.ModelVersion = forecast.CandidateModelVersion
	evaluation.CodeVersion = forecast.CandidateCodeVersion
	evaluation.Splits[0].CandidateBeatsBaseline = true
	return evaluation
}

func serviceSnapshot(t *testing.T, tenant string) forecast.FeatureSnapshot {
	t.Helper()
	start, err := time.Parse("2006-01-02", "2026-01-05")
	if err != nil {
		t.Fatal(err)
	}
	observations := make([]forecast.Observation, 21)
	for i := range observations {
		observations[i] = forecast.Observation{
			TenantID:       tenant,
			ItemID:         forecast.ItemFlour,
			AsOfDate:       start.AddDate(0, 0, i).Format("2006-01-02"),
			QuantityOnHand: int64(i % 4),
		}
	}
	snapshot, err := forecast.BuildFeatureSnapshot(forecast.History{Observations: observations}, tenant, forecast.SourceBoundary{
		HistoryChecksum: "history-v1",
		MaxRecordedAt:   "2026-01-25T09:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
