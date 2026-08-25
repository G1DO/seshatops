package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/G1DO/seshatops/bootstrap"
	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/forecast"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/platform"
	"github.com/G1DO/seshatops/relay"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestDemoFixtureCommandConfigAcceptsOnlyDeclaredActions(t *testing.T) {
	for _, action := range []demoFixtureAction{
		demoFixtureSource,
		demoFixtureInspect,
		demoFixtureDuplicate,
		demoFixturePoison,
		demoFixtureForecastIncomplete,
	} {
		t.Run(string(action), func(t *testing.T) {
			env := validDemoFixtureEnv()
			if !action.needsBroker() {
				delete(env, envBrokerSeeds)
			}
			cfg, err := demoFixtureCommandConfigFromEnv([]string{string(action)}, mapLookup(env))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Action != action || cfg.DatabaseURL != env[envDatabaseURL] || cfg.Timeout != defaultStartup {
				t.Fatalf("config=%+v", cfg)
			}
			if action.needsBroker() {
				if len(cfg.BrokerSeeds) != 1 || cfg.BrokerSeeds[0] != demoFixtureBroker {
					t.Fatalf("broker seeds=%v", cfg.BrokerSeeds)
				}
			} else if len(cfg.BrokerSeeds) != 0 {
				t.Fatalf("irrelevant broker seeds=%v", cfg.BrokerSeeds)
			}
		})
	}

	for _, args := range [][]string{nil, []string{}, {"unknown"}, {"source", "poison"}} {
		if _, err := demoFixtureCommandConfigFromEnv(args, mapLookup(validDemoFixtureEnv())); err == nil {
			t.Fatalf("accepted args=%v", args)
		}
	}
}

func TestDemoFixtureCommandConfigFailsClosedOutsideExactLocalStack(t *testing.T) {
	tests := []struct {
		name   string
		change func(map[string]string)
		want   string
	}{
		{name: "missing local opt in", change: func(env map[string]string) { delete(env, envLocalStack) }, want: envLocalStack},
		{name: "non exact local opt in", change: func(env map[string]string) { env[envLocalStack] = "TRUE" }, want: envLocalStack},
		{name: "missing confirmation", change: func(env map[string]string) { delete(env, envDemoFixtureConfirm) }, want: envDemoFixtureConfirm},
		{name: "wrong confirmation", change: func(env map[string]string) { env[envDemoFixtureConfirm] = "yes" }, want: envDemoFixtureConfirm},
		{name: "localhost database", change: func(env map[string]string) {
			env[envDatabaseURL] = "postgres://seshatops@localhost/seshatops_northstar_disposable"
		}, want: "database host"},
		{name: "non disposable database", change: func(env map[string]string) {
			env[envDatabaseURL] = "postgres://seshatops@postgres/seshatops"
		}, want: northstarDisposableDatabase},
		{name: "external database", change: func(env map[string]string) {
			env[envDatabaseURL] = "postgres://seshatops@example.com/seshatops_northstar_disposable"
		}, want: "database host"},
		{name: "wrong database port", change: func(env map[string]string) {
			env[envDatabaseURL] = "postgres://seshatops@postgres:5433/seshatops_northstar_disposable?sslmode=disable"
		}, want: "database port"},
		{name: "wrong database user", change: func(env map[string]string) {
			env[envDatabaseURL] = "postgres://admin@postgres:5432/seshatops_northstar_disposable?sslmode=disable"
		}, want: "database user"},
		{name: "query target override", change: func(env map[string]string) {
			env[envDatabaseURL] = "postgres://seshatops@postgres:5432/seshatops_northstar_disposable?sslmode=disable&host=example.com"
		}, want: "database query"},
		{name: "query transport mismatch", change: func(env map[string]string) {
			env[envDatabaseURL] = "postgres://seshatops@postgres:5432/seshatops_northstar_disposable?sslmode=require"
		}, want: "database query"},
		{name: "broker alias", change: func(env map[string]string) { env[envBrokerSeeds] = "localhost:9092" }, want: envBrokerSeeds},
		{name: "multiple brokers", change: func(env map[string]string) { env[envBrokerSeeds] = "redpanda:9092,redpanda:19092" }, want: envBrokerSeeds},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validDemoFixtureEnv()
			tt.change(env)
			_, err := demoFixtureCommandConfigFromEnv([]string{string(demoFixturePoison)}, mapLookup(env))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v, want %q", err, tt.want)
			}
		})
	}
}

