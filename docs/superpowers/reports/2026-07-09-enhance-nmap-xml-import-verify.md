# 验证报告：enhance-nmap-xml-import

- Change: enhance-nmap-xml-import
- Date: 2026-07-09
- Verify mode: full（任务数 12 > 阈值 3）
- Base ref: fa9b0590aadaca242a342db9912d6388cc37be38

## 1. 构建与测试

| 检查 | 命令 | 结果 |
|------|------|------|
| 编译 | `go build ./...` | exit 0 ✅ |
| 全量测试 | `go test ./...` | 14 个包全部 ok，0 失败 ✅ |
| 实跑导入 | `anchorscan import-nmap --xml fixture --db /tmp/import.sqlite --json /tmp/import.json --html /tmp/import.html` | exit 0，`run_id=import-20260709-202809` ✅ |

## 2. 改动规模

- 17 个改动文件 + 3 个新增文件（`internal/app/import.go`、`internal/app/import_test.go`、`internal/store/import.go`）
- 693 行新增，50 行删除
- 跨模块：fingerprint、store、report、web、app、cmd

## 3. tasks.md 完成度

12/12 任务全部勾选（`[x]`），对应 tasks.md 四节（Parser and Model / Persistence / Import Command / Verification and Docs）。

## 4. Spec 场景逐项核对

### Requirement: 导入已有 Nmap XML 为 AnchorScan run
- ✅ 成功导入：实跑生成完成态 run，DB `ListScanRuns` 返回 1 条 status=completed
- ✅ 拒绝空 XML：`TestImportNmapRejectsEmptyXML` + `TestImportNmapRejectsEmptyXMLWithoutRun`，DB 无新增 run
- ✅ 拒绝非 Nmap XML：`TestImportNmapRejectsNonNmaprun` + `TestImportNmapRejectsNonNmaprunWithoutRun`，DB 无新增 run

### Requirement: 保留端口协议身份
- ✅ TCP/UDP 同端口：实跑 JSON 显示 `tcp/53` 和 `udp/53` 为两条独立 PortReport；`TestSQLiteStoreKeepsTCPAndUDPSamePort` 验证落库后两条独立；`TestBuildKeepsTCPAndUDPSamePortSeparate` 验证报告层不合并

### Requirement: 保留服务增强字段
- ✅ CPE 值：实跑 JSON/HTML 含 `cpe:/a:isc:bind:9.18.0` 和 `cpe:/a:openbsd:openssh:9.0`；`TestParseNmapXMLParsesCPE` 验证多 CPE 合并

### Requirement: 保留 NSE script 输出及作用域
- ✅ port script：实跑 DB finding `nmap-import:port:dns-version` port=53 protocol=tcp
- ✅ host/global script：实跑 DB findings `nmap-import:hostscript:ssh-hostkey`(port=0)、`nmap-import:postscript:http-title`(port=0)、`nmap-import:prescript:whois`(port=0)
- ✅ 非端口级保留原始输出：scope 编码进 source，不伪造端口归属

### Requirement: 导入 run 可生成现有报告
- ✅ JSON 报告：实跑 `/tmp/import.json` 存在，含正确主机/端口/CPE
- ✅ HTML 报告：实跑 `/tmp/import.html` 存在，含 `tcp/53`、`udp/53`、CPE

## 5. Design 决策一致性

| Design 决策 | 实现 | 一致 |
|-------------|------|------|
| 在 Go 内吸收解析逻辑，不引入 Python | 全部 Go 实现，零 Python 依赖 | ✅ |
| 复用现有 run/store/report 模型 | import 复用 ScanRun、fingerprints、findings、report.Build | ✅ |
| `ip + port + protocol` 服务身份 | portKey 改为 `ip:port:protocol`，含 dedupeFindings | ✅ |
| 只持久化当前需要的字段 | migration v3 只加 protocol/cpe/extrainfo/tunnel/scope | ✅ |
| NSE script 落为 findings/output | scriptsToFindings 转换，scope 编码进 source | ✅ |

## 6. 代码现实校准（与 design 原稿差异）

design 原稿假设 protocol/CPE/ExtraInfo/Tunnel 都需新增。代码探索发现 protocol/ExtraInfo/Tunnel 在内存模型和解析器已存在，只是未落库/未进报告。实现按"打通已有字段"处理，CPE 为真正新增。此偏差已记录在实现 plan 的"代码现实校准"节，方向与 design 一致，无矛盾。

## 7. 安全检查

- 无硬编码密钥 ✅
- 无新增 unsafe 操作 ✅
- 导入采用事务，失败不留半截数据 ✅
- XML 解析有明确错误边界（空/非nmaprun/非法）✅

## 8. 失败项与偏差

无 CRITICAL 或 IMPORTANT 失败项。

已接受的偏差：
- Comet `buildPasses` 检查不识别 Go 项目（只认 npm/mvn/cargo），verify/build guard 用 `COMET_SKIP_BUILD=1` 跳过，构建已通过 `go build ./...` 手动验证。
- hostscript/prescript/postscript finding 的 scope 编码进 `source` 字段（如 `nmap-import:hostscript:ssh-hostkey`），`findings.scope` 列预留为空，符合 brainstorm-summary 确认的 Source 编码方案。

## 结论

✅ 全部 5 条 Requirement、10 个 Scenario 通过验证。实现符合 design 决策，tasks 全部完成，构建与测试通过。可推进到 archive 阶段。
