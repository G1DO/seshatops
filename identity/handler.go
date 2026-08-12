package identity

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type errorBody struct {
	Error string `json:"error"`
}

type sessionBody struct {
	PrincipalID     string `json:"principal_id"`
	Subject         string `json:"subject"`
	Issuer          string `json:"issuer"`
	AuthenticatedAt string `json:"authenticated_at"`
	ExpiresAt       string `json:"expires_at"`
	CorrelationID   string `json:"correlation_id"`
}

// Handler serves /auth/login, /auth/callback, /auth/logout, and /auth/session.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/login", s.handleLogin)
	mux.HandleFunc("/auth/callback", s.handleCallback)
	mux.HandleFunc("/auth/logout", s.handleLogout)
	mux.HandleFunc("/auth/session", s.handleSession)
	return mux
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, errorBody{Error: "method_not_allowed"})
		return
	}
	state, err := randomID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "login_failed"})
		return
	}
	nonce, err := randomID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "login_failed"})
		return
	}
	verifier := oauth2.GenerateVerifier()
	if err := s.store.putPending(state, verifier, nonce); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "login_failed"})
		return
	}
	s.setLoginCookie(w, state)
	url := s.oauth2.AuthCodeURL(
		state,
		oauth2.S256ChallengeOption(verifier),
		oidc.Nonce(nonce),
	)
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Service) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, errorBody{Error: "method_not_allowed"})
		return
	}
	if msg := r.URL.Query().Get("error"); msg != "" {
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "invalid_assertion"})
		return
	}
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid_callback"})
		return
	}
	loginCookie, err := r.Cookie(DefaultLoginCookieName)
	if err != nil || loginCookie.Value == "" || loginCookie.Value != state {
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "invalid_callback"})
		return
	}
	pending, ok := s.store.takePending(state)
	if !ok {
		s.clearLoginCookie(w)
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "invalid_callback"})
		return
	}
	s.clearLoginCookie(w)
	token, err := s.oauth2.Exchange(r.Context(), code, oauth2.VerifierOption(pending.Verifier))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "invalid_assertion"})
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "invalid_assertion"})
		return
	}
	idToken, err := s.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "invalid_assertion"})
		return
	}
	if idToken.Subject == "" {
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "invalid_assertion"})
		return
	}
	if pending.Nonce != "" && idToken.Nonce != pending.Nonce {
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "invalid_assertion"})
		return
	}
	sess, err := s.store.Create(idToken.Subject, idToken.Issuer, idToken.Subject, "")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "login_failed"})
		return
	}
	s.setSessionCookie(w, sess.ID, sess.ExpiresAt)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, errorBody{Error: "method_not_allowed"})
		return
	}
	if c, err := r.Cookie(s.cfg.CookieName); err == nil {
		s.store.Delete(c.Value)
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, errorBody{Error: "method_not_allowed"})
		return
	}
	sess, err := s.Session(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "unauthenticated"})
		return
	}
	writeJSON(w, http.StatusOK, sessionBody{
		PrincipalID:     sess.PrincipalID,
		Subject:         sess.Subject,
		Issuer:          sess.Issuer,
		AuthenticatedAt: sess.AuthenticatedAt.Format(time.RFC3339Nano),
		ExpiresAt:       sess.ExpiresAt.Format(time.RFC3339Nano),
		CorrelationID:   sess.CorrelationID,
	})
}

func (s *Service) setLoginCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     DefaultLoginCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   int(defaultPendingTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
	})
}

func (s *Service) clearLoginCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     DefaultLoginCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
	})
}

func (s *Service) setSessionCookie(w http.ResponseWriter, id string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.CookieName,
		Value:    id,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
	})
}

func (s *Service) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
