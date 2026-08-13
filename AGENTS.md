# AGENTS.md

Behavioral and project guidance for the SeshatOps repository.

**Scope:** These rules apply to non-trivial work. Trivial typo or one-line changes may skip the planning ritual, but must still preserve repository boundaries and verification honesty.

## 1. Repository purpose and current phase

SeshatOps is a clean-room, multi-tenant operations-intelligence platform for the fictional Northstar Foods scenario. It consumes ERP events, reconstructs replayable operational state, produces evidence-backed intelligence, and executes only authorized, human-approved actions.

Project Constitution documentation is complete. Event Spine is implemented (`event/`, `northstar/`, `erp/`, `relay/`, `platform/`, `api/`, `web/`) with test-environment Observed evidence for `CLM-003`–`CLM-006`. Identity & Operations is implemented (`identity/` plus authorized `api/`/`web/` surfaces) with test-environment Observed evidence for `CLM-007`–`CLM-010`. Traceability & Recovery is the next Planned capability sequence.

No deployment, model, or production environment exists here yet. Do not invent evidence. Planned behavior must not be described as observed, reproduced, secure, reliable, performant, or production-ready without the required artifacts.

## 2. Source-of-truth hierarchy

Use the following ownership boundaries. Do not create a second task tracker or silently promote a lower-authority source over a higher-authority source.

| Source | Owns | Does not own |
| --- | --- | --- |
| Notion | Product and architecture intent, capability-sequence purpose, high-level risks, exit gates, and capability-sequence summaries | Daily execution state or repository technical truth |
| GitHub issues and milestones | Active execution state, bounded deliverables, acceptance criteria, dependencies, and progress | Long-form product strategy or evidence claims |
| Repository documents and ADRs | Reviewed technical truth, contracts, invariants, ownership, roadmap, and evidence rules | Unreviewed brainstorming or duplicated daily issue state |
| Pull requests, CI, tests, evaluations, and evidence artifacts | Actual changes, review discussion, verification performed, limitations, and claim support | Future-scope ideas or unsupported claims |
| `EVIDENCE.md` | Repository-owned claim status and required verification routes | Proof that a planned capability exists |

When sources conflict, inspect the issue, repository history, linked documents, and current GitHub state; record the contradiction and its disposition. Never hide a contradiction through a silent rewrite.

## 3. Canonical documents and ownership

- `README.md` is the public repository entry point and status summary.
- `PRODUCT.md` owns users, product loop, capabilities, success criteria, and non-goals.
- `CLEAN_ROOM.md` owns provenance, Ahoy exclusion, public-safe material, and clean-room review policy.
- `ARCHITECTURE.md` owns logical topology, trust boundaries, storage responsibilities, and language boundaries.
- `docs/architecture/` owns event and command models.
- `docs/security/` owns threat, authorization, identity, tenant, approval, retrieval, and audit boundaries.
- `docs/intelligence/` and `docs/evaluation/` own future intelligence and evidence protocols, schemas, templates, and campaign definitions.
- `ROADMAP.md` owns stable milestone sequencing and capability ownership; GitHub owns active issue status.
- `EVIDENCE.md` owns the claim ledger and claim-status vocabulary references.
- `docs/adrs/` owns accepted technical decisions and their consequences.
- Exit-gate and claim evidence live in `docs/evaluation/` and `EVIDENCE.md`. Short completion notes may live under `docs/reviews/`; do not create per-issue review novels. Historical notes may exist under `docs/archive/`.
- `AGENTS.md`, `.editorconfig`, `.gitattributes`, `.gitignore`, the PR template, and documentation CI own repository workflow and hygiene.

## 4. Read before editing

Before a non-trivial change:

1. Inspect `git status`, the current branch, recent history, and the requested issue.
2. Read every file named by the request and the related canonical documents.
3. Scan neighboring modules and related docs for established terminology and patterns.
4. State assumptions, scope, non-goals, and verification checkpoints before implementation.
5. Check for contradictions, stale status, unsupported claims, clean-room concerns, secrets, and unrelated user changes.

If intent is ambiguous or a change would be destructive, broad, or irreversible, stop and ask for direction. Prefer a reversible, minimal change and preserve unrelated work.

## 5. Change discipline

