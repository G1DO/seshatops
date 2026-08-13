// Package identity implements OIDC Authorization Code + PKCE login,
// Go-owned principal/session establishment (Issue #45 / CAP-009), and
// tenant-scoped allow-list evaluation for query APIs and privileged
// controls (Issues #46–#48 / CAP-010 / CAP-012 / CAP-013), and append-only
// privileged authorization-decision audit (Issue #49).
//
// Validated IdP assertions establish identity only. They are not tenant
// membership, role, approval, or command authority. Authentication success
// never implies authorization (AUTH-05). Platform assignments plus the
// frozen permission matrix decide inventory projection reads (MX-001),
// ops-visibility reads (MX-002 / MX-003), privileged controls
// (MX-004 / MX-005 / MX-006), and audit read (MX-007); missing or unmatched
// membership is deny. Audit actor is the Go-owned session principal.
//
// TypeScript may present session state and start login; it cannot authorize.
package identity
