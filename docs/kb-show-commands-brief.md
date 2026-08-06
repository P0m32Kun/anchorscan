# 任务书：KB 详情页命令显示策略调整

**本文件即任务书。** 先读仓库根 AGENTS.md，再执行本文件。

## 决策

用户拍板方案 A：KB 详情页始终显示命令。知识库是参考入口不是执行入口；命令门禁只在报告/工作台的命令生成流程中触发（已由 Ticket 04 的 `enforceCommandGate` 覆盖）。

## 改动

1. `internal/web/knowledgebase.go` 第 44 行：`ShowCommands` 的赋值改为 `true`（或直接移除条件判断）。
2. `internal/web/templates/knowledgebase_detail.html` 第 38-39 行：移除"原始命令已由服务端门禁隐藏"分支，改为始终渲染命令（保留"知识库未提供可用命令"的 else 分支，因为有些条目确实没有 verify）。
3. KB 详情页保留显示 safety/effects/cleanup/status 信息（不变）——让用户在查阅时就清楚这条命令的风险等级。

## 铁律

- 不改 `enforceCommandGate` 及报告/工作台的命令门禁逻辑。
- 禁止 git commit/push。
- 不引入新依赖。

## 验收

```bash
go test ./internal/web/...
go build ./...
```

- [ ] KB 详情页对所有有条目的条目（含 needs-review/optional/manual-gated/legacy）都显示验证命令。
- [ ] 无 verify 的条目仍显示"知识库未提供可用命令"。
- [ ] safety/effects/cleanup/status 信息仍正常展示。
- [ ] 报告/工作台的命令门禁不受影响（既有 safety gate 测试全绿）。
