# run-20260729-093454 completed_with_errors 根因与修复

## 状态结论

`completed_with_errors` 是**正确状态**，不应改为 `completed`。根因是产品缺陷，已修复。

## 根因（双重，已用受控实验复现）

1. **`Port: "{{Port}}"`**：nuclei 禁止 javascript 模板的 `Port` 参数含 DSL 表达式（`could not compile request: cause="'Port' variable cannot contain any dsl expressions"`）。模板在加载期被丢弃。
2. **外部 payload 文件**：`payloads/users-mini.txt` / `passwords/passwords-mini.txt` 既依赖 CWD 解析，又需要 `-lfa` 才能访问（`access to helper file ... denied`）。生产 `RunNucleiTemplate` 不切换 CWD、不传 `-lfa`，两者都不满足。

两个问题叠加 → 模板加载期被剔除 → `Found 1 templates with runtime error` → `Could not run nuclei: no templates provided for scan` → `failed/command_failed` DetectionCheck → Run `completed_with_errors`。

`-validate` 只查静态 YAML，**不能**覆盖 javascript 模板的运行时错误（实验：`-validate` 返回 success，运行时仍 FTL）。

附加发现：未签名 javascript 模板也会被 nuclei v3.11 默认拒绝执行（仅警告）——这是加载门，与本次内容缺陷独立。

## 决策：不在本仓库内置模板

按架构原则（nuclei 模板由外部模板库管理：私有 RBKD-templates 与官方 nuclei-templates 合并部署在同一目录、一起生效，本项目只调用 nuclei + `-tags`），采用架构修复而非内容修补：

- SSH 弱口令模板归属 RBKD-templates；用户已在 RBKD 侧删除该模板的 `default-login` tag，使 `-tags ssh -exclude-tags default-login` 能精确选中它（ssh tag、无 default-login）、排除官方 223 组的 ssh-default-logins。
- 端口覆盖已验证成立：debug 实证 `-target 127.0.0.1:10022` 使模板 `Port:"22"` 运行时为 `10022`。
- 用户已用受控 lab SSH（lab/lab）验证 RBKD 模板能正确命中。

## 已实施的 anchorscan 侧改动

- 删除 `config/nuclei-templates/`（ssh-mini-brute.yaml + payloads/）。
- `config/service-tags.yaml`：SSH 规则移除 `template:`，改由 `nuclei_tags:["ssh"]` + `exclude_tags:["default-login"]` 路由。
- `Makefile`：移除 `cp -R config/nuclei-templates`。
- 测试同步：`scan_prepare_test.go`（SSH 断言改为 `-tags ssh`）、`nuclei_tags_test.go`（SSH 经 tags 选中）、`profile_test.go`（注释更新）。
- 文档同步：`README.md`、`docs/project-status.md`。
- 保留 `RunNucleiTemplate` / `TagRule.Template`：operator 单工具路径（`tool_run.go`）仍按需执行模板，与默认扫描管线解耦。

## 验证

`go build ./...`、`go vet`、`go test ./...`、`make package`、`make package-smoke` 全部通过；归档含 5 个必需配置、无 nuclei-templates。

## 不变更

历史 Run `run-20260729-093454` 的 DetectionCheck 事实与 `completed_with_errors` 状态；release 打包/平台矩阵；扫描授权边界。
