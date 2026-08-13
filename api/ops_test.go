package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/G1DO/seshatops/api"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/platform"
	"github.com/G1DO/seshatops/relay"
)

func TestOpsReaderCanReadSameTenantOps(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	seedOpsSignals(t, db, fx)

	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := getWithSession(t, ts.URL+opsPath(fx.TenantID), sess, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var snap api.OpsSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	assertTenantOpsSnapshot(t, snap, fx.TenantID, fx.Event.EventID)
}

func TestPlatformOperatorCanReadOpsButNotInventory(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	seedOpsSignals(t, db, fx)

	ts, sess := gatedSession(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))
	resp := getWithSession(t, ts.URL+opsPath(fx.TenantID), sess, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("ops status=%d body=%s", resp.StatusCode, body)
	}
	var snap api.OpsSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	assertTenantOpsSnapshot(t, snap, fx.TenantID, fx.Event.EventID)

	inv := getWithSession(t, ts.URL+inventoryPath(fx.TenantID), sess, nil)
	assertForbiddenNoProjection(t, inv)
}

func TestCrossTenantOpsReadDenied(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	seedOpsSignals(t, db, fx)

	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := getWithSession(t, ts.URL+opsPath(identity.TenantNS002UUID), sess, nil)
	assertForbiddenNoOps(t, resp)
}

func TestPlatformOperatorCrossTenantOpsDenied(t *testing.T) {
	db := openTestDB(t)
	ts, sess := gatedSession(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))
	resp := getWithSession(t, ts.URL+opsPath(identity.TenantNS002UUID), sess, nil)
	assertForbiddenNoOps(t, resp)
}

func TestMissingRoleOpsReadDenied(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	policy := identity.NewPolicy(identity.NewDirectory(identity.Assignment{
		PrincipalID: "operator-northstar",
		TenantID:    fx.TenantID,
		RoleID:      "",
	}))
	ts, sess := gatedSession(t, db, "operator-northstar", policy)
	resp := getWithSession(t, ts.URL+opsPath(fx.TenantID), sess, nil)
	assertForbiddenNoOps(t, resp)
}

func TestUnassignedPrincipalOpsReadDenied(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ts, sess := gatedSession(t, db, "svc-relay", identity.NewPolicy(identity.NewDirectory()))
	resp := getWithSession(t, ts.URL+opsPath(fx.TenantID), sess, nil)
	assertForbiddenNoOps(t, resp)
}

func TestNilPolicyOpsFailsClosed(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ts, sess := gatedSession(t, db, "operator-northstar", nil)
	resp := getWithSession(t, ts.URL+opsPath(fx.TenantID), sess, nil)
	assertForbiddenNoOps(t, resp)
}

func TestForgedTenantHeaderDoesNotAuthorizeOps(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	seedOpsSignals(t, db, fx)
	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))

	resp := getWithSession(t, ts.URL+opsPath(identity.TenantNS002UUID)+"?tenant_id="+fx.TenantID, sess, func(req *http.Request) {
		req.Header.Set("X-Tenant-ID", fx.TenantID)
		req.Header.Set("X-Role", identity.RoleOpsReader)
	})
	assertForbiddenNoOps(t, resp)

	resp = getWithSession(t, ts.URL+opsPath(fx.TenantID)+"?tenant_id="+identity.TenantNS002UUID, sess, func(req *http.Request) {
		req.Header.Set("X-Tenant-ID", identity.TenantNS002UUID)
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("same-tenant path status=%d body=%s", resp.StatusCode, body)
	}
}

func TestOpsSnapshotExcludesOtherTenantAndPayloadFragments(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	seedOpsSignals(t, db, fx)

	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := getWithSession(t, ts.URL+opsPath(fx.TenantID), sess, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), identity.TenantNS002UUID) {
		t.Fatalf("leaked other tenant: %s", raw)
	}
	if strings.Contains(string(raw), `"not"`) || strings.Contains(string(raw), "an-envelope") {
		t.Fatalf("leaked payload fragment: %s", raw)
	}
	if strings.Contains(string(raw), "other_tenant_poison") {
		t.Fatalf("leaked other-tenant diagnostic: %s", raw)
	}
	var snap api.OpsSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	assertTenantOpsSnapshot(t, snap, fx.TenantID, fx.Event.EventID)
}

