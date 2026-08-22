package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/G1DO/seshatops/forecast"
	"github.com/G1DO/seshatops/platform"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	envForecastPython        = "SESHATOPS_FORECAST_PYTHON"
	envForecastCandidate     = "SESHATOPS_FORECAST_CANDIDATE"
	envForecastTimeout       = "SESHATOPS_FORECAST_TIMEOUT"
	envForecastConfirm       = "SESHATOPS_FORECAST_CONFIRM"
	envLocalStack            = "SESHATOPS_LOCAL_STACK"
	defaultForecastPython    = "python3"
	defaultForecastCandidate = "forecast_candidate/stockout_candidate.py"
	defaultForecastTimeout   = 30 * time.Second
	forecastConfirmation     = "I_UNDERSTAND_FROZEN_M4_FORECAST_WRITE"

	frozenM4HistoryBoundaryChecksum    = "m4-official-history-v1"
	frozenM4ProjectionBoundaryChecksum = "m4-official-projection-v1"
	frozenM4MaxRecordedAt              = "2026-04-26T23:59:59Z"
	frozenM4DatasetChecksum            = "b29e795cbacd0a40ee2b7c15c0d52200a867fa24554f50b7f77989302c8e116a"
	frozenM4FeatureSnapshotID          = "2d4121bb6804b9dbe9ee0d3d68989e9271e348a3c3ddf0727df5d53428035fc6"
	frozenM4FeatureSnapshotChecksum    = "808980035fd123badb12df34224a8b510683d93dffbc1d8fde3df28590be4d78"
	frozenM4ObservationDate            = "2026-04-19"
	frozenM4CorrelationID              = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2101"
)

type forecastCommandConfig struct {
	DatabaseURL     string
	PythonCommand   string
	CandidateScript string
	Timeout         time.Duration
}

func loadForecastCommandConfig() (forecastCommandConfig, error) {
	return forecastCommandConfigFromEnv(os.LookupEnv)
}

func forecastCommandConfigFromEnv(lookup func(string) (string, bool)) (forecastCommandConfig, error) {
	databaseURL, err := requiredEnv(lookup, envDatabaseURL)
	if err != nil {
		return forecastCommandConfig{}, err
	}
	if err := validateDatabaseURL(databaseURL); err != nil {
		return forecastCommandConfig{}, fmt.Errorf("%s: %w", envDatabaseURL, err)
	}
	if err := validateDisposableNorthstarURL(databaseURL); err != nil && !localStackDatabaseAllowed(databaseURL, lookup) {
		return forecastCommandConfig{}, fmt.Errorf("%s: forecast writes require the disposable Northstar database: %w", envDatabaseURL, err)
	}
	confirmation, ok := lookup(envForecastConfirm)
	if !ok || strings.TrimSpace(confirmation) != forecastConfirmation {
		return forecastCommandConfig{}, fmt.Errorf("%s must equal %s", envForecastConfirm, forecastConfirmation)
	}
	pythonCommand := defaultForecastPython
	if raw, ok := lookup(envForecastPython); ok {
		pythonCommand = strings.TrimSpace(raw)
		if pythonCommand == "" {
			return forecastCommandConfig{}, fmt.Errorf("%s must not be blank", envForecastPython)
		}
	}
	candidateScript := defaultForecastCandidate
	if raw, ok := lookup(envForecastCandidate); ok {
		candidateScript = strings.TrimSpace(raw)
		if candidateScript == "" {
			return forecastCommandConfig{}, fmt.Errorf("%s must not be blank", envForecastCandidate)
		}
	}
	timeout := defaultForecastTimeout
	if raw, ok := lookup(envForecastTimeout); ok && strings.TrimSpace(raw) != "" {
		timeout, err = time.ParseDuration(raw)
		if err != nil || timeout <= 0 {
			return forecastCommandConfig{}, fmt.Errorf("%s must be a positive duration", envForecastTimeout)
		}
	}
	return forecastCommandConfig{
		DatabaseURL:     databaseURL,
		PythonCommand:   pythonCommand,
		CandidateScript: candidateScript,
		Timeout:         timeout,
	}, nil
}

