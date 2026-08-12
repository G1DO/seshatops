package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/G1DO/seshatops/platform"
)

// EventProjectionUpdated is the SSE event name for committed projection changes.
const EventProjectionUpdated = "inventory_projection.updated"

// Server is the Event Spine read-only projection HTTP surface.
type Server struct {
	db  *sql.DB
	hub *Hub
	now func() time.Time
}

// NewServer constructs a read-only API server. hub may be nil only when SSE is
// unused; REST still works. Callers that run the consumer should
// platform.SetAppliedNotifier(hub).
func NewServer(db *sql.DB, hub *Hub) *Server {
	if hub == nil {
		hub = NewHub()
	}
	return &Server{
		db:  db,
		hub: hub,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// Hub returns the notification hub used for SSE fanout.
func (s *Server) Hub() *Hub { return s.hub }

// Handler returns the HTTP handler for the Event Spine projection routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/tenants/", s.serveTenant)
	return mux
}

func (s *Server) serveTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, rest, ok := parseTenantPath(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorBody{Error: "not_found"})
		return
	}
	if !validTenantID(tenantID) {
		writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "invalid_tenant_id"})
		return
	}

	switch rest {
	case "inventory":
		switch r.Method {
		case http.MethodGet:
			s.handleSnapshot(w, r, tenantID)
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
		default:
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
		}
	case "inventory/stream":
		switch r.Method {
		case http.MethodGet:
			s.handleSSE(w, r, tenantID)
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
		default:
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
		}
	default:
		writeJSON(w, http.StatusNotFound, ErrorBody{Error: "not_found"})
	}
}

func parseTenantPath(path string) (tenantID, rest string, ok bool) {
	const prefix = "/v1/tenants/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	tail := strings.TrimPrefix(path, prefix)
	if tail == "" {
		return "", "", false
	}
	parts := strings.SplitN(tail, "/", 2)
	tenantID = parts[0]
	if tenantID == "" {
		return "", "", false
	}
	if len(parts) == 1 {
		return tenantID, "", false
	}
	return tenantID, parts[1], true
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request, tenantID string) {
	ctx := r.Context()
	snap, err := s.loadSnapshot(ctx, tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorBody{Error: "snapshot_failed"})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) loadSnapshot(ctx context.Context, tenantID string) (InventorySnapshot, error) {
	rows, err := platform.ListTenantProjection(ctx, s.db, tenantID)
	if err != nil {
		return InventorySnapshot{}, err
	}
	checksum, err := platform.ChecksumTenant(ctx, s.db, tenantID)
	if err != nil {
		return InventorySnapshot{}, err
	}
	items := make([]InventoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, InventoryItem{
			ItemID:           row.ItemID,
			QuantityOnHand:   row.QuantityOnHand,
			AggregateVersion: row.AggregateVersion,
		})
	}
	return InventorySnapshot{
		TenantID:   tenantID,
		Items:      items,
		Checksum:   checksum,
		ObservedAt: s.now().Format(time.RFC3339Nano),
	}, nil
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request, tenantID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, ErrorBody{Error: "streaming_unsupported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	updates, cancel := s.hub.Subscribe(tenantID)
	defer cancel()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case update, ok := <-updates:
			if !ok {
				return
			}
			payload, err := s.projectionUpdatedPayload(ctx, update)
			if err != nil {
				return
			}
			if err := writeSSE(w, EventProjectionUpdated, payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) projectionUpdatedPayload(ctx context.Context, update platform.AppliedUpdate) (ProjectionUpdated, error) {
	checksum, err := platform.ChecksumTenant(ctx, s.db, update.TenantID)
	if err != nil {
		return ProjectionUpdated{}, err
	}
	return ProjectionUpdated{
		TenantID:           update.TenantID,
		ItemID:             update.ItemID,
		QuantityOnHand:     update.QuantityOnHand,
		AggregateVersion:   update.AggregateVersion,
		LastAppliedEventID: update.EventID,
		Checksum:           checksum,
	}, nil
}

func writeSSE(w http.ResponseWriter, event string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
