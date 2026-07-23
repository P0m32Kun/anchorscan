# 报告与验证工作台可用性修复批次

**状态：completed**

本计划处理用户已确认的 10 项报告/工作台可见性问题，按用户批准的修改方案落地。各问题的根因与方案见会话记录；本文件只固化范围、验收标准与测试接缝。

## 范围

1. **封面标题**：移除"报告标题"输入；封面固定为 `{被测单位}安全渗透测试分析报告`。
2. **3.1 会话标签**：删除"接入记录"；`接入点`→`测试设备接入点`；`测试机 IP`→`测试设备 IP`。HTML 报告与 DOCX 模板同步。
3. **证据上传**：粘贴/拖拽提到 dialog 层（兼容 macOS Cmd+V），去掉强制描述弹窗；正向新建验证也能直接上传。
4. **漏洞自动纳入报告**：`confirmed`/`not_observed` 验证默认纳入报告，移除"纳入报告"手选。
5. **工具运行不计入扫描任务**：扫描任务列表只含 `kind='scan'`；首页计数同步；工具页跳转不自动绑项目。
6. **工具终端符号**：渲染前归一化工具 stdout（剥 ANSI、处理 `\r`），终端字体补等宽覆盖。
7. **"扫描任务"导航**：项目详情运行表区可被锚点定位。
8. **去重"正式报告"入口**：删除顶部"正式报告"tab，保留"预览 HTML 报告"+"导出 DOCX 报告"。
9. **待负向验证按服务指纹聚合**：负向候选按 `service(+product)` 分组为一卡片；每指纹可传任意数量截图；复用命令生成产出 nmap NSE / nuclei `-tags` 建议命令。
10. **检测执行事实默认折叠**：扫描报告（console 与导出模板）将该区块包进 `<details>`，默认收起。

## 非范围

- 不改变扫描流水线、指纹/漏洞解析规则、Run Lease、Artifact 落盘等既有契约。
- 不新增前端运行时或构建步骤；继续用 `html/template` + 原生 JS + 单 `style.css`（见 ADR 0005、`modernize-web-console` technical-design）。
- 不改变 Project Report 的聚合模型（`BuildProjectReport`）；issue 9 仅在视图层按指纹分组负向候选。
- `internal/tools/httpx*.go` 等 Windows 兼容性修复属于他处，本计划不触碰。

## 验收标准

- 新建/编辑项目表单无"报告标题"字段；封面标题为 `{被测单位}安全渗透测试分析报告`，项目详情不再单独展示"报告标题"卡片。
- 项目报告 3.1 会话行不含"接入记录"，标签为"测试设备接入点""测试设备 IP"；HTML 与 DOCX 一致；`scan_project.html` 表单标签同步。
- 在验证弹窗内任意位置 Ctrl+V / Cmd+V 或拖拽均可上传截图；上传不再弹描述框；新建正向验证即可上传。
- `confirmed` 与 `not_observed` 验证创建后默认进报告；验证弹窗无"纳入报告"复选框；`buildProjectDeliverable` 不再因 `Included` 开关漏纳这两种 outcome。
- 首页"历史任务"、`/runs`、项目"扫描历史"均不含 `kind='tool'` 运行；工具页从工作台跳转时不默认绑定项目。
- 工具终端框不再出现 ANSI 转义残片或因 `\r` 叠盖导致的缺字；等宽字体覆盖制表符。
- 项目详情"扫描任务"tab 可定位到运行表区；顶部不再有"正式报告"tab。
- 待负向验证按服务指纹分组展示；每指纹卡片可上传多张截图并给出 nmap/nuclei 建议命令；提交产出 `not_observed` 验证。
- 扫描报告"检测执行事实"默认折叠，展开后内容不回归。

## 测试接缝（TDD 只在这些 seam）

- **Go 单元**
  - `reportTitle()` 回退：`project_report.go`（issue 1）。
  - `ListScanRuns` / `ListProjectScanRuns` 的 `kind='scan'` 过滤：`store/runs.go`（issue 5）。
  - `normalizeToolOutput(s)`：从 `tool_run.go` 抽出的纯函数，剥 ANSI + 处理 `\r`（issue 6）。
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
