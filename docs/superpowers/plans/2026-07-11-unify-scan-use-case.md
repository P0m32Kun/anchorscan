---
change: unify-scan-use-case
design-doc: docs/superpowers/specs/2026-07-11-unify-scan-use-case-design.md
base-ref: 370e7ef66842adf193c9ad2b3e4d92fda2dfa2dc
---

# 统一扫描准备用例实施计划

> **执行要求：** 使用 `superpowers:subagent-driven-development`（当前会话）或 `superpowers:executing-plans`（独立实施会话）逐任务执行。每个任务都先补测试、确认失败原因，再写最小实现；本计划不授权提交、推送或顺手修复非目标问题。

**目标：** 让 CLI `runScan` 与 Web `scanCreate` 通过同一个 `app.PrepareScan` 完成配置、目标、端口、Profile、规则、预检和 `ScanOptions` 组装，同时保持两个入口现有协议和扫描运行时行为。

**架构：** 不增加新层级或生产依赖。低层类型归 `config`，目标/端口排除归各自领域包，`internal/app/scan_prepare.go` 只负责编排，CLI/Web 只保留各自协议、路径、Store/Manager 和响应职责。

**技术栈：** Go 1.26、标准库、现有 `gopkg.in/yaml.v3` 与项目内包；不修改前端依赖、数据库 schema 或报告模型。

**设计文档：** `docs/superpowers/specs/2026-07-11-unify-scan-use-case-design.md`

**OpenSpec：**

- `openspec/changes/unify-scan-use-case/specs/shared-scan-preparation/spec.md`
- `openspec/changes/unify-scan-use-case/tasks.md`

## 执行进度

- [x] Task 1：下沉工具值类型并解除 `preflight -> app` 依赖
- [ ] Task 2：把项目目标/端口排除归位到领域包
- [ ] Task 3：建立 `app.PrepareScan` 唯一扫描准备边界
- [ ] Task 4：迁移 CLI `runScan`
- [ ] Task 5：迁移 Web、删除第二条准备路径并完成兼容验证

## 全局约束

- 不创建 `internal/scanprep`、`internal/common`、Service/Repository 接口、Builder、Factory 或 feature flag。
- `PrepareScan` 不打开 Store、不启动扫描、不创建 HTTP 响应、不生成 HTML，也不产生 run ID。
- 普通错误返回 `error`；预检错误放在完整 `preflight.Result` 中并返回 `nil error`，此时 `ScanOptions` 必须保持零值。
- 保留 Web 当前的特殊兼容行为：预检摘要使用排除前的端口串，执行选项使用排除后的端口串。
- 保留 CLI 当前的 `--target` 必填检查、预检日志、同步执行、HTML 后处理和 stdout 输出。
- 保留 Web 当前的项目必填、项目默认值优先级、400/409/303、表单重绘和异步 Manager 启动。
- 不修复 303/落库竞态、CLI run ID 冲突、Web 预检端口不一致或目标语义校验。
- 不修改 `go.mod`、`go.sum`、数据库 migration、报告格式、模板或静态资源。
- 实施前后遵循项目 `pre-edit-safety-gate`：修改前查影响半径和 `tests_for`，修改后运行 `detect_changes`；发现高风险或计划失真时停止并重新设计。
- 当前工作树已有用户 UI 设计文档，实施期间不得修改或清理这些无关文件。

## 目标 API

新增 `internal/app/scan_prepare.go`，固定以下具体值类型，不引入接口：

```go
type PrepareScanRequest struct {
	ConfigPath string

	TargetSpec     string
	PortSpec       string
	ExcludeTargets string
	ExcludePorts   string
	Overrides      config.Overrides

	DBPath         string
	JSONReportPath string
	ArtifactRoot   string

	RunID     string
	ProjectID string
	Logf      func(format string, args ...any)
}

type PreparedScan struct {
	Options   ScanOptions
	Preflight preflight.Result
}

func PrepareScan(req PrepareScanRequest) (PreparedScan, error)
```

