# Threat Model - SeshatOps

**Status:** Planned security design for Issue #5. This document defines the threat model and security properties that future implementation must preserve. It is not evidence that authentication, authorization, isolation, monitoring, or any other control exists.

**Owns:** Security scope, assets, actors, trust boundaries, threat classes, planned control objectives, negative-test intent, and residual-risk framing.

**Does not own:** Concrete identity providers, policy languages, API routes, middleware, tokens, cryptographic protocols, database tables, secret-management products, deployment topology, penetration testing, or the full RAG evaluation protocol owned by Issue #6.

## 1. Scope and security assumptions

SeshatOps is a clean-room, multi-tenant operations-intelligence platform. The public logical architecture consists of a browser client, a Go-owned transactional platform, PostgreSQL, Redpanda, object storage, Python intelligence, asynchronous publishers and consumers, and an external operational adapter represented publicly by the synthetic ERP.

The model protects:

- tenant business state and operational decisions;
- the identity and session context used to make decisions;
- policies, assignments, approvals, commands, receipts, events, and audit records;
- forecasts, proposals, retrieved documents, and citations;
- credentials, evidence artifacts, and exports; and
- the confidentiality, integrity, and availability of core transactional operations.

The following are design assumptions, not verified properties:

1. Go is the logical owner of authoritative authorization and state-changing workflows.
2. PostgreSQL is the logical authoritative transactional and governance store; Redpanda is asynchronous transport and replay input.
3. Python is advisory intelligence and has no business-state write or command-execution authority.
4. Browser input, model output, retrieved content, client-supplied identifiers, citations, approvals, receipts, and idempotency keys are untrusted until validated by Go.
5. Asynchronous delivery may duplicate, reorder, delay, or lose an acknowledgement. Issue #4 correctness rules remain applicable.
6. Identity integration is defined to establish a principal and session context per [ADR-0005](../adrs/0005-identity-tenant-policy-and-service-delegation.md) (OIDC Authorization Code + PKCE; Go-owned session). Issue #45 library/test runtime is recorded in [OIDC_SESSION.md](OIDC_SESSION.md). Production authentication, IdP vendor, durable token storage, and authorization enforcement remain deferred.
7. Tenant context must be established and checked by trusted server-side processing. A tenant identifier supplied by a client is only an assertion to validate.
8. Future deployment controls may add network, storage, process, and credential boundaries, but this document defines the public logical architecture only.

Controls described as required, planned, or future are not implemented controls. No runtime security property is claimed until a later implementation and evidence review demonstrates it.

## 2. Trust classification

Trust is contextual. A component can be trusted for one narrow responsibility while remaining untrusted for another.

| Classification | Meaning in this model | Examples |
| --- | --- | --- |
| Trusted logical authority | A future component is designated as the owner of a responsibility, subject to implementation evidence | Go authorization and transactional boundary; PostgreSQL authoritative state |
| Partially trusted | The component may perform a bounded responsibility but must be validated, scoped, and monitored | Identity provider, service identities, asynchronous consumers and publishers, Python, object storage, external adapters, tenant administrators, platform operators |
| Untrusted input or content | The value may be useful data but cannot grant authority or change policy | Browser requests, user text, retrieved documents, model output, client-supplied IDs, citations, approval IDs, receipt IDs, idempotency keys, external actor input |

Trust classification does not mean that any component is currently secure. It defines the validation and authority that future implementation must provide.

## 3. Protected assets

| ID | Asset | Security properties and boundaries |
| --- | --- | --- |
| A-01 | Tenant business state | Confidentiality and integrity of orders, inventory, batches, purchasing, workflows, and other tenant-scoped operations; availability of state-changing transactions |
| A-02 | Identity and session context | Correct principal, tenant context, session freshness, authentication status, and delegated-actor lineage |
| A-03 | Policies and assignments | Integrity and freshness of authorization policy, scopes, assignments, limits, and separation-of-duty decisions |
| A-04 | Approval records | Integrity, authenticity, freshness, tenant binding, target binding, intent binding, and non-repudiable lineage appropriate to the future design |
| A-05 | Commands and durable receipts | Correct intent, idempotency, execution status, uncertainty, target, tenant, authority references, and observed result |
| A-06 | Events and audit records | Tenant context, producer lineage, payload integrity, ordering/version meaning, decision history, denial records, and investigation usefulness |
| A-07 | Forecasts, proposals, retrieved documents, and citations | Tenant/resource authorization, provenance, freshness, citation correctness, confidentiality, and non-executable interpretation |
| A-08 | Secrets and service credentials | Confidentiality, least privilege, bounded delegation, rotation, and separation from user authority |
| A-09 | Evidence artifacts and exports | Confidentiality, tenant scope, integrity, provenance, redaction, and accurate representation of observed versus planned results |
| A-10 | Transactional availability and integrity | Ability to reject unsafe or ambiguous work without silently crossing tenants, losing accepted state, or claiming false success |

