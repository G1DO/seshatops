# Governed-RAG Evaluation Protocol

**Status:** Planned. This document defines a future evaluation protocol for permission-aware retrieval, evidence-backed responses, citations, refusals, and typed proposals. It is not a corpus, prompt collection, model evaluation result, security-control result, or production-readiness claim.

**Owns:** Intelligence-specific retrieval, evidence, citation, refusal, prompt-injection, leakage, staleness, conflict, and typed-proposal evaluation requirements.

**Does not own:** Identity-provider selection, retrieval infrastructure, cache or index implementation, executable APIs or types, model/vendor selection, deployment, numerical thresholds, or the platform-wide security/reliability evidence protocol owned by Issue #7.

## 1. Purpose and authority boundary

The protocol defines how future governed-RAG capabilities may be evaluated while preserving tenant isolation, authorization semantics, evidence lineage, and the distinction between advisory intelligence and executable business actions.

The governing boundaries are:

- Authorization filtering occurs before protected content reaches Python or a model.
- The eligible authorized corpus is recorded independently from the retrieved result.
- Retrieved content and user text are untrusted data, not policy or executable instruction.
- Citations are revalidated for authorization and support before display or evidence export.
- Missing, ambiguous, stale, conflicting, unauthorized, or insufficient evidence may require refusal or abstention.
- Typed proposals are advisory data. Go validates, authorizes, approves, and executes; Python cannot do those things.

All checks and outcomes remain `Planned`, `Not evaluated`, or `TBD — evidence required` until supported by repository evidence.

## 2. Conceptual evaluation case and run

An evaluation case is a future, versioned record containing the context needed to assess one governed-RAG behavior:

| Element | Meaning | Initial status |
| --- | --- | --- |
| User query | The request being evaluated | Planned |
| Principal context | The initiating principal, any delegated actor, and relevant decision context | Planned |
| Tenant context | The initiating tenant and any explicitly delegated tenant context | Planned |
| Calling service identity | The service identity making or forwarding the request, distinct from the initiating principal | Planned |
| Authorization context | Resource type and identity, requested action, scope and constraints, policy or assignment version, and freshness basis | Planned |
| Authorized document set | The independently determined documents eligible for this case | Planned |
| Corpus snapshot | Version of the corpus and document state used for evaluation | Planned |
| Retrieval result | Candidate documents, chunks, ranks, and lineage | Planned |
| Generated response | Answer, explanation, refusal, abstention, or unavailable result | Planned |
| Citation set | Sources claimed to support material response claims | Planned |
| Typed proposal | Optional advisory structured proposal, never an executable command | Planned |
| Evaluation disposition | The planned review state and evidence status | Planned |
| Evaluation run | Versioned execution joining cases, evaluator, configuration, and artifacts | Planned |

No private corpus, production prompt, hidden instruction, model output, or identifier is created by this protocol.

## 3. Permission-aware retrieval

Every future evaluation case must establish authorization before retrieval and before model access:

1. Principal, tenant, resource, action, scope, policy, and freshness context are resolved by the authoritative platform boundary.
2. The eligible authorized corpus is recorded independently from whatever the retriever returns.
3. Unauthorized documents must not be inserted into or returned from retrieval candidates, model context, response text, citations, traces, logs, exports, evaluation artifacts, caches, or indexes.
4. Missing or ambiguous authorization context defaults to denial.
5. Citation authorization is rechecked immediately before display or evidence export because access may change after retrieval.
6. Any future cache or index used by evaluation must be tenant- and authorization-scoped. Cache/index namespace or partition, version, and hit/miss lineage must be recorded; a cache or index hit is never authority.

Any demonstrated cross-tenant retrieval or citation leakage is a security failure, not a quality trade-off. The evaluation record must preserve enough sanitized lineage to investigate without creating an unrestricted sensitive-data store.

## 4. Retrieval evaluation categories

For every declared case set, each applicable category below must be recorded as evaluated, unavailable, or not applicable with a reason. Categories may be scoped to the case set, but they must not be silently omitted:

