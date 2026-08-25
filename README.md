# SeshatOps

A synthetic ERP for fictional **Northstar Foods** records one-line orders and
M3 supplier-to-order lineage hops through a transactional outbox and Redpanda.
Operators see the inventory projection over REST/SSE after a Go-owned OIDC
session. **Ahoy is excluded** ([CLEAN_ROOM.md](docs/CLEAN_ROOM.md)).

The repository includes a runnable Go process under `cmd/seshatops` and a
public-safe, disposable local stack, not a claim of production deployment.
What the tests measured: [EVIDENCE.md](docs/EVIDENCE.md).

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
platform, and identity migrations, serves `/auth/*` and `/v1/*`, and runs the
relay and projection consumer with `/livez` and `/readyz` health checks. See
[the as-built process topology](docs/architecture/overview.md) for the full
configuration names and lifecycle behavior.

## Local quickstart

With Docker Engine and Docker Compose v2 available, start the complete
public-safe Northstar stack with one command:

```bash
./scripts/local-stack.sh quickstart
```

The command builds the pinned Go and TypeScript images, starts PostgreSQL,
Redpanda, and mock OIDC, applies migrations, creates the event topic, runs the
deterministic Northstar bootstrap and frozen forecast, and prints the browser
URL. The browser uses real Authorization Code + PKCE login and sends
application traffic only through the Go `/auth/*` and `/v1/*` paths. See
[Getting started](docs/getting-started.md) for status, logs, shutdown, reset,
headless smoke, and guarded release-demonstration commands. The release
campaign is destructive only to the declared disposable local Compose project
and requires a separate explicit confirmation.

## Develop

Toolchain pins (immutable image digests in
[event-spine.md](docs/design/specifications/event-spine.md) §9):
Go `1.25.0`, Node.js `24.14.0`, npm `11.9.0`, PostgreSQL `16.14`, Redpanda
`v25.2.1`. Docker is required for Testcontainers-backed Go tests. To reuse a
running Postgres instead, set `SESHATOPS_TEST_DATABASE_URL`.

```bash
go test ./... -count=1 -timeout 15m
cd web && npm ci && npm run typecheck && npm test && npm run build
```

UI: [web/README.md](web/README.md). How we work: [CONTRIBUTING.md](CONTRIBUTING.md).

## Docs

[docs/README.md](docs/README.md) — architecture, contracts, authorization, ADRs,
evidence.

## License

Apache License 2.0. See [LICENSE](LICENSE).
