package forecast_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
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
		t.Fatalf("M4 exit gate requires Python: %v", err)
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
	if forecast.Checksum(dataset) != "b29e795cbacd0a40ee2b7c15c0d52200a867fa24554f50b7f77989302c8e116a" || features.SnapshotID != "2d4121bb6804b9dbe9ee0d3d68989e9271e348a3c3ddf0727df5d53428035fc6" || features.Checksum != "808980035fd123badb12df34224a8b510683d93dffbc1d8fde3df28590be4d78" {
		t.Fatalf("frozen lineage changed: dataset=%s snapshot_id=%s snapshot_checksum=%s", forecast.Checksum(dataset), features.SnapshotID, features.Checksum)
	}
	if len(history.Observations) != 4*forecast.HistoryDayCount || len(dataset.Examples) != 3*forecast.LabeledDayCount || len(features.Rows) != len(dataset.Examples) {
		t.Fatalf("unexpected rebuilt shapes: history=%d dataset=%d features=%d", len(history.Observations), len(dataset.Examples), len(features.Rows))
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
	wantSplits := map[string]struct {
		baselineID                    string
		candidateAP, candidateBrier   float64
		baselineAP, baselineBrier     float64
		candidateN, baselinePredicted int
	}{
		forecast.SplitTrain:      {forecast.BaselineMovingAverage, 0.957489696104, 0.036397378241, 0.962987425654, 0.170493197279, 210, 192},
		forecast.SplitValidation: {forecast.BaselineSeasonalNaive, 0.963827614379, 0.126833502348, 1, 0, 42, 42},
		forecast.SplitTest:       {forecast.BaselineSeasonalNaive, 0.953728663154, 0.126833502348, 1, 0, 63, 63},
	}
	for _, split := range evaluation.Splits {
		want, ok := wantSplits[split.Split]
		if !ok || split.Baseline == nil || split.BaselineID != want.baselineID || split.CandidateBeatsBaseline || split.ComparisonReason != "candidate average precision does not beat baseline" {
			t.Fatalf("unexpected split outcome: %+v", split)
		}
		if split.Candidate.N != want.candidateN || split.Candidate.Predicted != want.candidateN || math.Abs(split.Candidate.Coverage-1) > 1e-12 || split.Baseline.Predicted != want.baselinePredicted || !closeFloat(split.Candidate.AP, want.candidateAP) || !closeFloat(split.Candidate.Brier, want.candidateBrier) || !closeFloat(split.Baseline.AP, want.baselineAP) || !closeFloat(split.Baseline.Brier, want.baselineBrier) {
			t.Fatalf("unexpected frozen metrics for %s: candidate=%+v baseline=%+v", split.Split, split.Candidate, *split.Baseline)
		}
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

func closeFloat(value *float64, want float64) bool {
	return value != nil && math.Abs(*value-want) <= 1e-12
}
