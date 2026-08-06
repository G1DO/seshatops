# M0 Security Model Review - Issue #5

This document records a documentation review of the SeshatOps threat model and authorization model. It records that the review occurred; it does not claim runtime security, verified tenant isolation, completed authentication, a working policy engine, or successful penetration testing.

## 1. Review record

| Field | Value |
| --- | --- |
| Reviewer | Codex documentation implementation/review pass; maintainer review remains a recorded follow-up |
| Date (UTC) | 2026-08-06 |
| Branch / base | docs/5-threat-authorization-model; base commit 6a0fbf9 |
| Reviewed tip | 365d23fc29b482a1e862eeb17cb638e37f02ee6e |
| Scope | Issue #5 threat model, authorization model, review evidence, and README discoverability link |
| Review type | Documentation pre-commit / pre-PR review |
| Result | Pass with recorded follow-ups |

The reviewed tree contains documentation only. No application code, authentication setup, policy engine, runtime configuration, database schema, API route, middleware, credential, or infrastructure artifact was introduced.

## 2. Sources reviewed

### GitHub and repository sources

- GitHub Issue #5, M0: Define threat model and authorization assumptions.
- GitHub PR #15 and reviewed implementation tip `365d23fc29b482a1e862eeb17cb638e37f02ee6e`.
- GitHub Issues #1 through #4 and merged PRs #11 through #14.
- README.md, PRODUCT.md, CLEAN_ROOM.md, and ARCHITECTURE.md.
- docs/architecture/EVENT_MODEL.md and docs/architecture/COMMAND_MODEL.md.
- docs/adrs/0001-transactional-outbox-and-at-least-once-delivery.md.
- docs/adrs/0002-idempotent-command-execution.md.
- docs/checklists/CLEAN_ROOM_REVIEW.md.
- docs/reviews/M0_ARCHITECTURE_REVIEW.md.
- docs/reviews/M0_CORRECTNESS_MODEL_REVIEW.md.

### Canonical planning sources

- SeshatOps - Master Project Blueprint.
- Workflow - Notion -> GitHub -> Evidence.
- M0 - Project Constitution.

No Ahoy repository or private Ahoy artifact was inspected or used.

## 3. Asset, actor, and trust-boundary coverage

| Review area | Coverage |
| --- | --- |
| Assets | Tenant business state; identity/session context; policies and assignments; approvals; commands and receipts; events and audit records; forecasts, proposals, retrieved documents, and citations; credentials; evidence and exports; transactional availability and integrity |
| Human actors | End user, approver, tenant administrator, platform operator, malicious external actor, malicious or compromised tenant user |
| Service and system actors | External identity provider, browser client, Go platform, Python intelligence, asynchronous publishers/consumers, external operational adapter, compromised service identity |
| Trust classes | Trusted logical authority, partially trusted bounded component, untrusted input/content |
| Boundaries | Browser-Go, External identity provider-Go, Go-PostgreSQL, Go-Redpanda, Go-object storage, Go-Python, Go-adapter, Python-approved inputs, asynchronous publisher/consumer, tenant, administrative/operational |
| Logical/deployment distinction | Explicitly preserved; no network topology, ports, cloud accounts, subnets, or deployment manifest is defined |

The trust-boundary Mermaid diagram is in THREAT_MODEL.md. The diagram is logical and does not claim that a runtime boundary has been implemented.

## 4. Threat-to-control-and-test matrix

