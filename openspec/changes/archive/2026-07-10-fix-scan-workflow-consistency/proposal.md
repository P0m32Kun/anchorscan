## Why

项目模板扫描会在端口排除阶段把 `top1000` 当作数字端口解析，导致扫描尚未启动即报 `invalid port: top1000`。同时，扫描报告、过程文件、项目关联展示和验证器路由存在不一致，需要一次收敛为可预测的现有行为。

## What Changes

- 端口输入仅接受 `top1000`、`full` 或逗号分隔的数字端口；移除 `top100`、`highrisk` 和自定义范围输入，高危端口按钮继续插入实际 CSV。
- 先解析端口预设再应用项目排除端口，修复项目模板使用 `top1000` 时的报错。
- 将扫描报告与过程文件写入同一个 run 目录，并在扫描历史中显示关联项目。
- 明确验证器职责：NSE 用于非 Web 协议服务检查，nuclei 用于 Web 服务模板检查，删除重复路由。

## Capabilities

### New Capabilities

- `scan-workflow-consistency`: 记录并验证既有扫描入口、run 存储、项目关联和验证器路由的收敛行为；不引入新的产品模块。

### Modified Capabilities

- 无现有 OpenSpec capability；本次仅修复和收敛既有扫描行为。

## Impact

- 影响端口解析、Web 扫描创建、run 目录生成、扫描历史模板和 NSE/nuclei 配置。
- 不新增依赖、数据库字段或 public API。
