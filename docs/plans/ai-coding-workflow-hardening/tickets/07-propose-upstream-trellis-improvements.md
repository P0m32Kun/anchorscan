# 07 — 形成 Trellis 上游改进提案

**What to build:** 在项目级流程经真实任务验证后，将通用机制整理为可讨论的 Trellis 上游 issue/PR 提案。

**Blocked by:** 06 — 添加 harness-check 并接入 PR CI；且至少两个真实行为变更已通过新流程完成。

**Status:** ready-for-agent

**Execution skills:** `research`、`code-review`。

## 行为契约

- 提案只包含跨项目可复用的机制，不包含 AnchorScan 测试命令、目录或领域规则。
- 提案有实际使用证据、最小 API/schema、迁移/兼容性分析和测试建议。
- 未取得用户后续授权前，不修改 Trellis 上游、全局安装或 npm 包。

## 候选内容

- `validate --ready/--complete` 的可配置 gate 框架；
- source-of-truth 和质量 evidence 的通用 metadata schema；
- self-check 与 read-only review 角色分离；
- hook doctor：区分 configured、manually observed、automatically observed；
- sub-agent context 的 mode-aware 0-entry 校验。

## 验收

- [ ] 至少两个 AnchorScan 真实任务的证据表明 gate 降低了已知风险。
- [ ] 提案明确哪些规则属于项目配置而非框架默认。
- [ ] 提案包含向后兼容和迁移策略。
- [ ] 用户明确授权前不创建上游 PR 或修改全局安装。

## 非目标

- 不承诺在本项目计划内完成 Trellis 上游合并。
