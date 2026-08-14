# SeshatOps

> Secure Operations Intelligence and Action Control Plane

SeshatOps is a clean-room, multi-tenant operations-intelligence platform for
the fictional **Northstar Foods** scenario. It consumes synthetic ERP events,
reconstructs replayable operational state, and exposes authorized operator
views. Intelligence, human-approved commands, and production deployment are
not built yet.

Public artifacts must remain understandable and runnable without any private
production system. **Ahoy is excluded.** See [CLEAN_ROOM.md](CLEAN_ROOM.md).

## Status

| Capability | Status |
| --- | --- |
| Event Spine | Implemented. Test-environment Observed: `CLM-003`–`CLM-006`. |
| Identity & Operations | Implemented. Test-environment Observed: `CLM-007`–`CLM-010`. `CAP-011` remains Planned. |
| Traceability & Recovery and later | Planned. Not in this repository as runtime. |

There is no deployment binary, hosted environment, or production evidence.
Public wording must not exceed [EVIDENCE.md](EVIDENCE.md).

## What is implemented

A synthetic Northstar order commits with a transactional outbox, a Go relay
publishes the exact bytes to Redpanda, a Go consumer updates a tenant-scoped
inventory projection, and a TypeScript operations view reads that projection
over REST and SSE. Go owns OIDC sessions, default-deny tenant authorization,
ops visibility, authorized quarantine/replay/rebuild, and privileged-decision
audit.

```text
Browser (Vite/React web/)
        │  REST + SSE + cookies
        ▼
Go (identity/, api/, platform/, relay/, erp/, event/, northstar/)
        │
        ├── PostgreSQL   (source, outbox, inbox, projection, audit)
        └── Redpanda     (at-least-once event transport)
```

Packages: `event/`, `northstar/`, `erp/`, `relay/`, `platform/`, `api/`,
`identity/`, `web/`.

## Develop

This repository is libraries plus tests. There is no `cmd/` server to start.

**Toolchain pins** (immutable image digests in [CONTRACTS.md](CONTRACTS.md) §9):
Go `1.25.0`, Node.js `24.14.0`, npm `11.9.0`, PostgreSQL `16.14`, Redpanda
`v25.2.1`. Docker is required for Testcontainers-backed Go tests; those tests
skip if Docker is unavailable.

```bash
go test ./... -count=1 -timeout 15m
cd web && npm ci && npm run typecheck && npm test && npm run build
```

Use `-timeout 25m` for the full Event Spine or Identity exit-gate campaigns.
UI-only work: [web/README.md](web/README.md).

Hosted checks on pull requests: Go CI, Web CI, Documentation CI (Markdown
lint, link check, YAML lint, secret scan).

How we work: [CONTRIBUTING.md](CONTRIBUTING.md). Career loop is Notion product
context → GitHub issue → short-lived branch → PR → `main`.

## Docs

| Doc | Owns |
| --- | --- |
| [ARCHITECTURE.md](ARCHITECTURE.md) | As-built topology and trust boundaries |
| [CONTRACTS.md](CONTRACTS.md) | Event Spine wire, persistence, and toolchain contract |
| [EVENT_MODEL.md](docs/architecture/EVENT_MODEL.md) | Event meaning and correctness invariants |
| [docs/adrs/](docs/adrs/) | Consequential technical decisions |
| [docs/security/](docs/security/) | Threat model, authorization, identity surfaces |
| [CLEAN_ROOM.md](CLEAN_ROOM.md) | Ahoy exclusion and provenance |
| [EVIDENCE.md](EVIDENCE.md) | Claim ledger |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Human GitHub issue → PR → `main` loop |
| [AGENTS.md](AGENTS.md) | Coding-agent contract |

Operating model: Career → Workflow. Notion owns product context and roadmap.
GitHub owns execution. This repository owns implemented technical truth.

## License

Licensed under the [Apache License 2.0](LICENSE).
