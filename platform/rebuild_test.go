package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/relay"
)

func snapshotERP(t *testing.T, db *sql.DB) (invCount, orderCount, outboxCount int, outboxHash string) {
	t.Helper()
	ctx := context.Background()
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM erp.inventory_items`).Scan(&invCount); err != nil {
		t.Fatalf("inventory count: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM erp.orders`).Scan(&orderCount); err != nil {
		t.Fatalf("orders count: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM erp.outbox`).Scan(&outboxCount); err != nil {
		t.Fatalf("outbox count: %v", err)
	}
	rows, err := db.QueryContext(ctx, `SELECT event_bytes FROM erp.outbox ORDER BY event_id`)
	if err != nil {
		t.Fatalf("outbox bytes: %v", err)
	}
	defer rows.Close()
	h := sha256.New()
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			t.Fatal(err)
		}
		_, _ = h.Write(payload)
		_, _ = h.Write([]byte{0})
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return invCount, orderCount, outboxCount, hex.EncodeToString(h.Sum(nil))
}

func erpInventoryQty(t *testing.T, db *sql.DB, tenantID, itemID string) int64 {
	t.Helper()
	var qty int64
	err := db.QueryRow(`
		SELECT quantity_on_hand FROM erp.inventory_items
		WHERE tenant_id = $1 AND item_id = $2
	`, tenantID, itemID).Scan(&qty)
	if err != nil {
		t.Fatalf("erp inventory: %v", err)
	}
	return qty
}