## 4. Actors and identities

| ID | Actor or identity | Trust and authority assumptions |
| --- | --- | --- |
| P-01 | Human end user | Untrusted input source. May act only through current authenticated identity, tenant membership, resource scope, and action authorization |
| P-02 | Approver | A human end user with a separate current authorization to approve a specific proposed intent; approval authority is never inferred from visibility or possession of an approval reference |
| P-03 | Tenant administrator | Tenant-scoped administrative principal, not a platform operator by default; exact role catalog and limits remain future decisions |
| P-04 | Platform operator | Operational principal for bounded health, recovery, and audit duties; operational access does not automatically grant tenant business authority |
| P-05 | External identity provider | Partially trusted authentication dependency. It may establish identity assertions, but Go must validate the resulting principal, tenant context, session state, and authorization independently |
| P-06 | Browser client | Untrusted execution environment. It may render and submit requests but cannot enforce authoritative authorization |
| P-07 | Go-owned transactional platform | Trusted logical authority for authorization, workflow decisions, state transitions, commands, receipts, and audit; this is a design ownership statement, not implementation evidence |
| P-08 | Python intelligence capability | Partially trusted advisory service. It may forecast, retrieve, cite, explain, and propose within approved inputs, but it cannot authorize, approve, write business state, or execute commands |
| P-09 | Asynchronous consumers and publishers | Partially trusted service identities. They may transport or process bounded events but must preserve tenant context, lineage, integrity, deduplication, and authorization boundaries |
| P-10 | External operational adapter | Partially trusted operational boundary. It receives only narrowly scoped Go-authorized commands and may return success, failure, or uncertainty; it does not own SeshatOps authorization |
| P-11 | Malicious external actor | Untrusted actor attempting discovery, token theft, injection, replay, denial of service, tampering, or cross-tenant access |
| P-12 | Malicious or compromised tenant user | Authenticated or partially authenticated actor attempting to exceed tenant, resource, action, or scope authority |
| P-13 | Compromised service identity | A service credential or process operating under attacker influence; service identity must not be treated as a user authorization substitute |

The initiating principal and tenant context must survive delegated service calls and remain visible in audit lineage. A service identity never erases, upgrades, or replaces the initiating user authority without an explicitly governed delegation rule.

## 5. Logical trust boundaries

These are logical boundaries. They do not define network topology, cloud accounts, ports, subnets, processes, or deployment manifests.

1. **Browser to Go APIs:** browser requests, identifiers, filters, approval references, receipts, and visible actions cross into an authoritative server boundary. Go must validate identity, tenant, resource, action, scope, and freshness.
2. **External identity provider to Go:** identity assertions cross a partially trusted authentication boundary. Go must validate the resulting principal, tenant membership, session state, and current authorization independently; an assertion does not carry business authority.
3. **Go to PostgreSQL:** authoritative state and governance reads/writes cross into the transactional store. A database capability must not be treated as permission to bypass Go policy.
4. **Go to Redpanda:** event publication or consumption crosses an asynchronous transport boundary. Event tenant context, producer identity, payload integrity, and version meaning require validation.
5. **Go to object storage:** evidence, exports, documents, and other artifacts cross a storage boundary. Object identity and storage access do not grant tenant or business authority.
6. **Go to Python:** approved feature or evidence inputs cross into advisory intelligence. Python output returns as data, not as executable authority.
7. **Go to external operational adapters:** a command crosses into an independent operational system. Go must bind tenant, principal, action, target, intent, approval, and idempotency before sending it.
8. **Python to approved read-only or evidence inputs:** retrieval and feature inputs are authorized before content reaches Python. Retrieved content is untrusted data, not instructions.
9. **Asynchronous publisher and consumer boundaries:** retries, duplicate deliveries, stale messages, conflicting identities, and tenant mismatches must not silently become accepted business effects.
10. **Tenant boundary:** every read, write, retrieval, citation, approval, command, export, audit lookup, replay, and recovery action must remain within its authorized tenant.
11. **Administrative and operational boundary:** platform-level recovery, audit, and health duties must not silently become unrestricted tenant business access.

