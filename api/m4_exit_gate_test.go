package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/G1DO/seshatops/api"
	"github.com/G1DO/seshatops/forecast"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/platform"
)

func TestM4ExitGatePersistsSelectedForecastForAuthorizedRead(t *testing.T) {
	db := openTestDB(t)
	history, err := forecast.GenerateHistory(forecast.HistorySeed)
	if err != nil {
		t.Fatal(err)
	}
	dataset, err := forecast.BuildDataset(history, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	features, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{
		HistoryChecksum:    "m4-official-history-v1",
		ProjectionChecksum: "m4-official-projection-v1",
		MaxRecordedAt:      "2026-04-26T23:59:59Z",
		AppliedEventCount:  len(history.Observations),
	})
	if err != nil {
		t.Fatal(err)
	}
	artifact := runM4Candidate(t, dataset, features)
	evaluation, err := forecast.EvaluateCandidateArtifact(dataset, features, artifact)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := forecast.SelectRuntimePredictor(evaluation)
	if err != nil {
		t.Fatal(err)
	}
	if selection.Predictor != forecast.RuntimePredictorBaseline || selection.BaselineID != forecast.BaselineSeasonalNaive {
		t.Fatalf("selection=%+v evaluation=%+v", selection, evaluation)
	}

	service := platform.NewForecastService(db, nil)
	record, err := service.PredictAndPersist(context.Background(), platform.ForecastRequest{
		TenantID:          forecast.TenantNS001,
		ItemID:            forecast.ItemFlour,
		ObservationDate:   "2026-04-19",
		CorrelationID:     "018f5d78-6e64-4f5f-bd16-8e9f7c4a2101",
		Features:          features,
		Evaluation:        evaluation,
		CandidateArtifact: &artifact,
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Predictor != forecast.RuntimePredictorBaseline || record.ModelVersion != forecast.BaselineSeasonalNaive || record.Status != forecast.CandidatePredictionStatusPredicted {
		t.Fatalf("record=%+v", record)
	}

	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := getWithSession(t, ts.URL+forecastPredictionPath(forecast.TenantNS001, forecast.ItemFlour), sess, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var snapshot api.ForecastPredictionSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.TenantID != forecast.TenantNS001 || snapshot.ResourceID != forecast.ItemFlour || snapshot.ObservationDate != "2026-04-19" || snapshot.Status != forecast.CandidatePredictionStatusPredicted || snapshot.ForecastHorizonDays != forecast.HorizonDays {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	// The current Event Spine source does not reproduce the dense official M4
	// fixture, so the read surface must expose unavailable freshness explicitly.
	if snapshot.StockoutRisk == nil || snapshot.Uncertainty == nil || snapshot.Freshness.Status != "unavailable" || snapshot.Lineage.Predictor != forecast.RuntimePredictorBaseline || snapshot.Lineage.ModelVersion != forecast.BaselineSeasonalNaive || snapshot.Lineage.FeatureSnapshotID != features.SnapshotID || snapshot.Lineage.FeatureSnapshotChecksum != features.Checksum {
		t.Fatalf("snapshot lineage=%+v freshness=%+v", snapshot.Lineage, snapshot.Freshness)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "quantity_on_hand") || strings.Contains(string(raw), "model_artifact") {
		t.Fatalf("unsafe prediction response: %s", raw)
	}

	foreign := getWithSession(t, ts.URL+forecastPredictionPath(identity.TenantNS002UUID, forecast.ItemFlour), sess, nil)
	assertForbiddenNoProjection(t, foreign)
}

func runM4Candidate(t *testing.T, dataset forecast.Dataset, features forecast.FeatureSnapshot) forecast.CandidateArtifact {
	t.Helper()
	inputJSON, err := json.Marshal(forecast.CandidateInput{
		Dataset:         dataset,
		DatasetChecksum: forecast.Checksum(dataset),
		Features:        features,
	})
	if err != nil {
		t.Fatal(err)
	}
	python, err := findM4Python()
	if err != nil {
		t.Fatalf("M4 exit gate requires Python: %v", err)
	}
	workDir := t.TempDir()
	inputPath := filepath.Join(workDir, "candidate-input.json")
	artifactPath := filepath.Join(workDir, "candidate-artifact.json")
	if err := os.WriteFile(inputPath, inputJSON, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	commandContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(commandContext, python,
		filepath.Join("..", "forecast_candidate", "stockout_candidate.py"),
		"--input", inputPath,
		"--output", artifactPath,
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("candidate command failed: %v; stdout=%s; stderr=%s", err, stdout.String(), stderr.String())
	}
	artifactJSON, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	var artifact forecast.CandidateArtifact
	if err := json.Unmarshal(artifactJSON, &artifact); err != nil {
		t.Fatalf("decode candidate artifact: %v", err)
	}
	return artifact
}

func findM4Python() (string, error) {
	for _, command := range []string{"python", "python3"} {
		if path, err := exec.LookPath(command); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("python or python3 not found")
}
