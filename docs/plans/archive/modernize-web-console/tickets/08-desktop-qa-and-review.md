# 08 — 完成桌面 QA 与双轴审查

**What to build:** 清理迁移遗留，完成所有桌面页面的交互、无障碍、视觉和回归验收，并按 fixed point 对 Standards 与 Spec 进行最终审查。

**Blocked by:** 07 — 现代化独立导出报告。

**Status:** done

**Execution skills:** `browser:control-in-app-browser`、`code-review`、`ponytail`。

- [x] 在实施开始时记录的 main fixed point 上运行 Standards 与 Spec 双轴 `code-review`。
- [x] 1440px 页面矩阵全部通过，1280px 下无非预期页面级横向溢出。
- [x] 检查 hover、focus-visible、disabled、loading、empty、error、popover、disclosure 和长文本状态。
- [x] 键盘可到达所有主要操作，焦点清晰；文本和状态对比度达到基本 WCAG AA。
- [x] 删除未使用的旧深色、橙色品牌、移动端和迁移期 CSS/JS。
- [x] 确认没有新增前端依赖、构建步骤、远程字体或业务功能。
- [x] 运行全量 Go 测试、原生 JS 测试和仓库现有静态检查。
- [x] 修复 review 发现并重新验证；用户已授权连续执行，以自动化视觉验收替代逐票截图确认。

## 验收记录

- fixed point：`6345b337d60b44b4095fda86f7675a4a5253ea92`。
- 1440px 与 1280px 下已检查项目、运行、导入、工具、知识库和配置页面；均无页面级横向溢出。
- 发现并修复空事件 API 返回 `null` 导致运行页轮询异常、1280px 固定最小宽度溢出，以及两处浅色主题中不可见的旧深色滚动条。
- Standards 与 Spec 双轴复审均无遗留发现；`make test build package` 通过。
