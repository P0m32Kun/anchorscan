# 项目级渗透测试与交付报告（HTML + DOCX）

## Problem Statement

当前 `Project` 同时承担任务容器和扫描参数预设：创建项目时要求目标、端口、排除项和档位；Web 报告则严格绑定单个 Run。真实渗透测试以一次客户任务为单位，同一任务会跨 I、II、III 区或其他分区，在多个网段、不同交换机接入点上创建多次扫描，最终只交付一份报告。

本地数据库中的甘肃数据已经体现该错位：同一次工作被拆成 4 个 Project，每个 Project 只有 1 个 completed Run；其中 3 个名称属于 I 区不同网段，1 个属于 III 区。正确形态应是一个“甘肃电力内网安全检查任务” Project，下面按 I 区和 III 区组织这些 Runs。

当前 Run 报告已经支持按漏洞聚合、复制 IP:PORT、生成验证命令并带参数打开单工具页，但这些能力被限制在单个 Run，工具页也会丢失 Project、Zone、验证项和返回位置。系统尚不能保存截图 Evidence、记录人工 Verification，或生成项目级汇总表与正式 HTML 报告。

## Solution

把 Project 收敛为一次渗透测试任务，在 Project 内维护 Network Zones，并让每个 Web Scan Run 在创建时选择 Zone、填写本次扫描参数和接入信息。Project Report 聚合用户明确纳入报告的多个 Runs；Run Report 继续承担单次运行的技术排查。

在 Project 中增加报告工作台：跨 Runs 聚合漏洞候选、人工验证结论与截图 Evidence；复用现有知识库、筛选、端点复制和命令生成能力，形成“发现 → 筛选 → 生成命令 → 打开工具页 → 验证 → 粘贴/上传截图 → 纳入报告”的闭环。

正式交付生成两种格式：单文件 HTML 和 DOCX，二者由同一个 ProjectReport 聚合模型驱动。原始示例 DOCX 不直接充当运行时模板（直接格式泛滥、固定行数表格、无内容控件），而是以它为蓝本一次性制作干净的 DOCX 模板，运行时把元数据、漏洞描述、汇总表和 Evidence 填充进模板。用户可见的 JSON/CSV 报告下载入口删除；扫描流水线内部 `report.json`、SQLite 数据和必要的 TXT 端点清单继续保留。

## Confirmed Product Decisions

- 一个 Project 对应一次渗透测试任务和一份正式 Project Report。
- Project 可包含 I、II、III 区及用户新增的其他 Zone。
- 一个 Zone 可包含多个网段、多个接入点和多次 Scan Run。
- 目标、端口、排除项、扫描档位和高级工具参数属于 Scan Run，不属于 Project 默认值。
- 每个 Web Scan Run 必须选择一个 Zone；重新运行默认继承原 Run 的 Zone 和参数，仍需用户确认。
- Project Report 聚合多个 Runs；不得继续以单 Run 直接充当正式交付报告。
- 确认漏洞存在必须建立 `confirmed` Verification 并至少保存一张截图 Evidence。
- 负向截图只用于已发现服务指纹、相关 NSE 与 nuclei 检查已完成、且没有非 `info` Finding 的端点；它先成为待人工验证候选，不能自动宣称不存在漏洞。
- 负向结论使用“本次验证未发现”（`not_observed`），不使用绝对的“不存在漏洞”。
- `inconclusive` Verification 不计入已存在或未发现汇总。
- 正式报告导出 HTML 与 DOCX 两类；删除 JSON/CSV 下载能力，但保留内部 JSON 制品。
- 报告元数据（报告标题、测试对象、测试人员、起止日期等）在项目管理界面维护，报告页只做引用、预览和缺失检查。
- Evidence 按漏洞类型归属 Verification，不按 IP：同一类漏洞的 confirmed Verification 至少一张截图即可覆盖全部关联资产；同一类验证项的 not_observed Verification 同样一张截图覆盖本次验证的全部端点，不要求每个 IP 单独截图。

## Project and Run Data

Project 创建和编辑至少维护：

