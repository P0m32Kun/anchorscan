## Context

Web 扫描当前先对项目端口排除执行纯数字解析，再解析 `top1000` 预设，因此带排除端口的项目模板会失败。报告路径由项目决定，过程文件路径由独立 artifact root 决定；run 已保存 `project_id`，但历史模板没有展示。NSE 与 nuclei 配置还对多种服务重复匹配。

用户已确认在文件数超过 hotfix 提示阈值后继续 hotfix，因为本次不改数据库 schema、public API 或架构。

## Goals / Non-Goals

**Goals:**

- 统一端口解析顺序和支持格式。
- 每次 Web 扫描只使用一个 run 目录保存报告与过程文件。
- 复用现有 `project_id` 在历史页面展示项目。
- 让 NSE 负责非 Web 协议检查，nuclei 负责 Web 模板检查。

**Non-Goals:**

- 不修改数据库 schema。
- 不新增验证引擎、依赖或路由抽象层。
- 不迁移历史 run 文件。

## Decisions

- 在 `internal/ports` 统一校验公开端口输入，只允许 `top1000`、`full` 和数字 CSV；高危端口文件通过独立 preset 读取函数提供给插入按钮。
- Web 扫描先解析端口，再应用项目排除；这样所有后续逻辑只处理数字 CSV。
- artifact root 继续表示 run 根目录，报告写入同一个 `<root>/<run-id>/report.json`；默认 root 按项目使用现有 managed runs 目录。
- 历史页面使用现有项目列表构造 ID→名称映射，不增加 SQL JOIN 或新模型。
- 配置层直接删除重复路由：Redis/MySQL/SMB/SSH 使用 NSE，HTTP/Tomcat/Nginx 使用 nuclei。

## Risks / Trade-offs

- 旧的 `top100`、`highrisk` 和范围输入将返回明确校验错误；高危按钮会插入等价 CSV。
- 历史报告与过程文件不自动搬迁；仅新扫描使用统一目录。
