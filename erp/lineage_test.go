package erp

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
)

const (
	otherTenantID = "22222222-2222-4222-8222-222222222222"
	otherEventID  = "318f5d78-6e64-4f5f-bd16-8e9f7c4a4011"
	otherEventID2 = "318f5d78-6e64-4f5f-bd16-8e9f7c4a4012"
	otherEventID3 = "318f5d78-6e64-4f5f-bd16-8e9f7c4a4013"
	otherCorrID   = "318f5d78-6e64-4f5f-bd16-8e9f7c4a4001"
	otherOrderID  = "318f5d78-6e64-4f5f-bd16-8e9f7c4a4020"
	chain2Event1  = "418f5d78-6e64-4f5f-bd16-8e9f7c4a5011"
	chain2Event2  = "418f5d78-6e64-4f5f-bd16-8e9f7c4a5012"
	chain2Event3  = "418f5d78-6e64-4f5f-bd16-8e9f7c4a5013"
	chain2Event4  = "418f5d78-6e64-4f5f-bd16-8e9f7c4a5014"
)

func ptr(s string) *string { return &s }

func mustLineage(t *testing.T) northstar.LineageFixture {
	t.Helper()
	fx, err := northstar.GenerateLineage(northstar.LineageSeed)
	if err != nil {
		t.Fatal(err)
	}
	return fx
}

func seedLineageDB(t *testing.T, db *sql.DB, fx northstar.LineageFixture) {
	t.Helper()
	if err := SeedLineageInventory(context.Background(), db, fx); err != nil {
		t.Fatal(err)
	}
}

func mustSupplierCmd(t *testing.T, fx northstar.LineageFixture) SupplierCommand {
	t.Helper()
	cmd, err := SupplierCommandFromLineage(fx)
	if err != nil {
		t.Fatal(err)
	}
	return cmd
}

func mustLotCmd(t *testing.T, fx northstar.LineageFixture) IngredientLotCommand {
	t.Helper()
	cmd, err := IngredientLotCommandFromLineage(fx)
	if err != nil {
		t.Fatal(err)
	}
	return cmd
}

func mustBatchCmd(t *testing.T, fx northstar.LineageFixture) ProductionBatchCommand {
	t.Helper()
	cmd, err := ProductionBatchCommandFromLineage(fx)
	if err != nil {
		t.Fatal(err)
	}
	return cmd
}

func mustShipCmd(t *testing.T, fx northstar.LineageFixture) ShipmentCommand {
	t.Helper()
	cmd, err := ShipmentCommandFromLineage(fx)
	if err != nil {
		t.Fatal(err)
	}
	return cmd
}

func mustLineageOrderCmd(t *testing.T, fx northstar.LineageFixture) OrderCommand {
	t.Helper()
	cmd, err := OrderCommandFromLineage(fx)
	if err != nil {
		t.Fatal(err)
	}
	return cmd
}