`PrepareScan` 的固定执行顺序：

1. `config.Load`
2. `target.Parse` + `target.Exclude`
3. 选择显式端口或配置默认端口
4. `ports.ResolveForConfig` + `ports.ExcludeForConfig`
5. `config.ResolveScan`
6. `config.LoadNSERulesForConfig` + `config.LoadTagRulesForConfig`
7. 构造一次工具路径和额外参数
8. `preflight.Run`
9. 预检失败只返回诊断；预检通过才构造完整 `ScanOptions`

---

## Task 1：下沉工具值类型并解除 `preflight -> app` 依赖

**对应 OpenSpec：** 2.1

**文件：**

- 修改：`internal/config/config.go`
- 修改：`internal/config/config_test.go`
- 修改：`internal/app/scan.go`
- 修改：`internal/preflight/preflight.go`
- 修改：`internal/preflight/preflight_test.go`

### 1.1 先写类型归属测试

在现有 `TestLoadParsesToolPathsAndDefaults` 中增加编译期类型约束；复用该测试已经加载的 `cfg`，不要复制 YAML fixture：

```go
var paths ToolPaths = cfg.Tools
if paths.Rustscan != "/usr/local/bin/rustscan" || paths.Nmap != "/usr/local/bin/nmap" {
	t.Fatalf("unexpected tool paths: %#v", paths)
}
```

先运行：

```bash
go test ./internal/config -run '^TestLoadParsesToolPathsAndDefaults$'
```

预期：因 `config.ToolPaths` 尚不存在而编译失败；失败原因必须只指向缺失的新类型。

### 1.2 实现唯一类型来源

在 `internal/config/config.go` 中新增命名类型，并让 `Config.Tools` 使用它：

```go
type ToolPaths struct {
	Rustscan string `yaml:"rustscan"`
	Nmap     string `yaml:"nmap"`
	Httpx    string `yaml:"httpx"`
	Nuclei   string `yaml:"nuclei"`
}

type Config struct {
	Tools ToolPaths `yaml:"tools"`
	// Scan 和 Profiles 保持原样。
}
```

在 `internal/app/scan.go` 删除两套重复结构，改为兼容别名：

```go
type ToolPaths = config.ToolPaths
type ToolExtraArgs = config.ToolArgs
```

只增加 `internal/config` import；`ScanOptions`、`ToolRunOptions` 和现有调用方字段名不变。

### 1.3 让预检只依赖低层配置类型

在 `internal/preflight/preflight.go`：

- 删除 `internal/app` import。
- 增加 `internal/config` import。
- `Options.Tools`、`Summary.Tools` 改为 `config.ToolPaths`。
- `Options.ExtraArgs`、`Summary.ExtraArgs` 改为 `config.ToolArgs`。

在 `internal/preflight/preflight_test.go` 把 fixture 的 `app.ToolPaths` / `app.ToolExtraArgs` 改为 `config.ToolPaths` / `config.ToolArgs`。不要改变任何预检断言。

### 1.4 格式化并验证

```bash
gofmt -w internal/config/config.go internal/config/config_test.go internal/app/scan.go internal/preflight/preflight.go internal/preflight/preflight_test.go
go test ./internal/config ./internal/preflight ./internal/app
```

验收：

- YAML 兼容不变。
- `preflight` 不再 import `app`。
- `app.ToolPaths` 和 `app.ToolExtraArgs` 的所有现有调用方继续编译。
- 不出现第二套等价工具类型。

---

## Task 2：把项目目标/端口排除归位到领域包

**对应 OpenSpec：** 1.1、2.2、3.3

**文件：**

- 修改：`internal/target/parse.go`
- 修改：`internal/target/parse_test.go`
- 修改：`internal/ports/resolve.go`
- 修改：`internal/ports/resolve_test.go`

此任务只复制并验证 Web 现有语义；暂不删除 `internal/web/server.go` 中的旧 helper，等 Web 完成迁移后一次删除。

