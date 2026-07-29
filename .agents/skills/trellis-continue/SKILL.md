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
- `status=in_progress` + self-check passed → Standards review → Spec/AC review → full verification → evidence/ticket/归档准备 → delivery PR。
- `status=completed` → archive flow.

For behavioral or high-risk work, the required execution order is: TDD Red → Green → self-check → Standards review → Spec/AC review → full verification → PR. `trellis-check` may fix code during self-check; it is not an independent review. Use `code-review` or equivalent isolated read-only reviewers from the recorded fixed point and authoritative ticket/spec.

`delivery.commit` 和 `delivery.pr` 是完成交付引用；`delivery.merged_at` 只是在 PR 合并后可观察到的可空值，不能阻塞 complete 或 archive，也不能引出纯元数据后续 PR。用户已授予持续自治时，分支、提交、push、PR、合并、归档和 journal 都直接执行；只为产品范围、安全/权限、未知并行工作，或 Trellis 上游、全局安装、npm 发布等外部持久变更升级。

## Step 4: Load the Specific Step

```bash
python3 ./.trellis/scripts/get_context.py --mode phase --step <X.X> --platform codex
```

Run required steps in order. Lightweight tasks may be PRD-only; complex tasks require `design.md` and `implement.md`.
