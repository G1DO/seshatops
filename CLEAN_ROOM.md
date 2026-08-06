# CLEAN_ROOM — SeshatOps Clean-Room Policy

**Status:** Enforceable policy. This document defines the public/private boundary for SeshatOps. It does not claim that every future artifact has already been reviewed.

**Owns:** Clean-room boundary, provenance rules, forbidden and permitted sources, AI-assisted work rules, review procedures, accidental-disclosure response, and review-evidence expectations.

**Companion checklist:** [docs/checklists/CLEAN_ROOM_REVIEW.md](docs/checklists/CLEAN_ROOM_REVIEW.md)

## 1. Purpose

SeshatOps must remain a standalone public product that is independently understandable, buildable, and demonstrable without any private production system.

The clean-room boundary exists so that:

- public artifacts never depend on private Ahoy systems, code, data, rules, or configuration;
- reviewers can explain every public artifact from SeshatOps sources alone;
- optional future private adapters stay outside this repository and never become a public dependency.

Ahoy may be named only as an **excluded** private system. It is never a source, specification, demo input, dataset, schema, screenshot, or runtime dependency for SeshatOps.

## 2. Scope

This policy applies to everything that can become public with the repository, including:

- source, docs, issues, pull requests, comments, commit messages, and branch names;
- fixtures, examples, screenshots, datasets, prompts, models, and evaluation corpora;
- CI logs, release notes, demo materials, and portfolio claims tied to this repository.

Any optional private adapter belongs in a separate private repository and is out of scope for SeshatOps public artifacts.

## 3. Forbidden source materials

Do not inspect, copy, paraphrase, reverse-engineer, or commit any of the following into SeshatOps:

1. Ahoy or other private application code, libraries, configs, or infrastructure definitions.
2. Private schemas, migrations, API contracts, or data dictionaries.
3. Private operational data, logs, traces, metrics dumps, or backups.
4. Screenshots, recordings, or exports from private systems.
5. Production identifiers, hostnames, account IDs, tenant IDs, or internal URLs.
6. Private business-specific rules, recipes, prices, customers, suppliers, or process knowledge.
7. Production secrets, credentials, tokens, certificates, or environment files from private systems.
8. Raw AI conversations, chat exports, or prompt histories that contain private context.
9. Translated or “sanitized” versions of the above that still encode private structure, naming, or rules.

This list is category-based. Do **not** commit a private denylist of real identifiers into this repository.

## 4. Permitted sources

The following may be used when their provenance is clear:

1. Independently authored SeshatOps documentation and code created inside this project.
2. The **Northstar Foods** fictional public scenario defined in [PRODUCT.md](PRODUCT.md).
3. Generated synthetic data created for SeshatOps, with provenance recorded (see §5).
4. Properly licensed public datasets, standards, RFCs, and documentation.
5. Generic industry knowledge that does **not** encode a private system’s schema, identifiers, business rules, or proprietary structure.
6. Public open-source dependencies chosen for SeshatOps on their own merits.

If material could only have been produced by studying a private system, it is not permitted—even if rewritten.

## 5. Synthetic-data provenance

Every synthetic dataset, fixture set, or evaluation corpus committed here must record:

| Field | Requirement |
| --- | --- |
| Origin | Created for SeshatOps / Northstar Foods, or licensed public source |
| Generation method | How it was produced (script, manual authoring, generator version) |
| License | Redistribution terms for inclusion in this repository |
| Reproducibility | Enough detail for an independent party to regenerate or explain it |
| Independence | Explicit statement that it was not derived from private production data |

Northstar Foods is the only public scenario domain. Do not invent parallel “thinly veiled” private domains. Do not reverse-engineer private systems to create synthetic stand-ins.

## 6. AI-assisted work

AI tools may help author SeshatOps artifacts only when:

1. Prompts and context exclude forbidden source materials (§3).
2. Private code, data, screenshots, logs, or identifiers are not pasted into the session that produces repository content.
3. AI output is treated as untrusted until a human clean-room review accepts it.
4. Uncertain provenance in AI output defaults to exclusion (§8).

Do not commit raw private-context AI transcripts. Summaries of SeshatOps-only work are allowed when they contain no private material.

