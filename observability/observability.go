// Package observability provides the bounded local release signals used by
// the runnable SeshatOps process. It deliberately has no telemetry exporter,
// datastore, or dependency outside the standard library.
package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Route is a bounded HTTP route class. It must never contain a raw URL.
type Route string

const (
	RouteAuth      Route = "auth"
	RouteInventory Route = "inventory"
	RouteOps       Route = "ops"
	RouteForecast  Route = "forecast"
	RouteMetrics   Route = "metrics"
	RouteHealth    Route = "health"
	RouteOther     Route = "other"
)

// DenialReason is a bounded authentication or authorization result.
type DenialReason string

const (
	DenialUnauthenticated DenialReason = "unauthenticated"
	DenialForbidden       DenialReason = "forbidden"
)

type RelayPublishOutcome string

const (
	RelayPublished   RelayPublishOutcome = "published"
	RelayTransient   RelayPublishOutcome = "transient"
	RelayQuarantined RelayPublishOutcome = "quarantined"
	RelayAmbiguous   RelayPublishOutcome = "ambiguous"
)

type ConsumerProcessOutcome string

const (
	ConsumerProcessed ConsumerProcessOutcome = "processed"
	ConsumerSkipped   ConsumerProcessOutcome = "skipped"
)

type ConsumerAckOutcome string

const (
	ConsumerAcked    ConsumerAckOutcome = "acknowledged"
	ConsumerWithheld ConsumerAckOutcome = "withheld"
	ConsumerFailed   ConsumerAckOutcome = "commit_failed"
)

type ControlOperation string

const (
	ControlQuarantineRelease ControlOperation = "quarantine_release"
	ControlReplay            ControlOperation = "replay"
	ControlRebuild           ControlOperation = "rebuild"
)

type ControlOutcome string

const (
	ControlReleased      ControlOutcome = "released"
	ControlComplete      ControlOutcome = "complete"
	ControlIncomplete    ControlOutcome = "incomplete"
	ControlDenied        ControlOutcome = "denied"
	ControlNotFound      ControlOutcome = "not_found"
	ControlNotReleasable ControlOutcome = "not_releasable"
	ControlFailed        ControlOutcome = "failed"
)

type Predictor string

const (
	PredictorCandidate Predictor = "candidate"
	PredictorBaseline  Predictor = "baseline"
)

type PredictionOutcome string

const (
	PredictionPredicted PredictionOutcome = "predicted"
	PredictionAbstained PredictionOutcome = "abstained"
)

type Freshness string

const (
	FreshnessFresh       Freshness = "fresh"
	FreshnessStale       Freshness = "stale"
	FreshnessUnavailable Freshness = "unavailable"
)

type PythonOutcome string

const (
	PythonAvailable       PythonOutcome = "available"
	PythonUnavailable     PythonOutcome = "unavailable"
	PythonTimeout         PythonOutcome = "timeout"
	PythonInvalidResponse PythonOutcome = "invalid_response"
)

type PythonAvailability string

const (
	PythonAvailabilityAvailable   PythonAvailability = "available"
	PythonAvailabilityUnavailable PythonAvailability = "unavailable"
)

// Worker is the fixed identity of a long-lived runtime background worker.
type Worker string

const (
	WorkerRelay    Worker = "relay"
	WorkerConsumer Worker = "consumer"
)

func validWorker(value Worker) bool {
	return value == WorkerRelay || value == WorkerConsumer
}

// Gauge is a fixed unlabelled local metric. Values are snapshots; callers
// must not use this type for IDs or arbitrary dimensions.
type Gauge string

