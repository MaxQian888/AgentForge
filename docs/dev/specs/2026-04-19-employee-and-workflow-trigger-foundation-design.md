# Employee Runtime and Workflow Trigger Foundation — Design

**Status**: Proposed
**Author**: AgentForge team
**Date**: 2026-04-19

## 1. 背景

AgentForge 现在能启动 workflow（`dag_workflow_service.StartExecution`），但只能通过直接 HTTP POST 手动触发；外部事件（飞书消息、Schedule）无法自动启动 workflow。同时，agent 的执行实体（`agent_runs`）是一次性的，`roles` 是只读的 YAML 模板，二者之间没有"可复用、可积累经验"的持久化身份层。这导致：

- 无法说"选品员工 A 今天又跑了 3 次，且逐步在改进" —— 每次 run 都是孤立的。
- 无法通过飞书消息"自动"启动一条代码评审 workflow —— 现有 `/review` 命令走一个独立的 review 子系统，不走 workflow 引擎。
- 没有通用的事件 → workflow 路径，后续任何事件源（webhook、Schedule、自定义 trigger）都要重新铺设基础设施。

## 2. 目标

本 spec 为"基础 spec"，交付两块地基：

1. **Employee 运行时**：一个持久化的 `Employee` 实体作为能力载体——绑定 role、可附加额外 skills、拥有 runtime 偏好、有生命周期状态、持有独立的记忆命名空间。workflow 的 `llm_agent` 节点优先通过 Employee 执行。
2. **事件 → Workflow 触发通路**：允许 IM 消息与 Schedule 自动启动 workflow execution，并把现有 `/api/v1/reviews/trigger` 改造为内部走 `system:code-review` workflow 模板。

非目标（显式推迟到其他 spec）：

| 延后的 spec | 内容 |
|---|---|
| 飞书 demo spec | 代码自动修复节点（codebot 打补丁、重开 PR） |
| 电商 demo spec | 选品 / 素材处理员工的具体业务实现（爬榜、下载图、去重、审核） |
| Webhook 事件源 spec | GitHub / HTTP 通用 webhook + HMAC 签名 |
| 自我优化 spec | harness 反馈循环、记忆写入策略、训练数据生成 |
| Workspace spec | 跨项目 Employee 共享 |
| Memory 扩展 spec | embedding / 语义检索 |

## 3. 关键决策

| 维度 | 决策 |
|---|---|
| Employee 深度 | B（DB 实体 + skill 绑定 + runtime prefs + 生命周期 + `EmployeeService.Invoke`） |
| 触发器注册 | C 混合方案（trigger 节点内 inline 作者，保存时同事务 materialize 到 `workflow_triggers` 表） |
| v1 事件源 | IM + Schedule（Webhook 推迟） |
| Employee 数据模型 | B（新 `employees` 表，项目级作用域；`agent_runs.employee_id` FK） |
| Code review 改造 | b（本 spec 内把 `/api/v1/reviews/trigger` 改为内部走 `system:code-review` workflow，对外 API 形状不变） |
| UI 覆盖面 | 后端完整 + 前端最小 CRUD 闭环：Employee CRUD 抽屉、trigger 节点 inspector、Triggers tab |
| 输入映射 DSL | 复用 `{{...}}` 模板 + `$event.*` / `$now` / `$trigger.*` 命名空间 |
| 幂等 | 每触发器可配 `idempotency_key_template` + `dedupe_window_seconds`，默认关闭 |
| Schedule overlap | 默认 `skip_if_running`，可改 `allow_parallel` |
| Employee 繁忙 | 走现有 `agent_pool_queue_entries` 排队 |
| Employee archived | Invoke fail-fast；paused → node fail + execution paused |

## 4. 命名与概念

本 spec 明确以下四个实体的边界：

| 概念 | 持久化 | 含义 |
|---|---|---|
| **Role** | YAML 文件（`roles/<id>/role.yaml`） | 行为模板（identity、system prompt、可用 skills、安全边界） |
| **Employee**（新） | `employees` 表 | 持久化能力载体：绑定一个 role + 可附加额外 skills + runtime 偏好 + 记忆命名空间 |
| **Member** | `members` 表 | 项目成员位（人或 agent 类型）；保持原意，不与 Employee 合并 |
| **AgentRun** | `agent_runs` 表 | 单次执行实例，新增 nullable `employee_id` FK |

