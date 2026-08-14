# Contributing

SeshatOps follows Career → Workflow. Notion owns product context and roadmap.
GitHub owns execution. This repository owns implemented technical truth.

```text
Notion product context
        → outcome GitHub milestone
        → small issue
        → short-lived branch
        → implementation + tests
        → pull request
        → CI + review
        → main
```

Create the next GitHub milestone only when that outcome is being designed and
executed. Do not copy GitHub issue status into Notion.

## Issues

Use the **Engineering change** template. A normal issue needs a goal, acceptance
criteria, important constraints, and verification. Non-goals are optional.
Add failure, security, or recovery detail only when the change is high-risk.

## Pull requests

Use the repository PR template: what changed, why, and how it was verified.
One coherent reviewable change. Title states why. Body includes `Closes #N`.
Squash merge is preferred. Update affected docs in the same PR when practical.

## Checks

Commands live in [README.md](README.md). Docker is required for Testcontainers
Go tests. Hosted checks on a PR are Go CI, Web CI, and Documentation CI.

## Boundaries

Clean-room: [CLEAN_ROOM.md](CLEAN_ROOM.md). Coding-agent contract:
[AGENTS.md](AGENTS.md). Public claims may not exceed [EVIDENCE.md](EVIDENCE.md).