const (
	GaugeRuntimeReady                  Gauge = "seshatops_runtime_ready"
	GaugeConsumerObservedLag           Gauge = "seshatops_consumer_observed_lag_records"
	GaugeConsumerObservedLagKnown      Gauge = "seshatops_consumer_observed_lag_known"
	GaugeOutboxPending                 Gauge = "seshatops_outbox_backlog_records_pending"
	GaugeOutboxPublishing              Gauge = "seshatops_outbox_backlog_records_publishing"
	GaugeOutboxQuarantined             Gauge = "seshatops_outbox_backlog_records_quarantined"
	GaugeOutboxOldestAgeSeconds        Gauge = "seshatops_outbox_oldest_unpublished_age_seconds"
	GaugeProcessingFailuresRetrying    Gauge = "seshatops_processing_failures_retrying"
	GaugeProcessingFailuresQuarantined Gauge = "seshatops_processing_failures_quarantined"
	GaugeProcessingQuarantinedGap      Gauge = "seshatops_processing_quarantined_gap"
	GaugeProcessingOldestFailureAge    Gauge = "seshatops_processing_oldest_failure_age_seconds"
	GaugeProcessingOldestGapAge        Gauge = "seshatops_processing_oldest_gap_age_seconds"
)

var gauges = []Gauge{
	GaugeRuntimeReady,
	GaugeConsumerObservedLag,
	GaugeConsumerObservedLagKnown,
	GaugeOutboxPending,
	GaugeOutboxPublishing,
	GaugeOutboxQuarantined,
	GaugeOutboxOldestAgeSeconds,
	GaugeProcessingFailuresRetrying,
	GaugeProcessingFailuresQuarantined,
	GaugeProcessingQuarantinedGap,
	GaugeProcessingOldestFailureAge,
	GaugeProcessingOldestGapAge,
}

// Registry owns the process-local metric counters. All state is in memory and
// resets when this process exits or restarts.
type Registry struct {
	mu sync.RWMutex

	relay      map[RelayPublishOutcome]uint64
	process    map[ConsumerProcessOutcome]uint64
	ack        map[ConsumerAckOutcome]uint64
	controls   map[ControlOperation]map[ControlOutcome]controlValue
	http       map[Route]map[string]uint64
	denials    map[Route]map[DenialReason]uint64
	prediction map[Predictor]map[PredictionOutcome]uint64
	freshness  Freshness
	python     map[PythonOutcome]uint64
	available  PythonAvailability
	gauges     map[Gauge]int64
}

type controlValue struct {
	count uint64
	total time.Duration
}

// NewRegistry returns an empty, bounded registry.
func NewRegistry() *Registry {
	return &Registry{
		relay:      make(map[RelayPublishOutcome]uint64),
		process:    make(map[ConsumerProcessOutcome]uint64),
		ack:        make(map[ConsumerAckOutcome]uint64),
		controls:   make(map[ControlOperation]map[ControlOutcome]controlValue),
		http:       make(map[Route]map[string]uint64),
		denials:    make(map[Route]map[DenialReason]uint64),
		prediction: make(map[Predictor]map[PredictionOutcome]uint64),
		python:     make(map[PythonOutcome]uint64),
		gauges:     make(map[Gauge]int64, len(gauges)),
	}
}

func (r *Registry) AddRelayPublish(outcome RelayPublishOutcome, count uint64) {
	if r == nil || count == 0 || !validRelayOutcome(outcome) {
		return
	}
	r.mu.Lock()
	r.relay[outcome] += count
	r.mu.Unlock()
}

func (r *Registry) AddConsumerProcess(outcome ConsumerProcessOutcome, count uint64) {
	if r == nil || count == 0 || !validConsumerProcessOutcome(outcome) {
		return
	}
	r.mu.Lock()
	r.process[outcome] += count
	r.mu.Unlock()
}

func (r *Registry) AddConsumerAck(outcome ConsumerAckOutcome, count uint64) {
	if r == nil || count == 0 || !validConsumerAckOutcome(outcome) {
		return
	}
	r.mu.Lock()
	r.ack[outcome] += count
	r.mu.Unlock()
}

func (r *Registry) RecordControl(operation ControlOperation, outcome ControlOutcome, duration time.Duration) {
	if r == nil || !validControlOperation(operation) || !validControlOutcome(outcome) {
		return
	}
	if duration < 0 {
		duration = 0
	}
	r.mu.Lock()
	if r.controls[operation] == nil {
		r.controls[operation] = make(map[ControlOutcome]controlValue)
	}
	value := r.controls[operation][outcome]
	value.count++
	value.total += duration
	r.controls[operation][outcome] = value
	r.mu.Unlock()
}

