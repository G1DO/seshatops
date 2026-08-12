# Identity & Operations OIDC session note

Issue #45 added OIDC Authorization Code + PKCE login and Go-owned session
establishment. Unauthenticated callers cannot read projection REST or SSE
data. Authentication is not authorization.

This note records documentation and test review only. It does not promote
`CAP-009`, `CLM-007`, or `CLM-008`, and it does not claim tenant isolation or
default-deny policy enforcement.

## Acceptance

| Criterion | Disposition |
| --- | --- |
| Unauthenticated callers cannot obtain protected projection/ops data | `/v1` REST and SSE return `401 unauthenticated` before any body or stream |
| Authenticated principal and session freshness are available to Go | Opaque session cookie; `identity.FromContext` / `GET /auth/session` |
| Docs state auth ≠ authorization | [OIDC_SESSION.md](../security/OIDC_SESSION.md) |
| Negative tests cover missing/expired/forged session assertions | `identity` mock-OIDC tests and `api` gate tests; unbound callback `state` is rejected |

## Non-goals confirmed

No permission-matrix evaluation, tenant-scoped default-deny (#46), custom
IdP, demo auth bypass, durable session table, deployment binary, or
`EVIDENCE.md` claim change.
