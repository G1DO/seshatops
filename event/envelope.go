package event

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// M1 fixed contract values from CONTRACTS.md.
const (
	EventTypeQuantityDecremented = "inventory.quantity_decremented"
	AggregateTypeInventoryItem   = "inventory_item"
	ProducerSyntheticERP         = "synthetic-erp"
	SchemaVersionV1              = int64(1)
)

const maxSafeInt = int64(9007199254740991) // 2^53 - 1

var (
	// UUIDv4 with RFC 4122 variant bits (CONTRACTS.md event_id rule).
	uuidV4Regexp = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	timeZRegexp  = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$`)
)

// Envelope is the M1 v1 event envelope. Timestamps keep their exact source
// strings. CausationID is nil when the JSON value is null.
type Envelope struct {
	EventID            string
	TenantID           string
	EventType          string
	EventSchemaVersion int64
	AggregateType      string
	AggregateID        string
	AggregateVersion   int64
	OccurredAt         string
	RecordedAt         string
	Producer           string
	CorrelationID      string
	CausationID        *string
	TraceID            string
	Payload            QuantityDecremented
}

// QuantityDecremented is the inventory.quantity_decremented v1 payload.
type QuantityDecremented struct {
	OrderID             string
	ItemID              string
	QuantityDecremented int64
	QuantityBefore      int64
	QuantityAfter       int64
}

// Validate checks envelope and payload invariants without parsing JSON.
func Validate(env Envelope) error {
	if err := requireUUID(env.EventID, "event_id"); err != nil {
		return err
	}
	if err := requireLowerUUID(env.TenantID, "tenant_id"); err != nil {
		return err
	}
	if env.EventType != EventTypeQuantityDecremented {
		return fmt.Errorf("%w: event_type %q", ErrUnsupported, env.EventType)
	}
	if env.EventSchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("%w: event_schema_version %d", ErrUnsupported, env.EventSchemaVersion)
	}
	if env.AggregateType != AggregateTypeInventoryItem {
		return fmt.Errorf("%w: aggregate_type %q", ErrMalformed, env.AggregateType)
	}
	if err := requireCanonicalID(env.AggregateID, "aggregate_id"); err != nil {
		return err
	}
	if env.AggregateVersion < 1 || env.AggregateVersion > maxSafeInt {
		return fmt.Errorf("%w: aggregate_version %d", ErrMalformed, env.AggregateVersion)
	}
	if err := requireTimeZ(env.OccurredAt, "occurred_at"); err != nil {
		return err
	}
	if err := requireTimeZ(env.RecordedAt, "recorded_at"); err != nil {
		return err
	}
	if env.Producer != ProducerSyntheticERP {
		return fmt.Errorf("%w: producer %q", ErrMalformed, env.Producer)
	}
	if err := requireUUID(env.CorrelationID, "correlation_id"); err != nil {
		return err
	}
	if env.CausationID != nil {
		if err := requireUUID(*env.CausationID, "causation_id"); err != nil {
			return err
		}
	}
	if strings.TrimSpace(env.TraceID) == "" {
		return fmt.Errorf("%w: missing trace_id", ErrMalformed)
	}
	return validatePayload(env)
}

func validatePayload(env Envelope) error {
	p := env.Payload
	if err := requireUUID(p.OrderID, "payload.order_id"); err != nil {
		return err
	}
	if err := requireCanonicalID(p.ItemID, "payload.item_id"); err != nil {
		return err
	}
	if p.ItemID != env.AggregateID {
		return fmt.Errorf("%w: payload.item_id must equal aggregate_id", ErrMalformed)
	}
	if p.QuantityDecremented < 1 || p.QuantityDecremented > maxSafeInt {
		return fmt.Errorf("%w: quantity_decremented %d", ErrMalformed, p.QuantityDecremented)
	}
	if p.QuantityBefore < 0 || p.QuantityBefore > maxSafeInt {
		return fmt.Errorf("%w: quantity_before %d", ErrMalformed, p.QuantityBefore)
	}
	if p.QuantityAfter < 0 || p.QuantityAfter > maxSafeInt {
		return fmt.Errorf("%w: quantity_after %d", ErrMalformed, p.QuantityAfter)
	}
	if p.QuantityBefore-p.QuantityDecremented != p.QuantityAfter {
		return fmt.Errorf("%w: quantity arithmetic invariant failed", ErrMalformed)
	}
	return nil
}

func requireUUID(v, field string) error {
	if !uuidV4Regexp.MatchString(v) {
		return fmt.Errorf("%w: invalid %s", ErrMalformed, field)
	}
	return nil
}

func requireLowerUUID(v, field string) error {
	if err := requireUUID(v, field); err != nil {
		return err
	}
	if v != strings.ToLower(v) {
		return fmt.Errorf("%w: %s must be lowercase", ErrMalformed, field)
	}
	return nil
}

func requireCanonicalID(v, field string) error {
	if v == "" {
		return fmt.Errorf("%w: missing %s", ErrMalformed, field)
	}
	if v != strings.ToLower(v) {
		return fmt.Errorf("%w: %s must be lowercase", ErrMalformed, field)
	}
	for _, r := range v {
		if unicode.IsSpace(r) {
			return fmt.Errorf("%w: %s contains whitespace", ErrMalformed, field)
		}
	}
	return nil
}

func requireTimeZ(v, field string) error {
	if !timeZRegexp.MatchString(v) {
		return fmt.Errorf("%w: invalid %s", ErrMalformed, field)
	}
	if _, err := time.Parse(time.RFC3339Nano, v); err != nil {
		return fmt.Errorf("%w: invalid %s", ErrMalformed, field)
	}
	return nil
}