~~~mermaid
flowchart LR
  subgraph browserBoundary["Browser trust boundary"]
    B["Browser client"]
  end
  subgraph goBoundary["Go authoritative boundary"]
    G["Go transactional platform"]
    PUB["Async publisher"]
    CON["Async consumer"]
  end
  subgraph dataBoundary["Data and transport boundaries"]
    PG["PostgreSQL"]
    BUS["Redpanda"]
    OBJ["Object storage"]
  end
  subgraph intelligenceBoundary["Advisory intelligence boundary"]
    PY["Python intelligence"]
    RET["Approved retrieval and evidence inputs"]
  end
  subgraph externalBoundary["External operational boundary"]
    ERP["Synthetic ERP or future adapter"]
  end
  IDP["External identity provider"]
  B -->|"session and API requests"| G
  IDP -->|"identity assertions"| G
  G <-->|"authoritative state and governance"| PG
  G -->|"publish or consume with validation"| PUB
  PUB --> BUS
  BUS --> CON
  CON --> G
  G -->|"approved context"| PY
  RET -->|"authorized untrusted content"| PY
  PY -->|"advisory output"| G
  G --> OBJ
  PY -->|"bounded artifacts"| OBJ
  G -->|"authorized idempotent command"| ERP
  B -.->|"must not authorize"| PY
  PY -.->|"must not command or write business state"| ERP
  B -.->|"must not access directly"| PG
  B -.->|"must not access directly"| BUS
~~~

The dashed edges are prohibited authority paths, not network-deny or firewall claims.

## 6. Threat handling principles

Future implementation must use these principles:

- Missing, malformed, stale, contradictory, or ambiguous security context fails closed.
- Tenant context is part of resource identity and must be checked independently of resource identifiers.
- A trusted internal caller must not bypass resource-level or tenant-level authorization.
- Service capabilities are least privilege and explicitly granted.
- Data returned for display is authorized for the requesting principal before it is returned.
- Approval is specific to an intended action and does not replace execution-time authorization.
- Model output and retrieved content are advisory or evidence data, never policy or executable instruction.
- Denials, suspicious attempts, integrity failures, and uncertain outcomes remain visible without logging unnecessary sensitive data.
- Safe refusal, quarantine, controlled backpressure, or reconciliation is preferable to silent success or unsafe continuation.

## 7. Threat taxonomy and required future tests

Each threat below identifies the asset, actor, entry boundary, consequence, preventive controls, detective or recovery controls, a future negative test, residual risk, and deferred owner. The controls are requirements for future work, not implemented claims.

### T-01 - Cross-tenant reads and writes

- **Asset:** A-01, A-06, A-07, A-09, A-10.
- **Actor and entry boundary:** P-11 or P-12 through browser/API, retrieval, event, command, export, replay, or administrative paths.
- **Consequence:** Confidentiality breach, unauthorized business effect, corrupted lineage, or tenant-isolation failure.
- **Preventive controls:** AUTH-01 default deny; AUTH-02 tenant-isolation invariant; AUTH-03 server-side tenant/resource binding; tenant checks at every boundary.
- **Detective/recovery controls:** Auditable denials and mismatch events; quarantine unsafe events; controlled investigation and recovery without cross-tenant replay.
- **Future negative test:** Attempt every read, retrieve, cite, approve, command, modify, export, audit, replay, and recovery action using another tenant's context and colliding identifiers.
- **Residual risk:** A missing enforcement point or incorrectly scoped service/cache can still leak data until implementation-wide testing exists.
- **Deferred owner:** Issue #5 defines the invariant; Event Spine identity/operations implementation and Issue #7 evidence own enforcement and measured proof.

### T-02 - Identifier substitution and insecure direct-object access

- **Asset:** A-01, A-04, A-05, A-06, A-07, A-09.
- **Actor and entry boundary:** P-11 or P-12 through client-supplied resource IDs, approval IDs, receipt IDs, event IDs, or citations at the Browser-to-Go boundary.
- **Consequence:** Access to an object outside the principal's scope or mutation of an unintended target.
- **Preventive controls:** AUTH-03 resolve tenant and ownership from trusted context; authorize resource type, identity, action, and scope rather than trusting identifiers.
- **Detective/recovery controls:** Record denials and identifier/context mismatches; refuse or quarantine malformed cross-context commands and events.
- **Future negative test:** Replace each target, receipt, approval, event, citation, and export identifier with a same-type identifier from another tenant or unauthorized scope.
- **Residual risk:** Identifier-only APIs or hidden client assumptions may reintroduce object-level authorization gaps.
- **Deferred owner:** Issue #5 model; later API and service implementation milestone.

### T-03 - Confused deputy behavior

- **Asset:** A-01, A-03, A-04, A-05, A-06.
- **Actor and entry boundary:** P-12 or P-13 through a trusted internal caller, service-to-service request, asynchronous consumer, or adapter.
- **Consequence:** A privileged component performs an action the initiating principal or tenant could not perform.
- **Preventive controls:** AUTH-12 preserve caller service, initiating principal, tenant, resource, action, scope, approval, command binding, and lineage; AUTH-13 require least-privilege service capabilities.
- **Detective/recovery controls:** Audit delegated principal and service identity separately; alert or quarantine missing/mismatched lineage; revoke or rotate compromised service capability.
- **Future negative test:** Use an authorized service identity with an unauthorized initiating principal, tenant, target, action, or scope and require denial.
- **Residual risk:** Delegation semantics and service-to-service context propagation remain unimplemented.
- **Deferred owner:** Issue #5 definition; later identity/operations and adapter milestones.

