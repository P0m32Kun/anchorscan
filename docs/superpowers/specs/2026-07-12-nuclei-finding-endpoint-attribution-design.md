# Nuclei Finding 端点归属修复设计（2026-07-12）

## 1. 背景与根因

`run-20260712-145702.423172000` 的 HTML 报告把 `redis-default-logins` 与 `tomcat-default-login` 同时展示在 `172.22.0.1:8080` 下，但 Redis 原始证据明确给出 `host=172.22.0.1`、`port=6379` 和 `matched-at=172.22.0.1:6379`。

问题由两个缺陷共同造成：

1. `config/service-tags.yaml` 的 Tomcat 规则包含宽泛的 `default-login` 标签，导致 Tomcat 扫描轮次也会执行 Redis 等其他产品的默认凭据模板。
2. `internal/app/scan.go` 把每条 Nuclei 结果的 `IP`、`Port` 和 `Protocol` 强制设置为当前指纹 `fp` 的端点，而没有使用 Nuclei 返回的真实端点。错误数据写入数据库后，`report.Build` 和 HTML 模板只是按 `IP + Port + Protocol` 正常分组并渲染。

因此，修复必须同时收窄模板选择并纠正 finding 的端点归属；不得在报告模板中补偿错误数据。

## 2. 目标

- Tomcat 规则不再携带宽泛的 `default-login` 标签。
- Nuclei finding 优先使用结构化结果中的真实 IP 和端口。
- 结构化端点缺失时保留当前回退行为，避免丢失已有结果。
- 完整扫描和单工具 Nuclei 执行使用同一套归属逻辑。
- 修正指定历史 run 中已经存错的 Redis finding，并重新导出 HTML/JSON。
- 增加能够稳定复现“当前指纹为 8080、结果命中 6379”的回归测试。

## 3. 非目标

- 不修改 `internal/report/html.go` 的 HTML 模板。
- 不修改 `internal/report/model.go` 的分组规则。
- 不引入永久数据库迁移框架或通用数据清洗子系统。
- 不依靠任意证据文本中的数字猜测端口。
- 不调整 Nmap、NSE 或手工复核 finding 的归属方式。

## 4. 方案

### 4.1 收窄 Web 产品标签

从 Tomcat 规则的 `nuclei_tags` 中移除 `default-login`，保留 `tomcat` 和 `apache-tomcat`。

同时检查所有 `target: "url"` 的产品规则。产品规则如果已经包含明确的产品标签，则移除宽泛的 `default-login`；HTTP 通用规则也不得通过 `default-login` 扩展到非 HTTP 产品。`target: "hostport"` 的非 Web 规则暂不调整，避免扩大本次修复的扫描覆盖变化。

此项减少跨产品模板执行，但不作为数据正确性的唯一保障。即使未来 Nuclei 因其他标签返回跨端口结果，端点归属逻辑仍必须正确。

### 4.2 保留 Nuclei 结构化端点

扩展 `internal/tools.NucleiFinding`，保存 Nuclei JSONL 中与归属有关的结构化字段：

- `host`
- `ip`
- `port`
- `url`
- `matched-at`（已有）

`port` 在 Nuclei JSON 中按字符串解析，再使用 `strconv.Atoi` 做严格十进制转换。空值、非数字、零或超出 `1..65535` 的值视为不可用，不导致整份 JSONL 解析失败。

真实 IP 的选择顺序为：

1. 非空 `ip`
2. 可解析为主机名/IP 的 `host`
3. 可解析的 `matched-at`
4. 当前指纹 `fp.IP`

真实端口的选择顺序为：

1. 有效结构化 `port`
2. `host`、`url` 或 `matched-at` 中的显式端口
3. URL scheme 的标准端口（仅 `http=80`、`https=443`）
4. 当前指纹 `fp.Port`

解析 URL、IPv4、IPv6 和 `host:port` 使用 Go 标准库 `net`、`net/netip`、`net/url`；不新增依赖。

### 4.3 统一 finding 转换

在 `internal/app` 中保留一个共享的 Nuclei 结果转换函数，由以下两个入口共同调用：

- 完整扫描：`internal/app/scan.go`
- 单工具执行：`internal/app/tool_run.go`

转换函数接收 Nuclei 结果与当前指纹作为回退值，生成 `report.Finding`。`Target` 继续保留 `matched-at`，证据格式保持不变；`IP` 和 `Port` 改为解析后的真实端点。

`Protocol` 优先使用同一扫描中与真实 `IP + Port` 匹配的服务指纹协议；找不到匹配时回退当前指纹协议。单工具执行没有其他指纹时直接使用当前指纹协议。