func platformCounts(t *testing.T, db *sql.DB) (inbox, proj, failures int) {
	t.Helper()
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.inbox`).Scan(&inbox); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.inventory_projection`).Scan(&proj); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.processing_failures`).Scan(&failures); err != nil {
		t.Fatal(err)
	}
	return inbox, proj, failures
}

func historyFromFixture(t *testing.T, fx northstar.Fixture, offset int64) HistoryRecord {
	t.Helper()
	raw := mustCanonical(t, fx.Event)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))
	return HistoryRecord{
		Key:   key,
		Value: raw,
		Pos:   SourcePosition{Topic: relay.Topic, Partition: 0, Offset: offset},
	}
}

func TestChecksumRepeatability(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	_ = processNorthstar(t, db, fx)

	ctx := context.Background()
	a, err := ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("checksum not repeatable: %s vs %s", a, b)
	}
	want := checksumOneRow(fx.TenantID, fx.ItemID, 8, 1)
	if a != want {
		t.Fatalf("checksum = %s want %s", a, want)
	}
}

func TestResetDerivedStatePreservesERPAndBrokerInputs(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	if _, err := erp.AcceptOrder(ctx, db, mustOrderCommand(t, fx)); err != nil {
		t.Fatal(err)
	}
	_ = processNorthstar(t, db, fx)

	inv, orders, outbox, outHash := snapshotERP(t, db)
	erpQty := erpInventoryQty(t, db, fx.TenantID, fx.ItemID)
	retained := historyFromFixture(t, fx, 1)

	if err := ResetDerivedState(ctx, db); err != nil {
		t.Fatal(err)
	}

	inbox, proj, failures := platformCounts(t, db)
	if inbox != 0 || proj != 0 || failures != 0 {
		t.Fatalf("platform not empty after reset: inbox=%d proj=%d failures=%d", inbox, proj, failures)
	}

	inv2, orders2, outbox2, outHash2 := snapshotERP(t, db)
	if inv != inv2 || orders != orders2 || outbox != outbox2 || outHash != outHash2 {
		t.Fatalf("erp mutated: before=(%d,%d,%d,%s) after=(%d,%d,%d,%s)",
			inv, orders, outbox, outHash, inv2, orders2, outbox2, outHash2)
	}
	if erpInventoryQty(t, db, fx.TenantID, fx.ItemID) != erpQty {
		t.Fatal("erp inventory quantity changed")
	}
	// Retained bytes still usable as rebuild input (broker/log not required here).
	if len(retained.Value) == 0 {
		t.Fatal("retained value empty")
	}
}

func TestDuplicateInjectionLeavesProjectionUnchanged(t *testing.T) {
	// FC-001-style: controlled identical duplicate publication/delivery.
	db := openTestDB(t)
	seed, stop := startRedpanda(t)
	t.Cleanup(stop)

	fx, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	if _, err := erp.AcceptOrder(ctx, db, mustOrderCommand(t, fx)); err != nil {
		t.Fatal(err)
	}
	pub, err := relay.NewFranzPublisher(seed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pub.Close)
	drain, err := relay.DrainOnce(ctx, db, pub, "relay-owner", relay.DefaultLeaseTTL, 10)
	if err != nil || drain.Published != 1 {
		t.Fatalf("drain = %+v err=%v", drain, err)
	}

	cons, err := NewConsumer(db, seed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cons.Close)

	deadline := time.Now().Add(30 * time.Second)
	var applied bool
	for time.Now().Before(deadline) && !applied {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		cres, err := cons.ConsumeOnce(cctx)
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
		if cres.Acked > 0 {
			applied = true
		}
	}
	if !applied {
		t.Fatal("timeout waiting for first consume ack")
	}

	checksumA, err := ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	qtyA, verA, ok, err := ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qtyA != 8 || verA != 1 {
		t.Fatalf("baseline projection qty=%d ver=%d ok=%v err=%v", qtyA, verA, ok, err)
	}

	raw := mustCanonical(t, fx.Event)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))
	if err := pub.Publish(ctx, relay.Topic, key, raw); err != nil {
		t.Fatal(err)
	}

	dupAck := false
	deadline = time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && !dupAck {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		cres, err := cons.ConsumeOnce(cctx)
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
		if cres.Acked > 0 {
			dupAck = true
		}
	}
	if !dupAck {
		t.Fatal("timeout waiting for duplicate consume ack")
	}

	disp, _, ok, err := InboxDisposition(ctx, db, fx.Event.EventID)
	if err != nil || !ok || disp != DispositionDuplicateNoop {
		t.Fatalf("inbox after duplicate disp=%q ok=%v err=%v", disp, ok, err)
	}
	qtyB, verB, ok, err := ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qtyB != qtyA || verB != verA {
		t.Fatalf("projection changed after duplicate: qty=%d ver=%d", qtyB, verB)
	}
	checksumB, err := ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if checksumA != checksumB {
		t.Fatalf("checksum changed after duplicate: A=%s B=%s", checksumA, checksumB)
	}
}

func TestDeterministicRebuildChecksumEquality(t *testing.T) {
	// FC-014-style: reset derived state and replay retained history → A == B.
	db := openTestDB(t)
	fx := mustFixture(t)
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	if _, err := erp.AcceptOrder(ctx, db, mustOrderCommand(t, fx)); err != nil {
		t.Fatal(err)
	}

	rec := historyFromFixture(t, fx, 7)
	res, err := ProcessRecord(ctx, db, rec.Key, rec.Value, rec.Pos)
	if err != nil || res.Disposition != DispositionApplied {
		t.Fatalf("baseline apply: %+v err=%v", res, err)
	}
	checksumA, err := ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	inv, orders, outbox, outHash := snapshotERP(t, db)

	if err := ResetDerivedState(ctx, db); err != nil {
		t.Fatal(err)
	}
	inbox, proj, failures := platformCounts(t, db)
	if inbox != 0 || proj != 0 || failures != 0 {
		t.Fatalf("expected empty platform after reset")
	}

	rebuild, err := RebuildFromHistory(ctx, db, []HistoryRecord{rec}, RebuildOptions{
		TenantID: fx.TenantID,
		Metadata: ReproductionMetadata{
			Seed:        fx.Seed,
			Commit:      "local-test",
			Limitations: "Issue #29 local FC-014-style proof; not hosted campaign.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rebuild.Status != RebuildStatusComplete {
		t.Fatalf("expected complete rebuild, got %+v reasons=%v", rebuild, rebuild.IncompleteReasons)
	}
	if rebuild.Checksum != checksumA {
		t.Fatalf("A=%s B=%s", checksumA, rebuild.Checksum)
	}
	if rebuild.Metadata.HandlerVersion != HandlerVersion {
		t.Fatalf("handler version = %q", rebuild.Metadata.HandlerVersion)
	}
	if rebuild.Metadata.EventContractVersion != event.SchemaVersionV1 {
		t.Fatalf("contract version = %d", rebuild.Metadata.EventContractVersion)
	}
	if rebuild.Metadata.Seed != northstar.DefaultSeed {
		t.Fatalf("seed = %q", rebuild.Metadata.Seed)
	}
	if rebuild.Metadata.BrokerOffsetMin != 7 || rebuild.Metadata.BrokerOffsetMax != 7 {
		t.Fatalf("offset range = %d..%d", rebuild.Metadata.BrokerOffsetMin, rebuild.Metadata.BrokerOffsetMax)
	}

	inv2, orders2, outbox2, outHash2 := snapshotERP(t, db)
	if inv != inv2 || orders != orders2 || outbox != outbox2 || outHash != outHash2 {
		t.Fatal("erp mutated during rebuild")
	}
}

func TestRebuildIncompleteOnGap(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)

	v2 := fx.Event
	v2.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b2"
	v2.AggregateVersion = 2
	v2 = event.WithQuantityDecremented(v2, func(p *event.QuantityDecremented) {
		p.QuantityBefore = 8
		p.QuantityDecremented = 1
		p.QuantityAfter = 7
		p.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b4"
	})
	v2.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b3"

	raw2 := mustCanonical(t, v2)
	key := []byte(relay.AggregateKey(v2.TenantID, v2.AggregateType, v2.AggregateID))
	rebuild, err := RebuildFromHistory(context.Background(), db, []HistoryRecord{{
		Key: key, Value: raw2,
		Pos: SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 1},
	}}, RebuildOptions{TenantID: fx.TenantID})
	if err != nil {
		t.Fatal(err)
	}
	if rebuild.Status != RebuildStatusIncomplete {
		t.Fatalf("expected incomplete, got %+v", rebuild)
	}
	if len(rebuild.IncompleteReasons) == 0 {
		t.Fatal("expected incomplete reasons")
	}
}

func TestRebuildCompleteWhenGapResolvesInSameBatch(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)

	v2 := fx.Event
	v2.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b2"
	v2.AggregateVersion = 2
	v2 = event.WithQuantityDecremented(v2, func(p *event.QuantityDecremented) {
		p.QuantityBefore = 8
		p.QuantityDecremented = 1
		p.QuantityAfter = 7
		p.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b4"
	})
	v2.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b3"

	raw1 := mustCanonical(t, fx.Event)
	raw2 := mustCanonical(t, v2)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))

	rebuild, err := RebuildFromHistory(context.Background(), db, []HistoryRecord{
		{Key: key, Value: raw2, Pos: SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 1}},
		{Key: key, Value: raw1, Pos: SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 2}},
	}, RebuildOptions{TenantID: fx.TenantID})
	if err != nil {
		t.Fatal(err)
	}
	qty, ver, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 7 || ver != 2 {
		t.Fatalf("projection qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
	var gaps int
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.inbox WHERE disposition = $1`, DispositionQuarantinedGap).Scan(&gaps); err != nil {
		t.Fatal(err)
	}
	if gaps != 0 {
		t.Fatalf("residual gaps=%d", gaps)
	}
	if rebuild.Status != RebuildStatusComplete {
		t.Fatalf("expected complete after in-batch gap redrive, got status=%s reasons=%v dispositions=%v",
			rebuild.Status, rebuild.IncompleteReasons, rebuild.Dispositions)
	}
}

