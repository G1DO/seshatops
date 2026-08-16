package platform

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/relay"
)

func mustLineage(t *testing.T) northstar.LineageFixture {
	t.Helper()
	fx, err := northstar.GenerateLineage(northstar.LineageSeed)
	if err != nil {
		t.Fatal(err)
	}
	return fx
}

func processRecordEnv(t *testing.T, db *sql.DB, env event.Envelope, offset int64) Result {
	t.Helper()
	raw := mustCanonical(t, env)
	key := []byte(relay.AggregateKey(env.TenantID, env.AggregateType, env.AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: offset,
	})
	if err != nil {
		t.Fatalf("ProcessRecord %s: %v", env.EventType, err)
	}
	return res
}

func TestLineageFixtureAppliesCompleteChain(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	ctx := context.Background()

	for i, env := range fx.Events {
		res := processRecordEnv(t, db, env, int64(i+1))
		if res.Disposition != DispositionApplied || !res.ShouldAck {
			t.Fatalf("event[%d] %s: %+v", i, env.EventType, res)
		}
	}

	trace, ok, err := TraceBatch(ctx, db, fx.TenantID, fx.BatchID)
	if err != nil || !ok {
		t.Fatalf("TraceBatch: ok=%v err=%v", ok, err)
	}
	if trace.TenantID != fx.TenantID {
		t.Fatalf("tenant = %q", trace.TenantID)
	}
	if trace.Supplier.ID != fx.SupplierID || trace.Lot.ID != fx.LotID || trace.Batch.ID != fx.BatchID {
		t.Fatalf("upstream supplier=%q lot=%q batch=%q", trace.Supplier.ID, trace.Lot.ID, trace.Batch.ID)
	}
	if trace.Lot.ParentID != fx.SupplierID || trace.Batch.ParentID != fx.LotID {
		t.Fatalf("parent links lot.parent=%q batch.parent=%q", trace.Lot.ParentID, trace.Batch.ParentID)
	}
	if trace.Shipment.ID != fx.ShipmentID || trace.Shipment.OrderID != fx.OrderID || trace.Shipment.ParentID != fx.BatchID {
		t.Fatalf("downstream shipment=%q order=%q parent=%q", trace.Shipment.ID, trace.Shipment.OrderID, trace.Shipment.ParentID)
	}
	if trace.Lot.ItemID != fx.ItemID {
		t.Fatalf("lot item_id=%q", trace.Lot.ItemID)
	}

	supplier := fx.Events[0]
	lot := fx.Events[1]
	batch := fx.Events[2]
	ship := fx.Events[3]
	assertHopProvenance(t, trace.Supplier, supplier)
	assertHopProvenance(t, trace.Lot, lot)
	assertHopProvenance(t, trace.Batch, batch)
	assertHopProvenance(t, trace.Shipment, ship)

	qty, ver, ok, err := ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || !ok {
		t.Fatalf("inventory: ok=%v err=%v", ok, err)
	}
	if qty != 8 || ver != 1 {
		t.Fatalf("inventory qty=%d ver=%d", qty, ver)
	}
	sum, err := ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	want := checksumOneRow(fx.TenantID, fx.ItemID, 8, 1)
	if sum != want {
		t.Fatalf("inventory checksum = %s want %s", sum, want)
	}
}

