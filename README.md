# SeshatOps

**Secure Operations Intelligence and Action Control Plane**

From business events to evidence-backed decisions and human-approved actions.

## What it is

SeshatOps is a clean-room, multi-tenant operations-intelligence platform that consumes ERP events, reconstructs replayable operational state, predicts stockout risk, explains recommendations with permission-aware evidence, and executes only authorized, human-approved, idempotent commands.

The complete product loop is:

**Observe → Reconstruct → Predict → Explain → Propose → Authorize → Approve → Execute → Audit → Replay**

## Public scenario

All public narrative and demos use **Northstar Foods**, a fictional manufacturer and distributor, backed by a standalone synthetic ERP. The project must work with no private production system.

## Status

**Implementation has not started.**

This repository currently holds product constitution documents only (Issue #1 / Milestone M0). No application code, services, databases, brokers, or runtime scaffolding are present. Planned capabilities must not be read as completed or measured results.

## Product constitution

See [PRODUCT.md](PRODUCT.md) for users, hero workflow, capability boundaries, the full product loop, success criteria, and explicit non-goals.

## Clean-room boundary

**Ahoy is not a dependency** and is not a source of public artifacts. No Ahoy code, schema, data, identifiers, screenshots, or business-specific knowledge belongs in this repository.

## License

Licensed under the [Apache License 2.0](LICENSE).
