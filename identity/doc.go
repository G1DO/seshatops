// Package identity implements OIDC Authorization Code + PKCE login,
// Go-owned principal/session establishment (Issue #45 / CAP-009), and
// tenant-scoped allow-list evaluation for query APIs (Issue #46 / CAP-010).
//
// Validated IdP assertions establish identity only. They are not tenant
// membership, role, approval, or command authority. Authentication success
// never implies authorization (AUTH-05). Platform assignments plus the
// frozen permission matrix decide inventory projection reads; missing or
// unmatched membership is deny.
//
// TypeScript may present session state and start login; it cannot authorize.
package identity
