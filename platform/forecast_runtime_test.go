package platform

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/G1DO/seshatops/forecast"
)

func TestCommandCandidateInvokerUsesTypedPythonBoundary(t *testing.T) {
	python, err := exec.LookPath("python")
	if err != nil {
		python, err = exec.LookPath("python3")
	}
	if err != nil {
		t.Skipf("Python runtime not installed: %v", err)
	}
	request := commandRuntimeRequest(t)
	invoker := CommandCandidateInvoker{
		Command: python,
		Args:    []string{filepath.Join("..", "forecast_candidate", "runtime.py")},
	}
	response, err := invoker.Invoke(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != forecast.CandidatePredictionStatusPredicted || response.StockoutRisk == nil || *response.StockoutRisk != 0.4 {
		t.Fatalf("response=%+v", response)
	}

	bad, err := (CommandCandidateInvoker{Command: python, Args: []string{"-c", "print('{}')"}}).Invoke(context.Background(), request)
	if !errors.Is(err, ErrPythonInvalidResponse) || bad != (forecast.RuntimeResponse{}) {
		t.Fatalf("bad response=%+v err=%v", bad, err)
	}

	_, err = (CommandCandidateInvoker{Command: python, Args: []string{"-c", "import time; time.sleep(1)"}, Timeout: 10 * time.Millisecond}).Invoke(context.Background(), request)
	if !errors.Is(err, ErrPythonTimeout) {
		t.Fatalf("timeout err=%v", err)
	}

	_, err = (CommandCandidateInvoker{Command: "command-that-does-not-exist-seshatops"}).Invoke(context.Background(), request)
	if !errors.Is(err, ErrPythonUnavailable) {
		t.Fatalf("unavailable err=%v", err)
	}
}

func commandRuntimeRequest(t *testing.T) forecast.RuntimeRequest {
	t.Helper()
	rowID := forecast.RowID(forecast.TenantNS001, forecast.ItemFlour, "2026-01-12")
	return forecast.RuntimeRequest{
		ContractVersion:          forecast.RuntimeContractVersion,
		Predictor:                forecast.RuntimePredictorCandidate,
		TenantID:                 forecast.TenantNS001,
		ItemID:                   forecast.ItemFlour,
		RowID:                    rowID,
		ObservationDate:          "2026-01-12",
		Target:                   forecast.CandidateTarget,
		HorizonDays:              forecast.HorizonDays,
		FeatureDefinitionVersion: forecast.FeatureDefinitionVersion,
		FeatureSnapshotID:        "snapshot-1",
		FeatureSnapshotChecksum:  "checksum-1",
		ModelVersion:             forecast.CandidateModelVersion,
		CodeVersion:              forecast.CandidateCodeVersion,
		Feature: forecast.FeatureRow{
			RowID:            rowID,
			TenantID:         forecast.TenantNS001,
			ItemID:           forecast.ItemFlour,
			AsOfDate:         "2026-01-12",
			SourceCutoffDate: "2026-01-12",
			Split:            forecast.SplitTrain,
			QuantityOnHand:   2,
			HistoryHash:      "history-1",
		},
		ModelArtifact: &forecast.CandidateArtifact{
			ModelVersion:              forecast.CandidateModelVersion,
			CodeVersion:               forecast.CandidateCodeVersion,
			EvaluationProtocolVersion: forecast.ProtocolID,
			FeatureDefinitionVersion:  forecast.FeatureDefinitionVersion,
			FeatureSnapshotID:         "snapshot-1",
			FeatureSnapshotChecksum:   "checksum-1",
			Predictions: []forecast.CandidatePrediction{{
				RowID:            rowID,
				Target:           forecast.CandidateTarget,
				HorizonDays:      forecast.HorizonDays,
				SourceCutoffDate: "2026-01-12",
				Status:           forecast.CandidatePredictionStatusPredicted,
				StockoutRisk:     float64Ptr(0.4),
				Uncertainty: &forecast.PredictionUncertainty{
					Method:      forecast.CandidateUncertaintyMethod,
					Lower:       0.1,
					Upper:       0.7,
					SampleCount: 10,
				},
			}},
		},
	}
}

func float64Ptr(value float64) *float64 { return &value }
