package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/G1DO/seshatops/forecast"
	"github.com/G1DO/seshatops/observability"
	"github.com/G1DO/seshatops/platform"
)

const (
	forecastFreshnessFresh       = "fresh"
	forecastFreshnessStale       = "stale"
	forecastFreshnessUnavailable = "unavailable"
)

func forecastPredictionResourceID(rest string) (string, bool) {
	const prefix = "forecast/predictions/"
	if !strings.HasPrefix(rest, prefix) {
		return "", false
	}
	resourceID := rest[len(prefix):]
	if resourceID == "" || strings.Contains(resourceID, "/") {
		return "", false
	}
	return resourceID, true
}

func (s *Server) serveForecastPrediction(w http.ResponseWriter, r *http.Request, tenantID, resourceID string) {
	switch r.Method {
	case http.MethodGet:
		if !s.authorizeForecastPredictionRead(w, r, tenantID) {
			return
		}
		if len(r.URL.Query()) != 0 {
			writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "unsupported_forecast_query"})
			return
		}
		s.handleForecastPrediction(w, r, tenantID, resourceID)
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
	default:
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
	}
}

func (s *Server) handleForecastPrediction(w http.ResponseWriter, r *http.Request, tenantID, resourceID string) {
	record, found, err := platform.GetLatestPredictionForTenantResource(r.Context(), s.db, tenantID, resourceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorBody{Error: "forecast_prediction_failed"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, ErrorBody{Error: "not_found"})
		return
	}

	freshness, err := s.forecastPredictionFreshness(r, record)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorBody{Error: "forecast_prediction_failed"})
		return
	}
	s.recordForecastObservation(r, record.Predictor, record.Status, freshness.Status)

	writeJSON(w, http.StatusOK, ForecastPredictionSnapshot{
		TenantID:            record.TenantID,
		ResourceType:        record.ResourceType,
		ResourceID:          record.ResourceID,
		PredictionID:        record.PredictionID,
		ObservationDate:     record.ObservationDate,
		ForecastHorizonDays: record.ForecastHorizonDays,
		Target:              record.Target,
		Status:              record.Status,
		StockoutRisk:        record.StockoutRisk,
		Uncertainty:         predictionUncertainty(record.Uncertainty),
		AbstentionReason:    record.AbstentionReason,
		Freshness:           freshness,
		Lineage: ForecastPredictionLineage{
			EvaluationProtocolVersion: record.EvaluationProtocolVersion,
			DatasetVersion:            record.DatasetVersion,
			FeatureDefinitionVersion:  record.FeatureDefinitionVersion,
			FeatureSnapshotID:         record.FeatureSnapshotID,
			FeatureSnapshotChecksum:   record.FeatureSnapshotChecksum,
			SourceCutoffDate:          record.SourceCutoffDate,
			Predictor:                 record.Predictor,
			ModelVersion:              record.ModelVersion,
			CodeVersion:               record.CodeVersion,
		},
		CorrelationID: record.CorrelationID,
		RecordedAt:    record.RecordedAt.UTC().Format(time.RFC3339Nano),
		ObservedAt:    s.now().Format(time.RFC3339Nano),
	})
}

func (s *Server) recordForecastObservation(r *http.Request, predictor, status, freshness string) {
	if s == nil || r == nil {
		return
	}
	observedPredictor := observability.Predictor(predictor)
	observedStatus := observability.PredictionOutcome(status)
	observedFreshness := observability.Freshness(freshness)
	if s.metrics != nil {
		s.metrics.RecordPrediction(observedPredictor, observedStatus)
		s.metrics.SetForecastFreshness(observedFreshness)
	}
	observability.Log(r.Context(), nil, observability.EventForecastCompleted, observability.Fields{
		Outcome:   string(observedStatus),
		Predictor: observedPredictor,
		Freshness: observedFreshness,
	})
}

func (s *Server) forecastPredictionFreshness(r *http.Request, record platform.PredictionRecord) (ForecastPredictionFreshness, error) {
	freshness := ForecastPredictionFreshness{
		Status:  forecastFreshnessUnavailable,
		FreshAt: record.FreshAt.UTC().Format(time.RFC3339Nano),
	}
	source, err := platform.LoadTenantForecastSource(r.Context(), s.db, record.TenantID)
	if err != nil {
		return ForecastPredictionFreshness{}, err
	}
	if source.Status == forecast.SnapshotStatusStale {
		freshness.Status = forecastFreshnessStale
		return freshness, nil
	}
	if source.Status != forecast.SnapshotStatusComplete {
		return freshness, nil
	}
	snapshot, err := forecast.BuildFeatureSnapshot(source.History, record.TenantID, source.Boundary)
	if err != nil {
		return freshness, nil
	}
	if snapshot.Status == forecast.SnapshotStatusStale {
		freshness.Status = forecastFreshnessStale
		return freshness, nil
	}
	if snapshot.Status != forecast.SnapshotStatusComplete {
		return freshness, nil
	}
	if record.FeatureSnapshotID == snapshot.SnapshotID && record.FeatureSnapshotChecksum == snapshot.Checksum && snapshotContainsPrediction(snapshot, record) {
		freshness.Status = forecastFreshnessFresh
	} else {
		freshness.Status = forecastFreshnessStale
	}
	return freshness, nil
}

func snapshotContainsPrediction(snapshot forecast.FeatureSnapshot, record platform.PredictionRecord) bool {
	for _, row := range snapshot.Rows {
		if row.TenantID == record.TenantID && row.ItemID == record.ResourceID && row.AsOfDate == record.ObservationDate && row.SourceCutoffDate == record.SourceCutoffDate {
			return true
		}
	}
	return false
}

func predictionUncertainty(value *forecast.PredictionUncertainty) *ForecastPredictionUncertainty {
	if value == nil {
		return nil
	}
	return &ForecastPredictionUncertainty{
		Method:      value.Method,
		Lower:       value.Lower,
		Upper:       value.Upper,
		SampleCount: value.SampleCount,
	}
}
