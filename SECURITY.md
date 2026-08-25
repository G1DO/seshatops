# Security policy

Bounded local release only. Not a production security, compliance, or formal-verification claim.

## Supported scope

| Release | Scope | Status |
| --- | --- | --- |
| `v0.1.x` (`main` at or after this hardening) | Disposable local Compose stack (`compose.yaml`), Go runtime `cmd/seshatops`, TypeScript console `web/`, PostgreSQL `16.14`, Redpanda `v25.2.1`, mock OIDC `4.0.0` on `127.0.0.1` only | Supported for local reproduction |
| Prior commits, forks, private deployments | Out of scope for this policy | Not supported |
| Hosted production deployment, on-call, or customer data | Explicitly not offered in `v0.1` | Not supported |

If you are not running the pinned disposable stack from a clean checkout of `main`, this policy does not apply.

## What `v0.1` claims and does not claim

Implemented:

- Default-deny allow-list (`identity/matrix.go`, `MX-001`–`MX-010`). Missing, stale, or contradictory context fails closed.
- Tenant isolation on implemented HTTP — path `{tenant_id}` is an assertion; Go validates before reading projections, ops, lineage, audit, forecast, or metrics.
- Privileged controls (`POST .../ops/quarantine/release`, `.../replay`, `.../rebuild`) require `audit insert before mutate`; `audit_failed` blocks mutation.
- Browser talks only to Go `/auth/*`, `/v1/*`, `/metrics` (same-origin via Vite). PostgreSQL, Redpanda, Python have no host-published ports and no browser path.
- Process-local sessions/assignments, httpOnly `SameSite=Lax` cookies, PKCE `S256` with nonce, audience/issuer/signature validation.

Not claimed:

- Production authentication strength, revocation, session persistence, rotation, or pentest.
- Network isolation beyond `127.0.0.1` Compose bindings and `no-new-privileges`/`cap_drop`.
- Formal verification, SAST completeness, supply-chain attestation beyond pinned digests and recorded audit.
- Regulatory compliance (SOC 2, ISO, GDPR adequacy, HIPAA).

See `docs/security/authorization.md`, `docs/operations/observability.md` (redacted logs/metrics), and `docs/CLEAN_ROOM.md`.

## Reporting a vulnerability

**Do not open a public issue with exploit details.**

1. Use GitHub *Private vulnerability reporting* on `G1DO/seshatops` (preferred), or email the maintainer contact listed under the repository’s `Security` tab. Include: affected commit (`git rev-parse HEAD`), `compose.yaml` fingerprint, steps to reproduce on the disposable stack, and impact scope.
2. We will acknowledge within 3 business days and coordinate a fix. Do not disclose broadly until a fix is merged or 90 days have passed, whichever is first.
3. No bounty is offered for `v0.1`.

If you must use email, encrypt if possible and avoid sending secrets, private identifiers, or production traces. We will not ask for them.

## Secret handling

- No secret is baked into images or committed configuration. `compose.yaml` uses synthetic local-only values (`seshatops-local-only`, `postgres://…`, mock OIDC with no password) — these are disposable test credentials, not secrets (see `docs/CLEAN_ROOM.md` and `docs/getting-started.md`).
- Real credentials must not be committed. The secret scan (`gitleaks detect --source . --log-opts=--all` in CI and in `docs/evaluation/RELEASE_AUDIT_REPORT.md`) runs on full history at `8.24.3`; contributing instructions treat untrusted input as shell-unsafe and require `REPLACE_ME` placeholders.
- If you accidentally commit a secret: stop distribution, remove from working tree, request maintainer-approved history rewrite (`git filter-repo` or BFG) — do **not** rewrite history yourself — redact hosted artifacts, rotate the credential, and re-check `docs/CLEAN_ROOM.md`.

## Dependency and update policy

- **Pins are the policy.** Release images are pinned by digest: PostgreSQL `16.14@sha256:95206741a5b214807675e14165369d05b93a9cf692223b616d07cca227e74b0b`, Redpanda `v25.2.1@sha256:218469e5d088757bb2c3ff4c5e272f7eebdc4e94c933e6e15aff10b845cbcd07`, mock-oauth2-server `4.0.0@sha256:79f51f412caddb1e2120a5ae10d1f203e134f6e8328f1bc63c444acba33c9086`, `golang:1.25.0-bookworm@sha256:81dc45d05a7444ead8c92a389621fafabc8e40f8fd1a19d7e5df14e61e98bc1a`, `python:3.13.7-slim-bookworm@sha256:adafcc17694d715c905b4c7bebd96907a1fd5cf183395f0ebc4d3428bd22d92d`, `node:24.14.0-bookworm-slim@sha256:d8e448a56fc63242f70026718378bd4b00f8c82e78d20eefb199224a4d8e33d8` (`docs/design/specifications/event-spine.md` §9, `compose.yaml`, `docker/*Dockerfile`). Go module and npm lockfile are committed (`go.sum`, `web/package-lock.json`). GitHub Actions are pinned by SHA (`actions/checkout@d23441…` etc., `.github/workflows/*`).
- Updates are applied by explicit PR, not mutable tags. `dependabot` is not auto-merging in `v0.1`; a human reviews license/compat notes before bump. Container bases run as non-root, `read_only: true`, `no-new-privileges:true`, `cap_drop:[ALL]` where practical.
- Until a stable update cadence exists, check `CHANGELOG.md` and the audit report at release tags. No SLA is promised for patch delivery.

## Hardening applied for `v0.1`

- `postgres`, `redpanda`, `redpanda-init`, `runtime`, `web` services use pinned digests; `runtime`/`web`/`redpanda` drop capabilities, use `read_only` + `tmpfs:/tmp` (see `compose.yaml`).
- Browser ↔ Go allow-list is 3-tuple (tenant+role+resource+action); `MX-010` is `SCOPE-RUNTIME` selected by Go, never from caller.
- Logs/metrics are bounded and redacted (no raw bodies, cookies, tokens, feature rows, artifacts — `docs/operations/observability.md`).
- Demo/`demo-fixture` paths require `SESHATOPS_DEMO_CONFIRM`/`SESHATOPS_DEMO_FIXTURE_CONFIRM` and are unreachable from public API; they validate the fixed `seshatops-local` project, local Unix socket, and disposable DB before destructive action (`scripts/release_demo.py`).

## Known residual risks (v0.1)

- Single-host disposable stack only. Durations are diagnostic, not SLOs. At-least-once delivery remains; `duplicate_noop` is expected.
- Process-local session/assignment store — restart invalidates sessions, no durable revocation list.
- No production WAF, rate limiting, or host firewall claim beyond loopback bindings.
- Python subprocess is a one-shot offline producer, not a hardened service.

See `docs/EVIDENCE.md` and `docs/evaluation/RELEASE_AUDIT_REPORT.md` for measured local evidence vs. unsupported production conclusions.
