package northstar_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
)

func TestGenerateDeterministic(t *testing.T) {
	a, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	b, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	ha, err := event.ContentHash(a.Event)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := event.ContentHash(b.Event)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("hashes differ: %s vs %s", ha, hb)
	}
	ca, err := event.CanonicalBytes(a.Event)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := event.CanonicalBytes(b.Event)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ca, cb) {
		t.Fatal("canonical bytes differ across generations")
	}
}

func TestGenerateMatchesGolden(t *testing.T) {
	fx, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	got, err := event.CanonicalBytes(fx.Event)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "order_line_v1.jcs"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch\ngot:  %s\nwant: %s", got, want)
	}
	hash, err := event.ContentHash(fx.Event)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := strings.TrimSpace(string(mustRead(t, filepath.Join("testdata", "order_line_v1.sha256"))))
	if hash != wantHash {
		t.Fatalf("hash = %s, want %s", hash, wantHash)
	}
	if fx.ItemID != "item-flour-001" || fx.QuantityBefore != 10 || fx.QuantityAfter != 8 {
		t.Fatalf("unexpected fixture values: %+v", fx)
	}
}

func TestGenerateUnsupportedSeed(t *testing.T) {
	for _, seed := range []string{"", "other-seed", northstar.LineageSeed} {
		if _, err := northstar.Generate(seed); err == nil {
			t.Fatalf("expected error for seed %q", seed)
		}
	}
}

func TestGenerateLineageDeterministic(t *testing.T) {
	a, err := northstar.GenerateLineage(northstar.LineageSeed)
	if err != nil {
		t.Fatal(err)
	}
	b, err := northstar.GenerateLineage(northstar.LineageSeed)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Events) != 5 || len(b.Events) != 5 {
		t.Fatalf("event count a=%d b=%d", len(a.Events), len(b.Events))
	}
	for i := range a.Events {
		ha, err := event.ContentHash(a.Events[i])
		if err != nil {
			t.Fatal(err)
		}
		hb, err := event.ContentHash(b.Events[i])
		if err != nil {
			t.Fatal(err)
		}
		if ha != hb {
			t.Fatalf("event %d hashes differ", i)
		}
	}
}

func TestGenerateLineageMatchesGolden(t *testing.T) {
	fx, err := northstar.GenerateLineage(northstar.LineageSeed)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"lineage_supplier", "lineage_lot", "lineage_batch", "lineage_shipment", "lineage_inventory"}
	if len(fx.Events) != len(names) {
		t.Fatalf("events = %d", len(fx.Events))
	}
	for i, name := range names {
		got, err := event.CanonicalBytes(fx.Events[i])
		if err != nil {
			t.Fatal(err)
		}
		want := mustRead(t, filepath.Join("testdata", name+".jcs"))
		if !bytes.Equal(got, want) {
			t.Fatalf("%s mismatch\ngot:  %s\nwant: %s", name, got, want)
		}
		hash, err := event.ContentHash(fx.Events[i])
		if err != nil {
			t.Fatal(err)
		}
		wantHash := strings.TrimSpace(string(mustRead(t, filepath.Join("testdata", name+".sha256"))))
		if hash != wantHash {
			t.Fatalf("%s hash = %s, want %s", name, hash, wantHash)
		}
	}
}

func TestGenerateLineageChain(t *testing.T) {
	fx, err := northstar.GenerateLineage(northstar.LineageSeed)
	if err != nil {
		t.Fatal(err)
	}
	if fx.TenantID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("tenant = %s", fx.TenantID)
	}
	if fx.ItemID != "item-flour-001" || fx.SupplierID != "mill-northstar-001" {
		t.Fatalf("ids: %+v", fx)
	}
	wantTypes := []string{
		event.EventTypeSupplierRegistered,
		event.EventTypeIngredientLotReceived,
		event.EventTypeProductionBatchProduced,
		event.EventTypeShipmentDispatched,
		event.EventTypeQuantityDecremented,
	}
	corr := fx.Events[0].CorrelationID
	trace := fx.Events[0].TraceID
	if fx.Events[0].CausationID != nil {
		t.Fatal("supplier causation_id must be null")
	}
	for i, env := range fx.Events {
		if env.EventType != wantTypes[i] {
			t.Fatalf("event %d type = %s", i, env.EventType)
		}
		if env.TenantID != fx.TenantID || env.CorrelationID != corr || env.TraceID != trace {
			t.Fatalf("event %d lineage fields diverged", i)
		}
		if i > 0 {
			if env.CausationID == nil || *env.CausationID != fx.Events[i-1].EventID {
				t.Fatalf("event %d causation_id = %v, want %s", i, env.CausationID, fx.Events[i-1].EventID)
			}
		}
	}
	ship, ok := fx.Events[3].Payload.(event.ShipmentDispatched)
	if !ok {
		t.Fatal("shipment payload")
	}
	inv, ok := event.AsQuantityDecremented(fx.Events[4])
	if !ok {
		t.Fatal("inventory payload")
	}
	if ship.OrderID != fx.OrderID || inv.OrderID != fx.OrderID {
		t.Fatalf("order_id mismatch ship=%s inv=%s fx=%s", ship.OrderID, inv.OrderID, fx.OrderID)
	}
}

func TestGenerateLineageUnsupportedSeed(t *testing.T) {
	for _, seed := range []string{"", "other-seed", northstar.DefaultSeed} {
		if _, err := northstar.GenerateLineage(seed); err == nil {
			t.Fatalf("expected error for seed %q", seed)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
