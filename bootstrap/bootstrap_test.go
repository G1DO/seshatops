package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/platform"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRunIsDeterministicAndIdempotent(t *testing.T) {
	db := openTestDB(t)
	pub := &testPublisher{}
	consumer := &testConsumer{db: db, tenantID: identity.TenantNS001UUID}
	cfg := testConfig()

	first, err := Run(context.Background(), db, pub, consumer, cfg)
	if err != nil {
		t.Fatal(err)
	}
	inspected, complete, err := InspectFixtureCheckpoint(context.Background(), db, northstar.LineageSeed)
	if err != nil || !complete {
		t.Fatalf("read-only checkpoint complete=%v error=%v", complete, err)
	}
	if inspected.ProjectionChecksum != first.ProjectionChecksum || inspected.LineageChecksum != first.LineageChecksum {
		t.Fatalf("read-only checkpoint=%+v first=%+v", inspected, first)
	}
	second, err := Run(context.Background(), db, pub, consumer, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.ProjectionChecksum == "" || first.LineageChecksum == "" {
		t.Fatalf("missing checksums: %+v", first)
	}
	if first.ProjectionChecksum != second.ProjectionChecksum || first.LineageChecksum != second.LineageChecksum {
		t.Fatalf("checksums changed across idempotent run: first=%+v second=%+v", first, second)
	}
	if first.EventCounts.Source != 5 || first.EventCounts.Published != 5 || first.EventCounts.Projected != 5 {
		t.Fatalf("unexpected first counts: %+v", first.EventCounts)
	}
	if pub.calls != 5 || consumer.calls != 1 {
		t.Fatalf("duplicate run touched transport: publisher=%d consumer=%d", pub.calls, consumer.calls)
	}
	if firstJSON, secondJSON := mustJSON(t, first), mustJSON(t, second); firstJSON != secondJSON {
		t.Fatalf("summary is not deterministic:\nfirst=%s\nsecond=%s", firstJSON, secondJSON)
	}

	var foreignSource, foreignProjection int
	if err := db.QueryRow(`SELECT COUNT(*) FROM erp.outbox WHERE tenant_id = $1`, identity.TenantNS002UUID).Scan(&foreignSource); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.inventory_projection WHERE tenant_id = $1`, identity.TenantNS002UUID).Scan(&foreignProjection); err != nil {
		t.Fatal(err)
	}
	if foreignSource != 0 || foreignProjection != 0 {
		t.Fatalf("bootstrap crossed tenant boundary: source=%d projection=%d", foreignSource, foreignProjection)
	}
	if len(second.Authorization.Assignments) != 2 || second.Authorization.NegativeTenantID != identity.TenantNS002UUID {
		t.Fatalf("unexpected authorization configuration: %+v", second.Authorization)
	}
}

func TestRunRecoversFromPartialSourceBootstrap(t *testing.T) {
	db := openTestDB(t)
	fx, err := northstar.GenerateLineage(northstar.LineageSeed)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := erp.SeedLineageInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	supplier, err := erp.SupplierCommandFromLineage(fx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := erp.RegisterSupplier(ctx, db, supplier); err != nil {
		t.Fatal(err)
	}

	pub := &testPublisher{}
	consumer := &testConsumer{db: db, tenantID: fx.TenantID}
	summary, err := Run(ctx, db, pub, consumer, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if summary.EventCounts.Source != len(fx.Events) || summary.EventCounts.Projected != len(fx.Events) {
		t.Fatalf("partial recovery counts: %+v", summary.EventCounts)
	}
	var sourceCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM erp.outbox WHERE tenant_id = $1`, fx.TenantID).Scan(&sourceCount); err != nil {
		t.Fatal(err)
	}
	if sourceCount != len(fx.Events) {
		t.Fatalf("source event count = %d, want %d", sourceCount, len(fx.Events))
	}
}

