package api

import (
	"net/http"
	"strings"

	"github.com/G1DO/seshatops/forecast"
	"github.com/G1DO/seshatops/platform"
)

func (s *Server) handleForecastFeatures(w http.ResponseWriter, r *http.Request, tenantID string) {
	source, err := platform.LoadTenantForecastSource(r.Context(), s.db, tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorBody{Error: "forecast_features_failed"})
		return
	}

	if source.Status != forecast.SnapshotStatusComplete {
		writeJSON(w, http.StatusOK, forecast.NewNonCompleteFeatureSnapshot(
			source.Status,
			tenantID,
			source.Boundary,
			source.StatusReasons...,
		))
		return
	}

	snapshot, err := forecast.BuildFeatureSnapshot(source.History, tenantID, source.Boundary)
	if err != nil {
		status := forecast.SnapshotStatusIncomplete
		if strings.Contains(err.Error(), "no observations") {
			status = forecast.SnapshotStatusInsufficient
		}
		snapshot = forecast.NewNonCompleteFeatureSnapshot(status, tenantID, source.Boundary, err.Error())
	}
	writeJSON(w, http.StatusOK, snapshot)
}
