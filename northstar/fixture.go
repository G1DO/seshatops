package northstar

import "github.com/G1DO/seshatops/event"

// DefaultSeed is the declared Event Spine Northstar Foods order-line workload seed.
const DefaultSeed = "northstar-m1-order-line-v1"

// LineageSeed is the declared Northstar Foods supplier-to-order lineage chain.
const LineageSeed = "northstar-m3-lineage-v1"

// Fixture is the deterministic synthetic order/item/inventory workload for Event Spine.
type Fixture struct {
	Seed           string
	TenantID       string
	ItemID         string
	OrderID        string
	QuantityBefore int64
	QuantityAfter  int64
	Event          event.Envelope
}

// LineageFixture is one complete synthetic supplier → order chain.
type LineageFixture struct {
	Seed       string
	TenantID   string
	SupplierID string
	LotID      string
	BatchID    string
	ShipmentID string
	OrderID    string
	ItemID     string
	Events     []event.Envelope
}

// Generate returns the fixed Northstar Foods Event Spine fixture for the declared seed.
// Only DefaultSeed is supported in this slice; empty seed is rejected.
func Generate(seed string) (Fixture, error) {
	if seed != DefaultSeed {
		return Fixture{}, errUnsupportedSeed(seed)
	}

	env := event.Envelope{
		EventID:            "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1",
		TenantID:           "11111111-1111-4111-8111-111111111111",
		EventType:          event.EventTypeQuantityDecremented,
		EventSchemaVersion: event.SchemaVersionV1,
		AggregateType:      event.AggregateTypeInventoryItem,
		AggregateID:        "item-flour-001",
		AggregateVersion:   1,
		OccurredAt:         "2026-08-07T09:00:00Z",
		RecordedAt:         "2026-08-07T09:00:00Z",
		Producer:           event.ProducerSyntheticERP,
		CorrelationID:      "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a2",
		CausationID:        nil,
		TraceID:            "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a3",
		Payload: event.QuantityDecremented{
			OrderID:             "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a4",
			ItemID:              "item-flour-001",
			QuantityDecremented: 2,
			QuantityBefore:      10,
			QuantityAfter:       8,
		},
	}
	if err := event.Validate(env); err != nil {
		return Fixture{}, err
	}
	p, ok := event.AsQuantityDecremented(env)
	if !ok {
		return Fixture{}, errUnsupportedSeed(seed)
	}
	return Fixture{
		Seed:           seed,
		TenantID:       env.TenantID,
		ItemID:         env.AggregateID,
		OrderID:        p.OrderID,
		QuantityBefore: p.QuantityBefore,
		QuantityAfter:  p.QuantityAfter,
		Event:          env,
	}, nil
}

const (
	lineageTenantID      = "11111111-1111-4111-8111-111111111111"
	lineageCorrelationID = "218f5d78-6e64-4f5f-bd16-8e9f7c4a3001"
	lineageTraceID       = "218f5d78-6e64-4f5f-bd16-8e9f7c4a3002"
	lineageSupplierEvent = "218f5d78-6e64-4f5f-bd16-8e9f7c4a3011"
	lineageLotEvent      = "218f5d78-6e64-4f5f-bd16-8e9f7c4a3012"
	lineageBatchEvent    = "218f5d78-6e64-4f5f-bd16-8e9f7c4a3013"
	lineageShipEvent     = "218f5d78-6e64-4f5f-bd16-8e9f7c4a3014"
	lineageInvEvent      = "218f5d78-6e64-4f5f-bd16-8e9f7c4a3015"
	lineageOrderID       = "218f5d78-6e64-4f5f-bd16-8e9f7c4a3020"
	lineageSupplierID    = "mill-northstar-001"
	lineageLotID         = "lot-flour-2026-001"
	lineageBatchID       = "batch-bread-001"
	lineageShipmentID    = "ship-northstar-001"
	lineageItemID        = "item-flour-001"
)

