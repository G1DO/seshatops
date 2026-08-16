package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/platform"
	"github.com/G1DO/seshatops/relay"
)

// EventProjectionUpdated is the SSE event name for committed projection changes.
const EventProjectionUpdated = "inventory_projection.updated"

// Server is the Event Spine projection HTTP surface plus Issue #48
// privileged quarantine/replay/rebuild controls and Issue #49 audit read.
type Server struct {
	db           *sql.DB
	hub          *Hub
	auth         identity.SessionLookup
	policy       identity.Authorizer
	now          func() time.Time
	sseHeartbeat time.Duration
	OnDecision   func(ControlDecision)
}

// NewServer constructs the API server. hub may be nil only when SSE is
// unused; REST still works. Callers that run the consumer should
// platform.SetAppliedNotifier(hub). auth is required: a nil lookup fails closed
// with 401 rather than serving projection data. policy is required: a nil
// authorizer fails closed with 403 rather than serving tenant data.
func NewServer(db *sql.DB, hub *Hub, auth identity.SessionLookup, policy identity.Authorizer) *Server {
	if hub == nil {
		hub = NewHub()
	}
	return &Server{
		db:           db,
		hub:          hub,
		auth:         auth,
		policy:       policy,
		now:          func() time.Time { return time.Now().UTC() },
		sseHeartbeat: 15 * time.Second,
	}
}

// Hub returns the notification hub used for SSE fanout.
func (s *Server) Hub() *Hub { return s.hub }

// SetSSEHeartbeatForTest shortens the SSE session-recheck interval.
func (s *Server) SetSSEHeartbeatForTest(d time.Duration) {
	s.sseHeartbeat = d
}

