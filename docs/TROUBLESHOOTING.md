# Troubleshooting

Disposable local release (`v0.1.0`) only. Not a production runbook.
Quickstart: [getting-started.md](getting-started.md).
Architecture and ports: [architecture/overview.md](architecture/overview.md).
Operations signals: [operations/observability.md](operations/observability.md).
Runbooks: [operations/runbooks/broker-interruption.md](operations/runbooks/broker-interruption.md) etc.
Known limitations: [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md).

## Prerequisites

| Symptom | Fix |
| --- | --- |
| `docker: command not found` or `compose: command not found` | Install Docker Engine + Compose v2. Required for all `go test` (Testcontainers), `compose.yaml`, and `local-stack.sh`. |
| `Release demonstrations require host Python 3.10 or newer.` | Install Python 3.10+ on the host (`python3 --version`). CI uses 3.12. |
| `git: command not found` | Install Git — harness fingerprinting and audit need `git ls-files`/`rev-parse`. |
| `worktree_dirty true` in demo evidence | Commit or stash changes; `git status --porcelain=v1` must be empty for deterministic `source_sha256` comparison. `RUNBOOK_EXERCISE_REPORT.md` is excluded from digest; other dirty files make `stable_identity` non-comparable. |
| `SESHATOPS_TEST_DATABASE_URL` unexpected | Unset or set to a disposable `seshatops_northstar_disposable` only; otherwise `go test` may target a live DB. |

## Quickstart / Compose

| Symptom | Fix |
| --- | --- |
| `./scripts/local-stack.sh: line …: docker: command not found` | See prerequisites above. |
| `compose up --wait` times out after 180s | `docker ps -a`, `./scripts/local-stack.sh logs postgres`, `… logs redpanda`, `… logs runtime`. Check `pg_isready` and `rpk cluster health`. Ensure no port `5173`/`9090` conflict on `127.0.0.1`. Retry: `./scripts/local-stack.sh status` → `down` → `quickstart`. |
| `address already in use 127.0.0.1:5173` or `:9090` | Another process occupies the loopback port. Stop it, or `lsof -i :5173`. The release intentionally binds only to `127.0.0.1`. |
| `postgres is unhealthy` | `./scripts/local-stack.sh logs postgres --tail 100`. Ensure `postgres-data` volume for project `seshatops-local` is not from a foreign project; if corrupted, guarded `reset` (see below). |
| `redpanda is unhealthy` | `./scripts/local-stack.sh logs redpanda --tail 100`. Check `rpk cluster health --api-urls redpanda:9644`. |
| `runtime health check failed …/readyz` | `./scripts/local-stack.sh logs runtime --tail 200`. Common: OIDC at `http://oidc.seshatops.localhost:9090/default` not healthy yet; wait for `oidc` health then `compose up --wait`. `readyz 503` after broker kill is expected until `broker-interruption` runbook recovery (`docker compose --project-name seshatops-local --file compose.yaml start redpanda && docker compose … restart runtime`). |
| Browser `http://web.seshatops.localhost:5173` not reachable | Ensure `hosts` resolves `web.seshatops.localhost` via Compose `aliases` (no manual `/etc/hosts` needed). Try `curl -i http://127.0.0.1:5173/` (`web` forwards). Check `docker compose --project-name seshatops-local ps`. |
| OIDC login loops or `401 unauthorized` | Mock OIDC at `127.0.0.1:9090` must be healthy. Use demo identity `northstar-demo-operator` with any password; real IdP creds are not used. Cookie is `seshatops_session` HttpOnly Lax; `SESHATOPS_COOKIE_SECURE=false` in Compose (http). After `runtime` restart sessions are lost — re-login. |

## Bootstrap / Forecast

