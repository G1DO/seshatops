package identity

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Config is the vendor-neutral OIDC relying-party settings. IdP product
// selection remains configuration-time.
type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// Audience is the expected ID-token audience. Empty means ClientID.
	Audience   string
	SessionTTL time.Duration
	// CookieSecure should be true for HTTPS. httptest and local HTTP leave it false.
	CookieSecure bool
	CookieName   string
	Now          func() time.Time
}

// Service is the OIDC relying party and session HTTP surface.
type Service struct {
	cfg      Config
	store    *Store
	auth     *Authenticator
	oauth2   oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// New constructs a Service by discovering the configured issuer.
func New(ctx context.Context, cfg Config) (*Service, error) {
	if cfg.Issuer == "" || cfg.ClientID == "" || cfg.RedirectURL == "" {
		return nil, fmt.Errorf("identity: issuer, client_id, and redirect_url are required")
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	if cfg.CookieName == "" {
		cfg.CookieName = DefaultCookieName
	}
	if cfg.Audience == "" {
		cfg.Audience = cfg.ClientID
	}
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("identity: oidc discovery: %w", err)
	}
	store := NewStore(cfg.SessionTTL, cfg.Now)
	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID},
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.Audience})
	return &Service{
		cfg:      cfg,
		store:    store,
		auth:     NewAuthenticator(store, cfg.CookieName),
		oauth2:   oauth2Cfg,
		verifier: verifier,
	}, nil
}

// Store returns the in-memory session store.
func (s *Service) Store() *Store { return s.store }

// Authenticator returns the cookie session lookup used by the API gate.
func (s *Service) Authenticator() *Authenticator { return s.auth }

// Session implements SessionLookup.
func (s *Service) Session(r *http.Request) (*Session, error) {
	if s == nil || s.auth == nil {
		return nil, ErrUnauthenticated
	}
	return s.auth.Session(r)
}
