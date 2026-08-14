# Clean-room review checklist

Copy the record into a pull-request description. Policy: [CLEAN_ROOM.md](../../CLEAN_ROOM.md).
Checked boxes mean a review occurred, not that no issue exists. Do not commit a
private identifier denylist.

```markdown
### Clean-room record

| Field | Value |
| --- | --- |
| Author | |
| Date (UTC) | |
| Commit / tip | |
| Scope (paths / PR) | |
| Result | Pass / Fail / Pass with remediation |

#### Checks

- [ ] No Ahoy or other private code, schemas, migrations, data, logs, traces, or production config
- [ ] No private screenshots, recordings, or exports
- [ ] No production identifiers, hostnames, internal URLs, or private account/tenant IDs
- [ ] No private business-specific rules, recipes, prices, customers, suppliers, or process knowledge
- [ ] No secrets, credentials, tokens, or private environment files
- [ ] No raw AI conversations or prompt histories containing private context
- [ ] New material has a permitted source or recorded synthetic provenance
- [ ] Uncertain provenance excluded (not sanitized and kept)
- [ ] Category search over scope for exclusion terms and secret-like strings
- [ ] Public artifacts remain explainable without private systems
- [ ] No private denylist of real identifiers was added

#### Findings

(None, or list each finding.)

#### Remediation

(None, or list actions taken.)
```
