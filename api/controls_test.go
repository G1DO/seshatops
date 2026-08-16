package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/G1DO/seshatops/api"
	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/event"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/platform"
	"github.com/G1DO/seshatops/relay"
)

func releasePath(tenantID string) string {
	return "/v1/tenants/" + tenantID + "/ops/quarantine/release"
}

func replayPath(tenantID string) string {
	return "/v1/tenants/" + tenantID + "/ops/replay"
}

func rebuildPath(tenantID string) string {
	return "/v1/tenants/" + tenantID + "/ops/rebuild"
}

func postWithSession(t *testing.T, rawURL string, sess identity.Session, body any, mutate func(*http.Request)) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(http.MethodPost, rawURL, rdr)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: identity.DefaultCookieName, Value: sess.ID})
	if mutate != nil {
		mutate(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func controlServer(t *testing.T, db *sql.DB, principalID string, policy identity.Authorizer) (*httptest.Server, *api.Server, identity.Session, *[]api.ControlDecision) {
	t.Helper()
	store := identity.NewStore(time.Hour, nil)
	sess, err := store.Create(principalID, "https://idp.test", principalID, "corr")
	if err != nil {
		t.Fatal(err)
	}
	auth := identity.NewAuthenticator(store, identity.DefaultCookieName)
	srv := newGatedServer(t, db, auth, policy)
	got := make([]api.ControlDecision, 0)
	srv.OnDecision = func(d api.ControlDecision) {
		got = append(got, d)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, srv, sess, &got
}

func seedQuarantinedOutbox(t *testing.T, db *sql.DB) northstar.Fixture {
	t.Helper()
	fx := mustFixture(t)
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	res, err := erp.AcceptOrder(ctx, db, mustOrderCommand(t, fx))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := relay.ClaimDue(ctx, db, "worker-a", time.Minute, 1); err != nil {
		t.Fatal(err)
	}
	if err := relay.Quarantine(ctx, db, res.EventID, "worker-a", "malformed_envelope"); err != nil {
		t.Fatal(err)
	}
	return fx
}

func outboxStatus(t *testing.T, db *sql.DB, eventID string) string {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT status FROM erp.outbox WHERE event_id = $1`, eventID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func assertForbiddenControl(t *testing.T, resp *http.Response) {
	t.Helper()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "forbidden") {
		t.Fatalf("body=%s", body)
	}
	if strings.Contains(string(body), `"control"`) || strings.Contains(string(body), `"checksum"`) {
		t.Fatalf("leaked control body=%s", body)
	}
}

func TestOperatorCanReleaseSameTenantQuarantine(t *testing.T) {
	db := openTestDB(t)
	fx := seedQuarantinedOutbox(t, db)
	ts, _, sess, decisions := controlServer(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))

	resp := postWithSession(t, ts.URL+releasePath(fx.TenantID), sess, api.ControlRequest{EventID: fx.Event.EventID}, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var out api.ControlResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "released" || out.EventID != fx.Event.EventID {
		t.Fatalf("result=%+v", out)
	}
	if outboxStatus(t, db, fx.Event.EventID) != relay.StatusPending {
		t.Fatal("outbox not released")
	}
	if len(*decisions) != 1 || (*decisions)[0].Outcome != "allow" || (*decisions)[0].Action != identity.ActQuarantineRelease {
		t.Fatalf("decisions=%+v", *decisions)
	}
}

func TestReaderCannotReleaseQuarantine(t *testing.T) {
	db := openTestDB(t)
	fx := seedQuarantinedOutbox(t, db)
	ts, _, sess, decisions := controlServer(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))

	resp := postWithSession(t, ts.URL+releasePath(fx.TenantID), sess, api.ControlRequest{EventID: fx.Event.EventID}, nil)
	assertForbiddenControl(t, resp)
	if outboxStatus(t, db, fx.Event.EventID) != relay.StatusQuarantined {
		t.Fatal("unauthorized release mutated outbox")
	}
	if len(*decisions) != 1 || (*decisions)[0].Outcome != "deny" {
		t.Fatalf("decisions=%+v", *decisions)
	}
}

func TestUnauthenticatedControlFailsClosed(t *testing.T) {
	db := openTestDB(t)
	fx := seedQuarantinedOutbox(t, db)
	srv := newGatedServer(t, db, identity.NewAuthenticator(identity.NewStore(time.Hour, nil), identity.DefaultCookieName), northstarOperatorPolicy("platform-operator"))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	raw, _ := json.Marshal(api.ControlRequest{EventID: fx.Event.EventID})
	resp, err := http.Post(ts.URL+releasePath(fx.TenantID), "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if outboxStatus(t, db, fx.Event.EventID) != relay.StatusQuarantined {
		t.Fatal("unauthenticated release mutated outbox")
	}
}

func TestMissingRoleControlDenied(t *testing.T) {
	db := openTestDB(t)
	fx := seedQuarantinedOutbox(t, db)
	policy := identity.NewPolicy(identity.NewDirectory(identity.Assignment{
		PrincipalID: "platform-operator",
		TenantID:    fx.TenantID,
		RoleID:      "",
	}))
	ts, _, sess, _ := controlServer(t, db, "platform-operator", policy)
	resp := postWithSession(t, ts.URL+releasePath(fx.TenantID), sess, api.ControlRequest{EventID: fx.Event.EventID}, nil)
	assertForbiddenControl(t, resp)
	if outboxStatus(t, db, fx.Event.EventID) != relay.StatusQuarantined {
		t.Fatal("empty role mutated outbox")
	}
}

func TestUnassignedAndNilPolicyControlDenied(t *testing.T) {
	db := openTestDB(t)
	fx := seedQuarantinedOutbox(t, db)

	ts, _, sess, _ := controlServer(t, db, "svc-relay", identity.NewPolicy(identity.NewDirectory()))
	resp := postWithSession(t, ts.URL+releasePath(fx.TenantID), sess, api.ControlRequest{EventID: fx.Event.EventID}, nil)
	assertForbiddenControl(t, resp)

	ts, _, sess, _ = controlServer(t, db, "platform-operator", nil)
	resp = postWithSession(t, ts.URL+rebuildPath(fx.TenantID), sess, map[string]string{}, nil)
	assertForbiddenControl(t, resp)
	if outboxStatus(t, db, fx.Event.EventID) != relay.StatusQuarantined {
		t.Fatal("nil policy mutated outbox")
	}
}

func TestCrossTenantAndForgedContextControlDenied(t *testing.T) {
	db := openTestDB(t)
	fx := seedQuarantinedOutbox(t, db)
	ts, _, sess, _ := controlServer(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))

	resp := postWithSession(t, ts.URL+releasePath(identity.TenantNS002UUID), sess, api.ControlRequest{
		EventID:  fx.Event.EventID,
		TenantID: fx.TenantID,
	}, func(req *http.Request) {
		req.Header.Set("X-Tenant-ID", fx.TenantID)
		req.URL.RawQuery = "tenant_id=" + fx.TenantID
	})
	assertForbiddenControl(t, resp)
	if outboxStatus(t, db, fx.Event.EventID) != relay.StatusQuarantined {
		t.Fatal("forged context mutated outbox")
	}

	foreignID := "018f5d78-6e64-4f5f-bd16-8e9f7c4a21aa"
	if _, err := db.Exec(`
		INSERT INTO erp.outbox (
			event_id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
			content_hash, event_bytes, status, recorded_at
		) VALUES ($1, $2, 'inventory_item', 'item-sugar-001', 1, 'cc', '{}', 'quarantined', now())
	`, foreignID, identity.TenantNS002UUID); err != nil {
		t.Fatal(err)
	}
	resp = postWithSession(t, ts.URL+releasePath(fx.TenantID), sess, api.ControlRequest{
		EventID:  foreignID,
		TenantID: identity.TenantNS002UUID,
	}, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if outboxStatus(t, db, foreignID) != relay.StatusQuarantined {
		t.Fatal("cross-tenant event released")
	}
	if outboxStatus(t, db, fx.Event.EventID) != relay.StatusQuarantined {
		t.Fatal("same-tenant row mutated by foreign event_id")
	}
}

func TestReleaseGapIsNotReleasable(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	gap := fx.Event
	gap.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a21ab"
	gap.AggregateVersion = 2
	gap = event.WithQuantityDecremented(gap, func(p *event.QuantityDecremented) {
		p.QuantityBefore = 8
		p.QuantityDecremented = 1
		p.QuantityAfter = 7
		p.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a21ac"
	})
	gap.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a21ad"
	res := processEnvelope(t, db, gap, 80)
	if res.Disposition != platform.DispositionQuarantinedGap {
		t.Fatalf("disposition=%s", res.Disposition)
	}

	ts, _, sess, decisions := controlServer(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))
	resp := postWithSession(t, ts.URL+releasePath(fx.TenantID), sess, api.ControlRequest{EventID: gap.EventID}, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "not_releasable") {
		t.Fatalf("body=%s", body)
	}
	_, _, ok, err := platform.ProjectionState(context.Background(), db, fx.TenantID, fx.ItemID)
	if err != nil || ok {
		t.Fatalf("gap skipped: ok=%v err=%v", ok, err)
	}
	if len(*decisions) != 1 || (*decisions)[0].Outcome != "allow" {
		t.Fatalf("authorized-but-unsafe should still record allow: %+v", *decisions)
	}
}

func TestOperatorReplayIsDuplicateNoop(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	if _, err := erp.AcceptOrder(ctx, db, mustOrderCommand(t, fx)); err != nil {
		t.Fatal(err)
	}
	applied := processEnvelope(t, db, fx.Event, 1)
	if applied.Disposition != platform.DispositionApplied {
		t.Fatalf("disposition=%s", applied.Disposition)
	}
	var erpQty int64
	if err := db.QueryRow(`SELECT quantity_on_hand FROM erp.inventory_items WHERE tenant_id=$1 AND item_id=$2`, fx.TenantID, fx.ItemID).Scan(&erpQty); err != nil {
		t.Fatal(err)
	}

	ts, _, sess, _ := controlServer(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))
	resp := postWithSession(t, ts.URL+replayPath(fx.TenantID), sess, api.ControlRequest{EventID: fx.Event.EventID}, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var out api.ControlResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.DuplicateNoop != 1 || out.Applied != 0 || out.Status != platform.RebuildStatusComplete {
		t.Fatalf("result=%+v", out)
	}
	var erpAfter int64
	if err := db.QueryRow(`SELECT quantity_on_hand FROM erp.inventory_items WHERE tenant_id=$1 AND item_id=$2`, fx.TenantID, fx.ItemID).Scan(&erpAfter); err != nil {
		t.Fatal(err)
	}
	if erpAfter != erpQty {
		t.Fatal("replay mutated erp")
	}
}

func TestOperatorRebuildLeavesOtherTenant(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ctx := context.Background()
	if err := erp.SeedNorthstarInventory(ctx, db, fx); err != nil {
		t.Fatal(err)
	}
	if _, err := erp.AcceptOrder(ctx, db, mustOrderCommand(t, fx)); err != nil {
		t.Fatal(err)
	}
	if processEnvelope(t, db, fx.Event, 1).Disposition != platform.DispositionApplied {
		t.Fatal("ns001 apply failed")
	}

	other := fx.Event
	other.EventID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a21ae"
	other.TenantID = identity.TenantNS002UUID
	other.AggregateID = "item-sugar-001"
	other = event.WithQuantityDecremented(other, func(p *event.QuantityDecremented) {
		p.ItemID = "item-sugar-001"
		p.OrderID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a21af"
	})
	other.CorrelationID = "018f5d78-6e64-4f5f-bd16-8e9f7c4a21b0"
	if processEnvelope(t, db, other, 2).Disposition != platform.DispositionApplied {
		t.Fatal("ns002 apply failed")
	}

	ts, _, sess, _ := controlServer(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))
	resp := postWithSession(t, ts.URL+rebuildPath(fx.TenantID), sess, map[string]string{}, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	qty, ver, ok, err := platform.ProjectionState(ctx, db, other.TenantID, other.AggregateID)
	if err != nil || !ok || qty != 8 || ver != 1 {
		t.Fatalf("other tenant mutated qty=%d ver=%d ok=%v err=%v", qty, ver, ok, err)
	}
}

func TestReaderCannotReplayOrRebuild(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ts, _, sess, _ := controlServer(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := postWithSession(t, ts.URL+replayPath(fx.TenantID), sess, api.ControlRequest{EventID: fx.Event.EventID}, nil)
	assertForbiddenControl(t, resp)
	resp = postWithSession(t, ts.URL+rebuildPath(fx.TenantID), sess, map[string]string{}, nil)
	assertForbiddenControl(t, resp)
}

func TestControlPathsRejectWrongMethod(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ts, sess := gatedSession(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))
	resp := getWithSession(t, ts.URL+releasePath(fx.TenantID), sess, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}