func localStackDatabaseAllowed(raw string, lookup func(string) (string, bool)) bool {
	// The Compose-only opt-in permits its exact internal service name while the
	// normal forecast and reset paths remain localhost-only.
	if value, ok := lookup(envLocalStack); !ok || strings.TrimSpace(value) != "true" {
		return false
	}
	if err := validateDatabaseURL(raw); err != nil {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || strings.ToLower(u.Hostname()) != "postgres" {
		return false
	}
	return strings.TrimPrefix(u.Path, "/") == northstarDisposableDatabase
}

type forecastPersister interface {
	PredictAndPersist(context.Context, platform.ForecastRequest) (platform.PredictionRecord, error)
}

// runForecastCommand rebuilds the frozen M4 inputs, evaluates the candidate in
// Go, derives the runtime choice from the frozen outcome, and persists one
// tenant-scoped advisory prediction.
func runForecastCommand(ctx context.Context, cfg forecastCommandConfig, out io.Writer) error {
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database failed; check %s", envDatabaseURL)
	}
	defer db.Close()

	commandCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	if err := db.PingContext(commandCtx); err != nil {
		return fmt.Errorf("database ping failed; check %s", envDatabaseURL)
	}
	if err := platform.MigrateForecast(commandCtx, db); err != nil {
		return fmt.Errorf("platform migration failed")
	}

	producer := platform.CommandCandidateArtifactProducer{
		Command: cfg.PythonCommand,
		Script:  cfg.CandidateScript,
		Timeout: cfg.Timeout,
	}
	service := platform.NewForecastService(db, nil)
	result, err := runFrozenForecast(commandCtx, producer, service)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(result)
}

func runFrozenForecast(ctx context.Context, producer platform.CandidateArtifactProducer, persister forecastPersister) (forecastCommandResult, error) {
	if producer == nil {
		return forecastCommandResult{}, fmt.Errorf("produce frozen candidate artifact: %w", platform.ErrPythonUnavailable)
	}
	history, err := forecast.GenerateHistory(forecast.HistorySeed)
	if err != nil {
		return forecastCommandResult{}, fmt.Errorf("build frozen history: %w", err)
	}
	dataset, err := forecast.BuildDataset(history, forecast.TenantNS001)
	if err != nil {
		return forecastCommandResult{}, fmt.Errorf("build frozen dataset: %w", err)
	}
	features, err := forecast.BuildFeatureSnapshot(history, forecast.TenantNS001, forecast.SourceBoundary{
		HistoryChecksum:    frozenM4HistoryBoundaryChecksum,
		ProjectionChecksum: frozenM4ProjectionBoundaryChecksum,
		MaxRecordedAt:      frozenM4MaxRecordedAt,
		AppliedEventCount:  len(history.Observations),
	})
	if err != nil {
		return forecastCommandResult{}, fmt.Errorf("build frozen feature snapshot: %w", err)
	}
	input := forecast.CandidateInput{
		Dataset:         dataset,
		DatasetChecksum: forecast.Checksum(dataset),
		Features:        features,
	}
	artifactJSON, err := producer.Produce(ctx, input)
	if err != nil {
		return forecastCommandResult{}, fmt.Errorf("produce frozen candidate artifact: %w", err)
	}
	artifact, err := forecast.DecodeCandidateArtifactJSON(artifactJSON)
	if err != nil {
		return forecastCommandResult{}, fmt.Errorf("%w: decode candidate artifact: %v", platform.ErrPythonInvalidResponse, err)
	}
	evaluation, err := forecast.EvaluateCandidateArtifact(dataset, features, artifact)
	if err != nil {
		return forecastCommandResult{}, fmt.Errorf("%w: evaluate candidate artifact: %v", platform.ErrPythonInvalidResponse, err)
	}
	selection, err := forecast.SelectRuntimePredictor(evaluation)
	if err != nil {
		return forecastCommandResult{}, fmt.Errorf("select runtime predictor: %w", err)
	}
	if err := validateFrozenM4Outcome(history, dataset, features, evaluation, selection); err != nil {
		return forecastCommandResult{}, err
	}
	if persister == nil {
		return forecastCommandResult{}, fmt.Errorf("persist frozen forecast: nil service")
	}
	record, err := persister.PredictAndPersist(ctx, platform.ForecastRequest{
		TenantID:          forecast.TenantNS001,
		ItemID:            forecast.ItemFlour,
		ObservationDate:   frozenM4ObservationDate,
		CorrelationID:     frozenM4CorrelationID,
		Features:          features,
		Evaluation:        evaluation,
		CandidateArtifact: &artifact,
	})
	if err != nil {
		return forecastCommandResult{}, fmt.Errorf("persist frozen forecast: %w", err)
	}
	return buildForecastCommandResult(history, features, evaluation, selection, record), nil
}

