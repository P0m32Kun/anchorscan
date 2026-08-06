# 批准书：Ticket 04 实施批准

本文件与 `docs/ticket-04-brief.md` 一起阅读，任务书内容不变。

**批准此计划。** 你 Phase 1 的输出（命令出口清单、统一 gate 设计、一次性 challenge token、五档门禁 UI、测试矩阵、Playwright 方案）全部认可，进入第二阶段直接实施，不再中途停下确认。

确认你计划中的几个关键设计决策：
1. 服务端一次性 challenge token（消费后失效、重启失效、防重放）作为确认/acknowledgement 机制——认可。
2. `/tools/{tool}?raw_args=...` 手工构造不再获得 catalog 命令预填，改由确认后的 token 链路预填；普通手工工具页用户自行填写参数的能力保留——认可。
3. 不新增前端框架/依赖，沿用现有 Vue/dialog 样式——必须遵守。
4. Playwright 若运行失败，诚实记录原因并以 httptest 覆盖，禁止伪造截图/trace。

验收命令以你列的为准：

```bash
go test ./internal/...
go build ./...
go vet ./...
npm run typecheck:web
npm run test:web
make pr-check   # 尽量执行；环境阻塞与代码失败需在报告中区分
```

授权范围：你为本任务创建/修改的所有文件均视为任务范围内，不再逐项确认。禁令不变：禁止 git commit/push；不引入账号/审批流/长期授权/自动扫描；不改 knowledgebase loader 与 report 命令构造层既有行为；纯视觉项在报告里列"未实测"。
