# Local Issue Tracker

本项目使用仓库内的持久化 spec 和 ticket，不连接外部 issue tracker。

- 功能 spec：`docs/plans/<feature>/spec.md`
- 设计决策：同目录下的 `product-design.md` 和可选 `technical-design.md`
- 执行 ticket：`docs/plans/<feature>/tickets/<NN>-<slug>.md`
- ticket 按依赖顺序编号，`Blocked by` 明确阻塞边；只实施阻塞项已完成的 frontier ticket。
- `ready-for-agent` 表示 ticket 内容已批准，不表示其阻塞项已经完成。

实施一个 ticket 时：

| Skill | 入口与输入 | 产物 | 不可用时降级 |
| --- | --- | --- | --- |
| `implement` | `~/.agents/skills/implement/SKILL.md`；ticket、spec、fixed point | 最小实现、聚焦验证、候选提交与必要文档更新 | 主实施者按 ticket 逐项执行，保留命令输出和候选提交 |
| `tdd` | `~/.agents/skills/tdd/SKILL.md`；行为契约与指定 seam | 失败测试、最小修复、绿色测试 | 在 ticket 验收记录中保留同样的 red-green 证据 |
| `code-review` | `~/.agents/skills/code-review/SKILL.md`；fixed point、spec、候选 diff | Standards / Spec 双轴结论 | 两个独立只读审查分别覆盖标准和 ticket 验收，并记录结论 |

1. 在独立 worktree/分支开始前记录当前 `main` 的提交作为 review fixed point。
2. 把 ticket 路径、对应 spec 路径和 fixed point 提供给 `implement`；它负责最小实现与本地验证。
3. 对行为变化，在 spec 指定的最低充分 seam 使用 `tdd`：先写失败断言，再实现，再运行聚焦测试。若 `tdd` skill 不可用，实施者必须在 ticket 验收记录中写明同样的 red-green 证据。
4. 定期运行聚焦测试；功能完成时运行全量测试和静态检查。
5. 使用 `code-review` 以 fixed point 和 spec 路径做 Standards / Spec 双轴审查。若该 skill 不可用，安排两个独立只读审查：一个核对仓库标准，一个逐条核对 ticket 验收，并记录结论。
6. 修正 blocker/high 发现、重新验证并提交最终结果，然后把 ticket 状态改为 `done`。

不得同时维护旧式合并任务清单或写死的提交 SHA。计划失效时先修订 spec/ticket，再继续实现。
