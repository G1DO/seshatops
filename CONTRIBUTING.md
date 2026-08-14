# Contributing

This repository owns implemented technical truth. GitHub owns execution
(issues, PRs, CI).

## Issues

Use the **Engineering change** template: goal, acceptance criteria, important
constraints, verification. Non-goals are optional. Add failure, security, or
recovery detail only when the change is high-risk.

## Pull requests

What changed, why, and how it was verified. One coherent change. Title states
why. Body includes `Closes #N`. Squash merge preferred. Update affected docs
in the same PR when practical.

## Checks

Commands: [README.md](README.md). Docker is required for Testcontainers Go
tests. Hosted checks: Go CI, Web CI, Documentation CI.

## Boundaries

[CLEAN_ROOM.md](CLEAN_ROOM.md). Agents: [AGENTS.md](AGENTS.md). Claims:
[EVIDENCE.md](EVIDENCE.md).