### T-04 - Stale roles, scopes, sessions, permissions, or policy assignments

- **Asset:** A-02, A-03, A-04, A-05.
- **Actor and entry boundary:** P-12 or P-13 through a long-lived browser session, queued workflow, approval, cache, or asynchronous retry.
- **Consequence:** Revoked or changed authority is used after it should have expired.
- **Preventive controls:** AUTH-01 default deny; AUTH-07 current authorization and freshness at every checkpoint; AUTH-11 treats changed policy, assignment, role, scope, session, membership, or target version as invalidating the attempt.
- **Detective/recovery controls:** Record stale-context denials; expire or revoke affected workflows; require renewed authorization or approval.
- **Future negative test:** Change role, scope, tenant membership, session state, or policy after display and after approval, then require execution denial or renewed approval.
- **Residual risk:** Cache invalidation, revocation timing, and clock/freshness policy are deferred.
- **Deferred owner:** Issue #5 conceptual requirement; later identity/operations implementation.

### T-05 - Replayed commands and approvals

- **Asset:** A-04, A-05, A-10.
- **Actor and entry boundary:** P-11, P-12, or P-13 through repeated API submission, message redelivery, receipt retrieval, or recovered workflow.
- **Consequence:** Duplicate approval, repeated business effect, or authority after expiry.
- **Preventive controls:** AUTH-06 approval does not replace execution-time authorization; AUTH-07 rechecks current authorization and freshness; AUTH-10 prevents caller-supplied receipts from becoming authority; AUTH-11 rejects stale or changed authority; Issue #4 CM-02 through CM-04 provide business-intent idempotency.
- **Detective/recovery controls:** Durable outcome retrieval; duplicate/replay audit records; reconciliation for uncertain downstream outcomes; quarantine conflicting intent.
- **Future negative test:** Repeat the same approval and command, retry after timeout, and replay after restart; verify one durable effect and no stale authority.
- **Residual risk:** External systems may have independent retry or reconciliation behavior.
- **Deferred owner:** Issue #4 correctness model; Issue #5 security binding; later adapter implementation and Issue #7 evidence.

### T-06 - Approval substitution or changed parameters/targets

- **Asset:** A-03, A-04, A-05.
- **Actor and entry boundary:** P-12 or P-13 through proposal, approval, command, or receipt references.
- **Consequence:** A valid human decision is reused to authorize a different target, amount, action, tenant, or intent.
- **Preventive controls:** AUTH-06 keeps approval separate from execution authority; AUTH-07 rechecks approval and authorization at each checkpoint; AUTH-11 invalidates changed intent, approval, or target version; Section 7.2 binds approval to tenant, action, target, important parameters, normalized intent, target version, approver, expiry, and revocation state.
- **Detective/recovery controls:** Compare approval and command intent at approval and execution; record mismatch and refuse execution.
- **Future negative test:** Modify each approval-bound field after approval and require rejection before any effect or a newly bound approval; an authorization failure must never be represented as permission to proceed.
- **Residual risk:** The final set of material parameters and normalization rules is intentionally not a schema decision here.
- **Deferred owner:** Issue #5 conceptual binding; later workflow and policy implementation.

### T-07 - Compromised sessions and stolen tokens

- **Asset:** A-02, A-03, A-04, A-05, A-09.
- **Actor and entry boundary:** P-11 using a browser/session/API credential at the Browser-to-Go boundary.
- **Consequence:** Impersonation, unauthorized display, approval, command, export, or audit access within the stolen session's authority.
- **Preventive controls:** Future authentication integration with bounded sessions; AUTH-01 default deny; AUTH-05 separates authentication from authorization; AUTH-07 requires current authorization, tenant binding, freshness, and approval/execution checks.
- **Detective/recovery controls:** Session revocation and suspicious-use audit; denial telemetry; credential rotation and incident response as future operational controls.
- **Future negative test:** Use an expired, revoked, altered, wrong-tenant, and stolen-session fixture against display, approval, execution, export, and audit operations.
- **Residual risk:** Authentication strength, token format, device binding, and revocation mechanisms are not selected.
- **Deferred owner:** Later identity/operations implementation and Issue #7 evidence.

### T-08 - Over-privileged or compromised service identities

