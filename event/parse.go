package event

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Parse strictly decodes UTF-8 JSON into a validated Event Spine envelope.
// Duplicate object member names, unknown fields, and non-integer numerics are rejected.
func Parse(raw []byte) (Envelope, error) {
	root, err := decodeStrict(raw)
	if err != nil {
		return Envelope{}, err
	}
	obj, ok := root.(map[string]any)
	if !ok {
		return Envelope{}, fmt.Errorf("%w: root must be an object", ErrMalformed)
	}
	env, err := envelopeFromObject(obj)
	if err != nil {
		return Envelope{}, err
	}
	if err := Validate(env); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

func decodeStrict(raw []byte) (any, error) {
	if err := ensureJCSCompatibleJSON(raw); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("%w: trailing JSON content", ErrMalformed)
		}
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return decodeObject(dec)
		case '[':
			return nil, fmt.Errorf("arrays are not permitted in the Event Spine event contract")
		default:
			return nil, fmt.Errorf("unexpected delimiter %q", t)
		}
	case bool:
		return nil, fmt.Errorf("booleans are not permitted in the Event Spine event contract")
	case string:
		return t, nil
	case json.Number:
		return parseIntegerNumber(t)
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported JSON token %T", tok)
	}
}

func decodeObject(dec *json.Decoder) (map[string]any, error) {
	out := make(map[string]any)
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("object key must be a string")
		}
		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("duplicate object member %q", key)
		}
		val, err := decodeValue(dec)
		if err != nil {
			return nil, err
		}
		out[key] = val
	}
	end, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if end != json.Delim('}') {
		return nil, fmt.Errorf("expected end of object")
	}
	return out, nil
}

func parseIntegerNumber(n json.Number) (int64, error) {
	s := n.String()
	if strings.ContainsAny(s, ".eE") {
		return 0, fmt.Errorf("non-integer number %q", s)
	}
	if len(s) > 1 && (s[0] == '0' || (s[0] == '-' && len(s) > 2 && s[1] == '0')) {
		return 0, fmt.Errorf("leading zeros are not permitted in %q", s)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("integer out of range %q", s)
	}
	if v < 0 || v > maxSafeInt {
		return 0, fmt.Errorf("integer out of allowed range %d", v)
	}
	return v, nil
}

func envelopeFromObject(obj map[string]any) (Envelope, error) {
	required := []string{
		"event_id", "tenant_id", "event_type", "event_schema_version",
		"aggregate_type", "aggregate_id", "aggregate_version",
		"occurred_at", "recorded_at", "producer",
		"correlation_id", "causation_id", "trace_id", "payload",
	}
	for _, k := range required {
		if _, ok := obj[k]; !ok {
			return Envelope{}, fmt.Errorf("%w: missing %s", ErrMalformed, k)
		}
	}
	for k := range obj {
		switch k {
		case "event_id", "tenant_id", "event_type", "event_schema_version",
			"aggregate_type", "aggregate_id", "aggregate_version",
			"occurred_at", "recorded_at", "producer",
			"correlation_id", "causation_id", "trace_id", "payload":
		default:
			return Envelope{}, fmt.Errorf("%w: unknown field %q", ErrMalformed, k)
		}
	}

	var env Envelope
	var err error
	if env.EventID, err = asString(obj["event_id"], "event_id"); err != nil {
		return Envelope{}, err
	}
	if env.TenantID, err = asString(obj["tenant_id"], "tenant_id"); err != nil {
		return Envelope{}, err
	}
	if env.EventType, err = asString(obj["event_type"], "event_type"); err != nil {
		return Envelope{}, err
	}
	if env.EventSchemaVersion, err = asInt(obj["event_schema_version"], "event_schema_version"); err != nil {
		return Envelope{}, err
	}
	if env.AggregateType, err = asString(obj["aggregate_type"], "aggregate_type"); err != nil {
		return Envelope{}, err
	}
	if env.AggregateID, err = asString(obj["aggregate_id"], "aggregate_id"); err != nil {
		return Envelope{}, err
	}
	if env.AggregateVersion, err = asInt(obj["aggregate_version"], "aggregate_version"); err != nil {
		return Envelope{}, err
	}
	if env.OccurredAt, err = asString(obj["occurred_at"], "occurred_at"); err != nil {
		return Envelope{}, err
	}
	if env.RecordedAt, err = asString(obj["recorded_at"], "recorded_at"); err != nil {
		return Envelope{}, err
	}
	if env.Producer, err = asString(obj["producer"], "producer"); err != nil {
		return Envelope{}, err
	}
	if env.CorrelationID, err = asString(obj["correlation_id"], "correlation_id"); err != nil {
		return Envelope{}, err
	}
	if env.CausationID, err = asNullableString(obj["causation_id"], "causation_id"); err != nil {
		return Envelope{}, err
	}
	if env.TraceID, err = asString(obj["trace_id"], "trace_id"); err != nil {
		return Envelope{}, err
	}

	payloadObj, ok := obj["payload"].(map[string]any)
	if !ok {
		return Envelope{}, fmt.Errorf("%w: payload must be an object", ErrMalformed)
	}
	payload, err := payloadFromObject(payloadObj)
	if err != nil {
		return Envelope{}, err
	}
	env.Payload = payload
	return env, nil
}

func payloadFromObject(obj map[string]any) (QuantityDecremented, error) {
	required := []string{
		"order_id", "item_id", "quantity_decremented", "quantity_before", "quantity_after",
	}
	for _, k := range required {
		if _, ok := obj[k]; !ok {
			return QuantityDecremented{}, fmt.Errorf("%w: missing payload.%s", ErrMalformed, k)
		}
	}
	for k := range obj {
		switch k {
		case "order_id", "item_id", "quantity_decremented", "quantity_before", "quantity_after":
		default:
			return QuantityDecremented{}, fmt.Errorf("%w: unknown payload field %q", ErrMalformed, k)
		}
	}
	var p QuantityDecremented
	var err error
	if p.OrderID, err = asString(obj["order_id"], "payload.order_id"); err != nil {
		return QuantityDecremented{}, err
	}
	if p.ItemID, err = asString(obj["item_id"], "payload.item_id"); err != nil {
		return QuantityDecremented{}, err
	}
	if p.QuantityDecremented, err = asInt(obj["quantity_decremented"], "payload.quantity_decremented"); err != nil {
		return QuantityDecremented{}, err
	}
	if p.QuantityBefore, err = asInt(obj["quantity_before"], "payload.quantity_before"); err != nil {
		return QuantityDecremented{}, err
	}
	if p.QuantityAfter, err = asInt(obj["quantity_after"], "payload.quantity_after"); err != nil {
		return QuantityDecremented{}, err
	}
	return p, nil
}

func asString(v any, field string) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%w: %s must be a string", ErrMalformed, field)
	}
	return s, nil
}

func asNullableString(v any, field string) (*string, error) {
	if v == nil {
		return nil, nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("%w: %s must be a string or null", ErrMalformed, field)
	}
	return &s, nil
}

func asInt(v any, field string) (int64, error) {
	n, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("%w: %s must be an integer", ErrMalformed, field)
	}
	return n, nil
}
