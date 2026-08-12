package identity

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const (
	testClientID = "seshatops-ops"
	testSubject  = "operator-northstar"
)

func newTestService(t *testing.T, op *mockOIDC) *Service {
	t.Helper()
	redirect := "http://seshatops.test/auth/callback"
	svc, err := New(context.Background(), Config{
		Issuer:      op.issuer(),
		ClientID:    testClientID,
		RedirectURL: redirect,
		SessionTTL:  time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func newAuthServer(t *testing.T, svc *Service) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(svc.Handler())
	t.Cleanup(ts.Close)
	// Callback URL in OAuth config is a fixed host; rewrite RedirectURL to this server.
	svc.oauth2.RedirectURL = ts.URL + "/auth/callback"
	return ts
}

func jarClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func followLogin(t *testing.T, client *http.Client, loginURL string) *http.Response {
	t.Helper()
	resp, err := client.Get(loginURL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("login status=%d body=%s", resp.StatusCode, body)
	}
	loc := resp.Header.Get("Location")
	_ = resp.Body.Close()
	if loc == "" {
		t.Fatal("login missing Location")
	}

	resp, err = client.Get(loc)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("authorize status=%d body=%s", resp.StatusCode, body)
	}
	callback := resp.Header.Get("Location")
	_ = resp.Body.Close()
	if callback == "" {
		t.Fatal("authorize missing Location")
	}

	resp, err = client.Get(callback)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestOIDCLoginEstablishesSession(t *testing.T) {
	op := startMockOIDC(t, testClientID, testSubject)
	svc := newTestService(t, op)
	ts := newAuthServer(t, svc)
	client := jarClient(t)

	resp := followLogin(t, client, ts.URL+"/auth/login")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("callback status=%d body=%s", resp.StatusCode, body)
	}
	if loc := resp.Header.Get("Location"); loc != "/" {
		t.Fatalf("callback redirect = %q", loc)
	}

	sessResp, err := client.Get(ts.URL + "/auth/session")
	if err != nil {
		t.Fatal(err)
	}
	defer sessResp.Body.Close()
	if sessResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(sessResp.Body)
		t.Fatalf("session status=%d body=%s", sessResp.StatusCode, body)
	}
	var body sessionBody
	if err := json.NewDecoder(sessResp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.PrincipalID != testSubject || body.Subject != testSubject || body.Issuer != op.issuer() {
		t.Fatalf("session = %+v", body)
	}
	if body.AuthenticatedAt == "" || body.ExpiresAt == "" || body.CorrelationID == "" {
		t.Fatalf("missing freshness fields: %+v", body)
	}
	if _, err := time.Parse(time.RFC3339Nano, body.ExpiresAt); err != nil {
		t.Fatalf("expires_at: %v", err)
	}
}

func TestCallbackRejectsUnboundState(t *testing.T) {
	op := startMockOIDC(t, testClientID, testSubject)
	svc := newTestService(t, op)
	ts := newAuthServer(t, svc)
	client := jarClient(t)

	loginResp, err := client.Get(ts.URL + "/auth/login")
	if err != nil {
		t.Fatal(err)
	}
	if loginResp.StatusCode != http.StatusFound {
		t.Fatalf("login status=%d", loginResp.StatusCode)
	}
	idpURL := loginResp.Header.Get("Location")
	_ = loginResp.Body.Close()

	var loginCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == DefaultLoginCookieName {
			loginCookie = c
		}
	}
	if loginCookie == nil || loginCookie.Value == "" {
		t.Fatal("login missing state cookie")
	}
	if !loginCookie.HttpOnly || loginCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("login cookie flags: httponly=%v samesite=%v", loginCookie.HttpOnly, loginCookie.SameSite)
	}

	authzResp, err := client.Get(idpURL)
	if err != nil {
		t.Fatal(err)
	}
	callback := authzResp.Header.Get("Location")
	_ = authzResp.Body.Close()
	if callback == "" {
		t.Fatal("authorize missing Location")
	}

	naked := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	attack, err := naked.Get(callback)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(attack.Body)
	_ = attack.Body.Close()
	if attack.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unbound callback status=%d body=%s", attack.StatusCode, body)
	}

	okResp, err := client.Get(callback)
	if err != nil {
		t.Fatal(err)
	}
	defer okResp.Body.Close()
	if okResp.StatusCode != http.StatusFound {
		got, _ := io.ReadAll(okResp.Body)
		t.Fatalf("bound callback status=%d body=%s", okResp.StatusCode, got)
	}

	sessResp, err := client.Get(ts.URL + "/auth/session")
	if err != nil {
		t.Fatal(err)
	}
	defer sessResp.Body.Close()
	if sessResp.StatusCode != http.StatusOK {
		t.Fatalf("session status=%d", sessResp.StatusCode)
	}
}

func TestExpiredPendingLoginIsEvicted(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	store := NewStore(time.Hour, func() time.Time { return now })
	if err := store.putPending("old", "v1", "n1"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(defaultPendingTTL + time.Second)
	if err := store.putPending("new", "v2", "n2"); err != nil {
		t.Fatal(err)
	}
	if store.pendingLen() != 1 {
		t.Fatalf("pendingLen=%d", store.pendingLen())
	}
	if _, ok := store.takePending("old"); ok {
		t.Fatal("expired pending survived")
	}
	if _, ok := store.takePending("new"); !ok {
		t.Fatal("fresh pending missing")
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	op := startMockOIDC(t, testClientID, testSubject)
	svc := newTestService(t, op)
	ts := newAuthServer(t, svc)
	client := jarClient(t)
	resp := followLogin(t, client, ts.URL+"/auth/login")
	_ = resp.Body.Close()

	u, _ := url.Parse(ts.URL)
	var sessionID string
	for _, c := range client.Jar.Cookies(u) {
		if c.Name == DefaultCookieName {
			sessionID = c.Value
		}
	}
	if sessionID == "" {
		t.Fatal("missing session cookie after login")
	}

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/auth/logout", nil)
	if err != nil {
		t.Fatal(err)
	}
	logoutResp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status=%d", logoutResp.StatusCode)
	}

	sessResp, err := client.Get(ts.URL + "/auth/session")
	if err != nil {
		t.Fatal(err)
	}
	defer sessResp.Body.Close()
	if sessResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("session after logout status=%d", sessResp.StatusCode)
	}

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/auth/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: DefaultCookieName, Value: sessionID})
	replay := httptest.NewRecorder()
	svc.Handler().ServeHTTP(replay, req)
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replay status=%d", replay.Code)
	}
}

