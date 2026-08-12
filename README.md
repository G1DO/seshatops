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

**Project Constitution is complete. Event Spine exit-gate evidence is recorded (Issue #30):
Issues #21–#29 delivered the contract, JSON event library and Northstar
fixture, synthetic ERP source/outbox, Redpanda relay, inventory projection
consumer with failure/restart safety, read-only REST/SSE API, TypeScript
operations view, and duplicate/rebuild checksum proofs. `CLM-003`–`CLM-006`
are Observed for the declared test environment only.**

This repository holds the completed Project Constitution documentation, the reviewed
Event Spine contract in [CONTRACTS.md](CONTRACTS.md), Go packages under
`event/`, `northstar/`, `erp/`, `relay/`, `platform/`, and `api/`, the TypeScript
`web/` operations view, the source/outbox
persistence note in
[SOURCE_OUTBOX_PERSISTENCE.md](docs/architecture/SOURCE_OUTBOX_PERSISTENCE.md),
the relay note in [OUTBOX_RELAY.md](docs/architecture/OUTBOX_RELAY.md), the
consumer note in
[INVENTORY_PROJECTION_CONSUMER.md](docs/architecture/INVENTORY_PROJECTION_CONSUMER.md),
the projection read API note in
[PROJECTION_READ_API.md](docs/architecture/PROJECTION_READ_API.md),
the operations view note in
[OPERATIONS_VIEW.md](docs/architecture/OPERATIONS_VIEW.md), the rebuild
proof note in
[PROJECTION_REBUILD.md](docs/architecture/PROJECTION_REBUILD.md), and the
exit-gate campaign artifacts under
[EVENT_SPINE_EXIT_GATE_PROCEDURE.md](docs/evaluation/EVENT_SPINE_EXIT_GATE_PROCEDURE.md) and
[EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md).
A long-running deployment service is not present yet. Planned capabilities must
not be read as completed or measured results beyond the linked evidence.

Repository contribution rules are defined in [AGENTS.md](AGENTS.md), pull requests use the [PR template](.github/pull_request_template.md), documentation hygiene is checked by [Documentation CI](.github/workflows/documentation-ci.yml), and Go package tests are checked by [Go CI](.github/workflows/go-ci.yml). The [Project Constitution governance review](docs/reviews/PROJECT_CONSTITUTION_GOVERNANCE_REVIEW.md), [integrated constitution review](docs/reviews/PROJECT_CONSTITUTION_INTEGRATED_REVIEW.md), [Project Constitution completion summary](docs/reviews/PROJECT_CONSTITUTION_COMPLETION_SUMMARY.md), [Event Spine event-contract fixture review](docs/reviews/EVENT_SPINE_EVENT_CONTRACT_FIXTURE_REVIEW.md), [Event Spine source/outbox review](docs/reviews/EVENT_SPINE_SOURCE_OUTBOX_REVIEW.md), [Event Spine outbox relay review](docs/reviews/EVENT_SPINE_OUTBOX_RELAY_REVIEW.md), [Event Spine inventory projection review](docs/reviews/EVENT_SPINE_INVENTORY_PROJECTION_REVIEW.md), [Event Spine consumer failure safety review](docs/reviews/EVENT_SPINE_CONSUMER_FAILURE_SAFETY_REVIEW.md), [Event Spine projection read API review](docs/reviews/EVENT_SPINE_PROJECTION_READ_API_REVIEW.md), [Event Spine operations view review](docs/reviews/EVENT_SPINE_OPERATIONS_VIEW_REVIEW.md), [Event Spine projection rebuild review](docs/reviews/EVENT_SPINE_PROJECTION_REBUILD_REVIEW.md), [event-spine exit-gate campaign review](docs/reviews/EVENT_SPINE_EXIT_GATE_CAMPAIGN_REVIEW.md), and [Event Spine completion summary](docs/reviews/EVENT_SPINE_COMPLETION_SUMMARY.md) record reviewed state, evidence boundaries, limitations, and residual risk.

## Product constitution

See [PRODUCT.md](PRODUCT.md) for users, hero workflow, capability boundaries, the full product loop, success criteria, and explicit non-goals.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for logical system topology, language ownership, trust boundaries, and storage responsibilities. Planned boundaries only — not an implementation or deployment claim.

See [EVENT_MODEL.md](docs/architecture/EVENT_MODEL.md), [COMMAND_MODEL.md](docs/architecture/COMMAND_MODEL.md), [CONTRACTS.md](CONTRACTS.md), and the [ADRs](docs/adrs/) for the planned correctness model and concrete Event Spine contract.

## Security model

See [THREAT_MODEL.md](docs/security/THREAT_MODEL.md) and [AUTHORIZATION_MODEL.md](docs/security/AUTHORIZATION_MODEL.md) for the planned threat, tenant-isolation, authorization, approval, service-identity, retrieval, audit, and receipt boundaries. See the [Project Constitution security-model review](docs/reviews/PROJECT_CONSTITUTION_SECURITY_REVIEW.md) for documentation traceability and recorded follow-ups. These documents define future requirements only; they do not claim implemented security controls.

## Intelligence evaluation

See the [forecasting evaluation protocol](docs/intelligence/FORECASTING_EVALUATION_PROTOCOL.md), [governed-RAG evaluation protocol](docs/intelligence/GOVERNED_RAG_EVALUATION_PROTOCOL.md), and [Project Constitution intelligence-evaluation review](docs/reviews/PROJECT_CONSTITUTION_INTELLIGENCE_EVAL_REVIEW.md). These are planned evaluation requirements; no runtime evaluation result is claimed.

## Operational evidence

See the [claim-status vocabulary](docs/evidence/CLAIM_STATUS_VOCABULARY.md), [security evidence protocol](docs/evaluation/SECURITY_EVIDENCE_PROTOCOL.md), [reliability and recovery evidence protocol](docs/evaluation/RELIABILITY_RECOVERY_EVIDENCE_PROTOCOL.md), [performance evidence protocol](docs/evaluation/PERFORMANCE_EVIDENCE_PROTOCOL.md), [fault campaign matrix](docs/evaluation/FAULT_CAMPAIGN_MATRIX.md), [event-spine exit-gate procedure](docs/evaluation/EVENT_SPINE_EXIT_GATE_PROCEDURE.md), [event-spine exit-gate experiment report](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md), [experiment report template](docs/evaluation/templates/EXPERIMENT_REPORT.md), and [Project Constitution operational evidence review](docs/reviews/PROJECT_CONSTITUTION_OPERATIONAL_EVIDENCE_REVIEW.md). Protocols define evidence requirements; only linked experiment records support Observed claims.

## Roadmap and evidence ledger

See the [canonical roadmap](ROADMAP.md), [evidence ledger](EVIDENCE.md), and [Project Constitution roadmap/evidence review](docs/reviews/PROJECT_CONSTITUTION_ROADMAP_EVIDENCE_REVIEW.md). These define capability-sequence ownership and future claim governance; they do not prove implementation or measured results.

## Clean-room boundary

**Ahoy is not a dependency** and is not a source of public artifacts. No Ahoy code, schema, data, identifiers, screenshots, or business-specific knowledge belongs in this repository.

See [CLEAN_ROOM.md](CLEAN_ROOM.md) for the enforceable policy and [docs/checklists/CLEAN_ROOM_REVIEW.md](docs/checklists/CLEAN_ROOM_REVIEW.md) for the reusable review checklist.

## License

Licensed under the [Apache License 2.0](LICENSE).