func TestAcceptSourceFixtureStopsAtOutboxAndIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	first, err := AcceptSourceFixture(ctx, db, northstar.LineageSeed)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AcceptSourceFixture(ctx, db, northstar.LineageSeed)
	if err != nil {
		t.Fatal(err)
	}
	if first.SourceCount != 5 || second.SourceCount != 5 || first.OutboxState["pending"] != 5 || second.OutboxState["pending"] != 5 {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	if len(first.EventIDs) != 5 || len(second.EventIDs) != 5 {
		t.Fatalf("first=%+v second=%+v", first.EventIDs, second.EventIDs)
	}
	var inbox, inventoryProjection, lineageProjection int
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.inbox WHERE tenant_id = $1`, first.TenantID).Scan(&inbox); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.inventory_projection WHERE tenant_id = $1`, first.TenantID).Scan(&inventoryProjection); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform.lineage_suppliers WHERE tenant_id = $1`, first.TenantID).Scan(&lineageProjection); err != nil {
		t.Fatal(err)
	}
	if inbox != 0 || inventoryProjection != 0 || lineageProjection != 0 {
		t.Fatalf("source-only fixture mutated projection: inbox=%d inventory=%d lineage=%d", inbox, inventoryProjection, lineageProjection)
	}
}

func TestRunTimesOutWithoutRelayOrConsumerCheckpoint(t *testing.T) {
	db := openTestDB(t)
	pub := &testPublisher{err: errors.New("broker unavailable")}
	consumer := &testConsumer{db: db, tenantID: identity.TenantNS001UUID, noOp: true}
	cfg := testConfig()
	cfg.Timeout = 150 * time.Millisecond
	cfg.PollTimeout = 10 * time.Millisecond
	cfg.RetryInterval = 10 * time.Millisecond
	_, err := Run(context.Background(), db, pub, consumer, cfg)
	if !errors.Is(err, ErrCheckpointTimeout) {
		t.Fatalf("error = %v, want checkpoint timeout", err)
	}
	if pub.calls == 0 || consumer.calls == 0 {
		t.Fatalf("timeout did not exercise bounded relay/consumer loops: publisher=%d consumer=%d", pub.calls, consumer.calls)
	}
}

type testPublisher struct {
	calls int
	err   error
}

func (p *testPublisher) Publish(context.Context, string, []byte, []byte) error {
	p.calls++
	return p.err
}

type testConsumer struct {
	db       *sql.DB
	tenantID string
	noOp     bool
	calls    int
}

func (c *testConsumer) ConsumeOnce(ctx context.Context) (platform.ConsumeResult, error) {
	c.calls++
	if c.noOp {
		return platform.ConsumeResult{}, nil
	}
	records, err := platform.LoadTenantOutboxHistory(ctx, c.db, c.tenantID, "")
	if err != nil {
		return platform.ConsumeResult{}, err
	}
	for _, record := range records {
		result, err := platform.ProcessRecord(ctx, c.db, record.Key, record.Value, record.Pos)
		if err != nil && !result.ShouldAck {
			return platform.ConsumeResult{}, err
		}
	}
	return platform.ConsumeResult{}, nil
}

func testConfig() Config {
	return Config{
		Seed:          northstar.LineageSeed,
		Timeout:       15 * time.Second,
		PollTimeout:   100 * time.Millisecond,
		RetryInterval: 5 * time.Millisecond,
		RelayOwner:    "bootstrap-test-owner",
		LeaseTTL:      time.Minute,
		DrainLimit:    100,
	}
}

func mustJSON(t *testing.T, summary Summary) string {
	t.Helper()
	bytes, err := MarshalSummary(summary)
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes)
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	if dsn := os.Getenv("SESHATOPS_TEST_DATABASE_URL"); dsn != "" {
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatalf("open test database: %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if err := db.PingContext(ctx); err != nil {
			t.Fatalf("ping test database: %v", err)
		}
		resetSchemas(t, db)
		migrateTestDB(t, db)
		return db
	}
	container, err := postgres.Run(ctx,
		erp.PostgresImage,
		postgres.WithDatabase("seshatops"),
		postgres.WithUsername("seshatops"),
		postgres.WithPassword("seshatops"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(2*time.Minute)),
	)
	if err != nil {
		t.Skipf("PostgreSQL integration tests require Docker or SESHATOPS_TEST_DATABASE_URL: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("terminate postgres: %v", err)
		}
	})
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	migrateTestDB(t, db)
	return db
}

func migrateTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	if err := erp.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := platform.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := identity.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
}

func resetSchemas(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, schema := range []string{"identity", "platform", "erp"} {
		if _, err := db.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE"); err != nil {
			t.Fatal(err)
		}
	}
}
