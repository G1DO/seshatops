# Reliability and Recovery Evidence Protocol

**Status:** Planned evidence protocol for Issue #7. This document defines future evidence requirements for reliability, recovery, degraded-mode, backup/restore, and SLO claims. It does not claim that any runtime behavior has been implemented, measured, or verified.

**Owns:** Failure evidence, recovery evidence, backup/restore evidence, degraded-mode evidence, SLO/SLI evidence, and claim limitations.

**Does not own:** Event and command correctness principles, concrete recovery tooling, infrastructure topology, monitoring products, executable tests, or runtime implementation.

## 1. Evidence boundary

Reliability evidence must be tied to a stable claim ID, the exact repository commit, configuration, environment class, workload or fixture, duration, fault conditions, artifacts, and reviewer decision. Expected behavior written in a design document is not observed behavior.

Claims must distinguish:

- a durable design invariant from an implemented control;
- a measured observation from a reproduced observation;
- a service being restored from data being intact and correct; and
- a dependency being reachable from the user-visible behavior being available.

One successful run cannot establish long-term reliability, universal recovery, or production readiness. Absence of observed failure cannot establish that failure is impossible.

## 2. Required scenario record

For every failure or recovery scenario, record:

1. Scenario and claim IDs.
2. Invariant being tested.
3. Fault-injection boundary and the exact injection-method placeholder or later method.
4. Preconditions, fixture/workload provenance, and environment class.
5. Expected externally visible behavior.
6. Expected durable state and prohibited state effects.
7. Expected audit, trace, log, receipt, quarantine, or evidence artifacts.
8. Detection signal and the source used to determine detection.
9. Recovery procedure and explicit recovery criterion.
10. Data-integrity verification, including missing, duplicated, reordered, or corrupted state.
11. Termination and safety conditions, including how unsafe or ambiguous work is stopped.
12. Raw artifacts, result summary, failures, anomalies, residual uncertainty, and limitations.
13. Reproduction instructions and the claim-status decision.

No scenario may be promoted because the expected behavior was documented but not exercised.

## 3. Reliability and failure coverage

Future evidence must cover, where applicable:

| Scenario | Evidence focus |
| --- | --- |
| Duplicate event delivery | Stable identity, deduplication, one required local effect, and no silent data loss. |
| Duplicate command submission | Business-intent idempotency, durable outcome retrieval, and no duplicate effect. |
| Concurrent command retries | Contention, equivalent intent, conflicting intent, and durable winner/outcome handling. |
| Client timeout after successful execution | Receipt retrieval and distinction between client uncertainty and execution result. |
| Downstream timeout with unknown outcome | Explicit uncertainty, reconciliation, and no blind irreversible retry. |
| Partial downstream failure | Local decision, external attempt state, reconciliation, and no unsupported distributed-transaction claim. |
| Broker outage | Durable accepted state/outbox behavior, stale asynchronous views, controlled backpressure, and recovery. |
| Python outage | Core Go-owned transactions and authorization remain independent; intelligence becomes unavailable or stale. |
| Poison events | Quarantine, sanitized diagnostics, isolation of unrelated aggregates, and controlled recovery. |
| Unsupported event versions | Safe rejection/quarantine and explicit compatibility outcome. |
| Missing, skipped, or reordered aggregate versions | Detection, hold/quarantine/reconciliation, and no silent version skipping. |
| Publisher or consumer restart | Durable progress, duplicate handling, acknowledgement uncertainty, and recovery. |
| Process crash during state transition | Atomicity, durable decision state, replay/recovery behavior, and no ambiguous silent success. |
| Replay | Deterministic replay of supported projections and versioned handler/reference inputs. |
| Replay attempting irreversible external effects | Suppression, simulation, or separate reconciliation; no repeated external effect. |
| Quarantine and controlled recovery | Preserved identity/lineage, authorization, safe release criteria, and auditability. |
| Degraded-mode operation | User-visible stale/unavailable behavior, preserved authority boundaries, and safe recovery. |
| Capacity exhaustion and controlled backpressure | Safe rejection or bounded degradation without silent loss or fail-open authorization. |
| Dependency unavailability | Attribution, user-visible behavior, safe refusal, and recovery without overclaiming platform availability. |
| Partial recovery | Service restoration versus recovered data and recovered dependent capabilities. |
| Recovery that loses or corrupts data | Explicit detection, containment, integrity results, claim withdrawal, and residual uncertainty. |

The detailed reusable campaign fields are in [FAULT_CAMPAIGN_MATRIX.md](FAULT_CAMPAIGN_MATRIX.md).

## 4. Backup and restore evidence

The existence of a backup is not evidence of restorability. A restore record must identify:

- the source backup, snapshot, or recovery artifact;
- backup age and covered data interval;
- the target restore environment and its differences from the source;
- repository commit, configuration, tools, and dependencies used for validation;
- application-readable data checks, not only file or object existence;
- tenant-boundary checks after restore;
- referential and domain-integrity checks;
- event, command, receipt, audit, and evidence continuity checks where applicable;
- missing, partially restored, duplicated, stale, or corrupted data; and
- recovery procedure, safety controls, raw artifacts, limitations, and reviewer decision.

Recovery-point and recovery-time observations may be reported only from measured experiments. Planned RPO or RTO targets must not be invented in M0. Destructive recovery tests require separately approved safety controls in a later milestone.

Restore success must not be reported merely because files exist, a process starts, or a health endpoint responds. The restored application must read and validate the relevant data and preserve tenant boundaries.

## 5. SLO and availability evidence

Before an SLO or availability claim is accepted, the evidence record must define:

- the SLI and the user-visible behavior being measured;
- the measurement source and exact query or calculation method;
- eligible events and excluded events, with rationale;
- observation window and clock/time-zone assumptions;
- missing-data treatment;
- error classification and degraded-mode classification;
- dependency attribution and treatment of partial dependency failure;
- data completeness and known telemetry gaps;
- environment class, configuration, repository commit, and relevant versions;
- limitations, exclusions, and reviewer decision; and
- claim IDs and status before and after the observation.

Synthetic checks alone do not establish user-visible availability. A short successful window does not establish long-term availability. Missing telemetry cannot be silently treated as success. Planned targets remain **Planned** until a later milestone approves and measures them.

An availability claim must not silently exclude errors, degraded behavior, dependency failures, incomplete telemetry, or user-visible failure modes. Exclusions must be explicit and justified.

## 6. Claim status and limitations

Use the exact five claim statuses in [CLAIM_STATUS_VOCABULARY.md](../evidence/CLAIM_STATUS_VOCABULARY.md). All scenarios and results in this M0 protocol remain **Planned** or **Not executed**. No reliability, recovery, restorability, availability, or SLO claim is promoted here.

## 7. Deferred decisions

This protocol does not select fault-injection products, monitoring products, load-testing products, backup products, infrastructure sizes, deployment topology, telemetry queries, SLO targets, error budgets, RPO/RTO targets, pass thresholds, or repetition counts. Concrete implementation and operations decisions belong to later milestones.

## Related documents

- [Event model](../architecture/EVENT_MODEL.md)
- [Command model](../architecture/COMMAND_MODEL.md)
- [Threat model](../security/THREAT_MODEL.md)
- [Authorization model](../security/AUTHORIZATION_MODEL.md)
- [Claim-status vocabulary](../evidence/CLAIM_STATUS_VOCABULARY.md)
- [Fault campaign matrix](FAULT_CAMPAIGN_MATRIX.md)
- [Experiment report template](templates/EXPERIMENT_REPORT.md)