| Category | Required question |
| --- | --- |
| Relevant-document retrieval | Were the authorized relevant sources available in the returned set? |
| Recall over the authorized relevant set | Which authorized relevant sources were missed? |
| Precision or irrelevant-content rate | How much returned content was irrelevant or unsafe for the case? |
| Ranking quality | Were the most useful authorized sources ranked appropriately? |
| Empty-result behavior | Does the system remain honest when no authorized evidence is available? |
| Stale-document behavior | Is stale evidence disclosed, rejected, or handled according to the future policy? |
| Duplicate or conflicting documents | Are duplicates and conflicts surfaced rather than silently merged? |
| Corpus-version sensitivity | Does the result identify the corpus and document versions that affected it? |
| Authorization-filter correctness | Did filtering occur before protected content reached Python or a model? |
| Tenant-isolation behavior | Can one tenant's content influence another tenant's result or artifact? |

No target, score, benchmark value, or pass threshold is selected here.

## 5. Answer, evidence, and citation evaluation

Future cases must evaluate:

- Factual support for each material claim.
- Claim-to-citation alignment.
- Citation existence and resolvability.
- Citation authorization for the requesting principal and tenant.
- Citation freshness and document-version lineage.
- Citation completeness for material claims.
- Handling of contradictory evidence.
- Refusal of unsupported claims.
- Uncertainty disclosure where evidence is incomplete or ambiguous.
- Correct refusal when evidence is insufficient or unauthorized.
- Absence of fabricated citations.
- Absence of hidden or unauthorized evidence in the answer, citation set, trace, or proposal.

Each applicable answer, citation, refusal, and evidence category must be recorded as evaluated, unavailable, or not applicable with a reason. Omission is not evidence of coverage.

A fluent answer without adequate authorized evidence cannot pass the evidence review. A citation that exists but does not support the claim is not adequate support.

## 6. Prompt-injection evaluation

The future adversarial case set must include independently authored, synthetic categories such as:

- Instructions embedded in retrieved documents.
- Requests to reveal system or hidden instructions.
- Attempts to expand tenant or resource scope.
- Attempts to disable authorization or citation checks.
- Attempts to invoke tools or execute commands.
- Instructions to ignore newer or authoritative evidence.
- Encoded or obfuscated instructions.
- Conflicting instructions across retrieved documents.
- Malicious metadata.
- Injection embedded in citation text.

Retrieved content remains untrusted data. A successful injection must not authorize retrieval, reveal inaccessible content, create credentials, alter approval requirements, or cause a command. The evaluation records refusal or containment behavior and does not claim a general security guarantee.

## 7. Leakage and authorization evaluation

Future negative tests must cover:

- Cross-tenant retrieval.
- Cross-user access within a tenant.
- Unauthorized citations.
- Hidden prompt or system-context disclosure.
- Sensitive content in traces, logs, reports, or evidence artifacts.
- Cache contamination.
- Index contamination.
- Evaluation-artifact leakage.
- Free-form output revealing inaccessible documents.
- Typed proposal fields carrying unauthorized content.
- Tenant, principal, resource, action, or scope substitution.

Future cache/index checks must record the relevant tenant and authorization partition or namespace, cache/index version, and hit/miss behavior without copying unauthorized content into the evaluation record.

The case record must distinguish quality failure, authorization failure, and security failure. Cross-tenant retrieval or citation is always a security failure. Broader availability, capacity, recovery, and security-control evidence remains owned by Issue #7.

## 8. Staleness and conflicting evidence

Every case involving time-sensitive or potentially changing evidence must record:

- Corpus snapshot and document version.
- Document freshness information where available.
- Superseded-document relationships where available.
- The evidence available for the evaluation snapshot.
- How stale evidence was disclosed, rejected, or treated.
- How conflicting sources were surfaced or resolved under a documented future policy.

The system must refuse when freshness is required but cannot be established. It must not silently choose a preferred source without support or erase material disagreement from the response.

## 9. Refusal and abstention

Valid refusal or abstention cases include:

- Missing, ambiguous, or invalid authorization context.
- No authorized evidence.
- Insufficient evidence for the requested claim.
- Conflicting evidence that cannot be resolved.
- Prompt-injection risk.
- Stale evidence for a freshness-dependent request.
- Unsupported requested action.
- A request for the model to authorize, approve, or execute.
- Failure to produce valid typed output.

Evaluation must consider both unsafe under-refusal and unnecessary over-refusal. A refusal is not a failure when the case lacks authorized, sufficient, or trustworthy evidence.

