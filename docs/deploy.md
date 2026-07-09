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
- 先用精确端口或 `highrisk` 预设，再考虑 `top1000` / `full`
- 对脆弱网络优先使用 `slow` 档位
