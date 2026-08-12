# Event Spine Event Contract and Fixture Review - Issue #22

This document records the Issue #22 implementation review for the executable Event Spine
event contract and deterministic Northstar Foods fixture. It does not claim
outbox, broker, projection, security enforcement, or production readiness.

## Review record

| Field | Value |
| --- | --- |
| Reviewer | Implementation pass on branch `feat/22-event-contract-fixtures`; maintainer review remains a follow-up |
| Date (UTC) | 2026-08-08 |
| Branch | `feat/22-event-contract-fixtures` |
| Scope | Issue #22 only: JSON/JCS event contract, Northstar fixture, tests, provenance, Go CI |
| Review type | Contract implementation, clean-room, and verification honesty review |
| Runtime disposition | Library and unit tests only; no PostgreSQL, Redpanda, outbox, inbox, or services |

## Wire-format disposition

Notion Event Spine deliverables and GitHub Issue #22 still mention a “Protobuf” event
contract. Issue #21 accepted [CONTRACTS.md](../../CONTRACTS.md) and
[ADR-0003](../adrs/0003-event-envelope-and-schema-compatibility.md), which
reject Protobuf/Avro for Event Spine and require strict UTF-8 JSON with RFC 8785 JCS
content identity.

**Disposition:** Implement JSON + JCS. Treat the Protobuf wording as stale
relative to the accepted Event Spine contract. No `.proto` files or schema registry were
added.

## Acceptance matrix

| Issue #22 criterion | Evidence | Disposition |
| --- | --- | --- |
| Concrete Event Spine event contains all #21 envelope fields | `event.Envelope` / `event.Parse` and tests | Covered (JSON, not Protobuf) |
| `event_id`, aggregate identity, aggregate-version semantics documented and testable | Package docs + validation tests | Covered |
| Same declared seed generates equivalent fixtures and logical event history | `northstar` tests + goldens | Covered |
| Tenant/aggregate/event/schema/time/producer/lineage/trace per #21 | Parse/validate field coverage | Covered |
| Compatibility accept/reject cases | `TestCompatibilityRejects`, `TestParseValidV1` | Covered |
| Same `event_id` with different content is an integrity violation | `TestIdentityConflict` | Covered |
| Synthetic provenance recorded | [SYNTHETIC_DATA_PROVENANCE.md](../events/SYNTHETIC_DATA_PROVENANCE.md) | Covered |
| No Ahoy-derived public artifact | Clean-room review of fixtures/identifiers | Covered |

## Verification record

| Check | Result |
| --- | --- |
| `go test ./event ./northstar` | Passed locally |
| `gofmt` on changed Go packages | Applied |
| Hosted Go CI | Not claimed until a GitHub Actions run exists for the reviewed commit |
| Documentation CI (markdown/links/yaml/secrets) | Not run locally in this pass; hosted Documentation CI remains required |
| `EVIDENCE.md` claim promotion | None; `CLM-005` remains `Planned` |

## Clean-room review

- Fixture and example identifiers are fictional Northstar Foods / CONTRACTS.md material.
- No Ahoy code, schemas, data, identifiers, logs, screenshots, or business rules were used.
- Provenance records origin, generation method, license, reproducibility, and independence.

## Remediation note

A follow-up review pass requested:

1. Restore documentation-tooling `.gitignore` entries while keeping Go build ignores.
2. Enforce UUIDv4 (RFC 4122 variant) and calendar-valid RFC 3339 `Z` timestamps.
3. Reject empty Northstar seeds instead of aliasing them to `DefaultSeed`.
4. Update ADR-0003 status to reflect the Issue #22 library without claiming runtime.

Those fixes are included on this branch. Hosted CI success is still not claimed
until GitHub shows a green run for the reviewed commit.

A later pass added a pre-decode Unicode/JCS gate: invalid UTF-8 bytes and
unpaired JSON `\u` surrogate escapes are rejected as `ErrMalformed` before
`encoding/json` can rewrite them to U+FFFD. Literal U+FFFD (`"\ufffd"`) remains
accepted. `Validate` and JCS serialization also require `utf8.ValidString` on
envelope string fields.

## Residual risk and follow-ups

- Update Notion Event Spine / Issue #22 wording from Protobuf to JSON when convenient.
- Hosted Go CI must be observed green before citing CI success.
- Contract unit tests do not prove transport ordering or projection enforcement;
  `CLM-005` stays Planned until later Event Spine evidence exists.
- Issue #23 may consume `event` and `northstar` without inventing a new envelope.
