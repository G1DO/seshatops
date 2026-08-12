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

Project Constitution is complete. Event Spine is complete for the declared test environment (`CLM-003`–`CLM-006` Observed). Identity & Operations is in progress on GitHub. A long-running deployment service is not present yet; Planned capabilities are not measured results beyond linked evidence.

## Docs

| Doc | Owns |
| --- | --- |
| [CONTRACTS.md](CONTRACTS.md) | Concrete Event Spine event-spine contract |
| [ROADMAP.md](ROADMAP.md) | Capability-sequence ownership and sequencing |
| [EVIDENCE.md](EVIDENCE.md) | Claim ledger and verification routes |
| [EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](docs/evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md) | Exit-gate experiment record |
| [EVENT_SPINE_COMPLETION_SUMMARY.md](docs/reviews/EVENT_SPINE_COMPLETION_SUMMARY.md) | Event Spine completion boundary |
| [AGENTS.md](AGENTS.md) | Contribution and verification rules |
| [CLEAN_ROOM.md](CLEAN_ROOM.md) | Ahoy exclusion and provenance policy |

Contribution hygiene: [PR template](.github/pull_request_template.md), [Documentation CI](.github/workflows/documentation-ci.yml), [Go CI](.github/workflows/go-ci.yml).

## Product

See [PRODUCT.md](PRODUCT.md) for users, hero workflow, capability boundaries, success criteria, and non-goals.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for logical topology, language ownership, trust boundaries, and storage responsibilities. Event path packages: `event/`, `erp/`, `relay/`, `platform/`, `api/`, `web/`. Correctness model: [EVENT_MODEL.md](docs/architecture/EVENT_MODEL.md), [COMMAND_MODEL.md](docs/architecture/COMMAND_MODEL.md), [CONTRACTS.md](CONTRACTS.md), [ADRs](docs/adrs/).

## Security, intelligence, and ops evidence

Planned security model: [THREAT_MODEL.md](docs/security/THREAT_MODEL.md), [AUTHORIZATION_MODEL.md](docs/security/AUTHORIZATION_MODEL.md). Planned intelligence eval: [docs/intelligence/](docs/intelligence/). Evidence protocols and fault matrix: [docs/evaluation/](docs/evaluation/), including the [fault campaign matrix](docs/evaluation/FAULT_CAMPAIGN_MATRIX.md).

## Clean-room boundary

**Ahoy is not a dependency** and is not a source of public artifacts. See [CLEAN_ROOM.md](CLEAN_ROOM.md) and [docs/checklists/CLEAN_ROOM_REVIEW.md](docs/checklists/CLEAN_ROOM_REVIEW.md).

## License

Licensed under the [Apache License 2.0](LICENSE).