// GenerateLineage returns the fixed Northstar Foods supplier-to-order chain.
func GenerateLineage(seed string) (LineageFixture, error) {
	if seed != LineageSeed {
		return LineageFixture{}, errUnsupportedSeed(seed)
	}

	supplierCausation := lineageSupplierEvent
	lotCausation := lineageLotEvent
	batchCausation := lineageBatchEvent
	shipCausation := lineageShipEvent

	events := []event.Envelope{
		{
			EventID:            lineageSupplierEvent,
			TenantID:           lineageTenantID,
			EventType:          event.EventTypeSupplierRegistered,
			EventSchemaVersion: event.SchemaVersionV1,
			AggregateType:      event.AggregateTypeSupplier,
			AggregateID:        lineageSupplierID,
			AggregateVersion:   1,
			OccurredAt:         "2026-08-07T10:00:00Z",
			RecordedAt:         "2026-08-07T10:00:00Z",
			Producer:           event.ProducerSyntheticERP,
			CorrelationID:      lineageCorrelationID,
			CausationID:        nil,
			TraceID:            lineageTraceID,
			Payload:            event.SupplierRegistered{SupplierID: lineageSupplierID},
		},
		{
			EventID:            lineageLotEvent,
			TenantID:           lineageTenantID,
			EventType:          event.EventTypeIngredientLotReceived,
			EventSchemaVersion: event.SchemaVersionV1,
			AggregateType:      event.AggregateTypeIngredientLot,
			AggregateID:        lineageLotID,
			AggregateVersion:   1,
			OccurredAt:         "2026-08-07T10:01:00Z",
			RecordedAt:         "2026-08-07T10:01:00Z",
			Producer:           event.ProducerSyntheticERP,
			CorrelationID:      lineageCorrelationID,
			CausationID:        &supplierCausation,
			TraceID:            lineageTraceID,
			Payload: event.IngredientLotReceived{
				LotID:      lineageLotID,
				SupplierID: lineageSupplierID,
				ItemID:     lineageItemID,
			},
		},
		{
			EventID:            lineageBatchEvent,
			TenantID:           lineageTenantID,
			EventType:          event.EventTypeProductionBatchProduced,
			EventSchemaVersion: event.SchemaVersionV1,
			AggregateType:      event.AggregateTypeProductionBatch,
			AggregateID:        lineageBatchID,
			AggregateVersion:   1,
			OccurredAt:         "2026-08-07T10:02:00Z",
			RecordedAt:         "2026-08-07T10:02:00Z",
			Producer:           event.ProducerSyntheticERP,
			CorrelationID:      lineageCorrelationID,
			CausationID:        &lotCausation,
			TraceID:            lineageTraceID,
			Payload: event.ProductionBatchProduced{
				BatchID: lineageBatchID,
				LotID:   lineageLotID,
			},
		},
		{
			EventID:            lineageShipEvent,
			TenantID:           lineageTenantID,
			EventType:          event.EventTypeShipmentDispatched,
			EventSchemaVersion: event.SchemaVersionV1,
			AggregateType:      event.AggregateTypeShipment,
			AggregateID:        lineageShipmentID,
			AggregateVersion:   1,
			OccurredAt:         "2026-08-07T10:03:00Z",
			RecordedAt:         "2026-08-07T10:03:00Z",
			Producer:           event.ProducerSyntheticERP,
			CorrelationID:      lineageCorrelationID,
			CausationID:        &batchCausation,
			TraceID:            lineageTraceID,
			Payload: event.ShipmentDispatched{
				ShipmentID: lineageShipmentID,
				BatchID:    lineageBatchID,
				OrderID:    lineageOrderID,
			},
		},
		{
			EventID:            lineageInvEvent,
			TenantID:           lineageTenantID,
			EventType:          event.EventTypeQuantityDecremented,
			EventSchemaVersion: event.SchemaVersionV1,
			AggregateType:      event.AggregateTypeInventoryItem,
			AggregateID:        lineageItemID,
			AggregateVersion:   1,
			OccurredAt:         "2026-08-07T10:04:00Z",
			RecordedAt:         "2026-08-07T10:04:00Z",
			Producer:           event.ProducerSyntheticERP,
			CorrelationID:      lineageCorrelationID,
			CausationID:        &shipCausation,
			TraceID:            lineageTraceID,
			Payload: event.QuantityDecremented{
				OrderID:             lineageOrderID,
				ItemID:              lineageItemID,
				QuantityDecremented: 2,
				QuantityBefore:      10,
				QuantityAfter:       8,
			},
		},
	}
	for i := range events {
		if err := event.Validate(events[i]); err != nil {
			return LineageFixture{}, err
		}
	}
	return LineageFixture{
		Seed:       seed,
		TenantID:   lineageTenantID,
		SupplierID: lineageSupplierID,
		LotID:      lineageLotID,
		BatchID:    lineageBatchID,
		ShipmentID: lineageShipmentID,
		OrderID:    lineageOrderID,
		ItemID:     lineageItemID,
		Events:     events,
	}, nil
}

type seedError string

func (e seedError) Error() string { return string(e) }

func errUnsupportedSeed(seed string) error {
	return seedError("northstar: unsupported seed " + seed)
}
