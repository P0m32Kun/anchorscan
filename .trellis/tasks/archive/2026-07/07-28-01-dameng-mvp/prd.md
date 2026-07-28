# PRD — 达梦数据库默认口令检测 MVP

## 1. 背景

AnchorScan 当前双引擎（nuclei + NSE）无法识别达梦数据库。nmap 对达梦默认端口 5236 通常返回 `padl2sim` 或无意义服务名，且达梦端口可配置，不能依赖固定端口触发 POC。

## 2. 目标

实现一个**达梦数据库默认口令检测 MVP**：
- 不固定端口，通过主动协议指纹识别发现达梦服务。
- 识别到达梦后，再调用内置 Go 检测器尝试默认口令 `SYSDBA/SYSDBA`。
- 默认口令存在则报告漏洞；口令已改或非达梦服务则不误报。

## 3. 范围

### In Scope
- 新增 `internal/fingerprint/probes/dameng.go`：使用 nuclei `dameng-detect.yaml` 的探测包做主动协议识别。
- 新增 `internal/tools/dameng.go`：基于达梦 Go 驱动实现默认口令检测器。
- 修改 `internal/app/scan_target.go`：在 nmap 指纹阶段后增加达梦主动识别，命中后更新 fingerprint；再按 `fp.Normalized == "dameng"` 触发 POC。
- 修改 `internal/fingerprint/normalize.go`：增加达梦归一化别名。
- 修改 `config/ports-highrisk.txt`：加入 `5236`，移除错误端口 `12345`（如有）。
- 修改 `config/default.yaml` 与 `.example`：增加 `tools.dameng` 开关（默认启用）。
- 修复 `docs/research/vulnerability-coverage-official-sources.md` 第 48 项端口错误。

### Out of Scope（留给后续通用指纹增强任务）
- 通用 nuclei detection 指纹增强引擎。
- 其他国产数据库或私有协议的主动识别。
- 达梦协议版本提取、多账号字典爆破。

## 4. 验收标准

- [ ] 对运行默认口令 `SYSDBA/SYSDBA` 的达梦实例，报告「达梦数据库默认口令」漏洞（severity high/critical）。
- [ ] 对修改过口令的达梦实例，不报告漏洞，但 `detection_checks` 记录检测已运行。
- [ ] 对非达梦服务，即使开放 5236 端口，也不误报、不崩溃。
- [ ] 不依赖固定端口：达梦跑在非 5236 端口时，只要主动指纹识别命中即可触发 POC。
- [ ] `make test` 与 `make pr-check` 通过。
- [ ] 新增代码有单元测试覆盖检测器 verdict 映射和触发条件。

## 5. 关键约束

- 优先使用 Nuclei 官方 `javascript/detection/dameng-detect.yaml` 的 hex 探测包，避免从零手写协议。
- 使用达梦 Go 驱动完成认证检测；`Ping()` 成功即默认口令存在。
- 驱动选型：`gitee.com/chunanyong/dm`（跨平台，go mod 友好）。
- 所有改动必须兼容现有扫描流程，失败时记录 `recordDetectionCheck` 并不阻断后续目标。
