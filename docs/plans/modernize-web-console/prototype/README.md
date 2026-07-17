# 报告页视觉原型

这是 ticket 01 的一次性 UI 原型，不会进入生产代码。

启动：

```bash
python3 -m http.server 8099 --directory docs/plans/modernize-web-console/prototype
```

打开 `http://127.0.0.1:8099/report-prototype.html?variant=a`；可将 `a` 改为 `b` 或 `c`，也可用页面底部按钮或左右方向键切换。

- A：聚焦工作台，摘要、筛选和结果形成连续工作面。
- B：调查双栏，左侧固定 run 上下文，主区专注筛选和结果。
- C：简报画布，先给出处理优先级，再进入完整结果。

原型用于确认信息层级和布局，不代表最终组件或生产实现。
