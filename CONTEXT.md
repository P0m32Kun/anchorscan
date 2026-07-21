# CONTEXT — anchorscan 领域词汇表

> 项目领域语言（ubiquitous language）。新增/重命名领域概念时同步更新本表。
> 架构深化讨论以本表为准（见 `docs/plans/`）；重要且难以逆转的决策记录在 `docs/adr/`。

## 项目一句话

**anchorscan** —— 对一组网络目标（主机/IP）编排安全工具（rustscan / nmap / httpx / nuclei 等），产出服务指纹、漏洞发现与可读报告的扫描器。

## 核心名词

### Scan Run（扫描运行，简称 Run）
一次扫描执行，由 `RunID` 唯一标识。生命周期状态：`running` / `completed` / `completed_with_errors` / `failed` / `canceled` / `interrupted`。`completed_with_errors` 表示执行已经结束并产生可用报告，但至少一个 Target 或 DetectionCheck 失败；`failed` 表示无法建立或保存有效运行结果；`interrupted` 表示进程终止导致执行失去连续控制，它保留已经产生的结果，但不是可断点恢复的运行。可归属于一个 Project，也可独立。CLI 与 web 共用同一 Run 概念。

### Project
一次有明确交付目标的渗透测试任务，例如“甘肃电力内网安全检查任务”。Project 按 Network Zone 组织多个 Scan Runs，并最终产出一份 Project Report；它不代表一个固定目标范围或一套扫描参数。

### Network Zone（网络分区，简称 Zone）
Project 内用于组织测试范围和报告内容的业务分区。创建 Project Scan Run 时必须且只能选择一个 Zone；标准选项为 I区、II区、III区，也允许使用甲方定义的自定义分区。Project Report 只生成实际有纳入内容的 Zone，不补空分区。

### Target（目标）
被扫描的**单个**主机/IP。一次 Run 扫描一组 Targets（`ScanOptions.Targets`）。

### TargetScan（单目标扫描结果）
对**一个** Target 执行流水线（rustscan→nmap→httpx→NSE/nuclei）产出的结果束：`Fingerprints` + `Findings` + `OpenPorts`（见 `internal/app.TargetScan`）。
> 由候选 #1 深化引入：原为位置式 4 元组穿过接缝，现已正名为具名类型。`scanTarget` 是产出它的纯流水线，持久化由 fan-out 承担。

### Fingerprint（服务指纹，`fingerprint.ServiceFingerprint`）
在一个 Target 的某端口上发现的服务：IP/端口/协议、service、product、version、是否 web、URL 等。由 nmap 服务识别产出，httpx 可对其进行 web 增强。

### Finding（漏洞发现，`report.Finding`）
绑定到某指纹（IP:Port）的一条漏洞或值得关注项。来源（`Source`）：`manual-review`（人工复核建议）、`nse`（nmap NSE 脚本）、`nuclei`（nuclei 模板）、`rdpscan`（rdpscan BlueKeep 检测）。带 severity。

### Verification（验证记录）
对一个漏洞或验证项在一组资产上的人工确认结论，状态为 `confirmed`（已确认存在）、`not_observed`（本次验证未发现）或 `inconclusive`（无法判定）。Finding 可以发起 Verification，但没有 Finding 的负向验证也可以独立建立 Verification。对外报告中的通用漏洞说明（description）、本次验证过程/结果（detail）和内部备注是三个不同概念，不得相互代用。

### Evidence（证据）
支撑某条 Verification 结论的人工保存材料，当前只包含按顺序排列的截图及说明；一张截图覆盖多个资产时，说明须明确本次验证的覆盖范围。Evidence 不等同于工具原始 Artifact，也不改变 Finding 或 DetectionCheck 的历史事实。

### DetectionCheck（检测检查）
一个检测引擎针对某个 Fingerprint 的实际执行记录。它回答“当时是否执行以及结果如何”，不表示漏洞覆盖率或目标安全程度。状态包括：`running`（已经开始且仍在执行）、`completed`（执行完成，Finding 可为零）、`skipped`（按规则未执行并说明原因）、`failed`（已尝试但失败）、`canceled`（操作者取消 Run 时仍未完成）、`interrupted`（租约过期或进程终止时仍未完成）。每次 Run 持久化自己的 DetectionChecks，后续规则变化不得改写历史执行事实。

### Detection Coverage（检测执行覆盖）
报告对每个指纹汇总 NSE、nuclei 与 rdpscan 的实际完成情况，显示各引擎、未覆盖及失败/跳过/取消/中断数量。它是本次执行记录的可见汇总，不是漏洞覆盖率或安全保证。

### Progress（进度事件流）
扫描进行中按 level/stage/message 报告的实时事件流，驱动 web 的 `/runs/:id/status` 与 `/runs/:id/events`。持久化为 `store.ScanEvent`。
> 由候选 #1 深化引入：`internal/app.Progress` 接口（单方法 `Emit`）是 scanTarget 报告进度的窄接缝，store 适配器 `storeProgress` 负责落 `ScanEvent` + 调日志。

### ScanEvent（`store.ScanEvent`）
Progress 的持久化形态：`{RunID, Time, Level, Stage, Message}`。web 进度轮询读取它。

### Run Lease（运行租约）
一个正在执行的 Run 对其执行所有权和存活状态的持久化声明。CLI、Web 和单工具运行共用 Run Lease，以避免同一数据库上出现多个活动任务；执行进程定期续租。只有租约过期的 `running` Run 才能被判定为 `interrupted`。Run Lease 不提供任务队列、抢占或断点恢复。

### Artifact（制品）
工具原始输出（rustscan ports、nmap-service XML、httpx JSONL、NSE XML、nuclei JSONL）落盘到本次 Run 的 `artifactDir`。命名由 `safeArtifactName` 规范化。报告与取证用。

### Run Report（扫描运行报告）
对一次 Run 的 Fingerprints + Findings + ScanData（活跃 IP、开放端口）的技术汇总产物：
- `report.json`（`report.BuildWithScanData` → `report.WriteJSON`）；
- HTML 视图（web 层 `report_handler`，按 ip/port 排序展示）。

### Project Report（项目交付报告）
一个 Project 的正式交付报告，按 Network Zone 聚合被纳入报告的多个 Runs，并只把已形成人工 Verification 结论的条目投影为正式漏洞或未发现验证项。只输出实际有纳入 Run 或 included Verification 的 Zone；缺失的 I区、II区、III区不生成空标题。对外交付格式为单文件 HTML 和 DOCX（由以示例报告为蓝本制作的干净模板填充生成），两者出自同一聚合模型。

## 结构性概念

| 术语 | 含义 | 载体 |
|------|------|------|
| ScanOptions | 一次 Run 的执行参数（Targets/Ports/Tools/Profile/HostWorkers 等） | `internal/app.ScanOptions` |
| PrepareScanRequest | 从用户输入构建 ScanOptions 的摄入请求 | `internal/app.PrepareScanRequest` |
| ToolRunOptions | 单工具单独执行（非完整扫描）的参数 | `internal/app.ToolRunOptions` |
| Profile | 工具组合/参数预设（如 `normal`/`fast`） | `ScanOptions.ProfileName` |
| tools.Runner | 执行外部工具的接缝（真实 exec / 测试 fake） | `internal/tools.Runner` |

## 备注

- **无 CONTEXT.md 历史**：本文件为首版，由架构深化（候选 #1）过程从代码提炼确立。
- **领域词中英并用**：代码标识符用英文（如 `TargetScan`），讨论可用中文（「单目标扫描结果」）。
