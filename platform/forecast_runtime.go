package platform

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/G1DO/seshatops/forecast"
)

const (
	defaultForecastTimeout   = 5 * time.Second
	defaultMaxForecastOutput = 1 << 20
)

// CandidateInvoker is the only runtime boundary that can call the learned
// predictor. Its request and response are both typed by package forecast.
type CandidateInvoker interface {
	Invoke(ctx context.Context, request forecast.RuntimeRequest) (forecast.RuntimeResponse, error)
}

// CommandCandidateInvoker invokes a short-lived Python process over stdin and
// stdout. It does not use a shell and passes only a minimal environment.
type CommandCandidateInvoker struct {
	Command        string
	Args           []string
	Timeout        time.Duration
	MaxOutputBytes int
}

// Invoke implements CandidateInvoker with a deadline, bounded output, strict
// JSON decoding, and response validation against the request lineage.
func (i CommandCandidateInvoker) Invoke(ctx context.Context, request forecast.RuntimeRequest) (forecast.RuntimeResponse, error) {
	if err := forecast.ValidateRuntimeRequest(request); err != nil {
		return forecast.RuntimeResponse{}, err
	}
	if i.Command == "" {
		return forecast.RuntimeResponse{}, ErrPythonUnavailable
	}
	timeout := i.Timeout
	if timeout <= 0 {
		timeout = defaultForecastTimeout
	}
	maxOutput := i.MaxOutputBytes
	if maxOutput <= 0 {
		maxOutput = defaultMaxForecastOutput
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rawRequest, err := json.Marshal(request)
	if err != nil {
		return forecast.RuntimeResponse{}, fmt.Errorf("platform: marshal forecast request: %w", err)
	}
	cmd := exec.CommandContext(callCtx, i.Command, i.Args...)
	cmd.Stdin = bytes.NewReader(rawRequest)
	output := limitedBuffer{limit: maxOutput}
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	cmd.Env = minimalPythonEnvironment()
	if err := cmd.Run(); err != nil {
		if errors.Is(output.err, errOutputLimit) {
			return forecast.RuntimeResponse{}, fmt.Errorf("%w: output exceeds limit", ErrPythonInvalidResponse)
		}
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			return forecast.RuntimeResponse{}, ErrPythonTimeout
		}
		if ctx.Err() != nil {
			return forecast.RuntimeResponse{}, ctx.Err()
		}
		return forecast.RuntimeResponse{}, ErrPythonUnavailable
	}
	if errors.Is(output.err, errOutputLimit) {
		return forecast.RuntimeResponse{}, fmt.Errorf("%w: output exceeds limit", ErrPythonInvalidResponse)
	}

	decoder := json.NewDecoder(bytes.NewReader(output.buf.Bytes()))
	decoder.DisallowUnknownFields()
	var response forecast.RuntimeResponse
	if err := decoder.Decode(&response); err != nil {
		return forecast.RuntimeResponse{}, fmt.Errorf("%w: %v", ErrPythonInvalidResponse, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return forecast.RuntimeResponse{}, fmt.Errorf("%w: trailing JSON", ErrPythonInvalidResponse)
	}
	if err := forecast.ValidateRuntimeResponse(request, response); err != nil {
		return forecast.RuntimeResponse{}, fmt.Errorf("%w: %v", ErrPythonInvalidResponse, err)
	}
	return response, nil
}

type limitedBuffer struct {
	buf   bytes.Buffer
	limit int
	err   error
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	if len(p) > b.limit-b.buf.Len() {
		b.err = errOutputLimit
		return 0, b.err
	}
	return b.buf.Write(p)
}

var errOutputLimit = errors.New("forecast output limit exceeded")

func minimalPythonEnvironment() []string {
	allowed := []string{"PATH", "PYTHONIOENCODING", "PYTHONUTF8", "SYSTEMDRIVE", "SYSTEMROOT", "WINDIR", "PATHEXT", "TEMP", "TMP"}
	env := make([]string, 0, len(allowed))
	for _, name := range allowed {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	if _, ok := os.LookupEnv("PYTHONIOENCODING"); !ok {
		env = append(env, "PYTHONIOENCODING=utf-8")
	}
	return env
}

// ForecastRequest declares one tenant-scoped inference input and its frozen
// evaluation result. The evaluation, not a caller-provided predictor string,
// determines whether Python is invoked or a Go baseline is used.
type ForecastRequest struct {
	TenantID          string
	ItemID            string
	ObservationDate   string
	CorrelationID     string
	Features          forecast.FeatureSnapshot
	Evaluation        forecast.CandidateEvaluation
	CandidateArtifact *forecast.CandidateArtifact
}

// ForecastService performs advisory inference and persists only the
// Go-validated result. It is independent of event ingestion and projection
// transactions.
type ForecastService struct {
	db        *sql.DB
	candidate CandidateInvoker
	timeout   time.Duration
	now       func() time.Time
}

// NewForecastService constructs a Go-owned forecast service. A nil candidate
// invoker is valid for baseline-only outcomes and fails closed for a promoted
// learned candidate.
func NewForecastService(db *sql.DB, candidate CandidateInvoker) *ForecastService {
	return &ForecastService{
		db:        db,
		candidate: candidate,
		timeout:   defaultForecastTimeout,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// SetTimeoutForTest sets the upper bound used around candidate invocation.
func (s *ForecastService) SetTimeoutForTest(timeout time.Duration) {
	if s != nil {
		s.timeout = timeout
	}
}

// PredictAndPersist selects, validates, evaluates, and idempotently persists
// one prediction. Candidate invocation failures return without a prediction
// row, so a later retry can recover without changing core platform state.
func (s *ForecastService) PredictAndPersist(ctx context.Context, request ForecastRequest) (PredictionRecord, error) {
	if s == nil || s.db == nil {
		return PredictionRecord{}, fmt.Errorf("platform: forecast: nil db")
	}
	selection, err := forecast.SelectRuntimePredictor(request.Evaluation)
	if err != nil {
		return PredictionRecord{}, err
	}
	request.CorrelationID = strings.TrimSpace(request.CorrelationID)
	if request.CorrelationID == "" {
		return PredictionRecord{}, ErrInvalidPrediction
	}
	artifact := request.CandidateArtifact
	if selection.Predictor == forecast.RuntimePredictorCandidate && artifact != nil {
		if err := validateCandidateArtifactMetadata(*artifact, request.Features, request.Evaluation, selection); err != nil {
			return PredictionRecord{}, err
		}
	} else {
		// Candidate artifacts are not part of a baseline request. The frozen
		// outcome, not the presence of candidate data, owns this choice.
		artifact = nil
	}
	runtimeRequest, err := forecast.NewRuntimeRequest(request.Features, request.TenantID, request.ItemID, request.ObservationDate, selection, artifact)
	if err != nil {
		return PredictionRecord{}, err
	}

	var response forecast.RuntimeResponse
	switch selection.Predictor {
	case forecast.RuntimePredictorCandidate:
		if s.candidate == nil {
			return PredictionRecord{}, ErrPythonUnavailable
		}
		invokeCtx := ctx
		cancel := func() {}
		if s.timeout > 0 {
			invokeCtx, cancel = context.WithTimeout(ctx, s.timeout)
		}
		response, err = s.candidate.Invoke(invokeCtx, runtimeRequest)
		cancel()
		if err != nil {
			return PredictionRecord{}, err
		}
		if err := forecast.ValidateRuntimeResponse(runtimeRequest, response); err != nil {
			return PredictionRecord{}, fmt.Errorf("%w: %v", ErrPythonInvalidResponse, err)
		}
	case forecast.RuntimePredictorBaseline:
		response, err = forecast.BaselineRuntimeResponse(request.Features, runtimeRequest)
		if err != nil {
			return PredictionRecord{}, err
		}
	default:
		return PredictionRecord{}, ErrInvalidPrediction
	}

	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}
	freshAt, err := forecastFreshAt(request.Features, response.SourceCutoffDate)
	if err != nil {
		return PredictionRecord{}, err
	}
	record := PredictionRecord{
		TenantID:                  response.TenantID,
		ResourceType:              "inventory_item",
		ResourceID:                response.ItemID,
		ObservationDate:           response.ObservationDate,
		ForecastHorizonDays:       response.HorizonDays,
		Target:                    response.Target,
		Status:                    response.Status,
		StockoutRisk:              response.StockoutRisk,
		Uncertainty:               response.Uncertainty,
		AbstentionReason:          response.AbstentionReason,
		EvaluationProtocolVersion: request.Evaluation.EvaluationProtocolVersion,
		DatasetVersion:            request.Features.DatasetVersion,
		FeatureDefinitionVersion:  request.Features.FeatureDefinitionVersion,
		FeatureSnapshotID:         request.Features.SnapshotID,
		FeatureSnapshotChecksum:   request.Features.Checksum,
		SourceCutoffDate:          response.SourceCutoffDate,
		Predictor:                 response.Predictor,
		ModelVersion:              response.ModelVersion,
		CodeVersion:               response.CodeVersion,
		FreshAt:                   freshAt,
		CorrelationID:             request.CorrelationID,
		RecordedAt:                now,
	}
	return PersistPrediction(ctx, s.db, record)
}

func validateCandidateArtifactMetadata(artifact forecast.CandidateArtifact, features forecast.FeatureSnapshot, evaluation forecast.CandidateEvaluation, selection forecast.RuntimeSelection) error {
	if artifact.ModelVersion != selection.ModelVersion || artifact.CodeVersion != selection.CodeVersion || artifact.EvaluationProtocolVersion != evaluation.EvaluationProtocolVersion || artifact.DatasetVersion != evaluation.DatasetVersion || artifact.DatasetChecksum != evaluation.DatasetChecksum || artifact.FeatureDefinitionVersion != features.FeatureDefinitionVersion || artifact.FeatureSnapshotID != features.SnapshotID || artifact.FeatureSnapshotChecksum != features.Checksum || artifact.Target != forecast.CandidateTarget || artifact.HorizonDays != forecast.HorizonDays {
		return fmt.Errorf("%w: candidate artifact lineage", ErrInvalidPrediction)
	}
	return nil
}

func forecastFreshAt(snapshot forecast.FeatureSnapshot, sourceCutoff string) (time.Time, error) {
	if snapshot.SourceBoundary.MaxRecordedAt != "" {
		freshAt, err := time.Parse(time.RFC3339Nano, snapshot.SourceBoundary.MaxRecordedAt)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: feature freshness", ErrInvalidPrediction)
		}
		return freshAt.UTC(), nil
	}
	day, err := time.Parse("2006-01-02", sourceCutoff)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: feature cutoff", ErrInvalidPrediction)
	}
	return day.UTC(), nil
}
