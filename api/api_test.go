package api_test

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/G1DO/seshatops/api"
	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/platform"
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
		if err := platform.Migrate(ctx, db); err != nil {
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
	if err := platform.Migrate(ctx, db); err != nil {
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

func processEnvelope(t *testing.T, db *sql.DB, env event.Envelope, offset int64) platform.Result {
	t.Helper()
	raw := mustCanonical(t, env)
	key := []byte(relay.AggregateKey(env.TenantID, env.AggregateType, env.AggregateID))
	res, err := platform.ProcessRecord(context.Background(), db, key, raw, platform.SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: offset,
	})
	if err != nil {
		t.Fatalf("ProcessRecord: %v", err)
	}
	return res
}

func newTestServer(t *testing.T, db *sql.DB) *api.Server {
	t.Helper()
	hub := api.NewHub()
	platform.SetAppliedNotifier(hub)
	t.Cleanup(func() { platform.SetAppliedNotifier(nil) })
	return api.NewServer(db, hub)
}

func inventoryPath(tenantID string) string {
	return "/v1/tenants/" + tenantID + "/inventory"
}

func streamPath(tenantID string) string {
	return "/v1/tenants/" + tenantID + "/inventory/stream"
}

func TestRESTReturnsCommittedProjection(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	srv := newTestServer(t, db)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + inventoryPath(fx.TenantID))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var empty api.InventorySnapshot
	if err := json.NewDecoder(resp.Body).Decode(&empty); err != nil {
		t.Fatal(err)
	}
	if len(empty.Items) != 0 || empty.Checksum == "" || empty.ObservedAt == "" {
		t.Fatalf("empty snapshot = %+v", empty)
	}
	wantEmpty, err := platform.ChecksumTenant(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Checksum != wantEmpty {
		t.Fatalf("checksum = %s want %s", empty.Checksum, wantEmpty)
	}

	res := processEnvelope(t, db, fx.Event, 1)
	if res.Disposition != platform.DispositionApplied {
		t.Fatalf("disposition = %s", res.Disposition)
	}

	resp, err = http.Get(ts.URL + inventoryPath(fx.TenantID))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var snap api.InventorySnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	if snap.TenantID != fx.TenantID || len(snap.Items) != 1 {
		t.Fatalf("snapshot = %+v", snap)
	}
	item := snap.Items[0]
	if item.ItemID != fx.ItemID || item.QuantityOnHand != 8 || item.AggregateVersion != 1 {
		t.Fatalf("item = %+v", item)
	}
	want, err := platform.ChecksumTenant(context.Background(), db, fx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Checksum != want {
		t.Fatalf("checksum = %s want %s", snap.Checksum, want)
	}
}

func TestSSEEmitsOnlyAfterCommit(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	srv := newTestServer(t, db)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+streamPath(fx.TenantID), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}

	events := make(chan api.ProjectionUpdated, 4)
	errCh := make(chan error, 1)
	go func() {
		errCh <- readSSEUpdates(resp.Body, events)
	}()

	// Give the subscriber time to register.
	time.Sleep(50 * time.Millisecond)

	platform.SetFailBeforeCommitForTest(func(context.Context) error {
		return errors.New("injected rollback")
	})
	raw := mustCanonical(t, fx.Event)
	key := []byte(relay.AggregateKey(fx.TenantID, fx.Event.AggregateType, fx.Event.AggregateID))
	_, err = platform.ProcessRecord(context.Background(), db, key, raw, platform.SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 1,
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}
	platform.SetFailBeforeCommitForTest(nil)

	select {
	case u := <-events:
		t.Fatalf("SSE fired on rollback: %+v", u)
	case <-time.After(200 * time.Millisecond):
	}

	res := processEnvelope(t, db, fx.Event, 2)
	if res.Disposition != platform.DispositionApplied {
		t.Fatalf("disposition = %s", res.Disposition)
	}

	select {
	case u := <-events:
		if u.ItemID != fx.ItemID || u.QuantityOnHand != 8 || u.AggregateVersion != 1 ||
			u.LastAppliedEventID != fx.Event.EventID || u.Checksum == "" {
			t.Fatalf("update = %+v", u)
		}
		want, err := platform.ChecksumTenant(context.Background(), db, fx.TenantID)
		if err != nil {
			t.Fatal(err)
		}
		if u.Checksum != want {
			t.Fatalf("checksum = %s want %s", u.Checksum, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE update after commit")
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE reader did not exit after cancel")
	}
}

func TestSSEDisconnectReconnectRESTConverge(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	srv := newTestServer(t, db)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	ctx1, cancel1 := context.WithCancel(context.Background())
	req1, err := http.NewRequestWithContext(ctx1, http.MethodGet, ts.URL+streamPath(fx.TenantID), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	events1 := make(chan api.ProjectionUpdated, 4)
	go func() { _ = readSSEUpdates(resp1.Body, events1) }()
	time.Sleep(50 * time.Millisecond)

	_ = processEnvelope(t, db, fx.Event, 1)
	select {
	case <-events1:
	case <-time.After(2 * time.Second):
		t.Fatal("missing first SSE update")
	}

	cancel1()
	_ = resp1.Body.Close()

	v2 := fx.Event
	v2.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b2"
	v2.AggregateVersion = 2
	v2.Payload.QuantityBefore = 8
	v2.Payload.QuantityDecremented = 1
	v2.Payload.QuantityAfter = 7
	v2.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b4"
	v2.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20b3"
	_ = processEnvelope(t, db, v2, 2)

	// Authoritative catch-up via REST while disconnected.
	resp, err := http.Get(ts.URL + inventoryPath(fx.TenantID))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var snap api.InventorySnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Items) != 1 || snap.Items[0].QuantityOnHand != 7 || snap.Items[0].AggregateVersion != 2 {
		t.Fatalf("REST catch-up = %+v", snap)
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	req2, err := http.NewRequestWithContext(ctx2, http.MethodGet, ts.URL+streamPath(fx.TenantID), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	events2 := make(chan api.ProjectionUpdated, 4)
	go func() { _ = readSSEUpdates(resp2.Body, events2) }()
	time.Sleep(50 * time.Millisecond)

	v3 := v2
	v3.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20c2"
	v3.AggregateVersion = 3
	v3.Payload.QuantityBefore = 7
	v3.Payload.QuantityDecremented = 1
	v3.Payload.QuantityAfter = 6
	v3.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20c4"
	v3.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a20c3"
	_ = processEnvelope(t, db, v3, 3)

	select {
	case u := <-events2:
		if u.QuantityOnHand != 6 || u.AggregateVersion != 3 || u.LastAppliedEventID != v3.EventID {
			t.Fatalf("post-reconnect SSE = %+v", u)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("missing SSE after reconnect")
	}
}

func TestReadOnlyRejectsMutatingMethods(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	srv := newTestServer(t, db)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	_ = processEnvelope(t, db, fx.Event, 1)
	qtyBefore, verBefore, ok, err := platform.ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok {
		t.Fatalf("projection: ok=%v err=%v", ok, err)
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		for _, path := range []string{inventoryPath(fx.TenantID), streamPath(fx.TenantID)} {
			req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader([]byte(`{}`)))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s status=%d body=%s", method, path, resp.StatusCode, body)
			}
		}
	}

	qty, ver, ok, err := platform.ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != qtyBefore || ver != verBefore {
		t.Fatalf("projection mutated: qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
}

func TestMalformedTenantRejected(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	srv := newTestServer(t, db)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	_ = processEnvelope(t, db, fx.Event, 1)

	cases := []string{
		"/v1/tenants/not-a-uuid/inventory",
		"/v1/tenants/11111111-1111-4111-8111-11111111111F/inventory", // uppercase
		"/v1/tenants//inventory",
	}
	for _, path := range cases {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
			t.Fatalf("path %s status=%d body=%s", path, resp.StatusCode, body)
		}
		if path != "/v1/tenants//inventory" && resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("path %s want 400, got %d", path, resp.StatusCode)
		}
	}

	qty, ver, ok, err := platform.ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("projection mutated: qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
}

func readSSEUpdates(r io.Reader, out chan<- api.ProjectionUpdated) error {
	sc := bufio.NewScanner(r)
	var eventName string
	var data strings.Builder
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, ":") {
			continue
		}
		if line == "" {
			if eventName == api.EventProjectionUpdated && data.Len() > 0 {
				var u api.ProjectionUpdated
				if err := json.Unmarshal([]byte(data.String()), &u); err != nil {
					return err
				}
				out <- u
			}
			eventName = ""
			data.Reset()
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	return sc.Err()
}
