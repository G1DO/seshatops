package relay

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
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
		if _, err := db.Exec(`DROP SCHEMA IF EXISTS erp CASCADE`); err != nil {
			t.Fatalf("drop erp schema: %v", err)
		}
		if err := erp.Migrate(ctx, db); err != nil {
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
	return db
}

func mustOrderCommand(t *testing.T, fx northstar.Fixture) erp.OrderCommand {
	t.Helper()
	cmd, err := erp.OrderCommandFromFixture(fx)
	if err != nil {
		t.Fatal(err)
	}
	return cmd
}

func seedAndAccept(t *testing.T, db *sql.DB) (northstar.Fixture, erp.AcceptResult) {
	t.Helper()
	fx, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	res, err := erp.AcceptOrder(ctx, db, mustOrderCommand(t, fx))
	if err != nil {
		t.Fatal(err)
	}
	return fx, res
}

func outboxStatus(t *testing.T, db *sql.DB, eventID string) (status string, attempts int, leaseOwner sql.NullString, leaseExp sql.NullTime) {
	t.Helper()
	err := db.QueryRow(`
		SELECT status, publish_attempts, publish_lease_owner, publish_lease_expires_at
		FROM erp.outbox WHERE event_id = $1
	`, eventID).Scan(&status, &attempts, &leaseOwner, &leaseExp)
	if err != nil {
		t.Fatal(err)
	}
	return status, attempts, leaseOwner, leaseExp
}

func expireLease(t *testing.T, db *sql.DB, eventID string) {
	t.Helper()
	_, err := db.Exec(`
		UPDATE erp.outbox
		SET publish_lease_expires_at = now() - interval '1 second'
		WHERE event_id = $1
	`, eventID)
	if err != nil {
		t.Fatal(err)
	}
}

func clearBackoff(t *testing.T, db *sql.DB, eventID string) {
	t.Helper()
	_, err := db.Exec(`
		UPDATE erp.outbox
		SET publish_lease_expires_at = NULL
		WHERE event_id = $1 AND status = 'pending'
	`, eventID)
	if err != nil {
		t.Fatal(err)
	}
}

type recordingPublisher struct {
	mu       sync.Mutex
	records  []publishedMsg
	failErr  error
	failOnce bool
	failed   bool
}

type publishedMsg struct {
	Topic string
	Key   []byte
	Value []byte
}

func (p *recordingPublisher) Publish(ctx context.Context, topic string, key, value []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failErr != nil {
		if p.failOnce {
			if !p.failed {
				p.failed = true
				return p.failErr
			}
		} else {
			return p.failErr
		}
	}
	p.records = append(p.records, publishedMsg{
		Topic: topic,
		Key:   append([]byte(nil), key...),
		Value: append([]byte(nil), value...),
	})
	return nil
}

func (p *recordingPublisher) snapshot() []publishedMsg {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]publishedMsg, len(p.records))
	copy(out, p.records)
	return out
}

func TestRedpandaImagePinDocumented(t *testing.T) {
	if RedpandaVersionLabel != "v25.2.1" {
		t.Fatalf("version label = %s", RedpandaVersionLabel)
	}
	want := "docker.redpanda.com/redpandadata/redpanda@sha256:218469e5d088757bb2c3ff4c5e272f7eebdc4e94c933e6e15aff10b845cbcd07"
	if RedpandaImage != want {
		t.Fatalf("unexpected image pin: %s", RedpandaImage)
	}
}

func TestBackoffAfterAttempts(t *testing.T) {
	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{7, 60 * time.Second},
		{8, 60 * time.Second},
		{0, time.Second},
	}
	for _, tc := range cases {
		if got := BackoffAfterAttempts(tc.attempts); got != tc.want {
			t.Fatalf("attempts=%d backoff=%s want %s", tc.attempts, got, tc.want)
		}
	}
}