// Handler returns the HTTP handler for the Event Spine projection routes.
// Every /v1 path requires a fresh Go-owned session (Issue #45). Inventory
// reads also require MX-001 for the path tenant (Issue #46). Ops visibility
// and batch lineage reads require MX-002 or MX-003 (Issue #47 / #74). Privileged POSTs require MX-004,
// MX-005, or MX-006 (Issue #48). Audit read requires MX-007 (Issue #49).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/tenants/", s.serveTenant)
	return identity.RequireSession(s.auth, mux)
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

	if batchID, ok := lineageBatchID(rest); ok {
		s.serveBatchLineage(w, r, tenantID, batchID)
		return
	}

	switch rest {
	case "inventory":
		switch r.Method {
		case http.MethodGet:
			if !s.authorizeInventoryRead(w, r, tenantID) {
				return
			}
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
			if !s.authorizeInventoryRead(w, r, tenantID) {
				return
			}
			s.handleSSE(w, r, tenantID)
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
		default:
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
		}
	case "ops":
		switch r.Method {
		case http.MethodGet:
			if !s.authorizeOpsRead(w, r, tenantID) {
				return
			}
			s.handleOps(w, r, tenantID)
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
		default:
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
		}
	case "ops/quarantine/release":
		s.handleControlMethod(w, r, tenantID, http.MethodPost, s.handleQuarantineRelease)
	case "ops/replay":
		s.handleControlMethod(w, r, tenantID, http.MethodPost, s.handleReplay)
	case "ops/rebuild":
		s.handleControlMethod(w, r, tenantID, http.MethodPost, s.handleRebuild)
	case "ops/audit":
		switch r.Method {
		case http.MethodGet:
			s.handleAuditRead(w, r, tenantID)
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

	interval := s.sseHeartbeat
	if interval <= 0 {
		interval = 15 * time.Second
	}
	heartbeat := time.NewTicker(interval)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if !s.stillAllowed(r, tenantID) {
				return
			}
			if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case update, ok := <-updates:
			if !ok {
				return
			}
			if !s.stillAllowed(r, tenantID) {
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

func (s *Server) handleOps(w http.ResponseWriter, r *http.Request, tenantID string) {
	ctx := r.Context()
	snap, err := s.loadOps(ctx, tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorBody{Error: "ops_failed"})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) loadOps(ctx context.Context, tenantID string) (OpsSnapshot, error) {
	rows, err := platform.ListTenantProjection(ctx, s.db, tenantID)
	if err != nil {
		return OpsSnapshot{}, err
	}
	checksum, err := platform.ChecksumTenant(ctx, s.db, tenantID)
	if err != nil {
		return OpsSnapshot{}, err
	}
	backlog, err := relay.InspectBacklogForTenant(ctx, s.db, tenantID)
	if err != nil {
		return OpsSnapshot{}, err
	}
	proc, err := platform.InspectProcessingForTenant(ctx, s.db, tenantID)
	if err != nil {
		return OpsSnapshot{}, err
	}

	quarantines := make([]OpsQuarantineSample, 0, len(backlog.Quarantines))
	for _, q := range backlog.Quarantines {
		quarantines = append(quarantines, OpsQuarantineSample{
			EventID:       q.EventID,
			LastErrorCode: q.LastErrorCode,
			CreatedAt:     q.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	failures := make([]OpsFailureSample, 0, len(proc.Failures))
	for _, f := range proc.Failures {
		failures = append(failures, OpsFailureSample{
			FailureID:        f.FailureID,
			EventID:          f.EventID,
			TenantID:         f.TenantID,
			AggregateType:    f.AggregateType,
			AggregateID:      f.AggregateID,
			EventType:        f.EventType,
			FailureCategory:  f.FailureCategory,
			DiagnosticCode:   f.DiagnosticCode,
			QuarantineStatus: f.QuarantineStatus,
			SourceTopic:      f.SourceTopic,
			SourcePartition:  f.SourcePartition,
			SourceOffset:     f.SourceOffset,
			AttemptCount:     f.AttemptCount,
			CreatedAt:        f.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	gaps := make([]OpsGapSample, 0, len(proc.Gaps))
	for _, g := range proc.Gaps {
		gaps = append(gaps, OpsGapSample{
			EventID:          g.EventID,
			TenantID:         g.TenantID,
			AggregateType:    g.AggregateType,
			AggregateID:      g.AggregateID,
			AggregateVersion: g.AggregateVersion,
			ExpectedVersion:  g.ExpectedVersion,
			ReceivedVersion:  g.ReceivedVersion,
			CreatedAt:        g.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}

	return OpsSnapshot{
		TenantID:   tenantID,
		ObservedAt: s.now().Format(time.RFC3339Nano),
		Projection: OpsProjection{
			Checksum:  checksum,
			ItemCount: len(rows),
		},
		Backlog: OpsBacklog{
			Pending:           backlog.Pending,
			Publishing:        backlog.Publishing,
			Published:         backlog.Published,
			Quarantined:       backlog.Quarantined,
			OldestUnpublished: formatTimePtr(backlog.OldestUnpublished),
			Quarantines:       quarantines,
		},
		Processing: OpsProcessing{
			Applied:               proc.Applied,
			DuplicateNoop:         proc.DuplicateNoop,
			QuarantinedConflict:   proc.QuarantinedConflict,
			QuarantinedGap:        proc.QuarantinedGap,
			QuarantinedStale:      proc.QuarantinedStale,
			QuarantinedInvalid:    proc.QuarantinedInvalid,
			QuarantinedMismatch:   proc.QuarantinedMismatch,
			QuarantinedTransition: proc.QuarantinedTransition,
			FailuresRetrying:      proc.FailuresRetrying,
			FailuresQuarantined:   proc.FailuresQuarantined,
			OldestGap:             formatTimePtr(proc.OldestGap),
			OldestFailure:         formatTimePtr(proc.OldestFailure),
			Failures:              failures,
			Gaps:                  gaps,
		},
	}, nil
}

func formatTimePtr(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.UTC().Format(time.RFC3339Nano)
	return &s
}

func (s *Server) authorizeInventoryRead(w http.ResponseWriter, r *http.Request, tenantID string) bool {
	sess, ok := identity.FromContext(r.Context())
	if !ok || sess == nil {
		writeJSON(w, http.StatusUnauthorized, ErrorBody{Error: "unauthenticated"})
		return false
	}
	if s.policy == nil || s.policy.Allow(sess.PrincipalID, tenantID, identity.ResInventoryProjection, identity.ActRead) != nil {
		writeJSON(w, http.StatusForbidden, ErrorBody{Error: "forbidden"})
		return false
	}
	return true
}

func (s *Server) authorizeOpsRead(w http.ResponseWriter, r *http.Request, tenantID string) bool {
	sess, ok := identity.FromContext(r.Context())
	if !ok || sess == nil {
		writeJSON(w, http.StatusUnauthorized, ErrorBody{Error: "unauthenticated"})
		return false
	}
	if s.policy == nil || s.policy.Allow(sess.PrincipalID, tenantID, identity.ResOpsVisibility, identity.ActRead) != nil {
		writeJSON(w, http.StatusForbidden, ErrorBody{Error: "forbidden"})
		return false
	}
	return true
}

func (s *Server) stillAllowed(r *http.Request, tenantID string) bool {
	if s.auth == nil {
		return false
	}
	sess, err := s.auth.Session(r)
	if err != nil || sess == nil {
		return false
	}
	if s.policy == nil {
		return false
	}
	return s.policy.Allow(sess.PrincipalID, tenantID, identity.ResInventoryProjection, identity.ActRead) == nil
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
