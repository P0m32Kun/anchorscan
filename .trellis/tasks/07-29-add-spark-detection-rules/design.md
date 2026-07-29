# Spark 检测规则设计

先取得当前 Nmap XML/httpx fixture 能稳定产生的 Spark service/product/tech/URL 证据，并以项目实际支持的外部 nuclei-templates 版本验证目标 tags。规则只匹配这些证据，不以 8080 端口猜测；匹配时 Target 为可用 URL，否则为 IP:port。

规则只能用 `nuclei_tags`/`exclude_tags`，遵守默认 `fuzz,dos` 排除；默认登录、爆破和高影响模板需显式排除，除非未来另获授权。此任务依赖默认 `template:` 契约移除完成。
