package api

// InventoryItem is one committed projection row exposed to clients.
type InventoryItem struct {
	ItemID           string `json:"item_id"`
	QuantityOnHand   int64  `json:"quantity_on_hand"`
	AggregateVersion int64  `json:"aggregate_version"`
}

// InventorySnapshot is the authoritative REST projection snapshot for a tenant.
type InventorySnapshot struct {
	TenantID   string          `json:"tenant_id"`
	Items      []InventoryItem `json:"items"`
	Checksum   string          `json:"checksum"`
	ObservedAt string          `json:"observed_at"`
}

// ProjectionUpdated is the SSE payload for event inventory_projection.updated.
type ProjectionUpdated struct {
	TenantID           string `json:"tenant_id"`
	ItemID             string `json:"item_id"`
	QuantityOnHand     int64  `json:"quantity_on_hand"`
	AggregateVersion   int64  `json:"aggregate_version"`
	LastAppliedEventID string `json:"last_applied_event_id"`
	Checksum           string `json:"checksum"`
}

// ErrorBody is a minimal JSON error response.
type ErrorBody struct {
	Error string `json:"error"`
}

// ForecastPredictionSnapshot is the authorized current advisory assessment
// for one tenant-owned inventory resource. It contains prediction output and
// traceable lineage, but no raw feature values or model artifacts.
type ForecastPredictionSnapshot struct {
	TenantID            string                         `json:"tenant_id"`
	ResourceType        string                         `json:"resource_type"`
	ResourceID          string                         `json:"resource_id"`
	PredictionID        string                         `json:"prediction_id"`
	ObservationDate     string                         `json:"observation_date"`
	ForecastHorizonDays int                            `json:"forecast_horizon_days"`
	Target              string                         `json:"target"`
	Status              string                         `json:"status"`
	StockoutRisk        *float64                       `json:"stockout_risk"`
	Uncertainty         *ForecastPredictionUncertainty `json:"uncertainty"`
	AbstentionReason    string                         `json:"abstention_reason"`
	Freshness           ForecastPredictionFreshness    `json:"freshness"`
	Lineage             ForecastPredictionLineage      `json:"lineage"`
	CorrelationID       string                         `json:"correlation_id"`
	RecordedAt          string                         `json:"recorded_at"`
	ObservedAt          string                         `json:"observed_at"`
}

// ForecastPredictionUncertainty is the validated interval and support behind
// a predicted risk score.
type ForecastPredictionUncertainty struct {
	Method      string  `json:"method"`
	Lower       float64 `json:"lower"`
	Upper       float64 `json:"upper"`
	SampleCount int     `json:"sample_count"`
}

// ForecastPredictionFreshness makes source freshness explicit to clients.
// unavailable means Go could not establish a current complete source
// snapshot; clients must not present the stored result as current.
type ForecastPredictionFreshness struct {
	Status  string `json:"status"`
	FreshAt string `json:"fresh_at"`
}

// ForecastPredictionLineage identifies the immutable data, predictor, model,
// and code versions used for the result.
type ForecastPredictionLineage struct {
	EvaluationProtocolVersion string `json:"evaluation_protocol_version"`
	DatasetVersion            string `json:"dataset_version"`
	FeatureDefinitionVersion  string `json:"feature_definition_version"`
	FeatureSnapshotID         string `json:"feature_snapshot_id"`
	FeatureSnapshotChecksum   string `json:"feature_snapshot_checksum"`
	SourceCutoffDate          string `json:"source_cutoff_date"`
	Predictor                 string `json:"predictor"`
	ModelVersion              string `json:"model_version"`
	CodeVersion               string `json:"code_version"`
}

// OpsSnapshot is the authorized lag/poison/freshness view for one tenant.
type OpsSnapshot struct {
	TenantID   string        `json:"tenant_id"`
	ObservedAt string        `json:"observed_at"`
	Projection OpsProjection `json:"projection"`
	Backlog    OpsBacklog    `json:"backlog"`
	Processing OpsProcessing `json:"processing"`
}

// OpsProjection is projection freshness metadata (not inventory quantities).
type OpsProjection struct {
	Checksum  string `json:"checksum"`
	ItemCount int    `json:"item_count"`
}

// OpsBacklog is tenant-scoped outbox lag and quarantine visibility.
type OpsBacklog struct {
	Pending           int                   `json:"pending"`
	Publishing        int                   `json:"publishing"`
	Published         int                   `json:"published"`
	Quarantined       int                   `json:"quarantined"`
	OldestUnpublished *string               `json:"oldest_unpublished"`
	Quarantines       []OpsQuarantineSample `json:"quarantines"`
}

// OpsQuarantineSample is one sanitized outbox quarantine row.
type OpsQuarantineSample struct {
	EventID       string `json:"event_id"`
	LastErrorCode string `json:"last_error_code"`
	CreatedAt     string `json:"created_at"`
}

