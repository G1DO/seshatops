package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/relay"
)

func TestResetDerivedStateForTenantLeavesOtherTenant(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ctx := context.Background()

	applied := processNorthstar(t, db, fx)
	if applied.Disposition != DispositionApplied {
		t.Fatalf("ns001 disposition = %s", applied.Disposition)
	}

	other := fx.Event
	other.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2181"
	other.TenantID = identity.TenantNS002UUID
	other.AggregateID = "item-sugar-001"
	other.Payload.ItemID = "item-sugar-001"
	other.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2182"
	other.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2183"
	raw := mustCanonical(t, other)
	key := []byte(relay.AggregateKey(other.TenantID, other.AggregateType, other.AggregateID))
	res, err := ProcessRecord(ctx, db, key, raw, SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 20})
	if err != nil || res.Disposition != DispositionApplied {
		t.Fatalf("ns002 apply: %+v err=%v", res, err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO platform.processing_failures (
			failure_id, consumer_name, failure_category, diagnostic_code,
			quarantine_status, source_topic, source_partition, source_offset, attempt_count
		) VALUES (
			'fail-unattributed', $1, 'malformed_envelope', 'malformed_envelope',
			'quarantined', $2, 0, 99, 1
		)
	`, ConsumerName, relay.Topic); err != nil {
		t.Fatal(err)
	}

	if err := ResetDerivedStateForTenant(ctx, db, fx.TenantID); err != nil {
		t.Fatal(err)
	}

	_, _, ok, err := ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || ok {
		t.Fatalf("ns001 projection should be gone: ok=%v err=%v", ok, err)
	}
	qty, ver, ok, err := ProjectionState(ctx, db, other.TenantID, other.AggregateID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("ns002 projection qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
	var unattributed int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM platform.processing_failures WHERE tenant_id IS NULL
	`).Scan(&unattributed); err != nil {
		t.Fatal(err)
	}
	if unattributed != 1 {
		t.Fatalf("unattributed failures = %d", unattributed)
	}
}

func TestReplayTenantHistoryIsDuplicateNoop(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	if _, err := erp.AcceptOrder(ctx, db, erp.OrderCommandFromFixture(fx)); err != nil {
		t.Fatal(err)
	}
	applied := processNorthstar(t, db, fx)
	if applied.Disposition != DispositionApplied {
		t.Fatalf("disposition = %s", applied.Disposition)
	}
	erpQty := erpInventoryQty(t, db, fx.TenantID, fx.ItemID)
	qtyA, verA, ok, err := ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || !ok {
		t.Fatalf("projection: ok=%v err=%v", ok, err)
	}

	replay, err := ReplayTenantHistory(ctx, db, fx.TenantID, fx.Event.EventID)
	if err != nil {
		t.Fatal(err)
	}
	if replay.DuplicateNoop != 1 || replay.Applied != 0 {
		t.Fatalf("replay = %+v", replay)
	}
	qtyB, verB, ok, err := ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qtyB != qtyA || verB != verA {
		t.Fatalf("projection changed: qty=%d ver=%d", qtyB, verB)
	}
	if erpInventoryQty(t, db, fx.TenantID, fx.ItemID) != erpQty {
		t.Fatal("erp mutated during replay")
	}
}