UI 展示文案：中文语境显示 "员工" / "触发器"；英文 locale 显示 "Employee" / "Trigger"。

## 5. 架构拓扑

```
事件源                 触发通路                        Workflow 引擎               Employee 运行时             持久化
─────                 ─────                          ─────────                  ───────────              ─────
IM Bridge ──/workflow──►  TriggerRegistrar ──►  StartExecution               llm_agent 节点
                          (workflow_triggers                                    ↓
Scheduler ──cron fire──►   注册表 + 幂等/限流)   AdvanceExecution          EmployeeService.Invoke ──► agent_runs
                                                 EffectSpawnAgent               ↓                      agent_memory
旧 /reviews/trigger ──►   (适配器：启动                                      若 employee_id=null         (+employee scope)
                          system:code-review                                 退回 role_id 旧路径
                          workflow 模板)         HandleExternalEvent(HTTP)
                                                 (恢复 wait_event / 人工审批)
```

## 6. 数据模型

### 6.1 新表

**`employees`**
```
id             uuid pk
project_id     uuid fk → projects.id
name           text        -- "选品员工 A"
display_name   text
role_id        text        -- 对应 role YAML manifest id
runtime_prefs  jsonb       -- {runtime, provider, model, budget_usd, max_turns}
config         jsonb       -- system_prompt_override、额外工具等
state          text        -- active | paused | archived
created_by     uuid NULL fk → members.id   -- YAML-seeded 行为 NULL
created_at, updated_at
unique(project_id, name)
```

**`employee_skills`**（role 基础技能之外的额外绑定）
```
employee_id   uuid fk → employees.id (cascade)
skill_path    text        -- 与 role manifest skills[].path 同命名空间
auto_load     bool default true
overrides     jsonb
added_at      timestamptz
pk (employee_id, skill_path)
```

**`workflow_triggers`**
```
id             uuid pk
workflow_id    uuid fk → workflow_definitions.id
project_id     uuid fk → projects.id          -- 冗余便于租户查询
source         text     -- 'im' | 'schedule'
config         jsonb    -- im: {platform, command, match_regex?, chat_allowlist?}
                        -- schedule: {cron, timezone, overlap_policy}
input_mapping  jsonb    -- {target_key: template_string}
idempotency_key_template text
dedupe_window_seconds    int default 0
enabled        bool default true
created_by     uuid NULL fk → members.id   -- workflow 保存自动 upsert 时取当前操作者
created_at, updated_at
-- 唯一性：同一 workflow + source + config 哈希，防止重复注册
unique index on (workflow_id, source, md5(config::text))
```

### 6.2 现有表修改

- `agent_runs` 加 `employee_id uuid NULL FK → employees.id`
- `agent_memory.scope` 枚举追加 `employee`；加 `employee_id uuid NULL` 列（仅当 scope=employee 时填）
- `workflow_executions` 加 `triggered_by uuid NULL FK → workflow_triggers.id`
- `reviews` 加 `execution_id uuid NULL FK → workflow_executions.id`

### 6.3 不动的表

`members`、`roles`（YAML 仍为权威源）、`workflow_definitions`、`workflow_executions`（仅加一列）、`workflow_node_executions`、`workflow_pending_reviews`、`workflow_run_mapping`。

## 7. 组件与接口

### 7.1 Employee 运行时（`src-go/internal/employee/`）

**`EmployeeService` Go 接口**
```go
type EmployeeService interface {
    Create(ctx, CreateInput) (*Employee, error)
    Update(ctx, id, UpdateInput) (*Employee, error)
    Get(ctx, id) (*Employee, error)
    List(ctx, projectID, filter) ([]*Employee, error)
    Delete(ctx, id) error
    SetState(ctx, id, state) error
    Invoke(ctx, InvokeInput) (*InvokeResult, error)
}

type InvokeInput struct {
    EmployeeID     uuid.UUID
    TaskID         uuid.UUID
    ExecutionID    uuid.UUID
    NodeID         string
    Prompt         string
    Context        map[string]any
    BudgetOverride *float64
}

type InvokeResult struct {
    AgentRunID uuid.UUID
}
```

