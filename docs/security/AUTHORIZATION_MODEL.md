# Authorization Model - SeshatOps

**Status:** Planned authorization design for Issue #5. This document defines decisions and invariants for future implementation. It does not claim that authentication, authorization, tenant isolation, approval enforcement, service identity controls, or audit protection currently exist.

**Owns:** Conceptual authorization semantics, default-deny behavior, tenant isolation, authorization checkpoints, service identity boundaries, confused-deputy protections, and the relationship between authorization and evidence.

**Does not own:** An identity provider, token format, policy language, API schema, route, middleware, database table, database role, cryptographic protocol, authorization library, deployment topology, or the full RAG evaluation protocol owned by Issue #6.

## 1. Purpose and scope

SeshatOps must execute only authorized, human-approved, idempotent state-changing commands through the Go-owned transactional boundary. Authorization is a current security decision, not a property inferred from a user interface, an identifier, a model response, an approval reference, a receipt, or a service credential.

This model applies to:

- display and investigation;
- retrieval, forecasts, proposals, and citations;
- approval and rejection;
- commands and external operational adapters;
- events, projections, replay, recovery, and quarantine;
- audit records, receipts, exports, logs, traces, and evidence; and
- administrative and operational actions.

All authorization controls in this document remain Planned. Event Spine packages implement the event path; authentication, authorization runtime, tenant isolation enforcement, approval enforcement, and audit protection are not claimed as implemented.

## 2. Four separate concepts

| Concept | Definition | What it does not imply |
| --- | --- | --- |
| Authentication | Establishes a principal and session context from an identity assertion and associated freshness state | Authentication does not imply authorization, tenant membership, approval authority, or command authority |
| Authorization | Decides whether the current principal or explicitly delegated service may perform an action on a resource within a tenant and scope under current policy and state | Authorization does not imply approval or successful execution |
| Approval | Records a governed human or policy decision for one specific proposed intent, target, tenant, and relevant version | Approval does not replace execution-time authorization, freshness checks, idempotency, or target-state validation |
| Execution | Performs an authorized and, where required, approved state-changing command through the Go-owned transactional boundary | Execution cannot be inferred from a proposal, UI action, receipt supplied by a client, or Python output |

The following statements are mandatory:

- Authentication does not imply authorization.
- Authorization does not imply approval.
- Approval does not replace execution-time authorization.
- Python recommendations do not imply authority.
- UI visibility does not grant authority.
- Possession of an identifier, approval ID, receipt, or idempotency key does not grant authority.

## 3. Principal and authorization context

The authoritative decision context is established by Go from trusted server-side inputs and current state. Client-supplied values are requests or assertions to validate, not authority.

The context must preserve, where applicable:

- the authenticated principal;
- the initiating tenant and any explicitly delegated tenant context;
- the calling service identity;
- the resource type and resource identity;
- the requested action;
- the applicable scope and contextual constraints;
- the current resource state and version;
- the relevant policy and assignment version;
- session and decision freshness;
- approval and separation-of-duty context; and
- correlation and causation lineage.

A service identity may authenticate a process or call, but it does not become the human principal and does not inherit unrestricted user authority. Delegated actions must preserve both the initiating principal and the service identity for authorization and audit.

## 4. Conceptual authorization tuple

Authorization is conceptually evaluated over this tuple:

> Tenant, principal, resource type, resource identity, action, scope or contextual constraints, current resource state, relevant policy and assignment version, and current time or freshness context where required.

The tuple is a design concept, not an implementation type, policy language, API shape, or database schema. An implementation may represent it differently only if it preserves the same meaning and default-deny behavior.

Authorization must not be reduced to:

- a role name without tenant and resource scope;
- possession of a resource identifier;
- a browser-visible button;
- a prior approval;
- a service-to-service trust relationship;
- a model recommendation;
- a client-provided tenant field; or
- a successful authentication response.

## 5. Default deny

Future enforcement must default to denial when:

1. The principal is unknown, unauthenticated, expired, revoked, or not fresh enough.
2. The tenant is unknown, missing, ambiguous, or not bound to the principal and resource.
3. The resource type, resource identity, action, or scope is unrecognized.
4. Ownership or tenant association cannot be established.
5. Required policy, assignment, state, approval, or freshness information is missing.
6. A service-to-service permission is not explicitly granted.
7. The caller supplies only an identifier, approval ID, receipt, citation, or idempotency key without current authority.
8. The request crosses a tenant, administrative, or operational boundary without explicit authorization.
9. The context is inconsistent across a browser request, event, command, proposal, approval, receipt, or service call.
10. The system cannot safely determine whether the requested action is permitted.

Python cannot make or override authorization decisions. Browser code cannot enforce authoritative authorization. A trusted internal caller cannot bypass resource-level or tenant-level checks.

## 6. Tenant isolation invariant

> No tenant may read, retrieve, cite, approve, command, modify, export, audit, replay, recover, or otherwise affect another tenant's data or operations.

This invariant applies even when:

