# Changelog

All notable changes are documented here. Format is based on Keep a Changelog and SemVer.
This is a bounded local release; no production deployment is implied.

## [v0.1.0] — 2026-08-25

First public-safe local release. Single-command disposable stack on `127.0.0.1` only.
See `docs/EVIDENCE.md` for measured evidence vs. unsupported production claims,
`SECURITY.md` for the bounded security posture, and
`docs/evaluation/RELEASE_AUDIT_REPORT.md` for the `v0.1.0` audit.

### Added

- Runnable `cmd/seshatops` (migrations, `/livez`/`/readyz`, relay+consumer workers, JSON `slog`, bounded aggregate `/metrics` `MX-010`, graceful shutdown) — #109.
- Deterministic Northstar bootstrap `go run ./cmd/seshatops bootstrap` (seed `northstar-m3-lineage-v1`, 5 events, idempotent, checksum) — #110.
- One-shot frozen forecast `go run ./cmd/seshatops forecast` (rebuild `northstar-m4-stockout-v1` protocol `m4-stockout-eval-v1`, offline `forecast_candidate`, Go evaluation, persisted via `platform.ForecastService`) — #111.
- Pinned disposable Compose stack (`compose.yaml`): PostgreSQL `16.14@sha256:952067…`, Redpanda `v25.2.1@sha256:218469…`, mock OIDC `4.0.0@sha256:79f51f…`, Go `1.25.0`, Python `3.13.7-slim`, Node `24.14.0` — #112.
- Release observability: authenticated `/metrics` + structured logs, redaction contract, process-local vs DB-backed gauges — #103.
- Eight-scenario demo harness `./scripts/local-stack.sh demo` with bounded JSON+text evidence, deterministic identities, guarded destructive actions — #104.
- Four recovery runbooks + exercise report — #105.
- Public hardening: `SECURITY.md`, release audit, troubleshooting/compatibility, corrected hosted-CI reconciliation for PR #98 (`bad5073` implementation vs `39911bc` docs-only head) — #106.

### Predictor outcome

- Frozen M4 evaluation selected deterministic `seasonal_naive` (Go-owned, `m4-deterministic-baselines-v1`). Candidate AP `0.953728…` on test was below baseline `1.0`; candidate **not promoted**. No forecasting-quality production claim.

### Test-environment boundaries

- Single-host Testcontainers/Compose. Durations are diagnostic, not SLOs. At-least-once transport persists; any `duplicate_noop` is honest.
- Local Windows gate without Docker skipped PostgreSQL-backed persistence/tenant-negative checks — hosted CI covers them (`api` gate on Docker).
- Sparse live Event Spine (`northstar-m3-lineage-v1`) vs dense frozen fixture (`northstar-m4-stockout-v1`): live `GET /v1/…/forecast/features` correctly returns `insufficient`/`stale`/`incomplete` with empty `rows`; `GET /…/predictions/*` may be `stale`.
- Demo/observation counters are process-local and reset on restart; backlog gauges are scrape-time snapshots.

### Breaking and unsupported surfaces

- `v0.1` is initial release; no prior compatibility promise.
- No public internet hosting, production TLS, HA, autoscaling, backup automation, multi-host, or cloud deployment.
- No mutable deployment, Kubernetes/Terraform/service-mesh/Grafana, Approved Actions/RAG, object storage, ERP HTTP surface, model serving, feature store, or Python writes/creds.
- Event families limited to 5 M3 families at `event_schema_version 1` (`docs/design/specifications/event-spine.md` §2); unknown schemas are quarantined, not migrated.
- Python is offline one-shot only; `seshatops forecast` requires disposable DB `seshatops_northstar_disposable` + `SESHATOPS_FORECAST_CONFIRM`.

### Upgrade and reset expectations

- No live upgrade path. To move between releases: `SESHATOPS_LOCAL_RESET_CONFIRM=I_UNDERSTAND_DISPOSABLE_LOCAL_RESET ./scripts/local-stack.sh reset` (removes only `seshatops-local` project, network, volumes; host Docker endpoint must be local Unix socket, guard validated) then `./scripts/local-stack.sh quickstart`.
- Bootstrap rerun is immutable/idempotent; second `forecast` returns same `prediction_id`. Redpanda history is retained across bootstrap; rebuild preserves `before==result==after` checksums on unchanged history.
- Evidence under `.release-evidence/` is gitignored and bounded (`256 KiB` result, `64 KiB` diagnostics).

### Toolchain pins (v0.1.0)

- Go `1.25.0`, Node `24.14.0`, npm `11.9.0`, TypeScript `6.0.3`, Python host `3.10+` (harness) / `3.13.7-slim` runtime image — see `docs/design/specifications/event-spine.md` §9.

### Release artifacts and reproducibility (`v0.1.0`)

- Immutable artifact set (from tagged commit): reproducible source archive `seshatops-v0.1.0-source.tar.gz`, stamped `seshatops` binary (`-X main.Version/Commit/BuildTime`, `seshatops version` + `GET /version` + `seshatops_build_info{version,commit}`), `web/dist` assets, `SHA256SUMS`, `VERSION`/`BUILD_TIME`, `go-sbom.json` / `web-sbom.cyclonedx.json` + `go-deps.txt`/`pinned-images.txt`, bounded `campaign.json` evidence index — see `docs/REPRODUCIBILITY.md`.
- Clean-checkout release CI (`.github/workflows/release-ci.yml`): Go/Python/Web/Docs lint/link/YAML/secret, `govulncheck@v1.1.4`, `npm audit`, license gates (no GPL/AGPL), SBOM generation, `go build` stamping, `SHA256SUMS`, pinned `docker compose build`, headless `local-stack.sh smoke` + `poison-isolation` fault, `down --remove-orphans` + bounded diagnostics on failure — no `latest` mutable sole identity.
- Tag workflow (`.github/workflows/release.yml`): `push: tags: v*.*.*`, `environment: release`, `contents: write` only on `refs/tags/v*`, creates immutable GitHub Release for that tag; PR code never receives publish permission.
- Local reproduce vs published checksums: `docs/REPRODUCIBILITY.md` documents exact commands and `sha256sum -c SHA256SUMS --ignore-missing` comparison; `BUILD_TIME` variance is documented — version/commit/fixture/protocol checksums (`frozen_m4_dataset b29e79…`, `frozen_m4_feature_snapshot 80898…`) are the deterministic identity, `stable_identity` excludes `durations_ms`.

### Hosted CI references (implementation vs docs-only)

- PR #98 implementation commits `8b53501` → `bad5073`: Go CI `32181303886` (with API gate), Web CI `32181304013`, Docs CI `32181303898/job/95854713070` (all green). Merged head was docs-only `39911bc` (no code change). Earlier Docs run on `8b53501` failed only on pre-existing Redpanda URL 500 `32180186848/job/95851340432`. See `docs/EVIDENCE.md` and `docs/evaluation/M4_STOCKOUT_INTELLIGENCE_EXIT_GATE_EXPERIMENT_REPORT.md`.

[v0.1.0]: https://github.com/G1DO/seshatops/releases/tag/v0.1.0
