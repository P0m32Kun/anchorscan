# Implement — 达梦数据库默认口令检测 MVP

## 1. 前置检查

- [ ] 确认 `github.com/ganl/go-dm` 可在当前网络/CI 下 `go get`。
- [ ] 读取现有 `internal/tools/*.go`、`internal/fingerprint/*.go`、`config/default.yaml` 确定接口风格。

## 2. 实现步骤

### Step 1: 新增依赖

```bash
go get gitee.com/chunanyong/dm
```

选择原因：`gitee.com/chunanyong/dm` 是官方 Go 驱动的社区封装，跨平台；`github.com/ganl/go-dm` 在 macOS 上缺少 `darwin` 构建文件。`go mod tidy` 清理未使用依赖。

### Step 2: 新增主动指纹识别模块

文件：`internal/fingerprint/probes/dameng.go`

- 定义探测 hex payload。
- 实现 `DetectDameng`。
- 实现超时控制、响应读取、命中判定。

### Step 3: 新增默认口令检测器

文件：`internal/tools/dameng.go`

- 定义 `DamengVerdict`、`DamengResult`、`DamengAuthChecker`。
- 实现 `RunDamengDefaultPassword`。
- 实现生产用 `damengDriverChecker`。
- 实现 `ParseDamengOutput`（如需要）。

### Step 4: 修改指纹归一化

文件：`internal/fingerprint/normalize.go`

- 添加达梦别名。

### Step 5: 修改扫描调度

文件：`internal/app/scan_target.go`

- 在 nmap 指纹循环中加入达梦主动识别调用。
- 在循环末尾加入 dameng POC 段，仿 rdpscan。

### Step 6: 修改配置

- `config/default.yaml` / `config/default.yaml.example`：增加 `tools.dameng`。
- `config/ports-highrisk.txt`：加 `5236`，处理 `12345`。

### Step 7: 修复研究文档

- `docs/research/vulnerability-coverage-official-sources.md`：第 48 项端口改为 5236。

### Step 8: 单元测试

- `internal/fingerprint/probes/dameng_test.go`：命中/未命中/超时。
- `internal/tools/dameng_test.go`：verdict 映射（vulnerable/safe/error）。
- `internal/app/scan_target_dameng_test.go`：调度触发条件。

### Step 9: 运行质量门

```bash
make test
make pr-check
```

## 3. 验证清单

- [ ] `go build ./...` 通过。
- [ ] `make test` 通过。
- [ ] `make pr-check` 通过。
- [ ] 新增测试覆盖 verdict 映射和识别触发条件。
- [ ] 没有破坏现有 rdpscan / nuclei / NSE 流程。

## 4. 提交信息

```
feat: detect Dameng DB default password with active protocol fingerprinting

- Add internal/fingerprint/probes/dameng.go using nuclei dameng-detect probe
- Add internal/tools/dameng.go default-password checker via Go driver
- Route dameng POC by fingerprint instead of fixed port
- Add 5236 to high-risk ports; fix 12345 mislabel in research doc
```
