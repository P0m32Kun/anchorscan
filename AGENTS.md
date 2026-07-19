# Project Agent Instructions

## Communication
- Always communicate with the user in Chinese (中文).

## Workflow
- Use the engineering skills installed from `mattpocock/skills` under `~/.agents/skills`.
- Keep durable specifications and implementation plans under `docs/plans/`.
- Use the local tracker contract in `docs/agents/issue-tracker.md`; implement one ready frontier ticket at a time.
- Use `implement` for an approved plan, with `tdd` at planned seams and `code-review` before completion.
- If implementation reveals that an approved plan is invalid, stop and revise the plan before continuing.

## Agent skills

- Issue tracker: see `docs/agents/issue-tracker.md`.
- Domain docs: see `docs/agents/domain.md` — read `CONTEXT.md` at repo root before exploring; check `docs/adr/` for decisions touching the area you're working in.
