# Known limitations and residual risks

Honest `v0.1.0` local release only. No production deployment, SLO, capacity, pentest,
formal verification, or compliance claim. See also: [EVIDENCE.md](EVIDENCE.md),
[COMPATIBILITY.md](COMPATIBILITY.md), [TROUBLESHOOTING.md](TROUBLESHOOTING.md),
[SECURITY.md](../SECURITY.md), [operations/observability.md](operations/observability.md),
and [evaluation/RELEASE_AUDIT_REPORT.md](evaluation/RELEASE_AUDIT_REPORT.md).

## Topology

- Single-host disposable Compose (`seshatops-local`) on `127.0.0.1` only. No public internet,
  no TLS production termination, no HA/failover, no autoscaling, no multi-host.
- PostgreSQL and Redpanda persistent volumes are per-project (`postgres-data`, `redpanda-data`);
  `down` preserves them, `reset --volumes` drops only this project (guarded).
- Vite `web` is the only browser entrypoint; Go `/auth/*`, `/v1/*`, `/metrics` are same-origin
  via proxy. PostgreSQL/Redpanda/Go have no host ports. `redpanda:9092` is internal only.

## Transport and projection

- At-least-once delivery persists; any `duplicate_noop` is expected and must not be read as
  exactly-once. Broker outage → `seshatops_runtime_ready 0`, `GET /readyz 503`, durable outbox `pending`.
- Checksums are `SHA-256` over tenant-sorted, newline-delimited, tab-delimited rows; empty projection
  hashes empty bytes. They exclude timestamps/offsets. Determinism is `before == result == after` only
  on unchanged history in the same checkout.
- Lineage hops are 1:1 (supplier → lot → batch → shipment → order). Multi-line orders, commands,
  approvals, and replenishment events remain out of scope.
- Retrying after `quarantined_gap` is automatic and revalidates content hash; terminal
  `quarantined_conflict`/`stale`/`invalid`/`mismatch`/`transition` is `409 not_releasable` by design.

## Identity and authorization

- Sessions/assignments are process-local memory; restart invalidates cookies, revocation is library/test
  OIDC only, no production revocation list. Missing/stale/contradictory context fails closed.
- Allow-list is explicit tenant+role+resource+action (`MX-001`–`MX-010`). `SCOPE-RUNTIME`+`ROLE-RELEASE-OBSERVER`
  for `/metrics` is Go-selected, never caller-supplied. Browser cannot authorize.
- Privileged ops are audit-before-mutate; `audit_failed` blocks mutation. Audit table is append-only via trigger.

## Forecasting and features

- Sparse live Event Spine (`northstar-m3-lineage-v1`, 5 events) cannot reproduce the dense
  112-day frozen fixture (`northstar-m4-stockout-v1`). Live `GET /…/forecast/features` therefore correctly
  returns `insufficient`/`stale`/`incomplete` with empty `rows`; this is honest reporting, not a model result.
- Frozen evaluation selected deterministic `seasonal_naive` (`m4-deterministic-baselines-v1`);
  candidate AP `0.953…` < baseline `1.0` on test — candidate **not promoted**. No quality or business-impact claim.
- `forecast` is one-shot, typed, bounded-deadline Python subprocess (`forecast_candidate/stockout_candidate.py`);
  `unavailable`/`timeout`/`invalid_response` are typed failures that persist **no** row. Freshness `stale`
  after `forecast` on sparse live history is correct.
- Feature route is `READ ONLY`/`REPEATABLE READ` over retained `erp.outbox` + `platform.inbox` + projection;
  no feature table, no Python DB creds, no labels in rows. Caller `horizon`/`cutoff`/`dataset` overrides are `400`.

## Observability

- Counters (`seshatops_*_total`, `_sum`, `_count`) are process-local and reset on `runtime` restart.
  Backlog/failure gauges are scrape-time snapshots, not durable lag history or SLO.
- Logs are JSON `slog` on stderr, bounded and redacted (no bodies/cookies/tokens/event bytes/artifact/feature rows/error strings).
  Only `correlation_id`, fixed enums, checksums, and durations are emitted.
- Durations/timestamps are diagnostic measurements, not recovery-time or availability evidence.

## Evidence and harness

- Machine evidence in gitignored `.release-evidence/<UTC>/` — `256 KiB` result / `64 KiB` diagnostics, redacted.
  `stable_identity` excludes `durations_ms`/`timestamps`; deterministic comparison requires clean checkout (`git rev-parse HEAD`
  equal, `source_sha256` over `git ls-files` `64 MiB` cap, `RUNBOOK_EXERCISE_REPORT.md` excluded).
- Windows local gate without Docker explicitly skips PostgreSQL-backed cases; hosted CI (Docker) is the covering run.
  Each scenario resets the stack; `all` stops on first failure.

## Deferred and rejected

- **Deferred (not `v0.1`):** managed cloud (any provider), Kubernetes/Terraform/service-mesh, Grafana/dashboards, object storage,
  production secrets manager, backup/restore automation, multi-region, sustained load/chaos platform, Approved Actions/RAG.
- **Rejected:** deriving tenant from headers/query/body/IdP, trusting client tenant headers, injecting sessions, directly mutating
  finished projections, broad `docker volume prune`/`system prune`, grant of `ROLE-PLATFORM-OPERATOR` → forecast reads,
  `UPDATE platform.inventory_projection`/`DELETE FROM erp.outbox`/`TRUNCATE` in runbooks, `shell=True`, production pentest/compliance claim.

## Unsupported production conclusions

Do **not** conclude from `v0.1` evidence: production availability/SLO/RTO/RPO, capacity, sustained-load resilience, multi-tenant
scale, global ordering, network partition tolerance beyond test harness, or forecasting utility for replenishment.

When in doubt, follow the runbooks (which use only implemented commands) and re-run
`SESHATOPS_DEMO_CONFIRM=I_UNDERSTAND_DISPOSABLE_LOCAL_DEMO ./scripts/local-stack.sh demo <scenario>`
for the local reproduction; record the bounded `*.json`/`*.txt` plus `logs runtime --tail 200` and stop.
