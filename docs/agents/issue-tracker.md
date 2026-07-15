# Local Issue Tracker

本项目使用仓库内的持久化 spec 和 ticket，不连接外部 issue tracker。

- 功能 spec：`docs/plans/<feature>/spec.md`
- 设计决策：同目录下的 `product-design.md` 和可选 `technical-design.md`
- 执行 ticket：`docs/plans/<feature>/tickets/<NN>-<slug>.md`
- ticket 按依赖顺序编号，`Blocked by` 明确阻塞边；只实施阻塞项已完成的 frontier ticket。
- `ready-for-agent` 表示 ticket 内容已批准，不表示其阻塞项已经完成。

实施一个 ticket 时：

1. 在独立 worktree/分支开始前记录当前 `main` 的提交作为 review fixed point。
2. 将 ticket 路径和对应 spec 路径交给 `implement`。
3. 只在 spec 已确认的测试 seam 上使用 `tdd`，按单个垂直切片完成 red-green 循环。
4. 定期运行聚焦测试；功能完成时运行全量测试和静态检查。
5. 提交候选实现后，以步骤 1 的 fixed point 和 spec 路径运行 `code-review`，分别处理 Standards 与 Spec 发现。
6. 修正发现、重新验证并提交最终结果，然后把 ticket 状态改为 `done`。

不得同时维护旧式合并任务清单或写死的提交 SHA。计划失效时先修订 spec/ticket，再继续实现。