**Invoke 语义**（异步启动，同步返回 agent_run_id）：
1. 取 Employee，校验 state；`archived` → `ErrEmployeeArchived`；`paused` → `ErrEmployeePaused`
2. 组装运行参数：`role.Manifest + employee.config + employee_skills + employee.runtime_prefs`
3. 委托 `AgentService.Spawn(...)`（现有签名），写 `agent_runs.employee_id`；返回 `AgentRunID`
4. 并发：经由 per-runtime pool；满则走 `agent_pool_queue_entries` 排队
5. 调用方（`EffectSpawnAgent` 的 applier）在返回后创建 `WorkflowRunMapping`，等 agent 完成事件恢复节点
6. `Invoke` 本身不等待 agent 完成；agent 完成→节点恢复的异步链路保持现有机制

**Employee 的 YAML seed**
- 目录 `employees/<id>.yaml`
- 启动时 `Registry.SeedFromDir()` 按 `(project_id, name)` 幂等 upsert
- 项目初始化钩子自动 seed `default-code-reviewer`（绑定 role `code-reviewer`）

示例：
```yaml
apiVersion: agentforge/v1
kind: Employee
metadata:
  id: default-code-reviewer
  name: 默认代码评审员
role_id: code-reviewer
runtime_prefs:
  runtime: claude_code
  provider: anthropic
  model: claude-opus-4-7
extra_skills:
  - path: skills/typescript
    auto_load: true
```

**`llm_agent` 节点接入**
- 节点 config 加可选字段 `employee_id`
- `LLMAgentHandler.Execute`：
  - 设置 `employee_id` → 经 `EmployeeService.Invoke`
  - 未设置 → 老路径（直接按 `roleID` spawn）
- 两条路径最终都 emit `EffectSpawnAgent`；`applier.applySpawnAgent` 内部分流

### 7.2 Trigger 运行时（`src-go/internal/trigger/`）

**`TriggerRegistrar`**
- 启动时扫 `workflow_triggers where enabled=true`，按 `source` 建内存索引
- 保存 workflow definition 时：解析 DAG 里的 trigger 节点 → diff 当前行 → upsert；移除已不存在节点对应的行
- 暴露 `Subscribe(source, handler)`

**`EventRouter.Route(ctx, TriggerEvent)`**
```go
type TriggerEvent struct {
    Source          string
    EventData       map[string]any
    IdempotencyHint string
    TaskCtx         *TaskContext
}
```
步骤：
1. 按 source 取候选触发器
2. 跑 match filter（IM: command/regex/chat 白名单；Schedule: 由 job 本身命中，不再过滤）
3. 渲染 `idempotency_key_template`；在 dedupe window 内命中则 skip + audit log
4. 渲染 `input_mapping` → DataStore 种子
5. 调 `dagSvc.StartExecution(workflowID, taskID, seed, triggeredBy=trigger_id)`

**IM 适配器**（`src-im-bridge/commands/workflow_commands.go`）
- 注册 `/workflow <name> [args...]`
- 将 normalized message + reply_target POST 到 `POST /api/v1/triggers/im/events`
- 后端处理器调 `EventRouter.Route(source=im, ...)`
- 无匹配 → 404 + 文案"未找到匹配的工作流"
- 有匹配 → 202 + `{execution_id, trigger_name}`，bridge 先回显"已启动 <name>"

**Schedule 适配器**（`src-go/internal/scheduler/workflow_job.go`）
- 通用 workflow-start job handler
- 每条 `source=schedule` 触发器 → `scheduler.RegisterJob(key=trigger_id, cron=config.cron, handler=workflowStartHandler)`
- handler 内检查 overlap_policy：`skip_if_running` 时查该触发器的 running execution 数，>0 则 skip+log

**`HandleExternalEvent` HTTP**
- 新路由 `POST /api/v1/workflow-executions/:exec_id/events`，body `{node_id, payload}`
- 包现有 service 方法 `dagSvc.HandleExternalEvent(...)`（`dag_workflow_service.go:646`）
- 鉴权：现有 middleware；项目作用域通过 execution→workflow→project 反查

### 7.3 Review 子系统改造（对外 API 形状不变）

