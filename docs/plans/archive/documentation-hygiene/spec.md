# 文档卫生

## 状态

已完成。

## 问题

`docs/plans/` 中保留了 15 个所有 ticket 均已完成的计划目录，和可执行计划混在一起。实验室运行文档仍使用过期的 V1/v1.3 叙述、1.5.1 版本号、已不存在的仓库内 compose 文件、旧容器名前缀及废弃端口写法。

## 目标

1. `docs/plans/` 只保留进行中或待执行的计划；完成计划整体归入 `docs/plans/archive/`。
2. 实验室清单和故障排查以当前外部实验室契约为准：`$SHARED_LAB_DIR`，默认 `~/DEV/lab`，compose 文件为 `docker-compose.yml`，容器名为 `lab-*`。
3. 文档入口明确区分当前基线、操作文档、ADR、进行中计划和已归档计划。

## 非目标

- 不合并 checklist、结果模板和故障排查：它们分别服务执行、记录和按症状检索。
- 不改变扫描器、E2E 或外部实验室的运行行为。
- 不重写完成计划的历史内容。

## 验收

- 15 个完成计划目录均在 `docs/plans/archive/`，不再位于 `docs/plans/` 顶层。
- `deepen-reportview` 与 `deepen-targetscan` 使用既有的中文 `状态：done` 格式，保留原状后归档。
- 实验室文档不再引用 `docker-compose.lab.yml`、`anchorscan-lab-*`、`--ports full` 或硬编码版本 `1.5.1`。
- `docs/project-status.md` 提供文档导航；所有 Markdown 相对链接有效。