- 任务名称；
- 被测单位名称；
- 报告标题（默认由被测单位生成，允许覆盖）；
- 测试对象（可与被测单位不同，例如具体系统名称）；
- 任务说明；
- 测试开始/结束日期；
- 测试人员；
- Zones 及稳定排序。

Scan Run 创建至少维护：

- Project 与 Zone；
- 可读名称或自动生成的运行标签；
- 本次目标、端口、排除目标、排除端口和档位；
- 接入点/交换机说明、测试机 IP 和备注；
- 是否纳入 Project Report；
- 运行种类：完整扫描或单工具验证。

## Verification and Evidence Rules

`Finding` 是工具观察事实，`DetectionCheck` 是检测引擎执行事实，二者都不得被人工验证操作改写。`Verification` 是报告编制事实，可以从 Finding/知识库条目发起，也可以为负向验证手工建立。

Verification 包含：Project、Zone、漏洞身份、展示标题、危险等级、结论、描述、修复建议、关联资产、来源 Runs、报告排序和纳入状态。知识库匹配成功时以稳定 Entry ID 分组；待补充项以现有 pending key 建立报告草稿并要求人工确认标题与等级。

Evidence 归属于 Verification。当前只接受 PNG/JPEG 截图，支持粘贴、拖拽、选择文件、填写说明、排序和删除。截图文件进入受管理的 Project 数据目录，SQLite 只保存相对路径、媒体类型、尺寸、SHA-256、说明和顺序；不得把图片 BLOB 写进数据库或引用用户机器上的任意绝对路径。当同一截图覆盖多个资产时，Evidence 说明必须写明本次验证覆盖的资产或验证范围，避免把“共享截图”误解为对未验证端点的结论。

截图数量按漏洞类型计，不按资产计：一条 Verification 要求至少一张截图，与其关联的 IP:port 数量无关。同一类漏洞影响 50 个 IP 时仍是一份证据；同一类验证项在多个端点上未发现时也是一份证据。

### Positive candidate

跨被纳入的 Runs 收集非 `info` Findings，按知识库 Entry ID 聚合；同一漏洞影响资产按规范化 `IP:port` 去重和稳定排序。用户建立 `confirmed` Verification、补齐内容并上传至少一张 Evidence 后，条目才进入正式报告。

### Negative candidate

端点只有同时满足下列条件才进入“待负向验证”队列：

1. 被纳入的 Run 中存在 Service Fingerprint；
2. 对该 `IP:port/protocol` 的 NSE 与 nuclei DetectionCheck 均为 `completed`；
3. 该端点不存在非 `info` Finding；允许存在 `info` Finding 或仅有服务开放/指纹事实；
4. 用户选择明确的漏洞/验证项，可一次勾选多个同类候选端点，实际执行验证并上传一张共享截图；
5. 用户显式提交 `not_observed` 结论，生成一条关联全部选中端点的 Verification。

DetectionCheck 缺失、失败、跳过、取消或中断时只能进入“无法判定/检查未完成”，不能作为负向候选。甘肃现有 Runs 没有持久化 DetectionCheck，因此迁移后可聚合已有正向 Findings，但不能自动生成负向结论。

## Project Report Content

正式 HTML 与 DOCX 共用同一语义结构（沿用提供的示例 DOCX）：封面、目录/导航、概述、测试方法、结果汇总、I/II/III/其他 Zone、漏洞详情、未发现验证项和结论。用户已确认整个第二节（含工具与方法介绍）是固定模板正文，原封不动保留，不从 Runs 或人工补充记录生成。

所有模板中的 `XX` 必须由 Project Report 数据槽替换，至少包括被测单位、报告标题、测试对象、测试日期、测试人员、各 Run 的接入点/测试机 IP/范围以及结论统计。同一 Zone 存在多次接入或多个网段时逐 Run 展示，不丢失现场上下文。导出前若仍存在未绑定占位符，必须失败并明确列出缺失字段；不得把 `XX` 原样交付。

### 表 3-1 渗透测试结果汇总表

表 3-1 只消费被纳入报告的 `confirmed` Verifications，动态生成任意行数。每行固定为：

