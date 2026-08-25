# Release reproducibility

How to reproduce the `v0.1.0` release CI from a clean checkout and compare
local output with published artifact checksums. No cloud, Kubernetes, or
production deployment is involved.

## Artifact set (`v0.1.0`)

From the tagged commit (`git rev-parse HEAD`) the release workflows produce
immutable artifacts under `dist/` and attach them to the GitHub Release for
`v0.1.0`:

| Artifact | Identity | Source |
| --- | --- | --- |
| Source archive | `seshatops-v0.1.0-source.tar.gz` + `SHA256SUMS` | `git archive --format=tar.gz HEAD` |
| Go executable | `seshatops` (linux/amd64, `CGO_ENABLED=0 -trimpath`) + `SHA256SUMS` | `go build -ldflags="-X main.Version=$TAG -X main.Commit=$COMMIT -X main.BuildTime=$BUILD_TIME"` |
| Web assets | `web/dist/` contents + `SHA256SUMS` | `npm ci && npm run build` |
| Checksums | `SHA256SUMS` (source archive, binary, web assets, `go.sum`, `web/package-lock.json`, fixture `.sha256`) | `sha256sum` |
| Version record | `VERSION` (`version commit build_time`), `BUILD_TIME` | `seshatops version` |
| SBOM / inventory | `go-sbom.json` (`go list -m -json all`), `web-sbom.cyclonedx.json` (`npm sbom`), `go-deps.txt`, `pinned-images.txt` | `go list`, `npm sbom` |
| Evidence index | `campaign.json` + per-scenario `<scenario>.json` (bounded `256 KiB`) | `./scripts/local-stack.sh demo all` |
| Release notes | `CHANGELOG.md` (`v0.1.0`), `docs/EVIDENCE.md`, `docs/evaluation/RELEASE_AUDIT_REPORT.md` | repo |

No `latest` mutable tag is published as the sole identity. Each tag `v*.*.*`
creates a new immutable set via `.github/workflows/release.yml` (`environment: release`,
`permissions: contents: write` only on tag). PR code never receives publish
permission.

Pinned image digests are not rebuilt as release artifacts; they are recorded in
`pinned-images.txt` and verified against `compose.yaml` (`postgres@sha256:952067…`,
`redpanda@sha256:218469…`, `mock-oauth2-server@sha256:79f51f…`,
`golang:1.25.0-bookworm@sha256:81dc45…`, `python:3.13.7-slim@sha256:adafcc…`,
`node:24.14.0-bookworm-slim@sha256:d8e448…`) — see `docs/COMPATIBILITY.md`.

## Stamping

The runtime and every machine result are stamped with the same identity:

- CLI: `seshatops version` / `seshatops --version` → `{version, commit, build_time, go_version, fixture_versions, protocol_versions, artifact_checksums}`
- HTTP: `GET /version` (public, no auth) → same payload; also `seshatops_build_info{version,commit} 1` in `/metrics` (bounded label, not tenant/cardinality).
- Bootstrap: `bootstrap` JSON now includes `runtime_version/runtime_commit/runtime_build_time` alongside `fixture_version/tenant_id/projection_checksum`.
- Forecast: `forecast` JSON includes `runtime_version/runtime_commit/runtime_build_time` alongside `dataset_version/feature_snapshot/checksum`.

`version` is the tag `v0.1.0` (default when building from non-tag), `commit` is
`git rev-parse --short HEAD`, `build_time` is `date -u +%Y-%m-%dT%H:%M:%SZ`.
Fixture versions (`northstar-m3-lineage-v1`, `northstar-m4-stockout-v1`, etc.)
and protocol versions (`m4-stockout-eval-v1`, `m4-raw-onhand-v1`,
`m4-deterministic-baselines-v1`, `m4-python-stockout-candidate-v1`,
`event_schema_version 1`) are compiled constants and never derived from the
network.

## Reproducing the CI release build locally

From a clean checkout — no pre-seeded `postgres-data`/`redpanda-data` and a clean
worktree (`git status --porcelain=v1` empty):

