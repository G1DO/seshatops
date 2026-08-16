package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/G1DO/seshatops/platform"
)

func lineageBatchID(rest string) (string, bool) {
	const prefix = "ops/lineage/batches/"
	if !strings.HasPrefix(rest, prefix) {
		return "", false
	}
	id := rest[len(prefix):]
	if id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

func (s *Server) serveBatchLineage(w http.ResponseWriter, r *http.Request, tenantID, batchID string) {
	switch r.Method {
	case http.MethodGet:
		if !s.authorizeOpsRead(w, r, tenantID) {
			return
		}
		s.handleBatchLineage(w, r, tenantID, batchID)
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
	default:
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
	}
}

func (s *Server) handleBatchLineage(w http.ResponseWriter, r *http.Request, tenantID, batchID string) {
	trace, ok, err := platform.TraceBatch(r.Context(), s.db, tenantID, batchID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorBody{Error: "lineage_failed"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorBody{Error: "not_found"})
		return
	}
	writeJSON(w, http.StatusOK, BatchTraceSnapshot{
		TenantID:   tenantID,
		ObservedAt: s.now().Format(time.RFC3339Nano),
		Supplier:   lineageHopFrom(trace.Supplier),
		Lot:        lineageHopFrom(trace.Lot),
		Batch:      lineageHopFrom(trace.Batch),
		Shipment:   lineageHopFrom(trace.Shipment),
	})
}

func lineageHopFrom(hop platform.LineageHop) LineageHop {
	return LineageHop{
		ID:                 hop.ID,
		ParentID:           hop.ParentID,
		ItemID:             hop.ItemID,
		OrderID:            hop.OrderID,
		AggregateVersion:   hop.AggregateVersion,
		SourceEventID:      hop.SourceEventID,
		EventSchemaVersion: hop.EventSchemaVersion,
		OccurredAt:         hop.OccurredAt,
		RecordedAt:         hop.RecordedAt,
		CorrelationID:      hop.CorrelationID,
		CausationID:        hop.CausationID,
		TraceID:            hop.TraceID,
	}
}
