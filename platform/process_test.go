package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
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
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.processing_failures`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("failures = %d", n)
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
	_, _, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || ok {
		t.Fatalf("projection must be empty: ok=%v err=%v", ok, err)
	}
}

func TestExplicitPoisonAttemptEventuallyAcks(t *testing.T) {
	// bumpPoisonAttempt remains the explicit handler-poison path; retryable
	// processValidated failures must not enter it (see TestCommitFailuresDoNotPoisonAck).
	db := openTestDB(t)
	fx := mustFixture(t)
	raw := mustCanonical(t, fx.Event)
	pos := SourcePosition{Topic: relay.Topic, Partition: 0, Offset: 77}
	env := fx.Event
	var res Result
	var err error
	for i := 0; i < MaxHandlerAttempts; i++ {
		res, err = bumpPoisonAttempt(context.Background(), db, &env, raw, pos, "handler_poison_test", errors.New("boom"))
		if i < MaxHandlerAttempts-1 {
			if err == nil || res.ShouldAck || !errors.Is(err, ErrTransient) {
				t.Fatalf("attempt %d: want transient no-ack, got res=%+v err=%v", i+1, res, err)
			}
			continue
		}
		if err == nil || !res.ShouldAck || !errors.Is(err, ErrPoison) {
			t.Fatalf("final attempt: want poison ack, got res=%+v err=%v", res, err)
		}
	}
	_, _, ok, err := ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || ok {
		t.Fatalf("poison path must not mutate projection: ok=%v err=%v", ok, err)
	}
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
