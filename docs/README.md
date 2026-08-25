# Documentation

Implemented technical truth for the **bounded `v0.1.0` local release** (disposable
Compose on `127.0.0.1` only). Product intent and roadmap live in Notion.
Contribution practice: [CONTRIBUTING.md](../CONTRIBUTING.md). Release notes: [../CHANGELOG.md](../CHANGELOG.md).
Security scope and reporting: [../SECURITY.md](../SECURITY.md).

| Area | Document | What it proves |
| --- | --- | --- |
| Prerequisites & quickstart | [getting-started.md](getting-started.md) | One-command `local-stack.sh quickstart`, lifecycle, headless smoke, demos |
| Configuration | [architecture/overview.md](architecture/overview.md) + [getting-started.md](getting-started.md#configuration) | `SESHATOPS_*` env, compose bindings, OIDC assignments |
| Architecture & topology | [architecture/overview.md](architecture/overview.md) | As-built packages, `compose.yaml` services, data flow |
| API & event contracts | [api/openapi-projection.yaml](api/openapi-projection.yaml), [event-spine.md](design/specifications/event-spine.md), [stockout-evaluation.md](design/specifications/stockout-evaluation.md), [forecast-feature-snapshots.md](design/specifications/forecast-feature-snapshots.md) | Wire, persistence, failure dispositions, forecast boundary |
| Operations & recovery | [operations/README.md](operations/README.md), [operations/observability.md](operations/observability.md), [runbooks](operations/runbooks/broker-interruption.md) | Bounded metrics/logs, 4 runbooks exercised |
| Compatibility & toolchain | [COMPATIBILITY.md](COMPATIBILITY.md), [event-spine.md](design/specifications/event-spine.md) §9 | Pinned digests, Go/Node/Python/PostgreSQL/Redpanda versions |
| Evidence | [EVIDENCE.md](EVIDENCE.md) + [evaluation/](evaluation/) + [evaluation/RELEASE_AUDIT_REPORT.md](evaluation/RELEASE_AUDIT_REPORT.md) | Measured local/test evidence vs unsupported claims |
| Troubleshooting | [TROUBLESHOOTING.md](TROUBLESHOOTING.md) + [getting-started.md](getting-started.md#troubleshooting) | Docker, bootstrap/forecast failures, OIDC/metrics, reset guardrails |
| Known limitations | [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md) | Residual risks, unsupported production conclusions, deferred/rejected |
| Security | [../SECURITY.md](../SECURITY.md), [security/authorization.md](security/authorization.md) | Default-deny, tenant isolation, audit-before-mutate |
| Decisions | [decisions/](decisions/) | ADRs 0001/0003/0004/0005 |
| Clean-room | [CLEAN_ROOM.md](CLEAN_ROOM.md) | Northstar provenance, synthetic data |

Campaigns:

- Event Spine: [evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md](evaluation/EVENT_SPINE_EXIT_GATE_EXPERIMENT_REPORT.md)
- Identity HTTP: [evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md](evaluation/IDENTITY_OPERATIONS_EXIT_GATE_EXPERIMENT_REPORT.md)
- M3 traceability/recovery: [evaluation/M3_TRACEABILITY_RECOVERY_EXIT_GATE_EXPERIMENT_REPORT.md](evaluation/M3_TRACEABILITY_RECOVERY_EXIT_GATE_EXPERIMENT_REPORT.md)
- M4 stockout-intelligence: [evaluation/M4_STOCKOUT_INTELLIGENCE_EXIT_GATE_EXPERIMENT_REPORT.md](evaluation/M4_STOCKOUT_INTELLIGENCE_EXIT_GATE_EXPERIMENT_REPORT.md)
- M5 demos: `SESHATOPS_DEMO_CONFIRM=… ./scripts/local-stack.sh demo all` (evidence in gitignored `.release-evidence/`)
- M5 runbook exercise: [evaluation/RUNBOOK_EXERCISE_REPORT.md](evaluation/RUNBOOK_EXERCISE_REPORT.md)
- Release audit: [evaluation/RELEASE_AUDIT_REPORT.md](evaluation/RELEASE_AUDIT_REPORT.md)

UI setup: [web/README.md](../web/README.md). Agents: [AGENTS.md](../AGENTS.md).
