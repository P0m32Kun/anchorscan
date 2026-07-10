# Project Agent Instructions

## Communication
- Always communicate with the user in Chinese (中文).

## Workflow
- This project uses project-scoped Comet for long-running product evolution.
- Use `/comet` for new capabilities, architecture changes, multi-step refactors, and changes that need persistent design/verification history.
- Use normal Codex/Superpowers flow for small bug fixes, docs tweaks, and trivial config edits.
- Do not install or rely on global Comet for this project; keep Comet artifacts project-scoped under `.codex/`, `.comet/`, `openspec/`, and `skills-lock.json`.
- For non-trivial new work, use a stronger model for planning and write the plan to a durable file first; after the plan is landed, tell the user they can switch to a more cost-effective model for implementation.
- If implementation reveals that the plan is invalid, stop and tell the user to switch back to a stronger model for replanning.
