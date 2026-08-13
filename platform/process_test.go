package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/relay"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	if dsn := os.Getenv("SESHATOPS_TEST_DATABASE_URL"); dsn != "" {
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatalf("open SESHATOPS_TEST_DATABASE_URL: %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if err := db.PingContext(ctx); err != nil {
			t.Fatalf("ping SESHATOPS_TEST_DATABASE_URL: %v", err)
		}
		resetSchemas(t, db)
		if err := erp.Migrate(ctx, db); err != nil {
			t.Fatal(err)
		}
		if err := Migrate(ctx, db); err != nil {
			t.Fatal(err)
		}
		return db
	}

	pgContainer, err := postgres.Run(ctx,
		erp.PostgresImage,
		postgres.WithDatabase("seshatops"),
		postgres.WithUsername("seshatops"),
		postgres.WithPassword("seshatops"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(2*time.Minute),
		),
	)
	if err != nil {
		t.Skipf("PostgreSQL integration tests require Docker or SESHATOPS_TEST_DATABASE_URL: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	if err := erp.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db
}

func resetSchemas(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP SCHEMA IF EXISTS platform CASCADE`); err != nil {
		t.Fatalf("drop platform schema: %v", err)
	}
	if _, err := db.Exec(`DROP SCHEMA IF EXISTS erp CASCADE`); err != nil {
		t.Fatalf("drop erp schema: %v", err)
	}
}

func mustFixture(t *testing.T) northstar.Fixture {
	t.Helper()
	fx, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	return fx
}

func mustCanonical(t *testing.T, env event.Envelope) []byte {
	t.Helper()
	b, err := event.CanonicalBytes(env)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func processNorthstar(t *testing.T, db *sql.DB, fx northstar.Fixture) Result {
	t.Helper()
	raw := mustCanonical(t, fx.Event)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 1,
	})
	if err != nil {
		t.Fatalf("ProcessRecord: %v", err)
	}
	return res
}

func TestMigrateCreatesPlatformSchema(t *testing.T) {
	db := openTestDB(t)
	var n int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = 'platform'
		  AND table_name IN ('inbox', 'inventory_projection', 'processing_failures')
	`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("expected 3 platform tables, got %d", n)
	}
}

func TestFirstValidEventAppliesProjection(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	res := processNorthstar(t, db, fx)
	if res.Disposition != DispositionApplied || !res.ShouldAck {
		t.Fatalf("result = %+v", res)
	}
	qty, ver, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok {
		t.Fatalf("projection: ok=%v err=%v", ok, err)
	}
	if qty != 8 || ver != 1 {
		t.Fatalf("projection qty=%d ver=%d", qty, ver)
	}
	disp, _, ok, err := InboxDisposition(context.Background(), db, fx.Event.EventID)
	if err != nil || !ok || disp != DispositionApplied {
		t.Fatalf("inbox disp=%q ok=%v err=%v", disp, ok, err)
	}
}

