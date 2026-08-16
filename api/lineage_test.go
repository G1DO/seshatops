package api_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/G1DO/seshatops/api"
	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/platform"
)

func lineagePath(tenantID, batchID string) string {
	return "/v1/tenants/" + tenantID + "/ops/lineage/batches/" + batchID
}

func mustLineageFixture(t *testing.T) northstar.LineageFixture {
	t.Helper()
	fx, err := northstar.GenerateLineage(northstar.LineageSeed)
	if err != nil {
		t.Fatal(err)
	}
	return fx
}

func applyLineageFixture(t *testing.T, db *sql.DB, fx northstar.LineageFixture) {
	t.Helper()
	for i, env := range fx.Events {
		if res := processEnvelope(t, db, env, int64(i+1)); res.Disposition != platform.DispositionApplied {
			t.Fatalf("apply %s: %+v", env.EventType, res)
		}
	}
}

func applyForeignLineage(t *testing.T, db *sql.DB, fx northstar.LineageFixture) {
	t.Helper()
	ids := []string{
		"628f5d78-6e64-4f5f-bd16-8e9f7c4a7011",
		"628f5d78-6e64-4f5f-bd16-8e9f7c4a7012",
		"628f5d78-6e64-4f5f-bd16-8e9f7c4a7013",
		"628f5d78-6e64-4f5f-bd16-8e9f7c4a7014",
		"628f5d78-6e64-4f5f-bd16-8e9f7c4a7015",
	}
	for i, env := range fx.Events {
		env.TenantID = identity.TenantNS002UUID
		env.EventID = ids[i]
		if i == 0 {
			env.CausationID = nil
		} else {
			prev := ids[i-1]
			env.CausationID = &prev
		}
		if err := event.Validate(env); err != nil {
			t.Fatal(err)
		}
		if res := processEnvelope(t, db, env, int64(200+i)); res.Disposition != platform.DispositionApplied {
			t.Fatalf("foreign apply %s: %+v", env.EventType, res)
		}
	}
}

func TestOpsReaderCanTraceSameTenantBatch(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	applyLineageFixture(t, db, fx)

	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := getWithSession(t, ts.URL+lineagePath(fx.TenantID, fx.BatchID), sess, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var snap api.BatchTraceSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
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
	if strings.Contains(string(mustJSON(t, snap)), identity.TenantNS002UUID) {
		t.Fatal("foreign tenant leaked")
	}
}

func TestPlatformOperatorCanTraceSameTenantBatch(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	applyLineageFixture(t, db, fx)

	ts, sess := gatedSession(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))
	resp := getWithSession(t, ts.URL+lineagePath(fx.TenantID, fx.BatchID), sess, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	inv := getWithSession(t, ts.URL+inventoryPath(fx.TenantID), sess, nil)
	assertForbiddenNoProjection(t, inv)
}

func TestUnauthenticatedTraceRefused(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	store := identity.NewStore(time.Hour, nil)
	auth := identity.NewAuthenticator(store, identity.DefaultCookieName)
	srv := newGatedServer(t, db, auth, northstarReaderPolicy())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + lineagePath(fx.TenantID, fx.BatchID))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("content-type=%q", resp.Header.Get("Content-Type"))
	}
	if !strings.Contains(string(body), "unauthenticated") {
		t.Fatalf("body=%s", body)
	}
	for _, leaked := range []string{`"supplier"`, `"source_event_id"`} {
		if strings.Contains(string(body), leaked) {
			t.Fatalf("leaked %s", leaked)
		}
	}
}

func TestMissingRoleTraceDenied(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	applyLineageFixture(t, db, fx)
	policy := identity.NewPolicy(identity.NewDirectory(identity.Assignment{
		PrincipalID: "operator-northstar",
		TenantID:    fx.TenantID,
		RoleID:      "",
	}))
	ts, sess := gatedSession(t, db, "operator-northstar", policy)
	resp := getWithSession(t, ts.URL+lineagePath(fx.TenantID, fx.BatchID), sess, nil)
	assertForbiddenNoLineage(t, resp)
}

func TestUnassignedPrincipalTraceDenied(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	ts, sess := gatedSession(t, db, "svc-relay", identity.NewPolicy(identity.NewDirectory()))
	resp := getWithSession(t, ts.URL+lineagePath(fx.TenantID, fx.BatchID), sess, nil)
	assertForbiddenNoLineage(t, resp)
}

