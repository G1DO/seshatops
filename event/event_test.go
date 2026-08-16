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
	p, ok := event.AsQuantityDecremented(env)
	if !ok {
		t.Fatal("payload is not inventory.quantity_decremented")
	}
	if p.QuantityAfter != 8 {
		t.Fatalf("quantity_after = %d", p.QuantityAfter)
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
	bp, ok := event.AsQuantityDecremented(b)
	if !ok {
		t.Fatal("payload is not inventory.quantity_decremented")
	}
	bp.QuantityAfter = 7
	bp.QuantityBefore = 9
	b.Payload = bp
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

func TestUnicodeJCSRejects(t *testing.T) {
	cases := []struct {
		name      string
		traceJSON []byte
	}{
		{
			name:      "invalid_utf8_byte",
			traceJSON: []byte{'"', 0xff, '"'},
		},
		{
			name:      "invalid_utf8_fe",
			traceJSON: []byte{'"', 0xfe, '"'},
		},
		{
			name:      "unpaired_high_surrogate",
			traceJSON: []byte(`"\ud800"`),
		},
		{
			name:      "unpaired_low_surrogate",
			traceJSON: []byte(`"\udc00"`),
		},
		{
			name:      "high_surrogate_not_followed_by_low",
			traceJSON: []byte(`"\ud800\u0041"`),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := event.Parse(envelopeWithTraceJSON(tc.traceJSON))
			if !errors.Is(err, event.ErrMalformed) {
				t.Fatalf("err=%v, want %v", err, event.ErrMalformed)
			}
		})
	}
}

func TestUnicodeJCSAccepts(t *testing.T) {
	cases := []struct {
		name      string
		traceJSON []byte
		wantTrace string
	}{
		{
			name:      "valid_surrogate_pair",
			traceJSON: []byte(`"\ud83d\ude00"`),
			wantTrace: "😀",
		},
		{
			name:      "normal_unicode",
			traceJSON: []byte("\"caf\u00e9\""),
			wantTrace: "café",
		},
		{
			name:      "literal_fffd",
			traceJSON: []byte(`"\ufffd"`),
			wantTrace: "\uFFFD",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, err := event.Parse(envelopeWithTraceJSON(tc.traceJSON))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if env.TraceID != tc.wantTrace {
				t.Fatalf("trace_id=%q, want %q", env.TraceID, tc.wantTrace)
			}
			if _, err := event.ContentHash(env); err != nil {
				t.Fatalf("ContentHash: %v", err)
			}
		})
	}
}

func TestUnicodeMalformedDoNotCollapse(t *testing.T) {
	a := envelopeWithTraceJSON([]byte{'"', 0xff, '"'})
	b := envelopeWithTraceJSON([]byte{'"', 0xfe, '"'})
	c := envelopeWithTraceJSON([]byte(`"\ud800"`))
	d := envelopeWithTraceJSON([]byte(`"\udfff"`))
	for i, raw := range [][]byte{a, b, c, d} {
		_, err := event.Parse(raw)
		if !errors.Is(err, event.ErrMalformed) {
			t.Fatalf("case %d: err=%v, want ErrMalformed", i, err)
		}
	}
}

func TestParseValidFamilies(t *testing.T) {
	cases := []struct {
		file      string
		eventType string
		aggType   string
		aggID     string
	}{
		{"supplier_registered_v1.json", event.EventTypeSupplierRegistered, event.AggregateTypeSupplier, "mill-northstar-001"},
		{"ingredient_lot_received_v1.json", event.EventTypeIngredientLotReceived, event.AggregateTypeIngredientLot, "lot-flour-2026-001"},
		{"production_batch_produced_v1.json", event.EventTypeProductionBatchProduced, event.AggregateTypeProductionBatch, "batch-bread-001"},
		{"shipment_dispatched_v1.json", event.EventTypeShipmentDispatched, event.AggregateTypeShipment, "ship-northstar-001"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			env, err := event.Parse(readTestdata(t, tc.file))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if env.EventType != tc.eventType {
				t.Fatalf("event_type = %q", env.EventType)
			}
			if env.AggregateType != tc.aggType || env.AggregateID != tc.aggID {
				t.Fatalf("aggregate = %s/%s", env.AggregateType, env.AggregateID)
			}
			if env.EventSchemaVersion != event.SchemaVersionV1 {
				t.Fatalf("schema = %d", env.EventSchemaVersion)
			}
		})
	}
}

