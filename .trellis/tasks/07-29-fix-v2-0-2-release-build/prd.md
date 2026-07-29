# 修复 v2.0.2 Tag Release 构建失败

## Goal

修复 Tag Release 在原生包 smoke 阶段的跨平台失败，使构建产物经过 Linux、macOS 和 Windows 验证后能够进入 GitHub Release 发布阶段。

## Background

- `v2.0.2` 最新运行 `30374031929` 中，lab 与三个 build jobs 均成功，三个 smoke jobs 均失败，release job 因依赖失败而跳过。
- Linux/macOS/Windows 都从 `scripts/` 包测试工作目录解析相对路径 `dist/...tar.gz`，因此找不到实际下载到仓库工作区 `dist/` 下的归档。
- Windows 还会执行没有 `.exe` 后缀的临时二进制路径，导致 `exec` 报告 executable not found。
- 已确认下载的 Linux artifact 文件名正确，归档生成本身不是根因。

## Requirements

- Release smoke job 必须使用不依赖 Go 包测试工作目录的归档绝对路径。
- 包集成测试在 Windows 上构建并执行临时二进制时必须使用原生 `.exe` 文件名。
- 不改变归档内容、目标平台矩阵、发布权限或应用运行时行为。
- 修复提交后发布新的不可变补丁 Tag `v2.0.3`；保留既有 `v2.0.2` Tag，不重写、删除或移动它。

## Acceptance Criteria

- [ ] Linux、macOS、Windows smoke job 均能定位各自下载的 `anchorscan-<tag>-<os>-<arch>.tar.gz`。
- [ ] Windows 的 `TestBuildVersionCanBeInjected` 能执行带 `.exe` 后缀的 injected 与 dev 临时二进制。
- [ ] 本地相对路径失败反馈循环在改用绝对路径后通过。
- [ ] 相关 package/release 检查通过，且独立代码审查无未解决的阻断项。
- [ ] 修复提交后创建 `v2.0.3` Tag 触发新 workflow；成功标准是三个 smoke jobs 通过并创建 Release assets/checksums。

## Out of Scope

- 修改打包格式或归档内容。
- 增减 release 构建平台。
- 修复与本次 smoke 日志无关的 lab、扫描器或应用功能。
- 删除、强制移动或重建远端 `v2.0.2` Tag。

## Key Decision

- 发布新的不可变补丁 Tag `v2.0.3`。这使修复提交成为工作流输入，同时保留失败的 `v2.0.2` Tag 历史。
