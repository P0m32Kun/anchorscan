# AnchorScan 部署指南

本文面向单机、本地、已授权的内网扫描场景，目标是帮助你在一台新设备上尽快把 AnchorScan 跑起来。

## 1. 前提

AnchorScan V1.2 仍然**不打包外部扫描器**。新设备需要你自己安装并配置以下工具：

- `rustscan`
- `nmap`
- `httpx`
- `nuclei`

建议先确认它们都能单独执行：

```bash
rustscan --help
nmap --version
httpx -version
nuclei -version
```

同时需要本机具备 Go（用于源码运行或重新编译）。

## 2. 从源码启动

```bash
git clone git@github.com:P0m32Kun/anchorscan.git
cd anchorscan
make test
make build
```

编译产物会输出到：

```text
dist/anchorscan
```

## 3. 使用打包产物

生成当前平台的归档包：

```bash
make package
```

默认会生成类似下面的文件：

```text
dist/anchorscan-v1.1-3-g<commit>-darwin-arm64.tar.gz
```

归档包中包含：

- `anchorscan` 可执行文件
- `config/default.yaml`
- `docs/deploy.md`
- `docs/README.md`

解压后建议目录结构如下：

```text
anchorscan/
  anchorscan
  config/default.yaml
  data/
  reports/
```

## 4. 配置外部工具路径

编辑 `config/default.yaml`，把工具路径改成新设备上的实际位置，例如：

```yaml
tools:
  rustscan: /opt/homebrew/bin/rustscan
  nmap: /opt/homebrew/bin/nmap
  httpx: /opt/homebrew/bin/httpx
  nuclei: /opt/homebrew/bin/nuclei
```

如果你有自定义的 `nse.yaml` 或 `service-tags.yaml`，也放在同级 `config/` 目录即可。

## 5. 首次自检

先运行 doctor：

```bash
./dist/anchorscan doctor \
  --config config/default.yaml \
  --db data/scans.sqlite \
  --reports reports
```

重点关注这些结果是否为 `ok`：

- `config`
- `rustscan`
- `nmap`
- `ports`
- `nse rules`
- `tag rules`
- `database`
- `reports`

如果是从源码目录直接运行打包后的二进制，也可以写成：

```bash
./dist/anchorscan doctor --config config/default.yaml --db data/scans.sqlite --reports reports
```

## 6. 首次扫描验证

推荐先做一次最小验证：

```bash
./dist/anchorscan scan \
  --config config/default.yaml \
  --target 127.0.0.1 \
  --ports 80,443,8080 \
  --db data/scans.sqlite \
  --json reports/smoke.json \
  --html reports/smoke.html
```

你会先看到预检输出，再进入扫描阶段：

```text
[scan] preflight targets=1 ports=80,443,8080 profile=normal workers=1
```

如果预检失败，AnchorScan 会在真正调用 `rustscan` 之前停止。

## 7. 启动 Web 控制台

```bash
./dist/anchorscan web \
  --config config/default.yaml \
  --db data/scans.sqlite \
  --listen 127.0.0.1:8088
```

浏览器打开：

```text
http://127.0.0.1:8088
```

## 8. 升级建议

- 升级前先备份 `data/scans.sqlite`
- V1.2 开始使用显式 SQLite migration；程序启动时会自动补齐 schema
- 如果迁移或数据库打开失败，`doctor` 会直接报出 `database: fail`

## 9. 常见建议

- 先用 `doctor`，再扫描
- 先用精确端口，再考虑 `top100` / `top1000` / `full`
- 对脆弱网络优先使用 `slow` 档位，必要时再单独覆盖工具参数
