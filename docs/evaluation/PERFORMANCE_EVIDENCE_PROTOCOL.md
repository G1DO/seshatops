# Performance Evidence Protocol

**Status:** Planned evidence protocol for Issue #7. This document defines future methodology for load and performance evidence. It does not select infrastructure, tools, workload values, thresholds, capacity targets, or production claims.

**Owns:** Workload description, performance measurements, experiment integrity, comparison limits, and claim discipline.

**Does not own:** Runtime implementation, deployment topology, observability products, load-testing products, infrastructure sizing, SLO targets, or capacity decisions.

## 1. Performance claim boundary

Performance evidence must identify a stable claim ID and the exact repository commit, branch or tag, working-tree state, environment class, topology, configuration, tool versions, workload, dataset or fixture provenance, duration, and conditions tested.

No capacity claim extends beyond the highest load actually observed under a documented comparable run. No production claim may be based solely on local demonstration evidence. A mean or a screenshot is not sufficient evidence for latency, failure, availability, or capacity.

## 2. Workload description

Every future load or performance experiment must record:

- operation mix;
- read/write ratio where relevant;
- tenant distribution;
- dataset or fixture size;
- record cardinality;
- request or message sizes;
- concurrency;
- arrival pattern;
- ramp-up;
- warm-up;
- steady-state duration;
- cool-down;
- cache state;
- background work;
- dependency behavior; and
- failure injection when included.

Values must come from the recorded workload, not from invented constitution-phase targets. A workload generator must be isolated from the system under test or its resource impact must be measured and reported.

## 3. Required measurements

Report distributions and time series where relevant, including:

- throughput;
- success and error rate;
- latency percentiles, including tail latency;
- saturation;
- queue depth or lag;
- resource utilization;
- retry rate;
- timeout rate;
- backpressure or rejection rate;
- data-integrity failures;
- recovery behavior after load; and
- cold versus warm behavior where relevant.

Errors must remain included in the relevant reporting rather than being silently removed. Aggregate summaries must not replace distributions, slices, or time-series behavior.

## 4. Experiment integrity

Each experiment must address:

1. Workload-generator isolation or measured generator resource impact.
2. Coordinated-omission risk and the treatment used to address it.
3. The distinction between client-side and server-side measurements.
4. Warm-up exclusion only with a recorded justification.
5. Inclusion and classification of errors in latency and throughput reporting.
6. Test duration and repetition rationale without inventing repetition counts in Project Constitution.
7. Environment contention and background activity.
8. Scaling or configuration changes as separate comparable runs.
9. Retention of raw results and machine-readable artifacts.
10. Comparable environments and workloads for comparative claims.
11. The highest observed load and the boundary beyond which no claim is made.
12. Explicit recovery and integrity checks after the load condition.

## 5. Comparison and interpretation rules

- Comparative claims require comparable environments, configurations, workloads, measurement sources, and analysis methods.
- A change in environment, configuration, scaling, dependency behavior, or workload is recorded as a separate run.
- Tail behavior, failures, saturation, queues, retries, and rejection must not be hidden by a favorable mean.
- A short successful run does not establish long-term performance or availability.
- Local demonstration evidence cannot establish distributed, staging, or production capacity.
- The workload generator's behavior must not be mistaken for system-under-test behavior.
- Cold and warm results remain distinct where cache or initialization affects behavior.
- Data-integrity or recovery failures remain part of the performance result and limitation record.

## 6. Claim status and unselected values

Use the exact five claim statuses in [CLAIM_STATUS_VOCABULARY.md](../evidence/CLAIM_STATUS_VOCABULARY.md). Performance scenarios in this Constitution protocol remain **Planned** and no run result exists.

This protocol does not select pass thresholds, latency targets, throughput targets, availability targets, repetition counts, hardware, infrastructure sizes, monitoring products, load-testing products, or fault-injection products.

## Related documents

- [Claim-status vocabulary](../evidence/CLAIM_STATUS_VOCABULARY.md)
- [Reliability and recovery evidence protocol](RELIABILITY_RECOVERY_EVIDENCE_PROTOCOL.md)
- [Fault campaign matrix](FAULT_CAMPAIGN_MATRIX.md)
- [Experiment report template](templates/EXPERIMENT_REPORT.md)
- [Forecasting evaluation protocol](../intelligence/FORECASTING_EVALUATION_PROTOCOL.md)