func TestLineageDuplicateDeliveryIsNoop(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	ctx := context.Background()

	for i, env := range fx.Events {
		if res := processRecordEnv(t, db, env, int64(i+1)); res.Disposition != DispositionApplied {
			t.Fatalf("first apply event[%d]: %+v", i, res)
		}
	}
	for i, env := range fx.Events {
		res := processRecordEnv(t, db, env, int64(100+i))
		if res.Disposition != DispositionDuplicateNoop || !res.ShouldAck {
			t.Fatalf("redelivery event[%d]: %+v", i, res)
		}
		disp, _, ok, err := InboxDisposition(ctx, db, env.EventID)
		if err != nil || !ok || disp != DispositionDuplicateNoop {
			t.Fatalf("inbox event[%d] disp=%q ok=%v err=%v", i, disp, ok, err)
		}
	}

	suppliers, lots, batches, shipments, err := countLineageRows(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if suppliers != 1 || lots != 1 || batches != 1 || shipments != 1 {
		t.Fatalf("duplicated lineage rows: suppliers=%d lots=%d batches=%d shipments=%d",
			suppliers, lots, batches, shipments)
	}
}

func TestLineageRollbackAtomicity(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	setTestFailBeforeCommitForTest(func(context.Context) error {
		return errors.New("injected commit failure")
	})
	t.Cleanup(func() { setTestFailBeforeCommitForTest(nil) })

	env := fx.Events[0]
	raw := mustCanonical(t, env)
	key := []byte(relay.AggregateKey(env.TenantID, env.AggregateType, env.AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 1,
	})
	if err == nil || res.ShouldAck {
		t.Fatalf("expected transient failure, got res=%+v err=%v", res, err)
	}

	suppliers, lots, batches, shipments, err := countLineageRows(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if suppliers != 0 || lots != 0 || batches != 0 || shipments != 0 {
		t.Fatalf("lineage must not commit on rollback: %d %d %d %d", suppliers, lots, batches, shipments)
	}
	_, _, inboxOK, err := InboxDisposition(context.Background(), db, env.EventID)
	if err != nil {
		t.Fatal(err)
	}
	if inboxOK {
		t.Fatal("inbox must not commit on rollback")
	}
}

func TestLineageHopsDoNotChangeInventoryChecksum(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		if res := processRecordEnv(t, db, fx.Events[i], int64(i+1)); res.Disposition != DispositionApplied {
			t.Fatalf("hop[%d]: %+v", i, res)
		}
	}
	_, _, ok, err := ProjectionState(ctx, db, fx.TenantID, fx.ItemID)
	if err != nil || ok {
		t.Fatalf("inventory must be empty before inventory hop: ok=%v err=%v", ok, err)
	}
	sum, err := ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if sum != sha256Empty() {
		t.Fatalf("checksum after lineage hops = %s", sum)
	}
}

func TestLineageOutOfOrderChildThenParentTraverses(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	ctx := context.Background()

	// Broker ordering is per aggregate; lot may arrive before supplier.
	order := []int{1, 0, 2, 3}
	for i, idx := range order {
		if res := processRecordEnv(t, db, fx.Events[idx], int64(i+1)); res.Disposition != DispositionApplied {
			t.Fatalf("out-of-order idx=%d: %+v", idx, res)
		}
	}
	trace, ok, err := TraceBatch(ctx, db, fx.TenantID, fx.BatchID)
	if err != nil || !ok {
		t.Fatalf("TraceBatch: ok=%v err=%v", ok, err)
	}
	if trace.Supplier.ID != fx.SupplierID || trace.Lot.ID != fx.LotID {
		t.Fatalf("chain supplier=%q lot=%q", trace.Supplier.ID, trace.Lot.ID)
	}
}

func TestLineageKeyMismatchQuarantined(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	env := fx.Events[0]
	raw := mustCanonical(t, env)
	res, err := ProcessRecord(context.Background(), db, []byte("wrong-key"), raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionQuarantinedMismatch || !res.ShouldAck {
		t.Fatalf("result = %+v", res)
	}
	suppliers, _, _, _, err := countLineageRows(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if suppliers != 0 {
		t.Fatalf("lineage wrote on mismatch: suppliers=%d", suppliers)
	}
}

func TestLineageCrossTenantParentDoesNotTraverse(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	ctx := context.Background()

	if res := processRecordEnv(t, db, fx.Events[0], 1); res.Disposition != DispositionApplied {
		t.Fatalf("home supplier: %+v", res)
	}

	foreignLot := fx.Events[1]
	foreignLot.TenantID = "22222222-2222-4222-8222-222222222222"
	foreignLot.EventID = "328f5d78-6e64-4f5f-bd16-8e9f7c4a4012"
	if err := event.Validate(foreignLot); err != nil {
		t.Fatal(err)
	}
	if res := processRecordEnv(t, db, foreignLot, 2); res.Disposition != DispositionApplied {
		t.Fatalf("foreign lot: %+v", res)
	}

	foreignBatch := fx.Events[2]
	foreignBatch.TenantID = foreignLot.TenantID
	foreignBatch.EventID = "328f5d78-6e64-4f5f-bd16-8e9f7c4a4013"
	if err := event.Validate(foreignBatch); err != nil {
		t.Fatal(err)
	}
	if res := processRecordEnv(t, db, foreignBatch, 3); res.Disposition != DispositionApplied {
		t.Fatalf("foreign batch: %+v", res)
	}

	home, ok, err := TraceBatch(ctx, db, fx.TenantID, fx.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("home tenant must not see foreign batch: %+v", home)
	}
	foreign, ok, err := TraceBatch(ctx, db, foreignLot.TenantID, fx.BatchID)
	if err != nil || !ok {
		t.Fatalf("foreign TraceBatch: ok=%v err=%v", ok, err)
	}
	if foreign.Supplier.ID != "" {
		t.Fatalf("cross-tenant supplier inferred: %q", foreign.Supplier.ID)
	}
	if foreign.Lot.ID != fx.LotID || foreign.Lot.ParentID != fx.SupplierID {
		t.Fatalf("foreign lot not stored under its tenant: %+v", foreign.Lot)
	}
}

func TestLineageWrongTenantLookupFailsClosed(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	for i, env := range fx.Events[:3] {
		if res := processRecordEnv(t, db, env, int64(i+1)); res.Disposition != DispositionApplied {
			t.Fatalf("apply[%d]: %+v", i, res)
		}
	}
	_, ok, err := TraceBatch(context.Background(), db, "22222222-2222-4222-8222-222222222222", fx.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("batch_id must not leak across tenants")
	}
	_, _, err = TraceBatch(context.Background(), db, "", fx.BatchID)
	if err == nil {
		t.Fatal("empty tenant must fail closed")
	}
}

func TestLineageConflictingParentEdgeQuarantined(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	if res := processRecordEnv(t, db, fx.Events[0], 1); res.Disposition != DispositionApplied {
		t.Fatalf("supplier: %+v", res)
	}
	if res := processRecordEnv(t, db, fx.Events[1], 2); res.Disposition != DispositionApplied {
		t.Fatalf("lot: %+v", res)
	}

	other := fx.Events[1]
	other.EventID = "428f5d78-6e64-4f5f-bd16-8e9f7c4a5012"
	other.AggregateID = "lot-flour-other-001"
	payload, ok := event.AsIngredientLotReceived(other)
	if !ok {
		t.Fatal("payload")
	}
	payload.LotID = other.AggregateID
	other.Payload = payload
	if err := event.Validate(other); err != nil {
		t.Fatal(err)
	}
	res := processRecordEnv(t, db, other, 3)
	if res.Disposition != DispositionQuarantinedInvalid {
		t.Fatalf("second lot claiming same supplier: %+v", res)
	}
	_, lots, _, _, err := countLineageRows(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if lots != 1 {
		t.Fatalf("conflicting lot duplicated: lots=%d", lots)
	}
}

func TestLineageStaleVersionQuarantined(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	if res := processRecordEnv(t, db, fx.Events[0], 1); res.Disposition != DispositionApplied {
		t.Fatalf("supplier: %+v", res)
	}
	stale := fx.Events[0]
	stale.EventID = "528f5d78-6e64-4f5f-bd16-8e9f7c4a6011"
	if err := event.Validate(stale); err != nil {
		t.Fatal(err)
	}
	res := processRecordEnv(t, db, stale, 2)
	if res.Disposition != DispositionQuarantinedStale {
		t.Fatalf("stale supplier v1: %+v", res)
	}
}

func TestLineageConcurrentSameEventKeepsApplied(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	env := fx.Events[0]
	raw := mustCanonical(t, env)
	key := []byte(relay.AggregateKey(env.TenantID, env.AggregateType, env.AggregateID))

	type outcome struct {
		res Result
		err error
	}
	ch := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func(offset int64) {
			res, err := ProcessRecord(context.Background(), db, key, raw, SourcePosition{
				Topic: relay.Topic, Partition: 0, Offset: offset,
			})
			ch <- outcome{res: res, err: err}
		}(int64(i + 1))
	}

	var applied, noop, invalid int
	for i := 0; i < 2; i++ {
		out := <-ch
		if out.err != nil {
			t.Fatalf("concurrent ProcessRecord: %v", out.err)
		}
		if !out.res.ShouldAck {
			t.Fatalf("concurrent result must ack: %+v", out.res)
		}
		switch out.res.Disposition {
		case DispositionApplied:
			applied++
		case DispositionDuplicateNoop:
			noop++
		case DispositionQuarantinedInvalid:
			invalid++
		default:
			t.Fatalf("unexpected disposition %+v", out.res)
		}
	}
	if invalid != 0 {
		t.Fatalf("concurrent identical delivery quarantined invalid: applied=%d noop=%d", applied, noop)
	}
	if applied+noop != 2 {
		t.Fatalf("applied=%d noop=%d", applied, noop)
	}

	disp, _, ok, err := InboxDisposition(context.Background(), db, env.EventID)
	if err != nil || !ok {
		t.Fatalf("inbox: ok=%v err=%v", ok, err)
	}
	if disp != DispositionApplied && disp != DispositionDuplicateNoop {
		t.Fatalf("inbox disposition %q", disp)
	}
	suppliers, _, _, _, err := countLineageRows(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if suppliers != 1 {
		t.Fatalf("suppliers=%d", suppliers)
	}
}

func assertHopProvenance(t *testing.T, hop LineageHop, env event.Envelope) {
	t.Helper()
	if hop.SourceEventID != env.EventID {
		t.Fatalf("%s source_event_id=%q want %q", env.EventType, hop.SourceEventID, env.EventID)
	}
	if hop.EventSchemaVersion != env.EventSchemaVersion {
		t.Fatalf("%s schema=%d", env.EventType, hop.EventSchemaVersion)
	}
	if hop.OccurredAt != env.OccurredAt || hop.RecordedAt != env.RecordedAt {
		t.Fatalf("%s timestamps occurred=%q recorded=%q", env.EventType, hop.OccurredAt, hop.RecordedAt)
	}
	if hop.CorrelationID != env.CorrelationID || hop.TraceID != env.TraceID {
		t.Fatalf("%s correlation=%q trace=%q", env.EventType, hop.CorrelationID, hop.TraceID)
	}
	if (env.CausationID == nil) != (hop.CausationID == nil) {
		t.Fatalf("%s causation presence env=%v hop=%v", env.EventType, env.CausationID, hop.CausationID)
	}
	if env.CausationID != nil && hop.CausationID != nil && *env.CausationID != *hop.CausationID {
		t.Fatalf("%s causation=%q want %q", env.EventType, *hop.CausationID, *env.CausationID)
	}
	if hop.AggregateVersion != env.AggregateVersion {
		t.Fatalf("%s version=%d", env.EventType, hop.AggregateVersion)
	}
}
