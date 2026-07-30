# TLS Nuclei tag 路由设计

## Boundary

本变更不创建新的扫描阶段，也不通过端口、unknown、空服务或 tcpwrapped 触发 Nuclei。它只改变已经被 `MatchNucleiTags` 选中、将要执行的 Nuclei 调用：若该指纹拥有明确 TLS 证据，则在复制后的 tags 列表末尾追加 `ssl`。

## TLS Predicate

在 `internal/fingerprint` 暴露单一纯函数，复用现有分类语义：

- `Tunnel`（大小写无关）等于 `ssl`；或
- `Service`（大小写无关）包含 `https`；或
- `Service` 包含 `ssl/http`。

`Classify` 使用同一函数决定 Web URL 的 HTTPS scheme；Nuclei tag 路由也调用该函数，避免两处 TLS 定义漂移。

## Data Flow

1. Nmap XML 产生 `ServiceFingerprint`。
2. `fingerprint.Classify` 填充归一化、Web 和 URL 信息。
3. `MatchNucleiTags` 先按当前 service/product/tech 逻辑找到首条规则；无匹配仍返回零值，由编排记录 `skipped/no_matching_rule`。
4. 仅在已有规则命中且 `fingerprint.IsTLS` 为真时，向新分配的 `MatchResult.Tags` 追加一次 `ssl`。
5. `RunNuclei` 继续传入 tags，以及既有默认 `fuzz,dos` 和规则级 exclude tags；URL/hostport 的选择不变。

## Compatibility and Rollback

- 非 TLS 或无规则候选的结果完全不变。
- 已确认 TLS 的 Web 与非 Web 候选在既有规则命中后只多一个 `ssl` tag；若原规则已含该 tag 则不重复追加，且不得改变 target 或 exclude tags。
- 外部 templates 仍非项目锁定输入；此改动只使工具接收额外选择条件，不承诺哪些模板存在或会运行。
- 回滚为移除追加逻辑与未来的 DetectionCheck tag-detail 写入；无需配置、格式或数据迁移，且历史 DetectionCheck 仍保留其原始运行事实。

## Risk Controls

- 保留全局 `fuzz,dos` 排除与规则级排除。
- 不引入 `code`、`default-login`、`brute` 或 `exploit` 标签，也不修改 timeout/concurrency/retry。
- 测试只通过匹配函数与现有 fake runner 验证命令参数，不访问真实目标。
