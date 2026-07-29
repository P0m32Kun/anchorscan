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
- 保留成功发布的 `v2.0.2` Release、三平台归档和 checksums，不再重复触发或重建该版本。

## Acceptance Criteria

- [x] Linux、macOS、Windows smoke job 均能定位各自下载的 `anchorscan-<tag>-<os>-<arch>.tar.gz`。
- [x] Windows 的 `TestBuildVersionCanBeInjected` 能执行带 `.exe` 后缀的 injected 与 dev 临时二进制。
- [x] 本地相对路径失败反馈循环在改用绝对路径后通过。
- [x] 相关 package/release 检查通过，且独立代码审查无未解决的阻断项。
- [x] PR #3 合并后，`v2.0.2` release run `30383917510` 成功，并发布三平台归档与 `checksums.txt`。

## Out of Scope

- 修改打包格式或归档内容。
- 增减 release 构建平台。
- 修复与本次 smoke 日志无关的 lab、扫描器或应用功能。
- 删除、强制移动或重建远端 `v2.0.2` Tag。

## Completion Reconciliation

- 修复由 PR #3 合并为 `051e65e`，对应 `v2.0.2` release run `30383917510` 成功。
- GitHub Release `v2.0.2` 已于 2026-07-29 发布 Linux、macOS、Windows 归档及 checksums。
- 初始计划中的 `v2.0.3` 补丁标签未采用；本记录按实际已发布状态收敛，不再触发额外发布。
