package forecast_test

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/G1DO/seshatops/forecast"
)

func TestBaselinePredictionsMatchFrozenDefinitions(t *testing.T) {
	history := forecast.History{Observations: dense(
		forecast.TenantNS001,
		forecast.ItemFlour,
		"2026-01-05",
		[]int64{8, 8, 8, 0, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8},
	)}
	ds, err := forecast.BuildDataset(history, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	features, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{})
	if err != nil {
		t.Fatal(err)
	}

	seasonal, err := forecast.BaselinePredictions(ds, features, forecast.BaselineSeasonalNaive)
	if err != nil {
		t.Fatal(err)
	}
	if score := predictionScore(seasonal, ds, "2026-01-05"); score != nil {
		t.Fatalf("seasonal-naive early score = %v, want abstention", *score)
	}
	if score := predictionScore(seasonal, ds, "2026-01-12"); score == nil || *score != 1 {
		t.Fatalf("seasonal-naive score = %v, want 1", score)
	}

	movingAverage, err := forecast.BaselinePredictions(ds, features, forecast.BaselineMovingAverage)
	if err != nil {
		t.Fatal(err)
	}
	if score := predictionScore(movingAverage, ds, "2026-01-10"); score != nil {
		t.Fatalf("moving-average early score = %v, want abstention", *score)
	}
	for _, date := range []string{"2026-01-11", "2026-01-12"} {
		score := predictionScore(movingAverage, ds, date)
		if score == nil || math.Abs(*score-(1.0/7.0)) > 1e-12 {
			t.Fatalf("moving-average score on %s = %v, want 1/7", date, score)
		}
	}
}