- **Asset:** A-01, A-03, A-05, A-06, A-08.
- **Actor and entry boundary:** P-13 through Go, publisher, consumer, Python, audit, storage, or adapter service credentials.
- **Consequence:** Broad cross-tenant access, silent policy bypass, unauthorized writes, or forged lineage.
- **Preventive controls:** AUTH-12 preserves initiating context; AUTH-13 requires explicit least privilege; separate service responsibilities; service credentials do not substitute for user authority; audit writers cannot gain business-state authority.
- **Detective/recovery controls:** Service-level authorization denials, credential use audit, rotation/revocation, quarantine of invalid producer or consumer context.
- **Future negative test:** Attempt each service identity's disallowed read, write, approve, command, tenant, storage, and audit operation.
- **Residual risk:** Concrete process and credential separation is deferred.
- **Deferred owner:** Issue #5 logical model; later implementation and deployment milestones.

### T-09 - Python gaining transactional or command authority

- **Asset:** A-01, A-03, A-05, A-06, A-07, A-08.
- **Actor and entry boundary:** P-08 or P-13 through Go-to-Python, Python-to-store, retrieval, or model-output paths.
- **Consequence:** Model or compromised intelligence process changes business state, approves actions, bypasses policy, or commands an adapter.
- **Preventive controls:** AUTH-08 Python has no authorization ownership, business-state write credentials, approval authority, or command execution path; Go validates all advisory output.
- **Detective/recovery controls:** Reject non-advisory output; audit Python invocation and initiating context; alert on unauthorized store or adapter access.
- **Future negative test:** Supply a proposal that attempts authorization, direct command, policy override, SQL/write behavior, or cross-tenant retrieval and require refusal.
- **Residual risk:** Runtime credential and process isolation are not implemented.
- **Deferred owner:** Architecture/Issue #5 invariant; later identity/operations implementation.

### T-10 - Prompt injection

- **Asset:** A-03, A-05, A-07, A-09.
- **Actor and entry boundary:** P-11 or P-12 through user text, retrieved documents, model context, or evidence inputs crossing into Python.
- **Consequence:** Scope expansion, policy bypass, hidden-instruction disclosure, unauthorized command proposal, or misleading answer.
- **Preventive controls:** AUTH-09 treat retrieved content and user text as untrusted data; separate policy from content; no model-controlled write tools; authorize retrieval before model input.
- **Detective/recovery controls:** Refusal or unavailable result; preserve prompt/retrieval lineage without unnecessary sensitive content; adversarial evaluation and review under Issue #6.
- **Future negative test:** Inject instructions to reveal hidden context, expand tenant scope, bypass approval, execute a command, or falsify citations.
- **Residual risk:** Model behavior and provider-specific defenses remain uncertain.
- **Deferred owner:** Issue #5 security boundary; Issue #6 full evaluation protocol.

### T-11 - Cross-tenant retrieval leakage

- **Asset:** A-01, A-07, A-09.
- **Actor and entry boundary:** P-11 or P-12 through retrieval, feature inputs, indexes, caches, documents, or evidence exports.
- **Consequence:** Unauthorized disclosure of another tenant's documents, features, operational state, or derived answer.
- **Preventive controls:** AUTH-02 tenant isolation; AUTH-09 authorize and filter before retrieval; tenant-aware cache and index boundaries.
- **Detective/recovery controls:** Leakage negative tests, retrieval audit, cache/index invalidation, refusal and incident handling.
- **Future negative test:** Search with colliding identifiers, semantic similarity, stale cache entries, unauthorized tenant filters, and replayed retrieval context.
- **Residual risk:** Concrete index, cache, and storage isolation are deferred.
- **Deferred owner:** Issue #5 invariant; Issue #6 evaluation; later retrieval implementation.

### T-12 - Unauthorized or misleading citations

- **Asset:** A-07, A-09, A-03.
- **Actor and entry boundary:** P-08 or P-12 through generated citations, document references, answer display, or evidence export.
- **Consequence:** Disclosure of unauthorized source material or a false basis for approval and execution.
- **Preventive controls:** AUTH-09 citations reference only documents authorized for the requesting principal and tenant; revalidate citations before display.
- **Detective/recovery controls:** Mark unavailable or unverifiable citations; record citation lineage and validation failure; Issue #6 adjudicated citation evaluation.
- **Future negative test:** Return citations from another tenant, stale versions, inaccessible documents, or documents that do not support the claim.
- **Residual risk:** Citation correctness and provenance quality require later evaluation.
- **Deferred owner:** Issue #6 evaluation protocol; later retrieval implementation.

### T-13 - Untrusted retrieved content influencing tools or commands

