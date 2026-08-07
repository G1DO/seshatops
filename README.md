# SeshatOps

> Secure Operations Intelligence and Action Control Plane

From business events to evidence-backed decisions and human-approved actions.

## What it is

SeshatOps is a clean-room, multi-tenant operations-intelligence platform that consumes ERP events, reconstructs replayable operational state, predicts stockout risk, explains recommendations with permission-aware evidence, and executes only authorized, human-approved, idempotent commands.

The complete product loop is:

> Observe → Reconstruct → Predict → Explain → Propose → Authorize → Approve → Execute → Audit → Replay

## Public scenario

All public narrative and demos use **Northstar Foods**, a fictional manufacturer and distributor, backed by a standalone synthetic ERP. The project must work with no private production system.

## Status

**M0 documentation and repository-governance scope is complete in this reviewed branch, pending the normal Issue #10 review and merge workflow. Implementation has not started.**

This repository currently holds the Milestone M0 documentation and repository-governance artifacts (product constitution, clean-room policy, logical architecture boundaries, evidence protocols, workflow instructions, and documentation CI). No application code, services, databases, brokers, or runtime scaffolding are present. Planned capabilities must not be read as completed or measured results.

Repository contribution rules are defined in [AGENTS.md](AGENTS.md), pull requests use the [PR template](.github/pull_request_template.md), and documentation hygiene is checked by [Documentation CI](.github/workflows/documentation-ci.yml). The [M0 repository-governance review](docs/reviews/M0_REPOSITORY_GOVERNANCE_REVIEW.md), [integrated constitution review](docs/reviews/M0_INTEGRATED_CONSTITUTION_REVIEW.md), and [M0 completion summary](docs/reviews/M0_COMPLETION_SUMMARY.md) record the reviewed state, evidence boundaries, limitations, and follow-ups.

## Product constitution

See [PRODUCT.md](PRODUCT.md) for users, hero workflow, capability boundaries, the full product loop, success criteria, and explicit non-goals.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for logical system topology, language ownership, trust boundaries, and storage responsibilities. Planned boundaries only — not an implementation or deployment claim.

See [EVENT_MODEL.md](docs/architecture/EVENT_MODEL.md), [COMMAND_MODEL.md](docs/architecture/COMMAND_MODEL.md), and the [Issue #4 ADRs](docs/adrs/) for the planned correctness model.

## Security model

See [THREAT_MODEL.md](docs/security/THREAT_MODEL.md) and [AUTHORIZATION_MODEL.md](docs/security/AUTHORIZATION_MODEL.md) for the planned threat, tenant-isolation, authorization, approval, service-identity, retrieval, audit, and receipt boundaries. See the [M0 security-model review](docs/reviews/M0_SECURITY_MODEL_REVIEW.md) for documentation traceability and recorded follow-ups. These documents define future requirements only; they do not claim implemented security controls.

## Intelligence evaluation

See the [forecasting evaluation protocol](docs/intelligence/FORECASTING_EVALUATION_PROTOCOL.md), [governed-RAG evaluation protocol](docs/intelligence/GOVERNED_RAG_EVALUATION_PROTOCOL.md), and [M0 intelligence-evaluation review](docs/reviews/M0_INTELLIGENCE_EVALUATION_REVIEW.md). These are planned evaluation requirements; no runtime evaluation result is claimed.

## Operational evidence

See the [claim-status vocabulary](docs/evidence/CLAIM_STATUS_VOCABULARY.md), [security evidence protocol](docs/evaluation/SECURITY_EVIDENCE_PROTOCOL.md), [reliability and recovery evidence protocol](docs/evaluation/RELIABILITY_RECOVERY_EVIDENCE_PROTOCOL.md), [performance evidence protocol](docs/evaluation/PERFORMANCE_EVIDENCE_PROTOCOL.md), [fault campaign matrix](docs/evaluation/FAULT_CAMPAIGN_MATRIX.md), [experiment report template](docs/evaluation/templates/EXPERIMENT_REPORT.md), and [M0 operational evidence review](docs/reviews/M0_OPERATIONAL_EVIDENCE_REVIEW.md). These define future evidence requirements; no runtime experiment or operational claim is made.

## Roadmap and evidence ledger

See the [canonical roadmap](ROADMAP.md), [evidence ledger](EVIDENCE.md), and [M0 roadmap/evidence review](docs/reviews/M0_ROADMAP_EVIDENCE_REVIEW.md). These define milestone ownership and future claim governance; they do not prove implementation or measured results.

## Clean-room boundary

**Ahoy is not a dependency** and is not a source of public artifacts. No Ahoy code, schema, data, identifiers, screenshots, or business-specific knowledge belongs in this repository.

See [CLEAN_ROOM.md](CLEAN_ROOM.md) for the enforceable policy and [docs/checklists/CLEAN_ROOM_REVIEW.md](docs/checklists/CLEAN_ROOM_REVIEW.md) for the reusable review checklist.

## License

Licensed under the [Apache License 2.0](LICENSE).
