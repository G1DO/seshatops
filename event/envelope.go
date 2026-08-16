package event

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxSafeInt = int64(9007199254740991) // 2^53 - 1

var (
	// UUIDv4 with RFC 4122 variant bits (docs/design/specifications/event-spine.md event_id rule).
	uuidV4Regexp = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	timeZRegexp  = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$`)
)

// Envelope is the Event Spine v1 event envelope. Timestamps keep their exact source
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
	Payload            Payload
}

// Validate checks envelope and payload invariants without parsing JSON.
func Validate(env Envelope) error {
	if err := requireUTF8(env.EventID, "event_id"); err != nil {
		return err
	}
	if err := requireUTF8(env.TenantID, "tenant_id"); err != nil {
		return err
	}
	if err := requireUTF8(env.EventType, "event_type"); err != nil {
		return err
	}
	if err := requireUTF8(env.AggregateType, "aggregate_type"); err != nil {
		return err
	}
	if err := requireUTF8(env.AggregateID, "aggregate_id"); err != nil {
		return err
	}
	if err := requireUTF8(env.OccurredAt, "occurred_at"); err != nil {
		return err
	}
	if err := requireUTF8(env.RecordedAt, "recorded_at"); err != nil {
		return err
	}
	if err := requireUTF8(env.Producer, "producer"); err != nil {
		return err
	}
	if err := requireUTF8(env.CorrelationID, "correlation_id"); err != nil {
		return err
	}
	if env.CausationID != nil {
		if err := requireUTF8(*env.CausationID, "causation_id"); err != nil {
			return err
		}
	}
	if err := requireUTF8(env.TraceID, "trace_id"); err != nil {
		return err
	}
	if err := requireUUID(env.EventID, "event_id"); err != nil {
		return err
	}
	if err := requireLowerUUID(env.TenantID, "tenant_id"); err != nil {
		return err
	}
	wantAgg, err := familyAggregateType(env.EventType, env.EventSchemaVersion)
	if err != nil {
		return err
	}
	if env.AggregateType != wantAgg {
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
	if env.Payload == nil {
		return fmt.Errorf("%w: missing payload", ErrMalformed)
	}
	return env.Payload.validateAgainst(env)
}

func requireUTF8(v, field string) error {
	if !utf8.ValidString(v) {
		return fmt.Errorf("%w: %s is not valid UTF-8", ErrMalformed, field)
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