type forecastCommandResult struct {
	HistorySeed               string                  `json:"history_seed"`
	EvaluationProtocolVersion string                  `json:"evaluation_protocol_version"`
	DatasetVersion            string                  `json:"dataset_version"`
	DatasetChecksum           string                  `json:"dataset_checksum"`
	FeatureDefinitionVersion  string                  `json:"feature_definition_version"`
	FeatureSnapshotID         string                  `json:"feature_snapshot_id"`
	FeatureSnapshotChecksum   string                  `json:"feature_snapshot_checksum"`
	SourceBoundary            forecast.SourceBoundary `json:"source_boundary"`
	EvaluationOutcome         string                  `json:"evaluation_outcome"`
	PromotionEligible         bool                    `json:"promotion_eligible"`
	SelectedPredictor         string                  `json:"selected_predictor"`
	SelectedModelVersion      string                  `json:"selected_model_version"`
	SelectedCodeVersion       string                  `json:"selected_code_version"`
	Splits                    []forecastSplitResult   `json:"splits"`
	TenantID                  string                  `json:"tenant_id"`
	ResourceID                string                  `json:"resource_id"`
	ObservationDate           string                  `json:"observation_date"`
	PredictionID              string                  `json:"prediction_id"`
	PredictionStatus          string                  `json:"prediction_status"`
	Limitations               []string                `json:"limitations"`
}

type forecastSplitResult struct {
	Split                  string   `json:"split"`
	BaselineID             string   `json:"baseline_id"`
	CandidateN             int      `json:"candidate_n"`
	CandidatePredicted     int      `json:"candidate_predicted"`
	CandidateCoverage      float64  `json:"candidate_coverage"`
	CandidateAP            *float64 `json:"candidate_average_precision,omitempty"`
	CandidateBrier         *float64 `json:"candidate_brier,omitempty"`
	BaselineN              int      `json:"baseline_n"`
	BaselinePredicted      int      `json:"baseline_predicted"`
	BaselineCoverage       float64  `json:"baseline_coverage"`
	BaselineAP             *float64 `json:"baseline_average_precision,omitempty"`
	BaselineBrier          *float64 `json:"baseline_brier,omitempty"`
	CandidateBeatsBaseline bool     `json:"candidate_beats_baseline"`
	ComparisonReason       string   `json:"comparison_reason"`
}

func buildForecastCommandResult(history forecast.History, features forecast.FeatureSnapshot, evaluation forecast.CandidateEvaluation, selection forecast.RuntimeSelection, record platform.PredictionRecord) forecastCommandResult {
	splits := make([]forecastSplitResult, 0, len(evaluation.Splits))
	for _, split := range evaluation.Splits {
		result := forecastSplitResult{
			Split:                  split.Split,
			BaselineID:             split.BaselineID,
			CandidateN:             split.Candidate.N,
			CandidatePredicted:     split.Candidate.Predicted,
			CandidateCoverage:      split.Candidate.Coverage,
			CandidateAP:            split.Candidate.AP,
			CandidateBrier:         split.Candidate.Brier,
			CandidateBeatsBaseline: split.CandidateBeatsBaseline,
			ComparisonReason:       split.ComparisonReason,
		}
		if split.Baseline != nil {
			result.BaselineN = split.Baseline.N
			result.BaselinePredicted = split.Baseline.Predicted
			result.BaselineCoverage = split.Baseline.Coverage
			result.BaselineAP = split.Baseline.AP
			result.BaselineBrier = split.Baseline.Brier
		}
		splits = append(splits, result)
	}
	return forecastCommandResult{
		HistorySeed:               history.Seed,
		EvaluationProtocolVersion: evaluation.EvaluationProtocolVersion,
		DatasetVersion:            evaluation.DatasetVersion,
		DatasetChecksum:           evaluation.DatasetChecksum,
		FeatureDefinitionVersion:  features.FeatureDefinitionVersion,
		FeatureSnapshotID:         features.SnapshotID,
		FeatureSnapshotChecksum:   features.Checksum,
		SourceBoundary:            features.SourceBoundary,
		EvaluationOutcome:         evaluation.Outcome,
		PromotionEligible:         evaluation.PromotionEligible,
		SelectedPredictor:         selection.Predictor,
		SelectedModelVersion:      selection.ModelVersion,
		SelectedCodeVersion:       selection.CodeVersion,
		Splits:                    splits,
		TenantID:                  record.TenantID,
		ResourceID:                record.ResourceID,
		ObservationDate:           record.ObservationDate,
		PredictionID:              record.PredictionID,
		PredictionStatus:          record.Status,
		Limitations: []string{
			"the persisted result uses the frozen M4 synthetic source boundary",
			"the authorized read surface must report stale or unavailable freshness when live Event Spine history differs",
		},
	}
}