| Threat | Primary assets | Control references | Future negative test | Deferred owner |
| --- | --- | --- | --- | --- |
| T-01 Cross-tenant reads/writes | A-01, A-06, A-07, A-09, A-10 | AUTH-01, AUTH-02, AUTH-03 | Exercise every read, retrieve, cite, approve, command, export, audit, replay, and recovery path across tenants | Issue #5; later identity/operations; Issue #7 evidence |
| T-02 Identifier substitution / object access | A-01, A-04, A-05, A-06, A-07, A-09 | AUTH-03 | Replace resource, approval, receipt, event, citation, and export identifiers with unauthorized or colliding identifiers | Issue #5; later API implementation |
| T-03 Confused deputy | A-01, A-03, A-04, A-05, A-06 | AUTH-12, AUTH-13 | Use an authorized service with an unauthorized initiating principal, tenant, target, action, or scope | Issue #5; later service/adapters |
| T-04 Stale authority | A-02, A-03, A-04, A-05 | AUTH-01, AUTH-07, AUTH-11 | Change role, scope, membership, session, policy, or target state between checkpoints | Issue #5; later identity/operations |
| T-05 Replayed commands/approvals | A-04, A-05, A-10 | AUTH-06, AUTH-07, AUTH-10, AUTH-11; CM-02 to CM-04 | Replay after retry, timeout, restart, and redelivery; verify one durable effect | Issue #4, Issue #5, adapters, Issue #7 |
| T-06 Approval substitution/parameter change | A-03, A-04, A-05 | AUTH-06, AUTH-07, AUTH-11; Section 7.2 approval binding | Alter tenant, target, action, material parameter, intent, or target version after approval | Issue #5; later workflow |
| T-07 Compromised session/token | A-02, A-03, A-04, A-05, A-09 | AUTH-01, AUTH-05, AUTH-07 | Exercise expired, revoked, altered, wrong-tenant, and stolen-session fixtures | Later identity/operations; Issue #7 |
| T-08 Service over-privilege/compromise | A-01, A-03, A-05, A-06, A-08 | AUTH-12, AUTH-13 | Attempt each service identity's disallowed tenant, read, write, approve, command, storage, and audit operation | Issue #5; later deployment |
| T-09 Python authority | A-01, A-03, A-05, A-06, A-07, A-08 | AUTH-08 | Supply output that attempts authorization, command, policy override, write, or cross-tenant access | Issue #5; later intelligence |
| T-10 Prompt injection | A-03, A-05, A-07, A-09 | AUTH-08, AUTH-09 | Inject instructions to reveal context, expand scope, bypass approval, execute, or falsify citations | Issue #5; Issue #6 |
| T-11 Retrieval leakage | A-01, A-07, A-09 | AUTH-02, AUTH-09 | Use colliding identifiers, semantic similarity, stale cache, wrong filters, and replayed retrieval context | Issue #5; Issue #6; later retrieval |
| T-12 Unauthorized/misleading citations | A-03, A-07, A-09 | AUTH-09 | Return inaccessible, stale, conflicting, wrong-tenant, or unsupported citations | Issue #6 |
| T-13 Retrieved content drives commands | A-03, A-05, A-07 | AUTH-08, AUTH-09 | Embed tool, SQL, policy-override, or parameter-change instructions in retrieved content | Issue #5; Issue #6; later approved actions |
| T-14 Audit tampering/deletion/omission/forgery | A-03, A-04, A-05, A-06, A-09 | AUTH-10 | Attempt to delete, rewrite, omit, forge, replay, or cross-tenant read audit records | Issue #5; Issue #7; later governance |
| T-15 Forged/misleading receipts | A-05, A-06, A-09, A-10 | AUTH-10; CM-07, CM-08 | Supply forged, altered, wrong-tenant, stale, or success-labeled uncertain receipts | Issue #4, Issue #5; later implementation |
| T-16 Event/command tenant mismatch | A-01, A-05, A-06, A-10 | AUTH-02, AUTH-03; EM-08 | Cross-wire tenant, aggregate, command, causation, and consumer context | Issue #4, Issue #5; later consumers |
| T-17 Payload/integrity conflict | A-01, A-05, A-06, A-07 | AUTH-03, AUTH-10; EM-06; CM-03 | Reuse event, command, approval, receipt, proposal, or idempotency identity with changed content | Issue #4, Issue #5; later implementation |
| T-18 Excessive exposure | A-02, A-06, A-07, A-08, A-09 | AUTH-02, AUTH-07, AUTH-10 | Trigger errors, denials, retries, quarantine, exports, traces, citations, and evidence with sensitive inputs | Issue #5; Issue #7; later governance |
| T-19 Availability abuse | A-01, A-03, A-05, A-10 | AUTH-01, AUTH-02, AUTH-11 | Exhaust authorization, retrieval, event, replay, export, and receipt paths for one tenant or service | Issue #7; later reliability |
| T-20 Identity-provider assertion / tenant-context substitution | A-02, A-03, A-09 | AUTH-01, AUTH-02, AUTH-05, AUTH-07 | Exercise swapped-tenant, stale-membership, revoked-session, forged-admin, and contradictory assertions against display, approval, execution, export, and audit | Issue #5; later identity/operations; Issue #7 |
| T-21 Privileged administrator/operator boundary misuse | A-01, A-03, A-04, A-05, A-06, A-09 | AUTH-02, AUTH-03, AUTH-07, AUTH-12, AUTH-13 | Exercise tenant-admin, platform-operator, recovery, replay, audit, export, and privileged-service actions outside their assigned boundary | Issue #5; later identity/operations and governance; Issue #7 |

Every threat entry in THREAT_MODEL.md contains the required asset, actor, entry boundary, consequence, preventive control, detective/recovery control, future negative test, residual risk, and deferred owner fields.

## 5. Authorization-invariant traceability

