# ADR-0005: Identity, Tenant Policy, and Service Delegation

- **Status:** Accepted Identity & Operations design decision; permission matrix (#44), OIDC/session runtime (#45), and default-deny API enforcement (#46) remain pending
- **Date:** 2026-08-12
- **Scope:** Issue #43 resolution of ADR-Q-004 — OIDC integration profile, Go-owned principal/session model, tenant visibility, policy representation, and service-identity delegation boundaries

## Context

Identity & Operations must not invent authentication or authorization architecture while implementing enforcement. Project Constitution already requires default deny, tenant isolation, least privilege, and Go-owned authorization ([AUTHORIZATION_MODEL.md](../security/AUTHORIZATION_MODEL.md), [THREAT_MODEL.md](../security/THREAT_MODEL.md)). ADR-Q-004 remained open for the concrete identity profile, session ownership, tenant visibility, policy representation shape, and service-delegation boundaries.

This ADR freezes those decisions so Issue #44 can publish a permission matrix and Issue #45 can integrate OIDC login and session establishment without selecting a custom IdP, treating client-supplied tenant or role as authority, or promoting runtime claims. Default-deny API enforcement remains Issue #46.

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
4. Missing, expired, revoked, or inconsistent session context defaults to deny. Authentication success never implies authorization (AUTH-05).

### Tenant visibility and policy representation

1. Tenant visibility for this milestone means tenant-scoped allow-list entries plus platform-owned tenant membership. Principals see and act only within tenants assigned by platform policy; IdP claims and client-supplied tenant fields are assertions to validate, not visibility authority.
2. Authorization continues to use the conceptual tuple in AUTHORIZATION_MODEL §4: tenant, principal, resource type, resource identity, action, scope or contextual constraints, current resource state, relevant policy and assignment version, and freshness where required.
3. Policy for Identity & Operations is an **explicit allow-list**. A missing entry is deny. Roles are labels bound to tenant, resource, and action; a role name alone is never sufficient.
4. No policy-engine product or policy DSL is selected here. Issue #44 owns the frozen Northstar demo permission matrix mapped to the tuple and stable matrix identifiers.

### Service identity and delegation

1. Service identities are distinct technical principals with least-privilege capabilities. Conceptual responsibilities include source/outbox, outbox relay, projection consumer, and ops API. They are not user substitutes.
2. Delegated actions preserve initiating principal, tenant, calling service identity, resource, action, scope, and lineage (AUTH-12, AUTH-13). Silent cross-tenant service delegation is forbidden.
3. Python has no write, command, or authorization authority. Browser code is not an authoritative enforcement point. Compromise of one service identity must not imply unrestricted access to other services, tenants, or responsibilities.
4. Concrete credentials, database roles, secret storage, and rotation remain deferred. This ADR does not introduce runtime scaffolding.

### Non-goals for this decision

1. This ADR does not implement OIDC middleware, login UI, cookies, routes, permission-matrix content, default-deny API enforcement, quarantine or replay operator controls, forecasting, RAG, approvals, backup/restore, or any demo bypass of default-deny.
2. Accepting this ADR alone does not promote any `EVIDENCE.md` claim.

## Consequences

### Benefits

- Issues #44 and #45 can implement against a fixed profile instead of inventing auth architecture mid-flight.
- Default-deny and Go ownership remain explicit before enforcement code exists.
- Service-identity and delegation rules are available for later CAP-011 design without selecting products.

### Costs and trade-offs

- Runtime work still needs configurable IdP settings, session storage, and negative tests.
- An allow-list matrix must be maintained and reviewed; missing entries deny access by design.
- Vendor-neutral protocol choice delays product-specific operational guidance until configuration time.

## Alternatives considered

### Custom identity provider

Rejected because the milestone requires standards-compliant OIDC and forbids a home-grown IdP.

### Treating IdP tenant or role claims as authority

Rejected because AUTH-05 separates authentication from authorization, and client- or provider-supplied tenant or role fields are assertions to validate against platform policy.

### Policy-engine product or DSL selection now

Rejected as premature product selection. The conceptual tuple plus an explicit allow-list is enough for the Northstar demo matrix.

### UI- or Python-owned authorization

Rejected by language ownership and AUTH-03, AUTH-04, and AUTH-08.

### Implementing enforcement in this ADR

Rejected because Issue #43 owns reviewed boundaries only. Runtime belongs to later Identity & Operations issues.

## Verification route and limitations

Documentation review must confirm:

- ADR-Q-004 disposition points to this ADR;
- OIDC profile, session model, tenant visibility, policy allow-list representation, and service-identity boundaries are explicit;
- Issue #44 (matrix), Issue #45 (OIDC/session runtime), and Issue #46 (default-deny enforcement) ownership remain clear;
- no runtime claim promotion appears in `EVIDENCE.md` from this change alone.

This ADR does not claim that authentication, authorization, tenant isolation enforcement, service-identity controls, or audit protection currently exist. CAP-009–CAP-013 remain Planned until later implementation and evidence.