### 2.1 先写目标排除测试

在 `internal/target/parse_test.go` 增加：

```go
func TestExcludeUsesExactMatchesAndPreservesOrder(t *testing.T)
func TestExcludeReturnsAllTargetsWhenSpecIsEmpty(t *testing.T)
func TestExcludeDoesNotExpandCIDR(t *testing.T)
```

核心断言：

- `targets=["10.0.0.1", "10.0.0.0/24", "host.local"]`
- `exclude="host.local,10.0.0.1"`
- 结果严格为 `["10.0.0.0/24"]`
- 排除 `10.0.0.2` 不应影响 `10.0.0.0/24`
- 空排除串保留原顺序和全部元素

运行并确认因 `target.Exclude` 不存在而失败：

```bash
go test ./internal/target -run '^TestExclude'
```

### 2.2 实现目标排除

在 `internal/target/parse.go` 增加：

```go
func Exclude(targets []string, excludeSpec string) ([]string, error) {
	excluded, err := Parse(excludeSpec)
	if err != nil {
		return nil, err
	}
	if len(excluded) == 0 {
		return targets, nil
	}

	blocked := make(map[string]struct{}, len(excluded))
	for _, item := range excluded {
		blocked[item] = struct{}{}
	}
	out := make([]string, 0, len(targets))
	for _, item := range targets {
		if _, ok := blocked[item]; !ok {
			out = append(out, item)
		}
	}
	return out, nil
}
```

不要增加 CIDR/IP range 解析或新的校验器。

### 2.3 先写端口排除测试

在 `internal/ports/resolve_test.go` 增加：

```go
func TestExcludeForConfigFiltersCSVAndRange(t *testing.T)
func TestExcludeForConfigLoadsTop1000OnlyWhenNeeded(t *testing.T)
func TestExcludeForConfigReturnsOriginalWhenExclusionIsEmpty(t *testing.T)
func TestExcludeForConfigRejectsInvalidExcludedPort(t *testing.T)
```

至少覆盖：

| 输入端口 | 排除 | 期望 |
| --- | --- | --- |
| `22,80,443` | `22,443` | `80` |
| `80-83` | `81,83` | `80,82` |
| `top1000` + config 同目录 preset `22,80,443` | `22` | `80,443` |
| `top1000` | 空 | `top1000`，且无需读取 preset |
| `80,443` | `70000` | `invalid port` error |

先运行：

```bash
go test ./internal/ports -run '^TestExcludeForConfig'
```

预期：因新函数不存在而失败。

### 2.4 迁移端口 helper 的原有算法

在 `internal/ports/resolve.go` 增加导出边界：

```go
func ExcludeForConfig(portSpec string, excludeSpec string, configPath string) (string, error)
```

实现要求：

1. 对两个输入保持现有 `strings.TrimSpace` 行为。
2. 只有 `portSpec == "top1000"` 且排除串非空时，调用 `LoadPresetForConfig("top1000", configPath)`。
3. 原样迁移 Web 的 `expandPortSpec`、端口校验、去重、排序和 CSV 输出逻辑为包内私有函数。
4. 不把 `full`、`highrisk`、混合 CSV+range 变成新能力；这些格式仍由现有 `ResolveForConfig` 拒绝。
5. 排除全部端口时允许返回空字符串，保持当前 Web 执行字段语义。

### 2.5 格式化并验证

```bash
gofmt -w internal/target/parse.go internal/target/parse_test.go internal/ports/resolve.go internal/ports/resolve_test.go
go test ./internal/target ./internal/ports
```

验收：新 helper 的结果与当前 Web 私有 helper 一致，且尚未改变 CLI/Web 入口。

---

## Task 3：建立 `app.PrepareScan` 唯一扫描准备边界

**对应 OpenSpec：** 1.1、2.2、4.1

**文件：**

- 新增：`internal/app/scan_prepare.go`
- 新增：`internal/app/scan_prepare_test.go`

