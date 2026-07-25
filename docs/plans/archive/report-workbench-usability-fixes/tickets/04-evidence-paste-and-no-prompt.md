# 04 — 证据上传粘贴/拖拽与去描述（issue 3）

**What to build:** 把证据上传的 `paste` 与 `drop` 监听提到验证弹窗（正向/负向）层级，不再依赖单 div 焦点；兼容 macOS Cmd+V；去掉每次上传的描述 `prompt`；正向新建验证也能直接上传（本地暂存，提交时随验证一起落库）。

**Blocked by:** None

**Status:** done

## 完成条件

- 在 `verify-dialog` / `negative-dialog` 上注册 `paste` 与 `drop`，提取剪贴板/拖拽里的图片；兼容 Cmd+V（粘贴事件与按键无关，监听 paste 即覆盖）。
- 抽出纯函数 `imagesFromClipboardData(items): File[]`（或等价），在 `app.test.mjs` 写 red-green 单测。
- 删除 `handleFile` / `handleNegFile` 里的 `prompt`；`caption` 默认空串；后端 `uploadEvidence` 已接受空 caption，无需改。
- 正向弹窗：新建验证（`verifyId` 为空）时图片暂存本地（仿 `negPendingFiles`），提交保存验证后批量上传；消除当前 `if(!vid) return` 的静默空操作。
- 负向弹窗沿用 `negPendingFiles` 暂存，逻辑保持一致。