1. 序号；
2. 安全问题：Verification 的正式标题，不包含危险等级后缀；
3. 关联资产/域名：聚合所有确认存在该漏洞的唯一 `IP:port`，按 IP、端口稳定排序；
4. 严重程度：Verification 的最终危害等级。

聚合身份使用稳定知识库 Entry ID 或人工 Verification ID，不能使用可编辑标题作为键。相同漏洞在多个 Runs、Zones 或重复扫描中命中同一端点时只显示一次；不同端点全部合并到同一行。表内资产按 `IP:port` 输出，不附加协议；漏洞详情仍可保留协议和来源 Run。

危害等级保留知识库或人工确认的原等级，支持 critical/high/medium/low；`info` 不进入漏洞汇总。结论段只统计实际出现的等级，不能为适配旧模板而把 critical 静默降为 high。

## Workflow and UX

Project 页面提供四个稳定入口：概览、扫描任务、验证工作台、正式报告。

验证工作台按 Zone 和状态组织三个队列：

- 待确认漏洞：跨 Runs 聚合的非 `info` 候选；
- 待负向验证：满足双引擎完成且无非 `info` Finding 的指纹端点；
- 检查未完成：缺少 DetectionCheck 或存在失败/跳过/取消/中断。

漏洞卡片保留现有复制 IP:PORT、复制整条、生成 Nuclei/Nmap/MSF 命令能力，并新增验证结论、Evidence 数量、上传/粘贴截图和纳入报告状态。点击“带参数打开工具页”必须携带 Project、Zone、Verification、原始参数和返回地址；工具运行完成后可回到原验证项，不要求用户重新筛选和查找。

## Acceptance Criteria

- 新建 Project 时不再要求目标、端口、排除项或扫描档位；报告标题、测试人员、起止日期等元数据在项目管理界面维护。
- 新建 Web Scan Run 时必须选择 Zone，并填写本次扫描参数；同一区可创建任意多个 Runs。
- Project 页面能按 Zone 查看所有 Runs，并选择哪些 Runs 纳入报告。
- 项目级漏洞聚合跨所有纳入 Runs 生效，重复命中不重复资产。
- 正向候选没有 `confirmed` Verification 或截图时不得进入正式报告。
- 负向候选严格遵守 DetectionCheck 规则，旧数据不会被误判为已验证安全。
- 用户能从候选卡片生成命令、打开预填工具页、返回原验证项并粘贴/上传证据。
- 表 3-1 行数、标题、唯一 IP:port 集合和危险等级与正式 Verifications 一致。
- 项目正式 HTML 不包含未替换的 `XX`，按 Zone 输出详情并保持单文件离线可读。
- 项目 DOCX 由干净模板填充生成，同样不包含未替换占位，内容、汇总表、Zone 详情和 Evidence 与 HTML 出自同一 ProjectReport 模型。
- 同一类漏洞的多个受影响 IP 共享一份截图证据；同一类未发现验证项的多个端点共享一份截图证据。
- Web 不再提供 JSON/CSV 报告下载或资产 CSV；HTML、复制与必要 TXT 清单仍可用。
- 内部 `report.json` 生成、CLI/扫描恢复所依赖的数据和历史 Run Report 不回归。

## Out of Scope

- PDF 导出。
- 报告编号、版本、审核人与批准人；参考模板没有这些槽位，后续有明确格式时再增加。
- OCR、截图内容识别或自动判断截图是否证明漏洞。
- 根据指纹猜测具体负向漏洞；用户必须选择明确验证项。
- 自动合并任意旧 Projects；系统无法可靠推断业务归属。
- 通用 Project 合并 UI；甘肃历史数据使用可预览、先备份的一次性迁移命令。
- 多用户、权限、云存储或分布式任务。

## Delivery Rule

本 spec 已批准（2026-07-21）。按 `tickets/` 的阻塞边一次实施一个 ticket；批准后将第一个无阻塞 ticket 01 标记为 `ready-for-agent`。如果实现发现 Project/Zone/Verification 的模型无法满足本 spec，先修订计划，不得在代码里临时绕过。