- **Asset:** A-03, A-05, A-07.
- **Actor and entry boundary:** Malicious document or user text crossing Python-to-Go advisory response and proposal-to-command flow.
- **Consequence:** Content becomes executable instruction, changes material parameters, or bypasses human approval.
- **Preventive controls:** Retrieved content cannot grant authority; Python cannot execute commands; typed proposals require Go validation, authorization, approval, and execution checks.
- **Detective/recovery controls:** Reject invalid or unsafe proposals; preserve refusal and validation results; do not expose write tools to intelligence.
- **Future negative test:** Embed tool-call, SQL, policy-override, or parameter-change instructions in retrieved content and require no command effect.
- **Residual risk:** Parser, proposal validation, and model isolation are later implementation concerns.
- **Deferred owner:** Issue #5 boundary; Issue #6 evaluation; later approved-action implementation.

### T-14 - Audit deletion, alteration, omission, or forgery

- **Asset:** A-03, A-04, A-05, A-06, A-09.
- **Actor and entry boundary:** P-13 or P-11 through audit storage, export, replay, service credentials, or client-supplied audit references.
- **Consequence:** Loss of accountability, concealed denial or action, false investigation result, or inability to prove lineage.
- **Preventive controls:** AUTH-10 protect audit integrity; record principal, tenant, action, resource, outcome, reason, policy context, lineage, and timestamps; audit writer cannot gain business authority.
- **Detective/recovery controls:** Detect missing/conflicting records; preserve denial and suspicious-attempt visibility; reconcile audit and command/receipt lineage.
- **Future negative test:** Attempt to delete, rewrite, omit, forge, cross-tenant read, or replay audit records and require denial or visible integrity failure.
- **Residual risk:** Append-only semantics, retention, storage protection, and cryptographic verification are deferred.
- **Deferred owner:** Issue #5 requirements; Issue #7 evidence; later governance implementation.

### T-15 - Forged or misleading durable receipts

- **Asset:** A-05, A-06, A-09, A-10.
- **Actor and entry boundary:** P-11 or P-12 through receipt IDs, client responses, external adapter responses, or reconciliation.
- **Consequence:** Client or operator treats an unexecuted, unauthorized, uncertain, or different action as success.
- **Preventive controls:** AUTH-10 receipt integrity; a supplied receipt reference does not grant authority; receipt binds tenant, command, target, intent, status, actor, approval, and observed result.
- **Detective/recovery controls:** Verification failure is visible and cannot become success; distinguish platform decision from independently confirmed external completion; reconcile uncertainty.
- **Future negative test:** Supply forged, altered, wrong-tenant, stale, or success-labeled uncertain receipts and require rejection or explicit uncertainty.
- **Residual risk:** Storage and verification mechanisms are not selected.
- **Deferred owner:** Issue #4 receipt semantics; Issue #5 security binding; later implementation.

### T-16 - Event or command tenant-context mismatch

- **Asset:** A-01, A-05, A-06, A-10.
- **Actor and entry boundary:** P-09 or P-13 through event envelopes, command requests, consumer context, replay, or adapter calls.
- **Consequence:** State or effects are applied under the wrong tenant, aggregate, principal, or workflow.
- **Preventive controls:** AUTH-02 and AUTH-03 validate tenant against aggregate, target, authorization context, command intent, and processing context; quarantine invalid or missing context; preserve EM-08 behavior.
- **Detective/recovery controls:** Integrity/mismatch audit; quarantine and controlled reconciliation; never apply later or replayed data under a substituted tenant.
- **Future negative test:** Cross-wire tenant IDs, aggregate IDs, command IDs, causation IDs, and consumer contexts, including colliding identifiers.
- **Residual risk:** Concrete envelope and processing schemas remain deferred to later implementation.
- **Deferred owner:** Issue #4 event/command correctness; Issue #5 security semantics; later consumers/adapters.

### T-17 - Tampered payloads and conflicting integrity identities

- **Asset:** A-01, A-05, A-06, A-07.
- **Actor and entry boundary:** P-11 or P-13 through event, command, proposal, receipt, storage, or asynchronous redelivery paths.
- **Consequence:** A valid identity is reused for different content, producing corrupted state, false lineage, or unsafe execution.
- **Preventive controls:** AUTH-03 validates trusted resource and command context; AUTH-10 protects audit and receipt integrity; reject conflicting event IDs or idempotency intent; bind approvals and receipts to material intent and target version; preserve EM-06 and CM-03 behavior.
- **Detective/recovery controls:** Integrity failure, quarantine, reconciliation, and visible non-success outcome; preserve sanitized diagnostic context.
- **Future negative test:** Reuse event, command, approval, receipt, proposal, and idempotency identities with changed payloads or versions.
- **Residual risk:** Concrete integrity and storage verification are deferred.
- **Deferred owner:** Issue #4 correctness; later implementation and Issue #7 evidence.

### T-18 - Excessive data exposure

