package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/G1DO/seshatops/api"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/observability"
)

func releaseObserverPolicy(principal string) identity.Authorizer {
	return identity.NewPolicy(identity.NewDirectory(identity.Assignment{
		PrincipalID: principal,
		TenantID:    identity.ScopeRuntime,
		RoleID:      identity.RoleReleaseObserver,
	}))
}

func TestMetricsHandlerFailsClosedAndRendersOnlyAggregateSignals(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	seedOpsSignals(t, db, fx)

	registry := observability.NewRegistry()
	registry.RecordHTTP(observability.RouteOps, "success")
	registry.SetGauge(observability.GaugeRuntimeReady, 1)

	unauthenticated := api.NewServer(db, api.NewHub(), nil, releaseObserverPolicy("test-operator"))
	unauthenticated.SetMetricsRegistry(registry)
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	unauthenticated.MetricsHandler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "seshatops_") {
		t.Fatalf("unauthenticated response=%d body=%s", response.Code, response.Body.String())
	}

	denied := api.NewServer(db, api.NewHub(), allowAllAuth{}, northstarReaderPolicy())
	denied.SetMetricsRegistry(registry)
	response = httptest.NewRecorder()
	denied.MetricsHandler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || strings.Contains(response.Body.String(), "seshatops_") {
		t.Fatalf("forbidden response=%d body=%s", response.Code, response.Body.String())
	}

	allowed := api.NewServer(db, api.NewHub(), allowAllAuth{}, releaseObserverPolicy("test-operator"))
	allowed.SetMetricsRegistry(registry)
	response = httptest.NewRecorder()
	allowed.MetricsHandler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("allowed response=%d body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain; version=0.0.4") {
		t.Fatalf("content type=%q", got)
	}
	raw, err := io.ReadAll(response.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"seshatops_outbox_backlog_records_pending 1",
		"seshatops_processing_failures_quarantined 3",
		"seshatops_http_requests_total{route=\"ops\",outcome=\"success\"} 1",
	} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("missing %q:\n%s", want, raw)
		}
	}
	for _, forbidden := range []string{fx.TenantID, fx.Event.EventID, "handler_poison", "an-envelope", "event_bytes"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("metrics leaked %q:\n%s", forbidden, raw)
		}
	}
}

func TestMetricsHandlerRejectsWrongMethodAndQuery(t *testing.T) {
	db := openTestDB(t)
	srv := api.NewServer(db, api.NewHub(), allowAllAuth{}, releaseObserverPolicy("test-operator"))
	srv.SetMetricsRegistry(observability.NewRegistry())
	for _, raw := range []string{"/metrics?tenant_id=anything", "/metrics"} {
		method := http.MethodGet
		if raw == "/metrics" {
			method = http.MethodPost
		}
		response := httptest.NewRecorder()
		srv.MetricsHandler().ServeHTTP(response, httptest.NewRequest(method, raw, nil))
		want := http.StatusBadRequest
		if method == http.MethodPost {
			want = http.StatusMethodNotAllowed
		}
		if response.Code != want {
			t.Fatalf("%s %s status=%d body=%s", method, raw, response.Code, response.Body.String())
		}
	}
}
