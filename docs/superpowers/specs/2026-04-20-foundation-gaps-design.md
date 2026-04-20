# Foundation Gaps for Digital Employee End-to-End (Spec 1)

- **Date**: 2026-04-20
- **Status**: Approved (brainstorm phase complete)
- **Owner**: Max Qian
- **Successor specs**: Spec 2 (Code Reviewer Employee), Spec 3 (E-commerce Streaming Employee) — both depend on this spec landing first.

## 1. Problem

The "数字员工" (digital employee) surface has CRUD and trigger plumbing, but no employee can be configured and exercised end-to-end from FE + Feishu without editing YAML, raw DAG JSON, or backend code. Investigation surfaced five concrete gaps that block both planned employee verticals (code-review-and-fix; e-commerce streaming):

1. **Workflow → IM outbound回写未连接**：`replyTarget` 被采集但完成事件没有任何订阅者把结果送回 Feishu。最大的单点阻塞。
2. **缺 HTTP API 调用节点**：GitHub / 千川 / 任意 webhook 都需要它。
3. **缺 IM 富消息发送节点**：现有只有 broadcast notification，无法定向回 thread / 带按钮卡片。
4. **缺 FE trigger CRUD**：只能改 YAML 或 DAG JSON。
5. **缺按员工的执行历史看板**：员工无法被观察。

横切影响：Spec 2 / Spec 3 在不解决以上 5 项之前都跑不通端到端。本 Spec 仅覆盖 Foundation。

## 2. Goals

- 用户在 FE 创建一个 Employee + 给它绑一个 Feishu trigger（可视化表单），群里发命令 → 触发 → 看到状态卡片回帖到原线程。
- workflow 编辑器中可以拖一个 HTTP API 节点 + 一个 IM 富消息发送节点（含按钮卡片），纯 UI 拼出"调外部 API → 富卡片回 Feishu → 用户点按钮恢复 wait_event"完整闭环。
- 项目级密钥库支撑 HTTP 节点凭证；密钥可创建、轮换、删除；明文不入日志/dataStore。
- 员工详情页能看到该员工驱动的 workflow 与 agent run 历史，可下钻到节点级 trace。

## 3. Non-Goals (delegated to Spec 2 / Spec 3 / Future)

- GitHub PR webhook 监听、code_fixer 节点、补丁应用 → **Spec 2**
- 千川 OpenAPI 客户端 plugin、直播指标节点、策略沙箱 → **Spec 3**
- 进度卡片实时编辑（"已开始 / 节点 X 完成"逐条 edit 原消息） → 后续增强
- 多租户密钥层级（org/team/user-scoped secrets） → 后续
- KMS / Vault 集成；本 Spec 用进程级 master key（env `AGENTFORGE_SECRETS_KEY`） + key_version 字段为后续轮换留口
- "员工档案"统一活动流（workflow + agent + review 时间线合一） → 后续
- **Plugin run 出站回写** — 本 Spec 只覆盖 DAG execution 的自动回帖；legacy plugin run 的回帖路径继续沿用现有 review 通道

## 4. Validated Design Decisions (Q&A trail consolidated)

| # | 决策 | 备选 | 选择理由 |
|---|---|---|---|
| 1 | Spec 1 范围 = trigger CRUD + 出站回写 + 执行看板 + HTTP 节点 + IM 富发送节点 | A 仅最小回环 / B 加看板 / C 全集 | C：两个后续 spec 都依赖这套地基 |
| 2 | 出站回写 = 自动 + 显式 IM 节点可覆盖 | B 完全显式 / C 进度增量 | A：作者无心智负担；C 留给后续增强 |
| 3 | 卡片按钮支持 wait_event 恢复 | A 仅展示 / B 仅触发新 trigger | C：Spec 2 的"Apply this fix"按钮天然需要；一次做对避免返工 |
| 4 | Trigger 升级为独立资源（FE 入口在员工详情页） | A 强化 DAG 节点 / C 双入口 | B：跟员工心智吻合；避免双源问题 |
| 5 | 项目级密钥库 | A 节点内联 / C 跳过 | B：Spec 2/3 都强依赖；一次做对加密注入接口 |
| 6 | 执行历史 = workflow + agent runs | A 仅 workflow / C 统一活动流 | B：能回答"接了什么 + 花了多少"两个核心问题 |
| 7 | 富卡片 provider-neutral schema + 各平台 renderer | A 仅 Feishu | B：Spec 1 仅落地 Feishu renderer + 文本 fallback；schema 一次做对；**老 hardcode 一次性删除，不并行保留** |

