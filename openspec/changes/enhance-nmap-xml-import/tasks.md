## 1. Parser and Model

- [x] 1.1 补充解析测试，覆盖 TCP/UDP 同端口服务、CPE 值、port script、host script、postscript 输出、空输入和非 `nmaprun` XML。
- [x] 1.2 扩展 Nmap XML 解析器，保留 protocol、服务字段、CPE 值、script 输出和 script scope，且不引入 Python viewer。
- [x] 1.3 更新 fingerprint/report 数据模型，使 `port/protocol` 身份能在 JSON 和 HTML 报告输出中保留下来。

## 2. Persistence

- [x] 2.1 为导入 run 所需的 protocol/CPE/script-scope 数据添加向后兼容的 SQLite migration。
- [x] 2.2 更新 store 的保存/读取方法及测试，确保同一端口上的导入 TCP 和 UDP 服务保持独立。
- [x] 2.3 将导入的 NSE script 输出持久化为可报告的 findings 或 script 衍生输出，并避免把 host/global script 归到错误端口。

## 3. Import Command

- [x] 3.1 为 `anchorscan import-nmap` 增加 CLI 参数解析、帮助文本，以及 `--xml`、`--db`、可选 `--run-id`、`--project`、`--json`、`--html` 的校验。
- [x] 3.2 实现导入编排：创建完成态 scan run，以事务方式导入解析数据，并按需写出报告。
- [x] 3.3 确保非法 XML、空文件和非 Nmap XML 都会以清晰错误失败，且不会留下部分 run 数据。

## 4. Verification and Docs

- [x] 4.1 增加 CLI/import 集成测试，使用包含 `53/tcp`、`53/udp`、CPE 和 NSE scripts 的最小 Nmap XML fixture。
- [x] 4.2 更新 README 或命令帮助文档，说明导入流程和明确的非目标。
- [x] 4.3 运行 `go test ./...`，并用该 fixture 实跑一次 `anchorscan import-nmap` 示例命令。
