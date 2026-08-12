# Event Spine Contract Review - Issue #21

This document records the documentation-only contract review for Event Spine.
It does not claim application implementation, runtime correctness, tenant
isolation, reliability, performance, deployment, or production readiness.

## Review record

| Field | Value |
| --- | --- |
| Reviewer | Codex implementation/review pass; maintainer review remains a follow-up |
| Date (UTC) | 2026-08-07 |
| Branch | `docs/21-event-spine-contract` |
| Base tip | `5bc9e65` (merged Constitution review head) |
| Scope | Issue #21, Constitution canonical documents, ADR-Q-001 through ADR-Q-003, and new Event Spine contract artifacts |
| Review type | Event Spine implementation-contract, consistency, security, and clean-room review |
| Documentation disposition | Pass with recorded follow-ups |
| Runtime disposition | Not applicable; no runtime project exists and no runtime files were added |

## Sources reviewed

- [GitHub Issue #21](https://github.com/G1DO/seshatops/issues/21)
- [README.md](../../../README.md), [PRODUCT.md](../../../PRODUCT.md), [ARCHITECTURE.md](../../../ARCHITECTURE.md), [ROADMAP.md](../../../ROADMAP.md), [EVIDENCE.md](../../../EVIDENCE.md), and [CLEAN_ROOM.md](../../../CLEAN_ROOM.md)
- [EVENT_MODEL.md](../../architecture/EVENT_MODEL.md), [COMMAND_MODEL.md](../../architecture/COMMAND_MODEL.md), and Project Constitution security models
- ADR-0001, ADR-0002, ADR-Q-001, ADR-Q-002, and ADR-Q-003
- Constitution completion, architecture, correctness, roadmap/evidence, security, and integrated review records

## Acceptance matrix

| Issue #21 criterion | Contract evidence | Disposition |
| --- | --- | --- |
| Resolve ADR-Q-001 through ADR-Q-003 | ADR-0003, amended ADR-0001, ADR-0004, and ADR index dispositions | Covered |
| Define the first event family and envelope | CONTRACTS.md sections 1-3 | Covered |
| Define Redpanda key and ordering policy | CONTRACTS.md section 5 | Covered |
| Define source/outbox and inbox/projection transactions | CONTRACTS.md sections 4 and 6 | Covered |
| Acknowledge only after durable commit | CONTRACTS.md sections 4-7 and amended ADR-0001 | Covered |
| Handle unsupported schema, conflicts, tenant mismatch, poison input, and gaps | CONTRACTS.md section 7 | Covered |
| Define deterministic checksum inputs and normalization | CONTRACTS.md section 8 | Covered |
| Bound the local toolchain | CONTRACTS.md section 9 | Covered |
| Avoid exactly-once claims and runtime promotion | CONTRACTS.md section 10 and unchanged EVIDENCE.md | Covered |
| Permit subsequent Event Spine implementation issues without new architecture | Contract cross-references and explicit later-decision boundary | Covered |

## Reconciliation findings

1. Issue #10 is closed and PR #20 is merged; current README, roadmap, and
   repository instructions now state Constitution complete and Event Spine contract planning active.
2. Historical Constitution review records retain their original facts and receive dated
   subsequent-status notes rather than being rewritten.
3. Notion Constitution remains an external workflow state and was not changed.
4. Constitution's deferred concrete schemas, topic/key policy, persistence boundaries,
   retry rules, and acknowledgement semantics are resolved only for the bounded
   Event Spine slice.
5. No accepted Project Constitution architecture or correctness invariant is contradicted.
6. EVIDENCE.md remains unchanged and all runtime claims remain Planned.

## Security and clean-room review

- Tenant context is validated against the aggregate identity and transport key.
- Conflicting event content, tenant mismatch, unsupported schemas, gaps, and
  impossible state transitions are quarantined without application.
- Failure records retain sanitized diagnostics only; raw payloads and secrets are
  excluded.
- The source relay, Go consumer, and PostgreSQL schemas have separated logical
  responsibilities.
- No authentication/RBAC, operator recovery, or external command path is added.
- All examples and identifiers are synthetic Northstar Foods material.
- No Ahoy code, schema, data, identifiers, logs, screenshots, workloads, or
  business-specific knowledge was accessed or used.

## Verification record

The following checks were performed for this documentation change. Checks that
require hosted tooling or a runtime remain explicitly unperformed:

| Check | Result |
| --- | --- |
| Changed-path allowlist | Passed; exactly 14 planned files changed and no extra paths were found |
| `git diff --check` | Passed; only existing CRLF-to-LF warnings were reported for two edited files |
| Repository-relative Markdown links | Passed with a repository-relative PowerShell link-resolution check |
| Markdown lint, YAML lint, link check, and secret scan | Not run locally; `markdownlint`, `lychee`, `yamllint`, and `gitleaks` are unavailable; hosted Documentation CI remains required |
| Evidence claim status scan | Passed; all 35 claim rows in unchanged `EVIDENCE.md` remain `Planned` |
| Unsupported exactly-once wording scan | Passed; remaining references are explicit prohibitions, rejections, or qualifications |
| Runtime-like file and dependency scan | Passed; no runtime-like files, manifests, containers, or dependency files were added |
| Runtime, typecheck, build, integration, performance, and deployment tests | Not run; no runtime project exists |

No hosted CI result, runtime result, or evidence claim promotion is implied by
this review record. The local checks used `git diff --check`, a changed-path
allowlist comparison, repository-relative Markdown-link resolution, claim-row
status matching, required-contract-term matching, an exactly-once wording scan,
and a runtime-like file/dependency scan.

## Subsequent correction note

The follow-up review identified ambiguity in gap-event re-drive storage,
tenant/key validation order, malformed-event content hashes, strict JCS input
handling, and the architecture diagram's source-publication path. The Event Spine
contract and ADRs now define canonical gap-event retention and disposition,
validation before duplicate acknowledgement, nullable canonical hashes when no
envelope exists, JCS-compatible parsing constraints, and an explicit
source-owned outbox relay path. `EVIDENCE.md` remains unchanged by design; its
claim statuses and historical follow-up wording were not promoted or rewritten.
