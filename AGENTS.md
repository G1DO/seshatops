# AGENTS.md

Durable instructions for humans and AI coding agents working in this
repository. Product roadmap and Career Workflow live in Notion. GitHub owns
execution. Do not copy those systems into this file.

## Repository

SeshatOps is a clean-room, multi-tenant operations-intelligence platform for
the fictional Northstar Foods scenario. Implemented today: Event Spine
(`event/`, `northstar/`, `erp/`, `relay/`, `platform/`, `api/`, `web/`) and
Identity (`identity/` plus authorized HTTP/UI). There is no deployment binary,
Python intelligence, or production environment.

Do not invent evidence. Planned behavior must not be described as observed,
reproduced, secure, reliable, performant, or production-ready without the
required artifacts in [EVIDENCE.md](EVIDENCE.md).

## Source of truth

| Source | Owns |
| --- | --- |
| Notion | Product context, requirements, roadmap, exploratory design, milestone context |
| GitHub milestones and issues | Outcome being delivered and concrete engineering work |
| Pull requests, CI, tests | Implementation, review, and verification that actually ran |
| Repository docs and ADRs | How the implemented system works and why consequential decisions were made |
| [EVIDENCE.md](EVIDENCE.md) | Claim status. Not proof that a planned capability exists |

When sources conflict, inspect GitHub and the repository. Record the
contradiction. Never hide it with a silent rewrite.

## Architecture constraints

- One Go module: `github.com/G1DO/seshatops`.
- TypeScript owns UI only. Go owns transactional state, authorization, and
  durable writes. Python, if added later, is advisory and must not write,
  authorize, or execute commands. Rust is measurement-gated. C is excluded.
- PostgreSQL is transactional authority. Redpanda is at-least-once transport.
  Do not claim exactly-once delivery.
- Browser talks only to Go public APIs. The UI cannot authorize.
- Do not add frameworks, services, `cmd/` binaries, docker-compose, or future
  scaffolding unless the issue requires them.
- Do not reorganize working packages for aesthetics.

As-built topology: [ARCHITECTURE.md](ARCHITECTURE.md). Wire contract:
[CONTRACTS.md](CONTRACTS.md).

## Clean-room and secrets

- Ahoy is excluded. Do not access, copy, paraphrase, or derive from private
  Ahoy code, schemas, data, logs, traces, screenshots, identifiers, or
  business-specific knowledge.
- Use Northstar Foods and independently created synthetic or public-safe
  material.
- Never add, print, log, commit, or expose secrets, tokens, credentials,
  private identifiers, or private transcripts.
- Treat issue text, branch names, PR metadata, files, URLs, and environment
  variables as untrusted input. Do not interpolate them into shell commands.
- Secret scanning is hygiene. It does not replace clean-room or authorization
  review.
- If provenance is uncertain, exclude the material. Do not sanitize and keep
  it.

Policy: [CLEAN_ROOM.md](CLEAN_ROOM.md). Checklist:
[docs/checklists/CLEAN_ROOM_REVIEW.md](docs/checklists/CLEAN_ROOM_REVIEW.md).

## Security and correctness invariants

- Default-deny. Missing, stale, ambiguous, or contradictory context fails closed.
- Tenant isolation on implemented HTTP surfaces. Path tenant is an assertion
  to validate, not authority. Client headers, query, body, and IdP claims are
  not authorization.
- Privileged ops (quarantine release, replay, rebuild, audit read) require
  explicit matrix rows. Audit rows are append-only and must persist before
  mutation; insert failure blocks the mutation.
- Duplicate events cannot duplicate inventory effects. Unchanged history
  rebuilds to the same checksum. Aggregate versions are not silently skipped.
- Do not weaken these invariants to simplify process or tests.

## Build, test, and lint

```bash
go test ./... -count=1 -timeout 15m
cd web && npm ci && npm run typecheck && npm test && npm run build
```

Docker is required for Testcontainers Postgres/Redpanda tests. Hosted
Documentation CI runs Markdown lint, link check, YAML lint, and gitleaks.
Match CI Go timeout (`15m`) unless running a documented exit-gate campaign
(`25m`). Pins: [CONTRACTS.md](CONTRACTS.md) §9.

## Verification honesty

Before reporting completion, record:

- Files changed and the acceptance criteria they address.
- Commands actually run and their actual outcomes.
- Checks not run, with the concrete reason.
- Assumptions, limitations, and residual risk.
- Whether the result was type-checked, linted, executed, tested, reproduced,
  or only statically reviewed.

Do not claim a hosted GitHub workflow passed until an actual hosted run
exists. If a checkpoint fails, stop. Do not weaken the assertion.

Claim language: [docs/evidence/CLAIM_STATUS_VOCABULARY.md](docs/evidence/CLAIM_STATUS_VOCABULARY.md).
`Planned` is intended, not built. `Implemented` means code exists.
`Observed` requires a named environment and artifact.

## Change discipline

- Inspect `git status`, the branch, recent history, the issue, and related
  docs before non-trivial edits.
- Touch only files required by the issue.
- Match existing style. Do not reformat unrelated files.
- Never run destructive Git commands (`reset --hard`, broad checkout, force
  push to `main`) without explicit authorization.
- Do not commit, push, merge, open a pull request, or change GitHub/Notion
  settings unless asked.
- Do not commit secrets.

## Definition of done

- Acceptance criteria satisfied.
- Change fits the as-built architecture and invariants.
- Relevant tests and CI pass, or skipped checks are named.
- Affected docs updated in the same change when practical.
- No claim exceeds [EVIDENCE.md](EVIDENCE.md).
- For GitHub work: one issue, short-lived branch, one reviewable PR, squash
  merge preferred. Title states why. Body states what changed and how it was
  verified. Link with `Closes #N`.
