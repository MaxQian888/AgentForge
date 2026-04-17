## Why

`projectH.Delete` 当前是**硬删**：项目下的 members、tasks、runs、wiki、automation 全部级联丢失。实际使用中几乎没人敢点这个按钮——宁可留着一堆僵尸项目。真实需求是"停止对这个项目的任何写入，但保留数据供查阅/合规/后续复活"，这就是归档（archive）。

现有 project 模型已有 `status` 字段（在 `project-management-api-contracts` 里承认为 `active|paused|archived`），但 `archived` 状态并没有任何后端强制语义：

- archived 项目在 list 里和 active 混在一起；
- archived 项目的任何写操作仍能通过 RBAC（没有额外 gate）；
- agent/team run 在归档后仍继续跑；
- 从 archived 恢复的路径没定义。

这个 change 把 `archived` 变成一等的生命周期状态，明确"归档即冻结"，并补齐硬删 gate（先归档才能删）。

## What Changes

- 在 `projects` 表上新增 `archived_at TIMESTAMPTZ NULL`、`archived_by_user_id UUID NULL`；`status` 字段保持 `active|paused|archived` 三值，不再混入 UI 侧虚拟状态。
- 新增 `POST /projects/:pid/archive`（`owner` 独有）：置 `status=archived` + 写入 `archived_at/_by`；副作用：取消所有 in-flight team run / dispatch / automation schedule（按取消语义，不硬杀），关闭所有 pending 邀请（`status=revoked` 标注 reason=`project_archived`）。
- 新增 `POST /projects/:pid/unarchive`（`owner` 独有）：置 `status=active`、清空 `archived_at/_by`；不自动恢复被取消的 run，由用户手动重新 dispatch。
- `DELETE /projects/:id` 语义收紧：仅当 `status=archived` 时允许删除；对 `active|paused` 返回 `409 project_must_be_archived_before_delete`。保留 `?hard=true` 由 owner 触发硬删（仍要求先归档一次，防误删）。
- archived 项目 RBAC 额外 gate：除 `project.unarchive`、`project.delete`、`project.view`、`audit.read` 几个显式动作外，**所有写 ActionID** 对 archived 项目一律拒绝，错误码 `project_archived`。这层检查独立于 projectRole 之上——即 archived 项目连 owner 也不能改任务。
- archived 项目从 `GET /projects` 默认列表中**隐藏**；支持 `?status=archived` 或 `?includeArchived=true` 显式拉取。UI 在项目列表加一个"已归档"视图切换。
- 与 agent run 生命周期：归档触发时，向所有 active run 发 cancel 信号；scheduler 定时任务跳过 archived 项目；automation engine 对 archived 项目的触发全部 no-op 并记 audit。
- 破坏性：`DELETE /projects/:id` 原有直接硬删的行为被移除；调用方必须先 archive 后 delete。

## Capabilities

### New Capabilities
- `project-lifecycle-archival`：定义归档/恢复/删除的状态机、archived 项目的只读约束、list 可见性规则、对 in-flight agent 动作的级联效应。

### Modified Capabilities
- `project-management-api-contracts`：`GET /projects` 默认过滤 `archived`；`DELETE /projects/:id` 必须先归档；新增 `POST archive/unarchive` 端点。
- `project-access-control`：新增 ActionID `project.archive|project.unarchive`（`owner`）；所有现有写 ActionID 在 archived 项目上被第二层中间件拒绝。

## Impact

- DB 迁移：`projects.archived_at`、`projects.archived_by_user_id` 新列 + 索引 `(status, archived_at)`；回填 `status='active'` for 未归档记录。
- 后端：`internal/handler/project_handler.go`（新增 archive/unarchive handler、delete 语义收紧）、`internal/repository/project_repo.go`（list 过滤参数）、`internal/middleware/archived_guard.go`（新增，挂在 RBAC 之后）、`internal/service/project_lifecycle_service.go`（新，封装归档副作用：run 取消、invite 撤销、automation 静默）、`internal/server/routes.go` 挂接。
- Scheduler/automation：`internal/scheduler/**` 与 `internal/service/automation_engine_service.go` 加入 archived 项目跳过检查。
- Team/dispatch：`internal/service/team_service.go`、`dispatch_preflight.go`、`agent_service.go` 入口侧重入归档检查（防止在归档切换瞬间还能挤进一个 run）。
- 前端：`lib/stores/project-store.ts`（list 查询加 `includeArchived`）、`components/project/project-card.tsx`（归档徽章）、`app/(dashboard)/projects/page.tsx`（archived filter 切换）、`components/project/edit-project-dialog.tsx`（archive 按钮，owner 可见）、国际化补词。

## Depends On

- `2026-04-17-add-project-rbac`：归档/恢复是 `owner` 独有 ActionID；archived-guard 中间件读取的是 Wave 1 的 ActionID 枚举；末位 owner 保护在归档前仍适用。