func TestEvaluateBaselinesIsDeterministicAndSelectsPerSplit(t *testing.T) {
	history := officialHistory(t)
	ds, err := forecast.BuildDataset(history, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	features, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{
		HistoryChecksum: "history-a",
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := forecast.EvaluateBaselines(ds, features)
	if err != nil {
		t.Fatal(err)
	}
	shuffledDataset := ds
	shuffledDataset.Examples = append([]forecast.Example(nil), ds.Examples...)
	for i, j := 0, len(shuffledDataset.Examples)-1; i < j; i, j = i+1, j-1 {
		shuffledDataset.Examples[i], shuffledDataset.Examples[j] = shuffledDataset.Examples[j], shuffledDataset.Examples[i]
	}
	shuffledFeatures := features
	shuffledFeatures.Rows = append([]forecast.FeatureRow(nil), features.Rows...)
	for i, j := 0, len(shuffledFeatures.Rows)-1; i < j; i, j = i+1, j-1 {
		shuffledFeatures.Rows[i], shuffledFeatures.Rows[j] = shuffledFeatures.Rows[j], shuffledFeatures.Rows[i]
	}
	second, err := forecast.EvaluateBaselines(shuffledDataset, shuffledFeatures)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("baseline evaluation changed when inputs were reordered")
	}

	if first.EvaluationProtocolVersion != forecast.ProtocolID ||
		first.DatasetVersion != forecast.ProtocolID ||
		first.DatasetChecksum != forecast.Checksum(ds) ||
		first.FeatureDefinitionVersion != forecast.FeatureDefinitionVersion ||
		first.FeatureSnapshotID != features.SnapshotID ||
		first.FeatureSnapshotChecksum != features.Checksum ||
		first.CodeVersion != forecast.EvaluationCodeVersion {
		t.Fatalf("evaluation metadata = %+v", first)
	}
	if len(first.Baselines) != 2 || len(first.Selections) != 3 {
		t.Fatalf("baseline output shape = baselines=%d selections=%d", len(first.Baselines), len(first.Selections))
	}
	for _, run := range first.Baselines {
		if len(run.Predictions) != len(ds.Examples) || len(run.Results) != 3 {
			t.Fatalf("run %s shape = predictions=%d results=%d", run.ID, len(run.Predictions), len(run.Results))
		}
		for i, prediction := range run.Predictions {
			if prediction.RowID != ds.Examples[i].RowID {
				t.Fatalf("run %s prediction %d row_id=%s want=%s", run.ID, i, prediction.RowID, ds.Examples[i].RowID)
			}
		}
	}
	for _, selection := range first.Selections {
		if selection.Found && selection.BaselineID != forecast.BaselineMovingAverage && selection.BaselineID != forecast.BaselineSeasonalNaive {
			t.Fatalf("invalid selection %+v", selection)
		}
		if selection.Found && selection.Reason != "qualifying baseline" {
			t.Fatalf("found selection reason = %q", selection.Reason)
		}
		if !selection.Found && selection.Reason != "no qualifying baseline" {
			t.Fatalf("missing selection reason = %q", selection.Reason)
		}
	}
}

func TestBaselinePredictionsDoNotUseFutureRows(t *testing.T) {
	history := forecast.History{Observations: dense(
		forecast.TenantNS001,
		forecast.ItemFlour,
		"2026-01-05",
		[]int64{8, 8, 8, 0, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8},
	)}
	baseDataset, err := forecast.BuildDataset(history, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	baseFeatures, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{})
	if err != nil {
		t.Fatal(err)
	}

	mutatedHistory := forecast.History{Observations: append([]forecast.Observation(nil), history.Observations...)}
	for i := range mutatedHistory.Observations {
		if mutatedHistory.Observations[i].AsOfDate == "2026-01-19" {
			mutatedHistory.Observations[i].QuantityOnHand = 0
		}
	}
	mutatedDataset, err := forecast.BuildDataset(mutatedHistory, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	mutatedFeatures, err := forecast.BuildFeatureSnapshot(mutatedHistory, forecast.TenantNS001, forecast.SourceBoundary{})
	if err != nil {
		t.Fatal(err)
	}

	for _, baselineID := range []string{forecast.BaselineSeasonalNaive, forecast.BaselineMovingAverage} {
		base, err := forecast.BaselinePredictions(baseDataset, baseFeatures, baselineID)
		if err != nil {
			t.Fatal(err)
		}
		mutated, err := forecast.BaselinePredictions(mutatedDataset, mutatedFeatures, baselineID)
		if err != nil {
			t.Fatal(err)
		}
		baseScore := predictionScore(base, baseDataset, "2026-01-12")
		mutatedScore := predictionScore(mutated, mutatedDataset, "2026-01-12")
		if (baseScore == nil) != (mutatedScore == nil) ||
			(baseScore != nil && math.Abs(*baseScore-*mutatedScore) > 1e-12) {
			t.Fatalf("future mutation changed %s prediction: base=%v mutated=%v", baselineID, baseScore, mutatedScore)
		}
	}
}

func TestEvaluateBaselinesRejectsIncompatibleOrInsufficientInputs(t *testing.T) {
	history := officialHistory(t)
	ds, err := forecast.BuildDataset(history, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	features, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		mutate    func(forecast.Dataset, forecast.FeatureSnapshot) (forecast.Dataset, forecast.FeatureSnapshot)
		wantError string
	}{
		{
			name: "stale snapshot",
			mutate: func(ds forecast.Dataset, _ forecast.FeatureSnapshot) (forecast.Dataset, forecast.FeatureSnapshot) {
				return ds, forecast.NewNonCompleteFeatureSnapshot(forecast.SnapshotStatusStale, forecast.TenantNS001, forecast.SourceBoundary{}, "pending source")
			},
			wantError: "status \"stale\"",
		},
		{
			name: "feature definition mismatch",
			mutate: func(ds forecast.Dataset, features forecast.FeatureSnapshot) (forecast.Dataset, forecast.FeatureSnapshot) {
				features.FeatureDefinitionVersion = "other-feature-v1"
				return ds, features
			},
			wantError: "feature definition version",
		},
		{
			name: "tenant mismatch",
			mutate: func(ds forecast.Dataset, features forecast.FeatureSnapshot) (forecast.Dataset, forecast.FeatureSnapshot) {
				features.TenantID = forecast.TenantNS002
				return ds, features
			},
			wantError: "feature tenant",
		},
		{
			name: "missing feature row",
			mutate: func(ds forecast.Dataset, features forecast.FeatureSnapshot) (forecast.Dataset, forecast.FeatureSnapshot) {
				features.Rows = features.Rows[:len(features.Rows)-1]
				return ds, features
			},
			wantError: "feature snapshot checksum",
		},
		{
			name: "non chronological dataset",
			mutate: func(ds forecast.Dataset, features forecast.FeatureSnapshot) (forecast.Dataset, forecast.FeatureSnapshot) {
				ds.Examples = append([]forecast.Example(nil), ds.Examples...)
				ds.Examples[0].Split = forecast.SplitValidation
				return ds, features
			},
			wantError: "non-chronological",
		},
		{
			name: "frozen split assignment",
			mutate: func(ds forecast.Dataset, features forecast.FeatureSnapshot) (forecast.Dataset, forecast.FeatureSnapshot) {
				ds.Examples = append([]forecast.Example(nil), ds.Examples...)
				for i := range ds.Examples {
					ds.Examples[i].Split = forecast.SplitTest
				}
				return ds, features
			},
			wantError: "frozen split assignment",
		},
		{
			name: "unsupported baseline",
			mutate: func(ds forecast.Dataset, features forecast.FeatureSnapshot) (forecast.Dataset, forecast.FeatureSnapshot) {
				return ds, features
			},
			wantError: "unsupported baseline",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutatedDS, mutatedFeatures := tt.mutate(ds, features)
			var err error
			if tt.name == "unsupported baseline" {
				_, err = forecast.BaselinePredictions(mutatedDS, mutatedFeatures, "random_forest")
			} else {
				_, err = forecast.EvaluateBaselines(mutatedDS, mutatedFeatures)
			}
			if !errors.Is(err, forecast.ErrInvalidInput) || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("err=%v, want invalid input containing %q", err, tt.wantError)
			}
		})
	}

	shortHistory := forecast.History{Observations: dense(
		forecast.TenantNS001,
		forecast.ItemFlour,
		"2026-01-05",
		[]int64{8, 8, 8, 8, 8, 8, 8},
	)}
	shortDataset, err := forecast.BuildDataset(shortHistory, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	shortFeatures, err := forecast.BuildFeatureSnapshot(shortHistory, forecast.TenantNS001, forecast.SourceBoundary{})
	if err != nil {
		t.Fatal(err)
	}
	if shortFeatures.Status != forecast.SnapshotStatusInsufficient {
		t.Fatalf("short feature status = %s", shortFeatures.Status)
	}
	if _, err := forecast.EvaluateBaselines(shortDataset, shortFeatures); !errors.Is(err, forecast.ErrInvalidInput) || !strings.Contains(err.Error(), "status \"insufficient\"") {
		t.Fatalf("short evaluation err=%v", err)
	}
}

func TestEvaluateBaselinesReportsDegenerateOutcomes(t *testing.T) {
	history := forecast.History{Observations: dense(
		forecast.TenantNS001,
		forecast.ItemSalt,
		"2026-01-05",
		[]int64{8, 8, 8, 8, 8, 8, 8, 8, 8},
	)}
	ds, err := forecast.BuildDataset(history, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	features, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{})
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := forecast.EvaluateBaselines(ds, features)
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range evaluation.Baselines {
		if run.Results[0].Defined || run.Results[0].Reason != "degenerate class" {
			t.Fatalf("run %s result = %+v", run.ID, run.Results[0])
		}
	}
	for _, selection := range evaluation.Selections {
		if selection.Found {
			t.Fatalf("degenerate selection = %+v", selection)
		}
	}
}

func predictionScore(predictions []forecast.Prediction, ds forecast.Dataset, date string) *float64 {
	for _, example := range ds.Examples {
		if example.AsOfDate != date {
			continue
		}
		for _, prediction := range predictions {
			if prediction.RowID == example.RowID {
				return prediction.Score
			}
		}
	}
	return nil
}
