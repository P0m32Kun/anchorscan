# 01 — 稳定 Nmap 存活探测

**What to build:** 防止 fast/normal/slow 的服务扫描参数影响 `-sn` 存活探测结果。

**Blocked by:** None — can start immediately.

**Status:** done

- [x] `RunScan` 回归测试证明 fast Nmap 参数不会进入 alive sweep。
- [x] profile Nmap 参数仍传给服务识别与 NSE。
- [x] 聚焦扫描测试通过。
