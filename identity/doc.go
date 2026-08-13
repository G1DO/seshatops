// Package identity implements OIDC Authorization Code + PKCE login,
// Go-owned principal/session establishment (Issue #45 / CAP-009), and
// tenant-scoped allow-list evaluation for query APIs (Issues #46–#47 /
// CAP-010 / CAP-012).
//
// Validated IdP assertions establish identity only. They are not tenant
// membership, role, approval, or command authority. Authentication success
// never implies authorization (AUTH-05). Platform assignments plus the
// frozen permission matrix decide inventory projection reads (MX-001) and
// ops-visibility reads (MX-002 / MX-003); missing or unmatched membership
// is deny.
//
// TypeScript may present session state and start login; it cannot authorize.
package identity
