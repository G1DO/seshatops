package main

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/G1DO/seshatops/forecast"
	"github.com/G1DO/seshatops/platform"
)

func TestRunFrozenForecastReproducesM4SelectionAndBoundedResult(t *testing.T) {
	python := testPython(t)
	producer := platform.CommandCandidateArtifactProducer{
		Command: python,
		Script:  filepath.Join("..", "..", "forecast_candidate", "stockout_candidate.py"),
	}
	persister := &recordingForecastPersister{}

	result, err := runFrozenForecast(context.Background(), producer, persister)
	if err != nil {
		t.Fatal(err)
	}
	if result.DatasetChecksum != frozenM4DatasetChecksum || result.FeatureSnapshotID != frozenM4FeatureSnapshotID || result.FeatureSnapshotChecksum != frozenM4FeatureSnapshotChecksum {
		t.Fatalf("lineage=%+v", result)
	}
	if result.SelectedPredictor != forecast.RuntimePredictorBaseline || result.SelectedModelVersion != forecast.BaselineSeasonalNaive || result.EvaluationOutcome != forecast.CandidateOutcomeBaseline {
		t.Fatalf("selection=%+v", result)
	}
	if result.PredictionID == "" || result.PredictionStatus != forecast.CandidatePredictionStatusPredicted || persister.calls != 1 {
		t.Fatalf("result=%+v calls=%d", result, persister.calls)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "quantity_on_hand") || strings.Contains(string(raw), "model_artifact") || strings.Contains(string(raw), "predictions") {
		t.Fatalf("unbounded result fields: %s", raw)
	}
}

func TestForecastCommandConfigUsesBoundedDefaultsAndOverrides(t *testing.T) {
	env := map[string]string{
		envDatabaseURL:       "postgres://seshatops@localhost/seshatops_northstar_disposable",
		envForecastPython:    "python",
		envForecastCandidate: "candidate.py",
		envForecastTimeout:   "4s",
		envForecastConfirm:   forecastConfirmation,
	}
	cfg, err := forecastCommandConfigFromEnv(mapLookup(env))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PythonCommand != "python" || cfg.CandidateScript != "candidate.py" || cfg.Timeout != 4*time.Second {
		t.Fatalf("config=%+v", cfg)
	}
}

func TestForecastCommandConfigAllowsOnlyTheExplicitLocalStackDatabase(t *testing.T) {
	env := map[string]string{
		envDatabaseURL:     "postgres://seshatops@postgres/seshatops_northstar_disposable",
		envForecastConfirm: forecastConfirmation,
		envLocalStack:      "true",
	}
	if _, err := forecastCommandConfigFromEnv(mapLookup(env)); err != nil {
		t.Fatal(err)
	}
	delete(env, envLocalStack)
	if _, err := forecastCommandConfigFromEnv(mapLookup(env)); err == nil || !strings.Contains(err.Error(), "disposable Northstar database") {
		t.Fatalf("err=%v", err)
	}
}

func TestForecastCommandConfigRejectsNonPositiveTimeout(t *testing.T) {
	env := map[string]string{
		envDatabaseURL:     "postgres://seshatops@localhost/seshatops_northstar_disposable",
		envForecastTimeout: "0s",
		envForecastConfirm: forecastConfirmation,
	}
	if _, err := forecastCommandConfigFromEnv(mapLookup(env)); err == nil || !strings.Contains(err.Error(), envForecastTimeout) {
		t.Fatalf("err=%v", err)
	}
}

