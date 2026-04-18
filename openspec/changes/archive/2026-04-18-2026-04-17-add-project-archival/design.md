## Context

AgentForge 的 `projectH.Delete` 是硬删，没人敢点；而 `status='archived'` 这个状态既存在于现有合同里（`project-management-api-contracts`）又在后端基本没有效果。结果是：

- 运营团队实际在用"改名加前缀 `[归档] xxx`"+ 口头约定"不要再动"来维护项目生命周期，这显然不可扩展；
- dashboard/listing/search 继续把这些伪归档项目和活跃项目一起算总数、算 readiness；
- 在 Wave 1 RBAC 之后，viewer/editor/admin 仍然可以在这些"应该冻结"的项目里发 dispatch，RBAC 本身无法阻止（因为它基于角色而非生命周期）。

这个 change 把 archived 从"一个被忽视的枚举值"升级为"冻结写入的生命周期状态"，并把 DELETE 从"一键摧毁"改成"归档后才能删"。

## Goals / Non-Goals

**Goals:**
- 给项目生命周期建立清晰状态机：`active → (pause → active) | (archive → unarchive → active) | (archive → delete)`，无隐式转移。
- archived 项目写入被后端**生命周期 guard** 拒绝，独立于 RBAC；即使 owner 也必须先 unarchive 才能改。
- DELETE 不再是一键摧毁路径——必须先归档。
- archived 项目从默认 list 隐藏；显式请求才返回。
- in-flight agent/team run、pending 邀请、scheduler/automation 在归档时级联停止/禁用，防止"归档后后台还在跑"。

**Non-Goals:**
- 不做数据导出（archived 项目可读，导出功能留 follow-up）。
- 不做软删除与硬删除的分级 retention（删除即物理删除；归档是软冻结）。
- 不做按时间自动归档（比如"90 天无活动自动归档"）——这是行为建议而非生命周期语义。
- 不改 `status='paused'` 的语义（paused 与 archived 是不同概念：paused 暂停但可写，archived 冻结不可写）。

## Decisions

### 1. 生命周期三态 + 显式转移

```
active  ◄────────────────── pause/resume ─────► paused
  │                                                │
  │  archive  (owner only)                         │  archive
  ▼                                                ▼
archived  ◄── unarchive (owner only) ── active ◄── resume
  │
  ▼  delete (owner only, requires archived)
(gone)
```

- 所有转移都要显式 API 调用，没有隐式或自动转移。
- `paused` 状态保持原语义（暂停不禁止写），本 change 不扩展。
- archive/unarchive/delete 三个动作都是 `owner` 独占。

**Why this**：显式转移 = 可审计可追溯；归档动作本身是 governance 决策，应只有 owner 能下令。

**Alternative rejected – admin 也可归档**：admin 能改配置但不能"关停项目"的分界更清晰；未来若证明 owner 过窄再放开。

### 2. Archived-guard middleware 独立于 RBAC

- 在 `projectGroup` 的中间件链上，RBAC 之后加一层 `archived_guard`。
- 读取项目 `status`；若为 `archived`，仅放行显式 whitelist 的 ActionID：`project.view`、`project.unarchive`、`project.delete`、`audit.read`、`invitation.view`（历史查看）、`member.view`、`task.view`、`wiki.view`、`workflow.view`，以及任何以 `.view` 结尾的只读 ActionID。
- 其余一律返回 `409 project_archived`。错误体附带 `archivedAt`、`archivedByUserId` 便于 UI 提示。

**Why this**：把"生命周期约束"和"角色约束"分成两层中间件，避免某一层膨胀；UI 拿到 `project_archived` 可以直接切到只读模式。

**Alternative rejected – 在 RBAC 矩阵里为 archived 再列一套 MinRole**：会让矩阵维度从 2D(action×role) 变 3D(action×role×status)，可读性和测试成本都爆炸。

### 3. 归档的级联副作用

归档触发时，`project_lifecycle_service.Archive(ctx, projectID, ownerUserID)` 在事务内：

1. 把 `projects.status` 置 `archived`，写 `archived_at = now()`、`archived_by_user_id = ownerUserID`；
2. 扫描该项目所有 `status='pending'` 的 invitations，批量置 `revoked` + `revoke_reason='project_archived'`；
3. 扫描该项目所有 `active | queued` 的 team runs、dispatch attempts、workflow executions，批量置 `cancelled` + 发 cancel 信号到 agent service（best-effort）；
4. scheduler 在其下一轮扫描中跳过此 projectID（scheduler 在执行前校验项目 status）；
5. automation engine 对此项目的触发 no-op 并记 audit `automation_skipped_project_archived`。

级联副作用里 2-5 的失败**不回滚归档**——归档本身是治理决定，in-flight 资源清理是尽力而为；个别失败通过 audit + metric 可见。

**Why this**：归档首要是"停止接受新写入"；老 run 被取消是副作用。如果把级联也做成强事务，归档会变得非常脆弱（任意一个 run 取消失败就整体回滚）。

