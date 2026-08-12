# M1 Projection Read API Review - Issue #27

This document records the Issue #27 implementation review for the read-only Go
REST snapshot and SSE live-update surface over the committed inventory
projection. It does not claim authentication enforcement, exactly-once SSE
delivery, production readiness, deployment completeness, or hosted CI success
without a recorded GitHub Actions run.

## Review record

| Field | Value |
| --- | --- |
| Reviewer | Implementation pass on branch `feat/27-projection-api-sse`; maintainer review remains a follow-up |
| Date (UTC) | 2026-08-12 |
| Branch | `feat/27-projection-api-sse` |
| Scope | Issue #27 only: `platform` list/notify helpers, `api` REST/SSE library, OpenAPI/contract docs, integration tests |
| Review type | Read-boundary, post-commit ordering, reconnect honesty, clean-room, and verification honesty review |
| Runtime disposition | PostgreSQL-backed library/integration tests via `httptest`; no deployment binary, auth, or TypeScript client |

## Acceptance matrix

| Issue #27 criterion | Evidence | Disposition |
| --- | --- | --- |
| REST returns only committed inventory-projection state | `TestRESTReturnsCommittedProjection`; `platform.ListTenantProjection` | Covered |
| SSE emits only after projection transaction commits | `TestSSEEmitsOnlyAfterCommit`; rollback silence via `SetFailBeforeCommitForTest` | Covered |
| Reconnecting client converges via REST snapshot/catch-up | `TestSSEDisconnectReconnectRESTConverge`; [PROJECTION_READ_API.md](../architecture/PROJECTION_READ_API.md) reconnect section | Covered |
| API surface is read-only for M1 projection state | `TestReadOnlyRejectsMutatingMethods` (`405`) | Covered |
| Browser-facing contracts do not require DB/broker access | `api` imports PostgreSQL helpers only (no `relay`/franz-go); architecture docs | Covered |
| Version/freshness metadata represented consistently | Snapshot `aggregate_version` + `checksum` + `observed_at`; SSE `last_applied_event_id` + `checksum` | Covered |
| Tenant/context fields without claiming M2 auth | Lowercase UUIDv4 validation; explicit M2 deferral in API docs | Covered |
| API/SSE errors do not mutate projection state | Malformed/read-only negatives leave projection unchanged | Covered |

## Verification record

| Check | Result |
| --- | --- |
| `go test ./platform ./api -count=1 -timeout 25m` | Passed locally on 2026-08-12 with Docker (pinned PostgreSQL image) |
| `go list` imports for `./api` | `platform`, `database/sql`, `net/http`, stdlib only — no Redpanda client |
| Hosted Go CI | Not claimed until a GitHub Actions run exists for the reviewed commit |
| Documentation CI | Not claimed until a hosted run exists |
| `EVIDENCE.md` claim promotion | None; live-view / projection claims remain `Planned` |

## Commit-then-update trace (local)

Local library path used for ordering evidence:

1. SSE client subscribes to `/v1/tenants/{tenant}/inventory/stream`.
2. `ProcessRecord` is forced to fail before PostgreSQL commit → no SSE frame.
3. Successful apply commits projection (`quantity_on_hand=8`, `aggregate_version=1`) → one `inventory_projection.updated` frame with matching checksum.

This is a local test trace only. It does not promote any claim to Observed or
Reproduced.

## Clean-room review

- Identifiers and fixture material remain fictional Northstar Foods /
  CONTRACTS.md artifacts.
- No Ahoy code, schemas, data, identifiers, logs, screenshots, or business rules
  were used.
- No secrets, credentials, or private hostnames were introduced.

## Residual risk and follow-ups

- In-process hub notifications are process-local; API process restart requires
  clients to REST catch-up and reopen SSE.
- Slow clients may miss SSE frames by design; REST remains authoritative.
- Authentication and tenant authorization enforcement remain M2.
- Issue #28 owns the TypeScript operations view that consumes this surface.
- Hosted Go CI must be observed green before citing CI success.