func seedOpsSignals(t *testing.T, db *sql.DB, fx northstar.Fixture) {
	t.Helper()
	ctx := context.Background()

	applied := processEnvelope(t, db, fx.Event, 50)
	if applied.Disposition != platform.DispositionApplied {
		t.Fatalf("applied disposition = %s", applied.Disposition)
	}

	gap := fx.Event
	gap.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2152"
	gap.AggregateID = "item-yeast-001"
	gap.AggregateVersion = 2
	gap.Payload.ItemID = "item-yeast-001"
	gap.Payload.QuantityBefore = 8
	gap.Payload.QuantityDecremented = 1
	gap.Payload.QuantityAfter = 7
	gap.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2154"
	gap.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2153"
	gapped := processEnvelope(t, db, gap, 51)
	if gapped.Disposition != platform.DispositionQuarantinedGap {
		t.Fatalf("gap disposition = %s", gapped.Disposition)
	}

	other := fx.Event
	other.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2151"
	other.TenantID = identity.TenantNS002UUID
	other.AggregateID = "item-sugar-001"
	other.Payload.ItemID = "item-sugar-001"
	other.Payload.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2155"
	other.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a2156"
	otherApplied := processEnvelope(t, db, other, 52)
	if otherApplied.Disposition != platform.DispositionApplied {
		t.Fatalf("other tenant disposition = %s", otherApplied.Disposition)
	}

	malformed := []byte(`{"not":"an-envelope"`)
	if _, err := platform.ProcessRecord(ctx, db, []byte("k"), malformed, platform.SourcePosition{
		Topic: relay.Topic, Partition: 0, Offset: 53,
	}); err != nil {
		t.Fatal(err)
	}

	insertProcessingFailure(t, db, "fail-same-tenant", fx.TenantID, fx.Event.EventID, 54, "handler_poison")
	insertProcessingFailure(t, db, "fail-other-tenant", identity.TenantNS002UUID, other.EventID, 55, "other_tenant_poison")

	insertOutbox(t, db, fx.Event.EventID, fx.TenantID, "pending", "")
	insertOutbox(t, db, "018f5d78-6e64-4f5f-bd16-8e9f7c4a2199", identity.TenantNS002UUID, "quarantined", "other_tenant_poison")
}

func insertOutbox(t *testing.T, db *sql.DB, eventID, tenantID, status, errCode string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO erp.outbox (
			event_id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
			content_hash, event_bytes, status, recorded_at, last_error_code
		) VALUES (
			$1, $2, 'inventory_item', 'item-flour-001', 1,
			'aa', '{}', $3, now(), NULLIF($4, '')
		)
	`, eventID, tenantID, status, errCode)
	if err != nil {
		t.Fatal(err)
	}
}

func insertProcessingFailure(t *testing.T, db *sql.DB, failureID, tenantID, eventID string, offset int64, diagnostic string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO platform.processing_failures (
			failure_id, consumer_name, event_id, tenant_id,
			failure_category, diagnostic_code, quarantine_status,
			source_topic, source_partition, source_offset, attempt_count
		) VALUES (
			$1, $2, $3, $4,
			'handler_poison', $5, 'quarantined',
			$6, 0, $7, 1
		)
	`, failureID, platform.ConsumerName, eventID, tenantID, diagnostic, relay.Topic, offset)
	if err != nil {
		t.Fatal(err)
	}
}

func assertTenantOpsSnapshot(t *testing.T, snap api.OpsSnapshot, tenantID, eventID string) {
	t.Helper()
	if snap.TenantID != tenantID || snap.ObservedAt == "" || snap.Projection.Checksum == "" {
		t.Fatalf("snapshot meta = %+v", snap)
	}
	if snap.Projection.ItemCount != 1 {
		t.Fatalf("item_count = %d", snap.Projection.ItemCount)
	}
	if snap.Backlog.Pending != 1 || snap.Backlog.Quarantined != 0 || len(snap.Backlog.Quarantines) != 0 {
		t.Fatalf("backlog leaked or missing: %+v", snap.Backlog)
	}
	if snap.Backlog.OldestUnpublished == nil {
		t.Fatal("expected oldest_unpublished")
	}
	if snap.Processing.Applied != 1 || snap.Processing.QuarantinedGap != 1 {
		t.Fatalf("processing counts = %+v", snap.Processing)
	}
	if snap.Processing.FailuresQuarantined != 1 || len(snap.Processing.Failures) != 1 {
		t.Fatalf("expected one attributed tenant failure, got %+v", snap.Processing)
	}
	fail := snap.Processing.Failures[0]
	if fail.DiagnosticCode != "handler_poison" || fail.EventID != eventID {
		t.Fatalf("attributed failure = %+v", fail)
	}
	if strings.Contains(fail.DiagnosticCode, `"not"`) || strings.Contains(fail.EventID, "an-envelope") {
		t.Fatalf("failure sample leaked payload: %+v", fail)
	}
	if len(snap.Processing.Gaps) != 1 || snap.Processing.Gaps[0].TenantID != tenantID {
		t.Fatalf("gaps = %+v", snap.Processing.Gaps)
	}
}

func assertForbiddenNoOps(t *testing.T, resp *http.Response) {
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
	for _, leaked := range []string{`"backlog"`, `"processing"`, `"quarantines"`, `"checksum"`, `"items"`} {
		if strings.Contains(string(body), leaked) {
			t.Fatalf("leaked %s body=%s", leaked, body)
		}
	}
}
