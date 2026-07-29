# Technical Design

## Root Cause and Invariant

该 Run 的 `completed_with_errors` 正确反映了一个 `nuclei failed/command_failed` DetectionCheck。失败不是 Nmap/Dameng 的 skipped 行造成的，而是 SSH 自定义模板执行阶段报 runtime error 后没有可执行模板。

`nuclei -validate` 只证明 YAML 静态可载入；修复验收必须运行模板。模板仍保持 `service-tags.yaml` 的精确 `-t` 路径、`hostport` target 与四次凭据尝试上限。

## Repair Shape

在 `../lab` 的 OpenSSH fixture 上以受控实验复现：容器为 `172.22.0.2:22`，宿主映射 `127.0.0.1:10022`，凭据 `lab/lab`。先确认 Docker daemon 可用。使用 `git show HEAD:javascript/default-logins/ssh-mini-brute.yaml` 读取 RBKD 基线；该外部工作树当前将该受追踪文件标为删除，且非 sparse checkout，不能擅自恢复。

RBKD 基线与项目副本不同：RBKD 使用内联凭据、`pitchfork` 和固定 `Port: "22"`，项目副本使用 payload 文件、`clusterbomb` 和 `{{Port}}`。实验须先独立验证两者，不能在未证实前将任一模板直接替换为另一份。

## Status Contract

检测阶段失败仍写 `failed/command_failed`，`RunScan` 因 partial error 写 `completed_with_errors`。不修复历史记录。

## Rollback

回退模板变更即可恢复旧扫描行为；绝不迁移或重算已持久化 DetectionCheck。
