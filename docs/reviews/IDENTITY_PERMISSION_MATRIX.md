# Identity & Operations permission matrix note

Issue #44 published the frozen Northstar demo allow-list at
[PERMISSION_MATRIX.md](../security/PERMISSION_MATRIX.md).

This note records documentation review only. It does not claim that
authentication, authorization, tenant isolation, or privileged-ops controls
exist, and it does not promote `CAP-010`, `CLM-007`, or `CLM-008`.

## Acceptance

| Criterion | Disposition |
| --- | --- |
| Matrix reviewed and frozen for this milestone | `MX-001`–`MX-007` are the only grants; `TENANT-NS-002` has none |
| Privileged vs read actions are explicit | `ACT-READ` is read; quarantine/replay/rebuild/audit actions are privileged |
| Default-deny gaps are called out | Section 8; missing or unknown action is deny |
| Downstream API and UI can reference IDs | `MX-*`, `TENANT-NS-*`, `ROLE-*`, `RES-*`, `ACT-*` |

## Non-goals confirmed

No runtime enforcement, OIDC session, ops-visibility product surface,
quarantine/replay controls, audit recording, policy-engine product, second
ERP tenant seed, or `EVIDENCE.md` claim change.
