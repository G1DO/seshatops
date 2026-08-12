package identity

import (
	"context"
	"time"
)

type sessionContextKey struct{}

// Session is the Go-owned principal/session context established after a
// validated OIDC assertion. Tenant and role fields are intentionally absent:
// IdP claims and client-supplied tenant or principal headers are not authority.
type Session struct {
	ID              string
	PrincipalID     string
	Issuer          string
	Subject         string
	AuthenticatedAt time.Time
	ExpiresAt       time.Time
	CorrelationID   string
}

// Fresh reports whether the session is still within its expiry.
func (s *Session) Fresh(now time.Time) bool {
	if s == nil {
		return false
	}
	if s.PrincipalID == "" || s.Subject == "" || s.Issuer == "" {
		return false
	}
	return now.Before(s.ExpiresAt)
}

// WithSession returns a child context carrying sess.
func WithSession(ctx context.Context, sess *Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, sess)
}

// FromContext returns the Go-owned session if one was established on ctx.
func FromContext(ctx context.Context) (*Session, bool) {
	sess, ok := ctx.Value(sessionContextKey{}).(*Session)
	if !ok || sess == nil {
		return nil, false
	}
	return sess, true
}