- resource identifiers collide;
- a service identity is trusted for a different tenant;
- an event, command, receipt, citation, cache entry, or approval reference is valid in another context;
- a user is a tenant administrator in one tenant;
- a platform operator is performing recovery or audit work;
- a request is repeated or replayed;
- a client or external system supplies a matching identifier; or
- an asynchronous message arrives without valid tenant context.

Tenant isolation must cover:

| Surface | Required security meaning |
| --- | --- |
| Transactional state | Reads and writes are authorized against current tenant and resource ownership |
| Events and consumers | Event tenant, aggregate, producer, and processing context agree before application |
| Commands and receipts | Intent, target, approval, idempotency, and observed outcome remain tenant-bound |
| Retrieval and documents | Permission filtering happens before content is supplied to intelligence |
| Forecasts and proposals | Derived outputs inherit tenant and resource authorization |
| Caches and indexes | Cached or indexed content cannot be returned across tenants |
| Logs, traces, exports, and evidence | Observability and evidence access remain scoped and minimized |
| Administration | Tenant administration does not silently become platform-wide authority |
| Replay and recovery | Reprocessing cannot apply another tenant's history or effects |

## 7. Authorization checkpoints

Authorization and freshness are checked at each checkpoint. Earlier success does not replace a later check.

### 7.1 Display

Before returning a sensitive resource, proposal, forecast, citation, audit record, receipt, export, or available action, Go must establish:

- current principal and tenant;
- resource type, identity, ownership, and scope;
- permitted action, such as view, investigate, approve, export, or audit;
- current policy and assignment context;
- data classification and disclosure limits; and
- freshness sufficient for the displayed action.

The UI may hide actions for usability, but it is never the authoritative enforcement point.

### 7.2 Approval

Before accepting an approval, Go must verify:

- the approver is currently authenticated and authorized for the approval action;
- the proposal, target, and approval context belong to the same tenant;
- the approval is bound to the intended action, target, important parameters, normalized intent, and relevant target/resource version;
- the proposal and approval have not expired, changed, or been revoked;
- any separation-of-duty requirement is satisfied; and
- the approval is associated with the initiating principal, approver, service context, correlation, and causation lineage needed for audit.

Approval IDs and approval records are references, not bearer authority.

### 7.3 Execution

Immediately before the Go-owned transactional boundary performs a state-changing command, it must recheck:

- current principal, service identity, and tenant;
- current permission and policy assignment;
- target ownership, resource identity, and current target state/version;
- approval validity, freshness, tenant binding, and intent equivalence;
- command intent and material parameter equivalence;
- idempotency state and any existing durable outcome;
- separation-of-duty or dual-control requirements; and
- the ability to record the decision and resulting receipt safely.

A role, scope, tenant membership, policy, approval, target state, target version, or intent change during the workflow invalidates that execution attempt. Go must fail closed and reject the command before any state-changing or external effect. A renewed authorization or approval may create a new valid attempt. An already-authorized downstream effect may remain explicitly uncertain until reconciled, but authorization failure itself is never an uncertain permission to proceed.

## 8. Service identities and delegated actions

Service identities are technical principals with narrow capabilities:

1. Each service receives only permissions required for its responsibility.
2. Python has no business-state write credentials and no command-execution authority.
3. Publishers and consumers cannot silently act across tenants.
4. External adapters receive narrowly scoped command authority from Go and do not own SeshatOps policy.
5. Audit writers cannot gain general business-state authority merely because they write audit records.
6. Service credentials do not substitute for end-user authorization when a service acts on a user's behalf.
7. Delegated actions preserve the initiating principal, tenant, service identity, resource, action, scope, approval, and lineage in the security context and audit record.
8. A storage, broker, database, or object capability is not by itself permission to perform a business action.
9. Compromise of one service identity must not imply unrestricted access to other services, tenants, or responsibilities.

Concrete service accounts, database roles, credential storage, rotation, process isolation, and deployment controls remain deferred.

## 9. Confused-deputy protections

Any privileged Go-owned or future service boundary must validate all context relevant to the requested action:

- calling service identity;
- initiating principal;
- tenant;
- resource type and identity;
- action;
- scope and contextual constraints;
- current resource state and version;
- approval and command binding;
- correlation and causation lineage; and
- policy and assignment version.

A trusted internal caller must not bypass tenant-level or resource-level authorization. A service may be allowed to read or transport data without being allowed to approve, command, modify, export, or audit it. A tenant administrator may be allowed to manage tenant assignments without becoming a platform operator or another tenant's administrator.

## 10. Prompt injection, retrieval, and citations

The security boundary for intelligence is:

- Retrieved content is untrusted data, not trusted instruction.
- User-provided text is untrusted data, not policy.
- Model output is advisory and cannot authorize actions.
- Retrieval is filtered and authorized before content is supplied to Python.
- Citations reference only documents the requesting principal may access.
- Generated citations are revalidated before display or evidence export.
- Prompt or retrieved content cannot expand tenant, resource, action, or scope authority.
- Prompt or retrieved content cannot bypass approval, reveal hidden instructions, create credentials, or cause a command.
- Unsafe, unauthorized, stale, conflicting, or unverifiable results are refused or clearly marked unavailable.
- Python has no write tools or command path.

