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

**M0 is complete. M1 Event Spine is active: the reviewed contract exists,
Issue #22 adds the executable JSON event library and deterministic Northstar
Foods fixture, Issue #23 adds the synthetic ERP source transaction with a
pending transactional outbox, and Issue #24 adds the source-owned outbox relay
that publishes exact stored bytes to Redpanda at least once.**

This repository holds the completed M0 documentation, the reviewed M1
event-spine contract in [CONTRACTS.md](CONTRACTS.md), Go packages under
`event/`, `northstar/`, `erp/`, and `relay/`, the source/outbox persistence note
in [SOURCE_OUTBOX_PERSISTENCE.md](docs/architecture/SOURCE_OUTBOX_PERSISTENCE.md),
and the relay note in [OUTBOX_RELAY.md](docs/architecture/OUTBOX_RELAY.md).
Projection consumer and service runtime are not present yet. Planned
capabilities must not be read as completed or measured results.

Repository contribution rules are defined in [AGENTS.md](AGENTS.md), pull requests use the [PR template](.github/pull_request_template.md), documentation hygiene is checked by [Documentation CI](.github/workflows/documentation-ci.yml), and Go package tests are checked by [Go CI](.github/workflows/go-ci.yml). The [M0 repository-governance review](docs/reviews/M0_REPOSITORY_GOVERNANCE_REVIEW.md), [integrated constitution review](docs/reviews/M0_INTEGRATED_CONSTITUTION_REVIEW.md), [M0 completion summary](docs/reviews/M0_COMPLETION_SUMMARY.md), [M1 event-contract fixture review](docs/reviews/M1_EVENT_CONTRACT_FIXTURE_REVIEW.md), [M1 source/outbox review](docs/reviews/M1_SOURCE_OUTBOX_REVIEW.md), and [M1 outbox relay review](docs/reviews/M1_OUTBOX_RELAY_REVIEW.md) record reviewed state, evidence boundaries, limitations, and residual risk.

## Product constitution

See [PRODUCT.md](PRODUCT.md) for users, hero workflow, capability boundaries, the full product loop, success criteria, and explicit non-goals.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for logical system topology, language ownership, trust boundaries, and storage responsibilities. Planned boundaries only — not an implementation or deployment claim.

See [EVENT_MODEL.md](docs/architecture/EVENT_MODEL.md), [COMMAND_MODEL.md](docs/architecture/COMMAND_MODEL.md), [CONTRACTS.md](CONTRACTS.md), and the [ADRs](docs/adrs/) for the planned correctness model and concrete M1 contract.

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