func TestAggregateKey(t *testing.T) {
	got := AggregateKey("t", "inventory_item", "item-1")
	if got != "t/inventory_item/item-1" {
		t.Fatalf("key = %q", got)
	}
}

func TestClaimDueExclusiveLeaseAndExpireReclaim(t *testing.T) {
	db := openTestDB(t)
	fx, res := seedAndAccept(t, db)
	ctx := context.Background()

	first, err := ClaimDue(ctx, db, "worker-a", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].EventID != res.EventID {
		t.Fatalf("first claim: %+v", first)
	}

	second, err := ClaimDue(ctx, db, "worker-b", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("live lease should block claim, got %d", len(second))
	}

	expireLease(t, db, res.EventID)
	third, err := ClaimDue(ctx, db, "worker-b", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(third) != 1 || third[0].EventID != fx.Event.EventID {
		t.Fatalf("expired reclaim: %+v", third)
	}
	if string(third[0].EventBytes) != string(res.EventBytes) {
		t.Fatal("reclaim rewrote event bytes")
	}
	status, attempts, owner, _ := outboxStatus(t, db, res.EventID)
	if status != StatusPublishing || attempts != 2 || owner.String != "worker-b" {
		t.Fatalf("status=%s attempts=%d owner=%v", status, attempts, owner)
	}
}

func TestMarkPublishedRequiresOwner(t *testing.T) {
	db := openTestDB(t)
	_, res := seedAndAccept(t, db)
	ctx := context.Background()
	if _, err := ClaimDue(ctx, db, "worker-a", time.Minute, 1); err != nil {
		t.Fatal(err)
	}
	if err := MarkPublished(ctx, db, res.EventID, "other"); err == nil {
		t.Fatal("expected owner mismatch failure")
	}
	if err := MarkPublished(ctx, db, res.EventID, "worker-a"); err != nil {
		t.Fatal(err)
	}
	status, _, _, _ := outboxStatus(t, db, res.EventID)
	if status != StatusPublished {
		t.Fatalf("status = %s", status)
	}
}

func TestReleaseTransientAppliesBackoff(t *testing.T) {
	db := openTestDB(t)
	_, res := seedAndAccept(t, db)
	ctx := context.Background()
	if _, err := ClaimDue(ctx, db, "worker-a", time.Minute, 1); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseTransient(ctx, db, res.EventID, "worker-a", "broker_publish_failed"); err != nil {
		t.Fatal(err)
	}
	status, attempts, owner, exp := outboxStatus(t, db, res.EventID)
	if status != StatusPending || attempts != 1 || owner.Valid {
		t.Fatalf("status=%s attempts=%d owner=%v", status, attempts, owner)
	}
	if !exp.Valid || !exp.Time.After(time.Now().UTC()) {
		t.Fatalf("expected future backoff expiry, got %v", exp)
	}
	blocked, err := ClaimDue(ctx, db, "worker-b", time.Minute, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocked) != 0 {
		t.Fatal("backoff should keep row undrainable")
	}
}

func TestQuarantineIsDurable(t *testing.T) {
	db := openTestDB(t)
	_, res := seedAndAccept(t, db)
	ctx := context.Background()
	if _, err := ClaimDue(ctx, db, "worker-a", time.Minute, 1); err != nil {
		t.Fatal(err)
	}
	if err := Quarantine(ctx, db, res.EventID, "worker-a", "malformed_envelope"); err != nil {
		t.Fatal(err)
	}
	status, _, _, _ := outboxStatus(t, db, res.EventID)
	if status != StatusQuarantined {
		t.Fatalf("status = %s", status)
	}
	b, err := InspectBacklog(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if b.Quarantined != 1 || len(b.Quarantines) != 1 || b.Quarantines[0].LastErrorCode != "malformed_envelope" {
		t.Fatalf("backlog = %+v", b)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 1 {
		t.Fatal("quarantine must not delete rows")
	}
}

func TestReleaseQuarantinedReturnsPendingForSameTenant(t *testing.T) {
	db := openTestDB(t)
	fx, res := seedAndAccept(t, db)
	ctx := context.Background()
	if _, err := ClaimDue(ctx, db, "worker-a", time.Minute, 1); err != nil {
		t.Fatal(err)
	}
	if err := Quarantine(ctx, db, res.EventID, "worker-a", "malformed_envelope"); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseQuarantined(ctx, db, fx.TenantID, res.EventID); err != nil {
		t.Fatal(err)
	}
	status, _, owner, exp := outboxStatus(t, db, res.EventID)
	if status != StatusPending {
		t.Fatalf("status = %s", status)
	}
	if owner.Valid || exp.Valid {
		t.Fatalf("lease still set owner=%v exp=%v", owner, exp)
	}
}

func TestReleaseQuarantinedRejectsWrongTenant(t *testing.T) {
	db := openTestDB(t)
	_, res := seedAndAccept(t, db)
	ctx := context.Background()
	if _, err := ClaimDue(ctx, db, "worker-a", time.Minute, 1); err != nil {
		t.Fatal(err)
	}
	if err := Quarantine(ctx, db, res.EventID, "worker-a", "malformed_envelope"); err != nil {
		t.Fatal(err)
	}
	err := ReleaseQuarantined(ctx, db, "22222222-2222-4222-8222-222222222222", res.EventID)
	if !errors.Is(err, ErrQuarantineNotFound) {
		t.Fatalf("err=%v", err)
	}
	status, _, _, _ := outboxStatus(t, db, res.EventID)
	if status != StatusQuarantined {
		t.Fatalf("status = %s", status)
	}
}

func TestDrainOncePublishesExactBytes(t *testing.T) {
	db := openTestDB(t)
	fx, res := seedAndAccept(t, db)
	ctx := context.Background()
	pub := &recordingPublisher{}
	out, err := DrainOnce(ctx, db, pub, "worker-a", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if out.Claimed != 1 || out.Published != 1 {
		t.Fatalf("drain result %+v", out)
	}
	msgs := pub.snapshot()
	if len(msgs) != 1 {
		t.Fatalf("published %d", len(msgs))
	}
	wantKey := AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID)
	if msgs[0].Topic != Topic || string(msgs[0].Key) != wantKey {
		t.Fatalf("topic/key = %s %q", msgs[0].Topic, msgs[0].Key)
	}
	if string(msgs[0].Value) != string(res.EventBytes) {
		t.Fatal("value bytes rewritten")
	}
	status, _, _, _ := outboxStatus(t, db, res.EventID)
	if status != StatusPublished {
		t.Fatalf("status = %s", status)
	}
	b, err := InspectBacklog(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if b.Published != 1 || b.Pending != 0 {
		t.Fatalf("backlog %+v", b)
	}
}

func TestDrainOnceTransientBrokerFailure(t *testing.T) {
	db := openTestDB(t)
	_, res := seedAndAccept(t, db)
	ctx := context.Background()
	pub := &recordingPublisher{failErr: errors.New("broker down")}
	out, err := DrainOnce(ctx, db, pub, "worker-a", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if out.Transient != 1 || out.Published != 0 {
		t.Fatalf("drain result %+v", out)
	}
	status, _, _, _ := outboxStatus(t, db, res.EventID)
	if status != StatusPending {
		t.Fatalf("status = %s", status)
	}
	b, err := InspectBacklog(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if b.Pending != 1 || b.OldestUnpublished.IsZero() {
		t.Fatalf("expected pending backlog, got %+v", b)
	}
}

func TestDrainOnceQuarantinesCorruptBytes(t *testing.T) {
	db := openTestDB(t)
	_, res := seedAndAccept(t, db)
	ctx := context.Background()
	_, err := db.Exec(`UPDATE erp.outbox SET event_bytes = $1 WHERE event_id = $2`, []byte(`{`), res.EventID)
	if err != nil {
		t.Fatal(err)
	}
	pub := &recordingPublisher{}
	out, err := DrainOnce(ctx, db, pub, "worker-a", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if out.Quarantined != 1 || len(pub.snapshot()) != 0 {
		t.Fatalf("drain %+v pubs=%d", out, len(pub.snapshot()))
	}
	status, _, _, _ := outboxStatus(t, db, res.EventID)
	if status != StatusQuarantined {
		t.Fatalf("status = %s", status)
	}
}

func TestAmbiguousWindowAllowsDuplicatePublish(t *testing.T) {
	db := openTestDB(t)
	fx, res := seedAndAccept(t, db)
	ctx := context.Background()
	pub := &recordingPublisher{}

	claimed, err := ClaimDue(ctx, db, "worker-a", time.Minute, 1)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v %+v", err, claimed)
	}
	key := []byte(AggregateKey(claimed[0].TenantID, claimed[0].AggregateType, claimed[0].AggregateID))
	if err := pub.Publish(ctx, Topic, key, claimed[0].EventBytes); err != nil {
		t.Fatal(err)
	}
	// Simulate crash after broker ACK before MarkPublished.
	expireLease(t, db, res.EventID)

	out, err := DrainOnce(ctx, db, pub, "worker-b", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if out.Published != 1 {
		t.Fatalf("second drain %+v", out)
	}
	msgs := pub.snapshot()
	if len(msgs) != 2 {
		t.Fatalf("want duplicate publications, got %d", len(msgs))
	}
	if string(msgs[0].Value) != string(msgs[1].Value) || string(msgs[0].Key) != string(msgs[1].Key) {
		t.Fatal("duplicate publications must preserve identity and content")
	}
	parsed, err := event.Parse(msgs[1].Value)
	if err != nil {
		t.Fatal(err)
	}
	if err := event.CheckIdentityConflict(parsed, fx.Event); err != nil {
		t.Fatal(err)
	}
}

func TestDrainOnceAmbiguousMarkPublishedFailure(t *testing.T) {
	db := openTestDB(t)
	_, res := seedAndAccept(t, db)
	ctx := context.Background()
	pub := &recordingPublisher{}
	setTestSkipMarkPublishedForTest(func() error {
		return errors.New("simulated crash after broker ack")
	})
	t.Cleanup(func() { setTestSkipMarkPublishedForTest(nil) })

	out, err := DrainOnce(ctx, db, pub, "worker-a", time.Minute, 1)
	if err != nil {
		t.Fatal(err)
	}
	if out.Ambiguous != 1 || out.Published != 0 || len(pub.snapshot()) != 1 {
		t.Fatalf("drain %+v pubs=%d", out, len(pub.snapshot()))
	}
	status, _, owner, _ := outboxStatus(t, db, res.EventID)
	if status != StatusPublishing || !owner.Valid || owner.String != "worker-a" {
		t.Fatalf("status=%s owner=%v; intent must remain publishing", status, owner)
	}
}

func TestDrainOnceQuarantinesKeyMismatch(t *testing.T) {
	db := openTestDB(t)
	_, res := seedAndAccept(t, db)
	ctx := context.Background()
	_, err := db.Exec(`UPDATE erp.outbox SET aggregate_id = 'item-other-001' WHERE event_id = $1`, res.EventID)
	if err != nil {
		t.Fatal(err)
	}
	pub := &recordingPublisher{}
	out, err := DrainOnce(ctx, db, pub, "worker-a", time.Minute, 1)
	if err != nil {
		t.Fatal(err)
	}
	if out.Quarantined != 1 || len(pub.snapshot()) != 0 {
		t.Fatalf("drain %+v pubs=%d", out, len(pub.snapshot()))
	}
	status, _, _, _ := outboxStatus(t, db, res.EventID)
	if status != StatusQuarantined {
		t.Fatalf("status = %s", status)
	}
}

func TestRelayRestartAfterClaimWithoutPublish(t *testing.T) {
	db := openTestDB(t)
	fx, res := seedAndAccept(t, db)
	ctx := context.Background()
	if _, err := ClaimDue(ctx, db, "crashed-worker", time.Minute, 1); err != nil {
		t.Fatal(err)
	}
	expireLease(t, db, res.EventID)
	pub := &recordingPublisher{}
	out, err := DrainOnce(ctx, db, pub, "restarted-worker", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if out.Published != 1 {
		t.Fatalf("drain %+v", out)
	}
	msgs := pub.snapshot()
	if len(msgs) != 1 || string(msgs[0].Value) != string(res.EventBytes) {
		t.Fatal("restart lost or rewrote event")
	}
	parsed, err := event.Parse(msgs[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	if err := event.CheckIdentityConflict(parsed, fx.Event); err != nil {
		t.Fatal(err)
	}
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAcceptSurvivesUnreachableBroker(t *testing.T) {
	db := openTestDB(t)
	fx, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	res, err := erp.AcceptOrder(ctx, db, mustOrderCommand(t, fx))
	if err != nil {
		t.Fatal(err)
	}
	pub, err := NewFranzPublisher("127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pub.Close)

	drainCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := DrainOnce(drainCtx, db, pub, "worker-a", time.Minute, 1)
	if err != nil {
		t.Fatal(err)
	}
	if out.Transient != 1 {
		t.Fatalf("expected transient failure, got %+v", out)
	}
	status, _, _, _ := outboxStatus(t, db, res.EventID)
	if status != StatusPending {
		t.Fatalf("status = %s", status)
	}
	b, err := InspectBacklog(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if b.Pending != 1 {
		t.Fatalf("backlog %+v", b)
	}
}

func TestInspectBacklogForTenantRejectsEmptyTenant(t *testing.T) {
	_, err := InspectBacklogForTenant(context.Background(), nil, "")
	if err == nil {
		t.Fatal("empty tenant_id must fail closed")
	}
}

func TestInspectBacklogForTenantExcludesOtherTenants(t *testing.T) {
	db := openTestDB(t)
	fx, _ := seedAndAccept(t, db)
	ctx := context.Background()
	otherTenant := "22222222-2222-4222-8222-222222222222"
	otherEvent := "018f5d78-6e64-4f5f-bd16-8e9f7c4a2099"
	_, err := db.ExecContext(ctx, `
		INSERT INTO erp.outbox (
			event_id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
			content_hash, event_bytes, status, recorded_at, last_error_code
		) VALUES (
			$1, $2, 'inventory_item', 'item-flour-001', 1,
			'aa', '{}', 'quarantined', now(), 'other_tenant_poison'
		)
	`, otherEvent, otherTenant)
	if err != nil {
		t.Fatal(err)
	}

	global, err := InspectBacklog(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if global.Pending != 1 || global.Quarantined != 1 {
		t.Fatalf("global backlog = %+v", global)
	}

	scoped, err := InspectBacklogForTenant(ctx, db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if scoped.Pending != 1 || scoped.Quarantined != 0 || len(scoped.Quarantines) != 0 {
		t.Fatalf("tenant backlog leaked other tenant: %+v", scoped)
	}
	if scoped.OldestUnpublished.IsZero() {
		t.Fatal("expected oldest unpublished for tenant pending row")
	}

	other, err := InspectBacklogForTenant(ctx, db, otherTenant)
	if err != nil {
		t.Fatal(err)
	}
	if other.Pending != 0 || other.Quarantined != 1 || len(other.Quarantines) != 1 {
		t.Fatalf("other tenant backlog = %+v", other)
	}
	if other.Quarantines[0].EventID != otherEvent || other.Quarantines[0].LastErrorCode != "other_tenant_poison" {
		t.Fatalf("other sample = %+v", other.Quarantines[0])
	}
}
