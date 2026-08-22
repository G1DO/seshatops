package observability

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestRegistryRendersFixedMetricSurfaceAndBoundedLabels(t *testing.T) {
	r := NewRegistry()
	r.AddRelayPublish(RelayPublished, 2)
	r.AddRelayPublish(RelayTransient, 1)
	r.AddRelayPublish(RelayQuarantined, 1)
	r.AddRelayPublish(RelayAmbiguous, 1)
	r.AddConsumerProcess(ConsumerProcessed, 1)
	r.AddConsumerProcess(ConsumerSkipped, 1)
	r.AddConsumerAck(ConsumerAcked, 1)
	r.AddConsumerAck(ConsumerWithheld, 1)
	r.AddConsumerAck(ConsumerFailed, 1)
	r.RecordControl(ControlRebuild, ControlComplete, 125*time.Millisecond)
	r.RecordHTTP(RouteMetrics, "success")
	r.RecordHTTPDenial(RouteOps, DenialForbidden)
	r.RecordPrediction(PredictorBaseline, PredictionPredicted)
	r.SetForecastFreshness(FreshnessStale)
	r.RecordPythonInvocation(PythonTimeout)
	r.SetPythonAvailability(PythonAvailabilityUnavailable)
	r.SetGauge(GaugeOutboxPending, 3)
	r.SetGauge(GaugeConsumerObservedLag, 4)
	r.SetGauge(GaugeConsumerObservedLagKnown, 1)
	r.SetGauge(GaugeOutboxPending, -1)

	raw := r.RenderPrometheus()
	for _, want := range []string{
		`seshatops_relay_publish_outcomes_total{outcome="published"} 2`,
		`seshatops_relay_publish_outcomes_total{outcome="quarantined"} 1`,
		`seshatops_consumer_ack_outcomes_total{outcome="commit_failed"} 1`,
		`seshatops_control_operations_total{operation="rebuild",outcome="complete"} 1`,
		`seshatops_auth_denials_total{route="ops",reason="forbidden"} 1`,
		`seshatops_forecast_freshness{state="stale"} 1`,
		`seshatops_python_candidate_invocations_total{outcome="timeout"} 1`,
		"seshatops_outbox_backlog_records_pending 0",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("metrics missing %q:\n%s", want, raw)
		}
	}
	for _, forbidden := range []string{"tenant", "event_id", "resource", "correlation_id", "cookie", "token", "payload", "feature", "artifact"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("metrics leaked %q:\n%s", forbidden, raw)
		}
	}
}

func TestCorrelationIDIsGeneratedAndCallerValuesAreRejected(t *testing.T) {
	ctx, id, err := EnsureCorrelationID(context.Background())
	if err != nil || id == "" || CorrelationID(ctx) != id {
		t.Fatalf("correlation id=%q ctx=%q err=%v", id, CorrelationID(ctx), err)
	}
	if CorrelationID(WithCorrelationID(context.Background(), "caller-input")) != "" {
		t.Fatal("accepted caller correlation input")
	}
}

func TestLogFiltersUnsafeFields(t *testing.T) {
	var output strings.Builder
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	ctx, _, err := EnsureCorrelationID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	Log(ctx, logger, EventControlCompleted, Fields{Route: RouteOps, Operation: ControlRebuild, Outcome: "complete", Checksum: "not-a-checksum"})
	raw := output.String()
	if !strings.Contains(raw, `"correlation_id"`) || !strings.Contains(raw, `"route":"ops"`) || !strings.Contains(raw, `"operation":"rebuild"`) {
		t.Fatalf("safe log=%s", raw)
	}
	if strings.Contains(raw, "not-a-checksum") || strings.Contains(raw, "caller-input") {
		t.Fatalf("unsafe log=%s", raw)
	}
}

func TestLogEmitsBoundedWorkerIdentityForRetrying(t *testing.T) {
	var output strings.Builder
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	ctx, _, err := EnsureCorrelationID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	Log(ctx, logger, EventWorkerRetrying, Fields{Worker: WorkerRelay})
	raw := output.String()
	if !strings.Contains(raw, `"msg":"worker.retrying"`) || !strings.Contains(raw, `"correlation_id"`) || !strings.Contains(raw, `"worker":"relay"`) {
		t.Fatalf("worker retry log=%s", raw)
	}

	output.Reset()
	Log(ctx, logger, EventWorkerRetrying, Fields{Worker: Worker("relay-2"), Outcome: "unavailable; retrying"})
	if strings.Contains(output.String(), "relay-2") || strings.Contains(output.String(), `"outcome"`) {
		t.Fatalf("unbounded worker fields leaked: %s", output.String())
	}
}
