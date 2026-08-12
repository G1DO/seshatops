# Security Evidence Protocol

**Status:** Planned evidence protocol for Issue #7. This document defines future evidence requirements for security claims. It does not claim that authentication, authorization, tenant isolation, logging, retrieval isolation, or any other control has been implemented or passed.

**Owns:** Security evidence scope, tenant-isolation coverage, authorization-negative-test categories, evidence-record requirements, and claim limitations.

**Does not own:** Identity-provider selection, policy implementation, executable fixtures, test code, infrastructure, monitoring products, or runtime security controls owned by later capability sequences.

## 1. Evidence boundary

Security evidence must be linked to a stable claim ID and the exact reviewed implementation commit and environment under test. A design requirement, a successful request, a screenshot, or an absence of observed failure is not sufficient evidence by itself.

Any demonstrated cross-tenant access, retrieval, citation, state effect, or evidence leakage is a security failure, not a quality trade-off. Evidence must preserve the failure and its limitations rather than hide it.

Security evidence remains scoped to the tested principal, tenant fixtures, resource set, permissions, implementation, configuration, environment, workload, and observation conditions. It must not be generalized to untested surfaces.

## 2. Tenant-isolation evidence categories

Future negative-test suites must cover each applicable category. A category may be recorded as evaluated, unavailable, or not applicable only with a reason; omission is not evidence of coverage.

| Category | Required negative-test focus |
| --- | --- |
| Transactional reads | Read another tenant's state, including colliding identifiers, filters, pagination, and stale context. |
| Transactional writes | Attempt to create, modify, delete, approve, or otherwise affect another tenant's state. |
| Identifier substitution | Replace resource, aggregate, command, receipt, approval, event, citation, or export identifiers across tenants or scopes. |
| Events and asynchronous consumers | Cross-wire tenant, aggregate, producer, consumer, causation, or processing context; verify rejection or quarantine. |
| Commands and durable receipts | Reuse, alter, replay, or swap commands, idempotency keys, approvals, targets, and receipts across tenants. |
| Retrieval candidates and model context | Attempt to return unauthorized documents, chunks, features, prompts, or context to retrieval or intelligence. |
| Citations | Return citations from another tenant, unauthorized scope, stale version, or unsupported source. |
| Forecasts and typed proposals | Attempt to expose, alter, or execute tenant-bound forecasts and advisory proposals outside authorized scope. |
| Caches | Attempt cross-tenant cache hits, namespace collisions, stale entries, or cache poisoning. |
| Search or retrieval indexes | Attempt cross-tenant index hits, namespace substitution, stale authorization, or contamination. |
| Logs and traces | Trigger errors, denials, retries, quarantine, and failures and verify that protected content is not exposed. |
| Exports and evidence artifacts | Attempt unauthorized download, sharing, lookup, alteration, or cross-tenant evidence access. |
| Replay | Replay another tenant's history or use replay to create unauthorized state or external effects. |
| Backup and restore | Restore or inspect another tenant's data, cross tenant boundaries after restore, or use a backup as authority without validation. |
| Administrative operations | Exercise tenant administration, platform operations, audit, recovery, export, and replay outside the assigned boundary. |
| Service-to-service access | Attempt confused-deputy behavior, missing delegated user context, over-privileged service actions, and service identity substitution. |

## 3. Authorization-negative-test categories

Future tests must include, where applicable:

- missing principal;
- missing or ambiguous tenant context;
- unknown resource;
- unauthorized action;
- scope mismatch;
- stale role or assignment;
- revoked session;
- changed permissions during a workflow;
- changed target version;
- expired or altered approval;
- approval substitution;
- replayed approval;
- replayed command;
- same idempotency key with changed intent;
- direct browser bypass;
- trusted-service confused-deputy attempt;
- service identity acting without delegated user context;
- Python attempting to authorize or execute;
- forged receipt;
- tampered audit record;
- prompt injection attempting to change authority; and
- unauthorized citation or retrieval.

The categories extend the planned controls and negative-test themes in [THREAT_MODEL.md](../security/THREAT_MODEL.md) and [AUTHORIZATION_MODEL.md](../security/AUTHORIZATION_MODEL.md). They do not create a runtime test suite in Project Constitution.

## 4. Required evidence for each scenario

Each future security scenario must record:

1. Test identity, principal fixtures, tenant fixtures, resource fixtures, and authorization context.
2. The claim IDs under evaluation and claim status before and after the scenario.
3. The exact implementation commit, branch or tag, dirty-tree state, environment class, topology, configuration, runtime, tool versions, and dependency versions.
4. The negative action or input, including the intended boundary being challenged.
5. An explicit expected denial, refusal, quarantine, containment, or safe-unavailable result.
6. Proof that no unauthorized state effect, command, approval, retrieval, citation, export, or external effect occurred.
7. Audit evidence for the denial, suspicious attempt, integrity failure, or containment decision.
8. Verification across display, approval, and execution checkpoints where the workflow has those stages.
9. Verification that errors, logs, traces, citations, exports, and evidence artifacts did not leak protected content.
10. Raw machine-readable artifacts where practical, plus sanitized human-readable summaries.
11. Failures, anomalies, untested surfaces, residual uncertainty, and limitations.
12. Reproduction instructions and the reviewer decision.

Evidence must identify the artifact provenance and must not create a new unrestricted sensitive-data store. Raw artifacts may require authorization, minimization, or redaction; redaction must not erase the fact or outcome needed to review the claim.

## 5. Checkpoint and result rules

- Display-time authorization does not replace approval-time or execution-time authorization.
- Approval evidence must remain bound to the intended tenant, action, target, important parameters, normalized intent, and relevant target version.
- A receipt reference, event ID, citation, approval ID, or idempotency key is not authority.
- Missing, contradictory, stale, or ambiguous security context fails closed or is explicitly recorded as an unsafe/incomplete result.
- A trusted service identity does not erase the initiating principal or tenant.
- Python output and retrieved content remain advisory or untrusted data; they cannot authorize, approve, or execute.
- A cross-tenant result is recorded as a security failure even if no later business effect occurred.
- A denied request is not a pass claim unless the expected denial and containment evidence are recorded.
- Security evidence never claims that an untested surface is secure or that failure is impossible.

## 6. Evidence and claim status

Use the exact vocabulary in [CLAIM_STATUS_VOCABULARY.md](../evidence/CLAIM_STATUS_VOCABULARY.md). Security scenarios in this Constitution matrix and protocol remain **Planned** with no observed result. Future records may use `Not executed` as a result marker, but it is not a claim status.

## 7. Deferred decisions

This protocol deliberately does not select identity providers, policy engines, database roles, cache or index products, secret-management products, penetration-test tools, monitoring products, fault-injection products, test fixtures, deployment topology, or production authorization procedures. Those decisions belong to later identity, operations, governance, reliability, and implementation capability sequences.

## Related documents

- [Threat model](../security/THREAT_MODEL.md)
- [Authorization model](../security/AUTHORIZATION_MODEL.md)
- [Event model](../architecture/EVENT_MODEL.md)
- [Command model](../architecture/COMMAND_MODEL.md)
- [Governed-RAG evaluation protocol](../intelligence/GOVERNED_RAG_EVALUATION_PROTOCOL.md)
- [Fault campaign matrix](FAULT_CAMPAIGN_MATRIX.md)
- [Experiment report template](templates/EXPERIMENT_REPORT.md)
