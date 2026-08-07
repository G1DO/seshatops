# PRODUCT — SeshatOps Constitution

**Status:** Planned. Implementation has not started. This document defines product intent and enforceable boundaries for SeshatOps. It is not evidence that capabilities exist.

## 1. Product thesis

> SeshatOps — Secure Operations Intelligence and Action Control Plane

SeshatOps is a clean-room, multi-tenant operations-intelligence platform that:

- consumes ERP business events,
- reconstructs replayable operational state,
- predicts stockout risk,
- explains recommendations with permission-aware evidence,
- and executes only authorized, human-approved, idempotent commands.

Public demonstration uses a standalone synthetic ERP and a fictional domain. The product must remain fully understandable and runnable without any private production system.

## 2. Public scenario: Northstar Foods

**Northstar Foods** is a fictional manufacturer and distributor used for all public product narrative, demos, datasets, documents, and evaluation corpora.

- Orders, inventory, batches, shipments, purchasing, and replenishment are expressed in this fictional domain.
- Generated or properly licensed public data only.
- No real customer, supplier, recipe, pricing, or operational content from any private business.

## 3. Users

| Persona | Goal | Permission emphasis |
| --- | --- | --- |
| Operations manager | See risk, understand impact, monitor execution | Read operations; approve within assigned scope |
| Inventory / production planner | Investigate stockout risk and prepare replenishment | Read planning data; create proposals |
| Platform / security operator | Inspect health, recover failures, audit access | Quarantine/replay; security and audit access |

Authorization is default-deny. The UI may hide unavailable actions; server-side policy is authoritative.

## 4. Hero workflow

Capability-level path for the primary demo story (not an implementation claim):

1. A customer order reduces inventory in the synthetic ERP.
2. The business transaction and outbox intent commit together.
3. A versioned event is published for platform consumption.
4. The platform deduplicates the event and updates live projections.
5. Intelligence evaluates feature state and predicts stockout risk with uncertainty.
6. The console shows risk, affected items, horizon, confidence, and freshness.
7. The operator asks why; governed retrieval returns only permitted fictional policies and runbooks.
8. Intelligence returns an evidence-backed, typed replenishment **proposal** — not an executable command.
9. The platform validates policy, tenant, role, limits, freshness, and current state.
10. An authorized human approves or rejects the proposal.
11. The platform sends one idempotent command to the ERP and records a durable receipt.
12. The decision timeline retains actor, data, model, citations, policy result, command, receipt, and trace lineage.

## 5. Secondary proof workflow

An operator selects a finished batch and traces ingredients, suppliers, production runs, shipments, and affected customer orders. The same event history, projections, authorization, audit, and replay capabilities support this path. Traceability must not require a separate product architecture.

## 6. Complete product loop

Every end-to-end journey follows:

> Observe → Reconstruct → Predict → Explain → Propose → Authorize → Approve → Execute → Audit → Replay

| Stage | Meaning |
| --- | --- |
| Observe | Ingest versioned business events from the synthetic ERP |
| Reconstruct | Build and maintain replayable operational projections |
| Predict | Forecast stockout risk with uncertainty and abstention when data is insufficient or stale |
| Explain | Answer operator questions with permission-aware, cited evidence |
| Propose | Emit typed action proposals only — never direct writes from intelligence |
| Authorize | Enforce tenant, role, resource, action, scope, and policy checks |
| Approve | Require an authorized human decision before execution |
| Execute | Dispatch idempotent commands and record durable receipts |
| Audit | Preserve decision and action lineage for review |
| Replay | Rebuild projections and re-examine history from the same event spine |

## 7. Capability boundaries

Designed end-state capabilities (all **Planned** until proven in-repository):

| Capability | Required outcome |
| --- | --- |
| Synthetic ERP | Orders, inventory, batches, purchasing, outbox, command receipts |
| Event spine | Durable, ordered, versioned event transport and consumption |
| Operational projections | Live orders, inventory, shortages, and batch lineage |
| Recovery controls | Lag visibility, poison handling, quarantine, replay, rebuild |
| Stockout intelligence | Risk, uncertainty, reason codes, abstention, temporal evaluation |
| Governed knowledge | Permission-aware answers from fictional policies and runbooks |
| Approved actions | Typed proposal → policy → human approval → command → receipt |
| Multi-tenant security | Tenant/resource/action authorization at every boundary |
| Model and data lifecycle | Lineage for datasets, features, models, retrieval, and recommendations |
| Reliability and operations | Telemetry, failure evidence, restoreability, honest reporting |
| Governance | Threat model, retention, risk register, and claims/evidence ledger |

## 8. Explicit non-goals

The following are out of scope for SeshatOps as a public product, or are forbidden as silent shortcuts:

1. **Autonomous writes** — no agent or model may execute business commands without human approval.
2. **Ahoy or other private systems as public dependencies** — Ahoy is never a required runtime, demo input, documentation source, dataset, screenshot, schema, or identifier in this repository.
3. **Copying private business knowledge** — no private code, schema, logs, recipes, prices, customers, or production identifiers in public artifacts.
4. **Claiming unmeasured results** — planned targets are not completed README, CV, or portfolio claims.
5. **Hyperscale theater** — Kubernetes, service mesh, multi-region, or “five-language” breadth are not goals unless a measured requirement later justifies them (recorded as ADRs, not assumed here).
6. **Training foundation models** — forecasting and RAG use evaluated, governed methods; this is not a foundation-model research project.
7. **Building a general ERP** — SeshatOps consumes and commands a synthetic ERP; it does not replace full ERP suites.
8. **Exactly-once delivery marketing** — the honest consistency stance is at-least-once delivery with idempotent business effects.
9. **UI-owned authorization** — hiding buttons is not a security control.
10. **Implementation in this constitution issue** — product definition only; no application runtime, frameworks, databases, brokers, or CI scaffolding under this deliverable.

## 9. Success criteria (planned)

Success for the product, once implemented and evidenced, means reviewers can verify that:

- One coherent hero workflow runs on synthetic Northstar Foods data alone.
- The complete product loop is demonstrable end to end.
- Duplicate delivery and retries do not create duplicate business effects.
- Unchanged history rebuilds to an identical projection checksum.
- Cross-tenant and unauthorized access attempts fail under test.
- Forecasts are compared honestly against declared temporal baselines.
- RAG cites permitted sources, refuses unsafe or unsupported questions, and cannot write.
- Every public claim links to repository evidence.

Until those proofs exist, every capability above remains **Planned**.

## 10. Document ownership

| Artifact | Owns |
| --- | --- |
| `PRODUCT.md` | Product thesis, users, workflows, boundaries, non-goals |
| `README.md` | Short public summary and honest status |
| `CLEAN_ROOM.md` | Public/private boundary and review policy |
| `ARCHITECTURE.md` | Logical topology, language ownership, trust and storage boundaries |
| Later M0 documents | Threat model, roadmap, evidence ledger, repository instructions |

Notion may hold planning intent. This repository owns publishable product truth for SeshatOps.
