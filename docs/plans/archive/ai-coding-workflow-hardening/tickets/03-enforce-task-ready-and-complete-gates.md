# 03 — 强制 task ready / complete gate

**What to build:** 将规划和完成证据从 workflow 文本转为 `task.py` 的阻断式校验。

**Blocked by:** 02 — 对齐权威计划并定义任务证据。

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`。

## 行为契约

- `task.py validate <task> --ready` 对缺 branch、fixed point、source ticket、批准、必要 artifacts 或真实 context 返回非零。
- `task.py start` 在 ready gate 失败时不修改 `task.json.status` 和 active pointer。
- `task.py validate <task> --complete` 对缺验收、TDD、验证、双轴 review、commit/PR 返回非零。
- `task.py archive` 在 complete gate 失败时不移动目录、不标记 completed、不产生 archive commit。
- `--force --reason` 是唯一绕过方式，并持久记录原因。

## 测试 seam

- Python task script：聚焦命令/文件系统集成测试或现有最低可用 Python seam。
- Workflow 静态检查：后续 `harness-check`。
- 不需要浏览器、扫描器或产品 E2E。

## 实施

1. 先为 seed-only JSONL、缺 evidence、缺 branch 和 archive 未完成写失败测试。
2. 在 task context/store 中实现 schema 校验与模式感知 JSONL 校验。
3. 在 `cmd_start` 的任何状态写入前调用 ready gate。
4. 在 `cmd_archive` 的任何状态写入/移动前调用 complete gate。
5. 实现 force reason 的安全记录，避免将无关工作树变更纳入提交。
6. 更新 workflow/help 文案和任务 command usage。

## 验收

- [x] 每个新 gate 都有先失败后转绿的证据。
- [x] start/archive 失败保持文件树和任务状态不变。
- [x] inline、sub-agent、bootstrap 豁免的行为被明确测试。
- [x] `task.py validate` 不再把 0-entry context 描述为 planning-ready。
- [x] 现有正常 archive task 可在补齐 required evidence 的 fixture 下通过。

## 完成证据

- 2026-07-29：`python3 scripts/test_task_gates.py -v`，14 项 task lifecycle
  文件系统集成测试通过；其中新增用例先复现了 seed-only、非法 schema、未就绪 source
  ticket、`main` 分支、缺失/越界 JSONL 路径等门禁缺口。
- 2026-07-29：`make pr-check` 通过；`make security-check` 通过。
- 2026-07-29：以 `663f5c1` 为固定点完成独立 Standards 和 Spec 双轴评审；审查发现的
  `main`/错误分支、无效上下文与 CI 未覆盖问题均已修复并复核关闭。

## 非目标

- 不替换 Trellis 内置 task 存储，也不修改全局 CLI。
