# AnchorScan

`anchorscan` 是一款面向已授权内网环境的便携式自动化扫描工具。

核心思路是「**指纹驱动、精准分类、服务多引擎**」：`rustscan` 做端口发现 → `nmap -sV` 做服务指纹识别 → 每个已识别服务按 `nuclei` + NSE 双引擎规则表（`config/service-tags.yaml` + `config/nse.yaml`）同时调度；RDP 服务可额外启用可选 `rdpscan` 引擎检测 BlueKeep（CVE-2019-0708）→ Web 服务额外走 `httpx` → 结果统一落入 SQLite → 导出 JSON / HTML / DOCX 报告。

## 快速开始

### 方式一：下载预编译归档（推荐）

到 [Releases 页面](../../releases) 下载并解压对应平台的 `.tar.gz` 归档（支持 linux/amd64、darwin/arm64、windows/amd64），无需安装 Go 环境。归档内包含 DOCX 导出 sidecar 与正式模板；使用 DOCX 导出还需安装 [uv](https://docs.astral.sh/uv/)。

```bash
# Linux / macOS 示例
tar -xzf anchorscan-v2.0.0-linux-amd64.tar.gz
cd anchorscan-v2.0.0-linux-amd64
chmod +x anchorscan
./anchorscan doctor    # 自动生成配置、检测工具路径
./anchorscan web       # 启动 Web 控制台
```

### 方式二：从源码编译

需要本机安装 [Go](https://go.dev/dl/) 和 [Node.js](https://nodejs.org/) 20+。Node 只用于构建嵌入式 Web 静态资源，运行已编译的 `anchorscan` 不需要 Node。

```bash
npm ci
```

#### 1. 前置依赖

确保本机已安装 `rustscan`、`nmap`、`httpx`、`nuclei`，并在系统 `PATH` 中可找到。配置文件无需手动创建——首次运行会自动生成 `config/default.yaml`，工具路径从 PATH 自动检测。

如需手动调整（例如工具不在 PATH、想固定路径），编辑自动生成的 `config/default.yaml` 即可，参考 [config/default.yaml.example](./config/default.yaml.example)。

#### 2. 自检

```bash
go run ./cmd/anchorscan doctor
```

检查配置、工具路径、数据库、报告目录是否就绪。

#### 3. 启动 Web 控制台（推荐日常使用）

```bash
go run ./cmd/anchorscan web
```

默认监听 `127.0.0.1:8088`，配置读 `config/default.yaml`，数据库用 `data/scans.sqlite`，无需传参。打开 http://127.0.0.1:8088 即可使用。中文界面，本机单兵操作。

如需覆盖，可选传参：

```bash
go run ./cmd/anchorscan web --listen 127.0.0.1:9000 --config custom.yaml --db other.sqlite
```

#### 4. 或直接命令行扫描

```bash
go run ./cmd/anchorscan scan --target 127.0.0.1 --ports top1000
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
go run ./cmd/anchorscan scan --target 127.0.0.1 --profile slow
```

## 常用命令

```bash
make test      # 运行全部测试
make build     # 编译到 dist/anchorscan
make package   # 打包到 dist/
make pr-check  # 完整质量门禁（首次执行前运行 npm ci 与 npx playwright install chromium）
```

导入已有的 Nmap XML：

```bash
go run ./cmd/anchorscan import-nmap --xml path/to/scan.xml
```

单工具调用（不走完整流水线，仅跑 rustscan / nmap / httpx / nuclei 之一）：

```bash
go run ./cmd/anchorscan tool nmap --mode alive --target 192.0.2.10
```

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
