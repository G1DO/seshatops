package event_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/G1DO/seshatops/event"
)

func TestParseValidV1(t *testing.T) {
	raw := readTestdata(t, "valid_v1.json")
	env, err := event.Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if env.EventType != event.EventTypeQuantityDecremented {
		t.Fatalf("event_type = %q", env.EventType)
	}
	if env.EventSchemaVersion != 1 || env.AggregateVersion != 1 {
		t.Fatalf("unexpected versions: schema=%d aggregate=%d", env.EventSchemaVersion, env.AggregateVersion)
	}
	if env.CausationID != nil {
		t.Fatalf("causation_id want nil, got %v", env.CausationID)
	}
	if env.Payload.QuantityAfter != 8 {
		t.Fatalf("quantity_after = %d", env.Payload.QuantityAfter)
	}
}

func TestParsePropertyOrderIndependentHash(t *testing.T) {
	a := []byte(`{"payload":{"quantity_after":8,"quantity_before":10,"quantity_decremented":2,"item_id":"item-flour-001","order_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a4"},"trace_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a3","causation_id":null,"correlation_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a2","producer":"synthetic-erp","recorded_at":"2026-08-07T09:00:00Z","occurred_at":"2026-08-07T09:00:00Z","aggregate_version":1,"aggregate_id":"item-flour-001","aggregate_type":"inventory_item","event_schema_version":1,"event_type":"inventory.quantity_decremented","tenant_id":"11111111-1111-4111-8111-111111111111","event_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1"}`)
	b := readTestdata(t, "valid_v1.json")

	ea, err := event.Parse(a)
	if err != nil {
		t.Fatalf("Parse a: %v", err)
	}
	eb, err := event.Parse(b)
	if err != nil {
		t.Fatalf("Parse b: %v", err)
	}
	ha, err := event.ContentHash(ea)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := event.ContentHash(eb)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("hashes differ:\n%s\n%s", ha, hb)
	}
	ca, err := event.CanonicalBytes(ea)
	if err != nil {
		t.Fatal(err)
	}
	want := readTestdata(t, "valid_v1.jcs")
	if !bytes.Equal(ca, want) {
		t.Fatalf("canonical bytes mismatch\ngot:  %s\nwant: %s", ca, want)
	}
	wantHash := strings.TrimSpace(string(readTestdata(t, "valid_v1.sha256")))
	if ha != wantHash {
		t.Fatalf("hash = %s, want %s", ha, wantHash)
	}
}

func TestCompatibilityRejects(t *testing.T) {
	base := string(readTestdata(t, "valid_v1.json"))
	cases := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{
			name:    "unknown_event_type",
			raw:     strings.Replace(base, `"inventory.quantity_decremented"`, `"inventory.quantity_incremented"`, 1),
			wantErr: event.ErrUnsupported,
		},
		{
			name:    "unsupported_schema_version",
			raw:     strings.Replace(base, `"event_schema_version": 1`, `"event_schema_version": 2`, 1),
			wantErr: event.ErrUnsupported,
		},
		{
			name:    "missing_tenant_id",
			raw:     removeJSONField(base, `"tenant_id": "11111111-1111-4111-8111-111111111111",`),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "unknown_field",
			raw:     strings.Replace(base, `"trace_id":`, `"extra":"nope","trace_id":`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "duplicate_member",
			raw:     `{"event_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1","event_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1"}`,
			wantErr: event.ErrMalformed,
		},
		{
			name:    "decimal_number",
			raw:     strings.Replace(base, `"quantity_decremented": 2`, `"quantity_decremented": 2.0`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "bad_arithmetic",
			raw:     strings.Replace(base, `"quantity_after": 8`, `"quantity_after": 7`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "uppercase_tenant",
			raw:     strings.Replace(base, `"tenant_id": "11111111-1111-4111-8111-111111111111"`, `"tenant_id": "11111111-1111-4111-8111-11111111111F"`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "invalid_aggregate_version_zero",
			raw:     strings.Replace(base, `"aggregate_version": 1`, `"aggregate_version": 0`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "item_id_mismatch",
			raw:     strings.Replace(base, `"item_id": "item-flour-001"`, `"item_id": "item-sugar-001"`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "non_z_timestamp",
			raw:     strings.Replace(base, `"2026-08-07T09:00:00Z"`, `"2026-08-07T09:00:00+00:00"`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "non_uuid_v4_event_id",
			raw:     strings.Replace(base, `"event_id": "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1"`, `"event_id": "018f5d78-6e64-1f5f-bd16-8e9f7c4a20a1"`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "invalid_uuid_variant",
			raw:     strings.Replace(base, `"event_id": "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1"`, `"event_id": "018f5d78-6e64-4f5f-cd16-8e9f7c4a20a1"`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "invalid_calendar_timestamp",
			raw:     strings.Replace(base, `"occurred_at": "2026-08-07T09:00:00Z"`, `"occurred_at": "2026-13-40T99:00:00Z"`, 1),
			wantErr: event.ErrMalformed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := event.Parse([]byte(tc.raw))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestIdentityConflict(t *testing.T) {
	raw := readTestdata(t, "valid_v1.json")
	a, err := event.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	b := a
	b.Payload.QuantityAfter = 7
	b.Payload.QuantityBefore = 9
	if err := event.CheckIdentityConflict(a, b); !errors.Is(err, event.ErrIdentityConflict) {
		t.Fatalf("err=%v", err)
	}
	if err := event.CheckIdentityConflict(a, a); err != nil {
		t.Fatalf("same content: %v", err)
	}
}

func TestRetryCanonicalStable(t *testing.T) {
	env, err := event.Parse(readTestdata(t, "valid_v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := event.CanonicalBytes(env)
	if err != nil {
		t.Fatal(err)
	}
	again, err := event.Parse(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := event.CanonicalBytes(again)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("retry changed canonical bytes")
	}
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func removeJSONField(raw, field string) string {
	return strings.Replace(raw, field, "", 1)
}
