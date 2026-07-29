# Implementation Plan

1. 确认 Docker daemon 可用，启动或复用 `../lab/docker-compose.yml` 的 SSH fixture；记录实验命令和凭据边界。
2. 从 RBKD `HEAD` 只读导出对照模板，分别在与其端口语义相符的 lab 网络路径上运行 RBKD 与项目模板，先复现并归因运行时错误。
3. 对照 Nuclei v3.11 支持的 SSH JavaScript API，最小修正项目 `ssh-mini-brute.yaml`，保持原有 2×2/stop-first-match 策略。
4. 用 fixture 证明模板实际运行，且不产生 runtime error；继续运行静态 `-validate` 作为辅助检查。
5. 在 app seam 验证工具失败仍持久化 `failed/command_failed` 且 Run 为 `completed_with_errors`。
6. 执行相关 package/runtime 质量检查和独立审查。

## Risky Files

- `config/nuclei-templates/ssh-mini-brute.yaml`：错误修复不得扩大弱口令尝试面。
- `config/service-tags.yaml`：不改变规则选择、目标类型或模板路径，除非运行时验证证明路径契约本身有误。
