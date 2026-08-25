# SeshatOps

A synthetic ERP for fictional **Northstar Foods** records one-line orders and
M3 supplier-to-order lineage hops through a transactional outbox and Redpanda.
Operators see the inventory projection over REST/SSE after a Go-owned OIDC
session. **Ahoy is excluded** ([CLEAN_ROOM.md](docs/CLEAN_ROOM.md)).

The repository is a **bounded local release** (`v0.1.0`, private until the
release gate): a runnable Go process under `cmd/seshatops`, a TypeScript
console under `web/`, and a disposable Compose stack on `127.0.0.1` only.
Not a production deployment claim.

```text
Browser (Vite/React web/)
        │  REST + SSE + cookies
        ▼
Go (identity/, api/, platform/, relay/, erp/, event/, northstar/, forecast/)
        │
        ├── PostgreSQL   (source, outbox, inbox, projection, audit)
        └── Redpanda     (at-least-once event transport)
```

OIDC sessions are process-local memory. PostgreSQL stores authorization-decision
audit, not sessions.

The process requires database, broker, OIDC, cookie, and listen-address
configuration through `SESHATOPS_*` environment variables. It applies the ERP,
platform, and identity migrations, serves `/auth/*`, `/v1/*`, `/metrics`, and
`/livez`/`/readyz`, and runs the relay and projection consumer. See
[the as-built process topology](docs/architecture/overview.md) for the full
configuration names and lifecycle behavior.

## Implemented capabilities (v0.1)

- One-command disposable stack `./scripts/local-stack.sh quickstart` — builds
  pinned Go+TS images, starts PostgreSQL/Redpanda/mock OIDC, applies
  migrations, creates topic, runs deterministic `northstar-m3-lineage-v1`
  bootstrap and frozen `northstar-m4-stockout-v1` forecast, prints the browser URL
  (`docs/getting-started.md`).
- Real Authorization Code + PKCE, default-deny tenant-scoped allow-list
  (`MX-001`–`MX-010`), audit-before-mutate (`docs/security/authorization.md`).
- At-least-once Event Spine with strict envelope (`event_schema_version 1`),
  transactional outbox, durable quarantine, duplicate-safe inbox, inventory+lineage
  projection, checksum-verified rebuild (`docs/design/specifications/event-spine.md`).
- Bounded `/metrics` + JSON `slog` with redacted, aggregate-only labels
  (`docs/operations/observability.md`).
- Eight guarded demo scenarios + four recovery runbooks exercised on the disposable
  stack (`docs/getting-started.md#release-demonstration-campaign`, `docs/operations/README.md`).

What the tests measured on that stack: [EVIDENCE.md](docs/EVIDENCE.md).
Release notes: [CHANGELOG.md](CHANGELOG.md). Security scope: [SECURITY.md](SECURITY.md).

## Measured local/test evidence

- Single-host Testcontainers/Compose evidence only: Event Spine exit gate, Identity
  HTTP gate, M3 traceability/recovery gate, M4 frozen stockout gate (selects
  `seasonal_naive` baseline — candidate not promoted), M5 forecast runner,
  M5 demos, and runbook exercise (`docs/EVIDENCE.md` + `docs/evaluation/`).
- Durations, counts, checksums, and timestamps are diagnostic observations from
  those runs, not SLOs. Counters reset on restart.
- Hosted CI references reconcile implementation commits vs docs-only follow-ups
  (PR #98 `bad5073` vs merged head `39911bc` — see `docs/EVIDENCE.md`).

## Unsupported production claims

This release **does not** claim: production hosting, public internet exposure,
production TLS, HA/autoscaling, backup/restore, multi-host failover,
disaster recovery, SLOs or capacity, production OIDC revocation/pentest,
forecasting quality or business impact, or complete supply-chain attestation.

## Known limitations and residual risks

- Process-local session store — restart invalidates sessions, no durable
  revocation list. At-least-once transport persists; `duplicate_noop` is expected.
- Sparse live `northstar-m3-lineage-v1` history vs dense frozen M4 fixture:
  live `GET /…/forecast/features` correctly returns `insufficient`/`stale`/`incomplete`
  with empty `rows`; predictions may be `stale`.
- Demo evidence is bounded (`256 KiB` result / `64 KiB` diagnostics),
  localhost-only, and ephemeral (`git describe --dirty` must be clean for deterministic
  comparison). See [Known limitations](docs/KNOWN_LIMITATIONS.md).

## Explicitly deferred and rejected

Deferred (out of scope for `v0.1`): managed cloud deployment, Kubernetes/Terraform,
service mesh, Grafana/dashboards, object storage, production secrets manager.

Rejected: exact-once delivery, deriving tenants from client headers/query/query
body/IdP claim, bypassing the allow-list, injecting sessions, directly editing
finished projections, `psql` surgery in runbooks, or granting
`ROLE-PLATFORM-OPERATOR` the forecast reads.

## Local quickstart

Prerequisites: Docker Engine, Docker Compose v2, Git. Demo/CI paths additionally
require host Python `3.10+`.

```bash
./scripts/local-stack.sh quickstart
# open http://web.seshatops.localhost:5173 as northstar-demo-operator (any password, mock OIDC)
# only same-origin /auth/* and /v1/* (+ /metrics for MX-010) reach Go
```

Lifecycle and verification:

```bash
./scripts/local-stack.sh status
./scripts/local-stack.sh logs [service]
./scripts/local-stack.sh down          # preserves volumes
SESHATOPS_LOCAL_RESET_CONFIRM=I_UNDERSTAND_DISPOSABLE_LOCAL_RESET ./scripts/local-stack.sh reset
./scripts/local-stack.sh smoke         # headless check (build + wait + bootstrap + forecast + restart)
SESHATOPS_DEMO_CONFIRM=I_UNDERSTAND_DISPOSABLE_LOCAL_DEMO ./scripts/local-stack.sh demo all
```

Full prerequisites, configuration, compatibility, troubleshooting, and demo
evidence layout: [Getting started](docs/getting-started.md). Operations and runbooks:
[docs/operations/README.md](docs/operations/README.md). Audit method and findings:
[docs/evaluation/RELEASE_AUDIT_REPORT.md](docs/evaluation/RELEASE_AUDIT_REPORT.md).
Topology: [docs/architecture/overview.md](docs/architecture/overview.md).

## Develop

Toolchain pins (immutable digests in [event-spine.md](docs/design/specifications/event-spine.md) §9):
Go `1.25.0`, Node.js `24.14.0`, npm `11.9.0`, TypeScript `6.0.3`,
PostgreSQL `16.14`, Redpanda `v25.2.1`, mock-oauth2-server `4.0.0`.
Docker is required for Testcontainers-backed Go tests. To reuse a running
Postgres instead, set `SESHATOPS_TEST_DATABASE_URL`.

```bash
go test ./... -count=1 -timeout 15m
cd web && npm ci && npm run typecheck && npm test && npm run build
```

UI: [web/README.md](web/README.md). How we work: [CONTRIBUTING.md](CONTRIBUTING.md).

## Docs

[docs/README.md](docs/README.md) — architecture, contracts, configuration,
operations, compatibility, evidence, security, decisions.

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
