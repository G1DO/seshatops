package erp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

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
		resetSchema(t, db)
		if err := Migrate(ctx, db); err != nil {
			t.Fatal(err)
		}
		return db
	}

	pgContainer, err := postgres.Run(ctx,
		PostgresImage,
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
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db
}

func resetSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP SCHEMA IF EXISTS erp CASCADE`); err != nil {
		t.Fatalf("drop erp schema: %v", err)
	}
}

func seedFixture(t *testing.T, db *sql.DB) northstar.Fixture {
	t.Helper()
	fx, err := northstar.Generate(northstar.DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	if err := SeedNorthstarInventory(context.Background(), db, fx); err != nil {
		t.Fatal(err)
	}
	return fx
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func inventoryState(t *testing.T, db *sql.DB, tenantID, itemID string) (qty, version int64) {
	t.Helper()
	err := db.QueryRow(`
		SELECT quantity_on_hand, aggregate_version
		FROM erp.inventory_items
		WHERE tenant_id = $1 AND item_id = $2
	`, tenantID, itemID).Scan(&qty, &version)
	if err != nil {
		t.Fatal(err)
	}
	return qty, version
}

func TestAcceptOrderCommitsSourceAndOutboxAtomically(t *testing.T) {
	db := openTestDB(t)
	fx := seedFixture(t, db)
	ctx := context.Background()

	res, err := AcceptOrder(ctx, db, OrderCommandFromFixture(fx))
	if err != nil {
		t.Fatal(err)
	}

	wantHash, err := event.ContentHash(fx.Event)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, err := event.CanonicalBytes(fx.Event)
	if err != nil {
		t.Fatal(err)
	}
	if res.ContentHash != wantHash {
		t.Fatalf("content hash = %s, want %s", res.ContentHash, wantHash)
	}
	if string(res.EventBytes) != string(wantBytes) {
		t.Fatalf("event bytes mismatch\ngot:  %s\nwant: %s", res.EventBytes, wantBytes)
	}
	if res.OutboxStatus != "pending" {
		t.Fatalf("status = %s, want pending", res.OutboxStatus)
	}
	if res.QuantityBefore != 10 || res.QuantityAfter != 8 || res.AggregateVersion != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}

	qty, version := inventoryState(t, db, fx.TenantID, fx.ItemID)
	if qty != 8 || version != 1 {
		t.Fatalf("inventory qty=%d version=%d", qty, version)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.orders`) != 1 {
		t.Fatal("expected one order")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 1 {
		t.Fatal("expected one outbox row")
	}

	var storedHash string
	var storedBytes []byte
	var status string
	err = db.QueryRow(`
		SELECT content_hash, event_bytes, status FROM erp.outbox WHERE event_id = $1
	`, fx.Event.EventID).Scan(&storedHash, &storedBytes, &status)
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
	if err := event.CheckIdentityConflict(parsed, fx.Event); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptOrderRollbackLeavesNoPartialState(t *testing.T) {
	db := openTestDB(t)
	fx := seedFixture(t, db)
	ctx := context.Background()

	setTestFailBeforeCommitForTest(func(context.Context) error {
		return errors.New("forced failure before commit")
	})
	t.Cleanup(func() { setTestFailBeforeCommitForTest(nil) })

	_, err := AcceptOrder(ctx, db, OrderCommandFromFixture(fx))
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
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 0 {
		t.Fatal("outbox should not be committed")
	}
}

func TestAcceptOrderDomainValidation(t *testing.T) {
	db := openTestDB(t)
	fx := seedFixture(t, db)
	ctx := context.Background()
	base := OrderCommandFromFixture(fx)

	tests := []struct {
		name    string
		mutate  func(cmd OrderCommand) OrderCommand
		wantErr error
	}{
		{
			name: "invalid_quantity_zero",
			mutate: func(cmd OrderCommand) OrderCommand {
				cmd.Quantity = 0
				return cmd
			},
			wantErr: ErrInvalidQuantity,
		},
		{
			name: "invalid_quantity_negative",
			mutate: func(cmd OrderCommand) OrderCommand {
				cmd.Quantity = -1
				return cmd
			},
			wantErr: ErrInvalidQuantity,
		},
		{
			name: "unknown_item",
			mutate: func(cmd OrderCommand) OrderCommand {
				cmd.ItemID = "item-unknown-001"
				cmd.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b1"
				return cmd
			},
			wantErr: ErrUnknownItem,
		},
		{
			name: "tenant_mismatch_unknown_row",
			mutate: func(cmd OrderCommand) OrderCommand {
				cmd.TenantID = "22222222-2222-4222-8222-222222222222"
				cmd.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b2"
				cmd.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b3"
				return cmd
			},
			wantErr: ErrUnknownItem,
		},
		{
			name: "malformed_tenant",
			mutate: func(cmd OrderCommand) OrderCommand {
				cmd.TenantID = "NOT-A-TENANT"
				return cmd
			},
			wantErr: ErrTenantMismatch,
		},
		{
			name: "uppercase_tenant",
			mutate: func(cmd OrderCommand) OrderCommand {
				cmd.TenantID = "11111111-1111-4111-8111-11111111111A"
				return cmd
			},
			wantErr: ErrTenantMismatch,
		},
		{
			name: "insufficient_inventory",
			mutate: func(cmd OrderCommand) OrderCommand {
				cmd.Quantity = 11
				cmd.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b4"
				cmd.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b5"
				return cmd
			},
			wantErr: ErrInvalidTransition,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AcceptOrder(ctx, db, tc.mutate(base))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			qty, version := inventoryState(t, db, fx.TenantID, fx.ItemID)
			if qty != 10 || version != 0 {
				t.Fatalf("inventory mutated on rejection: qty=%d version=%d", qty, version)
			}
			if countRows(t, db, `SELECT COUNT(*) FROM erp.orders`) != 0 {
				t.Fatal("order inserted on rejection")
			}
			if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 0 {
				t.Fatal("outbox inserted on rejection")
			}
		})
	}
}

