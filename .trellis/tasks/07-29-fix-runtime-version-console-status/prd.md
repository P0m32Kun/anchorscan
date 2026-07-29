# 修复运行版本、控制台输出与异常完成状态

## Goal

让交付给用户的 Web Console 显示可追溯的应用版本，扫描控制台只呈现对操作者有用的进度与可诊断错误，并让 `completed_with_errors` 能从持久化的失败事实解释和纠正。

## Confirmed Facts

- `internal/version.Version` 的源码开发回退值为 `dev`；`Makefile` 仅在 `make build/package/release-check VERSION=vX.Y.Z` 时用 linker `-X` 注入去掉 `v` 前缀后的版本。当前未注入版本的本地构建显示 `dev` 是既有行为。
- Web smoke 已验证发布构建应显示注入后的 `AnchorScan Console <version>`；本任务不能破坏该 release-version contract。
- supplied `data/projects/project-20260728-140618.086386000/runs/run-20260729-093454.039551000/report.json` 记录该 Run 唯一失败 DetectionCheck 为 `190.10.10.201:22/tcp` 的 `nuclei failed/command_failed`；同目录 artifact `nuclei-190.10.10.201-22-template.jsonl` 显示仓库 SSH 精确模板运行时错误并报 `Could not run nuclei: no templates provided for scan`。这解释 `completed_with_errors`；`skipped/no_matching_rule` 不是失败。
- 控制台当前会接收来自扫描流程的进度事件；用户提供的日志含 nuclei banner、ANSI 控制码、版本/模板信息和多条 `dameng-probe` 细节，妨碍观察关键扫描流程。

## Requirements

1. 为交付构建和本地开发构建定义并实现一致、可追溯的 Web Console 版本显示策略；不得伪造一个与实际构建无关的最新 Git tag。
2. 保留关键的扫描阶段、进度、发现和可操作错误，同时抑制或规范化工具 banner、ANSI 控制码、非行动性版本/模板信息和过于细粒度的探测噪声；不能以静默输出掩盖真正的失败原因。
3. 对 Run `run-20260729-093454.039551000` 的 `completed_with_errors` 给出基于持久化检测事实的根因结论；若根因属于产品缺陷，修复它并建立回归验证；若是环境或配置问题，记录可复现的运维修复步骤，不把运行状态改为错误的 `completed`。
4. 不改变 Release archive 内容、目标平台矩阵、发布权限或扫描授权边界。

## Deliverable Map

本任务是父任务，不直接实施产品代码。规划完成后拆分为独立、可验证的子任务：

- 版本显示策略与构建注入：`.trellis/tasks/07-29-fix-console-version-contract/`。
- 扫描控制台的进度/工具输出呈现：`.trellis/tasks/07-29-normalize-scan-console-output/`。
- 指定 Run 的 `completed_with_errors` 取证、根因处置及必要修复：`.trellis/tasks/07-29-diagnose-completed-with-errors-run/`。

子任务之间没有默认依赖；若调查证实控制台或状态问题共用同一接缝，将在子任务的 `prd.md` / `implement.md` 显式记录依赖。

## Acceptance Criteria

- [ ] 已明确 release 与本地开发构建各自显示的版本来源，并有自动化验证证明显示值可追溯至实际构建输入。
- [ ] 关键扫描阶段、进度与可行动错误仍可见；已列出的 banner/ANSI/冗余探测噪声不再干扰控制台，且相关自动化验证通过。
- [ ] 可从指定 Run 的持久化事实解释 `completed_with_errors`；处置与事实一致，并有测试或可复现运维证据。
- [ ] 每个子任务有独立的验收、验证和代码审查；父任务的交叉验收确认三项变更不会破坏 release-version 与扫描运行状态契约。

## Out of Scope

- 通过运行时查询远端 Git tag 来伪造版本。
- 修改 release 包格式、构建平台矩阵、发布权限或应用的扫描授权范围。
- 隐藏、降级或删除真实工具执行失败以获得 `completed` 状态。
- 将所有底层工具原始输出都作为 Web Console 的用户界面内容重新设计。

## Key Decisions

- 本地未注入版本的开发构建显示 `dev`；Release/package 构建仅显示由构建输入通过 linker 注入的正式版本。既不运行时查询远端 Tag，也不将分支名或最新 Tag 伪装为二进制版本。
- Web Console 默认只呈现结构化、简洁的阶段、进度和可行动错误摘要；不提供工具原始 stdout/stderr 的 UI 展开入口。原始工具输出保留在该 Run 的本地 artifact/日志中，供运维排查。
