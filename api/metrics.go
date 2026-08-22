package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/observability"
	"github.com/G1DO/seshatops/platform"
	"github.com/G1DO/seshatops/relay"
)

// MetricsHandler returns the aggregate release-health metrics endpoint. It is
// deliberately separate from tenant routes and requires the Go-selected
// ScopeRuntime plus MX-010; it never accepts a caller-selected scope.
func (s *Server) MetricsHandler() http.Handler {
	return identity.RequireSession(s.auth, http.HandlerFunc(s.handleMetrics))
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: "method_not_allowed"})
		return
	}
	if r.URL.RawQuery != "" {
		writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "unsupported_metrics_query"})
		return
	}
	sess, ok := identity.FromContext(r.Context())
	if !ok || sess == nil {
		writeJSON(w, http.StatusUnauthorized, ErrorBody{Error: "unauthenticated"})
		return
	}
	if s.policy == nil || s.policy.Allow(sess.PrincipalID, identity.ScopeRuntime, identity.ResReleaseMetrics, identity.ActRead) != nil {
		writeJSON(w, http.StatusForbidden, ErrorBody{Error: "forbidden"})
		return
	}
	if s.metrics == nil {
		writeJSON(w, http.StatusServiceUnavailable, ErrorBody{Error: "metrics_unavailable"})
		return
	}
	if err := s.refreshMetrics(r); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorBody{Error: "metrics_failed"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = fmt.Fprint(w, s.metrics.RenderPrometheus())
}

func (s *Server) refreshMetrics(r *http.Request) error {
	backlog, err := relay.InspectBacklog(r.Context(), s.db)
	if err != nil {
		return err
	}
	processing, err := platform.InspectProcessing(r.Context(), s.db)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}
	s.metrics.SetGauge(observability.GaugeOutboxPending, int64(backlog.Pending))
	s.metrics.SetGauge(observability.GaugeOutboxPublishing, int64(backlog.Publishing))
	s.metrics.SetGauge(observability.GaugeOutboxQuarantined, int64(backlog.Quarantined))
	s.metrics.SetGauge(observability.GaugeOutboxOldestAgeSeconds, ageSeconds(now, backlog.OldestUnpublished))
	s.metrics.SetGauge(observability.GaugeProcessingFailuresRetrying, int64(processing.FailuresRetrying))
	s.metrics.SetGauge(observability.GaugeProcessingFailuresQuarantined, int64(processing.FailuresQuarantined))
	s.metrics.SetGauge(observability.GaugeProcessingQuarantinedGap, int64(processing.QuarantinedGap))
	s.metrics.SetGauge(observability.GaugeProcessingOldestFailureAge, ageSeconds(now, processing.OldestFailure))
	s.metrics.SetGauge(observability.GaugeProcessingOldestGapAge, ageSeconds(now, processing.OldestGap))
	return nil
}

func ageSeconds(now, then time.Time) int64 {
	if then.IsZero() || now.Before(then) {
		return 0
	}
	return int64(now.Sub(then).Seconds())
}
