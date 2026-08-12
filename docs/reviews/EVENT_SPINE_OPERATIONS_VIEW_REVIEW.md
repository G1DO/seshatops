# Event Spine Operations View Review - Issue #28

This document records the Issue #28 implementation review for the minimal
TypeScript operations view over the Issue #27 Go REST/SSE projection surface.
It does not claim authentication enforcement, exactly-once SSE delivery,
production readiness, deployment completeness, or hosted CI success without a
recorded GitHub Actions run.

## Review record

| Field | Value |
| --- | --- |
| Reviewer | Implementation pass on branch `feat/28-operations-view`; maintainer review remains a follow-up |
| Date (UTC) | 2026-08-12 |
| Branch | `feat/28-operations-view` |
| Scope | Issue #28 only: `web/` operations screen, REST/SSE client, connection states, tests, docs |
| Review type | Presentation-boundary, reconnect honesty, clean-room, and verification honesty review |
| Runtime disposition | Vite/React TypeScript package with mocked fetch/EventSource tests; no deployment binary or auth |

## Acceptance matrix

| Issue #28 criterion | Evidence | Disposition |
| --- | --- | --- |
| Initial projection exclusively through Go REST | `projection.integration.test.tsx` REST load; `fetchSnapshot` | Covered |
| Committed change visible through Go SSE path | SSE update integration test; `openProjectionStream` | Covered |
| Deterministic before/after inventory presentation | Store + UI tests for 10→8 presentation transition | Covered |
| Version/freshness metadata rendered without inventing business rules | OperationsView freshness section; no checksum/qty math in `web/` | Covered |
| SSE loss/disconnection explicit (not shown as live current) | `data-authoritative-live`; disconnected/stale banners | Covered |
| Reconnect/refetch converges via REST | Reconnect integration test | Covered |
| Loading and API error states explicit and testable | Loading/error component and integration tests | Covered |
| TypeScript is not authoritative for inventory/business rules | Boundary review below; presentation-only previous qty | Covered |
| Synthetic Northstar data only | `web/src/fixtures/northstar.ts`; no Ahoy material | Covered |

## Boundary review

| Check | Result |
| --- | --- |
| Browser → PostgreSQL | Absent; no DB drivers in `web/` |
| Browser → Redpanda/broker | Absent; no broker clients in `web/` |
| Inventory/business rule ownership | Quantities, versions, checksums displayed from API payloads only |
| Duplicate SSE | Identical payloads do not invent a second transition |

## Verification record

| Check | Result |
| --- | --- |
| `cd web && npm test` | Passed locally on 2026-08-12 (15 tests) |
| `cd web && npm run typecheck` | Passed locally on 2026-08-12 |
| `cd web && npm run build` | Passed locally on 2026-08-12 |
| Hosted Web CI | Not claimed until a GitHub Actions run exists for the reviewed commit |
| Documentation CI | Not claimed until a hosted run exists |
| Full event-spine live demo binary | Not present; demo wiring documented against `api.Handler()` |
| `EVIDENCE.md` / `CAP-008` claim promotion | None; `CAP-008` remains `Planned` pending authorized live-view evidence |

## Clean-room review

- Identifiers and fixture material remain fictional Northstar Foods /
  CONTRACTS.md artifacts.
- No Ahoy code, schemas, data, identifiers, logs, screenshots, or business rules
  were used.
- No secrets, credentials, or private hostnames were introduced.

## Residual risk and follow-ups

- SSE frames may drop; REST remains authoritative and reconnect is required.
- No long-running deployment binary is provided; local demo requires wiring
  `api.NewServer` on `:8080` and using the Vite `/v1` proxy (default empty
  `VITE_API_BASE_URL`).
- Authentication and tenant authorization enforcement remain Identity.
- Security/identity/recovery UX is explicitly deferred.
- Hosted Web CI must be observed green before citing CI success.
- Issue #30 owns broader UI/runtime integration that consumes this view.