**原路径**
```
POST /api/v1/reviews/trigger {prUrl, projectId}
  → ReviewService.Trigger → spawn review agent run → 插 reviews 行
```

**新路径（内部）**
```
POST /api/v1/reviews/trigger {prUrl, projectId, replyTarget?}
  → ReviewService.Trigger:
      1. 建/复用 Task (type=code_review, context={pr_url})
      2. 确保项目内有 system:code-review 的 active workflow（首次 clone template）
      3. dagSvc.StartExecution(workflow_id, task_id, seed={pr_url, reply_target?}, triggered_by=<review_cmd>)
      4. 插 reviews 行 {status:pending, execution_id}
      5. 返回 {review_id, execution_id}
```

对外 API 形状保持不变（IM Bridge 的 `/review` 命令无需修改）；`replyTarget` 是可选字段——bridge 在已有"IM 作为 review 触发入口"场景里本来就附带；无 replyTarget 的调用方（如未来的其他客户端）依然可工作，只是 workflow 末端不会回发 IM 消息。

**`StartExecution` 签名扩展**

现有签名 `StartExecution(ctx, workflowID, taskID *uuid.UUID)` 扩展为：
```go
StartExecution(ctx, workflowID uuid.UUID, taskID *uuid.UUID, opts StartOptions) (*WorkflowExecution, error)

type StartOptions struct {
    Seed         map[string]any   // 预填入 WorkflowExecution.DataStore
    TriggeredBy  *uuid.UUID       // 写入 workflow_executions.triggered_by
}
```
向前兼容：现有调用点传空 `StartOptions{}` 即可。

**`system:code-review` workflow 最小结构**（本 spec 范围）
```
[trigger] → [llm_agent: employee=default-code-reviewer] → [human_review] → [status_transition] → (end)
```
- `llm_agent.config.employee_id` 指向项目 `default-code-reviewer`
- `human_review` 回调出口：`POST /api/v1/im/action`（action=`approve-review`/`request-changes`）的 handler 改造：当对应 review 行有 `execution_id` 时，转发给 `dagSvc.ResolveHumanReview(exec_id, node_id, decision)`；否则保留旧语义（直接改 `reviews` 行，仅在 feature flag 关闭时走到）
- `status_transition` effect 更新 `reviews.status`（approved / changes_requested）

**边界**：本 spec 的 workflow 仅"评审 + 人工决策"。"自动代码修复"节点在飞书 demo spec 里以扩展节点追加（`condition(decision==changes_requested) → llm_agent(employee=default-code-fixer) → ...`），无需再动基础设施。

### 7.4 前端

**`/agents` 页 · Employee tab**
- 列：name / role / state / 最近调用时间 / 累计调用数
- Drawer：
  - 基本信息（name、role 下拉来自 YAML registry API、display_name）
  - 技能绑定（role 继承部分只读 + 额外技能多选）
  - Runtime 偏好（runtime / provider / model / budget / max_turns）
  - 状态切换按钮（active / paused / archived）
  - 最近执行历史（最近 N 次 `agent_runs`）
- API：`GET|POST|PATCH|DELETE /api/v1/projects/:pid/employees[/:id]`

**Workflow 编辑器 · trigger 节点 inspector**
- 选中 trigger 节点后右侧加 "触发配置" 折叠区
- 字段按 source 切换：
  - `manual`（默认）：空
  - `im`：platform 下拉、command 输入、match_regex、chat 白名单、input_mapping 键值对编辑器
  - `schedule`：cron 输入 + 未来 5 次执行预览、timezone、overlap_policy 单选
- 保存 workflow 时后端同事务 upsert `workflow_triggers` 行

**`/workflow` 页 · Triggers tab**
- 表格：workflow 名 / source / config 摘要 / enabled 开关 / 最近触发时间 / 最近触发结果
- 一键 enable/disable；点击行展开最近 20 次触发记录（查 `workflow_executions where triggered_by=?`）

## 8. 端到端数据流

### 8.1 飞书 `/review` → 代码评审 workflow

