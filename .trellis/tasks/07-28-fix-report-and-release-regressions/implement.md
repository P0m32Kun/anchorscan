# 实施计划

## Fixed point

- 实施启动前记录 `main` 当前提交作为 review fixed point。
- 不提交或改写工作区中与本任务无关的既有改动。

## 1. 建立红色回归信号

1. 在 `internal/web` 增加同一区两个 runs 的正向验证 HTTP 契约回归；在 `scripts/web-smoke.mjs` 通过真实验证台创建 verification 并上传截图，先证明当前 Vue payload 会得到 `zone_id is not part of this project`。
2. 在 `internal/report` 增加同一区跨 run、同漏洞键、不同资产的聚合断言；确认现有行为并锁定不变量。
3. 在 `internal/report/docx_context_test.go` 增加同一区两个 sessions 的分区级多值 context 期望，先对当前 session 型 context 失败。
4. 在 `tools/docx-render/test_render_docx.py` 增加双 run 分区 fixture 渲染断言，先证明当前模板缺少分区级多值槽位。
5. 增加合成 `v9.8.7` 的构建/运行断言，先证明当前二进制仍输出硬编码版本。

聚焦命令：

```bash
go test ./internal/web -run 'Test.*Verification.*SameZone.*Evidence' -count=1
go test ./internal/report -run 'Test.*AcrossRuns|TestBuildDocxContext.*Multiple' -count=1
uv run --project tools/docx-render python -m unittest tools/docx-render/test_render_docx.py
make build VERSION=v9.8.7
./dist/anchorscan version
npm run test:web
```

## 2. 修复 verification payload

1. 为正向 verification 新建与更新共用一个 snake_case 请求序列化函数或明确对象。
2. 保持后端项目分区校验不变。
3. 运行正向创建、未知分区拒绝、evidence 上传的聚焦测试。

## 3. 锁定同区漏洞聚合

1. 补充/扩展 `report.BuildProjectReport` 测试，覆盖两个同区 runs 和一个异区 run。
2. 若现有实现已满足，不改生产聚合代码；只有测试暴露偏差时才做最小修复。
3. 验证资产、来源 run、finding source 的去重和分区隔离。

## 4. 改为 DOCX 分区级多值槽位

1. 在 `BuildDocxContext` 聚合唯一接入点、测试设备 IP、目标网段等文本。
2. 更新 JSON fixture 和 Python 渲染测试。
3. 用受版本管理的模板制备脚本或最小 OOXML/python-docx 编辑更新正式模板；同步 `prototype.py`，避免生成源与正式模板漂移。
4. 更新 `check_structure.py`：要求分区级槽位，拒绝旧 session 循环。
5. 运行 `make docx-test` 和结构检查。
6. 生成双 run DOCX，用文档渲染器输出所有页面 PNG，逐页检查裁切、重叠、字体、段落与分页；如有缺陷则修正并重渲染。

## 5. 自动注入发布版本

1. 将版本字段改为 linker 可覆盖变量。
2. 在 Makefile 组合版本注入参数与调用方 `LDFLAGS`，标准化前导 `v`。
3. 在 release workflow 增加宿主机二进制版本断言。
4. 用 `VERSION=v9.8.7` 构建并验证 CLI；让 Web smoke 启动同一产物并断言 footer。
5. 保持归档命名和 prerelease 检测行为不变。

## 6. 完整验证与审查

```bash
go test ./...
node --test internal/web/static/*.test.mjs
make docx-test
npm run build:web
make package VERSION=v9.8.7
make web-smoke
```

然后：

1. 运行 Trellis `trellis-check`，检查 spec、lint/type-check、测试和跨层数据流。
2. 以启动时 fixed point 做 Standards / Spec 双轴 code review。
3. 修复 blocker/major 发现并重新运行受影响测试。
4. 若形成可复用的非显然契约，将“Zone 是聚合边界、Run 只是来源”和“tag 必须 linker 注入”写入项目 spec。

## 回滚点

- 切片 A：`Workbench.vue` 与 verification 测试。
- 切片 B：DOCX context、fixture、模板、制备/结构脚本与测试，必须整体回滚。
- 切片 C：`internal/version`、Makefile、release workflow 与版本测试。