| Symptom | Fix |
| --- | --- |
| `go run ./cmd/seshatops bootstrap` hangs | Default wait is `2m` for `platform` consumer checkpoint. Slow host: `SESHATOPS_NORTHSTAR_BOOTSTRAP_TIMEOUT=3m go run ./cmd/seshatops bootstrap`. Inspect `relay`/`consumer` via `local-stack.sh logs runtime`. |
| `forecast_unavailable` / `forecast_timeout` / `invalid_response` JSON + non-zero exit | One-shot `SESHATOPS_FORECAST_PYTHON` unavailable, deadline (`SESHATOPS_FORECAST_TIMEOUT=5s`) with `SESHATOPS_FORECAST_CANDIDATE=/app/scripts/demo-forecast-timeout.py`, or malformed artifact. No `platform.forecast_predictions` row was written (intended). Fix `SESHATOPS_FORECAST_PYTHON=python3` and re-run with `SESHATOPS_FORECAST_CONFIRM=I_UNDERSTAND_FROZEN_M4_FORECAST_WRITE`. See `operations/runbooks/forecast-degradation.md`. |
| `GET /…/forecast/features` returns `insufficient`/`stale`/`incomplete` with `rows:[]` | Honest reporting, not a bug. Live Event Spine is 5 sparse events; it cannot reproduce the 112-day frozen fixture. See `forecast-feature-snapshots.md` and `KNOWN_LIMITATIONS.md`. |
| `GET /…/forecast/predictions/{id}` is `404` or `stale` | `404` before `forecast` on a fresh DB; `stale` after `forecast` when live history differs from frozen checksum — correct. See `forecast-degradation` runbook. |
| `SESHATOPS_DATABASE_URL` refused / wrong DB name | Must be `seshatops_northstar_disposable` on host `postgres` (Compose) or `localhost` for bare runs. `reset-northstar` and `forecast` refuse non-disposable names/hosts. Offending URL example: `postgres://…@localhost:5432/seshatops_northstar_disposable`. |

## Demos / Evidence

| Symptom | Fix |
| --- | --- |
| `GuardError: … SESHATOPS_DEMO_CONFIRM …` or `SESHATOPS_LOCAL_STACK …` | Set exactly `export SESHATOPS_DEMO_CONFIRM=I_UNDERSTAND_DISPOSABLE_LOCAL_DEMO` (and `SESHATOPS_DEMO_FIXTURE_CONFIRM` is set by harness internally). Do not export `DOCKER_HOST`/`DOCKER_CONTEXT` — local Unix socket `unix:///var/run/docker.sock` is required. |
| `InvariantFailure: … http_statuses …` | Read the bounded `*.json`/`*.txt` under `.release-evidence/<ts>/` (gitignored, `256 KiB`/`64 KiB` caps). A harness `--break-expectation <name>` run can verify the harness fails non-zero on broken expectations — not a runtime fault. |
| Deterministic comparison `sha256` mismatch | Keep checkout clean (`git rev-parse HEAD` equal), fixture `northstar-m3-lineage-v1` unchanged, harness `m5-local-release-v1` equal, and compare two runs made on the same host. Timestamps/durations are excluded from identity; counts/checksums must match. |
| `compose down --volumes` refused | The release only allows destructive removal of the `seshatops-local` project via `./scripts/local-stack.sh reset` with confirmation. Never `docker volume prune` or prune foreign projects. |

## Logs / Metrics

| Symptom | Fix |
| --- | --- |
| `fetch("/metrics")` returns `401`/`403` | Caller needs `MX-010` (`ROLE-RELEASE-OBSERVER` on `SCOPE-RUNTIME`) — granted to `northstar-demo-operator` in Compose. Login at `http://web.seshatops.localhost:5173` and fetch same-origin: `fetch("/metrics",{credentials:"same-origin"}).then(r=>r.text())`. Raw `/metrics` via curl needs cookies (`-b /tmp/cookies.txt`). |
| Logs contain no `correlation_id` | Every log line carries `correlation_id` UUID; HTTP lines use generated ID, not caller-supplied headers. If missing, you are reading a non-runtime container. Use `./scripts/local-stack.sh logs runtime --tail 200`. |
| Need bounded tail | `./scripts/local-stack.sh logs runtime --tail 200` / `… logs redpanda --tail 100`. Full-dump is not retained; evidence tails are `≤64 KiB` and redacted. |

## Documentation checks

```bash
python3 scripts/check_runbooks.py
python3 scripts/test_local_stack.py
python3 -m unittest scripts.test_release_demo -v
# then
go test ./... -count=1 -timeout 15m
python3 -m unittest discover -s forecast_candidate -p 'test_*.py' -v
cd web && npm ci && npm run typecheck && npm test && npm run build
```

Markdown/YAML/link checks use the same config as CI (`.markdownlint-cli2.yaml`, `.yamllint.yml`, `.lychee.toml:20s`).

If a symptom above persists after following the smallest safe action, capture a bounded tail (`docker compose --project-name seshatops-local --file compose.yaml ps`, `logs runtime --tail 200`, `GET /metrics` excerpt, `GET /ops` `backlog`/`processing`) redacting fixture markers, and stop — see `docs/operations/runbooks/*` stop conditions.
