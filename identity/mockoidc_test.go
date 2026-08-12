package identity

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

const testKeyID = "seshatops-test-key"

type issuedCode struct {
	Challenge string
	Nonce     string
	Subject   string
	Redirect  string
}

type tokenHook func(claims map[string]any, key *rsa.PrivateKey) (idToken string)

type mockOIDC struct {
	t         *testing.T
	server    *httptest.Server
	key       *rsa.PrivateKey
	clientID  string
	subject   string
	mu        sync.Mutex
	codes     map[string]issuedCode
	tokenHook tokenHook
	wrongKey  *rsa.PrivateKey
}

func startMockOIDC(t *testing.T, clientID, subject string) *mockOIDC {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	wrong, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	m := &mockOIDC{
		t:        t,
		key:      key,
		wrongKey: wrong,
		clientID: clientID,
		subject:  subject,
		codes:    make(map[string]issuedCode),
	}
	mux := http.NewServeMux()
	m.server = httptest.NewServer(mux)
	t.Cleanup(m.server.Close)
	mux.HandleFunc("/.well-known/openid-configuration", m.discovery)
	mux.HandleFunc("/jwks", m.jwks)
	mux.HandleFunc("/authorize", m.authorize)
	mux.HandleFunc("/token", m.token)
	return m
}

func (m *mockOIDC) issuer() string { return m.server.URL }

func (m *mockOIDC) discovery(w http.ResponseWriter, _ *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                m.issuer(),
		"authorization_endpoint":                m.issuer() + "/authorize",
		"token_endpoint":                        m.issuer() + "/token",
		"jwks_uri":                              m.issuer() + "/jwks",
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"code_challenge_methods_supported":      []string{"S256"},
	})
}

func (m *mockOIDC) jwks(w http.ResponseWriter, _ *http.Request) {
	n := base64.RawURLEncoding.EncodeToString(m.key.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(m.key.E)).Bytes())
	_ = json.NewEncoder(w).Encode(map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"kid": testKeyID,
			"use": "sig",
			"alg": "RS256",
			"n":   n,
			"e":   e,
		}},
	})
}

func (m *mockOIDC) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("client_id") != m.clientID {
		http.Error(w, "bad client", http.StatusBadRequest)
		return
	}
	redirect := q.Get("redirect_uri")
	state := q.Get("state")
	challenge := q.Get("code_challenge")
	if redirect == "" || state == "" || challenge == "" {
		http.Error(w, "missing params", http.StatusBadRequest)
		return
	}
	if q.Get("code_challenge_method") != "S256" {
		http.Error(w, "pkce required", http.StatusBadRequest)
		return
	}
	code, err := randomID()
	if err != nil {
		http.Error(w, "code", http.StatusInternalServerError)
		return
	}
	m.mu.Lock()
	m.codes[code] = issuedCode{
		Challenge: challenge,
		Nonce:     q.Get("nonce"),
		Subject:   m.subject,
		Redirect:  redirect,
	}
	m.mu.Unlock()
	u, err := url.Parse(redirect)
	if err != nil {
		http.Error(w, "redirect", http.StatusBadRequest)
		return
	}
	vals := u.Query()
	vals.Set("code", code)
	vals.Set("state", state)
	u.RawQuery = vals.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (m *mockOIDC) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form", http.StatusBadRequest)
		return
	}
	code := r.Form.Get("code")
	verifier := r.Form.Get("code_verifier")
	m.mu.Lock()
	issued, ok := m.codes[code]
	if ok {
		delete(m.codes, code)
	}
	hook := m.tokenHook
	m.mu.Unlock()
	if !ok {
		http.Error(w, "unknown code", http.StatusBadRequest)
		return
	}
	if s256Challenge(verifier) != issued.Challenge {
		http.Error(w, "pkce", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	claims := map[string]any{
		"iss":   m.issuer(),
		"sub":   issued.Subject,
		"aud":   m.clientID,
		"exp":   now.Add(time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": issued.Nonce,
	}
	var idToken string
	if hook != nil {
		idToken = hook(claims, m.key)
	} else {
		var err error
		idToken, err = signIDToken(claims, m.key)
		if err != nil {
			http.Error(w, "sign", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "not-used",
		"token_type":   "Bearer",
		"id_token":     idToken,
	})
}

func s256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func signIDToken(claims map[string]any, key *rsa.PrivateKey) (string, error) {
	header, err := json.Marshal(map[string]string{
		"alg": "RS256",
		"kid": testKeyID,
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	h := base64.RawURLEncoding.EncodeToString(header)
	p := base64.RawURLEncoding.EncodeToString(payload)
	signing := h + "." + p
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