func TestAcceptOrderConcurrentInventoryIntegrity(t *testing.T) {
	db := openTestDB(t)
	fx := seedFixture(t, db)
	ctx := context.Background()

	type outcome struct {
		err error
	}
	ch := make(chan outcome, 2)

	mkCmd := func(eventID, orderID string, qty int64) OrderCommand {
		cmd := OrderCommandFromFixture(fx)
		cmd.EventID = eventID
		cmd.OrderID = orderID
		cmd.Quantity = qty
		return cmd
	}

	go func() {
		_, err := AcceptOrder(ctx, db, mkCmd(
			"018f5d78-6e64-4f5f-bd16-8e9f7c4a21a1",
			"018f5d78-6e64-4f5f-bd16-8e9f7c4a21a2",
			6,
		))
		ch <- outcome{err: err}
	}()
	go func() {
		_, err := AcceptOrder(ctx, db, mkCmd(
			"018f5d78-6e64-4f5f-bd16-8e9f7c4a21b1",
			"018f5d78-6e64-4f5f-bd16-8e9f7c4a21b2",
			6,
		))
		ch <- outcome{err: err}
	}()

	var success, failure int
	for i := 0; i < 2; i++ {
		o := <-ch
		if o.err == nil {
			success++
			continue
		}
		if !errors.Is(o.err, ErrInvalidTransition) {
			t.Fatalf("unexpected error: %v", o.err)
		}
		failure++
	}
	if success != 1 || failure != 1 {
		t.Fatalf("success=%d failure=%d, want 1 and 1", success, failure)
	}

	qty, version := inventoryState(t, db, fx.TenantID, fx.ItemID)
	if qty != 4 || version != 1 {
		t.Fatalf("inventory qty=%d version=%d, want 4 and 1", qty, version)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.orders`) != 1 {
		t.Fatal("expected exactly one accepted order")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox`) != 1 {
		t.Fatal("expected exactly one outbox row")
	}
}

func TestAcceptOrderHasNoBrokerDependency(t *testing.T) {
	db := openTestDB(t)
	fx := seedFixture(t, db)

	if _, err := AcceptOrder(context.Background(), db, OrderCommandFromFixture(fx)); err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM erp.outbox WHERE status = 'pending'`) != 1 {
		t.Fatal("expected pending outbox without broker publication")
	}
}

func TestPostgresImagePinDocumented(t *testing.T) {
	if PostgresVersionLabel != "16.14" {
		t.Fatalf("version label = %s", PostgresVersionLabel)
	}
	if PostgresImage != "postgres@sha256:95206741a5b214807675e14165369d05b93a9cf692223b616d07cca227e74b0b" {
		t.Fatalf("unexpected image pin: %s", PostgresImage)
	}
}

func TestValidateRejectsWithoutDB(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://invalid:invalid@127.0.0.1:1/none")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = AcceptOrder(context.Background(), db, OrderCommand{
		EventID:       "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a1",
		TenantID:      "11111111-1111-4111-8111-111111111111",
		OrderID:       "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a4",
		ItemID:        "item-flour-001",
		Quantity:      0,
		OccurredAt:    "2026-08-07T09:00:00Z",
		RecordedAt:    "2026-08-07T09:00:00Z",
		CorrelationID: "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a2",
		TraceID:       "018f5d78-6e64-4f5f-bd16-8e9f7c4a20a3",
	})
	if !errors.Is(err, ErrInvalidQuantity) {
		t.Fatalf("err = %v, want ErrInvalidQuantity", err)
	}
}

func Example_imagePin() {
	fmt.Println(PostgresVersionLabel)
	// Output: 16.14
}
