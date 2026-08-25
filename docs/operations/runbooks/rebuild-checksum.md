# Tenant-scoped projection rebuild and checksum verification

Local, disposable release evidence only. Not a production backfill or disaster-recovery design.

## Trigger and observable symptoms

* Suspected projection drift after broker recovery, after poison isolation, or after a restart where `platform.inventory_projection` or lineage hops may not match retained source history.
* Desire to produce a deterministic proof that unchanged history rebuilds to the same checksum: the before/result/after inventory and lineage checksums should be equal (`event-spine.md` §8).
* Ops visibility shows `GET /v1/tenants/11111111-1111-4111-8111-111111111111/ops` `projection.checksum` diverges from bootstrap summary checksum, or `incomplete_reasons` present after prior `rebuild`/`replay`.
* Metrics prior state: `seshatops_control_operations_total{operation="rebuild",outcome="..."}` count for this process (resets on restart). Logs: prior `ops.control.completed` with `operation="rebuild"`.

A rebuild is not triggered by `401/403` (authorization denial) or by forecast `stale`/`insufficient` alone. It is tenant-scoped; no cross-tenant rebuild is possible in this release.

## Scope and safety assumptions

* Tenant-scoped derived-state rebuild only: `POST /v1/tenants/{tenant_id}/ops/rebuild` resets that tenant's `platform` derived rows (`inventory_projection`, lineage tables, inbox disposition for that tenant) and replays retained `erp.outbox` / `platform.inbox` history in Go. It never copies foreign-tenant rows, never mutates `erp.outbox` source, and never touches `identity.authorization_decisions`.
* Privileged control: requires authenticated Go-owned session and explicit allow-list row `MX-006` (`ROLE-PLATFORM-OPERATOR` on `RES-REBUILD` `ACT-REBUILD` for that tenant). Path tenant is authoritative; body `tenant_id` if sent is ignored.
* Audit-before-mutate enforced. Insert into `identity.authorization_decisions` must succeed before rebuild; `500 {"error":"audit_failed"}` means no mutation occurred.
* Deterministic checksums: inventory checksum is defined in `docs/design/specifications/event-spine.md` §8 (sorted canonical projection rows, SHA-256). Lineage checksum is independent inventory of traceability hops (`northstar-m3-lineage-v1`). Both are logged as validated hex-64, never as metric labels.
* Process-local counters reset on restart; checksum identity is the durable proof.

## Diagnosis commands and expected signals

All commands below are implemented (`#99`-`#104`). No direct `psql` or Redpanda topic editing.

```bash
./scripts/local-stack.sh status
./scripts/local-stack.sh logs runtime --tail 200
```

Authenticated baseline (login at `http://web.seshatops.localhost:5173` as `northstar-demo-operator`):

```js
fetch("/metrics", {credentials:"same-origin"}).then(r=>r.text()).then(t=>{
  console.log(t.match(/seshatops_control_operations_total\{operation="rebuild".*/g));
  console.log(t.match(/seshatops_control_duration_seconds.*/g));
  console.log(t.match(/seshatops_runtime_ready .*/g));
})
```

Ops and inventory snapshots:

```bash
curl -b /tmp/cookies.txt \
  http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops | jq .projection
curl -b /tmp/cookies.txt \
  http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/inventory | jq '{checksum, observed_at}'
curl -b /tmp/cookies.txt \
  http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops/lineage/batches/batch-bread-001 | jq .batch.id
curl -b /tmp/cookies.txt \
  "http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops/audit" | jq .records
```

Expected healthy baseline (after `quickstart` bootstrap of `northstar-m3-lineage-v1`):

* `GET /inventory` `checksum` is stable hex-64 (e.g., `a3f7...` 64 chars). `GET /ops` `projection.checksum` equals `inventory` checksum.
* `GET /ops/audit` contains only this tenant's records (no `TENANT-NS-002` leakage).
* `seshatops_runtime_ready 1` and `GET /readyz` `200 ready`.
* `logs runtime` shows `relay.cycle.completed` / `consumer.cycle.completed` at steady state.

Redacted log shape after a rebuild:

```json
{"time":"2026-08-25T12:02:00.000Z","level":"INFO","msg":"ops.control.completed","correlation_id":"c3d4e5f6-...","operation":"rebuild","outcome":"complete","duration_ms":87,"checksum":"a3f7...","lineage_checksum":"b4e8..."}
```

Only validated `checksum`/`lineage_checksum` appear; no raw rows or error strings.

## Smallest safe action sequence

1. **Record before state** — capture authenticated evidence:

   ```bash
   curl -b /tmp/cookies.txt \
     http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops | tee /tmp/ops-before.json
   curl -b /tmp/cookies.txt \
     http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/inventory | tee /tmp/inv-before.json
   jq -r .projection.checksum /tmp/ops-before.json  # before_inv_checksum
   jq -r .checksum /tmp/inv-before.json
   ```

