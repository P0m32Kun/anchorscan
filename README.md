# AnchorScan

`anchorscan` 是一款面向已授权内网环境的便携式自动化扫描工具。

核心思路是「**指纹驱动、精准分类**」：`rustscan` 做端口发现 → `nmap -sV` 做服务指纹识别 → 根据服务类型进入 `httpx`、`nuclei`、NSE 后续流程 → 结果统一落入 SQLite → 导出 JSON / HTML 报告。

## 快速开始

### 1. 前置依赖

确保本机已安装 `rustscan`、`nmap`、`httpx`、`nuclei`，并在系统 `PATH` 中可找到。配置文件无需手动创建——首次运行会自动生成 `config/default.yaml`，工具路径从 PATH 自动检测。

如需手动调整（例如工具不在 PATH、想固定路径），编辑自动生成的 `config/default.yaml` 即可，参考 [config/default.yaml.example](./config/default.yaml.example)。

### 2. 自检

```bash
go run ./cmd/anchorscan doctor
```

检查配置、工具路径、数据库、报告目录是否就绪。

### 3. 启动 Web 控制台（推荐日常使用）

```bash
go run ./cmd/anchorscan web
```

默认监听 `127.0.0.1:8088`，配置读 `config/default.yaml`，数据库用 `data/scans.sqlite`，无需传参。打开 http://127.0.0.1:8088 即可使用。中文界面，本机单兵操作。

如需覆盖，可选传参：

```bash
go run ./cmd/anchorscan web --listen 127.0.0.1:9000 --config custom.yaml --db other.sqlite
```

### 4. 或直接命令行扫描

```bash
go run ./cmd/anchorscan scan --target 127.0.0.1 --ports highrisk
```

不传 `--json` 时，JSON 报告默认写到 `reports/scan-<时间戳>.json`。如需 HTML 报告或自定义路径，加 `--html reports/test.html`。

## 端口预设

`--ports` 或表单端口框支持以下写法：

| 写法 | 含义 |
|------|------|
| `highrisk` | 高危端口列表（运维改端口 + 工控/SCADA + 标准高危服务，约 50 个） |
| `top100` | 常见 100 端口 |
| `top1000` | 常见 1000 端口 |
| `full` | 全端口 1-65535（较慢） |
| `80,443,8080` | 自定义端口列表 |
| `100-1000` | 自定义范围 |

**高危端口列表维护**：进入「全局配置」页，底部「高危端口列表」面板可可视化增删端口并保存，写回 `config/ports-highrisk.txt`（每次保存自动备份）。也可直接编辑该文件。

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

- 项目管理：保存默认目标、端口、档位，支持目标文件导入、排除目标/端口
- 扫描页：可临时覆盖目标/端口/档位，一键插入高危端口列表
- 运行页：实时事件日志，可取消扫描
- 单工具页：单独执行各引擎，带常用参数预设
- 报告页：筛选、证据详情、按主机聚合、复制/导出 IP / IP:PORT / URL 清单
- 配置页：编辑工具路径、默认端口，以及高危端口列表

## 说明

- 所有扫描需在已授权环境下进行
- 不含登录/多用户/分布式/SaaS，定位为本机单兵工具
- 部署细节见 [docs/deploy.md](./docs/deploy.md)
