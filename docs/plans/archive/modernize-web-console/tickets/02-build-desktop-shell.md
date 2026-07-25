# 02 — 建立桌面外壳与视觉基础

**What to build:** 将批准的视觉令牌落入全局 CSS，重构桌面应用外壳、左侧导航和共享原语，并用仪表盘完成第一条生产页面垂直切片。

**Blocked by:** 01 — 批准报告页视觉原型。

**Status:** done

**Execution skills:** `frontend-visual-design`、`codebase-design`、`browser:control-in-app-browser`、`ponytail`。

- [ ] `style.css` 使用批准的浅色设计令牌、系统字体、统一焦点环、圆角、分隔线和状态色。
- [ ] `base.html` 保留所有导航入口和当前页状态，使用新的轻量左侧边栏和简化蓝色盾牌。
- [ ] 删除移动页眉、遮罩、侧栏 toggle、移动断点和对应脚本；设置 1280px 桌面最小宽度。
- [ ] 按钮、输入、select、textarea、状态、提示、表格和 disclosure 获得统一共享样式。
- [ ] 仪表盘重新建立层级，不增加新统计或后端字段。
- [ ] 现有路由、链接和导航行为不变，相关 Go/JS 测试通过。
- [ ] 提供 1440px 仪表盘截图和 1280px 布局检查，经用户批准后完成。
