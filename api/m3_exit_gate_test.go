package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/G1DO/seshatops/api"
	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/platform"
	"github.com/G1DO/seshatops/relay"
)

func TestM3TraceabilityRecoveryExitGate(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	if fx.Seed != northstar.LineageSeed {
		t.Fatalf("seed=%q", fx.Seed)
	}

	recs := make([]platform.HistoryRecord, 0, len(fx.Events)+1)
	for i, env := range fx.Events {
		if res := processEnvelope(t, db, env, int64(i+1)); res.Disposition != platform.DispositionApplied {
			t.Fatalf("apply %s: %+v", env.EventType, res)
		}
		recs = append(recs, historyRecord(t, env, int64(i+1)))
	}

	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	assertAuthorizedBatchTrace(t, ts.URL+lineagePath(fx.TenantID, fx.BatchID), sess, fx)

	denied := getWithSession(t, ts.URL+lineagePath(identity.TenantNS002UUID, fx.BatchID), sess, nil)
	assertForbiddenNoLineage(t, denied)
	missing := getWithSession(t, ts.URL+lineagePath(fx.TenantID, "batch-missing-001"), sess, nil)
	assertNotFoundNoLineage(t, missing)

	invA, err := platform.ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	linA, err := platform.ChecksumLineage(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if invA == "" || linA == "" || invA == linA {
		t.Fatalf("checksums must be distinct non-empty fields inv=%s lin=%s", invA, linA)
	}

	for i, env := range fx.Events {
		res := processEnvelope(t, db, env, int64(100+i))
		if res.Disposition != platform.DispositionDuplicateNoop {
			t.Fatalf("duplicate %s: %+v", env.EventType, res)
		}
	}
	invDup, err := platform.ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	linDup, err := platform.ChecksumLineage(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if invDup != invA || linDup != linA {
		t.Fatalf("duplicate changed checksums inv %s→%s lin %s→%s", invA, invDup, linA, linDup)
	}
	assertLineageCounts(t, db, fx.TenantID, 1, 1, 1, 1)

	other := otherMill(t, fx.Events[0])
	poisonRaw := schemaV2Bytes(t, other)
	poisonKey := []byte(relay.AggregateKey(other.TenantID, other.AggregateType, other.AggregateID))
	poison, err := platform.ProcessRecord(ctx, db, poisonKey, poisonRaw, platform.SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !poison.ShouldAck || poison.Disposition != platform.DispositionQuarantinedInvalid {
		t.Fatalf("poison = %+v", poison)
	}
	if res := processEnvelope(t, db, other, 201); res.Disposition != platform.DispositionApplied {
		t.Fatalf("unrelated mill: %+v", res)
	}
	recs = append(recs, historyRecord(t, other, 201))
	assertLineageCounts(t, db, fx.TenantID, 2, 1, 1, 1)

	ins, err := platform.InspectProcessingForTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if ins.FailuresQuarantined != 1 || len(ins.Failures) != 1 {
		t.Fatalf("inspect = %+v", ins)
	}
	if ins.Failures[0].FailureCategory != "unsupported_contract" {
		t.Fatalf("category=%q", ins.Failures[0].FailureCategory)
	}
	if ins.Failures[0].AggregateID != other.AggregateID {
		t.Fatalf("poison aggregate=%q", ins.Failures[0].AggregateID)
	}

	assertAuthorizedBatchTrace(t, ts.URL+lineagePath(fx.TenantID, fx.BatchID), sess, fx)

	invB, err := platform.ChecksumTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	linB, err := platform.ChecksumLineage(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if invB != invA {
		t.Fatalf("unrelated mill mutated inventory checksum %s→%s", invA, invB)
	}
	if linB == linA {
		t.Fatal("unrelated mill must change lineage checksum")
	}

	if err := platform.ResetDerivedState(ctx, db); err != nil {
		t.Fatal(err)
	}
	assertLineageCounts(t, db, fx.TenantID, 0, 0, 0, 0)

	rebuild, err := platform.RebuildFromHistory(ctx, db, recs, platform.RebuildOptions{TenantID: fx.TenantID})
	if err != nil {
		t.Fatal(err)
	}
	if rebuild.Status != platform.RebuildStatusComplete {
		t.Fatalf("rebuild = %+v reasons=%v", rebuild, rebuild.IncompleteReasons)
	}
	if rebuild.Checksum != invB {
		t.Fatalf("inventory A=%s B=%s", invB, rebuild.Checksum)
	}
	if rebuild.LineageChecksum != linB {
		t.Fatalf("lineage A=%s B=%s", linB, rebuild.LineageChecksum)
	}
	assertLineageCounts(t, db, fx.TenantID, 2, 1, 1, 1)
	assertAuthorizedBatchTrace(t, ts.URL+lineagePath(fx.TenantID, fx.BatchID), sess, fx)
}

func assertAuthorizedBatchTrace(t *testing.T, url string, sess identity.Session, fx northstar.LineageFixture) {
	t.Helper()
	resp := getWithSession(t, url, sess, nil)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var snap api.BatchTraceSnapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.TenantID != fx.TenantID || snap.ObservedAt == "" {
		t.Fatalf("tenant/observed = %+v", snap)
	}
	if snap.Supplier.ID != fx.SupplierID || snap.Lot.ID != fx.LotID || snap.Batch.ID != fx.BatchID {
		t.Fatalf("upstream %+v", snap)
	}
	if snap.Shipment.ID != fx.ShipmentID || snap.Shipment.OrderID != fx.OrderID {
		t.Fatalf("downstream shipment=%q order=%q", snap.Shipment.ID, snap.Shipment.OrderID)
	}
	if snap.Supplier.SourceEventID != fx.Events[0].EventID {
		t.Fatalf("source_event_id=%q", snap.Supplier.SourceEventID)
	}
	if snap.Lot.EventSchemaVersion != event.SchemaVersionV1 || snap.Batch.OccurredAt != fx.Events[2].OccurredAt {
		t.Fatalf("schema/timestamps %+v", snap)
	}
	if snap.Shipment.CorrelationID != fx.Events[3].CorrelationID || snap.Shipment.TraceID != fx.Events[3].TraceID {
		t.Fatalf("correlation/trace %+v", snap)
	}
}

func historyRecord(t *testing.T, env event.Envelope, offset int64) platform.HistoryRecord {
	t.Helper()
	return platform.HistoryRecord{
		Key:   []byte(relay.AggregateKey(env.TenantID, env.AggregateType, env.AggregateID)),
		Value: mustCanonical(t, env),
		Pos:   platform.SourcePosition{Topic: relay.Topic, Partition: 0, Offset: offset},
	}
}

func otherMill(t *testing.T, mill event.Envelope) event.Envelope {
	t.Helper()
	env := mill
	env.EventID = "628f5d78-6e64-4f5f-bd16-8e9f7c4a7011"
	env.AggregateID = "mill-other-001"
	env.CorrelationID = "628f5d78-6e64-4f5f-bd16-8e9f7c4a7001"
	env.Payload = event.SupplierRegistered{SupplierID: env.AggregateID}
	if err := event.Validate(env); err != nil {
		t.Fatal(err)
	}
	return env
}

func schemaV2Bytes(t *testing.T, env event.Envelope) []byte {
	t.Helper()
	raw := mustCanonical(t, env)
	out := bytes.Replace(raw, []byte(`"event_schema_version":1`), []byte(`"event_schema_version":2`), 1)
	if bytes.Equal(out, raw) {
		t.Fatal("schema version not replaced")
	}
	return out
}

func assertLineageCounts(t *testing.T, db *sql.DB, tenantID string, suppliers, lots, batches, shipments int) {
	t.Helper()
	got := func(table string) int {
		t.Helper()
		var n int
		q := "SELECT COUNT(*) FROM " + table + " WHERE tenant_id = $1"
		if err := db.QueryRow(q, tenantID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if s, l, b, sh := got("platform.lineage_suppliers"), got("platform.lineage_ingredient_lots"), got("platform.lineage_production_batches"), got("platform.lineage_shipments"); s != suppliers || l != lots || b != batches || sh != shipments {
		t.Fatalf("lineage counts suppliers=%d lots=%d batches=%d shipments=%d want %d/%d/%d/%d", s, l, b, sh, suppliers, lots, batches, shipments)
	}
}