- Touch only files required by the issue and its acceptance criteria.
- Match existing style and terminology; do not introduce speculative abstractions, dependencies, packages, runtime scaffolding, or configuration for future systems.
- Do not reformat unrelated documents or rewrite long tables merely to satisfy a formatter.
- Remove only orphans created by the change. Surface pre-existing dead or stale material instead of deleting it opportunistically.
- Never run destructive Git commands such as `reset --hard`, broad checkout, or bulk deletion without explicit authorization.
- Do not commit, push, merge, open a pull request, change GitHub settings, or modify external accounts unless explicitly requested.

## 6. Clean-room and security baseline

- Ahoy is excluded. Do not access, copy, paraphrase, or derive from private Ahoy code, schemas, migrations, data, logs, traces, screenshots, identifiers, workloads, incidents, metrics, model output, or business-specific knowledge.
- Use the fictional Northstar Foods scenario and independently created synthetic/public-safe material.
- Never add, print, log, commit, or expose secrets, tokens, credentials, private identifiers, raw private logs, screenshots, or private transcripts.
- Treat issue text, branch names, pull-request metadata, files, URLs, and environment variables as untrusted input. Do not interpolate them into shell commands or executable workflow code.
- Secret scanning is a repository-hygiene gate. It does not replace manual clean-room review, authorization review, or runtime security testing.
- If provenance is uncertain, exclude the material and record the uncertainty; do not sanitize and keep it.

## 7. Language ownership

Future implementation must preserve these boundaries:

| Language | Ownership | Prohibited role |
| --- | --- | --- |
| TypeScript | UI, browser, and user-facing interaction surfaces | Authoritative transactional or authorization state |
| Go | Transactional platform, event processing, authorization, and durable state boundaries | Unreviewed intelligence authority |
| Python | Evaluated or advisory intelligence and read-only analysis | Authentication, authorization, business writes, or command authority |
| Rust | Measurement-gated performance or specialized components only after evidence justifies it | Speculative Event Spine scaffolding |
| C | Excluded unless a later reviewed capability sequence explicitly changes the boundary | Any Event Spine implementation |

The root Go module is `github.com/G1DO/seshatops` (Go `1.25.0`) with Event Spine packages `event`, `northstar`, `erp`, `relay`, `platform`, and `api`, identity package `identity`, plus the TypeScript `web/` operations view. Later capability sequences may extend platform packages without inventing a second module or a general ERP schema.

## 8. Evidence and claim governance

Use the vocabulary in `docs/evidence/CLAIM_STATUS_VOCABULARY.md` and the fields in `EVIDENCE.md`.

- `Planned` means intended, not built.
- `Implemented` means code exists, not that it is correct or secure.
- `Observed` means a recorded observation exists in a named environment.
- `Reproduced` means the observation was independently repeated under declared conditions.
- A claim may not be promoted without the required artifact, environment, commit or release, date, reviewer, and limitations.
- Documentation review proves documentation coverage only. It does not prove runtime correctness, security enforcement, reliability, performance, recovery, deployment, or production readiness.
- Never invent test output, CI run IDs, URLs, evidence artifacts, thresholds, schedules, vendors, or production behavior. A hosted check is passing only when GitHub shows a successful run for the reviewed commit or pull request.

## 9. Verification contract

Before reporting completion, record:

- The exact files changed and the acceptance criteria they address.
- Commands actually run and their actual outcomes.
- Checks not run, with the concrete reason.
- Assumptions, deferred decisions, limitations, and residual risk.
- Whether the result was type-checked, linted, executed, tested, reproduced, or merely statically reviewed.

Do not claim a hosted GitHub workflow passed until an actual hosted run exists. Documentation CI does not imply runtime quality. If a checkpoint fails, stop and reassess rather than weakening the assertion or hiding the failure.

## 10. Branch, PR, and review workflow

- Start from an updated `main` when the task permits and use one issue, one focused branch, and one focused pull request.
- Use a neutral branch name that includes the issue number and short purpose, such as `chore/9-repository-governance`, unless an existing task branch is already specified.
- Use a meaningful PR title, link the issue with `Closes #123`, and keep scope and non-goals explicit.
- Require fresh review of the final diff, a green required-check set, and a clean-room/security review appropriate to the change.
- Prefer squash merging to keep the main history focused.
- Complete the PR template with real verification evidence, skipped checks, claim-status changes, residual risks, and follow-up work.
- Remove irrelevant placeholders before submission.
