package platform

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/relay"
)

func otherSupplier(t *testing.T, mill event.Envelope) event.Envelope {
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

func TestLineageMalformedQuarantinedNoLineageEffect(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	res, err := ProcessRecord(context.Background(), db, []byte("k"), []byte(`{`), SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ShouldAck || res.Disposition != DispositionQuarantinedInvalid {
		t.Fatalf("result = %+v", res)
	}
	assertLineageCounts(t, db, fx.TenantID, 0, 0, 0, 0)
	if res := processRecordEnv(t, db, fx.Events[0], 201); res.Disposition != DispositionApplied {
		t.Fatalf("valid supplier after malformed: %+v", res)
	}
	assertLineageCounts(t, db, fx.TenantID, 1, 0, 0, 0)
}

func TestLineageUnsupportedSchemaQuarantinedNoLineageEffect(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	supplier := fx.Events[0]
	raw := schemaV2Bytes(t, supplier)
	key := []byte(relay.AggregateKey(supplier.TenantID, supplier.AggregateType, supplier.AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 202,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ShouldAck || res.Disposition != DispositionQuarantinedInvalid {
		t.Fatalf("result = %+v", res)
	}
	assertLineageCounts(t, db, fx.TenantID, 0, 0, 0, 0)

	var category, tenantID, eventType, aggID string
	if err := db.QueryRow(`
		SELECT failure_category, tenant_id, event_type, aggregate_id
		FROM platform.processing_failures
		WHERE source_offset = 202
	`).Scan(&category, &tenantID, &eventType, &aggID); err != nil {
		t.Fatal(err)
	}
	if category != "unsupported_contract" || tenantID != fx.TenantID {
		t.Fatalf("failure category=%q tenant=%q", category, tenantID)
	}
	if eventType != supplier.EventType || aggID != supplier.AggregateID {
		t.Fatalf("failure type=%q agg=%q", eventType, aggID)
	}

	ins, err := InspectProcessingForTenant(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if ins.FailuresQuarantined != 1 || len(ins.Failures) != 1 {
		t.Fatalf("inspect = %+v", ins)
	}
	if ins.Failures[0].TenantID != fx.TenantID || ins.Failures[0].AggregateID != supplier.AggregateID {
		t.Fatalf("inspect identity = %+v", ins.Failures[0])
	}
	if ins.Failures[0].EventType != supplier.EventType || ins.Failures[0].AggregateType != supplier.AggregateType {
		t.Fatalf("inspect type = %+v", ins.Failures[0])
	}

	if res := processRecordEnv(t, db, otherSupplier(t, supplier), 203); res.Disposition != DispositionApplied {
		t.Fatalf("unrelated supplier: %+v", res)
	}
	if res := processRecordEnv(t, db, supplier, 204); res.Disposition != DispositionApplied {
		t.Fatalf("valid schema after unsupported: %+v", res)
	}
	assertLineageCounts(t, db, fx.TenantID, 2, 0, 0, 0)
}

func TestLineageConflictingEventIDQuarantinedNoLineageEffect(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	supplier := fx.Events[0]
	if res := processRecordEnv(t, db, supplier, 205); res.Disposition != DispositionApplied {
		t.Fatalf("apply: %+v", res)
	}

	conflict := supplier
	conflict.OccurredAt = "2026-08-07T11:00:00Z"
	if err := event.Validate(conflict); err != nil {
		t.Fatal(err)
	}
	res := processRecordEnv(t, db, conflict, 206)
	if res.Disposition != DispositionQuarantinedConflict || !res.ShouldAck {
		t.Fatalf("conflict = %+v", res)
	}
	assertLineageCounts(t, db, fx.TenantID, 1, 0, 0, 0)

	trace, ok, err := TraceBatch(context.Background(), db, fx.TenantID, fx.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("batch must be missing: %+v", trace)
	}
	if res := processRecordEnv(t, db, otherSupplier(t, supplier), 207); res.Disposition != DispositionApplied {
		t.Fatalf("unrelated supplier: %+v", res)
	}
	assertLineageCounts(t, db, fx.TenantID, 2, 0, 0, 0)
}

func TestLineagePoisonDoesNotBlockUnrelatedHop(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	supplier := fx.Events[0]
	raw := mustCanonical(t, supplier)
	key := []byte(relay.AggregateKey(supplier.TenantID, supplier.AggregateType, supplier.AggregateID))
	pos := SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 210}

	setTestForceHandlerPoisonForTest(func() error {
		return errors.New("lineage handler boom")
	})
	t.Cleanup(func() { setTestForceHandlerPoisonForTest(nil) })

	var res Result
	var err error
	for i := 0; i < MaxHandlerAttempts; i++ {
		res, err = ProcessRecord(context.Background(), db, key, raw, pos)
		if i < MaxHandlerAttempts-1 {
			if err == nil || res.ShouldAck || !errors.Is(err, ErrTransient) {
				t.Fatalf("attempt %d: %+v err=%v", i+1, res, err)
			}
			continue
		}
		if err == nil || !res.ShouldAck || !errors.Is(err, ErrPoison) {
			t.Fatalf("final attempt: %+v err=%v", res, err)
		}
	}
	setTestForceHandlerPoisonForTest(nil)
	assertLineageCounts(t, db, fx.TenantID, 0, 0, 0, 0)

	if res := processRecordEnv(t, db, otherSupplier(t, supplier), 211); res.Disposition != DispositionApplied {
		t.Fatalf("unrelated supplier: %+v", res)
	}
	if res := processRecordEnv(t, db, fx.Events[1], 212); res.Disposition != DispositionApplied {
		t.Fatalf("lot after poison: %+v", res)
	}
	assertLineageCounts(t, db, fx.TenantID, 1, 1, 0, 0)

	var poisoned int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM platform.lineage_suppliers
		WHERE tenant_id = $1 AND supplier_id = $2
	`, fx.TenantID, fx.SupplierID).Scan(&poisoned); err != nil {
		t.Fatal(err)
	}
	if poisoned != 0 {
		t.Fatalf("poisoned mill projected: %d", poisoned)
	}

	ins, err := InspectProcessingForTenant(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if ins.Applied != 2 || ins.FailuresQuarantined != 1 {
		t.Fatalf("inspect = %+v", ins)
	}
	if len(ins.Failures) != 1 || ins.Failures[0].AggregateID != fx.SupplierID {
		t.Fatalf("poison identity = %+v", ins.Failures)
	}
	if ins.Failures[0].EventType != supplier.EventType || ins.Failures[0].TenantID != fx.TenantID {
		t.Fatalf("poison sample = %+v", ins.Failures[0])
	}
}

func TestLineageFailureRecordIsSanitized(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineage(t)
	secret := "SUPER_SECRET_TOKEN_do_not_store"
	raw := schemaV2Bytes(t, fx.Events[0])
	raw = bytes.Replace(raw,
		[]byte(`{"supplier_id":"mill-northstar-001"}`),
		[]byte(`{"secret":"`+secret+`","supplier_id":"mill-northstar-001"}`),
		1)
	if !bytes.Contains(raw, []byte(secret)) {
		t.Fatal("secret not in input")
	}
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Events[0].AggregateType, fx.Events[0].AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 220,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ShouldAck {
		t.Fatalf("result = %+v", res)
	}
	assertLineageCounts(t, db, fx.TenantID, 0, 0, 0, 0)
	assertFailuresOmitSecret(t, db, secret)

	ins, err := InspectProcessingForTenant(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ins.Failures) != 1 {
		t.Fatalf("failures = %+v", ins.Failures)
	}
	s := ins.Failures[0]
	for _, part := range []string{s.FailureID, s.EventID, s.TenantID, s.AggregateType, s.AggregateID, s.EventType, s.DiagnosticCode, s.FailureCategory} {
		if strings.Contains(part, secret) {
			t.Fatalf("inspect leaked secret: %+v", s)
		}
	}
}

func assertLineageCounts(t *testing.T, db *sql.DB, tenantID string, suppliers, lots, batches, shipments int) {
	t.Helper()
	gotS, gotL, gotB, gotH, err := countLineageRows(context.Background(), db, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if gotS != suppliers || gotL != lots || gotB != batches || gotH != shipments {
		t.Fatalf("lineage counts suppliers=%d lots=%d batches=%d shipments=%d want %d %d %d %d",
			gotS, gotL, gotB, gotH, suppliers, lots, batches, shipments)
	}
}

func assertFailuresOmitSecret(t *testing.T, db *sql.DB, secret string) {
	t.Helper()
	rows, err := db.Query(`
		SELECT failure_id::text, COALESCE(event_id, ''), COALESCE(tenant_id, ''),
		       COALESCE(aggregate_type, ''), COALESCE(aggregate_id, ''),
		       COALESCE(event_type, ''), COALESCE(content_hash, ''),
		       failure_category, diagnostic_code, COALESCE(received_bytes_hash, ''),
		       quarantine_status
		FROM platform.processing_failures
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var n int
	for rows.Next() {
		n++
		var cols [11]string
		if err := rows.Scan(&cols[0], &cols[1], &cols[2], &cols[3], &cols[4], &cols[5], &cols[6], &cols[7], &cols[8], &cols[9], &cols[10]); err != nil {
			t.Fatal(err)
		}
		for _, c := range cols {
			if strings.Contains(c, secret) {
				t.Fatalf("failure row leaked payload substring in %q", c)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("expected a failure row")
	}
}
