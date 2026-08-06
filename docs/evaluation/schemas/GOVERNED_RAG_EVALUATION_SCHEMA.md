# Governed-RAG Evaluation Schema

**Status:** Planned. This is a conceptual Markdown field catalog for future governed-RAG evaluation records. It is not JSON Schema, Protobuf, a database schema, an API contract, or a source-code type.

**Evidence rule:** Every result, target, benchmark value, threshold, and pass status remains `Planned`, `Not evaluated`, or `TBD — evidence required` until supported by repository evidence. Empty fields are intentional.

## Record catalog

| Field | Meaning | Initial value/status |
| --- | --- | --- |
| Evaluation run ID | Stable identity for one reproducible run | Planned |
| Case-set identity/version | Version of the evaluated cases and adjudication rules | Planned |
| Capability name/version | Governed-RAG capability under evaluation | Planned |
| Query identity | Stable identity for the evaluation query | Planned |
| Principal context | Authorized principal and relevant context reference | Planned |
| Tenant context | Tenant boundary for the case | Planned |
| Authorized corpus identity | Independently determined eligible corpus reference | Planned |
| Corpus snapshot/version | Versioned corpus and document state | Planned |
| Document/chunk lineage | Source, version, chunk, owner, classification, and freshness references | Planned |
| Authorization decision lineage | Policy/context reference proving eligibility | Planned |
| Retrieval configuration/version | Version of retrieval and ranking behavior | Planned |
| Retrieved candidates | Returned candidates, ranks, and authorization disposition | Not evaluated |
| Retrieval checks | Relevance, ranking, empty-result, stale, conflict, and leakage checks | Not evaluated |
| Response/refusal/abstention | Generated outcome and availability status | Not evaluated |
| Claim inventory | Material claims requiring support | Planned |
| Citation set | Citation references and lineage | Not evaluated |
| Citation checks | Existence, support, authorization, freshness, and completeness checks | Not evaluated |
| Prompt-injection checks | Injection category, observed behavior, and disposition | Not evaluated |
| Leakage checks | Tenant, principal, trace, cache, index, and artifact leakage checks | Not evaluated |
| Typed proposal | Advisory proposal fields, if present | Not evaluated |
| Proposal validation status | Conceptual validity and unavailable/rejected disposition | Planned |
| Model/evaluator version | Version lineage for generation and evaluation | Planned |
| Code commit | Repository commit used where applicable | Planned |
| Configuration | Non-secret configuration affecting the run | Planned |
| Environment | Declared execution environment and dependencies | Planned |
| Deterministic seed | Seed where relevant; otherwise reason not applicable | Planned |
| Run timestamp/freshness basis | Time and evidence-freshness context | Planned |
| Failures | Failed, incomplete, unsafe, or inconclusive cases | Planned |
| Abstentions/refusals | Conditions and observed safe/unsafe behavior | Not evaluated |
| Artifacts | Versioned inputs, outputs, reports, and sanitized manifests | Planned |
| Reviewer | Reviewer identity or role | Planned |
| Claim status | Status of any proposed capability claim | Not evaluated |
| Disposition | Review disposition and follow-up state | Planned |
| Limitations and exclusions | Known boundaries and unsupported conditions | Planned |
| Reproduction information | Instructions and dependencies required to reproduce | Planned |
| Rollback/invalidation record | Withdrawal or invalidation information if applicable | Planned |

## Record rules

- The eligible authorized corpus must be recorded separately from retrieved candidates.
- Unauthorized content must not be copied into evaluation artifacts.
- A citation or proposal reference is not authority by itself.
- A typed proposal is advisory and cannot become an executable command through free-form interpretation.
- Missing lineage, authorization context, or reproducibility evidence prevents the affected result from being established.

## Related documents

- [Governed-RAG evaluation protocol](../../intelligence/GOVERNED_RAG_EVALUATION_PROTOCOL.md)
- [Governed-RAG evaluation report template](../templates/GOVERNED_RAG_EVALUATION_REPORT.md)
- [Forecasting evaluation schema](FORECASTING_EVALUATION_SCHEMA.md)
