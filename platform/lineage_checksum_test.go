package platform

import (
	"context"
	"database/sql"
	"testing"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/relay"
)

func historyFromEnv(t *testing.T, env event.Envelope, offset int64) HistoryRecord {
	t.Helper()
	raw := mustCanonical(t, env)
	key := []byte(relay.AggregateKey(env.TenantID, env.AggregateType, env.AggregateID))
	return HistoryRecord{
		Key:   key,
		Value: raw,
		Pos:   SourcePosition{Topic: relay.Topic, Partition: 0, Offset: offset},
	}
}

func applyAll(t *testing.T, db *sql.DB, events []event.Envelope) []HistoryRecord {
	t.Helper()
	recs := make([]HistoryRecord, 0, len(events))
	for i, env := range events {
		rec := historyFromEnv(t, env, int64(i+1))
		recs = append(recs, rec)
		res, err := ProcessRecord(context.Background(), db, rec.Key, rec.Value, rec.Pos)
		if err != nil || res.Disposition != DispositionApplied {
			t.Fatalf("apply[%d] %s: %+v err=%v", i, env.EventType, res, err)
		}
	}
	return recs
}

func foreignSupplier(t *testing.T, mill event.Envelope) event.Envelope {
	t.Helper()
	env := mill
	env.TenantID = identity.TenantNS002UUID
	env.EventID = "328f5d78-6e64-4f5f-bd16-8e9f7c4a4011"
	env.AggregateID = "mill-ns002-001"
	env.CorrelationID = "328f5d78-6e64-4f5f-bd16-8e9f7c4a4001"
	env.Payload = event.SupplierRegistered{SupplierID: env.AggregateID}
	if err := event.Validate(env); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestChecksumLineageEmptyFailsClosed(t *testing.T) {
	db := openTestDB(t)
	_, err := ChecksumLineage(context.Background(), db, "")
	if err == nil {
		t.Fatal("empty tenant must fail closed")
	}
	sum, err := ChecksumLineage(context.Background(), db, identity.TenantNS001UUID)
	if err != nil {
		t.Fatal(err)
	}
	if sum != sha256Empty() {
		t.Fatalf("empty lineage checksum = %s", sum)
	}
}

func TestChecksumLineageSeparateFromInventory(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	ctx := context.Background()

	_ = applyAll(t, db, fx.Events[:4])
	inv, err := ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if inv != sha256Empty() {
		t.Fatalf("inventory checksum mutated by lineage hops: %s", inv)
	}
	lin, err := ChecksumLineage(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if lin == sha256Empty() {
		t.Fatal("lineage checksum empty after hops")
	}
	again, err := ChecksumLineage(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if again != lin {
		t.Fatalf("lineage checksum not repeatable: %s vs %s", lin, again)
	}

	if res := processRecordEnv(t, db, fx.Events[4], 5); res.Disposition != DispositionApplied {
		t.Fatalf("inventory hop: %+v", res)
	}
	inv2, err := ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	wantInv := checksumOneRow(fx.TenantID, fx.ItemID, 8, 1)
	if inv2 != wantInv {
		t.Fatalf("inventory checksum = %s want %s", inv2, wantInv)
	}
	lin2, err := ChecksumLineage(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if lin2 != lin {
		t.Fatalf("inventory hop mutated lineage checksum: %s vs %s", lin2, lin)
	}
}

func TestLineageRebuildChecksumEquality(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	ctx := context.Background()
	recs := applyAll(t, db, fx.Events)
	invA, err := ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	linA, err := ChecksumLineage(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}

	if err := ResetDerivedState(ctx, db); err != nil {
		t.Fatal(err)
	}
	assertLineageCounts(t, db, fx.TenantID, 0, 0, 0, 0)
	emptyLin, err := ChecksumLineage(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if emptyLin != sha256Empty() {
		t.Fatalf("lineage checksum after reset = %s", emptyLin)
	}

	rebuild, err := RebuildFromHistory(ctx, db, recs, RebuildOptions{TenantID: fx.TenantID})
	if err != nil {
		t.Fatal(err)
	}
	if rebuild.Status != RebuildStatusComplete {
		t.Fatalf("rebuild = %+v reasons=%v", rebuild, rebuild.IncompleteReasons)
	}
	if rebuild.Checksum != invA {
		t.Fatalf("inventory A=%s B=%s", invA, rebuild.Checksum)
	}
	if rebuild.LineageChecksum != linA {
		t.Fatalf("lineage A=%s B=%s", linA, rebuild.LineageChecksum)
	}
	assertLineageCounts(t, db, fx.TenantID, 1, 1, 1, 1)
}

func TestLineageDuplicateDoesNotChangeChecksums(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	ctx := context.Background()
	recs := applyAll(t, db, fx.Events[:4])
	invA, err := ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	linA, err := ChecksumLineage(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}

	for i, rec := range recs {
		rec.Pos.Offset = int64(100 + i)
		res, err := ProcessRecord(ctx, db, rec.Key, rec.Value, rec.Pos)
		if err != nil || res.Disposition != DispositionDuplicateNoop {
			t.Fatalf("duplicate[%d]: %+v err=%v", i, res, err)
		}
	}
	invB, err := ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	linB, err := ChecksumLineage(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if invA != invB || linA != linB {
		t.Fatalf("duplicate changed checksums inv %s→%s lin %s→%s", invA, invB, linA, linB)
	}
}

func TestLineageRebuildLeavesOtherTenant(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	ctx := context.Background()
	_ = applyAll(t, db, fx.Events[:4])
	foreign := foreignSupplier(t, fx.Events[0])
	if res := processRecordEnv(t, db, foreign, 20); res.Disposition != DispositionApplied {
		t.Fatalf("foreign: %+v", res)
	}
	for _, env := range fx.Events[:4] {
		insertTenantOutbox(t, db, env)
	}
	foreignLin, err := ChecksumLineage(ctx, db, foreign.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	homeLin, err := ChecksumLineage(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}

	rebuild, err := RebuildTenantFromHistory(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if rebuild.Status != RebuildStatusComplete || rebuild.LineageChecksum != homeLin {
		t.Fatalf("home rebuild = %+v want lineage %s", rebuild, homeLin)
	}
	got, err := ChecksumLineage(ctx, db, foreign.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if got != foreignLin {
		t.Fatalf("foreign lineage mutated: %s vs %s", got, foreignLin)
	}
}

func TestLineageRebuildRejectsInvalidHistory(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	ctx := context.Background()
	supplier := fx.Events[0]
	rec1 := historyFromEnv(t, supplier, 1)
	conflict := supplier
	conflict.OccurredAt = "2026-08-07T11:00:00Z"
	rec2 := historyFromEnv(t, conflict, 2)

	rebuild, err := RebuildFromHistory(ctx, db, []HistoryRecord{rec1, rec2}, RebuildOptions{
		TenantID: fx.TenantID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rebuild.Status != RebuildStatusIncomplete {
		t.Fatalf("expected incomplete, got %+v", rebuild)
	}
	assertLineageCounts(t, db, fx.TenantID, 1, 0, 0, 0)
	lin, err := ChecksumLineage(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if lin == sha256Empty() || rebuild.LineageChecksum != lin {
		t.Fatalf("diagnostic lineage checksum = %s result=%s", lin, rebuild.LineageChecksum)
	}
	if rebuild.Checksum != sha256Empty() {
		t.Fatalf("inventory checksum = %s", rebuild.Checksum)
	}
}
