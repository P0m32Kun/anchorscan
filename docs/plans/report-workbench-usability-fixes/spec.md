# 报告与验证工作台可用性修复批次

**状态：in-progress（2026-07-23 第二轮实测回归修正中）**

本计划处理用户已确认的 10 项报告/工作台可见性问题，按用户批准的修改方案落地。各问题的根因与方案见会话记录；本文件只固化范围、验收标准与测试接缝。

## 范围

1. **封面标题**：移除"报告标题"输入；封面固定为 `{被测单位}安全渗透测试分析报告`。
2. **3.1 会话标签**：删除"接入记录"；`接入点`→`测试设备接入点`；`测试机 IP`→`测试设备 IP`。HTML 报告与 DOCX 模板同步。
3. **证据上传**：粘贴/拖拽提到 dialog 层（兼容 macOS Cmd+V），去掉强制描述弹窗；正向新建验证也能直接上传。
4. **漏洞自动纳入报告**：`confirmed`/`not_observed` 验证默认纳入报告，移除"纳入报告"手选。
5. **工具运行不计入扫描任务**：扫描任务列表只含 `kind='scan'`；首页计数同步；工具页跳转不自动绑项目。
6. **工具终端符号**：持久化时处理 `\r` 覆盖语义并保留 ANSI；前端安全渲染颜色，终端字体补等宽覆盖。
7. **"扫描任务"导航**：项目详情运行表区可被锚点定位。
8. **去重"正式报告"入口**：删除顶部"正式报告"tab，保留"预览 HTML 报告"+"导出 DOCX 报告"。
9. **待负向验证按服务指纹聚合**：负向候选按 `service(+product)` 分组为一卡片；每指纹可传任意数量截图；复用命令生成产出 nmap NSE / nuclei `-tags` 建议命令。
10. **检测执行事实默认折叠**：扫描报告（console 与导出模板）将该区块包进 `<details>`，默认收起。

## 非范围

- 不改变扫描流水线、指纹/漏洞解析规则、Run Lease、Artifact 落盘等既有契约。
- 不新增前端运行时或构建步骤；继续用 `html/template` + 原生 JS + 单 `style.css`（见 ADR 0005、`modernize-web-console` technical-design）。
- 除第二轮实测补充明确修正检测覆盖判定外，不改变 Project Report 的其他聚合规则；issue 9 的服务指纹分组仍只在视图层完成。
- `internal/tools/httpx*.go` 等 Windows 兼容性修复属于他处，本计划不触碰。

## 验收标准

- 新建/编辑项目表单无"报告标题"字段；封面标题为 `{被测单位}安全渗透测试分析报告`，项目详情不再单独展示"报告标题"卡片。
- 项目报告 3.1 会话行不含"接入记录"，标签为"测试设备接入点""测试设备 IP"；HTML 与 DOCX 一致；`scan_project.html` 表单标签同步。
- 在验证弹窗内任意位置 Ctrl+V / Cmd+V 或拖拽均可上传截图；上传不再弹描述框；新建正向验证即可上传。
- `confirmed` 与 `not_observed` 验证创建后默认进报告；验证弹窗无"纳入报告"复选框；`buildProjectDeliverable` 不再因 `Included` 开关漏纳这两种 outcome。
- 首页"历史任务"、`/runs`、项目"扫描历史"均不含 `kind='tool'` 运行；工具页从工作台跳转时不默认绑定项目。
- 工具终端框不显示 ANSI 控制码方框，保留常见 SGR 颜色语义；不因 `\r` 叠盖导致缺字，等宽字体覆盖制表符。
- 项目详情"扫描任务"tab 可定位到运行表区；顶部不再有"正式报告"tab。
- 待负向验证按服务指纹分组展示；每指纹卡片可上传多张截图并给出 nmap/nuclei 建议命令；提交产出 `not_observed` 验证。
- 扫描报告"检测执行事实"默认折叠，展开后内容不回归。

## 测试接缝（TDD 只在这些 seam）