```
1. 用户在飞书 @AgentBot /review https://github.com/acme/web/pull/42

2. Feishu SDK → IM Bridge platform/feishu/live.go
   → core/engine.go 归一化为 Message{command, args, reply_target, tenant}

3. /review 命令 handler（IM Bridge 已实现）POST /api/v1/reviews/trigger
   （bridge 侧无改动；只有后端的 ReviewService.Trigger 内部实现变了）

4. ReviewService.Trigger（改造后）：
   a. 建/复用 Task (type=code_review, context={pr_url})
   b. 确保项目有 system:code-review 模板克隆出的 active workflow
   c. 组装 seed = {$event.pr_url, $event.reply_target}
   d. dagSvc.StartExecution(workflow_id, task_id, seed, triggered_by=<review_cmd>)
   e. 插 reviews 行 (status=pending, execution_id)
   f. 返回 202 {review_id, execution_id}

5. Engine 推进：
   - trigger 节点立即 complete
   - llm_agent 节点 (employee_id=default-code-reviewer)
     → EmployeeService.Invoke (state 校验, merge 参数)
     → AgentService.Spawn → bridge 执行
     → agent_runs(employee_id=…) + workflow_run_mapping
     → 节点 park (EffectSpawnAgent)
   - agent 完成 → 现有 mapping 路径恢复节点 → AdvanceExecution
   - human_review 节点 → EffectRequestReview → workflow_pending_reviews
   - Go 调 /im/notify，用 seed.reply_target 发飞书卡片（approve/request-changes 按钮）

6. 评审人点按钮：
   - Feishu P2CardActionTriggerEvent → bridge → POST /api/v1/im/action
   - action handler 改造后调 dagSvc.ResolveHumanReview(exec_id, node_id, decision)
   - 节点 complete → status_transition 节点更新 reviews.status=approved
   - notification 节点往飞书发"审查通过"卡片
```

### 8.2 Schedule 触发电商选品 workflow

```
1. 运营在 workflow 编辑器 trigger 节点配：
   source=schedule, cron="0 9 * * *", tz=Asia/Shanghai,
   overlap_policy=skip_if_running, input_mapping={target_count: 20}

2. 保存 workflow:
   - workflow_handler 同事务 upsert workflow + workflow_triggers 行
   - TriggerRegistrar 检测新触发器 → scheduler.RegisterJob(key=<trigger_id>, cron, handler)

3. 09:00 fire:
   - scheduler 调 workflowStartHandler(trigger_id)
   - overlap 检查：该触发器有无 running execution
   - EventRouter.Route(source=schedule, EventData={$now, $trigger.config})
   - input_mapping 渲染 → {target_count: 20}
   - dagSvc.StartExecution(workflow_id, task_id=nil, seed, triggered_by=<trigger_id>)

4. 三个 llm_agent 节点串联执行：
   - employee=product-selector → 返回选品列表（落 DataStore）
   - employee=material-processor → 读前一节点 output 做素材处理
   - notification 节点 → /im/notify → 飞书卡片
```

## 9. 错误处理矩阵

| 故障场景 | 行为 | 位置 |
|---|---|---|
| Employee `archived` | Invoke 返回 `ErrEmployeeArchived`；llm_agent 节点 fail；execution 标 failed | EmployeeService |
| Employee `paused` | 节点 fail；execution 进入 paused（保留手动恢复） | EmployeeService |
| role_id 在 YAML registry 不存在 | Employee `Create` 拒绝 | EmployeeService.Create |
| 触发器同时匹配多个 workflow | 每条都触发，互不影响；幂等 key 各自独立 | EventRouter |
| 幂等 key 在 dedupe 窗口命中 | skip + audit log，不启动 execution | IdempotencyStore |
| Schedule overlap | `skip_if_running`→skip+log；`allow_parallel`→继续 | scheduler handler |
| IM `/workflow <name>` 无匹配 | bridge 回复"未找到该工作流" | IM adapter |
| input_mapping 渲染失败 | 触发 fail；写 audit 行；其他触发器不受影响 | EventRouter |
| human_review decision 非法 | `ResolveHumanReview` 400；execution 不动 | 现有 dagSvc |
| reply_target 过期 | `/im/notify` 错误；execution 已 completed；写 dead-letter | 现有 |
| 项目 archived 时触发 | `StartExecution` 已拒绝（`dag_workflow_service.go:135-138`） | 现有 |
| `workflow_triggers` unique 冲突 | repository 409；UI 提示"同配置已存在" | repository |
| Pool 满 | 通过 `agent_pool_queue_entries` 排队；execution 保持 paused | 现有 |

