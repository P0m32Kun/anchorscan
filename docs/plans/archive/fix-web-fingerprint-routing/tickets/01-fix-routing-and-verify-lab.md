# 01 修复 Web 指纹路由并验证靶场

Status: done

Blocked by: none

Spec: `docs/plans/fix-web-fingerprint-routing/spec.md`

## 工作

- 为真实默认配置添加指纹路由和高危端口回归测试。
- 补全高危端口，修正具体 Web 产品规则与通用 HTTP 兜底的优先关系。
- 使用真实 Nmap、httpx、Nuclei 对普通 Web 和 Tomcat 本地靶场做验收。
- 运行全量测试、静态检查和代码审查。
