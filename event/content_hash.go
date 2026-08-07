package event

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// CanonicalBytes returns the RFC 8785 JCS UTF-8 serialization of a validated envelope.
func CanonicalBytes(env Envelope) ([]byte, error) {
	if err := Validate(env); err != nil {
		return nil, err
	}
	return []byte(jcsObject(envelopeMap(env))), nil
}

// ContentHash returns the lowercase hex SHA-256 of CanonicalBytes(env).
func ContentHash(env Envelope) (string, error) {
	b, err := CanonicalBytes(env)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// CheckIdentityConflict reports ErrIdentityConflict when two envelopes share
// event_id but differ in canonical content.
func CheckIdentityConflict(a, b Envelope) error {
	if a.EventID != b.EventID {
		return nil
	}
	ha, err := ContentHash(a)
	if err != nil {
		return err
	}
	hb, err := ContentHash(b)
	if err != nil {
		return err
	}
	if ha != hb {
		return fmt.Errorf("%w: event_id %s", ErrIdentityConflict, a.EventID)
	}
	return nil
}

func envelopeMap(env Envelope) map[string]any {
	var causation any
	if env.CausationID == nil {
		causation = nil
	} else {
		causation = *env.CausationID
	}
	return map[string]any{
		"event_id":             env.EventID,
		"tenant_id":            env.TenantID,
		"event_type":           env.EventType,
		"event_schema_version": env.EventSchemaVersion,
		"aggregate_type":       env.AggregateType,
		"aggregate_id":         env.AggregateID,
		"aggregate_version":    env.AggregateVersion,
		"occurred_at":          env.OccurredAt,
		"recorded_at":          env.RecordedAt,
		"producer":             env.Producer,
		"correlation_id":       env.CorrelationID,
		"causation_id":         causation,
		"trace_id":             env.TraceID,
		"payload": map[string]any{
			"order_id":             env.Payload.OrderID,
			"item_id":              env.Payload.ItemID,
			"quantity_decremented": env.Payload.QuantityDecremented,
			"quantity_before":      env.Payload.QuantityBefore,
			"quantity_after":       env.Payload.QuantityAfter,
		},
	}
}

func jcsValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return jcsString(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case map[string]any:
		return jcsObject(t)
	default:
		panic(fmt.Sprintf("event: unsupported JCS value %T", v))
	}
}

func jcsObject(obj map[string]any) string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return lessUTF16(keys[i], keys[j])
	})
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(jcsString(k))
		b.WriteByte(':')
		b.WriteString(jcsValue(obj[k]))
	}
	b.WriteByte('}')
	return b.String()
}

func lessUTF16(a, b string) bool {
	aa := utf16.Encode([]rune(a))
	bb := utf16.Encode([]rune(b))
	n := len(aa)
	if len(bb) < n {
		n = len(bb)
	}
	for i := 0; i < n; i++ {
		if aa[i] != bb[i] {
			return aa[i] < bb[i]
		}
	}
	return len(aa) < len(bb)
}

func jcsString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"', r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