- **Asset:** A-02, A-06, A-07, A-08, A-09.
- **Actor and entry boundary:** P-11 or P-13 through logs, traces, errors, exports, evidence artifacts, quarantine, citations, or administrative views.
- **Consequence:** Sensitive tenant data, credentials, prompts, retrieved content, or internal security context is disclosed beyond need.
- **Preventive controls:** AUTH-02 tenant isolation; AUTH-07 display and export authorization checks; AUTH-10 protects audit and evidence integrity; minimize and classify logged data; never treat traces or evidence as unrestricted.
- **Detective/recovery controls:** Review and audit exports; redact or revoke exposed artifacts; detect secret-like or cross-tenant content; preserve only sanitized diagnostics.
- **Future negative test:** Trigger errors, denials, retries, quarantine, exports, traces, citations, and evidence generation with sensitive inputs and verify scope/minimization.
- **Residual risk:** Data classification, retention, redaction, and storage controls are not implemented.
- **Deferred owner:** Issue #5 assumptions; Issue #7 evidence; later governance and implementation.

### T-19 - Availability abuse affecting authorization or tenant isolation

- **Asset:** A-01, A-03, A-05, A-10.
- **Actor and entry boundary:** P-11 or P-12 through request floods, authorization work amplification, replay storms, malformed events, retrieval abuse, or administrative recovery.
- **Consequence:** Unsafe fail-open behavior, denial of service, starvation of one tenant, skipped checks, or uncontrolled backlog/replay.
- **Preventive controls:** AUTH-01 and AUTH-02 fail closed on missing authorization and tenant context; AUTH-11 rejects stale or changed authority; bounded work and backpressure; isolate tenant and operational work; do not silently accept work that cannot be safely retained.
- **Detective/recovery controls:** Rate/lag/error visibility, quarantine, controlled rejection, recovery runbooks, and explicit stale/unavailable status.
- **Future negative test:** Exhaust authorization, retrieval, event, replay, export, and receipt paths for one tenant or service and verify no cross-tenant effect or fail-open decision.
- **Residual risk:** Capacity limits, quotas, SLOs, and fault campaigns are deferred.
- **Deferred owner:** Issue #7 evidence protocol; later reliability and operations implementation.

### T-20 - Identity-provider assertion or tenant-context substitution

- **Asset:** A-02, A-03, A-09.
- **Actor and entry boundary:** P-05, P-11, or P-12 through the External-Identity-Provider-to-Go or Browser-to-Go boundary using forged, stale, swapped, or contradictory identity and tenant assertions.
- **Consequence:** Wrong principal, tenant membership, role, or session context is accepted, enabling unauthorized display, approval, command, export, or audit access.
- **Preventive controls:** AUTH-01 default deny; AUTH-02 tenant isolation; AUTH-05 authentication does not imply authorization; AUTH-07 recheck current principal, tenant, membership, policy, and freshness at display, approval, and execution. Go independently validates the resulting authorization context rather than treating an identity assertion as business authority.
- **Detective/recovery controls:** Record sanitized assertion/context mismatches; reject or quarantine contradictory context; expire or revoke affected sessions; preserve investigation lineage without logging raw credentials or assertions.
- **Future negative test:** Present swapped-tenant, stale-membership, revoked-session, forged-admin, and contradictory identity assertions against display, approval, execution, export, and audit operations and require denial.
- **Residual risk:** Provider validation, session binding, revocation timing, and assertion format remain deferred.
- **Deferred owner:** Issue #5 conceptual requirement; later identity/operations implementation and Issue #7 evidence.

### T-21 - Privileged administrator, operator, recovery, or audit boundary misuse

- **Asset:** A-01, A-03, A-04, A-05, A-06, A-09.
- **Actor and entry boundary:** P-03, P-04, P-12, or P-13 through tenant administration, platform operations, recovery, replay, audit, export, or privileged service paths.
- **Consequence:** A tenant administrator acts outside the tenant, a platform operator gains unrestricted business authority, or a compromised privileged service crosses tenants or forges recovery/audit effects.
- **Preventive controls:** AUTH-02 tenant isolation; AUTH-03 Go-owned resource enforcement; AUTH-07 checkpoint rechecks; AUTH-12 preserve initiating principal and tenant context; AUTH-13 least-privilege service capabilities; tenant-administrator, platform-operator, recovery, and audit duties remain separate unless a future governed delegation explicitly permits the action.
- **Detective/recovery controls:** Audit administrative and service identities separately; alert on scope or tenant mismatch; quarantine unsafe recovery/replay; revoke compromised capability; preserve evidence without allowing recovery access to rewrite business truth.
- **Future negative test:** Attempt another tenant's administration, business command, approval, export, audit, replay, and recovery operation using a tenant administrator, platform operator, and privileged service identity; require denial or explicitly governed scoped behavior with preserved lineage.
- **Residual risk:** The Identity & Operations demo role catalog is frozen in [PERMISSION_MATRIX.md](PERMISSION_MATRIX.md). Break-glass rules, catalogs beyond this milestone, recovery authorization runtime, and the platform-operator operating model remain deferred.
- **Deferred owner:** Issue #5 conceptual requirement; later identity/operations and governance implementation; Issue #7 evidence.