func TestIdenticalRedeliveryIsDuplicateNoop(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	_ = processNorthstar(t, db, fx)
	res := processNorthstar(t, db, fx)
	if res.Disposition != DispositionDuplicateNoop || !res.ShouldAck {
		t.Fatalf("result = %+v", res)
	}
	qty, ver, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("projection qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
	var inboxCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.inbox`).Scan(&inboxCount); err != nil {
		t.Fatal(err)
	}
	if inboxCount != 1 {
		t.Fatalf("inbox rows = %d", inboxCount)
	}
}

func TestRollbackAtomicity(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	setTestFailBeforeCommitForTest(func(context.Context) error {
		return errors.New("injected commit failure")
	})
	t.Cleanup(func() { setTestFailBeforeCommitForTest(nil) })

	raw := mustCanonical(t, fx.Event)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 1,
	})
	if err == nil || res.ShouldAck {
		t.Fatalf("expected transient failure, got res=%+v err=%v", res, err)
	}
	_, _, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("projection must not commit on rollback")
	}
	_, _, inboxOK, err := InboxDisposition(context.Background(), db, fx.Event.EventID)
	if err != nil {
		t.Fatal(err)
	}
	if inboxOK {
		t.Fatal("inbox must not commit on rollback")
	}
}

func TestCommitFailuresDoNotPoisonAck(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	setTestFailBeforeCommitForTest(func(context.Context) error {
		return errors.New("injected commit failure")
	})
	t.Cleanup(func() { setTestFailBeforeCommitForTest(nil) })

	raw := mustCanonical(t, fx.Event)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))
	pos := SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 99}
	for i := 0; i < MaxHandlerAttempts+1; i++ {
		res, err := ProcessRecord(context.Background(), db, key, raw, pos)
		if err == nil || res.ShouldAck {
			t.Fatalf("attempt %d: expected no-ack transient, got res=%+v err=%v", i+1, res, err)
		}
	}
	var failures int
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.processing_failures`).Scan(&failures); err != nil {
		t.Fatal(err)
	}
	if failures != 0 {
		t.Fatalf("commit failures must not write poison rows, got %d", failures)
	}

	setTestFailBeforeCommitForTest(nil)
	res, err := ProcessRecord(context.Background(), db, key, raw, pos)
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionApplied || !res.ShouldAck {
		t.Fatalf("recover result = %+v", res)
	}
	qty, ver, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("projection qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
}

func TestRedriveFailureWithholdsAckAndRetriesOnDuplicate(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)

	v2 := fx.Event
	v2.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b2"
	v2.AggregateVersion = 2
	v2.Payload.QuantityBefore = 8
	v2.Payload.QuantityDecremented = 1
	v2.Payload.QuantityAfter = 7
	v2.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b4"
	v2.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b3"

	raw2 := mustCanonical(t, v2)
	key := []byte(relay.AggregateKey(v2.TenantID, v2.AggregateType, v2.AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw2, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionQuarantinedGap {
		t.Fatalf("expected gap, got %s", res.Disposition)
	}

	setTestFailRedriveForTest(func() error {
		return errors.New("injected redrive failure")
	})
	t.Cleanup(func() { setTestFailRedriveForTest(nil) })

	raw1 := mustCanonical(t, fx.Event)
	res, err = ProcessRecord(context.Background(), db, key, raw1, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 11,
	})
	if err == nil || res.ShouldAck {
		t.Fatalf("expected no-ack after redrive failure, got res=%+v err=%v", res, err)
	}
	if res.Disposition != DispositionApplied {
		t.Fatalf("v1 disposition = %s", res.Disposition)
	}
	qty, ver, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("v1 projection qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
	disp, _, ok, err := InboxDisposition(context.Background(), db, v2.EventID)
	if err != nil || !ok || disp != DispositionQuarantinedGap {
		t.Fatalf("v2 still gap: disp=%q ok=%v err=%v", disp, ok, err)
	}

	setTestFailRedriveForTest(nil)
	res, err = ProcessRecord(context.Background(), db, key, raw1, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 11,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionDuplicateNoop || !res.ShouldAck {
		t.Fatalf("retry result = %+v", res)
	}
	qty, ver, ok, err = ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 7 || ver != 2 {
		t.Fatalf("after redrive retry qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
	disp, _, ok, err = InboxDisposition(context.Background(), db, v2.EventID)
	if err != nil || !ok || disp != DispositionApplied {
		t.Fatalf("v2 inbox disp=%q ok=%v err=%v", disp, ok, err)
	}
}

func TestConflictingEventIDRejected(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	_ = processNorthstar(t, db, fx)

	conflict := fx.Event
	conflict.Payload.QuantityAfter = 7
	conflict.Payload.QuantityDecremented = 3
	raw := mustCanonical(t, conflict)
	key := []byte(relay.AggregateKey(conflict.TenantID, conflict.AggregateType, conflict.AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionQuarantinedConflict || !res.ShouldAck {
		t.Fatalf("result = %+v", res)
	}
	qty, ver, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("projection mutated: qty=%d ver=%d ok=%v", qty, ver, ok)
	}
}

func TestAggregateVersionGapAndRedrive(t *testing.T) {
	// FC-011-style: missing/reordered required aggregate version is quarantined
	// and deferred, never silently skipped (M1-INV-08).
	db := openTestDB(t)
	fx := mustFixture(t)

	v2 := fx.Event
	v2.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b2"
	v2.AggregateVersion = 2
	v2.Payload.QuantityBefore = 8
	v2.Payload.QuantityDecremented = 1
	v2.Payload.QuantityAfter = 7
	v2.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b4"
	v2.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b3"

	raw2 := mustCanonical(t, v2)
	key := []byte(relay.AggregateKey(v2.TenantID, v2.AggregateType, v2.AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw2, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionQuarantinedGap || !res.ShouldAck {
		t.Fatalf("expected gap ack, got %+v", res)
	}
	_, _, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || ok {
		t.Fatalf("projection should be empty: ok=%v err=%v", ok, err)
	}

	_ = processNorthstar(t, db, fx)

	qty, ver, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok {
		t.Fatalf("projection: ok=%v err=%v", ok, err)
	}
	if qty != 7 || ver != 2 {
		t.Fatalf("after redrive qty=%d ver=%d want 7/2", qty, ver)
	}
	disp, _, ok, err := InboxDisposition(context.Background(), db, v2.EventID)
	if err != nil || !ok || disp != DispositionApplied {
		t.Fatalf("v2 inbox disp=%q ok=%v err=%v", disp, ok, err)
	}
}

func TestReorderAggregateVersionIsNotSilentlySkipped(t *testing.T) {
	// FC-011: deliver v3 before v1/v2; projection must not jump to version 3.
	db := openTestDB(t)
	fx := mustFixture(t)

	v3 := fx.Event
	v3.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b5"
	v3.AggregateVersion = 3
	v3.Payload.QuantityBefore = 7
	v3.Payload.QuantityDecremented = 1
	v3.Payload.QuantityAfter = 6
	v3.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b6"
	v3.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b7"

	raw3 := mustCanonical(t, v3)
	key := []byte(relay.AggregateKey(v3.TenantID, v3.AggregateType, v3.AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw3, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 11,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionQuarantinedGap {
		t.Fatalf("expected gap for reorder, got %s", res.Disposition)
	}
	_, _, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || ok {
		t.Fatalf("projection must remain empty across gap: ok=%v err=%v", ok, err)
	}

	_ = processNorthstar(t, db, fx)
	qty, ver, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("after v1 only qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
	disp, _, ok, err := InboxDisposition(context.Background(), db, v3.EventID)
	if err != nil || !ok || disp != DispositionQuarantinedGap {
		t.Fatalf("v3 must remain gap until v2 arrives: disp=%q ok=%v err=%v", disp, ok, err)
	}
}

func TestStaleAggregateVersionQuarantined(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	_ = processNorthstar(t, db, fx)

	stale := fx.Event
	stale.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20c1"
	stale.AggregateVersion = 1
	stale.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20c4"
	raw := mustCanonical(t, stale)
	key := []byte(relay.AggregateKey(stale.TenantID, stale.AggregateType, stale.AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionQuarantinedStale {
		t.Fatalf("disposition = %s", res.Disposition)
	}
	qty, ver, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("projection mutated: qty=%d ver=%d", qty, ver)
	}
}

func TestKeyMismatchQuarantined(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	raw := mustCanonical(t, fx.Event)
	badKey := []byte("other-tenant/inventory_item/item-flour-001")
	res, err := ProcessRecord(context.Background(), db, badKey, raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionQuarantinedMismatch || !res.ShouldAck {
		t.Fatalf("result = %+v", res)
	}
	_, _, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || ok {
		t.Fatalf("projection must be empty: ok=%v err=%v", ok, err)
	}
}

func TestMalformedEnvelopeQuarantinedWithoutProjection(t *testing.T) {
	db := openTestDB(t)
	res, err := ProcessRecord(context.Background(), db, []byte("k"), []byte(`{`), SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ShouldAck || res.Disposition != DispositionQuarantinedInvalid {
		t.Fatalf("result = %+v", res)
	}
	var category, code string
	var n int
	if err := db.QueryRow(`
		SELECT COUNT(*), MAX(failure_category), MAX(diagnostic_code)
		FROM platform.processing_failures
	`).Scan(&n, &category, &code); err != nil {
		t.Fatal(err)
	}
	if n != 1 || category != "malformed_envelope" || code != "malformed_envelope" {
		t.Fatalf("failures n=%d category=%q code=%q", n, category, code)
	}
	var proj int
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.inventory_projection`).Scan(&proj); err != nil {
		t.Fatal(err)
	}
	if proj != 0 {
		t.Fatalf("projection rows = %d", proj)
	}
}

func TestUnsupportedSchemaQuarantined(t *testing.T) {
	// FC-010-style: unsupported schema version is quarantined, never applied.
	db := openTestDB(t)
	fx := mustFixture(t)
	// Craft JSON with schema 2; event.Parse rejects unsupported versions.
	raw := []byte(`{"aggregate_id":"item-flour-001","aggregate_type":"inventory_item","aggregate_version":1,"causation_id":null,"correlation_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a2","event_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20d1","event_schema_version":2,"event_type":"inventory.quantity_decremented","occurred_at":"2026-08-07T09:00:00Z","payload":{"item_id":"item-flour-001","order_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a4","quantity_after":8,"quantity_before":10,"quantity_decremented":2},"producer":"synthetic-erp","recorded_at":"2026-08-07T09:00:00Z","tenant_id":"11111111-1111-4111-8111-111111111111","trace_id":"018f5d78-6e64-4f5f-bd16-8e9f7c4a20a3"}`)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ShouldAck {
		t.Fatalf("expected ack after unsupported quarantine, got %+v", res)
	}
	var category, code string
	if err := db.QueryRow(`
		SELECT failure_category, diagnostic_code
		FROM platform.processing_failures
		WHERE source_offset = 6
	`).Scan(&category, &code); err != nil {
		t.Fatal(err)
	}
	if category != "unsupported_contract" || code != "unsupported_contract" {
		t.Fatalf("category=%q code=%q", category, code)
	}
	_, _, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || ok {
		t.Fatalf("projection must be empty: ok=%v err=%v", ok, err)
	}
}

func TestFailureRecordSanitization(t *testing.T) {
	db := openTestDB(t)
	secret := "SUPER_SECRET_TOKEN_do_not_store"
	raw := []byte(`{"leak":"` + secret + `",`)
	sum := sha256.Sum256(raw)
	wantHash := hex.EncodeToString(sum[:])

	res, err := ProcessRecord(context.Background(), db, []byte("k"), raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 55,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ShouldAck {
		t.Fatalf("expected durable quarantine ack, got %+v", res)
	}

	var (
		eventID, tenantID, aggID, contentHash sql.NullString
		category, code, receivedHash, status  string
		topic                                 string
		partition                             int
		offset                                int64
	)
	if err := db.QueryRow(`
		SELECT event_id, tenant_id, aggregate_id, content_hash,
		       failure_category, diagnostic_code, received_bytes_hash,
		       quarantine_status, source_topic, source_partition, source_offset
		FROM platform.processing_failures
		WHERE source_offset = 55
	`).Scan(
		&eventID, &tenantID, &aggID, &contentHash,
		&category, &code, &receivedHash,
		&status, &topic, &partition, &offset,
	); err != nil {
		t.Fatal(err)
	}
	if category != "malformed_envelope" || code != "malformed_envelope" {
		t.Fatalf("category=%q code=%q", category, code)
	}
	if status != "quarantined" || receivedHash != wantHash {
		t.Fatalf("status=%q hash=%q want %q", status, receivedHash, wantHash)
	}
	if eventID.Valid || tenantID.Valid || aggID.Valid || contentHash.Valid {
		t.Fatalf("identity fields must be null for unparseable input")
	}
	if topic != relay.Topic || partition != 0 || offset != 55 {
		t.Fatalf("source position topic=%q part=%d off=%d", topic, partition, offset)
	}

	rows, err := db.Query(`
		SELECT failure_id::text, COALESCE(event_id, ''), COALESCE(tenant_id, ''),
		       COALESCE(aggregate_id, ''), COALESCE(content_hash, ''),
		       failure_category, diagnostic_code, COALESCE(received_bytes_hash, ''),
		       quarantine_status
		FROM platform.processing_failures
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cols [9]string
		if err := rows.Scan(&cols[0], &cols[1], &cols[2], &cols[3], &cols[4], &cols[5], &cols[6], &cols[7], &cols[8]); err != nil {
			t.Fatal(err)
		}
		for _, c := range cols {
			if containsSubstring(c, secret) {
				t.Fatalf("failure row leaked payload substring in %q", c)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestExplicitPoisonAttemptEventuallyAcks(t *testing.T) {
	// FC-009-style: handler poison escalates through ProcessRecord; retryable
	// processValidated failures must not enter it (see TestCommitFailuresDoNotPoisonAck).
	db := openTestDB(t)
	fx := mustFixture(t)
	raw := mustCanonical(t, fx.Event)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))
	pos := SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 77}

	setTestForceHandlerPoisonForTest(func() error {
		return errors.New("boom")
	})
	t.Cleanup(func() { setTestForceHandlerPoisonForTest(nil) })

	var res Result
	var err error
	for i := 0; i < MaxHandlerAttempts; i++ {
		res, err = ProcessRecord(context.Background(), db, key, raw, pos)
		if i < MaxHandlerAttempts-1 {
			if err == nil || res.ShouldAck || !errors.Is(err, ErrTransient) {
				t.Fatalf("attempt %d: want transient no-ack, got res=%+v err=%v", i+1, res, err)
			}
			var status string
			var attempts int
			if qerr := db.QueryRow(`
				SELECT quarantine_status, attempt_count
				FROM platform.processing_failures
				WHERE source_offset = 77
			`).Scan(&status, &attempts); qerr != nil {
				t.Fatal(qerr)
			}
			if status != "retrying" || attempts != i+1 {
				t.Fatalf("attempt %d: status=%q attempts=%d", i+1, status, attempts)
			}
			continue
		}
		if err == nil || !res.ShouldAck || !errors.Is(err, ErrPoison) {
			t.Fatalf("final attempt: want poison ack, got res=%+v err=%v", res, err)
		}
	}
	var status, category, code string
	if err := db.QueryRow(`
		SELECT quarantine_status, failure_category, diagnostic_code
		FROM platform.processing_failures
		WHERE source_offset = 77
	`).Scan(&status, &category, &code); err != nil {
		t.Fatal(err)
	}
	if status != "quarantined" || category != "handler_poison" || code != "handler_poison" {
		t.Fatalf("status=%q category=%q code=%q", status, category, code)
	}
	_, _, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || ok {
		t.Fatalf("poison path must not mutate projection: ok=%v err=%v", ok, err)
	}
}

func TestInspectProcessingVisibility(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)

	malformed := []byte(`{"not":"an-envelope"`)
	_, err := ProcessRecord(context.Background(), db, []byte("k"), malformed, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 20,
	})
	if err != nil {
		t.Fatal(err)
	}

	v2 := fx.Event
	v2.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20e2"
	v2.AggregateVersion = 2
	v2.Payload.QuantityBefore = 8
	v2.Payload.QuantityDecremented = 1
	v2.Payload.QuantityAfter = 7
	v2.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20e4"
	v2.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20e3"
	raw2 := mustCanonical(t, v2)
	keyA := []byte(relay.AggregateKey(v2.TenantID, v2.AggregateType, v2.AggregateID))
	res, err := ProcessRecord(context.Background(), db, keyA, raw2, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 21,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionQuarantinedGap {
		t.Fatalf("expected gap, got %s", res.Disposition)
	}

	other := fx.Event
	other.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20e1"
	other.AggregateID = "item-sugar-001"
	other.Payload.ItemID = "item-sugar-001"
	other.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20e5"
	other.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20e6"
	rawB := mustCanonical(t, other)
	keyB := []byte(relay.AggregateKey(other.TenantID, other.AggregateType, other.AggregateID))
	res, err = ProcessRecord(context.Background(), db, keyB, rawB, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 22,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionApplied {
		t.Fatalf("unrelated aggregate result = %+v", res)
	}

	ins, err := InspectProcessing(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if ins.Applied != 1 || ins.QuarantinedGap != 1 || ins.FailuresQuarantined != 1 {
		t.Fatalf("inspect counts = %+v", ins)
	}
	if ins.OldestGap.IsZero() || ins.OldestFailure.IsZero() {
		t.Fatalf("expected oldest timestamps, got gap=%v failure=%v", ins.OldestGap, ins.OldestFailure)
	}
	if len(ins.Failures) != 1 || ins.Failures[0].FailureCategory != "malformed_envelope" {
		t.Fatalf("failures sample = %+v", ins.Failures)
	}
	if len(ins.Gaps) != 1 || ins.Gaps[0].EventID != v2.EventID || ins.Gaps[0].AggregateVersion != 2 {
		t.Fatalf("gaps sample = %+v", ins.Gaps)
	}
	for _, f := range ins.Failures {
		if containsSubstring(f.DiagnosticCode, `"not"`) || containsSubstring(f.EventID, "an-envelope") {
			t.Fatalf("failure sample leaked payload: %+v", f)
		}
	}
}

func TestUnsafeEventDoesNotBlockUnrelatedAggregate(t *testing.T) {
	// FC-009 invariant: durable quarantine of aggregate A must not prevent
	// independent aggregate B from applying.
	db := openTestDB(t)
	fx := mustFixture(t)

	gap := fx.Event
	gap.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20f2"
	gap.AggregateVersion = 2
	gap.Payload.QuantityBefore = 8
	gap.Payload.QuantityDecremented = 1
	gap.Payload.QuantityAfter = 7
	gap.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20f4"
	gap.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20f3"
	rawGap := mustCanonical(t, gap)
	keyA := []byte(relay.AggregateKey(gap.TenantID, gap.AggregateType, gap.AggregateID))
	res, err := ProcessRecord(context.Background(), db, keyA, rawGap, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionQuarantinedGap || !res.ShouldAck {
		t.Fatalf("gap result = %+v", res)
	}

	other := fx.Event
	other.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20f1"
	other.AggregateID = "item-sugar-001"
	other.Payload.ItemID = "item-sugar-001"
	other.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20f5"
	other.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20f6"
	rawB := mustCanonical(t, other)
	keyB := []byte(relay.AggregateKey(other.TenantID, other.AggregateType, other.AggregateID))
	res, err = ProcessRecord(context.Background(), db, keyB, rawB, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 31,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionApplied || !res.ShouldAck {
		t.Fatalf("unrelated result = %+v", res)
	}
	qty, ver, ok, err := ProjectionState(context.Background(), db, other.TenantID, other.AggregateID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("unrelated projection qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
	_, _, ok, err = ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || ok {
		t.Fatalf("gapped aggregate must have empty projection: ok=%v err=%v", ok, err)
	}
	ins, err := InspectProcessing(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if ins.QuarantinedGap != 1 || ins.Applied != 1 {
		t.Fatalf("inspect = %+v", ins)
	}
}

func containsSubstring(s, sub string) bool {
	return strings.Contains(s, sub)
}

func TestChecksumEmptyAndApplied(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	empty, err := ChecksumTenant(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256Empty()
	if empty != sum {
		t.Fatalf("empty checksum = %s want %s", empty, sum)
	}
	_ = processNorthstar(t, db, fx)
	got, err := ChecksumTenant(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	want := checksumOneRow(fx.TenantID, fx.ItemID, 8, 1)
	if got != want {
		t.Fatalf("checksum = %s want %s", got, want)
	}
}

func sha256Empty() string {
	return "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
}

func checksumOneRow(tenantID, itemID string, qty, ver int64) string {
	// Mirrors CONTRACTS §8 / ChecksumTenant encoding.
	line := tenantID + "\t" + itemID + "\t" + strconv.FormatInt(qty, 10) + "\t" + strconv.FormatInt(ver, 10) + "\n"
	sum := sha256.Sum256([]byte(line))
	return hex.EncodeToString(sum[:])
}

type recordingNotifier struct {
	updates []AppliedUpdate
}

func (n *recordingNotifier) NotifyApplied(update AppliedUpdate) {
	n.updates = append(n.updates, update)
}

func installRecordingNotifier(t *testing.T) *recordingNotifier {
	t.Helper()
	n := &recordingNotifier{}
	SetAppliedNotifier(n)
	t.Cleanup(func() { SetAppliedNotifier(nil) })
	return n
}

func TestListTenantProjectionEmptyAndApplied(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	rows, err := ListTenantProjection(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("empty list len=%d", len(rows))
	}
	_ = processNorthstar(t, db, fx)
	rows, err = ListTenantProjection(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ItemID != fx.ItemID || rows[0].QuantityOnHand != 8 || rows[0].AggregateVersion != 1 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestAppliedNotifierOnCommit(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	n := installRecordingNotifier(t)
	_ = processNorthstar(t, db, fx)
	if len(n.updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(n.updates))
	}
	u := n.updates[0]
	if u.TenantID != fx.TenantID || u.ItemID != fx.ItemID || u.QuantityOnHand != 8 ||
		u.AggregateVersion != 1 || u.EventID != fx.Event.EventID {
		t.Fatalf("update = %+v", u)
	}
}

func TestAppliedNotifierSilentOnRollback(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	n := installRecordingNotifier(t)
	SetFailBeforeCommitForTest(func(context.Context) error {
		return errors.New("injected rollback")
	})
	t.Cleanup(func() { SetFailBeforeCommitForTest(nil) })

	raw := mustCanonical(t, fx.Event)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))
	_, err := ProcessRecord(context.Background(), db, key, raw, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 1,
	})
	if err == nil {
		t.Fatal("expected commit failure")
	}
	if len(n.updates) != 0 {
		t.Fatalf("notifier fired on rollback: %+v", n.updates)
	}
}

func TestAppliedNotifierSilentOnDuplicate(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	n := installRecordingNotifier(t)
	_ = processNorthstar(t, db, fx)
	_ = processNorthstar(t, db, fx)
	if len(n.updates) != 1 {
		t.Fatalf("updates = %d, want 1 (duplicate must not notify)", len(n.updates))
	}
}

func TestAppliedNotifierOnGapRedrive(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	n := installRecordingNotifier(t)

	v2 := fx.Event
	v2.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b2"
	v2.AggregateVersion = 2
	v2.Payload.QuantityBefore = 8
	v2.Payload.QuantityDecremented = 1
	v2.Payload.QuantityAfter = 7
	v2.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b4"
	v2.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b3"

	raw2 := mustCanonical(t, v2)
	key := []byte(relay.AggregateKey(v2.TenantID, v2.AggregateType, v2.AggregateID))
	res, err := ProcessRecord(context.Background(), db, key, raw2, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionQuarantinedGap {
		t.Fatalf("expected gap, got %s", res.Disposition)
	}
	if len(n.updates) != 0 {
		t.Fatalf("gap must not notify: %+v", n.updates)
	}

	_ = processNorthstar(t, db, fx)
	if len(n.updates) != 2 {
		t.Fatalf("updates = %d, want 2 (v1 apply + v2 redrive)", len(n.updates))
	}
	if n.updates[0].AggregateVersion != 1 || n.updates[0].QuantityOnHand != 8 {
		t.Fatalf("first update = %+v", n.updates[0])
	}
	if n.updates[1].AggregateVersion != 2 || n.updates[1].QuantityOnHand != 7 ||
		n.updates[1].EventID != v2.EventID {
		t.Fatalf("redrive update = %+v", n.updates[1])
	}
}

func TestInspectProcessingForTenantRejectsEmptyTenant(t *testing.T) {
	_, err := InspectProcessingForTenant(context.Background(), nil, "")
	if err == nil {
		t.Fatal("empty tenant_id must fail closed")
	}
}

func TestInspectProcessingForTenantExcludesOtherAndNullTenant(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ctx := context.Background()

	malformed := []byte(`{"not":"an-envelope"`)
	if _, err := ProcessRecord(ctx, db, []byte("k"), malformed, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 40,
	}); err != nil {
		t.Fatal(err)
	}

	v2 := fx.Event
	v2.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2142"
	v2.AggregateVersion = 2
	v2.Payload.QuantityBefore = 8
	v2.Payload.QuantityDecremented = 1
	v2.Payload.QuantityAfter = 7
	v2.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2144"
	v2.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2143"
	raw2 := mustCanonical(t, v2)
	keyA := []byte(relay.AggregateKey(v2.TenantID, v2.AggregateType, v2.AggregateID))
	res, err := ProcessRecord(ctx, db, keyA, raw2, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 41,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionQuarantinedGap {
		t.Fatalf("expected gap, got %s", res.Disposition)
	}

	otherTenant := "22222222-2222-4222-8222-222222222222"
	other := fx.Event
	other.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2141"
	other.TenantID = otherTenant
	other.AggregateID = "item-sugar-001"
	other.Payload.ItemID = "item-sugar-001"
	other.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2145"
	other.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2146"
	rawOther := mustCanonical(t, other)
	keyB := []byte(relay.AggregateKey(other.TenantID, other.AggregateType, other.AggregateID))
	res, err = ProcessRecord(ctx, db, keyB, rawOther, SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 42,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Disposition != DispositionApplied {
		t.Fatalf("other tenant result = %+v", res)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO platform.processing_failures (
			failure_id, consumer_name, event_id, tenant_id,
			failure_category, diagnostic_code, quarantine_status,
			source_topic, source_partition, source_offset, attempt_count
		) VALUES (
			'fail-null-tenant', $1, '', NULL,
			'malformed_envelope', 'unattributed_poison', 'quarantined',
			$2, 0, 43, 1
		)
	`, ConsumerName, relay.Topic)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO platform.processing_failures (
			failure_id, consumer_name, event_id, tenant_id,
			failure_category, diagnostic_code, quarantine_status,
			source_topic, source_partition, source_offset, attempt_count
		) VALUES (
			'fail-same-tenant', $1, $2, $3,
			'handler_poison', 'handler_poison', 'quarantined',
			$4, 0, 44, 1
		)
	`, ConsumerName, fx.Event.EventID, fx.TenantID, relay.Topic)
	if err != nil {
		t.Fatal(err)
	}

	global, err := InspectProcessing(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if global.Applied != 1 || global.QuarantinedGap != 1 || global.FailuresQuarantined < 3 {
		t.Fatalf("global inspect = %+v", global)
	}

	scoped, err := InspectProcessingForTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if scoped.Applied != 0 || scoped.QuarantinedGap != 1 {
		t.Fatalf("tenant inspect leaked counts = %+v", scoped)
	}
	if scoped.FailuresQuarantined != 1 || len(scoped.Failures) != 1 {
		t.Fatalf("expected one attributed tenant failure, got %+v", scoped)
	}
	if scoped.Failures[0].DiagnosticCode != "handler_poison" {
		t.Fatalf("attributed failure = %+v", scoped.Failures[0])
	}
	if len(scoped.Gaps) != 1 || scoped.Gaps[0].TenantID != fx.TenantID {
		t.Fatalf("gaps sample = %+v", scoped.Gaps)
	}
	for _, f := range scoped.Failures {
		if containsSubstring(f.DiagnosticCode, `"not"`) || containsSubstring(f.EventID, "an-envelope") {
			t.Fatalf("failure sample leaked payload: %+v", f)
		}
	}

	otherIns, err := InspectProcessingForTenant(ctx, db, otherTenant)
	if err != nil {
		t.Fatal(err)
	}
	if otherIns.Applied != 1 || otherIns.QuarantinedGap != 0 {
		t.Fatalf("other tenant inspect = %+v", otherIns)
	}
}