### 3.1 先写共享契约测试

在 `internal/app/scan_prepare_test.go` 使用临时目录创建：

- 最小 YAML 配置（四个工具路径、默认端口、normal/slow profile 和工具参数）。
- 可执行的临时工具文件。
- config 同目录的最小 `nse.yaml`、`service-tags.yaml`、`ports-top1000.txt`。
- 临时 DB、JSON、artifact 路径。

新增以下测试：

```go
func TestPrepareScanBuildsOptionsFromDefaultsAndOverrides(t *testing.T)
func TestPrepareScanAppliesExclusionsButKeepsPreflightPortSpec(t *testing.T)
func TestPrepareScanReturnsPreflightErrorsWithoutOptions(t *testing.T)
func TestPrepareScanReturnsWarningsWithOptions(t *testing.T)
func TestPrepareScanUsesFixedOrdinaryErrorOrder(t *testing.T)
func TestPrepareScanKeepsRuleFileFallback(t *testing.T)
func TestPrepareScanEquivalentRequestsShareExecutionFields(t *testing.T)
```

关键断言：

1. 配置默认端口、Profile、Workers 和工具参数进入 `Options`。
2. 显式 `config.Overrides` 只覆盖对应字段。
3. 目标排除后顺序不变；`Options.Ports` 是排除后的端口。
4. `Preflight.Summary.PortSpec` 是排除前的已选端口串。
5. NSE/tag 规则完整进入 `Options`，预检中的规则数量一致。
6. 缺失必需工具时：`err == nil`、`Preflight.HasErrors() == true`、`Options == (ScanOptions{})`。
7. 可选工具为空时：存在 warning、没有 preflight error，并仍返回完整 `Options`。
8. 同时提供无效端口和未知 Profile 时，先返回端口错误，结果整体为零值。
9. 用 `t.Chdir` 构造“配置同目录无规则、工作目录 `config/` 有规则”的 fixture，确认继续使用现有整文件 fallback，而不是合并两份规则。
10. 两个仅在 `RunID`、`ProjectID`、JSON/artifact 路径不同的等价请求，Targets、Ports、Tools、Profile、Workers、ExtraArgs、NSE/Tag rules 一致。

先运行：

```bash
go test ./internal/app -run '^TestPrepareScan'
```

预期：因新 API 不存在而编译失败。

### 3.2 实现请求和返回类型

按本计划“目标 API”章节在 `internal/app/scan_prepare.go` 定义 `PrepareScanRequest`、`PreparedScan` 和 `PrepareScan`。不要增加接口、构造器或可注入 loader。

### 3.3 实现固定编排顺序

函数体按以下结构实现；`toolPaths` 和 `extraArgs` 各构造一次，预检与执行选项复用同一值：