## 8. Threat-to-control summary

| Threat IDs | Primary control references | Future evidence owner |
| --- | --- | --- |
| T-01 | AUTH-01, AUTH-02, AUTH-03 | Issue #5, later identity/operations implementation, Issue #7 evidence |
| T-02 | AUTH-03 | Issue #5, later API implementation |
| T-03, T-08 | AUTH-12, AUTH-13 | Issue #5, later service/adapter implementation |
| T-04 | AUTH-01, AUTH-07, AUTH-11 | Issue #5, later identity/workflow implementation |
| T-05 | AUTH-06, AUTH-07, AUTH-10, AUTH-11; CM-02 through CM-04 | Issue #4, Issue #5, later adapters, Issue #7 |
| T-06 | AUTH-06, AUTH-07, AUTH-11; Section 7.2 approval binding | Issue #5, later workflow implementation |
| T-07 | AUTH-01, AUTH-05, AUTH-07 | Later identity/operations implementation, Issue #7 |
| T-09, T-10, T-13 | AUTH-08, AUTH-09 | Issue #5, Issue #6, later intelligence implementation |
| T-11 | AUTH-02, AUTH-09 | Issue #5, Issue #6, later retrieval implementation |
| T-12 | AUTH-09 | Issue #6 |
| T-14, T-15 | AUTH-10; CM-07, CM-08 | Issue #4, Issue #5, Issue #7 |
| T-16 | AUTH-02, AUTH-03; EM-08 | Issue #4, Issue #5, later consumers |
| T-17 | AUTH-03, AUTH-10; EM-06; CM-03 | Issue #4, Issue #5, Issue #7 |
| T-18 | AUTH-02, AUTH-07, AUTH-10 | Issue #5, Issue #7, later governance implementation |
| T-19 | AUTH-01, AUTH-02, AUTH-11 | Issue #7, later reliability implementation |
| T-20 | AUTH-01, AUTH-02, AUTH-05, AUTH-07 | Issue #5, later identity/operations implementation, Issue #7 |
| T-21 | AUTH-02, AUTH-03, AUTH-07, AUTH-12, AUTH-13 | Issue #5, later identity/operations and governance implementation, Issue #7 |

## 9. Assumptions, residual risks, and deferred decisions

Decided for Identity & Operations design by [ADR-0005](../adrs/0005-identity-tenant-policy-and-service-delegation.md) / [IDENTITY_BOUNDARIES.md](IDENTITY_BOUNDARIES.md): OIDC protocol profile, Go-owned session model, tenant visibility via platform membership and tenant-scoped allow-list, allow-list policy representation, and service-delegation boundaries. The demo allow-list is [PERMISSION_MATRIX.md](PERMISSION_MATRIX.md). Issue #45 session runtime is recorded in [OIDC_SESSION.md](OIDC_SESSION.md). Default-deny authorization enforcement remains Issue #46.

The following remain unresolved by design:

- Identity-provider vendor/product, concrete token storage format, and revocation mechanism implementation.
- Assignment schema, scope syntax, and separation-of-duty rules beyond the frozen Identity & Operations demo matrix.
- Concrete service/database roles, credential storage, credential rotation, and adapter authentication.
- Cache and retrieval-index isolation, object-storage controls, audit retention, receipt verification, redaction, and cryptographic details.
- Availability limits, quotas, rate controls, SLOs, and recovery thresholds.
- The full RAG evaluation protocol, corpus, metrics, and release gates.

Issue #5 defines the conceptual security model and negative-test requirements. Issue #6 owns the complete forecasting/RAG evaluation protocol. Issue #7 owns reliability and security evidence protocols. Issue #10 owns integrated constitution review. Concrete enforcement belongs to the later identity/operations and implementation capability sequences.

The principal residual risk is that these are design requirements only. Issue #45 records OIDC/session library/test runtime; production authentication, authorization enforcement, penetration tests, and policy-engine verification do not exist yet.

## 10. Clean-room boundary

This document uses only independently authored SeshatOps concepts, the fictional Northstar Foods scenario, the public repository documents, GitHub Issue #5 and prior public issue/PR outcomes, and the named canonical planning pages. No private Ahoy repository or artifact was inspected or used. No private identifiers, schemas, data, logs, screenshots, business rules, incidents, or private denylist was created.
