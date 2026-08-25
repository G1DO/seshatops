# Release audit report — v0.1.0

Private-repository audit before public exposure. No private denylist is included;
findings are recorded as safe classifications only. This is `v0.1.0` disposable
local evidence, not production attestation.

- **Date (UTC):** 2026-08-25
- **Repository commit:** `8611b26` → now `feature/106-public-release-hardening` (HEAD at audit start `8611b26`, `git rev-parse HEAD`)
- **Harness / spec versions:** `m5-local-release-v1`, `northstar-m3-lineage-v1`, `northstar-m4-stockout-v1` / `m4-stockout-eval-v1`, `m4-raw-onhand-v1`
- **Tools at audit time (host):** Go `1.26.2` (pinned `1.25.0`), Node `v24.18.0` (pinned `24.14.0`), npm `11.16.0` (pinned `11.9.0`), Python `3.12.3`, Docker `29.4.2` / Compose `v5.1.3`, `gitleaks 8.24.3` via `ghcr.io/gitleaks/gitleaks:v8.24.3`

## Method

Each scope used the smallest reviewable command that reproduces from a clean checkout.
No history was rewritten; the prior `bad5073`/`39911bc` reconciliation preserves code vs docs distinction.
Generated artifacts were inspected, not executed, until the pinned manifest matched.

| Scope | Exact command(s) / tool | What was inspected |
| --- | --- | --- |
| **Ahoy / private-system provenance** | `grep -R -n -i "ahoy" --include="*.md" --include="*.go" --include="*.py" --include="*.yaml" --include="*.yml" --include="*.json" .` (excl. `node_modules`); `git log --all -p --full-history -- . \| grep -i -C2 "ahoy\|private.*identifier"`; `git ls-files -co --exclude-standard \| xargs file`; `ls -la northstar/testdata/ event/testdata/ forecast/testdata/` | Every reachable text file, patch history, goldens, fixtures, screenshots, config, docs |
| **Secrets / credentials / private identifiers / transcripts** | `docker run --rm -v $(pwd):/repo ghcr.io/gitleaks/gitleaks:v8.24.3 detect --source /repo --log-opts="--all" --verbose` (132 commits) and `--no-git` (workspace); `grep -R -n "seshatops-local-only\|REPLACE_ME"`; manual review of `compose.yaml:POSTGRES_PASSWORD`, `SESHATOPS_*`, `docker/oidc/config.json` | Full history + workspace, committed Tx fixtures, OIDC config, env examples |
| **Generated dependency licenses** | `go list -m all \| sort`; `grep -l "postgres@sha256\|redpanda@sha256\|mock-oauth2-server@sha256\|golang:.*@sha256\|node:.*@sha256\|python:.*@sha256"`; `npm audit --json` (web); `grep -E '"license"' web/package-lock.json`; manual `LICENSE` read | Go module (`go.sum`), npm lock (`web/package-lock.json:211 deps`), container bases |
| **Mutable / unpinned release dependencies & container bases** | `grep -R "FROM.*:" docker/go.Dockerfile docker/web.Dockerfile compose.yaml`; `grep -R "image:.*@sha256" compose.yaml`; check `.github/workflows/*.yml` for `actions/*@sha` pin | `compose.yaml` services, Dockerfiles, workflows |
| **Stale / contradictory / overstated claims** | `grep -R -n "production.*claim\|SLO\|pentest\|compliance\|capacity\|exactly-once\|availability" docs/ README.md`; diff current `README.md`/`docs/EVIDENCE.md` vs `AGENTS.md` invariants (`default-deny`, tenant isolation, audit-before-mutate, at-least-once) ; `lychee --config .lychee.toml`, `markdownlint-cli2`, `yamllint` | README, docs, evidence, runbooks, specs |

## Findings

### Ahoy / private-system provenance

| Item | Result |
| --- | --- |
| Repository history, worktree, `northstar/testdata/*.jcs`, `event/testdata/*.jcs`, `forecast/testdata/dataset.sha256`, docs, configs | **No Ahoy code, schema, dataset, log, screenshot, or identifier.** Only permitted mentions are the exclusion notice (`README.md:6`, `docs/CLEAN_ROOM.md:4,11`, `docs/architecture/overview.md:33`, `AGENTS.md:23`) — `grep -i ahoy` returned exactly those 5 lines. |
| Host Docker images `ahoy-*` (`ahoy-app/orderd:0.1.0`, `ahoy-backend:*` etc.) | Present on the *host* (`docker images`), **not** in `git ls-files`, `compose.yaml`, Dockerfiles, or `go.mod`. Out of repository scope; no code or schema was copied. |
| Fixture provenance | `northstar-m3-lineage-v1`, `northstar-m4-stockout-v1`, `northstar-m5-poison-v1` are synthetic Northstar Foods, seeded as documented in `docs/CLEAN_ROOM.md:36-48` and reproduced by `go test ./event ./northstar ./forecast`. Goldens are `*.jcs` JSON + `*.sha256` — no private structure. |
| Screenshots, recordings, exports | None committed (`git ls-files \| grep -E "\.png\|\.jpg\|\.mp4"` empty). |