```go
func PrepareScan(req PrepareScanRequest) (PreparedScan, error) {
	cfg, err := config.Load(req.ConfigPath)
	if err != nil {
		return PreparedScan{}, err
	}

	targets, err := target.Parse(req.TargetSpec)
	if err != nil {
		return PreparedScan{}, err
	}
	targets, err = target.Exclude(targets, req.ExcludeTargets)
	if err != nil {
		return PreparedScan{}, err
	}

	portSpec := req.PortSpec
	if portSpec == "" {
		portSpec = cfg.Scan.Ports
	}
	resolvedPorts, err := ports.ResolveForConfig(portSpec, req.ConfigPath)
	if err != nil {
		return PreparedScan{}, err
	}
	resolvedPorts, err = ports.ExcludeForConfig(resolvedPorts, req.ExcludePorts, req.ConfigPath)
	if err != nil {
		return PreparedScan{}, err
	}

	effective, err := config.ResolveScan(cfg, req.Overrides)
	if err != nil {
		return PreparedScan{}, err
	}

	nseRules, err := config.LoadNSERulesForConfig(req.ConfigPath)
	if err != nil {
		return PreparedScan{}, err
	}
	tagRules, err := config.LoadTagRulesForConfig(req.ConfigPath)
	if err != nil {
		return PreparedScan{}, err
	}

	toolPaths := cfg.Tools
	extraArgs := effective.ToolArgs
	preflightResult := preflight.Run(preflight.Options{
		ConfigDir:    filepath.Dir(req.ConfigPath),
		DBPath:       req.DBPath,
		JSONPath:     req.JSONReportPath,
		ReportDir:    filepath.Dir(req.JSONReportPath),
		Targets:      targets,
		PortSpec:     portSpec,
		Tools:        toolPaths,
		Profile:      effective.ProfileName,
		Workers:      effective.HostWorkers,
		ExtraArgs:    extraArgs,
		NSERuleCount: len(nseRules),
		TagRuleCount: len(tagRules),
	})
	prepared := PreparedScan{Preflight: preflightResult}
	if preflightResult.HasErrors() {
		return prepared, nil
	}

	prepared.Options = ScanOptions{
		RunID:          req.RunID,
		ProjectID:      req.ProjectID,
		Targets:        targets,
		Ports:          resolvedPorts,
		Tools:          toolPaths,
		ProfileName:    effective.ProfileName,
		HostWorkers:    effective.HostWorkers,
		ExtraArgs:      extraArgs,
		JSONReportPath: req.JSONReportPath,
		ArtifactRoot:   req.ArtifactRoot,
		NSERules:       nseRules,
		TagRules:       tagRules,
		Logf:           req.Logf,
	}
	return prepared, nil
}
```

不要设置新的 `ConfigSnapshot` 语义；保持零值。

预检输入必须是：

- `ConfigDir = filepath.Dir(req.ConfigPath)`
- `DBPath = req.DBPath`
- `JSONPath = req.JSONReportPath`
- `ReportDir = filepath.Dir(req.JSONReportPath)`
- `Targets =` 排除后的 targets
- `PortSpec =` 排除前的 `portSpec`
- Tools/Profile/Workers/ExtraArgs/规则数量来自同一份最终值

### 3.4 格式化并验证

```bash
gofmt -w internal/app/scan_prepare.go internal/app/scan_prepare_test.go
go test ./internal/app -run '^TestPrepareScan'
go test ./internal/app ./internal/config ./internal/target ./internal/ports ./internal/preflight
```

验收：三通道契约准确（普通错误、预检错误、成功/警告），且函数没有 Store、Runner、Manager 或 HTTP 依赖。

---

## Task 4：迁移 CLI `runScan`

**对应 OpenSpec：** 1.2、3.1、4.1

**文件：**

- 修改：`cmd/anchorscan/main.go`
- 修改：`cmd/anchorscan/main_test.go`

### 4.1 先固定 CLI 边界行为

保留现有扫描测试，并增加两个针对重构边界的特征测试：

```go
func TestExecuteScanReturnsPortErrorBeforeProfileError(t *testing.T)
func TestExecuteScanDoesNotOpenStoreWhenSharedPreflightFails(t *testing.T)
```

第二个测试向 `cliDeps.openStore` 注入记录/失败函数，断言缺失 rustscan/nmap 产生现有 `preflight failed` 和 stderr 诊断时，Store 从未打开、Runner 从未调用。

先运行现有与新增测试，记录重构前基线：

```bash
go test ./cmd/anchorscan -run '^TestExecuteScan'
```

特征测试在重构前允许直接通过；它们用于约束内部迁移，而不是新增业务行为。

### 4.2 用 `PrepareScan` 替换重复编排

`runScan` 继续负责：

1. flag 定义、help 和 `--target` 精确空值检查。
2. 默认 JSON 路径生成。
3. 组装 `config.Overrides` 和 `app.PrepareScanRequest`。
4. 调用 `logPreflight`；预检错误仍返回 `errors.New("preflight failed")`。
5. 预检通过后的 DB/JSON/HTML/artifact 目录准备。
6. 打开 Store。
7. 此时才生成 run ID，并写入 `prepared.Options.RunID`。
8. 设置 CLI 的 `prepared.Options.Logf`，同步调用 `app.RunScan`。
9. HTML 后处理和 stdout 输出。

