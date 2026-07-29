# 02 - 修复 SSH Nuclei 模板运行时失败

**What to build:** 修复 `config/nuclei-templates/ssh-mini-brute.yaml` 的运行时错误，使受控 SSH fixture 上的 Nuclei 检查可执行，同时保留 DetectionCheck 失败和 Run `completed_with_errors` 的既有历史事实语义。

**Blocked by:** 01 - 规范扫描 Console 工具输出已归档。该阻塞仅用于维持一次一个 ready frontier，并非产品代码依赖。

**Status:** ready-for-agent；在 Ticket 01 归档前不得开始。

## 已确认根因

现场 `run-20260729-093454.039551000` 唯一失败 DetectionCheck 是 `190.10.10.201:22/tcp` 的 `nuclei failed/command_failed`。对应 artifact 为 `nuclei-190.10.10.201-22-template.jsonl`，最终错误是 `Could not run nuclei: no templates provided for scan`。大量 `skipped/no_matching_rule` 是未适用，不是失败。

静态 `nuclei -validate -t config/nuclei-templates/ssh-mini-brute.yaml` 已在 Nuclei v3.11.0 通过，不能证明 SSH JavaScript 模板运行时可执行。因此修复必须包含受控 fixture 的实际运行验证。

## 行为契约

- 历史 Run、artifact 和 DetectionCheck 不得修改；该 Run 继续是 `completed_with_errors`。
- 修复后的模板在受控 OpenSSH 服务上不发生 template runtime error。
- 尝试预算保持 2 个用户名 x 2 个密码，首次命中停止；不得扩展为官方大字典或对现场 IP 重跑。
- Nuclei 模板实际失败仍应持久化 `failed/command_failed`，Run 仍正确为 `completed_with_errors`。
- RBKD `HEAD:javascript/default-logins/ssh-mini-brute.yaml` 仅为只读对照。其固定 `Port: "22"` 与项目模板的 `{{Port}}` 语义不同，未经 fixture A/B 验证不得直接替换。

## 测试 seam

- App fake Runner/Store：Nuclei 失败持久化 `failed/command_failed` 且 Run 为 `completed_with_errors`。
- Docker 实验室：确认 daemon 可用后，在 `../lab` OpenSSH fixture 实际运行模板；容器地址 `172.22.0.2:22`、宿主映射 `127.0.0.1:10022`、实验凭据 `lab/lab`。不得扫描原现场地址。

## 验收

- [ ] 新增失败状态回归测试先以旧行为失败，再以最小修复转绿。
- [ ] 根因结论准确链接到上述 DetectionCheck 和 artifact，而非把 skipped 项解释为失败。
- [ ] 修复后的模板在受控 SSH 服务实际运行且无 template runtime error，保持 2x2/首次命中停止上限；静态 `-validate` 只作辅助。
- [ ] 测试证明此类 Nuclei runtime failure 保留 `failed/command_failed`，Run 仍为 `completed_with_errors`。
- [ ] 聚焦测试、self-check、Standards/Spec 双轴只读评审和 `make pr-check` 全部通过；PR、合并与 Trellis complete gate 证据已记录。

## 非目标

- 对生产/现场地址重新扫描。
- 修改 Nuclei 路由、模板选择或历史扫描事实。
- 将 SSH 弱口令检测扩大为通用凭据爆破。
