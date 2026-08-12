# Identity Boundaries - SeshatOps

**Status:** Accepted design activation for Issue #43 / ADR-0005. This note records
identity boundaries so later Identity & Operations work does not invent
architecture mid-flight. It does not claim that authentication, authorization,
tenant isolation enforcement, service-identity controls, or audit protection
currently exist.

**Owns:** Short activation summary of the OIDC profile, Go-owned session model,
service-identity boundaries, and explicit non-goals for this milestone slice.

**Does not own:** Permission-matrix content (Issue #44), OIDC login or session
runtime (Issue #45), default-deny API enforcement (Issue #46), operational
visibility or quarantine/replay controls, or any `EVIDENCE.md` claim promotion.

## Accepted profile summary

Full decisions are in [ADR-0005](../adrs/0005-identity-tenant-policy-and-service-delegation.md).

| Area | Boundary |
| --- | --- |
| Authentication | Standards-compliant OIDC Authorization Code + PKCE; no custom IdP |
| IdP trust | Partially trusted assertions establish identity only; not tenant or business authority |
| Session ownership | Go establishes and revalidates principal/session context; UI is not authoritative |
| Tenant visibility | Platform-owned tenant membership plus tenant-scoped allow-list; IdP/client tenant fields are not visibility authority |
| Authorization data | Explicit allow-list policy mapped to the AUTHORIZATION_MODEL tuple; missing entry = deny |
| Service identity | Distinct least-privilege technical principals; delegation preserves initiating context |
| Language roles | Go owns authz decisions; TypeScript presents; Python remains non-authoritative |

## What later issues own

| Issue | Owns next |
| --- | --- |
| [#44](https://github.com/G1DO/seshatops/issues/44) | Frozen Northstar demo tenant/role/resource/action permission matrix |
| [#45](https://github.com/G1DO/seshatops/issues/45) | OIDC login integration and Go-owned session establishment runtime |
| [#46](https://github.com/G1DO/seshatops/issues/46) | Default-deny API enforcement |
| Later Identity & Operations issues | Privileged-ops authz, audit, operational visibility, quarantine/replay |

## Non-goals for Issue #43

- Runtime auth middleware, login UI, cookies, or routes
- Permission-matrix rows or role catalog content
- Forecasting, RAG, approval or command execution
- Backup/restore product work
- Demo bypass of default-deny
- Promoting Planned claims in `EVIDENCE.md`

## Related documents

- [ADR-0005](../adrs/0005-identity-tenant-policy-and-service-delegation.md)
- [AUTHORIZATION_MODEL.md](AUTHORIZATION_MODEL.md)
- [THREAT_MODEL.md](THREAT_MODEL.md)
