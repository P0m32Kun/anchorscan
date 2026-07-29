# 默认 Nuclei 模板规则移除设计

## 边界

`vuln.TagRule`、`MatchResult`、`config.LoadTagRules` 与 `scanTarget` 组成默认规则的完整路径。删除其中的 Template 字段、相对路径解析、模板条件分支和仅服务于该分支的 artifact/provenance 语义；`tools.RunNucleiTemplate` 保留给显式单工具命令。

## 兼容性

YAML 解码改为严格字段校验；默认 `service-tags.yaml` 含 `template:` 必须返回迁移错误，说明使用 `nuclei_tags`。tags 路径仍使用 `fuzz,dos` 默认排除与规则 `exclude_tags`。不改历史 artifact/DetectionCheck。

## 风险与回滚

唯一有意破坏是旧默认配置加载失败，避免静默漏检；以可编辑配置迁移回滚，不恢复仓库模板。