func TestDemoPoisonRecordIsDeterministicAttributedUnsupportedContract(t *testing.T) {
	first, err := buildDemoPoisonRecord()
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildDemoPoisonRecord()
	if err != nil {
		t.Fatal(err)
	}
	if first.EventID != demoPoisonEventID || first.TenantID == "" || first.Topic != relay.Topic {
		t.Fatalf("record=%+v", first)
	}
	if !bytes.Equal(first.Key, second.Key) || !bytes.Equal(first.Value, second.Value) {
		t.Fatal("poison record changed across builds")
	}
	if !bytes.Contains(first.Value, []byte(`"event_schema_version":2`)) {
		t.Fatalf("poison bytes=%s", first.Value)
	}
	if _, err := event.Parse(first.Value); !errors.Is(err, event.ErrUnsupported) {
		t.Fatalf("parse error=%v", err)
	}
	if string(first.Key) != first.TenantID+"/"+event.AggregateTypeSupplier+"/"+demoPoisonSupplierID {
		t.Fatalf("key=%q", first.Key)
	}
}

func TestDemoForecastIncompleteRecordIsDeterministicMalformedInventoryHistory(t *testing.T) {
	first, err := buildDemoForecastIncompleteRecord()
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildDemoForecastIncompleteRecord()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("record identity changed: first=%+v second=%+v", first, second)
	}
	if first.AggregateType != event.AggregateTypeInventoryItem || first.Status != relay.StatusQuarantined || first.TenantID == "" {
		t.Fatalf("record=%+v", first)
	}
	if _, err := event.Parse(first.EventBytes); !errors.Is(err, event.ErrMalformed) {
		t.Fatalf("parse error=%v", err)
	}
}

func TestPublishDemoRecordPreservesExactTopicKeyAndBytes(t *testing.T) {
	record, err := buildDemoPoisonRecord()
	if err != nil {
		t.Fatal(err)
	}
	pub := &recordingDemoPublisher{}
	if err := publishDemoRecord(context.Background(), pub, record); err != nil {
		t.Fatal(err)
	}
	if pub.calls != 1 || pub.topic != record.Topic || !bytes.Equal(pub.key, record.Key) || !bytes.Equal(pub.value, record.Value) {
		t.Fatalf("publisher=%+v", pub)
	}

	pub.err = errors.New("broker unavailable")
	if err := publishDemoRecord(context.Background(), pub, record); err == nil || !strings.Contains(err.Error(), record.EventID) {
		t.Fatalf("publish error=%v", err)
	}
}

func TestDemoFixtureSummaryIsBoundedAndContainsNoEventBytes(t *testing.T) {
	record, err := buildDemoPoisonRecord()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(demoSummaryForBrokerRecord(demoFixturePoison, record))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), demoPoisonEventID) || !strings.Contains(string(raw), `"action":"poison"`) {
		t.Fatalf("summary=%s", raw)
	}
	if bytes.Contains(raw, record.Value) || strings.Contains(string(raw), "event_bytes") {
		t.Fatalf("summary leaked event bytes: %s", raw)
	}
}

