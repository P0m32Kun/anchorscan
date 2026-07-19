# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`CONTEXT.md`** at the repo root — the project's ubiquitous language / domain vocabulary. Read it before exploring unfamiliar areas.
- **`docs/adr/`** — read ADRs that touch the area you're about to work in.

This is a single-context repo: there is no `CONTEXT-MAP.md` and no per-package `CONTEXT.md` files. `CONTEXT.md` at the root covers the whole project.

## Keeping it current

When you introduce or rename a domain concept, update `CONTEXT.md` in the same change. When you make a decision worth recording, add an ADR under `docs/adr/` rather than only noting it in a ticket or PR description.
