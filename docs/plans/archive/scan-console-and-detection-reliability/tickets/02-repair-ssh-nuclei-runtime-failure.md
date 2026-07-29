# 02 - 归档 SSH Nuclei 模板运行时修复（过时）

**Status:** done（不实施）。

## 处置结论

原计划假设仓库存在 `config/nuclei-templates/ssh-mini-brute.yaml`，并要求修复其 Nuclei JavaScript 运行时错误。调查已证明该假设不成立：当前仓库未跟踪或打包任何 Nuclei 模板；`config/service-tags.yaml` 对 SSH 仅按 `ssh` tag 调度并排除官方 `default-login` 大字典；`README.md` 与 `docs/project-status.md` 同样声明私有 RBKD 模板由外部合并部署，而非项目内置资源。

历史 Run `run-20260729-093454.039551000` 的 `190.10.10.201:22/tcp` nuclei `failed/command_failed` 与 artifact `nuclei-190.10.10.201-22-template.jsonl` 记录的最终错误 `Could not run nuclei: no templates provided for scan` 是不可变执行事实。历史记录称 Nuclei v3.11.0 对该路径的静态 `-validate` 曾通过，但当前 checkout 不含该模板，无法复现或归因该历史环境的部署状态。

## 保留事实与非目标

- 历史 Run、artifact、DetectionCheck 与 `completed_with_errors` 状态保持不变。
- `skipped/no_matching_rule` 仍表示未适用，不改写为失败。
- 不扫描现场 IP `190.10.10.201`，不启动凭据尝试，不修改 Nuclei 路由、模板选择或产品代码。
- 若未来需要 SSH 小字典检测，必须以独立产品任务定义模板供应者、固定版本/部署位置、2x2 尝试预算、首次命中停止和受控 lab runtime 验证；不得复活本 ticket 的失效实现方案。

## 归档验收

- [x] 已关联历史失败事实：`190.10.10.201:22/tcp` 的 nuclei `failed/command_failed`、`nuclei-190.10.10.201-22-template.jsonl` artifact、最终错误 `Could not run nuclei: no templates provided for scan`；未将 skipped 记录误判为失败。
- [x] 已验证当前 checkout 没有 `ssh-mini-brute.yaml` 或其他项目内置 Nuclei 模板，并记录 tags-only 的当前部署契约。
- [x] 已明确选择不实施，不触碰历史事实、生产地址、凭据预算或产品代码。
- [x] 已将后续可能的外部模板部署需求拆分为独立、需重新批准的产品决策。