// RecordHTTP stores a bounded HTTP result class. result must be one of
// success, client_error, or server_error.
func (r *Registry) RecordHTTP(route Route, result string) {
	if r == nil || !validRoute(route) || !validHTTPResult(result) {
		return
	}
	r.mu.Lock()
	if r.http[route] == nil {
		r.http[route] = make(map[string]uint64)
	}
	r.http[route][result]++
	r.mu.Unlock()
}

func (r *Registry) RecordHTTPDenial(route Route, reason DenialReason) {
	if r == nil || !validRoute(route) || !validDenial(reason) {
		return
	}
	r.mu.Lock()
	if r.denials[route] == nil {
		r.denials[route] = make(map[DenialReason]uint64)
	}
	r.denials[route][reason]++
	r.mu.Unlock()
}

func (r *Registry) RecordPrediction(predictor Predictor, outcome PredictionOutcome) {
	if r == nil || !validPredictor(predictor) || !validPredictionOutcome(outcome) {
		return
	}
	r.mu.Lock()
	if r.prediction[predictor] == nil {
		r.prediction[predictor] = make(map[PredictionOutcome]uint64)
	}
	r.prediction[predictor][outcome]++
	r.mu.Unlock()
}

func (r *Registry) SetForecastFreshness(freshness Freshness) {
	if r == nil || !validFreshness(freshness) {
		return
	}
	r.mu.Lock()
	r.freshness = freshness
	r.mu.Unlock()
}

func (r *Registry) RecordPythonInvocation(outcome PythonOutcome) {
	if r == nil || !validPythonOutcome(outcome) {
		return
	}
	r.mu.Lock()
	r.python[outcome]++
	r.mu.Unlock()
}

func (r *Registry) SetPythonAvailability(availability PythonAvailability) {
	if r == nil || (availability != PythonAvailabilityAvailable && availability != PythonAvailabilityUnavailable) {
		return
	}
	r.mu.Lock()
	r.available = availability
	r.mu.Unlock()
}

func (r *Registry) SetGauge(gauge Gauge, value int64) {
	if r == nil || !validGauge(gauge) {
		return
	}
	if value < 0 {
		value = 0
	}
	r.mu.Lock()
	r.gauges[gauge] = value
	r.mu.Unlock()
}

// RenderPrometheus returns the Prometheus text exposition for the fixed
// release metric set. It contains no caller-provided labels or values.
func (r *Registry) RenderPrometheus() string {
	if r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b strings.Builder
	writeCounter(&b, "seshatops_relay_publish_outcomes", "Relay publish outcomes acknowledged by the local process.", []string{"outcome"}, relayRows(r.relay))
	writeCounter(&b, "seshatops_consumer_processing_outcomes", "Consumer processing outcomes observed by the local process.", []string{"outcome"}, processRows(r.process))
	writeCounter(&b, "seshatops_consumer_ack_outcomes", "Consumer acknowledgement outcomes observed by the local process.", []string{"outcome"}, ackRows(r.ack))
	writeControl(&b, r.controls)
	writeHTTP(&b, r.http)
	writeDenials(&b, r.denials)
	writePrediction(&b, r.prediction)
	writeFreshness(&b, r.freshness)
	writePython(&b, r.python, r.available)
	for _, gauge := range gauges {
		fmt.Fprintf(&b, "# TYPE %s gauge\n%s %d\n", gauge, gauge, r.gauges[gauge])
	}
	return b.String()
}

type metricRow struct {
	labels []string
	value  uint64
}

func writeCounter(b *strings.Builder, name, help string, _ []string, rows []metricRow) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s counter\n", name, help, name)
	for _, row := range rows {
		fmt.Fprintf(b, "%s_total{%s} %d\n", name, strings.Join(row.labels, ","), row.value)
	}
}

func relayRows(values map[RelayPublishOutcome]uint64) []metricRow {
	outcomes := []RelayPublishOutcome{RelayPublished, RelayTransient, RelayQuarantined, RelayAmbiguous}
	rows := make([]metricRow, 0, len(outcomes))
	for _, outcome := range outcomes {
		rows = append(rows, metricRow{[]string{`outcome="` + string(outcome) + `"`}, values[outcome]})
	}
	return rows
}