调用形态：

```go
prepared, err := app.PrepareScan(app.PrepareScanRequest{
	ConfigPath:     *configPath,
	TargetSpec:     *targetSpec,
	PortSpec:       *portsSpec,
	Overrides:      config.Overrides{/* 现有 flag 映射 */},
	DBPath:         *dbPath,
	JSONReportPath: *jsonPath,
	ArtifactRoot:   strings.TrimSpace(*artifactRoot),
})
if err != nil {
	return err
}
logPreflight(stderr, prepared.Preflight)
if prepared.Preflight.HasErrors() {
	return errors.New("preflight failed")
}
```

删除 `runScan` 内以下重复逻辑：

- `config.Load`
- 规则加载
- `target.Parse`
- 端口默认/解析
- `config.ResolveScan`
- `preflight.Run` 参数字面量
- 公共 `ScanOptions` 字段字面量

不要改 `runTool`、`runDoctor` 或其他命令；它们仍可使用 `config`、`ports` 和 `app.ToolPaths`。

### 4.3 验证 CLI

```bash
gofmt -w cmd/anchorscan/main.go cmd/anchorscan/main_test.go
go test ./cmd/anchorscan -run '^TestExecuteScan'
go test ./cmd/anchorscan
```

验收：帮助、错误文本、预检日志、Runner 参数、artifact 目录、JSON/HTML 和 stdout 与现有测试一致；预检失败前不打开 Store、不启动 Runner。

---

## Task 5：迁移 Web、删除第二条准备路径并完成兼容验证

**对应 OpenSpec：** 1.2、3.2、3.3、4.1–4.3

**文件：**

- 修改：`internal/web/server.go`
- 修改：`internal/web/server_test.go`
- 仅验证：`go.mod`
- 仅验证：`go.sum`
- 仅验证：`internal/web/templates/`
- 仅验证：`internal/web/static/`

### 5.1 先补 Web 失败通道特征测试

保留现有以下测试：

- `TestScanCreateRendersPreflightErrors`
- `TestScanCreatePassesTop1000ToRustscanTop`
- `TestScanCreatePassesPortRangeToRustscanRange`
- `TestScanCreateRejectsUnsupportedPortFormats`
- `TestScanCreateUsesProjectDefaultsAndExclusions`

增加：

```go
func TestScanCreateDoesNotStartManagerWhenPreparationReturnsOrdinaryError(t *testing.T)
func TestScanCreateKeepsConflictAndRedirectResponses(t *testing.T)
```

覆盖无效端口返回 400 且无 run、Manager 冲突仍为 409、成功仍为 303。优先复用现有 `serverSequenceRunner`、Store fixture 和项目 helper，不增加 mock 框架。

先运行：

```bash
go test ./internal/web -run '^TestScanCreate'
```

### 5.2 让 Web 只保留交付层职责

`scanCreate` 保留：

1. POST 与表单解析。
2. 项目查询和 `project_id is required`。
3. 表单值/项目默认 target、ports、profile 的选择。
4. run ID、托管 JSON 路径和 artifact 路径生成。
5. `app.PrepareScan` 普通错误到 HTTP 400 的映射。
6. 预检错误的 HTTP 400 + `renderProjectScanForm`。
7. 预检通过后的 report 目录创建。
8. `Manager.Start`、409 和 303。

调用形态：