func TestFamilyGoldens(t *testing.T) {
	files := []string{
		"supplier_registered_v1",
		"ingredient_lot_received_v1",
		"production_batch_produced_v1",
		"shipment_dispatched_v1",
	}
	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			env, err := event.Parse(readTestdata(t, name+".json"))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			got, err := event.CanonicalBytes(env)
			if err != nil {
				t.Fatal(err)
			}
			want := readTestdata(t, name+".jcs")
			if !bytes.Equal(got, want) {
				t.Fatalf("canonical bytes mismatch\ngot:  %s\nwant: %s", got, want)
			}
			hash, err := event.ContentHash(env)
			if err != nil {
				t.Fatal(err)
			}
			wantHash := strings.TrimSpace(string(readTestdata(t, name+".sha256")))
			if hash != wantHash {
				t.Fatalf("hash = %s, want %s", hash, wantHash)
			}
		})
	}
}

func TestFamilyCompatibilityRejects(t *testing.T) {
	supplier := string(readTestdata(t, "supplier_registered_v1.json"))
	shipment := string(readTestdata(t, "shipment_dispatched_v1.json"))
	inventory := string(readTestdata(t, "valid_v1.json"))
	lot := string(readTestdata(t, "ingredient_lot_received_v1.json"))
	cases := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{
			name:    "unknown_type_inventory_shaped_payload",
			raw:     strings.Replace(inventory, `"inventory.quantity_decremented"`, `"inventory.quantity_incremented"`, 1),
			wantErr: event.ErrUnsupported,
		},
		{
			name:    "supplier_schema_v2",
			raw:     strings.Replace(supplier, `"event_schema_version": 1`, `"event_schema_version": 2`, 1),
			wantErr: event.ErrUnsupported,
		},
		{
			name:    "supplier_unknown_payload_field",
			raw:     strings.Replace(supplier, `"supplier_id": "mill-northstar-001"`, `"supplier_id": "mill-northstar-001", "name": "nope"`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name: "supplier_missing_payload_field",
			raw: strings.Replace(supplier, `"payload": {
    "supplier_id": "mill-northstar-001"
  }`, `"payload": {}`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "supplier_id_mismatch",
			raw:     strings.Replace(supplier, `"supplier_id": "mill-northstar-001"`, `"supplier_id": "mill-other-001"`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "supplier_wrong_aggregate_type",
			raw:     strings.Replace(supplier, `"aggregate_type": "supplier"`, `"aggregate_type": "inventory_item"`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "supplier_uppercase_id",
			raw:     strings.Replace(supplier, `"aggregate_id": "mill-northstar-001"`, `"aggregate_id": "Mill-northstar-001"`, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name:    "lot_missing_parent",
			raw:     strings.Replace(lot, `"supplier_id": "mill-northstar-001",`, ``, 1),
			wantErr: event.ErrMalformed,
		},
		{
			name: "shipment_inventory_payload",
			raw: strings.Replace(shipment, `"payload": {
    "shipment_id": "ship-northstar-001",
    "batch_id": "batch-bread-001",
    "order_id": "418f5d78-6e64-4f5f-bd16-8e9f7c4a4120"
  }`, `"payload": {
    "order_id": "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a4",
    "item_id": "item-flour-001",
    "quantity_decremented": 2,
    "quantity_before": 10,
    "quantity_after": 8
  }`, 1),
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

func TestValidateRejectsInvalidUTF8TraceID(t *testing.T) {
	env, err := event.Parse(readTestdata(t, "valid_v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	env.TraceID = string([]byte{0xff})
	if err := event.Validate(env); !errors.Is(err, event.ErrMalformed) {
		t.Fatalf("Validate err=%v, want ErrMalformed", err)
	}
	if _, err := event.CanonicalBytes(env); !errors.Is(err, event.ErrMalformed) {
		t.Fatalf("CanonicalBytes err=%v, want ErrMalformed", err)
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

// envelopeWithTraceJSON builds a valid v1 envelope with the given JSON string
// token (including quotes) spliced into trace_id.
func envelopeWithTraceJSON(traceJSON []byte) []byte {
	prefix := []byte(`{"event_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1","tenant_id":"11111111-1111-4111-8111-111111111111","event_type":"inventory.quantity_decremented","event_schema_version":1,"aggregate_type":"inventory_item","aggregate_id":"item-flour-001","aggregate_version":1,"occurred_at":"2026-08-07T09:00:00Z","recorded_at":"2026-08-07T09:00:00Z","producer":"synthetic-erp","correlation_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a2","causation_id":null,"trace_id":`)
	suffix := []byte(`,"payload":{"order_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a4","item_id":"item-flour-001","quantity_decremented":2,"quantity_before":10,"quantity_after":8}}`)
	out := make([]byte, 0, len(prefix)+len(traceJSON)+len(suffix))
	out = append(out, prefix...)
	out = append(out, traceJSON...)
	out = append(out, suffix...)
	return out
}
