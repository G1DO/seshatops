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
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/relay"
)

func auditPath(tenantID string) string {
	return "/v1/tenants/" + tenantID + "/ops/audit"
}

func mustCount(t *testing.T, db *sql.DB, tenantID string) int {
	t.Helper()
	var n int
	var err error
	if tenantID == "" {
		err = db.QueryRow(`SELECT COUNT(*) FROM identity.authorization_decisions`).Scan(&n)
	} else {
		err = db.QueryRow(`SELECT COUNT(*) FROM identity.authorization_decisions WHERE tenant_id = $1`, tenantID).Scan(&n)
	}
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func mustList(t *testing.T, db *sql.DB, tenantID string) []identity.DecisionRecord {
	t.Helper()
	rows, err := identity.ListDecisionsForTenant(context.Background(), db, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func findDecision(t *testing.T, rows []identity.DecisionRecord, action, outcome string) identity.DecisionRecord {
	t.Helper()
	for _, rec := range rows {
		if rec.Action == action && rec.Outcome == outcome {
			return rec
		}
	}
	t.Fatalf("missing action=%s outcome=%s in %+v", action, outcome, rows)
	return identity.DecisionRecord{}
}

func TestOperatorReleaseAllowPersistsAudit(t *testing.T) {
	db := openTestDB(t)
	fx := seedQuarantinedOutbox(t, db)
	ts, _, sess, decisions := controlServer(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))

	resp := postWithSession(t, ts.URL+releasePath(fx.TenantID), sess, api.ControlRequest{EventID: fx.Event.EventID}, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if len(*decisions) != 1 || (*decisions)[0].Outcome != "allow" || (*decisions)[0].DecisionID == "" {
		t.Fatalf("decisions=%+v", *decisions)
	}
	rec := findDecision(t, mustList(t, db, fx.TenantID), identity.ActQuarantineRelease, "allow")
	if rec.PrincipalID != "platform-operator" || rec.TenantID != fx.TenantID {
		t.Fatalf("record=%+v", rec)
	}
	if rec.Resource != identity.ResQuarantine || rec.Reason != "matrix_allow" {
		t.Fatalf("record=%+v", rec)
	}
	if rec.TargetID != fx.Event.EventID || rec.RecordedAt.IsZero() {
		t.Fatalf("record=%+v", rec)
	}
}

func TestReaderDenyPersistsAuditWithoutMutation(t *testing.T) {
	db := openTestDB(t)
	fx := seedQuarantinedOutbox(t, db)
	ts, _, sess, _ := controlServer(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))

	resp := postWithSession(t, ts.URL+releasePath(fx.TenantID), sess, api.ControlRequest{EventID: fx.Event.EventID}, nil)
	assertForbiddenControl(t, resp)
	if outboxStatus(t, db, fx.Event.EventID) != relay.StatusQuarantined {
		t.Fatal("unauthorized release mutated outbox")
	}
	rec := findDecision(t, mustList(t, db, fx.TenantID), identity.ActQuarantineRelease, "deny")
	if rec.PrincipalID != "operator-northstar" || rec.TenantID != fx.TenantID {
		t.Fatalf("record=%+v", rec)
	}
}

func TestAuditInsertFailureBlocksPrivilegedMutation(t *testing.T) {
	db := openTestDB(t)
	fx := seedQuarantinedOutbox(t, db)
	if _, err := db.Exec(`DROP TABLE identity.authorization_decisions`); err != nil {
		t.Fatal(err)
	}
	ts, _, sess, decisions := controlServer(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))

	resp := postWithSession(t, ts.URL+releasePath(fx.TenantID), sess, api.ControlRequest{EventID: fx.Event.EventID}, nil)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "audit_failed") {
		t.Fatalf("body=%s", body)
	}
	if outboxStatus(t, db, fx.Event.EventID) != relay.StatusQuarantined {
		t.Fatal("audit failure mutated outbox")
	}
	if len(*decisions) != 0 {
		t.Fatalf("OnDecision after failed insert: %+v", *decisions)
	}
}

func TestUnauthenticatedControlLeavesNoAuditRow(t *testing.T) {
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
	if n := mustCount(t, db, ""); n != 0 {
		t.Fatalf("audit rows=%d", n)
	}
}

func TestClientSuppliedActorAndTenantAreIgnored(t *testing.T) {
	db := openTestDB(t)
	fx := seedQuarantinedOutbox(t, db)
	ts, _, sess, _ := controlServer(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))

	body := map[string]string{
		"event_id":     fx.Event.EventID,
		"tenant_id":    identity.TenantNS002UUID,
		"principal_id": "forged-actor",
	}
	resp := postWithSession(t, ts.URL+releasePath(fx.TenantID), sess, body, func(req *http.Request) {
		req.Header.Set("X-Tenant-ID", identity.TenantNS002UUID)
		req.URL.RawQuery = "tenant_id=" + identity.TenantNS002UUID + "&principal_id=forged-actor"
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, raw)
	}
	rec := findDecision(t, mustList(t, db, fx.TenantID), identity.ActQuarantineRelease, "allow")
	if rec.PrincipalID != "platform-operator" {
		t.Fatalf("trusted forged actor: %+v", rec)
	}
	if rec.TenantID != fx.TenantID {
		t.Fatalf("trusted forged tenant: %+v", rec)
	}
	if n := mustCount(t, db, identity.TenantNS002UUID); n != 0 {
		t.Fatalf("ns002 rows=%d", n)
	}
}

