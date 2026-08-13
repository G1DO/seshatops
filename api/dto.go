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
	IncompleteReasons []string `json:"incomplete_reasons,omitempty"`
}

// ControlDecision is an in-process authorization decision for a privileged
// control. Issue #49 may persist it; this issue does not.
type ControlDecision struct {
	PrincipalID string `json:"principal_id"`
	TenantID    string `json:"tenant_id"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Outcome     string `json:"outcome"`
	Reason      string `json:"reason"`
	TargetID    string `json:"target_id,omitempty"`
	At          string `json:"at"`
}