端点解析失败不是扫描错误：记录仍保存到当前指纹，保持向后兼容。只有 JSONL 本身不是合法 JSON 时才沿用当前错误返回行为。

### 4.4 指定历史 run 的一次性纠正

历史纠正不进入产品运行时代码。实施时先定位该 run 所在 SQLite 数据库并创建文件级备份，然后在事务中执行精确更新，匹配条件至少包括：

- `run_id = 'run-20260712-145702.423172000'`
- `source = 'nuclei'`
- `finding_id = 'redis-default-logins'`
- `target = '172.22.0.1:6379'`
- 当前错误端口为 `8080`

更新内容为 `ip=172.22.0.1`、`port=6379`、`protocol=tcp`。更新前后分别查询并保存结果；受影响行数必须恰好为 1，否则回滚并停止。

数据纠正后复用现有报告导出流程重新生成该 run 的 JSON 和 HTML。若原数据库不可用，只能基于现有 HTML 生成纠正版副本，不声称数据库已修复。

## 5. 数据流

```text
Nuclei JSONL
  -> ParseNucleiJSONL 保留结构化 host/ip/port/url/matched-at
  -> 共享转换函数解析真实端点
  -> 失败时回退当前 fingerprint
  -> SaveFinding 写入正确 IP/Port/Protocol
  -> report.Build 按 IP+Port+Protocol 分组
  -> JSON/HTML 正常导出
```

## 6. 文件范围

预计修改：

- `config/service-tags.yaml`：移除 Web 产品规则中的宽泛 `default-login`。
- `internal/tools/nuclei.go`：解析并保留结构化端点字段。
- `internal/app/scan.go`：使用共享转换函数保存完整扫描结果。
- `internal/app/tool_run.go`：复用相同转换逻辑。
- `internal/tools/nuclei_test.go`：验证结构化字段解析和非法端口回退输入。
- `internal/app/scan_test.go`：验证 8080 扫描轮次返回 6379 结果时的最终归属。
- `internal/app/tool_run_test.go`：验证单工具入口使用结果真实端点。
- `internal/config/config_test.go`：验证 URL 规则不再包含宽泛 `default-login`。

不新增生产文件和第三方依赖。若现有测试结构允许，优先把共享转换函数留在 `scan.go`，避免为单个小函数拆分新文件。

## 7. 测试与验收

### 7.1 自动测试

1. Nuclei JSONL 包含 `ip=172.22.0.1`、`port=6379`、`matched-at=172.22.0.1:6379` 时，解析结果保留这些字段。
2. 当前指纹为 `172.22.0.1:8080/tcp`，Nuclei 返回 Redis `172.22.0.1:6379` 时，保存的 finding 必须为 `172.22.0.1:6379/tcp`。
3. Nuclei 未提供可用端点字段时，finding 仍归属当前指纹。
4. 单工具 Nuclei 入口同样使用结果真实端口。
5. 配置测试断言所有 `target: "url"` 规则均不包含 `default-login`。
6. 现有报告模型测试继续通过，证明无需修改分组和模板。

### 7.2 历史 run 验收

- 数据库中 `redis-default-logins` 的端口为 6379，目标仍为 `172.22.0.1:6379`。
- 重新导出的报告中，6379 行包含 Redis finding。
- 8080 行只保留 Tomcat finding，不再包含 Redis 证据。
- JSON 与 HTML 对同一 finding 的 IP、端口和目标一致。
- 数据库更新恰好影响一行，并保留可恢复备份。

### 7.3 验证命令

实施阶段至少运行：

```bash
go test ./internal/tools ./internal/app ./internal/config ./internal/report
go test ./...
```

## 8. 风险与控制

- **扫描覆盖减少**：移除 URL 规则的 `default-login` 可能减少没有产品标签的模板执行。控制方式是仅调整 `target: "url"` 规则，保留明确产品标签，并用现有规则测试确认配置内容。
- **IPv6/URL 解析错误**：只使用标准库解析，并用表驱动测试覆盖显式端口、标准 scheme 和 IPv6。
- **错误历史更新**：使用完整条件、事务、备份和“恰好一行”断言，禁止按模板 ID 批量更新。
- **旧结果字段缺失**：解析失败时回退当前指纹，不丢弃 finding。

## 9. 完成定义

代码、配置和回归测试全部通过；指定历史 run 的数据库记录与重新导出的 HTML/JSON 一致；报告模板和报告分组代码无修改；不存在新增依赖或永久迁移机制。
