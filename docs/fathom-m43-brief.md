# 任务书：fathom 集成 M4.3 — 收尾清理（doctor/web/文档/死代码）

> 本任务书由编排方（Hermes）下达，可直接阅读执行。实施完成后不要执行任何 git 提交/推送操作，由编排方审查后统一处理。
> Spec：`docs/plans/fathom-integration/spec.md`（v2.0）
> 前置：M4.1（PR #42）+ M4.2（PR #43）已合并。fathom 已是唯一扫描路径。

## 背景

M4.2 把 scan_target 前段切成了 fathom 唯一路径，但周边配套（doctor 检查、config 页面、文档、注释）还停留在 rustscan+nmap 时代。M4.3 收尾：让用户可见的每一处都反映 fathom 是唯一扫描引擎。

## 必读

1. `AGENTS.md`（仓库根）
2. `docs/plans/fathom-integration/spec.md`（v2.0，理解设计反转）
3. `internal/doctor/doctor.go`（toolCheck 列表，第 48-51 行）
4. `internal/web/templates/config.html`（工具路径表单 + 超时表单）
5. `config/default.yaml.example`（已改过 fathom 必填注释）
6. `CONTEXT.md`（领域词汇表）
7. `CHANGELOG.md`（[Unreleased] 段）
8. `docs/project-status.md`（停留在 2026-07-24）
9. `README.md`（用户文档）

## Scope

### 1. doctor 适配
- `internal/doctor/doctor.go`：加 `toolCheck("fathom", cfg.Tools.Fathom, false)`（**required=false 在 doctor 里表示"检查是否存在"**——实际 required 逻辑在 preflight；doctor 是诊断展示，fathom 缺失应标红但不是 doctor 自己阻断）
- 或者参照 preflight 的 `checkRequiredFathom` 模式：doctor 里 fathom 标注为必需（missing → fail），rustscan 标注为可选（missing → ok with note "not used in scan pipeline"）
- **选择哪种由你判断**，但 doctor 输出里 fathom 必须出现，rustscan 必须有"扫描路径不再使用"的说明

### 2. web config 页面适配
- `internal/web/templates/config.html`：
  - 工具路径表单加 fathom 输入框（放在 rustscan 前面或替换 rustscan 的位置——fathom 是主引擎应排在最前）
  - rustscan 输入框：加注释或调整 label 说明"仅单工具执行模式使用，扫描流水线不再调用"
  - 超时表单加 fathom（已在 config 结构里，模板漏了）
  - 端口格式帮助文本（第 108 行"端口格式保持 rustscan 习惯"）改为提及 fathom

### 3. 文档更新
- `CHANGELOG.md` [Unreleased]：加 fathom 集成条目（M4.1+M4.2：fathom 成为唯一侦察引擎，替代 rustscan+nmap -sV）
- `docs/project-status.md`：更新 Last reviewed 日期；Implemented capabilities 段反映 fathom（scan pipeline 从 rustscan→nmap→httpx→NSE/nuclei 改为 fathom→httpx→NSE/nuclei）
- `CONTEXT.md`：加 fathom 相关领域词汇（如果还没有）
- `README.md`：工具依赖/快速开始部分反映 fathom 为必配工具

### 4. 注释清理
- 扫描路径中遗留的 rustscan 相关注释（如 scan_target.go 顶部注释如果还提 rustscan）
- `internal/app/tool_run.go`：单工具执行保留 rustscan/nmap（这是独立功能），但注释可注明"单工具模式，非扫描流水线"

**不要做**：
- 不删除 `internal/tools/rustscan.go` / `internal/tools/nmap.go` 的函数定义（单工具模式 tool_run.go 仍用）
- 不删除 rustscan/nmap 的 config 字段（单工具模式仍用）
- 不改 scan_target.go 的逻辑（M4.2 已完成）
- 不做 git 操作
- 不引入新依赖

## 铁律

1. 零新依赖
2. 不删除单工具模式的 rustscan/nmap 功能（tool_run.go 的 runRustscanTool/runNmapTool 保留）
3. 诚实报告
4. 已知未跟踪文件（spikes/ 等）不得删除或修改
5. 完成后不得自行 commit
6. 报告文件：`docs/reports/fathom-m43-report.md`

## 验收

1. `go build ./...` 通过
2. `go test ./... -count=1` 全过
3. `make web-smoke` 通过（config 页有 fathom 输入框后 smoke 不断）
4. doctor 输出包含 fathom（`anchorscan doctor` 或测试验证）
5. config.html 有 fathom 输入框
6. CHANGELOG / project-status / README 反映 fathom
7. 报告 `docs/reports/fathom-m43-report.md`