func TestForecastCommandConfigRejectsNonDisposableDatabase(t *testing.T) {
	env := map[string]string{
		envDatabaseURL:     "postgres://seshatops@localhost/seshatops",
		envForecastConfirm: forecastConfirmation,
	}
	if _, err := forecastCommandConfigFromEnv(mapLookup(env)); err == nil || !strings.Contains(err.Error(), "disposable Northstar database") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunFrozenForecastRerunKeepsPredictionIdentity(t *testing.T) {
	python := testPython(t)
	producer := platform.CommandCandidateArtifactProducer{
		Command: python,
		Script:  filepath.Join("..", "..", "forecast_candidate", "stockout_candidate.py"),
	}
	persister := &recordingForecastPersister{}
	first, err := runFrozenForecast(context.Background(), producer, persister)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runFrozenForecast(context.Background(), producer, persister)
	if err != nil {
		t.Fatal(err)
	}
	if first.PredictionID != second.PredictionID || first.DatasetChecksum != second.DatasetChecksum || first.SelectedModelVersion != second.SelectedModelVersion {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	if persister.calls != 2 {
		t.Fatalf("persist calls=%d", persister.calls)
	}
}

func TestRunFrozenForecastFailuresDoNotPersist(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		err  error
	}{
		{name: "unavailable", err: platform.ErrPythonUnavailable},
		{name: "timeout", err: platform.ErrPythonTimeout},
		{name: "malformed", raw: []byte("{"), err: platform.ErrPythonInvalidResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			persister := &recordingForecastPersister{}
			producer := staticForecastProducer{raw: tt.raw, err: tt.err}
			_, err := runFrozenForecast(context.Background(), producer, persister)
			if !errors.Is(err, tt.err) {
				t.Fatalf("err=%v, want %v", err, tt.err)
			}
			if persister.calls != 0 {
				t.Fatalf("persist calls=%d", persister.calls)
			}
		})
	}
}

func TestPythonTelemetryOutcomeIsBounded(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want string
	}{
		{platform.ErrPythonUnavailable, "unavailable"},
		{platform.ErrPythonTimeout, "timeout"},
		{platform.ErrPythonInvalidResponse, "invalid_response"},
		{errors.New("other"), ""},
	} {
		if got := string(pythonTelemetryOutcome(tt.err)); got != tt.want {
			t.Fatalf("outcome(%v)=%q want %q", tt.err, got, tt.want)
		}
	}
}

func TestValidateFrozenM4OutcomeRejectsChangedMetric(t *testing.T) {
	python := testPython(t)
	producer := platform.CommandCandidateArtifactProducer{
		Command: python,
		Script:  filepath.Join("..", "..", "forecast_candidate", "stockout_candidate.py"),
	}
	history, err := forecast.GenerateHistory(forecast.HistorySeed)
	if err != nil {
		t.Fatal(err)
	}
	dataset, err := forecast.BuildDataset(history, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	features, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{
		HistoryChecksum:    frozenM4HistoryBoundaryChecksum,
		ProjectionChecksum: frozenM4ProjectionBoundaryChecksum,
		MaxRecordedAt:      frozenM4MaxRecordedAt,
		AppliedEventCount:  len(history.Observations),
	})
	if err != nil {
		t.Fatal(err)
	}
	artifactJSON, err := producer.Produce(context.Background(), forecast.CandidateInput{Dataset: dataset, DatasetChecksum: forecast.Checksum(dataset), Features: features})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := forecast.DecodeCandidateArtifactJSON(artifactJSON)
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := forecast.EvaluateCandidateArtifact(dataset, features, artifact)
	if err != nil {
		t.Fatal(err)
	}
	evaluation.Splits[0].Candidate.AP = float64Ptr(0.1)
	selection, err := forecast.SelectRuntimePredictor(evaluation)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateFrozenM4Outcome(history, dataset, features, evaluation, selection); err == nil {
		t.Fatal("changed metric passed frozen guard")
	}
}

type staticForecastProducer struct {
	raw []byte
	err error
}

func (p staticForecastProducer) Produce(context.Context, forecast.CandidateInput) ([]byte, error) {
	return p.raw, p.err
}

type recordingForecastPersister struct {
	calls int
}

func (p *recordingForecastPersister) PredictAndPersist(_ context.Context, request platform.ForecastRequest) (platform.PredictionRecord, error) {
	p.calls++
	return platform.PredictionRecord{
		PredictionID:             forecast.PredictionIdentity(request.TenantID, request.ItemID, request.ObservationDate, forecast.HorizonDays, request.Features.DatasetVersion, request.Features.FeatureDefinitionVersion, request.Features.SnapshotID, request.Features.Checksum),
		TenantID:                 request.TenantID,
		ResourceID:               request.ItemID,
		ObservationDate:          request.ObservationDate,
		Status:                   forecast.CandidatePredictionStatusPredicted,
		FeatureSnapshotID:        request.Features.SnapshotID,
		FeatureSnapshotChecksum:  request.Features.Checksum,
		DatasetVersion:           request.Features.DatasetVersion,
		FeatureDefinitionVersion: request.Features.FeatureDefinitionVersion,
	}, nil
}

func testPython(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	t.Skip("Python runtime not installed")
	return ""
}

func float64Ptr(value float64) *float64 { return &value }
