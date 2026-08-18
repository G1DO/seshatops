package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/G1DO/seshatops/api"
	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/forecast"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/platform"
	"github.com/G1DO/seshatops/relay"
)

func forecastFeaturesPath(tenantID string) string {
	return "/v1/tenants/" + tenantID + "/forecast/features"
}

func TestForecastFeaturesReturnsExplicitInsufficientSnapshot(t *testing.T) {
	db := openTestDB(t)
	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := getWithSession(t, ts.URL+forecastFeaturesPath(identity.TenantNS001UUID), sess, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var snapshot forecast.FeatureSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.TenantID != identity.TenantNS001UUID || snapshot.Status != forecast.SnapshotStatusInsufficient || len(snapshot.Rows) != 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "label") || strings.Contains(string(raw), "event_bytes") {
		t.Fatalf("unsafe feature response: %s", raw)
	}
}

func TestForecastFeaturesCompactReplayIsCompleteAndReadOnly(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	seedCompactForecastHistory(t, db, fx)
	before := forecastTableCounts(t, db)

	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := getWithSession(t, ts.URL+forecastFeaturesPath(fx.TenantID), sess, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var snapshot forecast.FeatureSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != forecast.SnapshotStatusComplete || len(snapshot.Rows) != 1 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	row := snapshot.Rows[0]
	if row.TenantID != fx.TenantID || row.ItemID != fx.ItemID || row.AsOfDate != "2026-01-05" || row.SourceCutoffDate != row.AsOfDate || row.QuantityOnHand != 9 || row.Split != forecast.SplitTrain {
		t.Fatalf("row=%+v", row)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "label") || strings.Contains(string(raw), "event_bytes") || strings.Contains(string(raw), fx.Event.EventID) {
		t.Fatalf("unsafe feature response: %s", raw)
	}
	if after := forecastTableCounts(t, db); fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("read changed table counts: before=%v after=%v", before, after)
	}
}

func TestForecastFeaturesAuthorizationAndControls(t *testing.T) {
	db := openTestDB(t)
	tenant := identity.TenantNS001UUID

	ts, readerSession := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := getWithSession(t, ts.URL+forecastFeaturesPath(identity.TenantNS002UUID), readerSession, nil)
	assertForbiddenNoProjection(t, resp)

	resp = getWithSession(t, ts.URL+forecastFeaturesPath(tenant)+"?horizon=7", readerSession, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unsupported query status=%d", resp.StatusCode)
	}

	operatorTS, operatorSession := gatedSession(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))
	resp = getWithSession(t, operatorTS.URL+forecastFeaturesPath(tenant), operatorSession, nil)
	assertForbiddenNoProjection(t, resp)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req, err := http.NewRequest(method, ts.URL+forecastFeaturesPath(tenant), nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: identity.DefaultCookieName, Value: readerSession.ID})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != http.MethodGet {
			t.Fatalf("%s status=%d allow=%q body=%s", method, resp.StatusCode, resp.Header.Get("Allow"), body)
		}
	}
}

func TestForecastFeaturesMissingSessionAndNilPolicyFailClosed(t *testing.T) {
	db := openTestDB(t)
	server := api.NewServer(db, nil, nil, northstarReaderPolicy())
	request := httptest.NewRequest(http.MethodGet, forecastFeaturesPath(identity.TenantNS001UUID), nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	ts, session := gatedSession(t, db, "operator-northstar", nil)
	resp := getWithSession(t, ts.URL+forecastFeaturesPath(identity.TenantNS001UUID), session, nil)
	assertForbiddenNoProjection(t, resp)
}

func seedCompactForecastHistory(t *testing.T, db *sql.DB, fx northstar.Fixture) {
	t.Helper()
	var previousEventID string
	for i := 0; i < 8; i++ {
		env := fx.Event
		env.EventID = compactForecastUUID(i + 1)
		env.AggregateVersion = int64(i + 1)
		env.OccurredAt = fmt.Sprintf("2026-01-%02dT09:00:00Z", 5+i)
		env.RecordedAt = env.OccurredAt
		env.CorrelationID = compactForecastUUID(100 + i)
		env.TraceID = compactForecastUUID(200 + i)
		if previousEventID == "" {
			env.CausationID = nil
		} else {
			causation := previousEventID
			env.CausationID = &causation
		}
		env.Payload = event.QuantityDecremented{
			OrderID:             compactForecastUUID(300 + i),
			ItemID:              fx.ItemID,
			QuantityDecremented: 1,
			QuantityBefore:      int64(10 - i),
			QuantityAfter:       int64(9 - i),
		}
		if err := event.Validate(env); err != nil {
			t.Fatal(err)
		}
		raw, err := event.CanonicalBytes(env)
		if err != nil {
			t.Fatal(err)
		}
		hash, err := event.ContentHash(env)
		if err != nil {
			t.Fatal(err)
		}
		recordedAt, err := time.Parse(time.RFC3339Nano, env.RecordedAt)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`
			INSERT INTO erp.outbox (
				event_id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
				content_hash, event_bytes, status, recorded_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8)
		`, env.EventID, env.TenantID, env.AggregateType, env.AggregateID, env.AggregateVersion, hash, raw, recordedAt); err != nil {
			t.Fatal(err)
		}
		result, err := platform.ProcessRecord(context.Background(), db, []byte(relay.AggregateKey(env.TenantID, env.AggregateType, env.AggregateID)), raw, platform.SourcePosition{Topic: relay.Topic, Partition: 0, Offset: int64(i + 1)})
		if err != nil || result.Disposition != platform.DispositionApplied {
			t.Fatalf("process version %d result=%+v err=%v", i+1, result, err)
		}
		previousEventID = env.EventID
	}
	future := fx.Event
	future.EventID = compactForecastUUID(9)
	future.AggregateVersion = 9
	future.OccurredAt = "2099-01-01T09:00:00Z"
	future.RecordedAt = future.OccurredAt
	future.CorrelationID = compactForecastUUID(400)
	future.TraceID = compactForecastUUID(500)
	future.CausationID = &previousEventID
	future.Payload = event.QuantityDecremented{
		OrderID:             compactForecastUUID(600),
		ItemID:              fx.ItemID,
		QuantityDecremented: 1,
		QuantityBefore:      2,
		QuantityAfter:       1,
	}
	raw, err := event.CanonicalBytes(future)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := event.ContentHash(future)
	if err != nil {
		t.Fatal(err)
	}
	recordedAt, err := time.Parse(time.RFC3339Nano, future.RecordedAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO erp.outbox (
			event_id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
			content_hash, event_bytes, status, recorded_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8)
	`, future.EventID, future.TenantID, future.AggregateType, future.AggregateID, future.AggregateVersion, hash, raw, recordedAt); err != nil {
		t.Fatal(err)
	}
}

func compactForecastUUID(n int) string {
	return fmt.Sprintf("018f5d78-6e64-4f5f-bd16-8e9f7c4a2%03d", n)
}

func forecastTableCounts(t *testing.T, db *sql.DB) []int64 {
	t.Helper()
	queries := []string{
		`SELECT COUNT(*) FROM erp.outbox`,
		`SELECT COUNT(*) FROM platform.inbox`,
		`SELECT COUNT(*) FROM platform.inventory_projection`,
		`SELECT COUNT(*) FROM platform.processing_failures`,
		`SELECT COUNT(*) FROM identity.authorization_decisions`,
	}
	counts := make([]int64, 0, len(queries))
	for _, query := range queries {
		var count int64
		if err := db.QueryRow(query).Scan(&count); err != nil {
			t.Fatal(err)
		}
		counts = append(counts, count)
	}
	return counts
}
