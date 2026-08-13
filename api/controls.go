package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/platform"
)

const (
	controlQuarantineRelease = "quarantine_release"
	controlReplay            = "replay"
	controlRebuild           = "rebuild"
	decisionAllow            = "allow"
	decisionDeny             = "deny"
)

func (s *Server) handleControlMethod(w http.ResponseWriter, r *http.Request, tenantID, allow string, fn func(http.ResponseWriter, *http.Request, string)) {
	if r.Method != allow {
		w.Header().Set("Allow", allow)
		writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
		return
	}
	fn(w, r, tenantID)
}

func (s *Server) authorizeControl(w http.ResponseWriter, r *http.Request, tenantID, resource, action, targetID string) bool {
	sess, ok := identity.FromContext(r.Context())
	if !ok || sess == nil {
		writeJSON(w, http.StatusUnauthorized, ErrorBody{Error: "unauthenticated"})
		return false
	}
	if s.policy == nil || s.policy.Allow(sess.PrincipalID, tenantID, resource, action) != nil {
		s.recordDecision(ControlDecision{
			PrincipalID: sess.PrincipalID,
			TenantID:    tenantID,
			Resource:    resource,
			Action:      action,
			Outcome:     decisionDeny,
			Reason:      "forbidden",
			TargetID:    targetID,
		})
		writeJSON(w, http.StatusForbidden, ErrorBody{Error: "forbidden"})
		return false
	}
	s.recordDecision(ControlDecision{
		PrincipalID: sess.PrincipalID,
		TenantID:    tenantID,
		Resource:    resource,
		Action:      action,
		Outcome:     decisionAllow,
		Reason:      "matrix_allow",
		TargetID:    targetID,
	})
	return true
}

func (s *Server) recordDecision(d ControlDecision) {
	if s == nil || s.OnDecision == nil {
		return
	}
	if d.At == "" {
		d.At = s.now().UTC().Format(time.RFC3339Nano)
	}
	s.OnDecision(d)
}

func (s *Server) handleQuarantineRelease(w http.ResponseWriter, r *http.Request, tenantID string) {
	req, ok := decodeControlRequest(w, r, true)
	if !ok {
		return
	}
	if !s.authorizeControl(w, r, tenantID, identity.ResQuarantine, identity.ActQuarantineRelease, req.EventID) {
		return
	}
	err := platform.ReleaseTenantQuarantine(r.Context(), s.db, tenantID, req.EventID)
	if errors.Is(err, platform.ErrNotReleasable) {
		writeJSON(w, http.StatusConflict, ErrorBody{Error: "not_releasable"})
		return
	}
	if errors.Is(err, platform.ErrControlNotFound) || errors.Is(err, platform.ErrTenantMismatch) {
		writeJSON(w, http.StatusNotFound, ErrorBody{Error: "not_found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorBody{Error: "control_failed"})
		return
	}
	writeJSON(w, http.StatusOK, ControlResult{
		TenantID: tenantID,
		Control:  controlQuarantineRelease,
		EventID:  req.EventID,
		Status:   "released",
	})
}

func (s *Server) handleReplay(w http.ResponseWriter, r *http.Request, tenantID string) {
	req, ok := decodeControlRequest(w, r, false)
	if !ok {
		return
	}
	if !s.authorizeControl(w, r, tenantID, identity.ResReplay, identity.ActReplay, req.EventID) {
		return
	}
	result, err := platform.ReplayTenantHistory(r.Context(), s.db, tenantID, req.EventID)
	s.writeReplayResult(w, tenantID, controlReplay, req.EventID, result, err)
}

func (s *Server) handleRebuild(w http.ResponseWriter, r *http.Request, tenantID string) {
	req, ok := decodeControlRequest(w, r, false)
	if !ok {
		return
	}
	if !s.authorizeControl(w, r, tenantID, identity.ResRebuild, identity.ActRebuild, req.EventID) {
		return
	}
	result, err := platform.RebuildTenantFromHistory(r.Context(), s.db, tenantID)
	s.writeReplayResult(w, tenantID, controlRebuild, "", result, err)
}

func (s *Server) writeReplayResult(w http.ResponseWriter, tenantID, control, eventID string, result platform.RebuildResult, err error) {
	if errors.Is(err, platform.ErrControlNotFound) || errors.Is(err, platform.ErrTenantMismatch) {
		writeJSON(w, http.StatusNotFound, ErrorBody{Error: "not_found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorBody{Error: "control_failed"})
		return
	}
	status := result.Status
	if status == "" {
		status = platform.RebuildStatusIncomplete
	}
	writeJSON(w, http.StatusOK, ControlResult{
		TenantID:          tenantID,
		Control:           control,
		EventID:           eventID,
		Status:            status,
		Applied:           result.Applied,
		DuplicateNoop:     result.DuplicateNoop,
		Quarantined:       result.Quarantined,
		Checksum:          result.Checksum,
		IncompleteReasons: result.IncompleteReasons,
	})
}

func decodeControlRequest(w http.ResponseWriter, r *http.Request, eventIDRequired bool) (ControlRequest, bool) {
	var req ControlRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "invalid_request"})
		return ControlRequest{}, false
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "invalid_request"})
			return ControlRequest{}, false
		}
	}
	if eventIDRequired {
		if !validTenantID(req.EventID) {
			writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "invalid_event_id"})
			return ControlRequest{}, false
		}
	} else if req.EventID != "" && !validTenantID(req.EventID) {
		writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "invalid_event_id"})
		return ControlRequest{}, false
	}
	return req, true
}
