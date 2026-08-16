# M3 traceability and recovery test campaign

Issue #77. One integrated Northstar Foods batch trace with provenance, duplicate
lineage delivery, poison isolation with unrelated work continuing, reset/rebuild
of unchanged history to matching inventory and lineage checksums, and
fail-closed tenant-negative traceability HTTP. Test environment only. Not
production behavior.

Operator G1DO on `feat/77-m3-exit-gate`, 2026-08-16. Measured on a working tree
based on `557d906` plus this campaign's named gate test and report. Hosted CI is
not claimed until this pull request's workflow runs exist. Matrix `MX-002` /
`MX-003`; tenants `TENANT-NS-001` / `TENANT-NS-002`; seed
`northstar-m3-lineage-v1`. Single Linux host with Docker Testcontainers
(PostgreSQL 16.14) and in-process sessions. Operator = reviewer. Operator Go
`1.26.2` (module targets `1.25.0`). Operator Node `24.18.0` / npm `11.16.0`
(package engines `24.14.0` / `11.9.0`).

## Commands

```bash
go test ./... -count=1 -timeout 15m
cd web && npm test && npm run typecheck && npm run build
```

`SESHATOPS_TEST_DATABASE_URL` was unset. `npm ci` was not re-run; `web/node_modules`
was already present.

## Results

Pass.

| # | Scenario | Result |
| --- | --- | --- |
| 1 | Authorized batch trace with upstream and downstream hops and provenance | Pass (`TestM3TraceabilityRecoveryExitGate`; supporting `TestOpsReaderCanTraceSameTenantBatch`) |
| 2 | Duplicate delivery has no duplicate lineage effect | Pass (gate test `duplicate_noop`; supporting `TestLineageDuplicateDoesNotChangeChecksums`) |
| 3 | Poison/incompatible traceability event isolated; unrelated mill applies | Pass (gate test `unsupported_contract`; supporting `TestLineageUnsupportedSchemaQuarantinedNoLineageEffect`) |
| 4 | Reset/rebuild of unchanged history reproduces inventory and lineage checksums | Pass (gate test; supporting `TestLineageRebuildChecksumEquality`) |
| 5 | Tenant-negative traceability read is fail-closed | Pass (`403` on `TENANT-NS-002` path with no hops; `404` for unknown batch; supporting `TestCrossTenantTracePathDenied`, `TestUnauthenticatedTraceRefused`, `TestCrossTenantBatchIdDoesNotLeakExistence`) |
| 6 | UI lineage is presentation | Pass (Vitest `fetchBatchTrace` / `useBatchTrace` / `OperationsView`; not isolation evidence) |

```text
ok  	github.com/G1DO/seshatops/api	400.646s
ok  	github.com/G1DO/seshatops/erp	219.837s
ok  	github.com/G1DO/seshatops/event	0.015s
ok  	github.com/G1DO/seshatops/identity	2.759s
ok  	github.com/G1DO/seshatops/northstar	0.016s
ok  	github.com/G1DO/seshatops/platform	441.794s
ok  	github.com/G1DO/seshatops/relay	456.227s
```

Web: 9 files / 44 tests passed; typecheck passed; Vite production-mode
`npm run build` passed. That build is not a production environment.

The named gate test applies canonical envelope bytes through
`platform.ProcessRecord` and authorized `httptest`. Full `go test ./...` still
includes Redpanda relay tests from the Event Spine campaign. This M3 scenario
does not add a new broker topology.

## Limitations

- Single-host Testcontainers is not staging or production.
- Package suites are not one long-lived multi-process binary.
- There is no deployment binary or production environment.
- Vitest covers lineage presentation only.
- Operator and reviewer are the same person.
- Library/test sessions and assignments; not production revocation or pentest.
- Rebuild proof is checksum equality on retained history, not backup/restore,
  RPO/RTO, or cloud deployment.
- Does not establish exactly-once delivery, SLO, or capacity.
