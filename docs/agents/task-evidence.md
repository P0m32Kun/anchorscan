# Task 权威引用与质量证据

跨会话、多 ticket 工作的需求与验收唯一权威是 `docs/plans/`。Trellis task 保存执行状态，
并用 `task.json.meta.source_of_truth` 指向权威 spec/ticket；它不得复制需求文本。

## `task.json` 元数据

行为任务使用如下对象；轻量任务可将 `type` 设为 `trellis-prd`，但不得同时拥有
`docs/plans/` 的同一需求副本。

```json
{
  "meta": {
    "source_of_truth": {
      "type": "docs-ticket",
      "spec": "docs/plans/<feature>/spec.md",
      "ticket": "docs/plans/<feature>/tickets/<nn>-<slug>.md"
    },
    "risk": "behavioral",
    "fixed_point": "<runtime-recorded-sha or null>"
  }
}
```

路径相对于仓库根目录，`type` 仅可为 `docs-ticket` 或 `trellis-prd`。新行为任务必须
记录 `fixed_point`；历史 task 若没有可验证值必须显式写为 `null`，并在证据文件说明，
不得猜测 SHA。

## `quality-evidence.json`

行为任务将证据保存在 task 目录。`schema` 当前为 `1`；每个结论的 `result` 使用
`passed`、`failed`、`unobserved` 或 `not_applicable`，不能用空字段暗示通过。

```json
{
  "schema": 1,
  "approval": {"recorded_at": "<ISO-8601 or null>", "summary": "...", "result": "passed"},
  "tdd": {
    "required": true,
    "red": {"command": "...", "result": "passed|failed|unobserved|not_applicable", "summary": "..."},
    "green": {"command": "...", "result": "passed|failed|unobserved|not_applicable", "summary": "..."}
  },
  "verification": [{"command": "...", "result": "passed", "at": "<ISO-8601 or null>"}],
  "reviews": {
    "standards": {"result": "passed|failed|unobserved", "artifact": "<path or null>"},
    "spec": {"result": "passed|failed|unobserved", "artifact": "<path or null>"}
  },
  "delivery": {"branch": "...", "commit": "...", "pr": "<URL>", "merged_at": "<ISO-8601 or null>"}
}
```

`docs` 风险任务可设 `tdd.required: false`，并以 `not_applicable` 明示 Red/Green；其他
字段仍须记录实际观察结果。`delivery.commit` 与 `delivery.pr` 是 complete/归档所需的
交付引用；`delivery.merged_at` 只是可为空的远端观测值，绝不是 complete、archive 或 journal
的前置条件。交付 ticket、质量证据和归档应随同一个 delivery PR 提交；PR 合并后不得仅为了
补录 `merged_at`、archive 或 journal 再创建纯元数据 PR。该格式是 task lifecycle gate 的文档契约。

可解析 fixture 位于
[`fixtures/docs-only-quality-evidence/`](fixtures/docs-only-quality-evidence/)。
