# Claim Status Vocabulary

**Status:** Planned evidence-governance vocabulary for Issue #7. This document defines how future claims and evidence records are classified. It is not evidence that any SeshatOps control, capability, experiment, or operational target exists.

## Purpose

Every security, reliability, recovery, SLO, degraded-mode, performance, or portfolio claim must have a stable identifier and an evidence record. A claim may not be promoted beyond what its evidence establishes.

The vocabulary below is the complete claim-status vocabulary for SeshatOps. Do not introduce synonyms or additional claim statuses.

## Claim statuses

| Status | Meaning | What it does not establish |
| --- | --- | --- |
| **Planned** | The capability, control, test, target, or claim is intended but has not been implemented or observed. | Planned is not evidence. |
| **Implemented** | The relevant implementation or control exists in a reviewed repository state. | Implementation does not establish that behavior works correctly under relevant conditions. |
| **Observed** | The behavior was measured in a named experiment with reproducible artifacts and recorded limitations. | The claim applies only to the exact environment, commit, configuration, workload, dataset, duration, and conditions tested. One successful run does not imply production readiness, universal correctness, or long-term reliability. |
| **Reproduced** | A prior observation was repeated under the documented reproduction requirements and produced materially consistent results. | Reproduction does not broaden the claim beyond the documented conditions or remove recorded limitations. |
| **Superseded** | A previous claim or evidence record is no longer the current basis for decision-making. | Superseded evidence is not deleted or silently replaced. |

`Not executed` is an experiment or matrix result marker, not a claim status. Existing Issue #6 record dispositions such as `Not evaluated`, `TBD — evidence required`, `Unavailable`, and `Not applicable` are record-level dispositions, not additions to this claim-status vocabulary.

## Promotion and withdrawal rules

- A new claim starts as **Planned**.
- **Implemented** may be assigned only when the relevant implementation or control is present in a reviewed repository state.
- **Observed** requires a named experiment, a complete reproducibility record, raw or machine-readable artifacts where practical, a result summary, failures and anomalies, limitations, and a reviewer decision.
- **Reproduced** requires the prior observation, documented reproduction instructions, independent-execution details, whether the environment was rebuilt, operator identity or role, equivalence criteria, tolerances when already approved elsewhere, and remaining differences. Constitution does not invent numerical tolerances.
- Missing environment, configuration, commit, workload, dataset, artifact, or limitation details prevent promotion to **Observed** or **Reproduced**.
- A stable claim identifier must be assigned before an experiment executes or a claim is promoted to **Observed** or **Reproduced**. A `Planned` claim-ID placeholder is not an assigned identifier. Issue #8 established the repository evidence ledger; future execution and promotion may not proceed without a stable ledger assignment and the required evidence record.
- Stale, invalidated, corrupted, contradicted, leaked, or non-reproducible evidence requires withdrawal of the affected claim or transition to **Superseded**, with a link to the replacing decision or evidence.
- A claim must never be promoted because a screenshot exists, because an average looks favorable, because a control is designed, or because a dependency reports success.
- A claim status change records both the prior and new status and links the decision to the supporting evidence.

## Stable claim identifiers

Every claim identifier must be:

- unique within the evidence ledger;
- stable across report revisions;
- descriptive of the claim boundary rather than a result;
- linked from the claim to its evidence records; and
- retained when the claim is withdrawn or superseded.

An experiment may evaluate multiple claim IDs. A claim may reference multiple experiment records, but each reference must state the exact scope and limitations it supports.

## Evidence record requirements

Every evidence record must identify:

- claim IDs and status before and after the experiment;
- repository and commit, branch or tag, and dirty-working-tree state;
- environment class, topology, operating system, runtime, tools, dependencies, and configuration;
- dataset, fixture, corpus, or workload provenance;
- seed or a reason it is not applicable, exact commands or automation entry point, timestamps, clock and time-zone assumptions;
- operator, reviewer, raw artifacts, checksums where applicable, results, failures, anomalies, and limitations; and
- reproduction instructions and the decision about claim status.

Missing details prevent promotion. Evidence must remain scoped to what was actually tested.

## Evidence-environment classes

| Environment class | Permitted use | Explicit limitation |
| --- | --- | --- |
| **Local demonstration** | Show an interaction or concept on one developer-controlled machine. | Cannot support claims about distributed behavior, realistic isolation, recovery, capacity, production security, or availability. |
| **Test environment** | Support deterministic, automated, or controlled functional and failure tests. | Claims remain scoped to the declared topology, data, tools, and test conditions. |
| **Staging environment** | Support production-like integration, operational, failure, recovery, and load evidence. | Differences from the intended production environment must be recorded; staging success does not establish production evidence. |
| **Production environment** | Support claims from real production operation during a separately recorded observation window. | Requires explicit authorization, safety controls, data-handling rules, environment and configuration records, and appropriate review. Production evidence must not be implied by staging results. |

No environment may be described as more realistic than its documented configuration proves.

## Claim language guardrails

- Claims are scoped to the tested environment and conditions.
- Missing telemetry is not silently treated as success.
- Averages alone cannot support latency or failure claims; distributions and relevant slices are required.
- Absence of observed failure is not proof that failure is impossible.
- Screenshots alone are not sufficient evidence.
- Local demonstrations cannot support production claims.
- Planned targets remain Planned until a later capability sequence approves and measures them.
- The vocabulary does not select SLO targets, RPO/RTO targets, infrastructure sizes, products, repetition counts, or pass thresholds.

## Related documents

- [Experiment report template](../evaluation/templates/EXPERIMENT_REPORT.md)
- [Fault campaign matrix](../evaluation/FAULT_CAMPAIGN_MATRIX.md)
- [Security evidence protocol](../evaluation/SECURITY_EVIDENCE_PROTOCOL.md)
- [Reliability and recovery evidence protocol](../evaluation/RELIABILITY_RECOVERY_EVIDENCE_PROTOCOL.md)
- [Performance evidence protocol](../evaluation/PERFORMANCE_EVIDENCE_PROTOCOL.md)
