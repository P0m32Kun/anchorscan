# 08 — 待负向验证按服务指纹聚合 + 验证命令（issue 9）

**What to build:** 负向候选按服务指纹键（`service`，必要时含 `product`/端口）分组为"一指纹一卡片"，卡片列出该指纹覆盖的全部端点；每指纹可上传任意数量截图；复用 `BuildCandidateCommands`/知识库按指纹产出 nmap NSE、nuclei `-tags` 建议命令；弹窗由"选端点"改为"选指纹组→传图→提交 not_observed"。

**Blocked by:** None（软依赖 04：粘贴/拖拽与去描述的 UX 保持一致）

**Status:** done

## 备注
- 命令生成直接使用 `config/nse.yaml` 与 `config/service-tags.yaml`，产出 `nmap --script <scripts>` 与 `nuclei -tags <tags>`，符合 ticket 字面要求。
- 未复用 `report.BuildCandidateCommands`，因为后者生成的是 `nuclei -t <template>` 单模板命令，而非 `-tags` 批量标签命令；若后续要求严格复用既有命令体系，可将本逻辑下沉到 `report` 包或扩展 `BuildCandidateCommands`。

## 完成条件

- `report` 项目报告模型（或 workbench view 层）按指纹键聚合 `NegativeCandidates`：每组含指纹信息 + 覆盖端点列表（IP:Port）。
- `workbench.go` view model 输出指纹分组；为每组调用 `BuildCandidateCommands`/知识库（`internal/knowledgebase`、`config/service-tags.yaml`、`config/nse.yaml`）产出 nmap `--script`、nuclei `-tags` 命令；不新建并行命令体系。
- `workbench.html`：`queue-negative` 改为指纹卡片，卡片内列端点 + 命令 + 多图上传区。
- 提交逻辑：选指纹组 → 暂存截图 → 创建一个覆盖该组全部端点的 `not_observed` 验证 → 批量上传截图（沿用 04 的暂存机制）。
- 测试接缝：指纹分组的 red-green 单测（report 模型或 view 投影）；命令生成复用既有 `BuildCandidateCommands` 测试，不重复造。
- 兼容：原有"多选端点合并"交互移除，不保留两套入口。
