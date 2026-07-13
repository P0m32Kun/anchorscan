# web-presentation-modularity Specification

## Purpose
TBD - created by archiving change modularize-web-presentation. Update Purpose after archive.
## Requirements
### Requirement: 表现层源码按独立修改原因组织
系统 MUST 仅在模板、JavaScript、CSS 或 Node 测试存在可独立描述和验证的修改原因时拆分对应文件，并 MUST 让测试边界与被拆分的职责一致；系统 MUST NOT 仅因文件行数拆分源码。

#### Scenario: 整理具有独立职责的前端行为
- **WHEN** 一个前端行为可以独立于其他页面行为进行修改和验证
- **THEN** 该行为及其最小必要测试被组织到职责明确的资源中，修改它不要求同步编辑无关职责的资源

#### Scenario: 保留没有独立边界的代码
- **WHEN** 一段模板、样式或脚本没有独立的修改原因或验证边界
- **THEN** 系统保留其现有归属，不为缩短文件而创建额外层次或文件

### Requirement: Web 表现契约保持兼容
表现层整理后，系统 MUST 保持现有 Web 路由、HTTP 状态、表单字段、模板数据、静态资源入口、DOM 标识与数据属性、视觉结果和用户交互语义不变，并 MUST 继续通过现有浏览器行为对应的 Node 测试。

#### Scenario: 访问现有 Web 页面
- **WHEN** 用户以与变更前相同的请求访问任一现有 Web 页面
- **THEN** 系统返回相同语义的页面结构和视觉结果，原有控件、表单与交互继续工作

#### Scenario: 加载现有静态资源入口
- **WHEN** 页面或测试加载现有 JavaScript 与 CSS 资源入口
- **THEN** 入口仍可直接使用且无需新增打包、转译或运行时依赖

### Requirement: 静态 HTML 报告使用内嵌模板
系统 MUST 使用 Go 标准库 `embed.FS` 将静态 HTML 报告模板编译进二进制，并 MUST 保持 `report.WriteHTML` 的调用契约以及相同 `ScanReport` 输入生成的 HTML 字节不变；JSON 报告不受此变更影响。

#### Scenario: 在没有外部模板文件的环境生成报告
- **WHEN** 已构建的 AnchorScan 二进制在运行目录中不存在报告模板文件时调用 `report.WriteHTML`
- **THEN** 系统仍可使用编译进二进制的模板生成完整 HTML 报告

#### Scenario: 比较迁移前后的报告
- **WHEN** 使用同一份固定 `ScanReport` 数据分别通过迁移前后的实现生成 HTML
- **THEN** 两份 HTML 输出逐字节相同，且现有调用方不需要修改调用参数或处理方式