**Alternative rejected – 只改 status，不管 in-flight**：会出现归档后 agent 还在跑的漂移，违反"归档即冻结"预期。

### 4. DELETE 收紧为"先归档才能删"

- `DELETE /projects/:id` 对 `status != 'archived'` 返回 `409 project_must_be_archived_before_delete`。
- 对 `status = 'archived'` 允许删除；owner 独占。删除执行物理删除及级联删除（tasks、members、wiki、runs、audit events 等）——这是真正的数据清除，不可恢复。
- 可选参数 `?keepAudit=true` 保留审计事件（仅删除项目及业务数据）；默认 `true`（审计保留便于事后回溯）。

**Why this**：误操作保护。先归档是一次自然的 "cooling period"——归档状态下项目仍可查看，让运营有充分时间决定是否真的要物理删除。

**Alternative rejected – 提供 `?force=true` 跳过归档**：任何 bypass 都会被滥用；不留后门。

### 5. List 默认隐藏 archived；不在 UI 混展

- `GET /projects` 默认返回 `status IN ('active', 'paused')`；加 `?includeArchived=true` 或 `?status=archived` 才返回 archived。
- 前端 `/projects` 页面顶部加一个"已归档"tab 或下拉切换，切换时请求带 `includeArchived=true`；archived 视图下卡片明显灰化并标"已归档于 YYYY-MM-DD"。
- dashboard 统计（project count、agent count、active run count）默认按 `active | paused` 聚合；archived 聚合独立呈现。

**Why this**：archived 是"治理参考"，不是"日常工作面"；默认混展会稀释真实状态。

### 6. 不自动恢复取消的 run

unarchive 不自动重新 dispatch 被归档时取消的 team run 或 workflow execution。重新启动需要用户显式再次触发。

**Why this**：被取消的 run 上下文（worktree、budget、runtime 参数）可能已失效，自动恢复容易误跑；把选择权交给用户。

## Risks / Trade-offs

- **[Risk] 归档触发时大量 in-flight run 取消可能失败或慢**：→ **Mitigation**：级联副作用 best-effort；失败经 audit + metric 可见；归档后台异步再补扫一轮兜底；API 调用本身在主事务完成后就 200 返回，不卡用户。
- **[Risk] archived-guard 与 RBAC 顺序**：若 RBAC 先、archived-guard 后，viewer 尝试写会先收到 403 而非 409 project_archived；反之亦然。→ **Mitigation**：顺序固定为 RBAC → archived-guard（先确认有权限再确认资源状态），UI 拿到 403 就知道角色问题，拿到 409 才是状态问题；文档明确这两层语义。
- **[Risk] 归档后 audit 读 API 仍可用，可能被用来"看走过什么"**：→ **Mitigation**：按设计允许；audit read 本身是 `admin+` 权限，不是泄漏。
- **[Risk] 旧代码路径在 service 层没走 handler → guard**（比如 service 直接被 scheduler 调）：→ **Mitigation**：核心 service（dispatch/team run/workflow exec/automation）入口必须 re-check 项目 status；scheduler 在 job enqueue 前 check。这些 re-check 都加在 service 签名里（已有 `initiatorUserID` 检查模式可复用）。
- **[Risk] 误归档**：owner 点错按钮导致项目被 freeze。→ **Mitigation**：归档操作 UI 要求 double-confirm（项目名输入确认），和常见 GitHub 归档交互一致。

## Migration Plan

1. Schema 迁移：`projects.archived_at`、`archived_by_user_id` 新列；回填 `status` 对 `archived_at IS NOT NULL` 的记录为 `archived`（如有已被手工改名"归档"标记的需人工对账，本 change 不自动推断）。
2. 实现 `project_lifecycle_service.Archive / Unarchive / Delete`，含级联副作用。
3. 实现 `archived_guard` 中间件并挂到 `projectGroup`（RBAC 之后）。
4. 修改 `projectH.Delete` 语义、新增 archive/unarchive handler。
5. scheduler、automation、team service、dispatch service 入口加 project-status 预检。
6. 前端 list 加 includeArchived 切换、卡片样式、archive/unarchive 按钮（owner 可见、double-confirm）。
7. 端到端测试：active 可写 / archived 不可写 / archived 可查看 / 归档级联取消 run / archived 删除成功 / non-archived 删除 409。
8. 回滚：schema 不回；如发现 guard 误拒，用 feature toggle 把 guard 临时 bypass，但不要回退到"写得进 archived"。

## Open Questions

- 归档后 memory / IM 相关绑定（飞书群回调、IM 活动聊天）是否也应该静默？当前设计：静默——IM 插件读取项目 status 在 archived 时不响应命令。细节留 Wave 3 IM 侧 spec 补齐。
- 删除项目后"项目名复用"是否允许？当前设计：允许，project name 不唯一约束；命名由人自行管理。
- archived 项目的 audit events 清理策略？当前：随项目删除而删除（除非 `?keepAudit=true`）；未来合规若要求更长保留，迁移到冷存。
