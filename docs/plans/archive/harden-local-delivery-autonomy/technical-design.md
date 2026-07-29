# 技术设计：本地交付自治与非递归收尾

## 变更面

| 层 | 责任 |
| --- | --- |
| `docs/agents/task-evidence.md` | 定义 `merged_at` 为可选观测，不参与完成门禁。 |
| `.trellis/workflow.md` | 规定 delivery PR 在合并前包含 task/ticket/归档证据；持续授权下直接执行常规 Git 交付。 |
| `.agents/skills/trellis-{continue,finish-work}` | 让 Codex 本地 skill 跟随同一交付边界。 |
| `.pi/prompts/trellis-{continue,finish-work}.md` | 给 Pi 相同的恢复和收尾指令。 |
| `scripts/check_ai_workflow.mjs` | 静态检查 evidence、workflow、skill 与 prompt 契约。 |
| `scripts/check_ai_workflow.test.mjs` | 通过临时 fixture 证明删除每个关键锚点会失败。 |

## 数据与状态

`quality-evidence.json` 保持 schema v1：

```json
{"delivery":{"branch":"...","commit":"...","pr":"...","merged_at":null}}
```

complete gate 只校验 `commit` 和 `pr`。合并后可从 PR 页面观察 `merged_at`；该观测不能触发
task 状态迁移，也不能生成新的远端提交。

## 验证策略

对 harness 使用 Node 标准库的临时 checkout fixture，这是最低充分 seam：它从调用者角度验证
仓库规则文件缺失关键契约时会失败，不耦合检查器的内部实现。先删除目标锚点得到 RED，再加入
最小检查得到 GREEN，最后运行 `make harness-check` 与 `make pr-check`。