func TestRebuildTenantFromHistoryLeavesOtherTenant(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	if _, err := erp.AcceptOrder(ctx, db, erp.OrderCommandFromFixture(fx)); err != nil {
		t.Fatal(err)
	}
	if processNorthstar(t, db, fx).Disposition != DispositionApplied {
		t.Fatal("ns001 apply failed")
	}

	other := fx.Event
	other.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2184"
	other.TenantID = identity.TenantNS002UUID
	other.AggregateID = "item-sugar-001"
	other.Payload.ItemID = "item-sugar-001"
	other.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2185"
	other.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2186"
	raw := mustCanonical(t, other)
	key := []byte(relay.AggregateKey(other.TenantID, other.AggregateType, other.AggregateID))
	res, err := ProcessRecord(ctx, db, key, raw, SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 21})
	if err != nil || res.Disposition != DispositionApplied {
		t.Fatalf("ns002 apply: %+v err=%v", res, err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO erp.outbox (
			event_id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
			content_hash, event_bytes, status, recorded_at
		) VALUES ($1, $2, $3, $4, 1, 'bb', $5, 'published', now())
	`, other.EventID, other.TenantID, other.AggregateType, other.AggregateID, raw); err != nil {
		t.Fatal(err)
	}

	rebuild, err := RebuildTenantFromHistory(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if rebuild.Status != RebuildStatusComplete || rebuild.Applied != 1 {
		t.Fatalf("rebuild = %+v", rebuild)
	}
	qty, ver, ok, err := ProjectionState(ctx, db, other.TenantID, other.AggregateID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("other tenant mutated qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
}

func TestReleaseTenantQuarantineRejectsGap(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ctx := context.Background()

	gap := fx.Event
	gap.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2187"
	gap.AggregateVersion = 2
	gap.Payload.QuantityBefore = 8
	gap.Payload.QuantityDecremented = 1
	gap.Payload.QuantityAfter = 7
	gap.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2188"
	gap.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2189"
	raw := mustCanonical(t, gap)
	key := []byte(relay.AggregateKey(gap.TenantID, gap.AggregateType, gap.AggregateID))
	res, err := ProcessRecord(ctx, db, key, raw, SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 40})
	if err != nil || res.Disposition != DispositionQuarantinedGap {
		t.Fatalf("gap = %+v err=%v", res, err)
	}

	err = ReleaseTenantQuarantine(ctx, db, fx.TenantID, gap.EventID)
	if !errors.Is(err, ErrNotReleasable) {
		t.Fatalf("err=%v", err)
	}
	disp, _, ok, err := InboxDisposition(ctx, db, gap.EventID)
	if err != nil || !ok || disp != DispositionQuarantinedGap {
		t.Fatalf("gap skipped: disp=%s ok=%v err=%v", disp, ok, err)
	}
	_, _, ok, err = ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || ok {
		t.Fatalf("later version applied: ok=%v err=%v", ok, err)
	}
}

func TestLoadTenantOutboxHistoryRejectsEnvelopeTenantMismatch(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	if _, err := erp.AcceptOrder(ctx, db, erp.OrderCommandFromFixture(fx)); err != nil {
		t.Fatal(err)
	}

	foreign := fx.Event
	foreign.TenantID = identity.TenantNS002UUID
	raw := mustCanonical(t, foreign)
	if _, err := db.ExecContext(ctx, `
		UPDATE erp.outbox SET event_bytes = $1 WHERE event_id = $2
	`, raw, fx.Event.EventID); err != nil {
		t.Fatal(err)
	}

	_, err := LoadTenantOutboxHistory(ctx, db, fx.TenantID, fx.Event.EventID)
	if !errors.Is(err, ErrTenantMismatch) {
		t.Fatalf("err=%v", err)
	}
}

func TestReleaseTenantQuarantineRedrivesPoisonFromRetainedBytes(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	if _, err := erp.AcceptOrder(ctx, db, erp.OrderCommandFromFixture(fx)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO platform.processing_failures (
			failure_id, consumer_name, event_id, tenant_id,
			failure_category, diagnostic_code, quarantine_status,
			source_topic, source_partition, source_offset, attempt_count
		) VALUES (
			'fail-poison', $1, $2, $3,
			'handler_poison', 'handler_poison', 'quarantined',
			$4, 0, 70, 5
		)
	`, ConsumerName, fx.Event.EventID, fx.TenantID, relay.Topic); err != nil {
		t.Fatal(err)
	}

	if err := ReleaseTenantQuarantine(ctx, db, fx.TenantID, fx.Event.EventID); err != nil {
		t.Fatal(err)
	}
	qty, ver, ok, err := ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("poison redrive qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
}