### Secrets, tokens, credentials, private transcripts

| Item | Result |
| --- | --- |
| `gitleaks detect --log-opts="--all" --verbose` | **No leaks found** — 132 commits, 3.06 MB scanned, workspace 2.14 MB. Tool `ghcr.io/gitleaks/gitleaks:v8.24.3` matches CI `gitleaks-action@e0c47…` `GITLEAKS_VERSION 8.24.3`. |
| `--no-git` (current worktree including new `SECURITY.md`, `CHANGELOG.md`, `KNOWN_LIMITATIONS.md`, `TROUBLESHOOTING.md`) | **No leaks found** (566 ms). |
| `compose.yaml` credentials | `POSTGRES_PASSWORD: seshatops-local-only` and `SESHATOPS_DATABASE_URL: postgres://…seshatops-local-only…` — **synthetic local-only**, `127.0.0.1`-only Compose, `read_only: true`, `no-new-privileges:true`, no host port for PG/Redpanda/Go. Classified safe, not a secret. |
| OIDC config (`docker/oidc/config.json`) | Mock OIDC `4.0.0` with synthetic `northstar-demo-operator → {sub, aud:["seshatops-local"]}`. No real issuer, client secret, or private transcript. |
| `REPLACE_ME` placeholders | Bare `go run ./cmd/seshatops bootstrap/forecast` examples use `postgres://…REPLACE_ME…` (`docs/getting-started.md:215,241`, `docs/CLEAN_ROOM.md:58`) — placeholder only. |
| Private transcripts / identifiers | No `ghp_`, `AKIA`, `Bearer`, or tenant-overlap beyond `11111111-…`/`22222222-…` (fictional Northstar). |

### Generated dependency licenses and obligations

| Ecosystem | Result |
| --- | --- |
| Go (`go list -m all`, `go.sum`) | `go list -m all` lowercased shows **no `gpl/agpl/lgpl`**. Top-level: `pgx/v5`, `franz-go`, `testcontainers-go`, `go-oidc`, `oauth2` — all `MIT`/`BSD-3`/`Apache-2.0`. Full list 90 modules; no incompatible copyleft for an Apache-2.0 repo. |
| npm (`web/package-lock.json`, `npm audit --json`) | **0 vulnerabilities** (prod 4, dev 208, total 211). License histogram: `Apache-2.0:5`, `BSD-2/3:4`, `ISC:6`, `MIT:192`, `CC-BY-4.0:1` (`@testing-library` types), `MIT-0:1`. No GPL in `web/package-lock.json`. |
| Python | No `forecast_candidate/requirements.txt` — offline producer imports stdlib only; no redistributed wheel. |
| Container bases | `postgres:16.14`, `redpanda:v25.2.1`, `mock-oauth2-server:4.0.0` are runtime images, not vendored source; their upstream licenses (PostgreSQL, Apache-2.0 for Redpanda community) permit disposable local use. Not redistributed as source; no source-offer trigger for `v0.1` local run. |

### Mutable / unpinned release dependencies and container bases

| Asset | Pin | Status |
| --- | --- | --- |
| `compose.yaml:postgres` | `postgres@sha256:95206741a5b214807675e14165369d05b93a9cf692223b616d07cca227e74b0b` | Pinned |
| `compose.yaml:redpanda`, `redpanda-init` | `docker.redpanda.com/redpandadata/redpanda@sha256:218469e5d088757bb2c3ff4c5e272f7eebdc4e94c933e6e15aff10b845cbcd07` | Pinned (both) |
| `compose.yaml:oidc` | `ghcr.io/navikt/mock-oauth2-server@sha256:79f51f412caddb1e2120a5ae10d1f203e134f6e8328f1bc63c444acba33c9086` | Pinned |
| `docker/go.Dockerfile` | `golang:1.25.0-bookworm@sha256:81dc45…` + `python:3.13.7-slim-bookworm@sha256:adafcc…` | Pinned |
| `docker/web.Dockerfile` | `node:24.14.0-bookworm-slim@sha256:d8e448…` | Pinned |
| `go.mod` | `go 1.25.0`, `go.sum` committed | Pinned |
| `web/package.json` | `engines: node 24.14.0, npm 11.9.0`, `typescript 6.0.3`, `package-lock.json` committed | Pinned |
| Workflows | `actions/checkout@d23441…`, `setup-go@d35c59…`, `setup-node@49933…`, `setup-python@a26af69…`, `markdownlint-cli2@21c1be…`, `lychee@e74777…`, `yamllint@257637…`, `gitleaks@e0c47…` | Pinned by SHA |

Residual note: host has Go `1.26.2` / Node `24.18` (newer than pinned) — not a source pin violation; CI enforces `1.25.0`/`24.14.0`. `forecast_candidate` has no lockfile — acceptable for offline one-shot (stdlib only).

### Stale, contradictory, or overstated claims