func TestDemoFixtureDatabaseActionsRetainExactScopedEvidence(t *testing.T) {
	db := openDemoFixtureTestDB(t)
	ctx := context.Background()
	source, err := bootstrap.AcceptSourceFixture(ctx, db, northstar.LineageSeed)
	if err != nil {
		t.Fatal(err)
	}
	if source.SourceCount != 5 || source.OutboxState[relay.StatusPending] != 5 {
		t.Fatalf("source=%+v", source)
	}
	if _, complete, err := bootstrap.InspectFixtureCheckpoint(ctx, db, northstar.LineageSeed); err != nil || complete {
		t.Fatalf("source-only checkpoint complete=%v error=%v", complete, err)
	}

	if _, err := loadDemoDuplicateRecord(ctx, db); err == nil || !strings.Contains(err.Error(), "must be published") {
		t.Fatalf("pending duplicate precondition error=%v", err)
	}
	finalEventID := source.EventIDs[len(source.EventIDs)-1]
	if _, err := db.ExecContext(ctx, `UPDATE erp.outbox SET status = 'published' WHERE event_id = $1`, finalEventID); err != nil {
		t.Fatal(err)
	}
	duplicate, err := loadDemoDuplicateRecord(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	fx, err := northstar.GenerateLineage(northstar.LineageSeed)
	if err != nil {
		t.Fatal(err)
	}
	want := fx.Events[len(fx.Events)-1]
	wantBytes, err := event.CanonicalBytes(want)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.EventID != want.EventID || string(duplicate.Key) != relay.AggregateKey(want.TenantID, want.AggregateType, want.AggregateID) || !bytes.Equal(duplicate.Value, wantBytes) {
		t.Fatalf("duplicate identity=%+v", duplicate)
	}

	incomplete, err := buildDemoForecastIncompleteRecord()
	if err != nil {
		t.Fatal(err)
	}
	if err := insertDemoForecastIncompleteRecord(ctx, db, incomplete); err != nil {
		t.Fatal(err)
	}
	if err := insertDemoForecastIncompleteRecord(ctx, db, incomplete); err != nil {
		t.Fatalf("idempotent insert: %v", err)
	}
	cutoff := incomplete.RecordedAt.Add(time.Hour)
	forecastSource, err := platform.LoadTenantForecastSourceAtCutoff(ctx, db, incomplete.TenantID, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if forecastSource.Status != forecast.SnapshotStatusIncomplete || !containsReason(forecastSource.StatusReasons, incomplete.EventID) {
		t.Fatalf("forecast source=%+v", forecastSource)
	}
	var retained, projected int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM erp.outbox WHERE event_id = $1 AND tenant_id = $2`, incomplete.EventID, incomplete.TenantID).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM platform.inventory_projection WHERE tenant_id = $1 AND item_id = $2`, incomplete.TenantID, incomplete.AggregateID).Scan(&projected); err != nil {
		t.Fatal(err)
	}
	if retained != 1 || projected != 0 {
		t.Fatalf("retained=%d projected=%d", retained, projected)
	}
}

func validDemoFixtureEnv() map[string]string {
	return map[string]string{
		envLocalStack:         "true",
		envDemoFixtureConfirm: demoFixtureConfirmation,
		envDatabaseURL:        "postgres://seshatops@postgres:5432/seshatops_northstar_disposable?sslmode=disable",
		envBrokerSeeds:        demoFixtureBroker,
	}
}

func containsReason(reasons []string, value string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, value) {
			return true
		}
	}
	return false
}

func openDemoFixtureTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	if dsn := os.Getenv("SESHATOPS_TEST_DATABASE_URL"); dsn != "" {
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if err := db.PingContext(ctx); err != nil {
			t.Fatal(err)
		}
		resetDemoFixtureTestSchemas(t, db)
		migrateDemoFixtureTestDB(t, db)
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
		t.Skipf("PostgreSQL integration test requires Docker or SESHATOPS_TEST_DATABASE_URL: %v", err)
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
	migrateDemoFixtureTestDB(t, db)
	return db
}

func migrateDemoFixtureTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	if err := erp.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := platform.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
}

func resetDemoFixtureTestSchemas(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, schema := range []string{"platform", "erp"} {
		if _, err := db.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE"); err != nil {
			t.Fatal(err)
		}
	}
}

type recordingDemoPublisher struct {
	calls int
	topic string
	key   []byte
	value []byte
	err   error
}

func (p *recordingDemoPublisher) Publish(_ context.Context, topic string, key, value []byte) error {
	p.calls++
	p.topic = topic
	p.key = append([]byte(nil), key...)
	p.value = append([]byte(nil), value...)
	return p.err
}
