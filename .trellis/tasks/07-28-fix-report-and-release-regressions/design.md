# 技术设计

## 边界与不变量

1. `ZoneID` 是验证台聚合和正式报告分组的业务边界；`RunID` 只用于追溯来源。
2. 正向漏洞候选以“网络分区 + 漏洞键”聚合。同一区不同 run 的资产合并，不同区不得合并。
3. DOCX 每个网络分区只输出一个章节。分区内的接入信息从所有 included、已完成或部分完成的 runs 聚合，不把 run 映射为子章节。
4. CLI 与 Web 只消费 `internal/version.Version`；发布 tag 通过链接参数写入这一唯一来源。

## 1. Verification 请求契约

`Workbench.vue` 的新建和更新路径都显式构造 snake_case JSON：

- `zone_id`
- `vulnerability_key`
- `outcome`
- `title`
- `severity`
- `description`
- `remediation`
- `notes`
- `included`
- `position`
- `assets`
- `sources`

不再直接 `JSON.stringify` 内部 PascalCase view model。后端继续验证 `zone_id` 属于 URL 中的项目，避免放宽真正的跨项目分区错误。

回归 seam 分两层：

- `internal/web` HTTP handler 测试锁定 snake_case 请求契约、分区归属和 evidence 保存；
- 现有 Playwright `scripts/web-smoke.mjs` 通过真实验证台对话框创建 verification 并上传内存 PNG，覆盖 Vue 序列化、HTTP 创建和 evidence 上传的完整用户路径。

## 2. 同区跨 Run 漏洞聚合

保留 `report.BuildProjectReport` 的现有聚合边界，并补精确回归：

- 两个 included runs 的 `ZoneID` 相同；
- 漏洞键相同、资产 IP 不同；
- 期望单一 `ProjectVulnerabilityCandidate`；
- `Assets`、`SourceRuns` 与 `Sources` 同时包含两个 run；
- 另加不同 `ZoneID` 的同漏洞断言，保证不跨区合并。

不新增数据库表、迁移或历史数据修复。

## 3. DOCX 分区级接入信息

`ProjectDeliverable` 可继续保留 run 级 `Sessions`，供其他适配器使用；`BuildDocxContext` 在 DOCX 边界把 session 数据投影为分区级多值文本：

- `access_points_text`
- `tester_ips_text`
- `targets_text`
- 如现有报告需要，保留聚合后的 `exclusions_text` 与 `notes_text`

聚合规则：

- 只读取该分区已进入 deliverable 的 sessions；
- trim 空白；
- 按 session 已有稳定顺序保留首次出现顺序；
- 去重但不重排为字典序，避免破坏扫描录入顺序；
- 多值按换行显示，模板只保留一组分区级占位符。

正式模板删除 `for session in network_zone.sessions` 的重复块，改为网络分区对象上的多值占位符。`check_structure.py` 同步验证新槽位存在、旧 session 循环不存在，且 Zone 循环仍只包围一个分区章节。

fixture 在同一个网络分区放入两个 run 的接入信息。Python 测试检查渲染后 OOXML 中两个接入点、两个测试设备 IP、两个网段各出现一次，并且网络分区标题只出现一次。最终用正式渲染器生成全页 PNG 做视觉检查。

## 4. 发布版本注入

将 `internal/version.Version` 从常量改为可由 Go linker `-X` 覆盖的包变量，并保留开发构建默认值。

Makefile：

- 从 `VERSION` 去掉一个前导 `v`，形成显示版本；
- `build` 与 `package` 始终追加 `-X github.com/P0m32Kun/anchorscan/internal/version.Version=<display-version>`；
- 保留调用方传入的其他 `LDFLAGS`（例如 `-s -w`），不覆盖它们；
- 归档名称仍使用原始 tag，避免无意改变现有资产命名。

release workflow 继续传入 `GITHUB_REF_NAME`，并增加在本机可执行构建上的版本断言；交叉编译归档仍走相同 Makefile 路径。测试使用合成 tag `v9.8.7`：执行宿主机 CLI 验证输出，并由使用同一构建产物的 Web smoke 断言 footer 为 `v9.8.7`。

## 兼容性与回滚

- SQLite schema 与持久化数据不变。
- 已创建的 verification 不需要迁移。
- DOCX context 是内部 sidecar 契约，Go context、fixture、模板、结构检查和打包模板必须同批更新。
- 回滚可按三个切片分别撤销：verification payload、DOCX context/template、版本注入；三者没有运行时耦合。

## 风险

- DOCX 模板 OOXML 直接修改可能破坏 run 样式或段落控制标签；必须以结构检查和全页 PNG 渲染双重验证。
- `LDFLAGS` 拼接与 shell 引号错误可能导致发布构建静默使用默认版本；必须运行真实构建并执行生成的宿主机二进制。
- 当前工作区存在用户未提交改动；实现只触碰本任务文件，任何重叠先检查 diff，绝不覆盖用户修改。