## 5. Architecture

```
┌───────────────────────────────────────────────────────────────┐
│  FE (Next.js)                                                 │
│  ├─ /employees/:id/triggers   (S1 Trigger CRUD UI)            │
│  ├─ /employees/:id/runs       (S5 Execution History UI)       │
│  ├─ /projects/:id/secrets     (S4 Secrets UI)                 │
│  └─ workflow editor: HTTP node + IM-send node config 面板     │
└────────────────────┬──────────────────────────────────────────┘
                     │ REST + WS
┌────────────────────┴──────────────────────────────────────────┐
│  Go Backend (src-go)                                          │
│                                                                │
│  S1 trigger_handler (CRUD)                                    │
│      └─ writes workflow_triggers; registrar 改为 merge        │
│                                                                │
│  S2 outbound_dispatcher (新)                                  │
│      ←── EventWorkflowExecutionCompleted/Failed (现有)         │
│      └─ 检查 execution.system_metadata.im_dispatched 标记       │
│           ├─ 是 → 跳过                                         │
│           └─ 否 → render 默认卡片 → IM Bridge.Send(replyTarget)│
│                                                                │
│  S3 nodetypes/http_call.go    (新节点)                        │
│  S3 nodetypes/im_send.go      (新节点；运行时设 im_dispatched) │
│  S3 card_action_router (新)                                   │
│      ←── IM Bridge inbound card_action callback                │
│      ├─ correlation 命中 wait_event → wait_event resumer       │
│      └─ 无 correlation → 走 trigger_router (作为 action source)│
│                                                                │
│  S4 secrets_handler + secrets_store + 模板引擎扩展             │
│      └─ {{secrets.X}} 解析点：HTTP 节点执行时（非 start time） │
│                                                                │
│  S5 employee_runs_handler                                     │
│      └─ UNION query: workflow_executions ∪ agent_runs         │
│           where acting_employee_id = :id                       │
└────────────────────┬──────────────────────────────────────────┘
                     │
┌────────────────────┴──────────────────────────────────────────┐
│  IM Bridge (src-im-bridge)                                     │
│  ├─ core/card_schema.go (新，provider-neutral 抽象)            │
│  ├─ platform/feishu/render.go (新，从 live.go 抽出 render)    │
│  ├─ platform/{slack,dingtalk,...}/render.go (新，文本 fallback)│
│  └─ inbound card_action → POST /api/v1/im/card-actions (新)    │
│  ⚠️ 删除 feishu/live.go 内 renderInteractiveCard /             │
│       renderStructuredMessage 旧实现（不并行保留）             │
└────────────────────────────────────────────────────────────────┘
```

**关键不变量**

- 触发链路（IM 入站 → trigger router → workflow start）不动
- DAG 执行器与节点注册机制不动
- workflow_triggers 表仅扩字段（不改主键不删字段），registrar 改为"合并"

## 6. Data Model

### 6.1 New tables

```sql
secrets (
  id uuid PRIMARY KEY,
  project_id uuid NOT NULL REFERENCES projects(id),
  name varchar(128) NOT NULL,
  ciphertext bytea NOT NULL,    -- AES-256-GCM
  nonce bytea NOT NULL,
  key_version int NOT NULL DEFAULT 1,
  description text,
  last_used_at timestamptz,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, name)
);

card_action_correlations (
  token uuid PRIMARY KEY,        -- 嵌入按钮 payload，无遍历价值
  execution_id uuid NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
  node_id text NOT NULL,         -- wait_event 节点 id
  action_id text NOT NULL,       -- 按钮逻辑 id ('approve' / 'reject' / ...)
  payload jsonb,                 -- 注入到 wait_event 的输入
  expires_at timestamptz NOT NULL,  -- 默认 now() + 7 days
  consumed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON card_action_correlations (expires_at) WHERE consumed_at IS NULL;
```

### 6.2 workflow_triggers 字段扩展（仅加字段）

```sql
ALTER TABLE workflow_triggers
  ADD COLUMN created_via varchar(16) NOT NULL DEFAULT 'dag_node',
       -- enum('dag_node', 'manual')
  ADD COLUMN display_name varchar(128),
  ADD COLUMN description text;
```

**Registrar 行为变更**：从"删后全插"→"按 (workflow_id, source, config_hash) upsert，仅作用于 `created_via='dag_node'` 行"。FE 创建的 `manual` 行不受 DAG 保存影响。

### 6.3 workflow_executions 字段扩展

调查显示 `WorkflowExecution` 当前结构为 `currentNodes[] / dataStore{} / status`，无系统级元数据袋。新增：

