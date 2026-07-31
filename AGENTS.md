# Project Agent Instructions

## Communication
- Always communicate with the user in Chinese (中文).

## Workflow
- Select the smallest applicable model-invoked skill from its metadata; user-only skills require an explicit request.
- Resolve the outcome, constraints, and observable acceptance criteria before editing.
- Keep durable specifications and plans under `docs/plans/` only for multi-session work, team handoff, or a genuinely large decision space.
- Use the local tracker contract in `docs/agents/issue-tracker.md` for tracked feature work.
- Verify changes proportionately and report exact commands, observed results, and any unverified items.

## Agent skills

- Testing strategy: see `docs/testing-strategy.md` — choose the lowest sufficient test seam and avoid duplicate cross-layer coverage by default.
- Issue tracker: see `docs/agents/issue-tracker.md`.
- Domain docs: see `docs/agents/domain.md` — read `CONTEXT.md` at repo root before exploring; check `docs/adr/` for decisions touching the area you're working in.
