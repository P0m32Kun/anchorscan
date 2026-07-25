# 02 — 创建归属 Zone 的 Scan Runs

**What to build:** 把目标、端口、排除项、档位和接入信息放到扫描创建页及 Run 快照，并要求每个 Web Run 选择 Zone。

**Blocked by:** 01 — 建立任务型 Project 与 Network Zones。

**Status:** done

- [x] scan_runs 兼容新增 zone_id、kind、label、access_point、tester_ip、notes、include_in_report。
- [x] 扫描页把 Zone、目标、端口和档位作为主要字段，不再回退 Project 默认值。
- [x] 重新运行继承并展示原 Zone/参数，提交后创建新 Run。
- [x] 完整 completed/completed_with_errors Scan 默认纳入，Tool/failed/running 默认不纳入。
- [x] Project 页按 Zone 展示多个 Runs，并可切换纳入状态。
- [x] CLI 独立 Run 与 ADR-0001 的摄入结构不受影响。