```sql
ALTER TABLE workflow_executions
  ADD COLUMN system_metadata jsonb NOT NULL DEFAULT '{}'::jsonb;
  -- 系统保留键：
  --   reply_target          {provider, chat_id, thread_id, message_id, tenant_id}
  --   im_dispatched         bool   ← im_send 节点执行成功后置 true，抑制 outbound_dispatcher 默认回帖
  --   final_output          jsonb  ← 可选；workflow 作者显式声明的"完成态摘要"
```

**为什么单独开 `system_metadata` 而不是塞 `dataStore`**：dataStore 是节点 I/O 缓存，作者可读写；系统标记必须与作者数据隔离（避免命名冲突 + 防止作者代码误改 im_dispatched）。trigger_handler 在创建 execution 时把 replyTarget 写入此字段。

## 7. API Endpoints (new)

```
S1 Trigger CRUD
  GET    /api/v1/employees/:id/triggers
  POST   /api/v1/triggers
         body: {workflow_id, source, config, input_mapping,
                acting_employee_id, display_name, description}
  PATCH  /api/v1/triggers/:id
  DELETE /api/v1/triggers/:id
  POST   /api/v1/triggers/:id/test            sample IM event 干跑

S4 Secrets
  GET    /api/v1/projects/:id/secrets         无值，仅 name + 元数据
  POST   /api/v1/projects/:id/secrets         返回的 value 仅创建/轮换时可见
  PATCH  /api/v1/projects/:id/secrets/:name
  DELETE /api/v1/projects/:id/secrets/:name

S5 Employee runs
  GET /api/v1/employees/:id/runs?type=workflow|agent|all&page=&size=

S3 Card action 入站
  POST /api/v1/im/card-actions                IM Bridge → backend
  body: {correlation_token, action_id, value, replyTarget,
         user_id, tenant_id}
```

错误码（FE 据此渲染本地化文案）：
```
trigger:workflow_not_found, trigger:acting_employee_archived,
secret:not_found, secret:decrypt_failed,
card_action:expired, card_action:consumed,
card_action:execution_not_waiting
```

## 8. Provider-Neutral Card Schema

```go
// src-im-bridge/core/card_schema.go (字段以 JSON 线协议为准；下文用 TS interface 仅为可读性)
// 真实 Go 结构: 使用 struct + json tag，CardAction 使用 type 字段做 discriminated union 解码

interface ProviderNeutralCard {
  title: string
  status?: 'success' | 'failed' | 'running' | 'pending' | 'info'
  summary?: string                // 短 markdown
  fields?: Array<{label: string, value: string, inline?: boolean}>
  actions?: Array<CardAction>
  footer?: string
}

export type CardAction =
  | { id: string, label: string, style?: 'primary'|'danger'|'default', type: 'url', url: string }
  | { id: string, label: string, style?: 'primary'|'danger'|'default', type: 'callback',
      correlation_token: string, payload?: Record<string, unknown> }
```

每 provider 一个 renderer（IM Bridge 是 **Go** 模块，文件后缀实际为 `.go`，下面写 `.ts` 是历史草稿，对应 Go 文件）：
- `platform/feishu/render.go` → 完整富卡片
- `platform/slack/render.go` → blocks
- `platform/dingtalk/render.go` → ActionCard
- 其余 → `platform/_fallback/text.go`：`title\nsummary\n[label] value\n...\nURL` 拼接

## 9. End-to-End Data Flows

### Trace A — 最小回环

```
[FE] /employees/:id/triggers
     POST /api/v1/triggers {workflow_id, source:im, config:{platform:feishu, command:'/echo'},
                            input_mapping, acting_employee_id}
     → trigger row (created_via='manual')

[Feishu] 用户发 "/echo hello"
     IM Bridge inbound → POST /api/v1/triggers/im/events {…, replyTarget}
     trigger_handler.router 匹配 → workflow start
     execution.system_metadata.reply_target = <Feishu thread ref>  # trigger_handler 写入

[DAG runner] 完成 → broadcast EventWorkflowExecutionCompleted

[outbound_dispatcher]
     execution.system_metadata.im_dispatched ? skip : continue
     last node output → ProviderNeutralCard{title, status:'success', summary, footer:exec_id,
       actions:[{type:url, label:'查看详情', url:'/runs/<id>'}]}
     POST IM Bridge /im/send {replyTarget, card}
     bridge → feishu/render → Feishu reply API
[Feishu] 同线程出现卡片 ✅
```

### Trace B — 交互闭环（HTTP + secret + 按钮恢复）