```bash
git rev-parse HEAD && git status --porcelain=v1   # must be clean
go version                                         # want go1.25.0 (pinned)
node --version; npm --version                      # want v24.14.0 / 11.9.0
python3 --version                                  # want 3.10+ (CI uses 3.12)
docker --version; docker compose version

# 1. Component gates (mirror Go/Web/Documentation CI)
go test ./... -count=1 -timeout 15m
python3 -m unittest discover -s forecast_candidate -p 'test_*.py' -v
python3 -m unittest scripts/test_release_demo.py -v
python3 scripts/test_local_stack.py
python3 scripts/check_runbooks.py
npm --prefix web ci
npm --prefix web run typecheck
npm --prefix web run test
npm --prefix web run build

# 2. Dependency / security checks (pinned, fail-or-disposition)
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
govulncheck ./...                                  # must report no vulnerabilities (else disposition in SECURITY.md)
npm --prefix web audit --json                      # high+critical must be 0 (see docs/evaluation/RELEASE_AUDIT_REPORT.md)
go list -m all > go-deps.txt                       # inspect for GPL/AGPL (must be empty)
grep -E '"license"' web/package-lock.json | sort | uniq -c

# 3. SBOM / inventory (optional traceability, no signing claim beyond pin)
go list -m -json all > go-sbom.json
npm --prefix web sbom --sbom-format cyclonedx --omit dev > web-sbom.cyclonedx.json || npm --prefix web sbom --omit dev > web-sbom.json

# 4. Stamped binary and web assets
TAG=$(git describe --tags --always 2>/dev/null || echo "v0.1.0")
if ! echo "$TAG" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+'; then TAG="v0.1.0"; fi
COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
mkdir -p dist
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.Version=$TAG -X main.Commit=$COMMIT -X main.BuildTime=$BUILD_TIME" -o dist/seshatops ./cmd/seshatops
./dist/seshatops version | python3 -m json.tool
sha256sum dist/seshatops > dist/SHA256SUMS
sha256sum go.sum web/package-lock.json >> dist/SHA256SUMS
if [ -d web/dist ]; then find web/dist -type f -exec sha256sum {} \; | sort >> dist/SHA256SUMS; fi
cat dist/SHA256SUMS

# 5. Pinned images
grep -E 'image:.*@sha256' compose.yaml
docker compose --project-name seshatops-local --file compose.yaml build

# 6. Packaged stack (headless, no bypass)
bash scripts/local-stack.sh smoke
# After smoke leaves stack down, run a representative fault without bypassing the real runtime:
SESHATOPS_DEMO_CONFIRM=I_UNDERSTAND_DISPOSABLE_LOCAL_DEMO python3 scripts/release_demo.py poison-isolation --evidence-dir .release-evidence/ci-local-test
cat .release-evidence/ci-local-test/poison-isolation.json | python3 -m json.tool | head -n 80
```

The `release-ci.yml` job runs the same sequence from a clean checkout
(`fetch-depth: 0`), waits for `/readyz` and `/version`, bootstraps Northstar,
forecasts, and runs `poison-isolation`. It uploads bounded diagnostics
(`64 KiB` tails, `256 KiB` result) only on failure via
`actions/upload-artifact@834a144...  v4.6.2`.

## Comparing local output with published checksums

After the release job publishes `v0.1.0`:

```bash
TAG=v0.1.0
curl -fsSL -o /tmp/SHA256SUMS "https://github.com/G1DO/seshatops/releases/download/${TAG}/SHA256SUMS"
curl -fsSL -o /tmp/seshatops-source.tar.gz "https://github.com/G1DO/seshatops/releases/download/${TAG}/seshatops-${TAG}-source.tar.gz"
sha256sum -c /tmp/SHA256SUMS --ignore-missing
# For the binary, the build-time embedded field means bitwise equality requires
# using the published BUILD_TIME; compare version/commit/fixture identity instead:
./dist/seshatops version | python3 -c "import json,sys; d=json.load(sys.stdin); assert d['version']=='$TAG' and d['fixture_versions']['northstar-m3-lineage-v1']=='northstar-m3-lineage-v1'"
# Deterministic harness identities exclude diagnostic timestamps/durations:
python3 -c "import hashlib,json; print(hashlib.sha256(open('dist/SHA256SUMS','rb').read()).hexdigest())"
```

Two builds from the **same commit with the same `BUILD_TIME`** produce the same
`SHA256SUMS`; when `BUILD_TIME` differs the SHA differs but `version/commit`
plus the fixture/protocol checksums (`frozen_m4_dataset b29e79…`,
`frozen_m4_feature_snapshot 80898…`) remain identical — the latter is the
declared deterministic identity. `release_demo.stable_identity` likewise excludes
`durations_ms`/`timestamps` and is used with `scripts/release_demo.py --compare`.

## Documentation and audit parity

Before any tag:

```bash
# Documentation, link, YAML, secret scans (mirrors Documentation CI)
python3 -m pip install --quiet markdownlint-cli2 2>&1 | head -n 20 || true
# hosted CI uses DavidAnson/markdownlint-cli2-action@21c1be1 w/ '**/*.md'
lychee --config .lychee.toml --no-progress './**/*.md' 2>&1 | head -n 100 || true
yamllint . --config-file .yamllint.yml --strict 2>&1 | head -n 50 || true
gitleaks detect --source . --verbose --log-opts="--all" 2>&1 | head -n 50
```

`docs/evaluation/RELEASE_AUDIT_REPORT.md` records the exact commands, tool
versions (`gitleaks 8.24.3`), findings, dispositions, and residual limitations
(not a pentest, not full SBOM attestation). PRs fail closed on any scanner
failure without disposition.