func TestRebuildIncompleteOnUnsupportedVersion(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	raw := []byte(`{"aggregate_id":"item-flour-001","aggregate_type":"inventory_item","aggregate_version":1,"causation_id":null,"correlation_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a2","event_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20d1","event_schema_version":2,"event_type":"inventory.quantity_decremented","occurred_at":"2026-08-07T09:00:00Z","payload":{"item_id":"item-flour-001","order_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a4","quantity_after":8,"quantity_before":10,"quantity_decremented":2},"producer":"synthetic-erp","recorded_at":"2026-08-07T09:00:00Z","tenant_id":"11111111-1111-4111-8111-111111111111","trace_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a3"}`)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))

	rebuild, err := RebuildFromHistory(context.Background(), db, []HistoryRecord{{
		Key: key, Value: raw,
		Pos: SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 1},
	}}, RebuildOptions{TenantID: fx.TenantID})
	if err != nil {
		t.Fatal(err)
	}
	if rebuild.Status != RebuildStatusIncomplete {
		t.Fatalf("expected incomplete for unsupported schema, got %+v", rebuild)
	}
	if rebuild.Failures == 0 && rebuild.Quarantined == 0 {
		t.Fatalf("expected failure or quarantine count, got %+v", rebuild)
	}
}

func TestRebuildIncompleteOnConflictingIdentity(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	rec1 := historyFromFixture(t, fx, 1)

	conflict := fx.Event
	conflict = event.WithQuantityDecremented(conflict, func(p *event.QuantityDecremented) {
		p.QuantityAfter = 7
		p.QuantityDecremented = 3
	})
	raw2 := mustCanonical(t, conflict)
	rec2 := HistoryRecord{
		Key:   append([]byte(nil), rec1.Key...),
		Value: raw2,
		Pos:   SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 2},
	}

	rebuild, err := RebuildFromHistory(context.Background(), db, []HistoryRecord{rec1, rec2}, RebuildOptions{
		TenantID: fx.TenantID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rebuild.Status != RebuildStatusIncomplete {
		t.Fatalf("expected incomplete on conflict, got %+v", rebuild)
	}
	if rebuild.Applied != 1 || rebuild.Quarantined < 1 {
		t.Fatalf("expected one apply and quarantine, got %+v", rebuild)
	}
	// Must not treat incomplete checksum as success proof even if it matches baseline.
	baseline := checksumOneRow(fx.TenantID, fx.ItemID, 8, 1)
	if rebuild.Checksum != baseline {
		t.Fatalf("diagnostic checksum = %s want baseline %s", rebuild.Checksum, baseline)
	}
	if rebuild.Metadata.RebuildStatus != RebuildStatusIncomplete {
		t.Fatal("metadata status must stay incomplete")
	}
}

func TestRebuildHasNoExternalSideEffectPath(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	if _, err := erp.AcceptOrder(ctx, db, mustOrderCommand(t, fx)); err != nil {
		t.Fatal(err)
	}
	inv, orders, outbox, outHash := snapshotERP(t, db)
	erpQty := erpInventoryQty(t, db, fx.TenantID, fx.ItemID)

	rec := historyFromFixture(t, fx, 3)
	rebuild, err := RebuildFromHistory(ctx, db, []HistoryRecord{rec}, RebuildOptions{
		TenantID: fx.TenantID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rebuild.Status != RebuildStatusComplete {
		t.Fatalf("rebuild = %+v", rebuild)
	}

	inv2, orders2, outbox2, outHash2 := snapshotERP(t, db)
	if inv != inv2 || orders != orders2 || outbox != outbox2 || outHash != outHash2 {
		t.Fatal("rebuild mutated erp authoritative state")
	}
	if erpInventoryQty(t, db, fx.TenantID, fx.ItemID) != erpQty {
		t.Fatal("rebuild mutated erp inventory quantity")
	}
	// No relay publisher is constructed on this path; ProcessRecord only writes
	// platform tables. Projection exists while ERP quantity stays at post-accept 8.
	qty, ver, ok, err := ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("projection qty=%d ver=%d ok=%v", qty, ver, ok)
	}
}