Issue #6 owns the complete RAG and forecasting evaluation protocol, including corpus design, adjudication, metrics, and release gates.

## 11. Audit records and durable receipts

Security-relevant decisions must preserve, subject to minimization and future retention policy:

- principal and service identity;
- tenant;
- action and resource;
- outcome and reason category;
- relevant policy and assignment context;
- approval and command binding;
- correlation and causation lineage; and
- decision, execution, observation, and receipt timestamps.

Denials, suspicious attempts, authorization-context mismatches, integrity failures, and uncertain outcomes must remain auditable without logging unnecessary secrets, prompts, retrieved sensitive content, or raw credentials.

Audit records and receipts must be protected against unauthorized modification. A receipt is evidence of a platform decision and observed result, not authority and not automatic proof of every independent external effect.

The future system must ensure:

- a client-supplied receipt is not accepted merely because it is well formed;
- audit or receipt verification failure is visible and cannot silently become success;
- an unresolved external outcome remains visibly uncertain until reconciled;
- an audit writer cannot rewrite business truth; and
- replay cannot forge, erase, or silently replace security-relevant history.

Detailed storage, retention, redaction, immutability, and cryptographic implementation are deferred.

## 12. Authorization invariants

| ID | Required invariant |
| --- | --- |
| AUTH-01 | Missing, unknown, invalid, stale, or ambiguous authorization context defaults to deny |
| AUTH-02 | No tenant can read or affect another tenant |
| AUTH-03 | Go owns authoritative authorization and resource-level enforcement |
| AUTH-04 | Browser visibility never grants authority |
| AUTH-05 | Authentication does not imply authorization |
| AUTH-06 | Approval does not replace execution-time authorization |
| AUTH-07 | Authorization and freshness are rechecked at display, approval, and execution |
| AUTH-08 | Python never owns or overrides authorization and cannot execute commands |
| AUTH-09 | Retrieved content cannot become executable instruction or expand authority |
| AUTH-10 | Audit records and receipts cannot be trusted solely because an untrusted caller supplied them |
| AUTH-11 | Changed intent, target state, approval, role, scope, session, or policy can invalidate later execution |
| AUTH-12 | Service identity does not erase initiating tenant and principal context |
| AUTH-13 | Service identities have explicitly granted least-privilege capabilities and cannot expand, replace, or erase initiating principal or tenant authority |

These invariants complement, rather than replace, Issue #4 invariants CM-02 through CM-09 and EM-06 through EM-09.

## 13. Future negative-test themes

Future implementation tests must include at least:

- cross-tenant access with colliding resource IDs;
- missing and ambiguous tenant context;
- identifier substitution for resources, approvals, receipts, events, citations, and exports;
- invoke an action directly against Go when the browser hides or disables it, and require server-side denial;
- role, scope, policy, session, approval, and target-version changes between display, approval, and execution;
- replayed and concurrent commands or approvals;
- altered material parameters under a reused approval or idempotency key;
- compromised or over-privileged service identities;
- forged, stale, swapped, or contradictory identity-provider assertions and tenant context;
- tenant-administrator, platform-operator, recovery, audit, replay, and privileged-service actions outside their assigned boundary;
- Python attempts to write, authorize, command, or cross tenants;
- prompt injection and malicious retrieved instructions;
- unauthorized, stale, conflicting, or misleading citations;
- event and command tenant-context mismatch;
- tampered payloads and conflicting logical identities;
- forged, altered, uncertain, or wrong-tenant receipts;
- audit deletion, alteration, omission, and forgery;
- sensitive data exposure through errors, logs, traces, exports, quarantine, and evidence;
- authorization work amplification and availability abuse.

The full test inventory and evidence ownership are recorded in the Issue #5 review document.

## 14. Deferred implementation decisions

The following are deliberately not selected here:

- identity provider, authentication protocol, session/token format, and revocation mechanism;
- policy language, policy engine, role catalog, assignment schema, and scope syntax;
- API routes, request/response schemas, middleware, and generated clients;
- database tables, indexes, database roles, row-level mechanisms, caches, and storage layout;
- cryptographic receipt or audit protocols;
- secret-management products, credential rotation mechanisms, and deployment topology;
- external adapter contracts, reconciliation protocols, and retry schedules; and
- RAG evaluation protocol and model/provider-specific safeguards.

Issue #5 owns the conceptual security model. Issue #6 owns intelligence evaluation. Issue #7 owns security and reliability evidence protocols. Issue #10 owns integrated constitution review. Concrete enforcement belongs to later identity/operations and implementation capability sequences.

## 15. Clean-room boundary

This document is independently authored from the SeshatOps public repository, the approved Issue #5 requirements, prior public SeshatOps issue/PR outcomes, and the named canonical planning pages. No private Ahoy repository or artifact was accessed or used. No private identifiers, schemas, data, logs, screenshots, business rules, incidents, or private denylist was created.
