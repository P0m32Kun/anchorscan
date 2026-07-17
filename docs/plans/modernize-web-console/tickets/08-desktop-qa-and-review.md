# 08 — 完成桌面 QA 与双轴审查

**What to build:** 清理迁移遗留，完成所有桌面页面的交互、无障碍、视觉和回归验收，并按 fixed point 对 Standards 与 Spec 进行最终审查。

**Blocked by:** 07 — 现代化独立导出报告。

**Status:** draft

**Execution skills:** `browser:control-in-app-browser`、`code-review`、`ponytail`。

- [ ] 在实施开始时记录的 main fixed point 上运行 Standards 与 Spec 双轴 `code-review`。
- [ ] 1440px 页面矩阵全部通过，1280px 下无非预期页面级横向溢出。
- [ ] 检查 hover、focus-visible、disabled、loading、empty、error、popover、disclosure 和长文本状态。
- [ ] 键盘可到达所有主要操作，焦点清晰；文本和状态对比度达到基本 WCAG AA。
- [ ] 删除未使用的旧深色、橙色品牌、移动端和迁移期 CSS/JS。
- [ ] 确认没有新增前端依赖、构建步骤、远程字体或业务功能。
- [ ] 运行全量 Go 测试、原生 JS 测试和仓库现有静态检查。
- [ ] 修复 review 发现并重新验证；最终截图经用户确认后，将全部 tickets 标记为 done。
