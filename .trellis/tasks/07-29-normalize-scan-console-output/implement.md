# Implementation Plan

1. 为 ScanEvent 摘要写失败的最小测试：ANSI、多行 nuclei banner/FTL、单行进度。
2. 实现纯摘要函数，并让 `storeProgress.Emit` 原样写日志、摘要写事件。
3. 删除未命中 Dameng 的 `progress.Emit`；保留 matched 事件。
4. 运行 app/store 的聚焦测试与相关 Web smoke；审查日志、artifact 和 DetectionCheck 不变。

## Risky Files

- `internal/app/progress.go`：同一接口服务日志和事件，必须分离而非全局静默。
- `internal/app/scan_target.go`：不得改变实际 Dameng 探测或检测状态。