```
DAG: trigger → http_call → im_send(callback buttons) → wait_event → condition → end

[http_call] config.headers = {"Authorization": "Bearer {{secrets.GITHUB_TOKEN}}"}
     secret_resolver 在节点执行点解析（非 workflow start 时）
       secrets_store.Decrypt(project_id, 'GITHUB_TOKEN') → 注入 → fetch
     response → dataStore[node_id] = {status, body, headers}

[im_send] 渲染 card.actions:[{id:'approve', type:'callback', payload:{}}, {id:'reject', ...}]
     对每 callback action 调 card_action_correlations.Create(execution_id, wait_event_node_id, action_id)
        → correlation_token → 写进 button payload
     execution.system_metadata.im_dispatched = true   # 抑制 outbound_dispatcher 默认回帖
     POST IM Bridge /im/send

[wait_event] park execution (status=waiting)

[Feishu 用户] 点 "Approve"
     IM Bridge inbound card_action → POST /api/v1/im/card-actions
       {correlation_token, action_id:'approve', value, user_id, tenant_id}
     card_action_router.Lookup(token)
       命中 + 未 consumed + 未过期 → wait_event_resumer.Resume(execution_id, node_id, payload)
       consumed_at = now()
       不命中 → 走 trigger_router 视为 IM 事件（兜底）

[DAG runner] wait_event 收到 input → 续跑 → condition 分支 → end
[outbound_dispatcher] im_dispatched=true → 跳过默认卡片 ✅
```

### Trace C — 失败回写

```
[http_call] 401 → 节点 error → execution status=failed
[outbound_dispatcher] EventWorkflowExecutionFailed
     {title:'workflow X 执行失败', status:'failed',
      summary:'HTTP 401 at node http_call',
      fields:[{label:'失败节点', value:'http_call'}, {label:'Run', value:exec_id}],
      actions:[{label:'查看 trace', type:'url', url:'/runs/<id>'}]}
     发回原 replyTarget ✅
```

### 边界

- **schedule trigger 无 replyTarget** → outbound_dispatcher 静默跳过；需要通知必须在 DAG 末显式 im_send 节点（指定固定 chat_id）
- **employee dashboard 实时刷新**：复用现有 WS workflow_step / agent_run 事件，按 `acting_employee_id` 过滤；无新事件类型
- **secret 解析时机**：节点执行点；轮换后下一次执行立即生效；不缓存

## 10. Error Handling

| 场景 | 行为 |
|---|---|
| outbound_dispatcher 发送失败 | 3 次指数退避（1s/4s/16s）→ 失败：log + emit `EventOutboundDeliveryFailed`，FE execution 详情显示"回帖失败"badge；**不**回滚 workflow 状态 |
| secret 不存在 | 节点 error `secret:not_found` |
| secret 解密失败（key 不匹配） | 节点 error `secret:decrypt_failed`，**不泄露** ciphertext / nonce / key_version |
| HTTP 4xx/5xx | 默认 fail；`treat_as_success: [...]` 白名单可豁免 |
| HTTP timeout | 节点 config 默认 30s；超时 → fail |
| card_action token 已过期 | IM Bridge 410 → Feishu toast"操作已过期" |
| card_action token 已消费 | IM Bridge 409 → toast"操作已处理" |
| card_action 命中但 execution 已结束 | wait_event_resumer 检测 status≠waiting → 409 → toast"工作流已结束" |
| Trigger CRUD 校验失败 | 400 + 结构化错误码 |
| Registrar merge | 仅作用 `created_via='dag_node'` 行；manual 行不可被 DAG 覆盖 |

## 11. Security Boundaries

- **secret 明文生命周期**：仅在解密 → 注入 HTTP req 之间存在内存中。**不进** dataStore / log / 错误信息 / WS 广播
- **`{{secrets.X}}` 模板白名单**：仅允许在 HTTP 节点 config 的 `headers` / `url query` / `body` 字段；其它节点 / dataStore 引用 = 编译期 reject
- **审计**：secret CRUD / trigger CRUD / card_action 命中全部写 audit；secret audit 含 `name / actor / op` 不含值
- **Master key**：env `AGENTFORGE_SECRETS_KEY`，min 32 字节；缺失则 secrets 子系统启动期 panic（fail-fast）
- **CORS / 鉴权**：所有新端点走现有 JWT + project RBAC（owner/admin/editor 可写 secret/trigger，viewer 仅读）

## 12. Old Code Deletion Checklist

与"AgentForge 在内部测试期，breaking changes freely permitted；老代码及时清理"对齐：