## 7. Screenshots, examples, fixtures, names, schemas, and terminology

| Artifact | Rule |
| --- | --- |
| Screenshots / recordings | Northstar Foods or generic SeshatOps UI only; no private hostnames, URLs, or identifiers in chrome or content |
| Examples and fixtures | Independently invented fictional values; explainable without private context |
| Names | Fictional (Northstar Foods and related demo entities); never real customer, supplier, or employee names from private systems |
| Schemas and field names | Designed for SeshatOps public contracts; do not mirror private schemas |
| Terminology | Generic operations vocabulary or SeshatOps-defined terms; do not import private-only business vocabulary |
| Secrets in examples | Placeholders only (for example `REPLACE_ME`); never real credentials |

Public artifacts must remain independently explainable and reproducible from this repository’s own sources.

## 8. Uncertain provenance

If authorship, origin, or independence cannot be established with confidence:

1. **Exclude** the material by default.
2. Document the uncertainty in the review record.
3. Do not “sanitize and keep.”
4. Replace only with independently created permitted material.

Doubt favors exclusion.

## 9. Review procedures

Use [docs/checklists/CLEAN_ROOM_REVIEW.md](docs/checklists/CLEAN_ROOM_REVIEW.md). Reviews are manual in this milestone; automated DLP or secret-scanning workflows are out of scope here.

### Pre-commit

Before committing:

1. Confirm every new or changed path has a known permitted source (§4) or synthetic provenance (§5).
2. Confirm no forbidden materials (§3) and no secrets.
3. Spot-check screenshots, fixtures, names, schemas, and terminology (§7).
4. If AI assistance was used, confirm clean-room inputs (§6).
5. Exclude anything with uncertain provenance (§8).

### Pre-PR

Before opening or merging a pull request:

1. Complete a clean-room review checklist for the PR scope.
2. Run a repository search over changed paths for exclusionary terms, secret-like strings, and private-looking identifiers (category search only; no private denylist file).
3. Confirm the PR does not introduce application or documentation dependence on Ahoy or other private systems.
4. Record reviewer, date, commit, scope, findings, remediation, and result on the checklist.

### Pre-public-release

Before making the repository or a release public:

1. Complete a full-repository clean-room review checklist (pre-public-release type).
2. Review git history, issues, PR discussion, and release artifacts for forbidden materials and secrets.
3. Confirm public demo and docs run on Northstar Foods / synthetic inputs alone.
4. Remediate any findings before publication; record evidence of the release review.

## 10. Accidental disclosure and remediation

If forbidden or sensitive material may have entered the repository:

1. **Contain** — Stop further commits, pushes, merges, and distribution of the affected artifacts.
2. **Assess** — Identify what was exposed, where (working tree, commit, PR, issue, CI log), and who could have accessed it.
3. **Remove** — Delete from the working tree and open changes immediately.
4. **Remediate history** — If committed or pushed, rewrite or otherwise remove the content from git history using maintainer-approved history remediation; treat force-push as an explicit maintainer decision.
5. **Rotate credentials** — If secrets or credentials were exposed, rotate them and revoke old credentials before considering the incident closed.
6. **Record** — Document findings, remediation steps, commits, and rotations in the review or incident record.
7. **Re-review** — Complete a clean-room checklist on the remediated scope before continuing work or release.

## 11. Evidence of review

A completed [clean-room review checklist](docs/checklists/CLEAN_ROOM_REVIEW.md) with reviewer, date, commit, scope, findings, remediation, and result is the evidence that a review **occurred**.

A checked box is **not** proof that no undiscovered issue exists. Residual risk remains with any manual review.

## 12. Roles and responsibility

Even while this repository is solo-maintained:

| Role | Responsibility |
| --- | --- |
| Author | Ensures provenance of contributed material; does not introduce forbidden sources |
| Reviewer | Completes the clean-room checklist; may be the same person after a second, time-separated pass until collaborators exist |
| Release Approver | Confirms pre-public-release review passed before publication; may be the same maintainer with an explicit review record |

The Author remains accountable for the provenance of what they introduce. Review evidence does not transfer that accountability away.
