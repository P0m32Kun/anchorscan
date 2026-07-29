# 执行计划

1. 为纯 formatter 写 RED 测试：UTC 样例、跨日、毫秒补零、无效输入。
2. 实现共享 formatter，并替换两个 Console 的字符串拼接。
3. 运行前端单测/typecheck/build；人工检查 after_id、排序和 API payload 未变。
4. 执行浏览器 smoke，确认两处显示完全相同。
