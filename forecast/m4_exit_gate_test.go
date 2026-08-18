package forecast_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/G1DO/seshatops/forecast"
)

func TestM4ExitGateRebuildsAndEvaluatesTheFrozenCandidate(t *testing.T) {
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

	input := forecast.CandidateInput{
		Dataset:         dataset,
		DatasetChecksum: forecast.Checksum(dataset),
		Features:        features,
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	workDir := t.TempDir()
	inputPath := filepath.Join(workDir, "candidate-input.json")
	artifactPath := filepath.Join(workDir, "candidate-artifact.json")
	if err := os.WriteFile(inputPath, inputJSON, 0o600); err != nil {
		t.Fatal(err)
	}

	python, err := findPython()
	if err != nil {
		t.Skipf("Python runtime not installed: %v", err)
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
	evaluation, err := forecast.EvaluateCandidateArtifactJSON(dataset, features, artifactJSON)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := forecast.SelectRuntimePredictor(evaluation)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.PromotionSplit != forecast.SplitTest || evaluation.Outcome == forecast.CandidateOutcomeNoQualifyingBaseline {
		t.Fatalf("invalid M4 outcome: %+v", evaluation)
	}
	var testSplit *forecast.CandidateSplitEvaluation
	for i := range evaluation.Splits {
		if evaluation.Splits[i].Split == forecast.SplitTest {
			testSplit = &evaluation.Splits[i]
			break
		}
	}
	if testSplit == nil || testSplit.BaselineID != forecast.BaselineSeasonalNaive || testSplit.CandidateBeatsBaseline || evaluation.Outcome != forecast.CandidateOutcomeBaseline || selection.Predictor != forecast.RuntimePredictorBaseline || selection.BaselineID != forecast.BaselineSeasonalNaive {
		t.Fatalf("unexpected frozen test selection: split=%+v selection=%+v outcome=%s", testSplit, selection, evaluation.Outcome)
	}
	if len(dataset.Examples) != 3*forecast.LabeledDayCount || len(features.Rows) != len(dataset.Examples) {
		t.Fatalf("unexpected rebuilt shapes: dataset=%d features=%d history=%d", len(dataset.Examples), len(features.Rows), len(history.Observations))
	}

	t.Logf("M4 protocol=%s dataset_checksum=%s feature_snapshot_id=%s feature_snapshot_checksum=%s", forecast.ProtocolID, forecast.Checksum(dataset), features.SnapshotID, features.Checksum)
	for _, split := range evaluation.Splits {
		t.Logf("split=%s candidate=%s baseline_id=%s baseline=%s candidate_beats_baseline=%t reason=%s", split.Split, resultSummary(split.Candidate), split.BaselineID, resultSummaryPtr(split.Baseline), split.CandidateBeatsBaseline, split.ComparisonReason)
	}
	t.Logf("selected_predictor=%s baseline_id=%s model_version=%s code_version=%s outcome=%s", selection.Predictor, selection.BaselineID, selection.ModelVersion, selection.CodeVersion, evaluation.Outcome)
}

func findPython() (string, error) {
	for _, command := range []string{"python", "python3"} {
		if path, err := exec.LookPath(command); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("python or python3 not found")
}

func resultSummary(result forecast.Result) string {
	return fmt.Sprintf("defined=%t n=%d predicted=%d coverage=%.6f ap=%s brier=%s reason=%s", result.Defined, result.N, result.Predicted, result.Coverage, optionalFloat(result.AP), optionalFloat(result.Brier), result.Reason)
}

func resultSummaryPtr(result *forecast.Result) string {
	if result == nil {
		return "none"
	}
	return resultSummary(*result)
}

func optionalFloat(value *float64) string {
	if value == nil {
		return "undefined"
	}
	return fmt.Sprintf("%.12f", *value)
}