| Invariant | Primary coverage | Related threat coverage |
| --- | --- | --- |
| AUTH-01 Missing, unknown, invalid, stale, or ambiguous context defaults to deny | AUTHORIZATION_MODEL.md sections 3 and 5 | T-01, T-04, T-07, T-19, T-20, T-21 |
| AUTH-02 No tenant can read or affect another tenant | AUTHORIZATION_MODEL.md section 6 | T-01, T-02, T-11, T-16, T-18, T-19, T-20, T-21 |
| AUTH-03 Go owns authoritative authorization | AUTHORIZATION_MODEL.md sections 3 and 5 | T-02, T-03, T-09, T-16, T-17, T-21 |
| AUTH-04 Browser visibility never grants authority | AUTHORIZATION_MODEL.md sections 2 and 5 | T-02, T-07, T-13 |
| AUTH-05 Authentication does not imply authorization | AUTHORIZATION_MODEL.md sections 2 and 3 | T-04, T-07, T-20 |
| AUTH-06 Approval does not replace execution-time authorization | AUTHORIZATION_MODEL.md sections 2 and 7 | T-05, T-06, T-15 |
| AUTH-07 Checks occur at display, approval, and execution | AUTHORIZATION_MODEL.md section 7 | T-04, T-05, T-06, T-07, T-18, T-20, T-21 |
| AUTH-08 Python never owns or overrides authorization | AUTHORIZATION_MODEL.md sections 2, 8, and 10 | T-09, T-10, T-13 |
| AUTH-09 Retrieved content cannot become executable instruction | AUTHORIZATION_MODEL.md section 10 | T-10, T-11, T-12, T-13 |
| AUTH-10 Audit records and receipts are not trusted from untrusted callers | AUTHORIZATION_MODEL.md section 11 | T-14, T-15, T-17, T-18 |
| AUTH-11 Changed intent/state/approval/role/scope/session/policy can invalidate execution | AUTHORIZATION_MODEL.md sections 7 and 12 | T-04, T-05, T-06, T-17, T-19 |
| AUTH-12 Service identity preserves initiating context | AUTHORIZATION_MODEL.md sections 3, 8, and 9 | T-03, T-08, T-16, T-21 |
| AUTH-13 Service identities use explicitly granted least privilege | AUTHORIZATION_MODEL.md sections 8 and 12 | T-03, T-08, T-21 |

## 6. Future negative-test inventory

The following tests are required future scenarios, not executed tests:

1. Cross-tenant access with colliding resource identifiers.
2. Missing and ambiguous tenant context.
3. Identifier substitution for resources, approvals, receipts, events, citations, and exports.
4. Direct invocation of a browser-hidden or disabled action against Go.
5. Role, scope, policy, session, approval, and target-version changes between display, approval, and execution.
6. Replayed and concurrent commands or approvals.
7. Changed material parameters under a reused approval or idempotency key.
8. Over-privileged or compromised service identities.
9. Forged, stale, swapped, or contradictory identity-provider assertions and tenant context.
10. Tenant-administrator, platform-operator, recovery, audit, replay, and privileged-service actions outside their assigned boundary.
11. Python attempts to write state, authorize, command, or cross tenants.
12. Prompt injection and malicious retrieved instructions.
13. Unauthorized, stale, conflicting, and misleading citations.
14. Event and command tenant-context mismatch.
15. Tampered payloads and conflicting logical identities.
16. Forged, altered, uncertain, and wrong-tenant receipts.
17. Audit deletion, alteration, omission, and forgery.
18. Sensitive exposure through errors, logs, traces, exports, quarantine, and evidence.
19. Authorization work amplification and availability abuse.

No test in this inventory is represented as having passed.

## 7. Consistency checks

### ARCHITECTURE.md

- Go remains the logical owner of authentication integration, authorization enforcement, tenant isolation, workflow/approval orchestration, commands, receipts, audit, and replay coordination.
- Python remains advisory and cannot write business state, own authorization, or execute operational commands.
- The browser remains a presentation/client boundary and never an authorization authority.
- PostgreSQL, Redpanda, and object storage retain their existing logical responsibilities.
- The new documents refine least-privilege and delegated-context requirements without inventing concrete database roles or credentials.
- The existing logical-versus-deployment distinction remains intact.

### EVENT_MODEL.md

- Tenant context remains part of event meaning and must agree with aggregate and processing context.
- Producer identity supports provenance and validation but does not grant consumer authority.
- Cross-tenant, malformed, unsupported, conflicting, or unsafe events remain quarantine/reconciliation cases.
- Replay remains controlled reprocessing and cannot repeat irreversible effects.
- No event schema, topic, partition, retention, or consumer implementation was added.

### COMMAND_MODEL.md

- Commands remain controlled state-changing requests through Go.
- Business-intent idempotency remains distinct from transport retry.
- Approval remains bound to tenant, target, important parameters, normalized intent, and relevant target version.
- Authorization remains rechecked immediately before execution.
- Receipts continue to distinguish platform decisions from independently confirmed external completion.
- The security model adds principal, tenant, service, policy, freshness, and confused-deputy semantics without redefining the command schema.