| Item | Result |
| --- | --- |
| Production / SLO / pentest / capacity | Repo now separates buckets (`README.md` *Implemented / Measured / Unsupported / Known limitations / Deferred*). `EVIDENCE.md`, `KNOWN_LIMITATIONS.md`, each runbook header, `observability.md`, and exercise reports all state **not production / not SLO / not pentest / at-least-once not exactly-once**. `grep` for production claims outside those buckets returned only the new guard sentences. |
| PR #98 reconciliation | Prior M4 wording calling `bad5073` final head was corrected: `docs/EVIDENCE.md:86-91` and `docs/evaluation/M4_STOCKOUT_INTELLIGENCE_EXIT_GATE_EXPERIMENT_REPORT.md:82-88` now state implementation commit `bad5073` vs merged docs-only head `39911bc` with exact CI runs (`32181303886` Go, `32181304013` Web, `32181303898/job/95854713070` docs, failed `32180186848/job/95851340432` on `8b53501`). Implementation vs docs-only preserved. |
| Paths / commands | `scripts/check_runbooks.py` validates every `seshatops_*` metric, `/v1/*`, `/metrics`, `local-stack.sh` subcommand, and `go run ./cmd/seshatops` referenced in runbooks exists in `observability.go` / `openapi-projection.yaml` / `local-stack.sh` / `cmd/seshatops/main.go`. `python3 scripts/test_local_stack.py` validates rendered Compose. Both passed during this audit (run as part of doc checks). |
| No private denylist | This report lists classifications, not raw secrets/identifiers. |

## Dispositions

| Finding | Disposition |
| --- | --- |
| Host `ahoy-*` images | **No repository action.** Not committed; `docs/CLEAN_ROOM.md` remains the boundary. Contributors must not inspect/copy them; CI never pulls them. |
| Synthetic `seshatops-local-only` password | **Accepted as test-only**; documented in `SECURITY.md:46`, `docs/CLEAN_ROOM.md:58`, `docs/getting-started.md:39`. No history rewrite. |
| No `requirements.txt` for `forecast_candidate` | **Accepted.** Offline stdlib-only producer; no new lockfile added to keep change minimal. |
| Host Go/Node newer than pinned | **No pin change.** Pins remain `1.25.0`/`24.14.0` in `go.mod`, `docker/web.Dockerfile`, `event-spine.md` §9; host variance noted as residual. |
| Prior `bad5073` miswording | **Remediated** by rewording `EVIDENCE.md` + `M4` report (this PR). No commit rewriting; original commits preserved. |
| Docs gaps (prereqs/config/operations/compatibility/troubleshooting/limitations) | **Remediated** by adding `COMPATIBILITY.md`, `TROUBLESHOOTING.md`, `KNOWN_LIMITATIONS.md`, expanding `docs/README.md` table, adding `SECURITY.md` + `CHANGELOG.md v0.1.0`. |

## Confirmation

- **Apache-2.0** (`LICENSE` at repo root, `README.md:docs` and `docs/README` reference it) applies to all release contents. Required notices/attributions are in `NOTICE` (top-level) and `SECURITY.md` dependency policy. See [NOTICE](../../NOTICE) and [LICENSE](../../LICENSE).
- **History rewrite:** not performed. Full-history scan is clean; any future concrete secret/provenance finding requires explicit maintainer approval before `filter-repo`/`BFG` + artifact redaction + rotation.
- **Repository visibility:** remains **private** until this issue and the later release gate pass; visibility change is owner-controlled, not part of this PR.

## Residual limitations

- This audit is reachable-history + workspace only (gitignored `.release-evidence/` is excluded by design and bounded to 64 KiB tails). It is not a hosted-run replay, pentest, formal verification, or full SBOM attestation.
- License review is top-level manifest (`go list -m all`, `npm audit`+lock histogram); transitive native `node_modules` licenses are counted via lock, not per-file header walk.
- Supply-chain beyond digest pins (e.g., registry signing, SLSA, container SBOM) is not claimed for `v0.1`.
- Host Docker images `ahoy-*` are out of repo control; future contributors must re-run this scope from a clean checkout without access to that host.
- `lychee` link checks are live network observations (Redpanda URL historically 500 — documented in evidence); a transient 500 is not a provenance finding.

## Re-run from a clean checkout

```bash
git rev-parse HEAD && git status --porcelain=v1
docker run --rm -v "$PWD":/repo ghcr.io/gitleaks/gitleaks:v8.24.3 detect --source /repo --log-opts="--all" --verbose
docker run --rm -v "$PWD":/repo ghcr.io/gitleaks/gitleaks:v8.24.3 detect --source /repo --log-opts="--all" --verbose --no-git
go list -m all | sort
npm --prefix web audit --json
grep -R "FROM.*:" docker/go.Dockerfile docker/web.Dockerfile; grep -R "image:.*@sha256" compose.yaml
python3 scripts/check_runbooks.py && python3 scripts/test_local_stack.py && python3 -m unittest scripts.test_release_demo -v
```