## 10. Typed proposals

A conceptual typed proposal may include:

- Proposal identity.
- Tenant and subject reference.
- Proposal type.
- Structured recommended intent.
- Important parameters.
- Evidence and citation references.
- Assumptions.
- Uncertainty or confidence representation.
- Data and corpus lineage.
- Model and evaluator version.
- Creation and expiry context.
- Required approval class.
- Validation status.

The proposal is not an executable command. Free-form prose cannot silently become command parameters. Invalid or incomplete proposals are rejected or marked unavailable. Go validates the proposal against current state and authorization, performs display, approval, and execution checks, and owns command execution. Python cannot authorize, approve, write business state, or execute the proposal.

This is a conceptual field catalog only. It is not an API schema, database schema, Protobuf definition, JSON Schema, or source-code type.

## 11. Lineage and reproducibility

Each future evaluation run must identify, where applicable:

- Evaluation-run ID and case-set identity.
- Capability and version.
- Corpus, document, and snapshot lineage.
- Authorization-context and eligible-corpus lineage.
- Initiating principal, delegated actor, calling service identity, resource, action, scope, policy/assignment version, and authorization-decision lineage.
- Retrieval, prompt/context assembly, evaluator, and code versions.
- Cache/index namespace or partition and version where applicable, including hit/miss lineage.
- Configuration and declared environment.
- Deterministic seed where relevant.
- Evaluation timestamp and freshness basis.
- Retrieval, answer, citation, refusal, abstention, proposal, and security checks produced.
- Artifacts and sanitized evidence links.
- Known limitations and exclusions.
- Reviewer, claim status, and disposition.

Reproduction requires the same versioned corpus, authorized case context, evaluator, configuration, permitted inputs, and environment declaration. Lost lineage or non-reproducible artifacts prevent the affected claim from being established.

## 12. Claim withdrawal and rollback

A governed-RAG capability or claim must be withdrawn, marked unavailable, or returned to a prior supported state when:

- Authorization filtering or citation revalidation is shown to be bypassable.
- Cross-tenant or unauthorized content appears in any evaluated surface.
- Prompt injection causes unsafe disclosure, authority expansion, or command-oriented behavior.
- Citation support, freshness, or completeness is materially misrepresented.
- Refusal or abstention behavior is unsafe for the declared use.
- Corpus, case-set, evaluator, or artifact lineage is lost.
- Reproduction fails or artifacts are corrupted.

Rollback means withdrawing or reverting a capability, configuration, or claim. Deployment and infrastructure mechanisms are deferred.

## 13. Required invariants

1. Retrieval is permission-filtered before model access.
2. Citations are authorized and revalidated before display.
3. Retrieved content is untrusted data, not executable instruction.
4. Cross-tenant retrieval or citation is a security failure.
5. Missing or unreliable evidence may require refusal or abstention.
6. Typed proposals remain advisory until Go validation, authorization, approval, and execution.
7. Intelligence output cannot authorize, approve, or execute business changes.
8. Every reported result identifies corpus, case-set, evaluator, and version lineage.
9. No threshold or result is presented as established without repository evidence.
10. Evaluation or claim rollback is required when evidence becomes invalid.
11. Any cache or index used by evaluation is tenant- and authorization-scoped, and its namespace or partition, version, and hit/miss lineage are recorded where applicable.

## 14. Deferred decisions

The following remain open: corpus construction, prompt and case-set authoring, adjudication rules, retrieval systems, caches, indexes, model/provider selection, output validation implementation, numerical targets, promotion thresholds, identity/session enforcement, and the broader Issue #7 security, reliability, recovery, and performance evidence protocol.

## 15. Related documents

- [Product constitution](../../PRODUCT.md)
- [Logical architecture](../../ARCHITECTURE.md)
- [Threat model](../security/THREAT_MODEL.md)
- [Authorization model](../security/AUTHORIZATION_MODEL.md)
- [Forecasting evaluation protocol](FORECASTING_EVALUATION_PROTOCOL.md)
- [Governed-RAG evaluation schema](../evaluation/schemas/GOVERNED_RAG_EVALUATION_SCHEMA.md)
- [Governed-RAG evaluation report template](../evaluation/templates/GOVERNED_RAG_EVALUATION_REPORT.md)
