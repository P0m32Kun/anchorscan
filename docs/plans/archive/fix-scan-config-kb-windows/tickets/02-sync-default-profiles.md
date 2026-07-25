# 02 — 同步默认 profile

**What to build:** 让自动初始化配置和 `config/default.yaml.example` 使用当前 `config/default.yaml` 的实测 profile 参数。

**Blocked by:** 01-stable-nmap-alive-sweep

**Status:** done

- [x] slow、normal、fast 的 host workers 和四类工具参数全部一致。
- [x] 保留 example 中通用工具名和空知识库路径。
- [x] 配置初始化回归测试通过。