func validateFrozenM4Outcome(history forecast.History, dataset forecast.Dataset, features forecast.FeatureSnapshot, evaluation forecast.CandidateEvaluation, selection forecast.RuntimeSelection) error {
	if history.Seed != forecast.HistorySeed || len(history.Observations) != 4*forecast.HistoryDayCount {
		return fmt.Errorf("frozen M4 history changed: seed=%q observations=%d", history.Seed, len(history.Observations))
	}
	if forecast.Checksum(dataset) != frozenM4DatasetChecksum || len(dataset.Examples) != 3*forecast.LabeledDayCount {
		return fmt.Errorf("frozen M4 dataset changed: checksum=%s examples=%d", forecast.Checksum(dataset), len(dataset.Examples))
	}
	wantBoundary := forecast.SourceBoundary{
		HistoryChecksum:    frozenM4HistoryBoundaryChecksum,
		ProjectionChecksum: frozenM4ProjectionBoundaryChecksum,
		MaxRecordedAt:      frozenM4MaxRecordedAt,
		AppliedEventCount:  len(history.Observations),
	}
	if features.SourceBoundary != wantBoundary || features.SnapshotID != frozenM4FeatureSnapshotID || features.Checksum != frozenM4FeatureSnapshotChecksum || len(features.Rows) != len(dataset.Examples) {
		return fmt.Errorf("frozen M4 feature snapshot changed: id=%s checksum=%s rows=%d", features.SnapshotID, features.Checksum, len(features.Rows))
	}
	if evaluation.ArtifactVersion != forecast.CandidateArtifactVersion || evaluation.ModelVersion != forecast.CandidateModelVersion || evaluation.CodeVersion != forecast.CandidateCodeVersion || evaluation.EvaluationProtocolVersion != forecast.ProtocolID || evaluation.DatasetVersion != forecast.ProtocolID || evaluation.DatasetChecksum != frozenM4DatasetChecksum || evaluation.FeatureDefinitionVersion != forecast.FeatureDefinitionVersion || evaluation.FeatureSnapshotID != frozenM4FeatureSnapshotID || evaluation.FeatureSnapshotChecksum != frozenM4FeatureSnapshotChecksum || evaluation.BaselineEvaluation.CodeVersion != forecast.EvaluationCodeVersion || evaluation.PromotionSplit != forecast.SplitTest || evaluation.Outcome != forecast.CandidateOutcomeBaseline || evaluation.PromotionEligible || evaluation.Reason != "candidate average precision does not beat baseline" {
		return fmt.Errorf("frozen M4 evaluation changed: outcome=%s promotion_eligible=%t reason=%q", evaluation.Outcome, evaluation.PromotionEligible, evaluation.Reason)
	}
	if selection.Predictor != forecast.RuntimePredictorBaseline || selection.BaselineID != forecast.BaselineSeasonalNaive || selection.ModelVersion != forecast.BaselineSeasonalNaive || selection.CodeVersion != forecast.EvaluationCodeVersion {
		return fmt.Errorf("frozen M4 runtime selection changed: %+v", selection)
	}
	want := map[string]struct {
		baselineID                                             string
		candidateN, candidatePredicted, baselinePredicted      int
		candidateAP, candidateBrier, baselineAP, baselineBrier float64
	}{
		forecast.SplitTrain:      {forecast.BaselineMovingAverage, 210, 210, 192, 0.957489696104, 0.036397378241, 0.962987425654, 0.170493197279},
		forecast.SplitValidation: {forecast.BaselineSeasonalNaive, 42, 42, 42, 0.963827614379, 0.126833502348, 1, 0},
		forecast.SplitTest:       {forecast.BaselineSeasonalNaive, 63, 63, 63, 0.953728663154, 0.126833502348, 1, 0},
	}
	for _, split := range evaluation.Splits {
		wantSplit, ok := want[split.Split]
		if !ok || split.Baseline == nil || split.BaselineID != wantSplit.baselineID || split.CandidateBeatsBaseline || split.ComparisonReason != "candidate average precision does not beat baseline" || split.Candidate.N != wantSplit.candidateN || split.Candidate.Predicted != wantSplit.candidatePredicted || split.Candidate.Abstained != 0 || split.Candidate.Coverage != 1 || !closeFrozenMetric(split.Candidate.AP, wantSplit.candidateAP) || !closeFrozenMetric(split.Candidate.Brier, wantSplit.candidateBrier) || split.Baseline.Predicted != wantSplit.baselinePredicted || !closeFrozenMetric(split.Baseline.AP, wantSplit.baselineAP) || !closeFrozenMetric(split.Baseline.Brier, wantSplit.baselineBrier) {
			return fmt.Errorf("frozen M4 split changed: %+v", split)
		}
	}
	if len(evaluation.Splits) != len(want) {
		return fmt.Errorf("frozen M4 split count changed: %d", len(evaluation.Splits))
	}
	return nil
}

func closeFrozenMetric(value *float64, want float64) bool {
	return value != nil && absFrozen(value, want) <= 1e-12
}

func absFrozen(value *float64, want float64) float64 {
	if *value >= want {
		return *value - want
	}
	return want - *value
}
