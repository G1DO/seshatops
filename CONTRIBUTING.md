# Contributing

Operating model: [Career → Workflow](https://app.notion.com/p/3bb0a821b3cc817394cdf93a936a3612)
(Notion). This file is repository-specific contribution practice. GitHub owns
execution (issues, PRs, CI). This repository owns implemented technical truth.

```text
GitHub issue → short-lived branch → implement + tests → PR → CI/review → main
```

## Issues

Use [.github/ISSUE_TEMPLATE/engineering.md](.github/ISSUE_TEMPLATE/engineering.md):
goal, acceptance criteria, important constraints, verification. Non-goals are
optional. Add failure, security, or recovery detail only when the change is
high-risk.

Work the issue assigned to you. If you want an unassigned issue, request
assignment first. Do not start work on an issue that already has an assignee.

## Pull requests

Use [.github/pull_request_template.md](.github/pull_request_template.md).
What changed, why, and how it was verified. One coherent change. Title states
why. Body includes `Closes #N`. Squash merge preferred. Update affected docs
in the same PR when practical.

## Checks

Commands: [README.md](README.md). Docker is required for Testcontainers Go
tests. Hosted checks: Go CI, Web CI, Documentation CI. Branch protection and
rulesets are unavailable on the current GitHub private plan; CI still runs on
pull requests and `main`.

## Boundaries

[CLEAN_ROOM.md](docs/CLEAN_ROOM.md). Agents: [AGENTS.md](AGENTS.md). Claims:
[EVIDENCE.md](docs/EVIDENCE.md).