func acceptThroughShipment(t *testing.T, db *sql.DB, fx northstar.LineageFixture) {
	t.Helper()
	ctx := context.Background()
	if _, err := RegisterSupplier(ctx, db, mustSupplierCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReceiveIngredientLot(ctx, db, mustLotCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	if _, err := ProduceProductionBatch(ctx, db, mustBatchCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	if _, err := DispatchShipment(ctx, db, mustShipCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
}

func acceptFullLineage(t *testing.T, db *sql.DB, fx northstar.LineageFixture) AcceptResult {
	t.Helper()
	acceptThroughShipment(t, db, fx)
	res, err := AcceptOrder(context.Background(), db, mustLineageOrderCmd(t, fx))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func assertMatchesEnvelope(t *testing.T, res SourceAcceptResult, env event.Envelope) {
	t.Helper()
	wantHash, err := event.ContentHash(env)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, err := event.CanonicalBytes(env)
	if err != nil {
		t.Fatal(err)
	}
	if res.ContentHash != wantHash || string(res.EventBytes) != string(wantBytes) {
		t.Fatalf("result identity mismatch hash=%s", res.ContentHash)
	}
	if res.OutboxStatus != "pending" {
		t.Fatalf("status = %s, want pending", res.OutboxStatus)
	}
}

func assertOutboxEquals(t *testing.T, db *sql.DB, env event.Envelope) {
	t.Helper()
	wantHash, err := event.ContentHash(env)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, err := event.CanonicalBytes(env)
	if err != nil {
		t.Fatal(err)
	}
	var storedHash string
	var storedBytes []byte
	var status string
	err = db.QueryRow(`
		SELECT content_hash, event_bytes, status FROM erp.outbox WHERE event_id = $1
	`, env.EventID).Scan(&storedHash, &storedBytes, &status)
	if err != nil {
		t.Fatal(err)
	}
	if storedHash != wantHash || string(storedBytes) != string(wantBytes) || status != "pending" {
		t.Fatalf("stored outbox mismatch hash=%s status=%s", storedHash, status)
	}
	parsed, err := event.Parse(storedBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := event.CheckIdentityConflict(parsed, env); err != nil {
		t.Fatal(err)
	}
}

func TestLineageChainCommitsSourceAndOutboxAtomically(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()

	supplier, err := RegisterSupplier(ctx, db, mustSupplierCmd(t, fx))
	if err != nil {
		t.Fatal(err)
	}
	assertMatchesEnvelope(t, supplier, fx.Events[0])
	assertOutboxEquals(t, db, fx.Events[0])

	lot, err := ReceiveIngredientLot(ctx, db, mustLotCmd(t, fx))
	if err != nil {
		t.Fatal(err)
	}
	assertMatchesEnvelope(t, lot, fx.Events[1])
	assertOutboxEquals(t, db, fx.Events[1])

	batch, err := ProduceProductionBatch(ctx, db, mustBatchCmd(t, fx))
	if err != nil {
		t.Fatal(err)
	}
	assertMatchesEnvelope(t, batch, fx.Events[2])
	assertOutboxEquals(t, db, fx.Events[2])

	ship, err := DispatchShipment(ctx, db, mustShipCmd(t, fx))
	if err != nil {
		t.Fatal(err)
	}
	assertMatchesEnvelope(t, ship, fx.Events[3])
	assertOutboxEquals(t, db, fx.Events[3])

	res, err := AcceptOrder(ctx, db, mustLineageOrderCmd(t, fx))
	if err != nil {
		t.Fatal(err)
	}
	wantHash, err := event.ContentHash(fx.Events[4])
	if err != nil {
		t.Fatal(err)
	}
	if res.ContentHash != wantHash {
		t.Fatalf("inventory hash = %s, want %s", res.ContentHash, wantHash)
	}
	assertOutboxEquals(t, db, fx.Events[4])

	qty, version := inventoryState(t, db, fx.TenantID, fx.ItemID)
	if qty != 8 || version != 1 {
		t.Fatalf("inventory qty=%d version=%d", qty, version)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.suppliers`) != 1 {
		t.Fatal("expected one supplier")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.ingredient_lots`) != 1 {
		t.Fatal("expected one lot")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.production_batches`) != 1 {
		t.Fatal("expected one batch")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.shipments`) != 1 {
		t.Fatal("expected one shipment")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.orders`) != 1 {
		t.Fatal("expected one order")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 5 {
		t.Fatal("expected five outbox rows")
	}
}

func TestRegisterSupplierRollbackLeavesNoPartialState(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	setTestFailBeforeCommitForTest(func(context.Context) error {
		return errors.New("forced failure before commit")
	})
	t.Cleanup(func() { setTestFailBeforeCommitForTest(nil) })

	_, err := RegisterSupplier(context.Background(), db, mustSupplierCmd(t, fx))
	if err == nil {
		t.Fatal("expected forced failure")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.suppliers`) != 0 {
		t.Fatal("supplier should not be committed")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 0 {
		t.Fatal("outbox should not be committed")
	}
}

func TestLineageAcceptOrderRollbackLeavesInventoryUnchanged(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	acceptThroughShipment(t, db, fx)

	setTestFailBeforeCommitForTest(func(context.Context) error {
		return errors.New("forced failure before commit")
	})
	t.Cleanup(func() { setTestFailBeforeCommitForTest(nil) })

	_, err := AcceptOrder(context.Background(), db, mustLineageOrderCmd(t, fx))
	if err == nil {
		t.Fatal("expected forced failure")
	}
	qty, version := inventoryState(t, db, fx.TenantID, fx.ItemID)
	if qty != 10 || version != 0 {
		t.Fatalf("inventory changed after rollback: qty=%d version=%d", qty, version)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.orders`) != 0 {
		t.Fatal("order should not be committed")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 4 {
		t.Fatal("lineage hops must remain; inventory outbox must roll back")
	}
}

func TestLineageMissingParents(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()

	if _, err := ReceiveIngredientLot(ctx, db, mustLotCmd(t, fx)); !errors.Is(err, ErrUnknownSource) {
		t.Fatalf("lot without supplier: err=%v", err)
	}
	if _, err := ProduceProductionBatch(ctx, db, mustBatchCmd(t, fx)); !errors.Is(err, ErrUnknownSource) {
		t.Fatalf("batch without lot: err=%v", err)
	}
	if _, err := DispatchShipment(ctx, db, mustShipCmd(t, fx)); !errors.Is(err, ErrUnknownSource) {
		t.Fatalf("shipment without batch: err=%v", err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 0 {
		t.Fatal("outbox inserted on missing parent")
	}
}

func TestLineageWrongTenantParentIsUnknownSource(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()
	if _, err := RegisterSupplier(ctx, db, mustSupplierCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO erp.inventory_items (tenant_id, item_id, quantity_on_hand, aggregate_version)
		VALUES ($1, $2, 10, 0)
	`, otherTenantID, fx.ItemID); err != nil {
		t.Fatal(err)
	}

	lot := mustLotCmd(t, fx)
	lot.TenantID = otherTenantID
	lot.EventID = otherEventID
	lot.CorrelationID = otherCorrID
	if _, err := ReceiveIngredientLot(ctx, db, lot); !errors.Is(err, ErrUnknownSource) {
		t.Fatalf("err = %v, want ErrUnknownSource", err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.ingredient_lots`) != 0 {
		t.Fatal("lot inserted for wrong tenant")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 1 {
		t.Fatal("expected only supplier outbox")
	}
}

func TestLineageLotUnknownItem(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	ctx := context.Background()
	if _, err := RegisterSupplier(ctx, db, mustSupplierCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReceiveIngredientLot(ctx, db, mustLotCmd(t, fx)); !errors.Is(err, ErrUnknownItem) {
		t.Fatalf("err = %v, want ErrUnknownItem", err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.ingredient_lots`) != 0 {
		t.Fatal("lot inserted without inventory item")
	}
}

func TestLineageDuplicatesAndOneToOne(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()
	acceptThroughShipment(t, db, fx)

	if _, err := RegisterSupplier(ctx, db, mustSupplierCmd(t, fx)); !errors.Is(err, ErrDuplicateSource) {
		t.Fatalf("duplicate supplier: %v", err)
	}

	lot := mustLotCmd(t, fx)
	lot.LotID = "lot-flour-2026-002"
	lot.EventID = otherEventID
	if _, err := ReceiveIngredientLot(ctx, db, lot); !errors.Is(err, ErrDuplicateSource) {
		t.Fatalf("second lot for supplier: %v", err)
	}

	batch := mustBatchCmd(t, fx)
	batch.BatchID = "batch-bread-002"
	batch.EventID = otherEventID2
	if _, err := ProduceProductionBatch(ctx, db, batch); !errors.Is(err, ErrDuplicateSource) {
		t.Fatalf("second batch for lot: %v", err)
	}

	ship := mustShipCmd(t, fx)
	ship.ShipmentID = "ship-northstar-002"
	ship.EventID = otherEventID3
	if _, err := DispatchShipment(ctx, db, ship); !errors.Is(err, ErrDuplicateSource) {
		t.Fatalf("second shipment for batch: %v", err)
	}

	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 4 {
		t.Fatal("duplicate attempts must not insert outbox rows")
	}
}

func TestLineageDuplicateEventID(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()
	if _, err := RegisterSupplier(ctx, db, mustSupplierCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	lot := mustLotCmd(t, fx)
	lot.EventID = fx.Events[0].EventID
	if _, err := ReceiveIngredientLot(ctx, db, lot); !errors.Is(err, ErrDuplicateEvent) {
		t.Fatalf("err = %v, want ErrDuplicateEvent", err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.ingredient_lots`) != 0 {
		t.Fatal("lot committed with duplicate event_id")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 1 {
		t.Fatal("expected only supplier outbox")
	}
}

func TestRegisterSupplierConcurrentDuplicate(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()

	type outcome struct{ err error }
	ch := make(chan outcome, 2)
	base := mustSupplierCmd(t, fx)
	go func() {
		_, err := RegisterSupplier(ctx, db, base)
		ch <- outcome{err: err}
	}()
	go func() {
		cmd := base
		cmd.EventID = otherEventID
		_, err := RegisterSupplier(ctx, db, cmd)
		ch <- outcome{err: err}
	}()

	var success, failure int
	for i := 0; i < 2; i++ {
		o := <-ch
		if o.err == nil {
			success++
			continue
		}
		if !errors.Is(o.err, ErrDuplicateSource) {
			t.Fatalf("unexpected error: %v", o.err)
		}
		failure++
	}
	if success != 1 || failure != 1 {
		t.Fatalf("success=%d failure=%d, want 1 and 1", success, failure)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.suppliers`) != 1 {
		t.Fatal("expected one supplier")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 1 {
		t.Fatal("expected one outbox row")
	}
}

func TestLineageCausationAndIdentity(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()

	supplier := mustSupplierCmd(t, fx)
	supplier.CausationID = ptr(otherEventID)
	if _, err := RegisterSupplier(ctx, db, supplier); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("supplier causation: %v", err)
	}

	supplier = mustSupplierCmd(t, fx)
	supplier.TenantID = "NOT-A-TENANT"
	if _, err := RegisterSupplier(ctx, db, supplier); !errors.Is(err, ErrTenantMismatch) {
		t.Fatalf("malformed tenant: %v", err)
	}

	supplier = mustSupplierCmd(t, fx)
	supplier.SupplierID = "Mill-Northstar-001"
	if _, err := RegisterSupplier(ctx, db, supplier); !errors.Is(err, ErrTenantMismatch) {
		t.Fatalf("uppercase id: %v", err)
	}

	if _, err := RegisterSupplier(ctx, db, mustSupplierCmd(t, fx)); err != nil {
		t.Fatal(err)
	}

	lot := mustLotCmd(t, fx)
	lot.CausationID = nil
	if _, err := ReceiveIngredientLot(ctx, db, lot); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("nil lot causation: %v", err)
	}

	lot = mustLotCmd(t, fx)
	lot.CausationID = ptr("not-a-uuid")
	if _, err := ReceiveIngredientLot(ctx, db, lot); !errors.Is(err, ErrTenantMismatch) {
		t.Fatalf("malformed lot causation: %v", err)
	}

	lot = mustLotCmd(t, fx)
	lot.CausationID = ptr(otherEventID)
	if _, err := ReceiveIngredientLot(ctx, db, lot); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("wrong lot causation: %v", err)
	}

	if countRows(t, db, `SELECT COUNT(*) FROM erp.ingredient_lots`) != 0 {
		t.Fatal("lot inserted on causation failure")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 1 {
		t.Fatal("expected only supplier outbox")
	}
}

func TestAcceptOrderCausationCoupling(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()

	cmd := mustLineageOrderCmd(t, fx)
	if _, err := AcceptOrder(ctx, db, cmd); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("causation without shipment: %v", err)
	}

	m1 := mustLineageOrderCmd(t, fx)
	m1.CausationID = nil
	m1.EventID = otherEventID
	if _, err := AcceptOrder(ctx, db, m1); err != nil {
		t.Fatal(err)
	}
	qty, version := inventoryState(t, db, fx.TenantID, fx.ItemID)
	if qty != 8 || version != 1 {
		t.Fatalf("m1-style accept qty=%d version=%d", qty, version)
	}

	db = openTestDB(t)
	fx = mustLineage(t)
	seedLineageDB(t, db, fx)
	acceptThroughShipment(t, db, fx)

	nilCausation := mustLineageOrderCmd(t, fx)
	nilCausation.CausationID = nil
	if _, err := AcceptOrder(ctx, db, nilCausation); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("nil causation with shipment: %v", err)
	}

	wrong := mustLineageOrderCmd(t, fx)
	wrong.CausationID = ptr(fx.Events[0].EventID)
	if _, err := AcceptOrder(ctx, db, wrong); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("wrong causation with shipment: %v", err)
	}
	qty, version = inventoryState(t, db, fx.TenantID, fx.ItemID)
	if qty != 10 || version != 0 {
		t.Fatalf("inventory mutated on causation reject: qty=%d version=%d", qty, version)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.orders`) != 0 {
		t.Fatal("order inserted on causation reject")
	}
}

func TestLineageHasNoBrokerDependency(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	_ = acceptFullLineage(t, db, fx)
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox WHERE status = 'pending'`) != 5 {
		t.Fatal("expected pending lineage outbox without broker publication")
	}
}

func TestM1AcceptOrderUnchangedWithoutShipment(t *testing.T) {
	db := openTestDB(t)
	fx := seedFixture(t, db)
	res, err := AcceptOrder(context.Background(), db, mustOrderCommand(t, fx))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := event.Parse(res.EventBytes)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.CausationID != nil {
		t.Fatalf("m1 causation_id = %v, want nil", parsed.CausationID)
	}
}

func TestLineageCommandBuildersRejectWrongFamily(t *testing.T) {
	fx := mustLineage(t)
	empty := northstar.LineageFixture{}
	if _, err := SupplierCommandFromLineage(empty); !errors.Is(err, ErrInvalidFixture) {
		t.Fatalf("empty fixture: %v", err)
	}
	wrong := fx
	wrong.Events = []event.Envelope{fx.Events[1]}
	if _, err := SupplierCommandFromLineage(wrong); !errors.Is(err, ErrInvalidFixture) {
		t.Fatalf("wrong family: %v", err)
	}
}

func TestLineageHopRollbackLeavesNoPartialState(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()

	if _, err := RegisterSupplier(ctx, db, mustSupplierCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	forceFailBeforeCommit(t)
	if _, err := ReceiveIngredientLot(ctx, db, mustLotCmd(t, fx)); err == nil {
		t.Fatal("expected lot rollback")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.ingredient_lots`) != 0 {
		t.Fatal("lot should not be committed")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.suppliers`) != 1 {
		t.Fatal("supplier must remain")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 1 {
		t.Fatal("expected only supplier outbox")
	}

	setTestFailBeforeCommitForTest(nil)
	if _, err := ReceiveIngredientLot(ctx, db, mustLotCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	forceFailBeforeCommit(t)
	if _, err := ProduceProductionBatch(ctx, db, mustBatchCmd(t, fx)); err == nil {
		t.Fatal("expected batch rollback")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.production_batches`) != 0 {
		t.Fatal("batch should not be committed")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.ingredient_lots`) != 1 {
		t.Fatal("lot must remain")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 2 {
		t.Fatal("expected supplier and lot outbox")
	}

	setTestFailBeforeCommitForTest(nil)
	if _, err := ProduceProductionBatch(ctx, db, mustBatchCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	forceFailBeforeCommit(t)
	if _, err := DispatchShipment(ctx, db, mustShipCmd(t, fx)); err == nil {
		t.Fatal("expected shipment rollback")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.shipments`) != 0 {
		t.Fatal("shipment should not be committed")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.production_batches`) != 1 {
		t.Fatal("batch must remain")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 3 {
		t.Fatal("expected three prior outbox rows")
	}
}

func TestLineageOrderIDUniqueness(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()
	acceptThroughShipment(t, db, fx)

	supplier := mustSupplierCmd(t, fx)
	supplier.SupplierID = "mill-northstar-002"
	supplier.EventID = chain2Event1
	second, err := RegisterSupplier(ctx, db, supplier)
	if err != nil {
		t.Fatal(err)
	}

	lot := mustLotCmd(t, fx)
	lot.LotID = "lot-flour-2026-002"
	lot.SupplierID = "mill-northstar-002"
	lot.EventID = chain2Event2
	lot.CausationID = ptr(second.EventID)
	if _, err := ReceiveIngredientLot(ctx, db, lot); err != nil {
		t.Fatal(err)
	}

	batch := mustBatchCmd(t, fx)
	batch.BatchID = "batch-bread-002"
	batch.LotID = "lot-flour-2026-002"
	batch.EventID = chain2Event3
	batch.CausationID = ptr(chain2Event2)
	if _, err := ProduceProductionBatch(ctx, db, batch); err != nil {
		t.Fatal(err)
	}

	ship := mustShipCmd(t, fx)
	ship.ShipmentID = "ship-northstar-002"
	ship.BatchID = "batch-bread-002"
	ship.EventID = chain2Event4
	ship.CausationID = ptr(chain2Event3)
	if _, err := DispatchShipment(ctx, db, ship); !errors.Is(err, ErrDuplicateSource) {
		t.Fatalf("reused order_id: %v", err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.shipments`) != 1 {
		t.Fatal("expected one shipment")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 7 {
		t.Fatal("failed shipment must not insert outbox")
	}
}

func TestLineageWrongTenantBatchAndShipmentAreUnknownSource(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()
	if _, err := RegisterSupplier(ctx, db, mustSupplierCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReceiveIngredientLot(ctx, db, mustLotCmd(t, fx)); err != nil {
		t.Fatal(err)
	}

	batch := mustBatchCmd(t, fx)
	batch.TenantID = otherTenantID
	batch.EventID = otherEventID
	batch.CorrelationID = otherCorrID
	if _, err := ProduceProductionBatch(ctx, db, batch); !errors.Is(err, ErrUnknownSource) {
		t.Fatalf("batch wrong tenant: %v", err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.production_batches`) != 0 {
		t.Fatal("batch inserted for wrong tenant")
	}

	if _, err := ProduceProductionBatch(ctx, db, mustBatchCmd(t, fx)); err != nil {
		t.Fatal(err)
	}

	ship := mustShipCmd(t, fx)
	ship.TenantID = otherTenantID
	ship.EventID = otherEventID2
	ship.CorrelationID = otherCorrID
	if _, err := DispatchShipment(ctx, db, ship); !errors.Is(err, ErrUnknownSource) {
		t.Fatalf("shipment wrong tenant: %v", err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.shipments`) != 0 {
		t.Fatal("shipment inserted for wrong tenant")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 3 {
		t.Fatal("expected only tenant-A hops in outbox")
	}
}

func TestLineageOrphanParentIsUnknownSource(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO erp.suppliers (
			tenant_id, supplier_id, aggregate_version, registered_at, correlation_id, trace_id
		) VALUES ($1, $2, 1, $3, $4, $5)
	`, fx.TenantID, fx.SupplierID, "2026-08-07T10:00:00Z", fx.Events[0].CorrelationID, fx.Events[0].TraceID); err != nil {
		t.Fatal(err)
	}

	if _, err := ReceiveIngredientLot(ctx, db, mustLotCmd(t, fx)); !errors.Is(err, ErrUnknownSource) {
		t.Fatalf("orphan supplier: %v", err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.ingredient_lots`) != 0 {
		t.Fatal("lot inserted against orphan parent")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 0 {
		t.Fatal("outbox inserted against orphan parent")
	}
}

func TestLineageBatchAndShipmentCausation(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()
	if _, err := RegisterSupplier(ctx, db, mustSupplierCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReceiveIngredientLot(ctx, db, mustLotCmd(t, fx)); err != nil {
		t.Fatal(err)
	}

	batch := mustBatchCmd(t, fx)
	batch.CausationID = nil
	if _, err := ProduceProductionBatch(ctx, db, batch); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("nil batch causation: %v", err)
	}
	batch = mustBatchCmd(t, fx)
	batch.CausationID = ptr("not-a-uuid")
	if _, err := ProduceProductionBatch(ctx, db, batch); !errors.Is(err, ErrTenantMismatch) {
		t.Fatalf("malformed batch causation: %v", err)
	}
	batch = mustBatchCmd(t, fx)
	batch.CausationID = ptr(otherEventID)
	if _, err := ProduceProductionBatch(ctx, db, batch); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("wrong batch causation: %v", err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.production_batches`) != 0 {
		t.Fatal("batch inserted on causation failure")
	}

	if _, err := ProduceProductionBatch(ctx, db, mustBatchCmd(t, fx)); err != nil {
		t.Fatal(err)
	}

	ship := mustShipCmd(t, fx)
	ship.CausationID = nil
	if _, err := DispatchShipment(ctx, db, ship); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("nil shipment causation: %v", err)
	}
	ship = mustShipCmd(t, fx)
	ship.CausationID = ptr("not-a-uuid")
	if _, err := DispatchShipment(ctx, db, ship); !errors.Is(err, ErrTenantMismatch) {
		t.Fatalf("malformed shipment causation: %v", err)
	}
	ship = mustShipCmd(t, fx)
	ship.CausationID = ptr(otherEventID)
	if _, err := DispatchShipment(ctx, db, ship); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("wrong shipment causation: %v", err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.shipments`) != 0 {
		t.Fatal("shipment inserted on causation failure")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 3 {
		t.Fatal("expected only successful hops in outbox")
	}
}

func TestLineageShipmentBeforeOrder(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	acceptThroughShipment(t, db, fx)

	if countRows(t, db, `SELECT COUNT(*) FROM erp.shipments`) != 1 {
		t.Fatal("expected one shipment")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.orders`) != 0 {
		t.Fatal("order must not exist before AcceptOrder")
	}

	if _, err := AcceptOrder(context.Background(), db, mustLineageOrderCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.orders`) != 1 {
		t.Fatal("expected one order after accept")
	}
}

func TestLineageInsufficientInventoryWithShipmentCausation(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	acceptThroughShipment(t, db, fx)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		UPDATE erp.inventory_items
		SET quantity_on_hand = 1
		WHERE tenant_id = $1 AND item_id = $2
	`, fx.TenantID, fx.ItemID); err != nil {
		t.Fatal(err)
	}

	if _, err := AcceptOrder(ctx, db, mustLineageOrderCmd(t, fx)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("insufficient inventory: %v", err)
	}
	qty, version := inventoryState(t, db, fx.TenantID, fx.ItemID)
	if qty != 1 || version != 0 {
		t.Fatalf("inventory mutated: qty=%d version=%d", qty, version)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.orders`) != 0 {
		t.Fatal("order inserted on insufficient inventory")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.shipments`) != 1 {
		t.Fatal("shipment must remain")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 4 {
		t.Fatal("expected four lineage outbox rows")
	}
}

func TestLineageConcurrentOneToOne(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	ctx := context.Background()
	if _, err := RegisterSupplier(ctx, db, mustSupplierCmd(t, fx)); err != nil {
		t.Fatal(err)
	}

	lotA := mustLotCmd(t, fx)
	lotB := mustLotCmd(t, fx)
	lotB.LotID = "lot-flour-2026-002"
	lotB.EventID = otherEventID
	assertExclusiveInsert(t, func() error {
		_, err := ReceiveIngredientLot(ctx, db, lotA)
		return err
	}, func() error {
		_, err := ReceiveIngredientLot(ctx, db, lotB)
		return err
	})
	if countRows(t, db, `SELECT COUNT(*) FROM erp.ingredient_lots`) != 1 {
		t.Fatal("expected one lot")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 2 {
		t.Fatal("expected supplier and one lot outbox")
	}

	winnerLotID := fx.LotID
	if countRows(t, db, `SELECT COUNT(*) FROM erp.ingredient_lots WHERE lot_id = $1`, "lot-flour-2026-002") == 1 {
		winnerLotID = "lot-flour-2026-002"
	}
	lotEventID := outboxEventID(t, db, fx.TenantID, event.AggregateTypeIngredientLot, winnerLotID)

	batchA := mustBatchCmd(t, fx)
	batchA.LotID = winnerLotID
	batchA.CausationID = ptr(lotEventID)
	batchB := batchA
	batchB.BatchID = "batch-bread-002"
	batchB.EventID = otherEventID2
	assertExclusiveInsert(t, func() error {
		_, err := ProduceProductionBatch(ctx, db, batchA)
		return err
	}, func() error {
		_, err := ProduceProductionBatch(ctx, db, batchB)
		return err
	})
	if countRows(t, db, `SELECT COUNT(*) FROM erp.production_batches`) != 1 {
		t.Fatal("expected one batch")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 3 {
		t.Fatal("expected three outbox rows")
	}

	winnerBatchID := fx.BatchID
	if countRows(t, db, `SELECT COUNT(*) FROM erp.production_batches WHERE batch_id = $1`, "batch-bread-002") == 1 {
		winnerBatchID = "batch-bread-002"
	}
	batchEventID := outboxEventID(t, db, fx.TenantID, event.AggregateTypeProductionBatch, winnerBatchID)

	shipA := mustShipCmd(t, fx)
	shipA.BatchID = winnerBatchID
	shipA.CausationID = ptr(batchEventID)
	shipB := shipA
	shipB.ShipmentID = "ship-northstar-002"
	shipB.EventID = otherEventID3
	shipB.OrderID = otherOrderID
	assertExclusiveInsert(t, func() error {
		_, err := DispatchShipment(ctx, db, shipA)
		return err
	}, func() error {
		_, err := DispatchShipment(ctx, db, shipB)
		return err
	})
	if countRows(t, db, `SELECT COUNT(*) FROM erp.shipments`) != 1 {
		t.Fatal("expected one shipment")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 4 {
		t.Fatal("expected four outbox rows")
	}
}

func TestOutboxRejectsDuplicateAggregateIdentity(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	seedLineageDB(t, db, fx)
	if _, err := RegisterSupplier(context.Background(), db, mustSupplierCmd(t, fx)); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(`
		INSERT INTO erp.outbox (
			event_id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
			content_hash, event_bytes, status, recorded_at
		) VALUES ($1, $2, $3, $4, 1, 'aa', '{}', 'pending', now())
	`, otherEventID, fx.TenantID, event.AggregateTypeSupplier, fx.SupplierID)
	if err == nil {
		t.Fatal("expected unique violation on aggregate identity")
	}
	if !isUniqueViolation(err) {
		t.Fatalf("err = %v, want unique violation", err)
	}
}

func forceFailBeforeCommit(t *testing.T) {
	t.Helper()
	setTestFailBeforeCommitForTest(func(context.Context) error {
		return errors.New("forced failure before commit")
	})
	t.Cleanup(func() { setTestFailBeforeCommitForTest(nil) })
}

func outboxEventID(t *testing.T, db *sql.DB, tenantID, aggregateType, aggregateID string) string {
	t.Helper()
	var eventID string
	err := db.QueryRow(`
		SELECT event_id FROM erp.outbox
		WHERE tenant_id = $1 AND aggregate_type = $2 AND aggregate_id = $3
	`, tenantID, aggregateType, aggregateID).Scan(&eventID)
	if err != nil {
		t.Fatal(err)
	}
	return eventID
}

func assertExclusiveInsert(t *testing.T, a, b func() error) {
	t.Helper()
	ch := make(chan error, 2)
	go func() { ch <- a() }()
	go func() { ch <- b() }()
	var success, failure int
	for i := 0; i < 2; i++ {
		err := <-ch
		if err == nil {
			success++
			continue
		}
		if !errors.Is(err, ErrDuplicateSource) {
			t.Fatalf("unexpected error: %v", err)
		}
		failure++
	}
	if success != 1 || failure != 1 {
		t.Fatalf("success=%d failure=%d, want 1 and 1", success, failure)
	}
}