func TestMissingSessionIsUnauthenticated(t *testing.T) {
	op := startMockOIDC(t, testClientID, testSubject)
	svc := newTestService(t, op)
	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unauthenticated") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestForgedSessionCookieRejected(t *testing.T) {
	op := startMockOIDC(t, testClientID, testSubject)
	svc := newTestService(t, op)
	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: DefaultCookieName, Value: "forged-session-id"})
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestExpiredSessionRejected(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	store := NewStore(time.Minute, func() time.Time { return now })
	sess, err := store.Create(testSubject, "https://idp.test", testSubject, "corr")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	auth := NewAuthenticator(store, DefaultCookieName)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: DefaultCookieName, Value: sess.ID})
	if _, err := auth.Session(req); err != ErrUnauthenticated {
		t.Fatalf("err=%v", err)
	}
}

func TestClientSuppliedPrincipalIgnored(t *testing.T) {
	store := NewStore(time.Hour, nil)
	auth := NewAuthenticator(store, DefaultCookieName)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-Principal-ID", "attacker")
	req.Header.Set("X-Tenant-ID", "11111111-1111-4111-8111-111111111111")
	if _, err := auth.Session(req); err != ErrUnauthenticated {
		t.Fatalf("header-only session: %v", err)
	}

	sess, err := store.Create(testSubject, "https://idp.test", testSubject, "corr")
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: DefaultCookieName, Value: sess.ID})
	got, err := auth.Session(req)
	if err != nil {
		t.Fatal(err)
	}
	if got.PrincipalID != testSubject {
		t.Fatalf("principal=%q", got.PrincipalID)
	}
}

func TestRequireSessionGatesHandler(t *testing.T) {
	store := NewStore(time.Hour, nil)
	auth := NewAuthenticator(store, DefaultCookieName)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := FromContext(r.Context())
		if !ok {
			t.Error("missing session on context")
		}
		_, _ = w.Write([]byte("secret:" + sess.PrincipalID))
	})
	h := RequireSession(auth, inner)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/secret", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie status=%d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content-type=%q", rec.Header().Get("Content-Type"))
	}

	sess, err := store.Create(testSubject, "https://idp.test", testSubject, "corr")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/secret", nil)
	req.AddCookie(&http.Cookie{Name: DefaultCookieName, Value: sess.ID})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "secret:"+testSubject {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestForgedIDTokenRejected(t *testing.T) {
	op := startMockOIDC(t, testClientID, testSubject)
	op.mu.Lock()
	op.tokenHook = func(claims map[string]any, _ *rsa.PrivateKey) string {
		tok, err := signIDToken(claims, op.wrongKey)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	op.mu.Unlock()
	assertCallbackUnauthorized(t, op)
}

func TestSwappedAudienceRejected(t *testing.T) {
	op := startMockOIDC(t, testClientID, testSubject)
	op.mu.Lock()
	op.tokenHook = func(claims map[string]any, key *rsa.PrivateKey) string {
		claims["aud"] = "other-client"
		tok, err := signIDToken(claims, key)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	op.mu.Unlock()
	assertCallbackUnauthorized(t, op)
}

func TestSwappedIssuerRejected(t *testing.T) {
	op := startMockOIDC(t, testClientID, testSubject)
	op.mu.Lock()
	op.tokenHook = func(claims map[string]any, key *rsa.PrivateKey) string {
		claims["iss"] = "https://evil.example"
		tok, err := signIDToken(claims, key)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	op.mu.Unlock()
	assertCallbackUnauthorized(t, op)
}

func TestExpiredIDTokenRejected(t *testing.T) {
	op := startMockOIDC(t, testClientID, testSubject)
	op.mu.Lock()
	op.tokenHook = func(claims map[string]any, key *rsa.PrivateKey) string {
		claims["exp"] = time.Now().UTC().Add(-2 * time.Hour).Unix()
		claims["iat"] = time.Now().UTC().Add(-3 * time.Hour).Unix()
		tok, err := signIDToken(claims, key)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	op.mu.Unlock()
	assertCallbackUnauthorized(t, op)
}

func TestMissingCallbackParamsRejected(t *testing.T) {
	op := startMockOIDC(t, testClientID, testSubject)
	svc := newTestService(t, op)
	req := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func assertCallbackUnauthorized(t *testing.T, op *mockOIDC) {
	t.Helper()
	svc := newTestService(t, op)
	ts := newAuthServer(t, svc)
	client := jarClient(t)
	resp := followLogin(t, client, ts.URL+"/auth/login")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("callback status=%d body=%s", resp.StatusCode, body)
	}
	sessResp, err := client.Get(ts.URL + "/auth/session")
	if err != nil {
		t.Fatal(err)
	}
	defer sessResp.Body.Close()
	if sessResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("session status=%d", sessResp.StatusCode)
	}
}
