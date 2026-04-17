## Why

AgentForge 目前的日志体系里没有"人工操作审计链路"：

- `log_handler` 记录的是业务/执行日志（agent 步骤、scheduler 跑、dispatch 状态），面向运行可观测。
- `plugin_event_audit` 只覆盖插件生命周期事件。
- 没有表/接口专门回答"**谁**在**什么时间**对**哪个资源**做了**什么操作**"。

随着 Wave 1 的 `add-project-rbac` 引入了 `projectRole`，也就意味着每个写操作都隐含了"发起者、所属项目、所用角色"三项元数据。把这些元数据沉淀为可查询的审计事件流，是后续合规、排障、行为回溯的必需基座，也是 RBAC 自身是否被正确执行的 observable。

## What Changes

- 新增 `project_audit_events` 表与对应 repository/service。事件 schema：`id`、`project_id`、`actor_user_id`、`actor_project_role_at_time`、`action_id`、`resource_type`、`resource_id`、`payload_snapshot_json`、`system_initiated`、`configured_by_user_id`、`ip`、`user_agent`、`occurred_at`、`request_id`。
- 审计事件通过现有 `eventbus` 发射与消费，由一个独立的 `AuditSink` 订阅所有已声明的 ActionID 发生事件。写入失败（DB 不可达）**不阻塞主业务**，但必须升级为 warning log + metric，并在一个退避重试队列中补写。
- 事件源覆盖：项目 CRUD、member CRUD（含 projectRole 变更）、task create/update/delete/dispatch/assign/transition、workflow execute/cancel、team run start/retry/cancel、automation 手动触发、settings 变更、wiki page 写操作。所有这些入口复用 `add-project-rbac` 声明的 `ActionID` 枚举，使审计 action 命名与 RBAC 矩阵完全对齐。
- 新增查询 API：
  - `GET /projects/:pid/audit-events`：支持按 `actionId`, `actorUserId`, `resourceType`, `resourceId`, `occurredAt` 时间范围筛选，分页、游标查询。默认 `admin+` 才能读。
  - `GET /projects/:pid/audit-events/:eventId`：单事件详情含完整 payload snapshot。
- 新增前端简易审计视图：`/project/audit`（或挂在 `/settings` 下的 Audit Log 子页），分页列表 + 过滤 + 详情抽屉。不做图表、不做导出（保留 Wave 2 扩展空间）。
- 保留策略：**内测期永久保留**。不做 TTL、不做归档、不做分区；为未来扩展预留 schema 位（事件表单独建，便于未来迁移到冷存）。
- 破坏性：所有已声明 ActionID 的 write 入口都新增一次事件发射；现有业务路径不改语义，但写入延迟会增加可观测但无感的 overhead。无兼容层。

## Capabilities

### New Capabilities
- `project-audit-log`：定义审计事件模型、事件发射契约、查询 API 权限模型，以及与 `project-access-control` 的 ActionID 对齐。

## Impact

- 数据库迁移（新表）：`project_audit_events` 及相关索引（`project_id + occurred_at desc`，`project_id + action_id`，`project_id + actor_user_id`）。
- 后端：`src-go/internal/model/audit_event.go`（新）、`src-go/internal/repository/audit_event_repo.go`（新）、`src-go/internal/service/audit_service.go`（新）、`src-go/internal/handler/audit_handler.go`（新）、`src-go/internal/server/routes.go`（挂接新路由 + 订阅 eventbus）。
- 后端 eventbus：在现有 `internal/eventbus` 上新增 `AuditableEvent` payload（通过 publisher wrapper 或显式发射点）；RBAC middleware 在 allow 后发射事件，deny 时发射独立的 `rbac_denied` 审计事件。
- 前端：`lib/stores/audit-store.ts`（新）、`components/project/audit-log-panel.tsx`（新）、`app/(dashboard)/project/audit/page.tsx`（新，或挂到 settings 下）。
- 文档：API 参考补充、运维文档补充审计事件语义。

## Depends On

- `2026-04-17-add-project-rbac`：审计事件模型中的 `actor_project_role_at_time`、`action_id` 枚举与 RBAC 中间件引入同一枚举；先合入 RBAC change 后再合入 audit log，避免矩阵声明分裂。
