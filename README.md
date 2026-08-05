# AnchorScan

`anchorscan` 是一款面向已授权内网环境的便携式自动化扫描工具。

核心思路是「**指纹驱动、精准分类、服务多引擎**」：`rustscan` 做端口发现 → `nmap -sV` 做服务指纹识别 → 按服务指纹和适用规则独立调度 `nuclei`、NSE、`httpx` 等引擎 → 结果统一落入 SQLite → 导出 JSON / HTML / DOCX 报告。RDP 服务可额外启用可选 `rdpscan` 检测 BlueKeep（CVE-2019-0708）。

默认流水线保留非 SSH 服务的弱口令/默认凭据检测；SSH 通过 `-tags ssh -exclude-tags default-login` 调用私有 RBKD-templates 的 `ssh-mini-brute` 模板（最多 2 用户 × 2 密码，4 次尝试）。RBKD-templates 与官方 nuclei-templates 合并部署在同一目录、一起生效；本项目不内置任何 nuclei 模板，仅用 `-tags` 选择服务。使用 `--args` 或 Web 单工具「原生参数」会绕过默认安全限制，调用记录会审计保存但不会进入客户报告。

## 快速开始

### 方式一：下载预编译归档（推荐）

到 [Releases 页面](https://github.com/P0m32Kun/anchorscan/releases) 下载并解压对应平台的 `.tar.gz` 归档（支持 linux/amd64、darwin/arm64、windows/amd64），无需安装 Go 环境。归档内包含 DOCX 导出 sidecar 与正式模板；使用 DOCX 导出还需安装 [uv](https://docs.astral.sh/uv/)。

```bash
# Linux / macOS 示例
tar -xzf anchorscan-v2.0.0-linux-amd64.tar.gz
cd anchorscan-v2.0.0-linux-amd64
chmod +x anchorscan
./anchorscan doctor    # 自动生成配置、检测工具路径
./anchorscan web       # 启动 Web 控制台
```

### 方式二：从源码编译

需要本机安装 [Go](https://go.dev/dl/) 1.26+ 与 [Node.js](https://nodejs.org/) 20+。先获取源码；macOS/Linux 或安装了 GNU Make 的 Windows 环境可执行：

```bash
git clone https://github.com/P0m32Kun/anchorscan.git
cd anchorscan
npm ci
make build
```

Windows PowerShell 不需要安装 `make`：

```powershell
git clone https://github.com/P0m32Kun/anchorscan.git
cd anchorscan
npm ci
npm run build:web
New-Item -ItemType Directory -Force dist
go build -o dist/anchorscan.exe ./cmd/anchorscan
```

编译产物为 `dist/anchorscan`（Windows 为 `dist/anchorscan.exe`）。Node 只用于构建嵌入式 Web 静态资源，运行已编译的 `anchorscan` 不需要 Node。

#### 1. 前置依赖

确保本机已安装 `rustscan`、`nmap`、`httpx`、`nuclei`，并在系统 `PATH` 中可找到。配置文件无需手动创建——首次运行会自动生成 `config/default.yaml`，工具路径从 PATH 自动检测。

如需手动调整（例如工具不在 PATH、想固定路径），编辑自动生成的 `config/default.yaml` 即可，参考 [config/default.yaml.example](./config/default.yaml.example)。

#### 2. 自检

```bash
./dist/anchorscan doctor
```

检查配置、工具路径、数据库、报告目录是否就绪。

#### 3. 启动 Web 控制台（推荐日常使用）

```bash
./dist/anchorscan web
```

默认监听 `127.0.0.1:8088`，配置读 `config/default.yaml`，数据库用 `data/scans.sqlite`，无需传参。打开 http://127.0.0.1:8088 即可使用。中文界面，本机单兵操作。

如需覆盖，可选传参：

```bash
./dist/anchorscan web --listen 127.0.0.1:9000 --config custom.yaml --db other.sqlite
```

#### 4. 或直接命令行扫描

```bash
./dist/anchorscan scan --target 127.0.0.1 --ports top1000
```

不传 `--json` 时，JSON 报告默认写到 `reports/scan-<时间戳>.json`。如需 HTML 报告或自定义路径，加 `--html reports/test.html`。

## 端口格式

`--ports` 或表单端口框支持以下写法：

| 写法 | 含义 |
|------|------|
| `top1000` | 使用 rustscan `--top` 扫常见 1000 端口 |
| `100-1000` | 使用 rustscan `--range 100-1000` 扫端口范围 |
| `80,443,8080` | 使用 rustscan `--ports 80,443,8080` 扫自定义端口列表 |

不再接受 `full`、`highrisk` 或混合格式。需要全端口时填写 `1-65535`。

**高危端口列表维护**：进入「全局配置」页，底部「高危端口列表」面板可可视化增删端口并保存，写回 `config/ports-highrisk.txt`（每次保存自动备份）。扫描表单的「插入高危端口列表」会写入实际 CSV，扫描输入不再接受 `highrisk` 短语。

## 扫描档位

- `slow`：脆弱网络 / 老旧设备
- `normal`：默认，均衡
- `fast`：健康高速网络，多目标

```bash
./dist/anchorscan scan --target 127.0.0.1 --profile slow
```

## 常用命令

```bash
make test      # 运行 Go 与 JavaScript 测试
make build     # 编译到 dist/anchorscan
make package   # 打包到 dist/
make pr-check  # PR 质量门禁：测试、构建/打包和 Playwright Chromium smoke
make e2e       # 使用真实扫描器运行 Docker 实验室 E2E
```

首次运行 `make pr-check` 前执行 `npm ci` 和 `npx playwright install chromium`。测试分层与选型原则见 [docs/testing-strategy.md](./docs/testing-strategy.md)。

导入已有的 Nmap XML：

```bash
./dist/anchorscan import-nmap --xml path/to/scan.xml
```

单工具调用（不走完整流水线，仅跑 rustscan / nmap / httpx / nuclei 之一）：

```bash
./dist/anchorscan tool nmap --mode alive --target 192.0.2.10
```

## 漏洞知识库（KB）

AnchorScan 的漏洞知识库（KB）为报告 enrich、验证工作台与命令生成提供条目：扫描发现会按 nuclei/nse/CVE/名称匹配到 KB 条目，展示漏洞描述、修复建议，并按条目声明的 safety/status 档位生成验证命令。

**发行自带 catalog**：发布归档包含 `config/catalog.json`（catalog 协议 **version 2**、`source: handbook-v3`，与程序版本匹配），默认配置 `knowledge_base.path: catalog.json` 即指向该包内文件，解压后开箱可用。该文件是上游 producer artifact（`handbook-v3/dist/catalog.json`）的字节级拷贝，来源与 SHA-256（`7d8ce203a503f63b8d733e6c07fa10c2f1bbb1daf4d5c0619b61e553f374224e`）记录在 `internal/knowledgebase/testdata/README.md`，运行时不访问任何外部仓库。

**协议版本**：JSON 知识库必须满足 catalog v2 顶层协议（`version: 2`、`source: "handbook-v3"`、`entry_count` 与条目数一致）。不满足协议、JSON 无效、缺失或无法读取时，`/kb` 页显示明确的 **unavailable** 诊断；**不会回退到另一份知识库**（包括包内默认副本），也不会当作安全条目处理。部分条目不合法时知识库进入 degraded 并跳过/禁用对应内容，其余条目照常可用。

**外部路径与更新**：`knowledge_base.path` 可改为任意外部 JSON（catalog v2）或旧版 Markdown 手册路径（相对路径相对配置文件目录解析）；留空则禁用知识库。更新步骤：用新的 catalog 文件覆盖该路径所指文件（或另存后修改路径），重启 AnchorScan 生效。恢复方式：还原受支持的文件，或恢复归档中的 `config/catalog.json` 与默认配置。

**safety/status/legacy 行为边界**：JSON 条目保留 `safety`（safe / optional / manual-gated）与 `status`（stable / needs-review），命令按服务端门禁放行——`stable + safe` 直通；`needs-review` 需显式 acknowledgement；`optional` / `manual-gated` 需确认 effects（与 cleanup）；缺失或非法 safety 的条目不返回命令。旧版 Markdown 手册仍可阅读与匹配（未移除兼容），但其条目标记为 **legacy-unknown**，命令按不低于 manual-gated 的强度确认，不能继承 safe 默认值。详细规则见 [docs/plans/catalog-json-knowledgebase/spec.md](./docs/plans/catalog-json-knowledgebase/spec.md)。

## Web 控制台功能

- 主题切换：默认跟随系统，单按钮切换浅色/深色，显式偏好跨刷新保留
- 项目交付：按 Network Zone 组织扫描、人工 Verification 与 Evidence，并导出项目 HTML / DOCX 报告
- 扫描创建：选择项目分区后填写目标、端口和档位，支持目标文件导入、排除项和高危端口预设
- 运行与单工具：实时事件、连续输出、取消操作和常用参数预设
- 验证工作台：按服务指纹整理正向/负向候选，支持证据粘贴或拖放与确认结论
- 报告阅读：风险摘要、检测覆盖、筛选、证据详情、主机/漏洞聚合，以及 IP / IP:PORT / URL 复制导出
- 全局设置：编辑主题、工具路径、超时、原始 YAML 和高危端口列表

## 说明

- 所有扫描需在已授权环境下进行
- 不含登录/多用户/分布式/SaaS，定位为本机单兵工具
- 部署细节见 [docs/deploy.md](./docs/deploy.md)