2. **Execute authorized tenant rebuild** — requires `ROLE-PLATFORM-OPERATOR` on this tenant. Empty body; `tenant_id` in path is authoritative:

   ```bash
   curl -b /tmp/cookies.txt -c /tmp/cookies.txt \
     -X POST http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops/rebuild \
     -H "Content-Type: application/json" -d '{}' | tee /tmp/rebuild-result.json
   jq -r '.status, .checksum, .lineage_checksum' /tmp/rebuild-result.json
   ```

   Expected `200` body (see `docs/api/openapi-projection.yaml` `ControlResult`):

   ```json
   {"tenant_id":"11111111-1111-4111-8111-111111111111","control":"rebuild","status":"complete","applied":5,"duplicate_noop":0,"quarantined":0,"checksum":"a3f7...","lineage_checksum":"b4e8..."}
   ```

   * `status == "complete"` with both checksums present is the only successful proof. `status == "incomplete"` with `incomplete_reasons` is not a successful checksum proof — follow stop condition.
   * Metric `seshatops_control_operations_total{operation="rebuild",outcome="complete"}` increments by 1; `seshatops_control_duration_seconds_sum/count` advance.

3. **Verify after state** — re-read projections and compare:

   ```bash
   curl -b /tmp/cookies.txt \
     http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops | tee /tmp/ops-after.json
   curl -b /tmp/cookies.txt \
     http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/inventory | tee /tmp/inv-after.json
   jq -r .projection.checksum /tmp/ops-after.json  # after_inv_checksum
   jq -r .checksum /tmp/inv-after.json
   ```

   Require `before_inv_checksum == rebuild_result.checksum == after_inv_checksum` and `rebuild_result.lineage_checksum` equals both `ops-before`/`ops-after` lineage if present. For `northstar-m3-lineage-v1`, both inventory and lineage checksums are defined; for sparse live history the lineage checksum may be stable but the relevant invariant is inventory equality.

4. **Confirm audit and logs**:

   ```bash
   curl -b /tmp/cookies.txt \
     "http://web.seshatops.localhost:5173/v1/tenants/11111111-1111-4111-8111-111111111111/ops/audit" | jq '.records | last'
   ./scripts/local-stack.sh logs runtime --tail 200 | grep ops.control.completed
   ```

   Last audit record has `principal_id == northstar-demo-operator`, `tenant_id == 11111111-...`, `resource == RES-REBUILD`, `action == ACT-REBUILD`, `outcome == allow`, `reason == matrix_allow`, with monotonic `at`. Log line matches the rebuild checksums and `duration_ms`.

Do not: supply `event_id` (rebuild ignores it), set `tenant_id` in JSON body to escalate, run two concurrent rebuilds, or copy checksums from untrusted caller input.

## Stop / rollback / escalation conditions

* Stop if `401 {"error":"unauthenticated"}` or `403 {"error":"forbidden"}` — caller lacks `MX-006` for this tenant. Re-login with correct assignment; do not add a permissive row.
* Stop if `500 {"error":"audit_failed"}` — audit insert failed, no rebuild occurred; check `postgres` health, retain bounded tail.
* Stop if `200` with `status == "incomplete"` or non-empty `incomplete_reasons` — retained history is not yet complete (e.g., pending outbox, quarantined rows not resolved). This is not a checksum proof; resolve underlying backlog/poison first (see sibling runbooks) before claiming determinism.
* Stop if checksums differ — this indicates retained history changed between reads (concurrent source accept or dirty `reset-northstar`). Re-capture before/after without concurrent writes; if persistent, retain `ops-before`, `rebuild-result`, `ops-after`, `logs runtime` and halt.
* No automatic rollback reverts a rebuild; rebuilding again on the same unchanged history is idempotent and must yield the same checksums.

## Post-recovery checks and evidence to retain

* Three-way checksum equality: `before == result == after` for inventory, and `result.lineage_checksum` matches after (and before if present).
* `GET /readyz` remains `200 ready` and `seshatops_runtime_ready 1` (rebuild does not degrade readiness).
* `GET /v1/tenants/.../ops/audit` shows one new `allow` record for `RES-REBUILD` `ACT-REBUILD` with correct tenant.
* Metric `seshatops_control_operations_total{operation="rebuild",outcome="complete"}` advanced; duration summary reflects wall-clock handler time (diagnostic, not SLO).
* Retain (redacted, ≤64 KiB): `/tmp/ops-before.json` `projection.checksum`, `/tmp/rebuild-result.json`, `/tmp/ops-after.json` `projection.checksum`, one audit record tail, `logs runtime --tail 200` containing `ops.control.completed` with `correlation_id`, and `GET /metrics` rebuild counters. Search for any fixture identifiers before retaining; do not commit secrets.

## Limitations and unsupported production conclusions

* Scope is one disposable local host, one tenant (`TENANT-NS-001`), stable `northstar-m3-lineage-v1` history. Not production backup/restore, not cross-region rebuild, not sustained load, not object-store recovery.
* Duration and `observed_at` are diagnostic; not recovery-time or availability evidence.
* At-least-once transport persists; duplicate delivery after rebuild must be `duplicate_noop` and checksum-preserving (honest claim).
* No claim that rebuild fixes poison — `409 not_releasable` inbox rows remain isolated; replay/release controls are separate.