## 10. 测试策略

### 10.1 单元测试（Go）

- `employee/service_test.go`：Create 校验、Invoke state 分支、role/skills merge
- `trigger/router_test.go`：match filter、模板渲染、idempotency
- `trigger/registrar_test.go`：workflow 保存 → 触发器 diff/upsert
- `scheduler/workflow_job_test.go`：overlap policy 分支
- `workflow/nodetypes/llm_agent_test.go`：扩展现有测试覆盖 `employee_id` 分支

### 10.2 集成测试（touch Postgres+Redis）

- `review_integration_test.go`：旧 API 入口 → workflow 跑通 → `reviews` 行状态正确
- `trigger_im_integration_test.go`：`POST /api/v1/triggers/im/events` → execution 起 → DataStore 正确 seed
- `trigger_schedule_integration_test.go`：scheduler 跑 due → execution 起
- `employee_memory_integration_test.go`：两个 Employee 多次 Invoke → `agent_memory` 按 employee_id 正确隔离

### 10.3 端到端 smoke

- `scripts/smoke/fixtures/feishu-review.json`：完整 `/review <PR>` 链路
- `scripts/smoke/fixtures/schedule-trigger.yaml`：测试模式让 scheduler 立即 fire
- 合入 `pnpm dev:all:verify`

### 10.4 手动验证清单（落地时 demo）

- [ ] 通过 `/agents` 页 UI 建 Employee，在 workflow 里被 `llm_agent` 节点引用并跑通
- [ ] 在 workflow 编辑器给 trigger 节点配 IM → 保存后 Triggers tab 可见
- [ ] 飞书 `/review <真实 PR>` → 卡片出现 → approve → `reviews` 状态更新 → 用户收到"审查通过"卡片
- [ ] 建定时 workflow → 到点触发 → 结果回飞书

## 11. 迁移与 rollout

### 11.1 DB 迁移（一个 migration 文件）

1. 建 `employees` / `employee_skills` / `workflow_triggers` 三表
2. `agent_runs` 加 `employee_id uuid NULL`
3. `agent_memory.scope` 枚举追加 `employee` + 加 `employee_id uuid NULL`
4. `workflow_executions` 加 `triggered_by uuid NULL`
5. `reviews` 加 `execution_id uuid NULL`

### 11.2 向前兼容

- 所有旧 `agent_runs.employee_id=NULL` → `llm_agent` 节点仍按 roleID 跑
- 所有旧 `workflow_executions.triggered_by=NULL` → Triggers tab 显示 "manual / legacy"
- 所有旧 `reviews.execution_id=NULL` → UI 兼容（新字段仅内部使用）

### 11.3 Feature flag

- `USE_WORKFLOW_BACKED_REVIEW=true|false`（env）；默认 `true`
- 第一次生产演示顺利后移除

### 11.4 Seed 数据

- 项目创建流程（`ProjectService.Create` 及现有 sprint/member 初始化附近）追加一步：upsert `default-code-reviewer` employee（role_id=`code-reviewer`），失败仅告警不阻塞项目创建
- 对已存在项目：启动时 `Registry.SeedFromDir()` 扫 `employees/*.yaml` 时按 `(project_id, name)` 幂等 upsert 到每个活跃项目
- `system:code-review` workflow template 加入 `SeedSystemTemplates()`（启动时幂等 seed 到 `workflow_definitions`）

### 11.5 文档更新

- `CLAUDE.md` 追加 "Employee runtime notes" 小节，说明 Employee/Trigger 概念与 YAML seed 目录
- `docs/PRD.md` 更新"员工 / workflow"核心概念说明

## 12. 成功标准

本 spec 完成后应满足：

1. `pnpm dev:all:verify` 通过，新增两条 smoke case 全绿
2. 手动验证清单（第 10.4 节）全部勾掉
3. 所有现有集成测试不退化
4. 两个后续 demo spec（飞书代码评审 + 电商推流）只需在已有 workflow 模板上加节点，不必再触动基础设施
