package northstar

import "github.com/G1DO/seshatops/event"

// DefaultSeed is the declared M1 Northstar Foods order-line workload seed.
const DefaultSeed = "northstar-m1-order-line-v1"

// Fixture is the deterministic synthetic order/item/inventory workload for M1.
type Fixture struct {
	Seed           string
	TenantID       string
	ItemID         string
	OrderID        string
	QuantityBefore int64
	QuantityAfter  int64
	Event          event.Envelope
}

// Generate returns the fixed Northstar Foods M1 fixture for the declared seed.
// Only DefaultSeed is supported in this slice.
func Generate(seed string) (Fixture, error) {
	if seed == "" {
		seed = DefaultSeed
	}
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
	return Fixture{
		Seed:           seed,
		TenantID:       env.TenantID,
		ItemID:         env.AggregateID,
		OrderID:        env.Payload.OrderID,
		QuantityBefore: env.Payload.QuantityBefore,
		QuantityAfter:  env.Payload.QuantityAfter,
		Event:          env,
	}, nil
}

type seedError string

func (e seedError) Error() string { return string(e) }

func errUnsupportedSeed(seed string) error {
	return seedError("northstar: unsupported seed " + seed)
}
