## 1. 稳定性门槛与迁移基线

- [x] 1.1 确认当前 UI 工作已经稳定，且 `internal/web/templates/`、`internal/web/static/`、`internal/report/` 没有重叠的未完成改动；条件不满足时停止实施
- [x] 1.2 在修改前运行 `go test ./...`、`node --test internal/web/static/app.test.mjs` 和 `make package`，确认现有基线通过
- [x] 1.3 使用固定 `ScanReport` 在旧实现上记录 HTML 输出 SHA-256，并保存代表性 Web 页面在固定浏览器与视口下的视觉及关键 DOM 基线

## 2. 静态报告模板迁移

- [x] 2.1 添加固定报告输入的字节哈希回归测试，使模板内容或首尾空白变化能够被检测
- [x] 2.2 将 `htmlTemplate` 正文不做格式化地迁移到 `internal/report/templates/report.html`，并用私有 `embed.FS`、`ParseFS` 和 `ExecuteTemplate` 保持 `WriteHTML` 边界
- [x] 2.3 验证固定输入的 HTML 哈希不变、无外部模板文件时仍可生成报告，并确认现有 `WriteHTML` 调用方无需修改

## 3. Web 表现资源按职责整理

- [x] 3.1 基于稳定 UI 快照确认具有独立页面或行为、独立验证边界且无隐含共享状态的抽取清单；不满足条件的代码明确保留原位
- [x] 3.2 将确认独立的 JavaScript 行为抽取为平级叶子资源，保留 `/static/app.js` 入口，并由对应模板按原执行顺序加载
- [x] 3.3 让 Node 测试布局镜像实际 JavaScript 职责，保留 `app.test.mjs` 测试入口，并仅在至少两个测试文件复用时提取最小测试 helper
- [x] 3.4 仅将与确认职责对应的完整 CSS 规则块抽取为平级叶子资源，保留 `/static/style.css` 入口以及原选择器、声明和级联顺序
- [x] 3.5 更新相关现有模板的叶子资源引用，并验证路由、表单字段、模板数据、静态资源入口和关键 DOM 契约未改变

## 4. 兼容性验证

- [x] 4.1 运行 `go test ./...`，确认报告、Web Handler、模板渲染及其他 Go 行为全部通过
- [x] 4.2 运行 `node --test internal/web/static/app.test.mjs` 和 `make package`，确认脚本加载顺序、交互回归与单二进制打包通过
- [x] 4.3 在固定浏览器与视口下执行相关 Web 冒烟流程并比较基线，确认页面视觉、DOM、交互、路由、HTTP 状态以及 JSON/HTML 输出没有预期外差异
- [x] 4.4 审查最终依赖和文件边界，确认未增加框架、打包器或第三方依赖，且每个新增文件都对应一个真实修改原因