func TestNilPolicyTraceFailsClosed(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	applyLineageFixture(t, db, fx)
	ts, sess := gatedSession(t, db, "operator-northstar", nil)
	resp := getWithSession(t, ts.URL+lineagePath(fx.TenantID, fx.BatchID), sess, nil)
	assertForbiddenNoLineage(t, resp)
}

func TestCrossTenantTracePathDenied(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	applyLineageFixture(t, db, fx)
	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := getWithSession(t, ts.URL+lineagePath(identity.TenantNS002UUID, fx.BatchID), sess, nil)
	assertForbiddenNoLineage(t, resp)
}

func TestPlatformOperatorCrossTenantTraceDenied(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	applyLineageFixture(t, db, fx)
	ts, sess := gatedSession(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))
	resp := getWithSession(t, ts.URL+lineagePath(identity.TenantNS002UUID, fx.BatchID), sess, nil)
	assertForbiddenNoLineage(t, resp)
}

func TestForgedTenantHeaderDoesNotAuthorizeTrace(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	applyLineageFixture(t, db, fx)
	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	denied := getWithSession(t, ts.URL+lineagePath(identity.TenantNS002UUID, fx.BatchID), sess, func(req *http.Request) {
		req.Header.Set("X-Tenant-ID", fx.TenantID)
		q := req.URL.Query()
		q.Set("tenant_id", fx.TenantID)
		req.URL.RawQuery = q.Encode()
	})
	assertForbiddenNoLineage(t, denied)
	ok := getWithSession(t, ts.URL+lineagePath(fx.TenantID, fx.BatchID), sess, nil)
	defer ok.Body.Close()
	if ok.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(ok.Body)
		t.Fatalf("same-tenant status=%d body=%s", ok.StatusCode, body)
	}
}

func TestAuthorizedUnknownBatchIsNotFound(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := getWithSession(t, ts.URL+lineagePath(fx.TenantID, "batch-missing-001"), sess, nil)
	assertNotFoundNoLineage(t, resp)
}

func TestCrossTenantBatchIdDoesNotLeakExistence(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	applyForeignLineage(t, db, fx)
	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	missing := getWithSession(t, ts.URL+lineagePath(fx.TenantID, "batch-missing-001"), sess, nil)
	foreign := getWithSession(t, ts.URL+lineagePath(fx.TenantID, fx.BatchID), sess, nil)
	missingBody := notFoundBody(t, missing)
	foreignBody := notFoundBody(t, foreign)
	if missingBody != foreignBody {
		t.Fatalf("existence leak: missing=%s foreign=%s", missingBody, foreignBody)
	}
}

func TestTraceRejectsMutatingMethods(t *testing.T) {
	db := openTestDB(t)
	fx := mustLineageFixture(t)
	applyLineageFixture(t, db, fx)
	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req, err := http.NewRequest(method, ts.URL+lineagePath(fx.TenantID, fx.BatchID), bytes.NewReader([]byte(`{}`)))
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: identity.DefaultCookieName, Value: sess.ID})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("%s status=%d body=%s", method, resp.StatusCode, body)
		}
	}
}

func assertForbiddenNoLineage(t *testing.T, resp *http.Response) {
	t.Helper()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("content-type=%q", resp.Header.Get("Content-Type"))
	}
	if !strings.Contains(string(body), "forbidden") {
		t.Fatalf("body=%s", body)
	}
	for _, leaked := range []string{`"supplier"`, `"shipment"`, `"source_event_id"`, `"correlation_id"`} {
		if strings.Contains(string(body), leaked) {
			t.Fatalf("leaked %s body=%s", leaked, body)
		}
	}
}

func assertNotFoundNoLineage(t *testing.T, resp *http.Response) {
	t.Helper()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "not_found") {
		t.Fatalf("body=%s", body)
	}
	for _, leaked := range []string{`"supplier"`, `"shipment"`, `"source_event_id"`, `"mill-northstar-001"`} {
		if strings.Contains(string(body), leaked) {
			t.Fatalf("leaked %s body=%s", leaked, body)
		}
	}
}

func notFoundBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var parsed api.ErrorBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Error != "not_found" {
		t.Fatalf("error=%q body=%s", parsed.Error, body)
	}
	return string(body)
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