func processRows(values map[ConsumerProcessOutcome]uint64) []metricRow {
	outcomes := []ConsumerProcessOutcome{ConsumerProcessed, ConsumerSkipped}
	rows := make([]metricRow, 0, len(outcomes))
	for _, outcome := range outcomes {
		rows = append(rows, metricRow{[]string{`outcome="` + string(outcome) + `"`}, values[outcome]})
	}
	return rows
}

func ackRows(values map[ConsumerAckOutcome]uint64) []metricRow {
	outcomes := []ConsumerAckOutcome{ConsumerAcked, ConsumerWithheld, ConsumerFailed}
	rows := make([]metricRow, 0, len(outcomes))
	for _, outcome := range outcomes {
		rows = append(rows, metricRow{[]string{`outcome="` + string(outcome) + `"`}, values[outcome]})
	}
	return rows
}

func writeControl(b *strings.Builder, values map[ControlOperation]map[ControlOutcome]controlValue) {
	const name = "seshatops_control_operations"
	fmt.Fprintf(b, "# HELP %s Privileged control outcomes observed by the local process.\n# TYPE %s counter\n", name, name)
	fmt.Fprintln(b, "# HELP seshatops_control_duration_seconds Privileged control handler duration observed by the local process.")
	fmt.Fprintln(b, "# TYPE seshatops_control_duration_seconds summary")
	operations := []ControlOperation{ControlQuarantineRelease, ControlReplay, ControlRebuild}
	outcomes := []ControlOutcome{ControlReleased, ControlComplete, ControlIncomplete, ControlDenied, ControlNotFound, ControlNotReleasable, ControlFailed}
	for _, operation := range operations {
		for _, outcome := range outcomes {
			value := values[operation][outcome]
			labels := `operation="` + string(operation) + `",outcome="` + string(outcome) + `"`
			fmt.Fprintf(b, "%s_total{%s} %d\n", name, labels, value.count)
			fmt.Fprintf(b, "seshatops_control_duration_seconds_sum{%s} %s\n", labels, formatSeconds(value.total))
			fmt.Fprintf(b, "seshatops_control_duration_seconds_count{%s} %d\n", labels, value.count)
		}
	}
}

func writeHTTP(b *strings.Builder, values map[Route]map[string]uint64) {
	const name = "seshatops_http_requests"
	fmt.Fprintf(b, "# HELP %s HTTP response classes observed by the local process.\n# TYPE %s counter\n", name, name)
	for _, route := range orderedRoutes() {
		for _, outcome := range []string{"success", "client_error", "server_error"} {
			fmt.Fprintf(b, "%s_total{route=%q,outcome=%q} %d\n", name, route, outcome, values[route][outcome])
		}
	}
}

func writeDenials(b *strings.Builder, values map[Route]map[DenialReason]uint64) {
	const name = "seshatops_auth_denials"
	fmt.Fprintf(b, "# HELP %s Authentication and authorization denials observed by the local process.\n# TYPE %s counter\n", name, name)
	for _, route := range orderedRoutes() {
		for _, reason := range []DenialReason{DenialUnauthenticated, DenialForbidden} {
			fmt.Fprintf(b, "%s_total{route=%q,reason=%q} %d\n", name, route, reason, values[route][reason])
		}
	}
}

func writePrediction(b *strings.Builder, values map[Predictor]map[PredictionOutcome]uint64) {
	const name = "seshatops_prediction_outcomes"
	fmt.Fprintf(b, "# HELP %s Forecast prediction outcomes observed by the local process.\n# TYPE %s counter\n", name, name)
	for _, predictor := range []Predictor{PredictorCandidate, PredictorBaseline} {
		for _, outcome := range []PredictionOutcome{PredictionPredicted, PredictionAbstained} {
			fmt.Fprintf(b, "%s_total{predictor=%q,outcome=%q} %d\n", name, predictor, outcome, values[predictor][outcome])
		}
	}
}