func TestOperatorCanReadSameTenantAudit(t *testing.T) {
	db := openTestDB(t)
	fx := seedQuarantinedOutbox(t, db)
	ts, _, sess, _ := controlServer(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))

	resp := postWithSession(t, ts.URL+releasePath(fx.TenantID), sess, api.ControlRequest{EventID: fx.Event.EventID}, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}

	got := getWithSession(t, ts.URL+auditPath(fx.TenantID), sess, nil)
	defer got.Body.Close()
	if got.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(got.Body)
		t.Fatalf("status=%d body=%s", got.StatusCode, body)
	}
	var snap api.AuditSnapshot
	if err := json.NewDecoder(got.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	if snap.TenantID != fx.TenantID || snap.ObservedAt == "" {
		t.Fatalf("snapshot=%+v", snap)
	}
	var sawRelease, sawAuditRead bool
	for _, rec := range snap.Records {
		if rec.TenantID != fx.TenantID {
			t.Fatalf("leaked tenant %+v", rec)
		}
		if rec.Action == identity.ActQuarantineRelease && rec.Outcome == "allow" && rec.PrincipalID == "platform-operator" {
			sawRelease = true
		}
		if rec.Action == identity.ActAuditRead && rec.Outcome == "allow" {
			sawAuditRead = true
		}
	}
	if !sawRelease || !sawAuditRead {
		t.Fatalf("timeline=%+v", snap.Records)
	}
}

func TestReaderCannotReadAudit(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ts, _, sess, _ := controlServer(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))

	resp := getWithSession(t, ts.URL+auditPath(fx.TenantID), sess, nil)
	assertForbiddenAudit(t, resp)
	rec := findDecision(t, mustList(t, db, fx.TenantID), identity.ActAuditRead, "deny")
	if rec.PrincipalID != "operator-northstar" {
		t.Fatalf("record=%+v", rec)
	}
}

func TestCrossTenantAndNilPolicyAuditReadDenied(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ts, _, sess, _ := controlServer(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))

	resp := getWithSession(t, ts.URL+auditPath(identity.TenantNS002UUID), sess, func(req *http.Request) {
		req.Header.Set("X-Tenant-ID", fx.TenantID)
		req.URL.RawQuery = "tenant_id=" + fx.TenantID
	})
	assertForbiddenAudit(t, resp)

	ts, _, sess, _ = controlServer(t, db, "platform-operator", nil)
	resp = getWithSession(t, ts.URL+auditPath(fx.TenantID), sess, nil)
	assertForbiddenAudit(t, resp)
}

func TestAuditReadDoesNotLeakOtherTenant(t *testing.T) {
	db := openTestDB(t)
	fx := seedQuarantinedOutbox(t, db)
	if _, err := identity.AppendDecision(context.Background(), db, identity.DecisionRecord{
		PrincipalID: "other-operator",
		TenantID:    identity.TenantNS002UUID,
		Resource:    identity.ResAudit,
		Action:      identity.ActAuditRead,
		Outcome:     "allow",
		Reason:      "seed",
	}); err != nil {
		t.Fatal(err)
	}
	ts, _, sess, _ := controlServer(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))
	resp := getWithSession(t, ts.URL+auditPath(fx.TenantID), sess, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var snap api.AuditSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	for _, rec := range snap.Records {
		if rec.TenantID == identity.TenantNS002UUID || rec.PrincipalID == "other-operator" {
			t.Fatalf("leaked ns002 %+v", rec)
		}
	}
}

func TestAuthorizationDecisionsAreAppendOnly(t *testing.T) {
	db := openTestDB(t)
	rec, err := identity.AppendDecision(context.Background(), db, identity.DecisionRecord{
		PrincipalID: "platform-operator",
		TenantID:    identity.TenantNS001UUID,
		Resource:    identity.ResAudit,
		Action:      identity.ActAuditRead,
		Outcome:     "allow",
		Reason:      "matrix_allow",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE identity.authorization_decisions SET reason = 'tampered' WHERE decision_id = $1`, rec.DecisionID); err == nil {
		t.Fatal("update succeeded")
	}
	if _, err := db.Exec(`DELETE FROM identity.authorization_decisions WHERE decision_id = $1`, rec.DecisionID); err == nil {
		t.Fatal("delete succeeded")
	}
	got := mustList(t, db, identity.TenantNS001UUID)
	if len(got) != 1 || got[0].Reason != "matrix_allow" || got[0].DecisionID != rec.DecisionID {
		t.Fatalf("rows=%+v", got)
	}
}

func TestAuditPathRejectsMutatingMethods(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	ts, sess := gatedSession(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req, err := http.NewRequest(method, ts.URL+auditPath(fx.TenantID), bytes.NewReader([]byte(`{}`)))
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

func TestUnauthenticatedAuditReadFailsClosed(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	srv := newGatedServer(t, db, identity.NewAuthenticator(identity.NewStore(time.Hour, nil), identity.DefaultCookieName), northstarOperatorPolicy("platform-operator"))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	resp, err := http.Get(ts.URL + auditPath(fx.TenantID))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if n := mustCount(t, db, ""); n != 0 {
		t.Fatalf("audit rows=%d", n)
	}
}

func assertForbiddenAudit(t *testing.T, resp *http.Response) {
	t.Helper()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "forbidden") {
		t.Fatalf("body=%s", body)
	}
	if strings.Contains(string(body), `"records"`) || strings.Contains(string(body), `"principal_id"`) {
		t.Fatalf("leaked audit body=%s", body)
	}
}
