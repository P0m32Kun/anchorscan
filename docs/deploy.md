# AnchorScan 部署与运维

AnchorScan 面向单机、已授权的内网扫描。首次安装、从源码构建、启动 Web 控制台和执行首个扫描请遵循 [README](../README.md) 的唯一快速开始；本文只记录部署差异、DOCX、升级、备份和运行限制。

## 发布归档与运行目录

Release 归档支持 linux/amd64、darwin/arm64 和 windows/amd64。归档不包含 `fathom`（必配：scan_target 内唯一端口/指纹引擎）、`rustscan`、`nmap`、`httpx` 或 `nuclei`，这些外部工具必须由操作者安装并置于 `PATH`，再运行 `anchorscan doctor` 检查。启用达梦默认口令检测时，还需要在 `tools.nuclei_templates` 配置 Nuclei 社区模板仓库根目录，使 `javascript/detection/dameng-detect.yaml` 可读。

从归档根目录运行程序，并保留其目录结构：其中的 `config/`、`tools/docx-render/` 和 DOCX 模板是运行期 sidecar。运行 `doctor` 首次初始化时会创建 `config/default.yaml`、`data/` 和 SQLite 数据库。

运行期目录：

```text
config/default.yaml    本机工具路径、超时和扫描配置（不纳入版本控制）
config/ports-highrisk.txt  可在 Web 配置页维护的端口预设
data/scans.sqlite      SQLite 数据库
data/projects/         项目 Evidence
reports/                扫描和项目导出结果
```

默认 Web 监听 `127.0.0.1:8088`。这是本机单操作者控制面；不要将其暴露到 LAN 或公网。CLI 的 `--args` 和 Web 单工具的“原生参数”会绕过默认安全限制，调用会审计保存但不会进入客户报告。

## DOCX 导出

项目 DOCX 导出依赖归档中的 `tools/docx-render/`、模板与 [uv](https://docs.astral.sh/uv/)。安装 uv 后从归档根目录或源码仓库根目录运行程序；不要单独移动 sidecar 或模板。若 `doctor` 或导出提示 sidecar 缺失，重新解压完整归档，不要用本机生成的配置替代发布文件。

DOCX 是正式项目交付格式；同一 Network Zone 的多个扫描 run 会聚合到一个章节，Evidence 和 Verification 由项目数据库提供。

## 备份与恢复

在升级前或迁移到新设备前创建备份：

```bash
./anchorscan backup
```

默认归档写入 `data/backups/anchorscan-backup-<timestamp>.tar.gz`，包含 SQLite 一致快照、项目 Evidence 和必要配置；不包含 `reports/` 或运行期 Artifact。使用 `--include-artifacts` 才会包含后者。

恢复会替换 `data/scans.sqlite`、`data/projects/` 和 `config/`：

```bash
./anchorscan restore --archive data/backups/anchorscan-backup-<timestamp>.tar.gz
```

恢复前会验证 manifest 的路径、大小和 SHA-256；校验失败不会替换目标数据。备份和恢复都拒绝存在活动 Run Lease 的情况，先取消或等待运行结束。

## 升级

1. 确认没有活动 Run Lease，并执行 `backup`。
2. 解压新归档到新的目录，不要覆盖仍在运行的旧目录。
3. 保留并审阅旧目录的 `config/default.yaml`；新版本的默认配置字段以示例和 `doctor` 输出为准。
4. 将备份恢复到新工作目录，运行 `anchorscan doctor`，再做一次已授权的小范围扫描。

程序启动时会执行 SQLite migration。若 migration、数据库或 sidecar 检查失败，停止升级并保留旧目录与备份，依据 `doctor` 的具体诊断处理。

## 知识库（catalog）运维

catalog 为**单源模式**：发行归档不包含 catalog，catalog 只在知识库仓库（Pentest-Playbook `handbook-v3/dist/catalog.json`，协议 version 2、`source: handbook-v3`）更新。安装后需自行 clone 知识库仓库，并把配置 `knowledge_base.path` 指向其 `dist/catalog.json`（相对路径相对配置文件目录解析），重启 AnchorScan 生效。默认配置 `knowledge_base.path` 为空，此时知识库禁用，`/kb` 页显示 disabled 与明确诊断（无文件可加载）。AnchorScan 运行时只读取操作者显式配置的路径，不访问其他仓库；测试 fixture 锁定上游 producer artifact checksum（commit 57d739e，SHA-256 `7d8ce203a503f63b8d733e6c07fa10c2f1bbb1daf4d5c0619b61e553f374224e`），仅用于测试、不进发行物。

- **外部路径**：`knowledge_base.path` 可改为外部 JSON（catalog v2）或旧版 Markdown 手册（相对路径相对配置文件目录解析）；留空禁用知识库。
- **外部更新**：进入克隆的知识库仓库执行 `git pull` 拉取新 catalog（或另存新文件后修改路径），重启 AnchorScan 生效。
- **诊断与恢复**：外部文件缺失、JSON 无效或协议版本不符时，`/kb` 页与报告页显示明确的 unavailable 诊断，不会回退到另一份知识库；恢复方式为修复外部文件，或重新 clone/还原知识库仓库。
- **safety/status/legacy 边界**：JSON 条目按 catalog v2 保留 safety（safe / optional / manual-gated）与 status（stable / needs-review），所有命令出口由服务端按条目门禁放行；旧版 Markdown 条目标记为 legacy-unknown，命令按不低于 manual-gated 强度确认。

升级时保留并审阅旧目录的 `config/default.yaml`：若旧配置未设置 `knowledge_base.path`，升级后知识库保持禁用（不会自动启用），按上文配置 clone 路径即可。

## 运行限制与操作说明

- 一份数据库同一时刻只允许一个 pipeline scan 或单工具运行；SQLite Run Lease 会阻止并发所有者。
- 默认主机发现模式为 `auto`：IPv4 的存活探测由 fathom scan 内置（ICMP + TCP 回退），不再有外层 nmap -sn；IPv6（fathom 仅 IPv4）保留 nmap `-sn`。已确认在线的授权资产可显式使用 `--discovery assume-up`——anchorscan 侧不做任何存活预处理，scope 内全部地址直接进入扫描（fathom 内部探测照常），fathom 调用参数不变。该选择会进入配置快照和报告，且不会自动启用 UDP 扫描。
- Linux/macOS 取消会终止扫描器进程组；Windows 只保证直接启动的进程终止。
- `completed_with_errors` 表示主扫描已有结果但可选检查失败；Detection Coverage 是本次执行事实，不是漏洞覆盖率或安全保证。
- 工具超时默认关闭；配置 `30s`、`5m` 等显式时长后，超时会记录为 `failed` 或 `completed_with_errors`。