func writeFreshness(b *strings.Builder, current Freshness) {
	const name = "seshatops_forecast_freshness"
	fmt.Fprintf(b, "# HELP %s Last forecast freshness state observed by the local process.\n# TYPE %s gauge\n", name, name)
	for _, state := range []Freshness{FreshnessFresh, FreshnessStale, FreshnessUnavailable} {
		value := 0
		if state == current {
			value = 1
		}
		fmt.Fprintf(b, "%s{state=%q} %d\n", name, state, value)
	}
}

func writePython(b *strings.Builder, values map[PythonOutcome]uint64, availability PythonAvailability) {
	const name = "seshatops_python_candidate_invocations"
	fmt.Fprintf(b, "# HELP %s Python candidate invocation outcomes for this process.\n# TYPE %s counter\n", name, name)
	for _, outcome := range []PythonOutcome{PythonAvailable, PythonUnavailable, PythonTimeout, PythonInvalidResponse} {
		fmt.Fprintf(b, "%s_total{outcome=%q} %d\n", name, outcome, values[outcome])
	}
	fmt.Fprintln(b, "# HELP seshatops_python_candidate_available Last Python candidate availability observed by this process.")
	fmt.Fprintln(b, "# TYPE seshatops_python_candidate_available gauge")
	value := 0
	if availability == PythonAvailabilityAvailable {
		value = 1
	}
	fmt.Fprintf(b, "seshatops_python_candidate_available %d\n", value)
}

func formatSeconds(value time.Duration) string {
	return strconv.FormatFloat(value.Seconds(), 'f', -1, 64)
}

func orderedRoutes() []Route {
	return []Route{RouteAuth, RouteInventory, RouteOps, RouteForecast, RouteMetrics, RouteHealth, RouteOther}
}

func validRoute(value Route) bool {
	for _, v := range orderedRoutes() {
		if v == value {
			return true
		}
	}
	return false
}
func validRelayOutcome(value RelayPublishOutcome) bool {
	return value == RelayPublished || value == RelayTransient || value == RelayQuarantined || value == RelayAmbiguous
}
func validConsumerProcessOutcome(value ConsumerProcessOutcome) bool {
	return value == ConsumerProcessed || value == ConsumerSkipped
}
func validConsumerAckOutcome(value ConsumerAckOutcome) bool {
	return value == ConsumerAcked || value == ConsumerWithheld || value == ConsumerFailed
}
func validControlOperation(value ControlOperation) bool {
	return value == ControlQuarantineRelease || value == ControlReplay || value == ControlRebuild
}
func validControlOutcome(value ControlOutcome) bool {
	return value == ControlReleased || value == ControlComplete || value == ControlIncomplete || value == ControlDenied || value == ControlNotFound || value == ControlNotReleasable || value == ControlFailed
}
func validDenial(value DenialReason) bool {
	return value == DenialUnauthenticated || value == DenialForbidden
}
func validPredictor(value Predictor) bool {
	return value == PredictorCandidate || value == PredictorBaseline
}
func validPredictionOutcome(value PredictionOutcome) bool {
	return value == PredictionPredicted || value == PredictionAbstained
}
func validFreshness(value Freshness) bool {
	return value == FreshnessFresh || value == FreshnessStale || value == FreshnessUnavailable
}
func validPythonOutcome(value PythonOutcome) bool {
	return value == PythonAvailable || value == PythonUnavailable || value == PythonTimeout || value == PythonInvalidResponse
}
func validHTTPResult(value string) bool {
	return value == "success" || value == "client_error" || value == "server_error"
}
func validGauge(value Gauge) bool {
	for _, v := range gauges {
		if v == value {
			return true
		}
	}
	return false
}

type correlationKey struct{}

// NewCorrelationID returns a UUIDv4-style opaque correlation ID. It is never
// derived from an HTTP header, session, event, or other caller-supplied value.
func NewCorrelationID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}

// WithCorrelationID stores a generated correlation ID in ctx. Invalid values
// are discarded rather than becoming log fields.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	if !validCorrelationID(id) {
		return ctx
	}
	return context.WithValue(ctx, correlationKey{}, id)
}

