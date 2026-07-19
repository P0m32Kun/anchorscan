# 09 — 增加克制的视觉层次与效果点缀

**What to build:** 在不改变功能与信息架构的前提下，为共享应用外壳、主操作、仪表盘焦点和可交互内容面增加环境光、材质层级与轻量反馈，解决界面过平、缺少设计感的问题。

**Blocked by:** 08 — 完成桌面 QA 与双轴审查。

**Status:** done

**Execution skills:** `ask-matt`、`ui-ux-pro-max`、`frontend-visual-design`、`browser:control-in-app-browser`、`ponytail`、`code-review`。

- [x] 更新产品设计中的二次视觉校准原则。
- [x] 仅使用现有 HTML 与共享 CSS，不增加依赖、素材或构建步骤。
- [x] 仪表盘有一个明确焦点，侧栏、卡片和主按钮形成一致层级。
- [x] 表格、代码与证据区域保持实色和高可读性。
- [x] 1440px 与 1280px 下无页面级横向溢出，焦点与 reduced-motion 行为不退化。
- [x] 运行现有测试，并按 fixed point 完成 Standards 与 Spec 双轴审查。

## 验收记录

- 1440px 仪表盘、1280px 扫描表单与报告页均无页面级横向溢出；长端口串保持在默认配置面内。
- `go test ./...` 通过 326 个测试，原生前端测试通过；Standards 与 Spec 复审均无遗留发现。
- 完整浏览器冒烟仍停在既有文案定位器：脚本查找“发起扫描”，当前页面使用“新建扫描”；本 ticket 未扩大范围修改该基线问题。