```go
prepared, err := app.PrepareScan(app.PrepareScanRequest{
	ConfigPath:     s.opts.ConfigPath,
	TargetSpec:     targetValue,
	PortSpec:       portValue,
	ExcludeTargets: project.ExcludeTargets,
	ExcludePorts:   project.ExcludePorts,
	Overrides: config.Overrides{
		ProfileName:  coalesce(strings.TrimSpace(r.FormValue("profile")), defaultProjectProfile(project)),
		RustscanArgs: r.FormValue("rustscan_args"),
		NmapArgs:     r.FormValue("nmap_args"),
		HttpxArgs:    r.FormValue("httpx_args"),
		NucleiArgs:   r.FormValue("nuclei_args"),
	},
	DBPath:         s.opts.DBPath,
	JSONReportPath: jsonPath,
	ArtifactRoot:   artifactRoot,
	RunID:          runID,
	ProjectID:      project.ID,
})
```

普通错误仍用 `http.Error(..., http.StatusBadRequest)`；预检错误必须使用 `prepared.Preflight` 重绘表单；成功直接把 `prepared.Options` 传给 `s.manager.Start`。

### 5.3 删除旧 helper 和无用 import

确认 Web 已无调用后，从 `internal/web/server.go` 删除：

- `excludeTargets`
- `excludePorts`
- `(*server).excludePortsForScan`
- `expandPortSpec`
- `parsePortNumber`
- `compressPorts`

同时清理只为这些 helper 使用的 import。保留 `coalesce` 和 `defaultProjectProfile`，因为它们仍属于 Web 表单/项目默认值选择。

用全仓 literal 搜索确认没有第二条路径：

```bash
rg -n 'excludeTargets|excludePortsForScan|func excludePorts|func expandPortSpec|func compressPorts' .
rg -n 'preflight\.Run\(' cmd internal
rg -n 'PrepareScan\(' cmd internal
```

预期：

- 旧 Web helper 为零结果。
- 完整扫描入口不再直接调用 `preflight.Run`；唯一生产调用位于 `internal/app/scan_prepare.go`。
- `PrepareScan` 至少有 CLI、Web 两个生产调用方和 app 测试调用方。

### 5.4 定向与全量验证

```bash
gofmt -w internal/web/server.go internal/web/server_test.go
go test ./internal/web -run '^TestScanCreate'
go test ./internal/app ./internal/preflight ./internal/target ./internal/ports ./cmd/anchorscan ./internal/web
go test ./...
make test
make package
git diff --check
git diff -- go.mod go.sum
git status --short
```

再运行变更规范校验：

```bash
openspec validate unify-scan-use-case --type change --strict --json --no-interactive
```

如果仓库 CLI 约定通过 `rtk`，则给以上长输出命令加 `rtk` 前缀，但不得改变实际测试范围。

### 5.5 最终结构与安全复核

在提交给用户确认前：

1. 运行 Code Review Graph `detect_changes(detail_level="minimal")`。
2. 运行 `get_affected_flows()` 和 `get_suggested_questions()`。
3. 检查依赖方向为 `CLI/Web -> app -> config/target/ports/preflight`，且 `preflight` 不依赖 `app`。
4. 对照 OpenSpec `tasks.md` 逐项确认，不因测试通过而跳过外部契约检查。
5. 确认 `go.mod`/`go.sum` 无变化，模板、静态资源、数据库和报告文件无变化。
6. 确认用户已有 UI 设计文件未被修改。

验收：CLI 与 Web 都只剩一个扫描准备入口；所有测试、打包和 OpenSpec strict validation 通过；没有新增依赖或第二套抽象。

## 实施中止条件

出现以下任一情况时停止实施，不在当前计划外自行扩张：

- `PrepareScan` 无法在不引入 Store/Manager/HTTP 的情况下满足入口需求。
- 为保持现有行为必须改变数据库、报告、模板、扫描运行时或公共 CLI/Web 协议。
- 实现发现 Web 当前端口/预检语义与已确认设计不同。
- Code Review Graph 显示高风险影响超出 CLI/Web 扫描入口与所列低层 helper。
- 测试要求新增第三方依赖、mock 框架或新的包层级。

此时应回到设计阶段重新确认，而不是在实现阶段临时改架构。