// CorrelationID returns the generated correlation ID stored in ctx, if any.
func CorrelationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(correlationKey{}).(string)
	if !validCorrelationID(id) {
		return ""
	}
	return id
}

// EnsureCorrelationID preserves a generated ID in ctx or creates one.
func EnsureCorrelationID(ctx context.Context) (context.Context, string, error) {
	if id := CorrelationID(ctx); id != "" {
		return ctx, id, nil
	}
	id, err := NewCorrelationID()
	if err != nil {
		return ctx, "", err
	}
	return WithCorrelationID(ctx, id), id, nil
}

func validCorrelationID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// Event is a stable, machine-readable structured-log event name.
type Event string

const (
	EventHTTPCompleted       Event = "http.request.completed"
	EventAuthorizationDenied Event = "authorization.denied"
	EventRelayCycle          Event = "relay.cycle.completed"
	EventRelayFailed         Event = "relay.cycle.failed"
	EventConsumerCycle       Event = "consumer.cycle.completed"
	EventConsumerFailed      Event = "consumer.cycle.failed"
	EventControlCompleted    Event = "ops.control.completed"
	EventForecastCompleted   Event = "forecast.command.completed"
	EventForecastFailed      Event = "forecast.command.failed"
	EventWorkerRetrying      Event = "worker.retrying"
)

// Fields permits only bounded values or validated checksum identity in a log
// record. It intentionally has no generic map or error-string field.
type Fields struct {
	Route           Route
	Reason          DenialReason
	Operation       ControlOperation
	Worker          Worker
	Outcome         string
	StatusCode      int
	Duration        time.Duration
	Checksum        string
	LineageChecksum string
	Predictor       Predictor
	Freshness       Freshness
}

// Log emits one safe structured record. It never emits a raw request, error,
// token, cookie, identifier, payload, feature row, or model artifact.
func Log(ctx context.Context, logger *slog.Logger, event Event, fields Fields) {
	if logger == nil {
		logger = slog.Default()
	}
	attrs := make([]slog.Attr, 0, 8)
	if id := CorrelationID(ctx); id != "" {
		attrs = append(attrs, slog.String("correlation_id", id))
	}
	if validRoute(fields.Route) {
		attrs = append(attrs, slog.String("route", string(fields.Route)))
	}
	if validDenial(fields.Reason) {
		attrs = append(attrs, slog.String("reason", string(fields.Reason)))
	}
	if validControlOperation(fields.Operation) {
		attrs = append(attrs, slog.String("operation", string(fields.Operation)))
	}
	if validWorker(fields.Worker) {
		attrs = append(attrs, slog.String("worker", string(fields.Worker)))
	}
	if validHTTPResult(fields.Outcome) || validControlOutcome(ControlOutcome(fields.Outcome)) || validRelayOutcome(RelayPublishOutcome(fields.Outcome)) || validConsumerProcessOutcome(ConsumerProcessOutcome(fields.Outcome)) || validConsumerAckOutcome(ConsumerAckOutcome(fields.Outcome)) || validPredictionOutcome(PredictionOutcome(fields.Outcome)) || validPythonOutcome(PythonOutcome(fields.Outcome)) {
		attrs = append(attrs, slog.String("outcome", fields.Outcome))
	}
	if fields.StatusCode >= 100 && fields.StatusCode <= 599 {
		attrs = append(attrs, slog.Int("status_code", fields.StatusCode))
	}
	if fields.Duration >= 0 {
		attrs = append(attrs, slog.Int64("duration_ms", fields.Duration.Milliseconds()))
	}
	if validChecksum(fields.Checksum) {
		attrs = append(attrs, slog.String("checksum", fields.Checksum))
	}
	if validChecksum(fields.LineageChecksum) {
		attrs = append(attrs, slog.String("lineage_checksum", fields.LineageChecksum))
	}
	if validPredictor(fields.Predictor) {
		attrs = append(attrs, slog.String("predictor", string(fields.Predictor)))
	}
	if validFreshness(fields.Freshness) {
		attrs = append(attrs, slog.String("freshness", string(fields.Freshness)))
	}
	logger.LogAttrs(ctx, slog.LevelInfo, string(event), attrs...)
}

func validChecksum(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
