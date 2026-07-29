# 明确 Web Console 版本显示契约

## Goal

确认并锁定 Web Console 显示值与实际二进制构建输入的一对一关系。

## Requirements

- 未注入版本的开发构建显示 `dev`。
- `make build/package/release-check VERSION=vX.Y.Z` 的产物显示去掉前缀后的 `X.Y.Z`。
- 不运行时查询 Git 分支或远端最新 Tag。

## Acceptance Criteria

- [ ] CLI 与 Web 对开发构建显示 `dev`，并有最低充分自动化验证。
- [ ] Release/package 的现有 linker 注入与 Web smoke 继续验证正式显示版本。
- [ ] 若当前实现已满足契约，记录验证证据，不作无意义代码修改。

## Out of Scope

- 为本地开发构建自动伪造 Tag 或分支版本。
