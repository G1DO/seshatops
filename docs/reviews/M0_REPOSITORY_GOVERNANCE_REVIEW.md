# M0 Repository Governance Review - Issue #9

This document records the documentation-only implementation and static review for Issue #9, “M0: Establish repository instructions, PR workflow, and documentation CI.” It does not claim application implementation, runtime correctness, security enforcement, performance, reliability, deployment readiness, or clean-room perfection.

## 1. Review record

| Field | Value |
| --- | --- |
| Reviewer | Repository implementation/review pass; maintainer review remains a recorded follow-up |
| Date (UTC) | 2026-08-07 |
| Branch | `chore/9-repository-governance` |
| Base commit | `c7408ee` |
| Reviewed state | Issue #9 working tree after documentation implementation; hosted CI evidence remains pending |
| Scope | Repository instructions, contribution workflow, documentation CI, and surgical status corrections |
| Review type | Documentation implementation and static governance review |
| Documentation result | Pass with recorded follow-ups |
| Hosted CI result | Pending; no hosted run was fabricated |
| Runtime result | Not run by design; no runtime project exists and runtime work is out of scope |

The change remains documentation/governance-only. No application code, package manifest, lockfile, runtime configuration, infrastructure, schema, dashboard, model, dataset, deployment configuration, dependency bot, or GitHub settings mutation was introduced.

## 2. Sources reviewed

### GitHub and repository sources

- GitHub Issue #9 and its acceptance criteria, dependencies, and non-goals.
- Issues #1 through #8 and merged PRs #11 through #18.
- `README.md`, `PRODUCT.md`, `CLEAN_ROOM.md`, `ARCHITECTURE.md`, `ROADMAP.md`, `EVIDENCE.md`, and `LICENSE`.
- `docs/architecture/EVENT_MODEL.md` and `docs/architecture/COMMAND_MODEL.md`.
- `docs/security/THREAT_MODEL.md` and `docs/security/AUTHORIZATION_MODEL.md`.
- `docs/intelligence/`, `docs/evaluation/`, `docs/adrs/`, `docs/checklists/`, and all existing M0 review documents.
- The current working tree and history at base commit `c7408ee`.

### Canonical planning sources