## 8. Contradictions, ambiguities, and dispositions

| ID | Finding | Disposition |
| --- | --- | --- |
| C-01 | The Master Blueprint names OIDC, database roles, secret-management behavior, and concrete RAG controls; Issue #5 forbids selecting products or implementing infrastructure | Preserve as future control goals and assumptions only; defer products, schemas, and runtime enforcement |
| C-02 | ARCHITECTURE.md has a high-level Go credential boundary but does not define service/database role detail | Refine conceptually to least privilege and explicit delegation; defer concrete role and credential design |
| C-03 | Existing event/command documents already mention tenant context, approvals, receipts, and execution rechecks | Cross-reference and extend security semantics; do not duplicate or redefine Issue #4 correctness |
| C-04 | Tenant administrator, platform operator, approver, and service authority are not an exhaustive role catalog | Define separation and scope principles only; defer the final role/assignment catalog |
| C-05 | Identity provider, session lifecycle, revocation, policy representation, cache invalidation, and cryptographic protection are unspecified | Record as unresolved implementation decisions for later identity/operations and governance milestones |
| C-06 | The older Blueprint progress snapshot can lag the repository and current M0 page | Treat GitHub/repository documents as current technical execution truth per the Workflow page; record as freshness risk, not a security contradiction |

## 9. Assumptions and unresolved decisions

- Authentication integration will establish a principal and session context, but no provider, token format, or session protocol is selected.
- Tenant context must be validated server-side and cannot be trusted from a client field.
- Go is the authoritative policy enforcement boundary in the logical architecture, but no runtime enforcement exists.
- Python can receive approved inputs and return advisory results, but cannot authorize, approve, write business state, or command an adapter.
- Service identity, delegated principal, tenant, resource, action, scope, approval, and lineage must be preserved together, but the wire/storage representation is deferred.
- Data classification, retention, redaction, audit storage, receipt verification, cache/index isolation, and cryptographic controls require later decisions.
- The final role and assignment catalog must not be inferred from the illustrative personas in this document.

## 10. Deferred implementation work and owners

| Work | Owner |
| --- | --- |
| Conceptual threat model, authorization invariants, service-identity boundaries, and future negative tests | Issue #5 |
| Forecast/RAG evaluation protocol, injection corpus, citation scoring, and refusal evaluation | Issue #6 |
| Security/reliability evidence protocols, availability campaigns, recovery evidence, and measurable assurance | Issue #7 |
| Integrated constitution and adversarial cross-document review | Issue #10 |
| Identity provider, session/token lifecycle, policy engine, role catalog, concrete service/database credentials, adapter authority, audit/receipt protection, and runtime enforcement | Later identity/operations and implementation milestones |
| API routes, schemas, middleware, caches, database structures, cryptographic details, deployment topology, and runtime configuration | Later implementation milestones |

## 11. Residual risks

- The model is documentation only; no runtime control or tenant-isolation result exists.
- Authentication, authorization, service identity, retrieval isolation, audit protection, and receipt verification remain unimplemented.
- External adapters may produce uncertain outcomes requiring adapter-specific reconciliation.
- Model and retrieved-content behavior remains adversarially unverified until Issue #6.
- Availability limits and operational recovery evidence remain deferred to Issue #7.
- Manual review and clean-room checks remain necessary until later repository governance work.

## 12. Clean-room confirmation

- No Ahoy repository or private Ahoy artifact was accessed.
- No private code, schema, migration, data, log, trace, screenshot, identifier, business rule, incident, or production behavior was used.
- All examples and terms are generic SeshatOps concepts or the existing fictional Northstar Foods scenario.
- No private identifier denylist was created.
- The documents are safe for eventual public release subject to later clean-room review.

## 13. Verification record

The following checks are required for this documentation-only change and must be reported honestly:

| Check | Result |
| --- | --- |
| Changed-path allowlist | Pass; exactly README.md plus the three Issue #5 documents are changed |
| git diff --check | Pass; no whitespace errors in tracked changes |
| Markdown-link target validation | Pass; repository-relative links in all changed files resolve |
| Required threat, asset, actor, boundary, checkpoint, and invariant search | Pass; required coverage terms and IDs are present |
| Threat-row field completeness | Pass; 21 threat entries each contain all required fields |
| Authorization-invariant completeness | Pass; 13 invariants each map to document sections and future negative tests |
| Cross-document consistency review | Completed conceptually; no runtime claim |
| Clean-room category search | Pass; no private Ahoy material, private identifiers, or secret values found |
| Runtime, authentication, policy-engine, penetration, and security-control tests | Not applicable; not run |

No runtime or security-control test result is implied by this review.
