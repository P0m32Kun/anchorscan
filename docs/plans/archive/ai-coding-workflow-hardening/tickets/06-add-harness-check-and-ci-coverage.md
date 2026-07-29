# 06 — 添加 harness-check 并接入 PR CI

**What to build:** 为 AI 工作流自身增加低成本、可重复的回归检查，并让它成为 PR 质量门的一部分。

**Blocked by:** 05 — 完成项目 spec bootstrap。

**Status:** done

**Delivery:** PR #8（`f1a46f4`）已合并，`quality-gate` 通过。

**Execution skills:** `implement`、`tdd`、`code-review`。

## 行为契约

- `make harness-check` 在干净 checkout 无外部 GitHub 写权限下可运行。
- 它检测 workflow、task gate、spec 完整性和平台 prompt 的关键契约。
- `make pr-check` 调用它，PR CI 因 harness 回归失败。

## 测试 seam

- Node 标准库脚本/fixture 测试；不需要新增依赖。
- 只检查项目内已跟踪文件，不依赖当前 session/runtime state。

## 实施

1. 先添加故意缺 TDD/review、0-entry JSONL、spec placeholder 的 fixture/失败断言。
2. 新增 `scripts/check_ai_workflow.mjs`。
3. 增加 `Makefile` target 并接入 `pr-check`。
4. 只检查稳定的语义锚点，避免对大段 prompt 做脆弱字符串精确匹配。
5. 更新文档说明本地和 CI 用法。

## 验收

- [ ] `make harness-check` 通过。
- [ ] 删除 TDD/review/gate 关键锚点会导致失败。
- [ ] 恢复 backend/frontend placeholder 会导致失败。
- [ ] `make pr-check` 在本地和 PR workflow 都包含 harness-check。
- [ ] 脚本不访问网络、不读用户目录、不依赖未跟踪 runtime 文件。

## 非目标

- 不在 CI 中写 GitHub ruleset；远端设置由 ticket 01 的独立核验处理。