- **Go 单元**
  - `reportTitle()` 回退：`project_report.go`（issue 1）。
  - `ListScanRuns` / `ListProjectScanRuns` 的 `kind='scan'` 过滤：`store/runs.go`（issue 5）。
  - `normalizeToolOutput(s)`：从 `tool_run.go` 抽出的纯函数，保留 ANSI + 处理 `\r`（issue 6）。
  - 验证创建/更新默认 `Included`：`web/verifications.go`；`buildProjectDeliverable` 对 confirmed/not_observed 直接纳入：`web/project_report.go`（issue 4）。
  - 负向候选按指纹分组：`report` 项目报告模型 + 视图（issue 9）。
- **Python 断言**
  - `tools/docx-render/check_structure.py` 增/改对 3.1 标签与封面的结构断言（issue 1、2）。
- **JS 单元**
  - `internal/web/static/app.test.mjs` 增加 `imagesFromClipboardData(items)` 纯函数与粘贴/拖拽归一化测试（issue 3）。
- **非 TDD（模板文本/结构）**：issue 2 的 HTML/DOCX 标签、issue 7、8、10 的 `<details>` 与锚点属结构改动，以渲染快照/视觉检查验收，不写单测。

## 技术说明

- **DOCX 3.1 标签**：按 ADR 0005，模板是受版本管理的人工产物。改 `tools/docx-render/prototype.py:250-252` 的三行后重新生成 `templates/project-report.docx`，或直接编辑其 `word/document.xml`；同步 `check_structure.py` 断言。
- **issue 1 DB 列**：已决定保留 `report_title` 列，仅停止采集/展示，依赖 `reportTitle()` 回退。无迁移。
- **issue 9 命令生成**：复用 `report.BuildCandidateCommands` 与知识库（`internal/knowledgebase`、`config/service-tags.yaml`、`config/nse.yaml`）按指纹产出命令；不新建并行命令体系。

## 2026-07-23 实测回归补充

本节覆盖现场验证推翻的旧假设，优先级高于上文中与之冲突的描述：

- `rpcbind` 必须命中 RPC NSE 规则；`rdpscan` 仍只用于 RDP，rpcbind 上的 `no_matching_rule` 属预期。
- 负向候选键为 `zone + service + product`，不得包含 IP 或端口；同服务的不同 IP/端口必须进入一个验证项。界面不再使用单选 radio，每个聚合组直接进入自己的验证弹窗。
- 负向候选没有漏洞知识库条目，不能调用只接受正向漏洞候选的 `report.BuildCandidateCommands`；命令继续直接以 `config/nse.yaml` 和 `config/service-tags.yaml` 为唯一规则源，按组覆盖全部唯一 IP、端口和 URL。此条替代上文 issue 9 的命令生成说明。
- 负向验证弹窗必须显式展示可点击、可拖放、可粘贴且支持多张图片的��图区。
- 工具输出区占工具页完整下半区；工具运行持久化后可从专属历史列表和当前结果链接查看。ANSI 处理以第二轮回归补充为准。
- 验证工作台不得残留“正式报告”tab。
- DOCX 测试范围中的多个目标按行输出；`confirmed`/`not_observed` 验证的漏洞信息与证据截图必须通过端到端导出回归。

对应执行 ticket：`tickets/09-field-regression-closure.md`。

## 2026-07-23 第二轮实测回归补充

- DOCX 测试范围列表统一相对标签缩进 4 个字符；修改建议的每一条都使用相同段落缩进，不依赖仅影响首行的空格。
- 单工具实时输出保留 ANSI 颜色语义，由前端安全渲染常见 SGR 颜色和换行；不得把控制码作为可见方框，也不得丢掉颜色信息。
- `skipped/no_matching_rule` 只表示该单个检测引擎没有适用于当前服务的规则，不能提升为服务整体“不适用”。无非 info 漏洞结果时，只要至少一个漏洞检测引擎 `completed`，且其余引擎均为 `completed` 或 `skipped/no_matching_rule`，该服务就进入待负向验证；所有引擎均未匹配规则时不得形成负向证明。`failed`、`canceled`、`interrupted` 等异常终态仍进入“检查未完成”。
- Samba 指纹必须能从默认 NSE/tag 规则生成 Nmap 与 nuclei 建议命令。
- 负向验证保存并刷新后继续显示“待负向验证”队列。
- 负向证明标题统一为“{service/product}相关漏洞不存在证明，端口（{ports}）”，前端和 DOCX 使用同一文案。

对应执行 ticket：`tickets/10-second-field-regression.md`。
