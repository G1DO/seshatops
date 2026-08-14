# ADR-0005: Identity, Tenant Policy, and Service Delegation

- **Status:** Accepted; Identity HTTP implements this for the Northstar demo allow-list
- **Date:** 2026-08-12
- **Scope:** OIDC profile, Go-owned session, tenant visibility, policy representation, service-identity boundaries

## Context

Default deny, tenant isolation, least privilege, and Go-owned authorization remain required ([AUTHORIZATION.md](../security/AUTHORIZATION.md), [THREAT_MODEL.md](../security/THREAT_MODEL.md)). This ADR freezes the identity profile so runtime work cannot treat client-supplied tenant or role, or raw IdP claims, as authority.

## Decision

### OIDC integration profile

1. Browser operators authenticate through standards-compliant **OpenID Connect Authorization Code Flow with PKCE**. SeshatOps does not implement a custom identity provider.
2. The external IdP is partially trusted: validated assertions establish identity, not tenant membership, role authority, approval authority, or command authority.
3. Go validates issuer, audience, signature, subject binding, and expiry before establishing principal/session context. Forged, swapped, stale, or contradictory assertions are rejected and default to deny.
4. IdP vendor or product selection is deferred to configuration time. This ADR locks the protocol profile and trust boundary only.

### Go-owned principal and session context

1. Authoritative principal and session context is established and revalidated by Go from trusted server-side inputs. TypeScript/UI may present session state but cannot authorize.
2. Minimum conceptual session fields: authenticated principal identifier and subject binding, authentication time, session freshness/expiry (and revocation when available), and correlation lineage.
3. Tenant membership and role or permission assignment come from platform policy data, not from client-supplied tenant or role fields and not from raw IdP claims treated as authority.
4. Missing, expired, revoked, or inconsistent session context defaults to deny. Authentication success never implies authorization.

### Tenant visibility and policy representation

1. Tenant visibility for this milestone means tenant-scoped allow-list entries plus platform-owned tenant membership. Principals see and act only within tenants assigned by platform policy; IdP claims and client-supplied tenant fields are assertions to validate, not visibility authority.
2. Authorization uses tenant, principal, resource, action, and freshness. A role name alone is not a grant.
3. Policy for Identity & Operations is an **explicit allow-list**. A missing entry is deny. Roles are labels bound to tenant, resource, and action; a role name alone is never sufficient.
4. No policy-engine product or policy DSL is selected here. The frozen Northstar demo allow-list and `MX-*` identifiers are in [AUTHORIZATION.md](../security/AUTHORIZATION.md) and `identity/matrix.go`.

### Service identity and delegation

1. Service identities are distinct technical principals with least-privilege capabilities. Conceptual responsibilities include source/outbox, outbox relay, projection consumer, and ops API. They are not user substitutes.
2. Delegated actions preserve initiating principal, tenant, calling service identity, resource, action, scope, and lineage. Silent cross-tenant service delegation is forbidden.
3. Python has no write, command, or authorization authority. Browser code is not an authoritative enforcement point. Compromise of one service identity must not imply unrestricted access to other services, tenants, or responsibilities.
4. Concrete credentials, database roles, secret storage, and rotation remain deferred.

Runtime for the user HTTP path lives in `identity/` and authorized `api/` routes. This ADR does not introduce a deployment binary.

## Consequences

### Benefits

- Runtime work implements a fixed profile instead of inventing auth architecture mid-flight.
- Default-deny and Go ownership remain explicit.
- Service-identity and delegation rules are recorded without selecting products. Service credentials are not implemented.

### Costs and trade-offs

- Production IdP configuration, durable session storage, and pentest remain open.
- An allow-list matrix must be maintained and reviewed; missing entries deny access by design.
- Vendor-neutral protocol choice delays product-specific operational guidance until configuration time.

## Alternatives considered

### Custom identity provider

Rejected because the milestone requires standards-compliant OIDC and forbids a home-grown IdP.

### Treating IdP tenant or role claims as authority

Rejected because authentication is not authorization, and client- or provider-supplied tenant or role fields are assertions to validate against platform policy.

### Policy-engine product or DSL selection now

Rejected as premature product selection. The conceptual tuple plus an explicit allow-list is enough for the Northstar demo matrix.

### UI- or Python-owned authorization

Rejected by language ownership: Go authorizes; the UI and Python do not.

### Implementing enforcement in this ADR

Rejected because this ADR records boundaries. Runtime lives in `identity/` and authorized HTTP.

## Implemented HTTP

[AUTHORIZATION.md](../security/AUTHORIZATION.md).
Test-environment evidence: [IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md](../evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md).
