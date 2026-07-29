# 执行计划

本任务为契约澄清与证据记录，当前实现已满足契约，不修改产品代码。

1. 运行现有 focused 验证：
   - `go test ./cmd/anchorscan -run TestVersionCommandPrintsVersion`
   - `go test -tags packageintegration ./scripts -run TestBuildVersionCanBeInjected`
   - `go test ./internal/web -run 'TestHomePageRenders|TestReportPageRendersCurrentVersion'`
   - `make release-check`
   - `make web-smoke`
2. 若验证通过，更新 PRD 验收条件为已完成。
3. 创建 `quality-evidence.json` 记录批准、验证结果与契约文档。
4. 提交契约文档与证据，发起 metadata-only PR。
