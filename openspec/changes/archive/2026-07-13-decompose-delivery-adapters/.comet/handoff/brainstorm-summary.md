# Brainstorm Summary

- 用户于 2026-07-12 确认采用现有 package 内按真实职责机械拆分方案。
- `main.go` 只保留进程入口和根命令分派；CLI 命令按共同修改原因分组，不引入 Command 框架。
- `server.go` 保留服务器装配和单一路由表；Handler 按项目、扫描、工具、导入、运行、配置和报告职责移动。
- 报告筛选、分页、导出和视图组装按现有纯函数边界整理；公开契约和依赖均不改变。
- 本 change 必须在 `decompose-scan-runtime` 完成后实施。

