# 移除默认扫描规则的 Nuclei 精确模板能力

## Goal

保证默认自动扫描仅通过 Nuclei tags 调度外部模板库，避免重新引入仓库模板路径和打包耦合，同时保留操作者显式单工具模板能力。

## Requirements

- 默认 `service-tags.yaml` 规则不再支持 `template:` 字段；旧自定义规则出现该字段时，加载必须失败并提示迁移到 `nuclei_tags`，不得静默忽略或仅警告。
- 移除默认规则模型中的精确模板字段及相对路径解析。
- 移除默认扫描管线的 `nuclei -t` 分支和只为该分支存在的 provenance 处理。
- 默认扫描的现有 tags、exclude-tags 和检测状态语义保持不变。
- CLI/Web 单工具的显式 `--template` 功能继续可用。
- 测试不得继续把默认规则的模板路径能力锁定为契约。
- 文档明确区分默认 tags 调度与操作者显式模板执行。

## Non-goals

- 不删除底层 `RunNucleiTemplate`，因为单工具仍使用它。
- 不删除历史 Run 或 artifact 中的旧模板路径。
- 不内置或复制任何 Nuclei 模板。

## Acceptance Criteria

- [ ] 默认规则模型、配置加载和扫描执行路径均不再接受或执行 `template:`；含该字段的旧自定义规则必须产生明确迁移错误。
- [ ] 默认 tags 与 exclude-tags 路径的现有测试继续通过。
- [ ] 单工具显式 `--template` 的单元/集成契约继续通过。
- [ ] 仓库和发布归档均不包含 Nuclei 模板。
- [ ] 文档不再暗示默认扫描支持仓库相对模板路径。
