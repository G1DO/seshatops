# ADR-0005: Identity, Tenant Policy, and Service Delegation

- **Status:** Accepted; Identity HTTP implements this for the Northstar demo allow-list
- **Date:** 2026-08-12

## Context

Default deny, tenant isolation, least privilege, and Go-owned authorization
remain required ([authorization.md](../security/authorization.md)). This ADR
freezes the identity profile so runtime work cannot treat client-supplied
tenant or role, or raw IdP claims, as authority.

## Decision

1. Browser operators authenticate through **OpenID Connect Authorization Code Flow with PKCE**. SeshatOps does not implement a custom identity provider.
2. Validated IdP assertions establish identity only — not tenant membership, role, or command authority.
3. Go validates issuer, audience, signature, subject binding, and expiry before establishing a session. Forged, swapped, stale, or contradictory assertions are denied.
4. Authoritative principal and session context is Go-owned. The UI may present session state but cannot authorize.
5. Tenant membership and permissions come from platform policy data, not from client-supplied tenant or role fields.
6. Policy is an **explicit allow-list**. A missing entry is deny. A role name alone is never a grant. The Northstar demo rows are in [authorization.md](../security/authorization.md) and `identity/matrix.go`.
7. Service-identity credentials are not implemented.

IdP vendor selection is configuration, not this ADR.

## Alternatives considered

### Custom identity provider

Rejected; the milestone requires standards-compliant OIDC.

### Treating IdP tenant or role claims as authority

Rejected; authentication is not authorization.

### Policy-engine product or DSL selection now

Rejected as premature. An explicit allow-list is enough for the Northstar demo.

### UI- or Python-owned authorization

Rejected. Go authorizes; the UI does not.
