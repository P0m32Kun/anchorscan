# 02 — 更新实验室文档

**What to build:** 修正实验室清单和故障排查中的过期版本、compose、容器名和端口说明，统一指向当前外部实验室契约。

**Blocked by:** `01-archive-completed-plans.md`

**Status:** done

**Spec:** `docs/plans/archive/documentation-hygiene/spec.md`

- [x] `testing-lab-checklist.md` 使用 `$SHARED_LAB_DIR`（默认 `~/DEV/lab`）、`docker-compose.yml` 与 `lab-*` 容器名。
- [x] 移除 V1/V1.1/V1.2/V1.3 和硬编码版本号；以功能标题替代。
- [x] 删除废弃的 `--ports full` 写法，保留 `1-65535`。
- [x] `troubleshooting-lab.md` 使用同一实验室启动契约。
