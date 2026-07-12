# Brainstorm Summary

- 用户于 2026-07-12 确认采用稳定入口加最少叶子资源的方案。
- 先用固定输入锁定 `WriteHTML` 字节输出，再将模板正文原样迁移到包内 `embed.FS`。
- `/static/app.js` 与 `/static/style.css` 保持稳定入口；只提取可独立验证、无隐含共享状态的页面职责。
- 不采用 ES modules、bundler、转译器、前端框架或第三方依赖。
- 本 change 最后实施；无法证明加载顺序或 CSS 级联等价的代码保持原位。