// OpsProcessing is tenant-scoped consumer poison/gap visibility.
type OpsProcessing struct {
	Applied               int                `json:"applied"`
	DuplicateNoop         int                `json:"duplicate_noop"`
	QuarantinedConflict   int                `json:"quarantined_conflict"`
	QuarantinedGap        int                `json:"quarantined_gap"`
	QuarantinedStale      int                `json:"quarantined_stale"`
	QuarantinedInvalid    int                `json:"quarantined_invalid"`
	QuarantinedMismatch   int                `json:"quarantined_mismatch"`
	QuarantinedTransition int                `json:"quarantined_transition"`
	FailuresRetrying      int                `json:"failures_retrying"`
	FailuresQuarantined   int                `json:"failures_quarantined"`
	OldestGap             *string            `json:"oldest_gap"`
	OldestFailure         *string            `json:"oldest_failure"`
	Failures              []OpsFailureSample `json:"failures"`
	Gaps                  []OpsGapSample     `json:"gaps"`
}

// OpsFailureSample is one sanitized processing_failures row.
type OpsFailureSample struct {
	FailureID        string `json:"failure_id"`
	EventID          string `json:"event_id"`
	TenantID         string `json:"tenant_id"`
	AggregateType    string `json:"aggregate_type"`
	AggregateID      string `json:"aggregate_id"`
	EventType        string `json:"event_type"`
	FailureCategory  string `json:"failure_category"`
	DiagnosticCode   string `json:"diagnostic_code"`
	QuarantineStatus string `json:"quarantine_status"`
	SourceTopic      string `json:"source_topic"`
	SourcePartition  int32  `json:"source_partition"`
	SourceOffset     int64  `json:"source_offset"`
	AttemptCount     int    `json:"attempt_count"`
	CreatedAt        string `json:"created_at"`
}

// OpsGapSample is one quarantined_gap inbox row without retained event bytes.
type OpsGapSample struct {
	EventID          string `json:"event_id"`
	TenantID         string `json:"tenant_id"`
	AggregateType    string `json:"aggregate_type"`
	AggregateID      string `json:"aggregate_id"`
	AggregateVersion int64  `json:"aggregate_version"`
	ExpectedVersion  int64  `json:"expected_version"`
	ReceivedVersion  int64  `json:"received_version"`
	CreatedAt        string `json:"created_at"`
}

// ControlRequest is the JSON body for privileged ops POSTs.
// TenantID, if present, is ignored; the path tenant is the assertion.
type ControlRequest struct {
	EventID  string `json:"event_id"`
	TenantID string `json:"tenant_id"`
}

// ControlResult is the authorized outcome of a privileged control.
type ControlResult struct {
	TenantID          string   `json:"tenant_id"`
	Control           string   `json:"control"`
	EventID           string   `json:"event_id,omitempty"`
	Status            string   `json:"status"`
	Disposition       string   `json:"disposition,omitempty"`
	Applied           int      `json:"applied"`
	DuplicateNoop     int      `json:"duplicate_noop"`
	Quarantined       int      `json:"quarantined"`
	Checksum          string   `json:"checksum,omitempty"`
	LineageChecksum   string   `json:"lineage_checksum,omitempty"`
	IncompleteReasons []string `json:"incomplete_reasons,omitempty"`
}

// ControlDecision is a privileged authorization decision. Issue #49 persists
// it append-only from the Go-owned session principal and path tenant.
type ControlDecision struct {
	DecisionID  string `json:"decision_id,omitempty"`
	PrincipalID string `json:"principal_id"`
	TenantID    string `json:"tenant_id"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Outcome     string `json:"outcome"`
	Reason      string `json:"reason"`
	TargetID    string `json:"target_id,omitempty"`
	At          string `json:"at"`
}

// AuditSnapshot is the authorized same-tenant privileged-decision timeline.
type AuditSnapshot struct {
	TenantID   string            `json:"tenant_id"`
	ObservedAt string            `json:"observed_at"`
	Records    []ControlDecision `json:"records"`
}

// LineageHop is one projected lineage node plus source provenance.
type LineageHop struct {
	ID                 string  `json:"id"`
	ParentID           string  `json:"parent_id"`
	ItemID             string  `json:"item_id"`
	OrderID            string  `json:"order_id"`
	AggregateVersion   int64   `json:"aggregate_version"`
	SourceEventID      string  `json:"source_event_id"`
	EventSchemaVersion int64   `json:"event_schema_version"`
	OccurredAt         string  `json:"occurred_at"`
	RecordedAt         string  `json:"recorded_at"`
	CorrelationID      string  `json:"correlation_id"`
	CausationID        *string `json:"causation_id"`
	TraceID            string  `json:"trace_id"`
}

// BatchTraceSnapshot is the authorized supplier → order chain for one batch.
type BatchTraceSnapshot struct {
	TenantID   string     `json:"tenant_id"`
	ObservedAt string     `json:"observed_at"`
	Supplier   LineageHop `json:"supplier"`
	Lot        LineageHop `json:"lot"`
	Batch      LineageHop `json:"batch"`
	Shipment   LineageHop `json:"shipment"`
}