- [SeshatOps - Master Project Blueprint](https://app.notion.com/p/3b40a821b3cc813081f0ea44fd72692a).
- [Workflow - Notion -> GitHub -> Evidence](https://app.notion.com/p/3b40a821b3cc810eadebc2fc9a067000).
- [M0 - Project Constitution](https://app.notion.com/p/3b40a821b3cc81f082c2e5e77d0499ee).
- Live GitHub Issue #9 and repository branch history are authoritative for current execution status.

No private Ahoy repository or private Ahoy artifact was accessed.

## 3. Issue #9 acceptance-criteria coverage

| Criterion | Coverage | Result |
| --- | --- | --- |
| Repository instructions exist | Root `AGENTS.md` defines project purpose, source ownership, clean-room rules, language boundaries, evidence governance, verification, and M0 invariants. | Covered |
| Editor and repository hygiene are defined | `.editorconfig`, `.gitattributes`, and `.gitignore` define text encoding, line endings, binary handling, local-only files, and secret-file behavior. | Covered |
| Pull-request workflow is defined | `.github/pull_request_template.md` requires linked issue, scope, evidence, verification, skipped checks, clean-room review, residual risks, and follow-up work. | Covered |
| Canonical ownership is explicit | `AGENTS.md` and the existing roadmap distinguish Notion intent, GitHub execution state, repository technical truth, and PR/evidence proof. | Covered |
| Read-before-edit and honest verification are required | `AGENTS.md` requires inspection, assumptions, actual commands, skipped-check reasons, and no fabricated runtime or hosted-CI claims. | Covered |
| Documentation CI covers Markdown | Markdown lint runs over `**/*.md` with the existing intentional wide-table exception documented in `.markdownlint.yaml`. | Covered |
| Documentation CI covers links | Lychee checks Markdown links with a read-only GitHub token and only private Notion planning URLs excluded. | Covered |
| Documentation CI covers YAML | Yamllint checks repository YAML, including the workflow and its configuration. | Covered |
| Documentation CI covers secrets | Gitleaks scans full history with comments, artifacts, and summaries disabled; the pinned action source invokes the scanner with `--redact`. | Covered |
| CI is least privilege and supply-chain constrained | Workflow permissions are `contents: read`; actions are pinned to reviewed full commit SHAs; no unsafe trigger or deployment capability exists. | Covered |
| Main remains documentation-only | No runtime files, packages, services, containers, deployment settings, or dependency bots were added. | Covered |

## 4. AGENTS.md review

`AGENTS.md` now records:

- The M0 documentation-only state and Issue #9/Issue #10 ownership.
- Notion, GitHub, repository, PR/CI/evidence, and `EVIDENCE.md` boundaries.
- Canonical document ownership and contradiction handling.
- Read-before-edit, surgical-change, non-destructive Git, and scope rules.
- Clean-room, secret, untrusted-input, and Ahoy-exclusion requirements.
- TypeScript, Go, Python, Rust, and C ownership boundaries.
- Planned, implemented, observed, and reproduced claim distinctions.
- The verification contract and hosted-CI evidence rule.
- Branch, PR, review, and squash-merge conventions.
- The Issue #9 M0 definition of done.

The existing user-provided default guidance was retained in substance and made repository-specific rather than left as placeholder project metadata.

## 5. Canonical source-of-truth review

The repository preserves the existing boundary:

| Source | Ownership recorded |
| --- | --- |
| Notion | Product and architecture intent, milestone purpose, high-level risks, exit gates, and summaries |
| GitHub | Active issue and milestone execution state, dependencies, and progress |
| Repository documents and ADRs | Reviewed technical truth, contracts, invariants, roadmap, and evidence rules |
| Pull requests, CI, tests, evaluations, and artifacts | Actual change, review, verification, limitations, and claim support |
| `EVIDENCE.md` | Repository-owned claim ledger and status vocabulary route |

`ROADMAP.md` remains a stable milestone sequence rather than a second task tracker. Its stale Issue #8/current-deliverable statements were corrected to reflect Issues #1 through #8 complete, Issue #9 active, and Issue #10 remaining. `EVIDENCE.md` required no correction.

## 6. Clean-room and secret handling

- No Ahoy repository or private Ahoy artifact was accessed.
- No private code, schema, migration, data, log, trace, screenshot, identifier, workload, incident, metric, model output, or production behavior was used.
- The repository continues to use the fictional Northstar Foods scenario and generic public-safe terminology.
- `.gitignore` excludes local environment files while explicitly permitting `.env.example` and `.env.sample`.
- The workflow has no cloud credentials, deployment credentials, write permissions, or secret values.
- Gitleaks is a narrow repository-hygiene gate. It does not replace manual clean-room review, authorization review, or runtime security testing.
- Gitleaks comments, artifacts, and summaries are disabled in the workflow. The pinned action source was inspected and confirmed to invoke the scanner with `--redact`. No real secret was used for verification.

## 7. Language ownership

The governance file preserves the architecture boundary:

| Language | Allowed ownership | Boundary |
| --- | --- | --- |
| TypeScript | UI, browser, and user-facing interaction | Not authoritative for transactional or authorization state |
| Go | Transactional platform, event processing, authorization, and durable state | Not an unreviewed intelligence authority |
| Python | Evaluated/advisory intelligence and read-only analysis | Never authentication, authorization, business writes, or command authority |
| Rust | Measurement-gated specialized or performance components | No speculative M0 scaffolding |
| C | Excluded unless a later reviewed milestone changes the boundary | No M0 implementation |

No language workspace was created by Issue #9.

## 8. Git and pull-request workflow

The repository now requires one focused issue/branch/PR, explicit scope and non-goals, a linked closing issue, fresh final-diff review, clean-room review, actual verification evidence, and preferred squash merging. It prohibits unrequested commits, pushes, merges, pull requests, GitHub settings changes, and destructive Git operations.

The current working branch is `chore/9-repository-governance`. No commit, push, merge, pull request, or GitHub settings mutation was performed by this implementation.

## 9. Pull-request template review

The template requires:

- Linked issue and summary.
- Scope and explicit non-goals.
- Changed files/components.
- Acceptance-criteria coverage.
- Correctness, security, and clean-room impact.
- Evidence claim IDs and status changes.
- Actual verification outcomes and checks not run.
- Content and evidence review before submission.
- Documentation/ADR updates.
- Residual risks and follow-up work.

It contains no assertion that unexecuted tests, CI, or runtime checks passed.

## 10. Documentation CI and supply-chain review

### Triggers and permissions

- Pull requests.
- Pushes to `main`.
- Manual dispatch.
- Top-level `permissions: contents: read`.
- No `pull_request_target`, write permission, deployment step, artifact upload, cache, cloud credential, or PR-text execution.
- Each job uses a ten-minute timeout and checks out with `persist-credentials: false`.

### Action pinning

All third-party actions use full immutable commit SHAs with adjacent release comments:

| Action | Pin | Release reference |
| --- | --- | --- |
| `actions/checkout` | `d23441a48e516b6c34aea4fa41551a30e30af803` | v6.1.0 |
| `DavidAnson/markdownlint-cli2-action` | `21c1be1b93ad9ed58fa840aacc3f279cde2a72ff` | v24.2.0 |
| `lycheeverse/lychee-action` | `e7477775783ea5526144ba13e8db5eec57747ce8` | v2.9.0 |
| `ibiqlik/action-yamllint` | `2576378a8e339169678f9939646ee3ee325e845c` | v3.1.1 |
| `gitleaks/gitleaks-action` | `e0c47f4f8be36e29cdc102c57e68cb5cbf0e8d1e` | v3.0.0 |

Pins were recorded from the reviewed upstream release references. Future action updates require a fresh pin review.

### Tool coverage and exclusions

- Markdown lint: all Markdown files, with only `MD013` disabled because existing evidence tables contain intentional wide rows.
- Link checking: all Markdown files, with a 20-second timeout and two retries.
- Link exclusions: only `https://app.notion.com/p/` planning pages that require private access. GitHub links remain in scope and receive the read-only `GITHUB_TOKEN`.
- YAML lint: repository YAML with explicit handling for GitHub Actions `on` keys, document-start conventions, and immutable action-SHA line length.
- Secret scan: full Git history, with comments, artifacts, and summaries disabled.

The configuration intentionally does not add broad URL exclusions, a secret allowlist, a Gitleaks license, dependency automation, package installation, or runtime-specific checks.

## 11. Verification performed

The following checks are required for the implementation review and are recorded with their actual outcomes below:

| Check | Outcome |
| --- | --- |
| `git diff --check` | Passed with exit code 0 |
| Repository-relative Markdown link scan | Passed after implementation; no broken repository-relative links found |
| Markdownlint | Not available locally; hosted CI required |
| Lychee | Not available locally; hosted CI required |
| Yamllint | Not available locally; hosted CI required |
| Gitleaks | Not available locally; hosted CI required; no real secret used |
| Pinned Gitleaks action source | Passed: the pinned action supports the configured comment/artifact/summary controls and invokes Gitleaks with `--redact` |
| TOML parse | Passed with Python `tomllib` for `.lychee.toml` |
| YAML parser | Not available locally because PyYAML is not installed; hosted yamllint required |
| Static workflow review | Passed: eight action references use full 40-character SHAs; no `pull_request_target`, write permission, external secret reference, runtime manifest, or container manifest found |
| `git ls-files --eol` | Touched tracked files are LF; 25 unrelated pre-existing tracked Markdown files remain CRLF and were not reformatted |
| Local credential-pattern heuristic | Passed; no common credential-pattern matches found |
| Runtime tests/typecheck/build/application lint | Not run; no runtime project exists and Issue #9 excludes runtime work |

Hosted CI evidence remains **Pending** until a real GitHub Actions run verifies all four jobs, private-link behavior, and secret-scanner behavior. No run ID or URL is claimed here.

## 12. Assumptions and unresolved items

| Item | Disposition |
| --- | --- |
| Existing root `AGENTS.md` was untracked | Treated as user-provided default guidance and updated in place as requested. |
| Existing Markdown line widths | Preserved; only `MD013` is disabled because wide evidence tables are intentional. |
| Private Notion links | Narrowly excluded because they are planning-source links and require authenticated access. |
| Private GitHub links | Remain in scope and are checked with the read-only workflow token. |
| Gitleaks organization licensing | The current repository owner is a personal GitHub account; no license was added. A future organization transfer may require an explicit license decision. |
| Hosted CI | Pending and must not be represented as green until an actual run exists. |
| Issue #10 integrated review | Remains the owning follow-up for final M0 integration and milestone exit. |

## 13. Residual risk and deferred work

- The local environment lacks the four hosted documentation tools, so their real outputs remain unverified until GitHub Actions runs.
- Network availability, private GitHub-token access, and the upstream action implementations are not proven by local static inspection.
- Gitleaks output safety was verified for the pinned action source: it supports the configured output controls and passes `--redact`; future action updates require the same source/output review.
- Documentation CI does not prove application correctness, runtime security, authorization, reliability, recovery, performance, deployment, production readiness, or final clean-room independence.
- Issue #10 owns the integrated M0 review and any cross-document contradictions that remain after the hosted checks.

## 14. Result

**Pass with recorded follow-ups.** The repository-governance implementation covers the Issue #9 documentation and CI scope without introducing runtime work. Hosted CI evidence and the final integrated M0 review remain pending.
