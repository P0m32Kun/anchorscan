# 仅对已确认 TLS 服务路由 Nuclei SSL 标签

## Goal

让 AnchorScan 在已被 Nmap 确认的 TLS/HTTPS 服务上，把 `ssl` 作为额外 Nuclei tag 传入，从而先启用现有 TLS 模板的检测路径；不把普通 HTTP、`unknown` 或 `tcpwrapped` 服务送入该路径。

## Confirmed Facts

- 当前 `TagRule` 只支持 `service`、`product`、`tech`、`nuclei_tags`、`exclude_tags` 和 `target`，没有 TLS tunnel/SNI 条件（`internal/vuln/nuclei_tags.go:16-24`）。
- `MatchNucleiTags` 命中首条 service/product/tech 规则后立即返回（`internal/vuln/nuclei_tags.go:39-56`）；因此把 `ssl` 追加到 `http-generic` 会作用于普通 HTTP 与 HTTPS，不能满足本任务限定范围。
- `fingerprint.Classify` 在 Nmap tunnel 为 `ssl`、服务名含 `https` 或 `ssl/http` 时生成 HTTPS URL（`internal/fingerprint/classify.go:10-33`）。
- 当前全局 Nuclei 排除 `fuzz,dos`；项目不锁定外部 templates。TLS 研究证实外部模板可能经 `misconfig` 条件路径被选中，但没有稳定默认模板清单。

## Requirements

- 只要 Nmap 指纹满足以下任一 TLS 证据，就视为 TLS：`tunnel: ssl`、服务名含 `https` 或服务名含 `ssl/http`。该定义与现有 HTTPS URL 分类保持一致。
- 仅当既有 Nuclei 服务/产品/技术栈规则已经命中时，才在其原有 tags 上追加 `ssl`；没有既有规则的 TLS 服务也不得因此新增 Nuclei 执行。
- 普通 HTTP、空服务、`unknown` 或 `tcpwrapped` 不得因本变更获得 `ssl` tag。已确认 TLS 的非 Web 服务在既有规则命中时同样可追加该 tag。
- 保留既有 `fuzz,dos` 全局排除与现有服务规则的额外排除。
- 不增加单独的 TLS 网络扫描阶段、NSE 脚本或主动协议探针；Nuclei 仍只在原有 Nuclei 阶段运行。
- 输出和 DetectionCheck 必须能够显示实际 tags，回归测试必须验证正反候选和排除标签。

## Out of Scope

- 锁定、下载或更新 Nuclei templates。
- 证书审计、密码套件枚举、JARM、Heartbleed 或通用 TLS CVE 扫描。
- 变更 Nuclei 的并发、重试、超时或全局排除策略。
- 将 `ssl` tag 加到全部 HTTP 服务或 unknown 服务。

## Acceptance Criteria

- [x] 已确认 TLS/HTTPS 指纹命中既有 Nuclei 规则时，将在原有 tags 上追加 `ssl`。
- [x] 普通 HTTP、unknown 和 tcpwrapped 不获得 `ssl` tag，也不触发新增 Nuclei 路由；已确认 TLS 的非 Web 指纹在既有规则命中时可以获得该 tag。
- [x] 无既有 Nuclei 规则的 TLS 指纹保持 `skipped/no_matching_rule`，不因本变更新增执行。
- [x] 现有 tags、目标 URL/hostport 与 `fuzz,dos` 及规则级排除标签保持不变，除已确认 TLS 的 `ssl` 追加外。
- [x] 单元/集成 seam 测试覆盖三种 TLS 正例、HTTP/unknown/tcpwrapped 反例、无规则 TLS 反例和输出 tags。
- [x] 配置和运行时契约记录 TLS 条件、外部 templates 未锁定的限制与回滚方式。

## Key Decision

- TLS 资格接受 Nmap 的 `tunnel: ssl`、服务名含 `https` 或服务名含 `ssl/http` 三种证据；与现有 HTTPS 分类一致。已确认 TLS 的非 Web 服务在既有规则命中时亦适用。
- 不新增 TLS 专用规则。只有既有 Nuclei 路由先命中时才追加 `ssl`，以避免扩大扫描对象集。
