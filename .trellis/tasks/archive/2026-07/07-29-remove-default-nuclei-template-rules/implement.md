# 执行计划

1. 写 RED 配置测试：旧 `template:` 返回明确迁移错误；tags 规则仍可加载。
2. 删除默认规则 Template/MatchResult 字段、路径解析与 scanTarget `-t` 分支；保留单工具路径。
3. 更新 tags/exclude-tags、PrepareScan、doctor、单工具和 package 归档测试。
4. 运行 `make test`、`go vet ./...`、`make package VERSION=<test>` 与 `make pr-check`。
