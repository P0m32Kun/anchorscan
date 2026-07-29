# Technical Design

## Boundary

父任务只协调三个可独立验证的子任务，不直接修改产品代码。它保留 release-version、ScanEvent/DetectionCheck 历史事实和扫描授权范围的既有契约。

## Cross-cutting Contracts

- 构建版本来源：源码回退为 `dev`；只有构建输入通过 linker 注入的版本可作为正式显示版本。
- Console 展示：`ScanEvent` 是用户界面的摘要，不能承载完整工具 stdout/stderr；日志和 artifact 保留原始诊断。
- Run 状态：`completed_with_errors` 是至少一个可选检测阶段失败后的真实完成状态，不能为改善 UI 而改写。
- 发现的指定 Run 事实：`report.json` 记录 `190.10.10.201:22/tcp` 的 `nuclei` 检测为 `failed/command_failed`；其 artifact 显示仓库的 SSH 模板发生运行时错误并报 `no templates provided for scan`。这解释该状态；其余 nuclei 检查中有一个已完成。

## Dependency Map

三个子任务可并行规划；控制台子任务不得依赖模板修复才能验证摘要策略。状态处置子任务使用 `../lab` 的 OpenSSH fixture（容器 `172.22.0.2:22`、宿主 `127.0.0.1:10022`、`lab/lab`）并依赖可用 Docker daemon；不得对生产目标重跑。RBKD 基线模板只读对照，不因其本地工作树删除状态而被自动恢复或直接替换。

## Rollback

每个子任务独立回滚。不得将历史 Run 或 Release 标签移动来掩盖结果。
