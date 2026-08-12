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
