# Compatibility and toolchain

Pinned toolchain for the bounded `v0.1.0` local release. No production platform is claimed.
Prerequisites and lifecycle: [getting-started.md](getting-started.md).

## Host prerequisites

- Docker Engine + Docker Compose v2 (required for `go test` Testcontainers, `compose.yaml`, and `./scripts/local-stack.sh`).
- Git (required for `release_demo.source_digest`, demo deterministic identities, and audit).
- Host Python `3.10+` for `./scripts/local-stack.sh demo` and `forecast_candidate` validation. CI uses `3.12`.
- One host, `127.0.0.1` only. No cloud, Kubernetes, or external broker.

## Pinned versions (v0.1.0)

All digests are immutable; tags are not trusted.

| Component | Version | Digest / pin | Source |
| --- | --- | --- | --- |
| Go | `1.25.0` | `go.mod:3`, `go 1.25.0` | `go.mod` |
| Node.js | `24.14.0` | `node:24.14.0-bookworm-slim@sha256:d8e448…` | `docker/web.Dockerfile`, `web/package.json:engines` |
| npm | `11.9.0` | `web/package.json:engines` | `web/package-lock.json` |
| TypeScript | `6.0.3` | `typescript@6.0.3` | `web/package.json` |
| PostgreSQL | `16.14` | `postgres@sha256:95206741a5b214807675e14165369d05b93a9cf692223b616d07cca227e74b0b` | `compose.yaml`, `erp.PostgresImage`, `event-spine.md` §9 |
| Redpanda | `v25.2.1` | `docker.redpanda.com/redpandadata/redpanda@sha256:218469e5d088757bb2c3ff4c5e272f7eebdc4e94c933e6e15aff10b845cbcd07` | `compose.yaml`, `relay.RedpandaImage`, `event-spine.md` §9 |
| mock-oauth2-server | `4.0.0` | `ghcr.io/navikt/mock-oauth2-server@sha256:79f51f412caddb1e2120a5ae10d1f203e134f6e8328f1bc63c444acba33c9086` | `compose.yaml` |
| Go image | `golang:1.25.0-bookworm` | `@sha256:81dc45d05a7444ead8c92a389621fafabc8e40f8fd1a19d7e5df14e61e98bc1a` | `docker/go.Dockerfile` |
| Python image | `3.13.7-slim-bookworm` | `@sha256:adafcc17694d715c905b4c7bebd96907a1fd5cf183395f0ebc4d3428bd22d92d` | `docker/go.Dockerfile` |
| GitHub Actions | pinned SHA | `actions/checkout@d23441…` (v6.1.0), `setup-go@d35c59…` (v5.5.0), `setup-node@49933e…` (v4.4.0), `setup-python@a26af69…` (v5.6.0), `markdownlint-cli2@21c1be…` (v24.2.0), `lychee@e74777…` (v2.9.0), `yamllint@257637…` (v3.1.1), `gitleaks@8.24.3` | `.github/workflows/*` |

Installing anything other than the above versions is not required to reproduce `v0.1.0`. Newer hosts (e.g. Go `1.26`, Node `24.18`) are not claimed compatible; use the declared pins via `go`/`nvm` when reproducing exact evidence.

## How to verify your checkout matches the release

From a clean checkout of `main`:

```bash
go version # want go1.25.0
node --version # want v24.14.0
npm --version  # want 11.9.0
docker --version; docker compose version
python3 --version # want 3.10+ (harness guards this)
git rev-parse HEAD && git status --porcelain=v1 # want clean
cat compose.yaml | grep -E 'image:.*@sha256'
python3 scripts/test_local_stack.py
python3 -m unittest scripts.test_release_demo -v
```

The demo harness records `version`, `commit`, `worktree_dirty`, and `source_sha256` (bounded `64 MiB` over `git ls-files` excluding `RUNBOOK_EXERCISE_REPORT.md`) in every `*.json` result; two clean runs on the same commit compare equal under `stable_identity` (durations excluded).

## Compatibility boundaries

- `v0.1.0` has no prior compatibility promise. Future releases will note breaking changes in `CHANGELOG.md`.
- PostgreSQL and Redpanda persistent volumes are `postgres-data`/`redpanda-data` scoped to Compose project `seshatops-local`; `down` preserves them, `reset --volumes` drops only that project (guarded by `SESHATOPS_LOCAL_RESET_CONFIRM`).
- Browser must be on the same host as `web.seshatops.localhost:5173`; Vite proxies only `/auth/*`, `/v1/*`, `/metrics` to Go. No host ports for `postgres`/`redpanda`/`runtime`.
- Forecast Python is `python3` in Compose; host `SESHATOPS_FORECAST_PYTHON` may override locally, but bare `go run ./cmd/seshatops forecast` requires a disposable `seshatops_northstar_disposable` DB.

## Upgrade and reset

No live upgrade. To adopt a new tag/commit:

```bash
git fetch origin && git checkout <new-tag>
SESHATOPS_LOCAL_RESET_CONFIRM=I_UNDERSTAND_DISPOSABLE_LOCAL_RESET ./scripts/local-stack.sh reset
./scripts/local-stack.sh quickstart
```

Second `bootstrap`/`forecast` on the same Northstar fixture is idempotent (`prediction_id` stable).

## Limitations

Single-host disposable Compose only. Durations are diagnostic, not SLOs. At-least-once transport persists. See [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md) and [EVIDENCE.md](EVIDENCE.md).
