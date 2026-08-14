# SeshatOps

A synthetic ERP order for fictional **Northstar Foods** flows through a
transactional outbox and Redpanda into a tenant-scoped inventory projection.
Operators see that projection over REST/SSE after a Go-owned OIDC session.
**Ahoy is excluded** ([CLEAN_ROOM.md](docs/CLEAN_ROOM.md)).

Libraries plus tests. There is no deployment binary or production environment.
What the tests measured: [EVIDENCE.md](docs/EVIDENCE.md).

```text
Browser (Vite/React web/)
        │  REST + SSE + cookies
        ▼
Go (identity/, api/, platform/, relay/, erp/, event/, northstar/)
        │
        ├── PostgreSQL   (source, outbox, inbox, projection, audit)
        └── Redpanda     (at-least-once event transport)
```

OIDC sessions are process-local memory. PostgreSQL stores authorization-decision
audit, not sessions.

## Develop

Toolchain pins (immutable image digests in [CONTRACTS.md](docs/CONTRACTS.md) §9):
Go `1.25.0`, Node.js `24.14.0`, npm `11.9.0`, PostgreSQL `16.14`, Redpanda
`v25.2.1`. Docker is required for Testcontainers-backed Go tests. To reuse a
running Postgres instead, set `SESHATOPS_TEST_DATABASE_URL`.

```bash
go test ./... -count=1 -timeout 15m
cd web && npm ci && npm run typecheck && npm test && npm run build
```

UI: [web/README.md](web/README.md). How we work: [CONTRIBUTING.md](CONTRIBUTING.md).

## Docs

- [Architecture](docs/architecture/README.md) — as-built topology
- [CONTRACTS.md](docs/CONTRACTS.md) — event wire, checksum, toolchain pins
- [docs/architecture/openapi-projection.yaml](docs/architecture/openapi-projection.yaml) — HTTP/SSE
- [docs/security/AUTHORIZATION.md](docs/security/AUTHORIZATION.md) — default-deny HTTP
- [docs/adrs/](docs/adrs/) — why those decisions
- [CLEAN_ROOM.md](docs/CLEAN_ROOM.md) — Ahoy exclusion

## License

Apache License 2.0. See [LICENSE](LICENSE).
