# AnchorScan 部署指南

本文面向单机、本地、已授权的内网扫描场景，帮助你在一台新设备上尽快把 AnchorScan 跑起来。

## 1. 前提

AnchorScan **不打包外部扫描器**，新设备需要自行安装以下工具并加入系统 `PATH`：

- `rustscan`
- `nmap`
- `httpx`
- `nuclei`

确认它们能单独执行：

```bash
rustscan --help
nmap --version
httpx -version
nuclei -version
```

## 2. 获取程序

### 方式一：下载预编译二进制（推荐）

到 [Releases 页面](https://github.com/P0m32Kun/anchorscan/releases) 下载对应平台的二进制：

- `anchorscan-linux-amd64`（Linux x86_64）
- `anchorscan-darwin-arm64`（macOS Apple Silicon）
- `anchorscan-windows-amd64.exe`（Windows x86_64）

无需安装 Go 环境。下载后赋予执行权限：

```bash
chmod +x anchorscan-linux-amd64
```

### 方式二：从源码编译

需要本机安装 [Go](https://go.dev/dl/)：

```bash
git clone git@github.com:P0m32Kun/anchorscan.git
cd anchorscan
make build
```

编译产物输出到 `dist/anchorscan`。

## 3. 首次自检（自动初始化）

直接运行 doctor，**无需任何参数**：

```bash
./anchorscan doctor
```

首次运行会自动完成：

- 生成 `config/default.yaml`（工具路径从 PATH 自动检测）
- 创建 `data/` 目录和 `data/scans.sqlite` 数据库

确认所有检查项为 `ok`，重点关注 `rustscan`/`nmap`/`httpx`/`nuclei` 是否检测到。若某工具显示未配置，说明它不在 PATH 中，安装后重新运行 doctor 即可。

## 4. 调整工具路径（可选）

如果自动检测的工具路径需要修改（例如工具不在 PATH、想固定路径），编辑 `config/default.yaml`：

```yaml
tools:
  rustscan: /usr/local/bin/rustscan
  nmap: /usr/local/bin/nmap
  httpx: /usr/local/bin/httpx
  nuclei: /usr/local/bin/nuclei
```

参考模板见 `config/default.yaml.example`。

## 5. 首次扫描验证

```bash
./anchorscan scan --target 127.0.0.1 --ports 80,443,8080
```

不传 `--json` 时，JSON 报告默认写到 `reports/scan-<时间戳>.json`。如需 HTML 报告，加 `--html reports/smoke.html`。

## 6. 启动 Web 控制台

```bash
./anchorscan web
```

默认监听 `127.0.0.1:8088`，配置读 `config/default.yaml`，数据库用 `data/scans.sqlite`，无需传参。浏览器打开 http://127.0.0.1:8088。

Web 控制台扫描工作流：先在「项目管理」创建项目（配置默认目标、端口、档位），进入项目详情页后点「新建扫描」发起扫描。扫描自动归属当前项目并预填项目默认值，无需从全局扫描页选择项目。「扫描历史」保留为全部运行记录的运维视角，但日常扫描入口在项目详情页内。CLI 不受此约束，仍可执行无项目的临时扫描。

如需覆盖默认值：

```bash
./anchorscan web --listen 127.0.0.1:9000 --config custom.yaml --db other.sqlite
```

## 7. 目录结构

首次运行后，工作目录结构如下：

```text
anchorscan              可执行文件
config/
  default.yaml          自动生成的配置（首次运行生成）
  ports-highrisk.txt    高危端口预设（可在 Web 配置页编辑）
data/
  scans.sqlite          数据库（自动创建）
reports/
  scan-*.json           扫描报告
```

## 8. 升级建议

- 升级前先备份 `data/scans.sqlite`
- 程序启动时会自动运行 SQLite migration 补齐 schema
- 如果迁移或数据库打开失败，`doctor` 会直接报出 `database: fail`

## 9. 常见建议

- 先用 `doctor`，再扫描
- 先用精确端口 CSV 或「插入高危端口列表」，再考虑 `top1000`；全端口请明确填写 `1-65535`
- 对脆弱网络优先使用 `slow` 档位

## 10. 看懂扫描状态与检测记录

扫描结束后，在「扫描历史」或报告页会看到以下状态：

- `completed`：扫描正常结束。
- `completed_with_errors`：主扫描已经产出可用结果，但某个可选检查（例如 httpx、NSE 或 nuclei）失败；先查看报告和检测记录，不必把它当成“没有结果”。
- `failed`：无法建立扫描事实或保存关键结果；修正工具、目标或数据库问题后重新运行。
- `canceled`：有人在运行中按了中止；已经保存的结果仍可查看。
- `interrupted`：程序或机器在运行中断开；它不是断点续传，使用运行详情页的「重新运行」并确认预填的目标和端口。
- `running`：仍在执行中。

报告中的「检测执行覆盖」列出每个服务的 NSE 与 nuclei 实际状态：完成、跳过、失败、取消或中断（程序或租约失联前未完成）。它用来解释本次运行做了什么，**不是**漏洞覆盖率，也不代表目标安全。

## 11. 工具超时（默认关闭）

默认情况下所有工具都不限时，以免长扫描被程序提前停止。如确实需要限制单个工具，在 Web 的「配置管理」填写 rustscan、nmap、httpx、NSE 或 nuclei 的超时；使用 `30s`、`5m` 这样的时长，填 `0` 表示不限时。扫描档位不会改写这些全局值。