```
delete  src-im-bridge/platform/feishu/live.go::renderInteractiveCard (~line 1281+)
delete  src-im-bridge/platform/feishu/live.go::renderStructuredMessage 同区段
migrate 所有 hardcoded Feishu card builder 调用 → core/card_renderer.Dispatch
modify  components/workflow/workflow-triggers-section.tsx
        删除内嵌 toggle CRUD；改为 list-only-with-link
        编辑入口统一去 /employees/:id/triggers
delete  workflow-trigger-store.ts 的 toggle-only 路径，改为完整 CRUD store
⚠️ 不保留旧路径 feature flag、不并行 deprecate
```

## 13. Testing Strategy

```
Unit Go
  ├─ secrets_store: encrypt/decrypt round-trip + key_version mismatch
  ├─ card_action_correlations: hit / miss / expired / consumed 4 路径
  ├─ outbound_dispatcher: 4 矩阵（replyTarget × im_dispatched）
  ├─ trigger registrar merge: dag_node 替换 + manual 保留
  └─ http_call handler: 2xx / 4xx / treat_as_success / timeout

Unit Go (IM Bridge — 非 TS)
  ├─ card schema render: Feishu / Slack / 钉钉 snapshot
  ├─ 不支持 provider → plain text 退化 snapshot
  └─ inbound card_action: valid / expired token

Integration Go (PG + Redis)
  ├─ Trace A: trigger CRUD via API → 模拟 IM 事件 → 完成 → outbound 发送（mock bridge endpoint）
  ├─ Trace B: card → callback → wait_event resume → continue
  └─ Trace C: HTTP 401 → 失败卡片

E2E smoke (扩展 src-im-bridge/scripts/smoke/)
  ├─ feishu-workflow-with-card.json     (Trace A)
  ├─ feishu-workflow-button-resume.json (Trace B)
  └─ feishu-workflow-http-fail.json     (Trace C)

FE Jest
  ├─ trigger CRUD form: validation + submit + list refresh + WS
  ├─ secret 表单: create + rotate + value 仅创建/轮换时可见
  └─ /employees/:id/runs: union list + drill-down + WS 增量
```

## 13.1 Spec Drifts Found During Plan Writing

Plan-writer subagents surfaced 3 facts that contradict earlier spec text. Preserved here so future readers see the truth:

- **IM Bridge is Go, not TypeScript/Bun.** §5 / §8 / §13 originally referred to `.ts` files; corrected above. Go-side `ProviderNeutralCard` struct lives in `src-im-bridge/core/card_schema.go` with JSON tags matching the schema in §8.
- **No separate `EventWorkflowExecutionFailed`.** Existing `EventWorkflowExecutionCompleted` carries `status ∈ {completed, failed, cancelled}`; outbound_dispatcher branches on payload status field. Plan 1D follows this contract.
- **`acting_employee_id` already exists on `workflow_executions` and `workflow_triggers`** (added by migration 064 — `merge: Employee runtime + workflow trigger foundation`). §6.2 / §6.3 only ADD the new columns; they do not re-add `acting_employee_id`.

## 14. Open Risks

- **Card payload size 限制**：Feishu 单卡片 JSON 有大小上限；执行 metadata + correlation 信息全塞按钮可能溢出。缓解：correlation_token 是 uuid（36 字节），payload 不放完整 dataStore 而只放 action_id + 用户透传字段。
- **wait_event resumer 存在性**：调查报告说 `wait_event` 节点已注册但实现"minimal stubs"。本 Spec 假定 resumer 接口已可用；若需补完，纳入本 Spec 实现成本（标记为前置任务）。
- **Registrar 迁移**：把 "删后全插" 改成 merge 涉及现有数据；需要一个一次性 backfill 脚本把存量行标记 `created_via='dag_node'`。
- **`system_metadata` 字段尺寸**：默认 jsonb 无硬上限但 reply_target + final_output 期望 < 4 KB；DAG 节点禁止写此字段（在 dataStore 模板引擎里 reject `system_metadata` 引用）。

## 15. Successor Spec Hooks

- **Spec 2 (Code Reviewer)** 将依赖：
  - HTTP 节点 + secret 库（GitHub PAT）
  - card 按钮 callback + wait_event resume（per-finding "Apply this fix"）
  - 出站回写（review summary 卡片）
- **Spec 3 (E-commerce Streaming)** 将依赖：
  - HTTP 节点 + secret 库（千川 OAuth refresh token）
  - schedule trigger（minute cron）+ 显式 im_send 节点（GMV 告警）
  - 员工执行历史（策略命中/动作执行轨迹）
