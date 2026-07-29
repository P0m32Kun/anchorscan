# 调查并处置异常完成扫描运行

## Goal

给出 `run-20260729-093454.039551000` 的可审计状态根因，并修复导致该 Run 可选检测失败的产品配置缺陷。

## Confirmed Facts

- supplied `report.json` 的 provenance 为 `version: dev`，运行从 `2026-07-29T01:34:54Z` 到 `01:50:17Z`。
- 该报告有且仅有一个失败的检测事实：`190.10.10.201:22/tcp`，`nuclei`，`failed`，`command_failed`。
- 对应 artifact `nuclei-190.10.10.201-22-template.jsonl` 显示仓库 SSH 模板运行时错误，最终报 `Could not run nuclei: no templates provided for scan`。
- 检测计数还包含一个已完成 Nuclei 检查；大量 `skipped/no_matching_rule` 是正常未适用，不是失败。
- `nuclei -validate -t config/nuclei-templates/ssh-mini-brute.yaml` 在本机 v3.11.0 成功，因此静态验证不能覆盖该 SSH JavaScript 模板的运行时错误。
- `/Users/kun/nuclei-templates/RBKD-templates` 的 `HEAD`（`b4d76f9`）追踪 `javascript/default-logins/ssh-mini-brute.yaml`，但该工作树将文件标为 `D`；它不是 sparse checkout。项目副本与该 RBKD blob 不同，必须在受控 fixture 中做实际 A/B 验证，不能假设任一版本正确。

## Requirements

- 以该 DetectionCheck 解释 `completed_with_errors`，不将其误标为 `completed`。
- 修复 `config/nuclei-templates/ssh-mini-brute.yaml` 的运行时错误，同时保持最多四次 SSH 凭据尝试、首次命中停止的既有安全边界。
- 在 `../lab` 的 OpenSSH fixture 上进行受控验证：容器地址 `172.22.0.2:22`，宿主映射 `127.0.0.1:10022`，凭据 `lab/lab`。测试开始前须确认 Docker daemon 已启动；不得对原生产 IP 重跑。
- 使用 RBKD `HEAD:javascript/default-logins/ssh-mini-brute.yaml` 作为只读对照；由于其 `Port: "22"`，不能直接以宿主 `127.0.0.1:10022` 作为等价测试，必须选择容器网络内地址或仅为实验创建临时适配副本。

## Acceptance Criteria

- [ ] 根因结论精确指向上述 failed DetectionCheck 和对应 artifact。
- [ ] 修复后的模板在受控 SSH 服务上不再产生运行时 template error，并遵守 2 用户 × 2 密码、首次命中停止的上限。
- [ ] 测试证明该类 Nuclei template runtime failure 会留下 failed DetectionCheck，Run 仍正确为 `completed_with_errors`。
- [ ] 不修改历史 Run、其检测事实或状态。

## Out of Scope

- 对原生产地址重新扫描。
- 将 SSH 弱口令检测扩大为官方大字典爆破。
