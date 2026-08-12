package identity

import "net/http"

// DefaultCookieName is the opaque session cookie. The value is a store key,
// not an identity assertion.
const DefaultCookieName = "seshatops_session"

// DefaultLoginCookieName binds OIDC state to the browser that started login.
const DefaultLoginCookieName = "seshatops_login"

// SessionLookup resolves a Go-owned session from a request. Implementations
// must ignore client-supplied principal, tenant, or role headers.
type SessionLookup interface {
	Session(r *http.Request) (*Session, error)
}

// Authenticator looks up sessions from the opaque session cookie.
type Authenticator struct {
	store      *Store
	cookieName string
}

// NewAuthenticator returns a cookie-backed SessionLookup.
func NewAuthenticator(store *Store, cookieName string) *Authenticator {
	if cookieName == "" {
		cookieName = DefaultCookieName
	}
	return &Authenticator{store: store, cookieName: cookieName}
}

// Session returns the fresh Go-owned session for r, or ErrUnauthenticated.
// Client-supplied principal headers are ignored.
func (a *Authenticator) Session(r *http.Request) (*Session, error) {
	if a == nil || a.store == nil || r == nil {
		return nil, ErrUnauthenticated
	}
	c, err := r.Cookie(a.cookieName)
	if err != nil || c.Value == "" {
		return nil, ErrUnauthenticated
	}
	sess, ok := a.store.Get(c.Value)
	if !ok {
		return nil, ErrUnauthenticated
	}
	return sess, nil
}
