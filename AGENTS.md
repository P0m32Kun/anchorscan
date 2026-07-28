# Project Agent Instructions

## Communication
- Always communicate with the user in Chinese (中文).

## Workflow
- Use the engineering skills installed from `mattpocock/skills` when their workflow materially improves correctness.
- Use the full plan -> implement -> TDD -> code-review workflow for behavior changes, cross-layer work, migrations, or changes with non-trivial regression risk.
- Documentation-only, configuration-only, formatting, dependency pinning, and trivial fixes may be implemented directly with proportionate verification.
- Keep durable specifications and plans under `docs/plans/` only when the knowledge must survive across sessions or coordinate multiple tickets. Do not create a durable plan for a single-session obvious change.
- Use the local tracker contract in `docs/agents/issue-tracker.md` for tracked feature work; implement one ready frontier ticket at a time.
- Mark a ticket done only after required verification and review complete.
- If implementation proves an approved plan materially invalid, update the plan before continuing.

## Trellis

- Trellis is initialized under `.trellis/`; its task lifecycle and commands are defined in `.trellis/workflow.md`.
- Existing work tracked in `docs/plans/` and `docs/agents/issue-tracker.md` remains authoritative. Do not migrate it to, or duplicate it in, Trellis without user approval.
- For new durable work, get user approval before creating a Trellis task. Trellis task planning, implementation, and checks must still satisfy this repository's testing, review, and completion gates.
- `session_auto_commit` uses Trellis's default (`true`); review any journal or archive commits it creates.

## Agent skills

- Testing strategy: see `docs/testing-strategy.md` — choose the lowest sufficient test seam and avoid duplicate cross-layer coverage by default.
- Issue tracker: see `docs/agents/issue-tracker.md`.
- Domain docs: see `docs/agents/domain.md` — read `CONTEXT.md` at repo root before exploring; check `docs/adr/` for decisions touching the area you're working in.
