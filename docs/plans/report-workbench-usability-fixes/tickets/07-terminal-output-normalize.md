# 07 — 工具终端输出归一化与字体（issue 6）

**What to build:** 工具 stdout 在进入事件流前做归一化——剥离 ANSI 转义、把 `\r` 进度覆写处理成换行/最终态；终端等宽字体补覆盖（制表符等）。

**Blocked by:** None

**Status:** done

## 完成条件

- 从 `app/tool_run.go` 抽出纯函数 `normalizeToolOutput(s string) string`：剥 ANSI（`\x1b\[[0-9;]*[A-Za-z]` 等）、处理 `\r`（保留每行 `\r` 之后的最终态或转成换行，择一并加 `ponytail:` 注释）。
- `emitTool` 写入 output 前统一过 `normalizeToolOutput`（原生参数路径 line 151 与各分支输出）。
- `style.css` 的 `--mono` 增加覆盖更广的等宽 fallback（如 JetBrains Mono / "Cascadia Code" / Nerd Font 之后再退回既有栈）。
- 测试接缝：`normalizeToolOutput` 的 red-green 单测（含 ANSI、`\r`、纯文本三例）。
- 前端 `tool-form.js` / `run-status.js` 无需再处理控制字符（Go 侧已归一）。
