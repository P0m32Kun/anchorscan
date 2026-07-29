---
name: trellis-continue
description: "Resume work on the current task. Loads the workflow Phase Index, figures out which phase/step to pick up at, then pulls the step-level detail via get_context.py --mode phase. Use when coming back to an in-progress task and you need to know which workflow step is next."
---

# Continue Current Task

Resume work on the current task — pick up at the right phase/step in `.trellis/workflow.md`.

## Step 1: Load Current Context

```bash
python3 ./.trellis/scripts/get_context.py
```

## Step 2: Load the Phase Index

```bash
python3 ./.trellis/scripts/get_context.py --mode phase
```

## Step 3: Decide Where You Are

Route by `task.json.status` and artifact presence. This command does not itself approve implementation.

- `status=planning` + no `prd.md` → **1.1** (`trellis-brainstorm`).
- `status=planning` + `prd.md` only → decide lightweight vs. complex; complex work returns to **1.1** for `design.md` and `implement.md`.
- `status=planning` + complex artifacts complete + sub-agent JSONL only contains seed rows → **1.3**.
- `status=planning` + required artifacts and curated JSONL (or inline mode) → **1.4**; run `task.py validate --ready` before `task.py start`.
- `status=in_progress` + implementation not started → **2.1**.
- `status=in_progress` + implementation done, not yet self-checked → **2.2** (write-capable self-check).
- `status=in_progress` + self-check passed → Standards review → Spec/AC review → full verification → PR → **3.3** (spec update) → **3.4** (commit).
- `status=completed` → archive flow.

For behavioral or high-risk work, the required execution order is: TDD Red → Green → self-check → Standards review → Spec/AC review → full verification → PR. `trellis-check` may fix code during self-check; it is not an independent review. Use `code-review` or equivalent isolated read-only reviewers from the recorded fixed point and authoritative ticket/spec.

## Step 4: Load the Specific Step

```bash
python3 ./.trellis/scripts/get_context.py --mode phase --step <X.X> --platform codex
```

Run required steps in order. Lightweight tasks may be PRD-only; complex tasks require `design.md` and `implement.md`.
