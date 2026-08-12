// Package identity implements OIDC Authorization Code + PKCE login and
// Go-owned principal/session establishment (Issue #45 / CAP-009).
//
// Validated IdP assertions establish identity only. They are not tenant
// membership, role, approval, or command authority. Authentication success
// never implies authorization (AUTH-05). Default-deny tenant/resource/action
// enforcement remains Issue #46.
//
// TypeScript may present session state and start login; it cannot authorize.
package identity
