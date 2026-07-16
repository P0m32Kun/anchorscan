# 修复扫描、配置、知识库与 Windows 兼容性

## Problem Statement

- `fast` profile 的 Nmap 服务扫描参数被复用于存活探测，导致存活 IP 结果少于其他 profile。
- 配置示例和自动初始化 profile 已落后于当前 `config/default.yaml` 的实测参数。
- 漏洞知识库长文本会撑破页面，导航缺少图标，条目尾部的 Markdown 分隔线被当成正文展示。
- 工具预检只接受可直接 `os.Stat` 且带 Unix 执行位的路径，不能正确支持 PATH 工具名和 Windows 可执行文件。

## Solution

保持现有配置结构和工具接口，只在现有 seam 修复：存活探测不继承服务扫描参数；内置 profile 与示例同步到当前实测值；知识库使用局部模板/CSS 自适应换行并复用现有 SVG 导航风格；解析器忽略条目尾部分隔线；预检使用标准库 PATH 解析规则。

## User Stories

1. 使用任意扫描 profile 时，Nmap 存活探测应使用相同的稳定参数，profile 只调节后续服务扫描。
2. 新用户自动生成的配置和 `config/default.yaml.example` 应使用当前实测 profile 参数。
3. 知识库描述、命令和修复建议应始终限制在内容面板内并自动换行。
4. 知识库导航应与其他模块一样显示图标，页面不得展示手册条目分隔线。
5. Windows 用户可使用 PATH 中的工具名或有效可执行文件路径通过预检并被 runner 调用。

## Testing Decisions

以下 seam 已由本次请求确认：

1. `RunScan` 命令记录：fast profile 的 Nmap 参数只出现在服务扫描，不出现在 `-sn` 存活探测。
2. `config.Init` 与 `config/default.yaml.example`：两者 profile 参数一致，并匹配当前实测值。
3. 知识库 HTTP 页面与静态 CSS：响应包含知识库图标和局部换行样式钩子，长文本不依赖截断。
4. 知识库 `Catalog` 解析：尾部 `---` 不进入 remediation。
5. `preflight.Run`：PATH 中的工具名可通过；缺失工具仍阻断。

## Out of Scope

- 新增独立的 `nmap_alive_args` 配置项。
- 引入图标库、前端框架或 JavaScript 文本截断。
- 为 Windows 建立专用 runner 或 shell 包装层。
- 修改用户现有 `config/default.yaml` 中的绝对工具路径和知识库路径。

## Further Notes

- Review fixed point: `64e3abc3faeb3243e1b3fa563b7216e81abe37cc`。
- 按 `tickets/` 的编号逐个实施；前一 ticket 完成后下一 ticket 才成为 frontier。
