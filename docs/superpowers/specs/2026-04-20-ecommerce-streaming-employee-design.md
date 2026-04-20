# E-commerce Streaming Digital Employee (Spec 3)

- **Date**: 2026-04-20
- **Status**: Approved (brainstorm phase complete)
- **Owner**: Max Qian
- **Predecessor specs**:
  - [Spec 1 — Foundation Gaps](./2026-04-20-foundation-gaps-design.md) — Trigger CRUD, outbound IM dispatcher, HTTP node, project secrets store + `{{secrets.X}}` template, IM-send node, interactive cards with `wait_event` resumption (Trace B). **Required.**
  - [Spec 2 — Code Reviewer Employee](./2026-04-20-code-reviewer-employee-design.md) — Provider-neutral `vcs.Provider` interface and the digital-employee composition shape that Spec 3 mirrors. **Reference only**, no runtime dependency.

## 1. Problem

The "数字员工" (digital employee) framing only earns its name once we ship a vertical that demonstrates **closed-loop autonomy** — observe a high-velocity external system, decide using author-supplied policy, act on that system, and report back. E-commerce live-streaming on Douyin / Qianchuan (巨量千川) is the canonical instance: live sessions burn ¥10k+/hour in ad spend, ROI swings minute-by-minute, and human operators are presently glued to dashboards adjusting bids, budgets and creatives by hand. The user's earlier reference project (`D:\Project\qianchuan-claw-main`) proved the mechanics work — minute-bucket polling, sandboxed strategies, control commands, Feishu notification — but it is platform-locked, runs raw `new Function()` strategies, and is not embedded in any agent platform.

AgentForge needs to land that capability **as a digital employee** so that:

1. A user creates an "Live-Stream Operator" employee in the FE, OAuths a Qianchuan account, picks a strategy from a library, and walks away.
2. The employee polls metrics, runs the strategy, executes ad-platform actions, and posts a Feishu summary card every minute (only when something happened).
3. Every action the agent takes is auditable, reversible (within Qianchuan's contract), and gated by per-binding policy — the **digital employee is conservative by default**.
4. The architecture is provider-neutral so Taobao, JD Cloud Ads, Kuaishou, TikTok Ads can plug in later without re-doing the workflow shape, the FE, or the strategy DSL.

This spec is exclusively the e-commerce streaming employee. It is the second vertical riding the Spec 1 foundation; the first is Spec 2 (Code Reviewer).

## 2. Goals

- A user with no Go / TypeScript skill can: bind a Qianchuan account via OAuth → assign a system-seeded strategy → see metrics charts and action logs in the FE within the same session.
- The employee runs a **canonical workflow** — one DAG per binding fires every N seconds (default 60s, range 10s–1h) — that fetches metrics, evaluates a YAML strategy, applies the emitted actions through a policy gate, and posts a Feishu card when anything material occurred.
- Strategies are **declarative YAML resources** (loaded the same way roles/skills are loaded), authored without writing code; expressions are evaluated by the existing `nodetypes/expr.go` evaluator with no `eval` / `new Function` escape hatch.
- Every Action passes through a **policy gate** (per-binding limits + human-approval list) and high-risk actions are routed through Spec 1's interactive card + `wait_event` flow for explicit operator approval.
- OAuth tokens persist as project secrets via Spec 1B's encrypted store; refresh runs unattended on a minute cron and degrades gracefully (auth-expired marking + admin notification + binding pause) on failure.
- Provider-neutral interface (`internal/adsplatform/Provider`) lands on day 1 with Qianchuan as the only concrete implementation; future platforms add files, not contracts.

## 3. Non-Goals (deferred or out of scope)

- **Multi-platform** (Taobao, JD, Kuaishou, TikTok Ads) — interface only; no concrete implementations beyond Qianchuan in v1.
- **JS / TS sandbox strategy authoring** — defensible default is YAML-only. A scripted escape hatch is enumerated in §16 as a successor hook only.
- **Time-series compaction job** (rolling minute → hourly buckets after 90 days) — table is partition-friendly but the compaction worker is deferred to v1.1; see §15.
- **Strategy backtesting / replay** — the snapshot table makes this possible later but the backtester is not in scope.
- **Material upload pipeline** — v1 only references already-uploaded materials by Qianchuan asset ID; the upload UX is its own surface.
- **Cost dashboard integration** — ad spend numbers stay scoped to this employee's view; piping into the global cost surface is deferred.
- **Multi-tenant secrets sharing** (one secret usable across projects) — same constraint as Spec 1B; project-scoped.
- **Real-time websocket push of metrics** — FE polls the snapshot table every 10s while the employee page is open; live push deferred.

## 4. Validated Design Decisions

| # | 决策 | 备选 | 选择理由 |
|---|---|---|---|
| 1 | Platform scope = Douyin / Qianchuan only for v1; provider-neutral interface from day 1 | A 多平台并行 / B 千川硬编码 | C：单一深度交付证明形态可行，接口预留后续插拔；与 Spec 1 card-provider 抽象一致 |
| 2 | Automation depth = 监控 + 决策 + 执行 + 通知 全闭环 | A 仅监控 / B 监控+通知 | C：数字员工命题要求闭环；与 Spec 2 选 C 对齐 |
| 3 | 策略授权模型 = YAML 声明式，复用 `nodetypes/expr.go` 表达式引擎，固定动作原语 | A 原生 JS 沙箱 / C 可视化拖拽 | B：避免引入新沙箱面积；表达式引擎已生产可用；YAML 与 role/skill 资源同形 |
| 4 | Trigger = schedule trigger（默认 60s，可配 10s–1h），按 binding Redis 锁去重 | A 事件驱动 / B 手动 | C：千川无 webhook；polling 是唯一选项；锁防止节拍重叠 |
| 5 | Token 刷新 = Go 后端独立 goroutine + minute cron；过期 10 分钟内主动 refresh；失败降级 | A 同步刷新 / B BFF 刷新 | C：业务请求与刷新解耦；失败可一次性通知并暂停所有 binding 策略 |
| 6 | Workflow 形态 = 单一 canonical DAG `qianchuan_strategy_loop`，每 binding 一份 trigger | A 多 workflow / B 内嵌循环 | C：作者心智成本最低；trigger 维度与 binding 维度对齐 |
| 7 | 存储 = 5 张新表 + token 走 Spec 1B 加密 secrets | A token 入库 / C 内存 cache | B：token 是高敏感资产，必须复用 Spec 1B；策略/快照是审计与图表的基础 |
| 8 | OAuth = 后端 callback 换 token → secrets 加密 → 创建 binding；FE 绑定按钮跳转 | A 前端持有 token | B：token 不出后端；与 Spec 1B `{{secrets.X}}` 注入路径同源 |
| 9 | FE 表面 = `/employees/[id]/qianchuan` overview + `/projects/[id]/qianchuan/strategies` + `/projects/[id]/qianchuan/bindings` | A 单页大表 / B 内嵌员工详情 | C：employee 维度看运行态，project 维度看资源管理；同 Spec 1 双入口模型 |
| 10 | 多租户隔离 = bindings/strategies/action_logs 都按 project_id；可选系统 seed 策略 | A 全局共享 / B 用户私有 | C：项目是 AgentForge 的一等隔离单元；seed 策略降低空白成本 |
| 11 | Provider 抽象 = `internal/adsplatform/Provider` 接口包 + `internal/qianchuan/` 实现包；后续平台同形复制 | A 千川硬编 / B 插件机制 | C：mirror Spec 2 的 `vcs.Provider` 形态；编译期接口胜过运行期插件复杂度 |
| 12 | Action 安全 = 每动作走 policy gate；超阈或在 `require_human_approval_for` 列表 → Spec 1E 卡片 + wait_event | A 全自动 / B 全人工 | C：保守默认 + 显式开闸；复用 Spec 1E 通路是零成本 |
| 13 | Telemetry = 每 strategy run + 每 action 一行；FE 图表查这两张表；compaction 在 §15 | A 仅日志 / B 时序数据库 | C：PG 已经在场；90 天热数据 + 后续 compaction job 即可；不引新依赖 |
| 14 | Old code cleanup = 无遗留路径，全 greenfield | — | 项目内不存在旧 qianchuan 实现；与"内部测试期 breaking changes"对齐 |

## 5. Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│  FE (Next.js)                                                          │
│  ├─ /employees/:id/qianchuan        Overview tab                       │
│  │     bindings list · recent metrics chart · recent actions          │
│  │     strategy assignment selector                                    │
│  ├─ /employees/:id/qianchuan/oauth  OAuth init + callback landing      │
│  ├─ /projects/:id/qianchuan/strategies  YAML editor + schema validate │
│  └─ /projects/:id/qianchuan/bindings    Bind / pause / resume / status │
└────────────────────┬───────────────────────────────────────────────────┘
                     │ REST + WS (existing)
┌────────────────────┴───────────────────────────────────────────────────┐
│  Go Backend (src-go)                                                   │
│                                                                         │
│  internal/adsplatform/                                                 │
│      Provider interface  { RefreshToken, FetchMetrics, ApplyAction,    │
│                            OAuthExchange, OAuthAuthorizeURL }          │
│      registry  { providerID → factory }                                │
│                                                                         │
│  internal/qianchuan/                  ← only concrete provider in v1   │
│      client.go         HTTP client (URL build, signing, retry)         │
│      provider.go       Provider impl                                   │
│      oauth.go          OAuth state CSRF + code exchange                │
│      mapping.go        normalize Qianchuan response → adsplatform.*    │
│                                                                         │
│  internal/strategy/                                                    │
│      manifest.go       parse YAML → Strategy{} (mirrors role/parser.go)│
│      engine.go         Evaluate(snapshot, strategy) → []Action         │
│      policy_gate.go    GateAction(action, policy) → ok | needs_approve │
│      registry.go       in-memory cache, repo-backed reload             │
│                                                                         │
│  internal/qianchuan_runtime/                                           │
│      token_refresher.go  minute goroutine; refresh @≤10min to expiry  │
│      binding_runner.go   per-trigger orchestration helper used by     │
│                          DAG nodes (or invoked from a synthetic        │
│                          system workflow)                              │
│                                                                         │
│  Reuses from Spec 1:                                                   │
│   · nodetypes/http_call.go   metric fetch + action apply               │
│   · nodetypes/im_send.go     summary card / approval card              │
│   · nodetypes/wait_event.go  human-approval pause                      │
│   · secrets store + {{secrets.X}}                                      │
│   · outbound_dispatcher (silent for schedule-only runs — Spec 1 §9)    │
└────────────────────┬───────────────────────────────────────────────────┘
                     │
┌────────────────────┴───────────────────────────────────────────────────┐
│  External: api.oceanengine.com (Qianchuan OpenAPI)                     │
│  IM Bridge → Feishu (reuses Spec 1 pipeline)                           │
└────────────────────────────────────────────────────────────────────────┘
```

**Key invariants**

- Adsplatform interface is **the only** import boundary the workflow nodes know about. The DAG never references `internal/qianchuan` directly; it always goes through the registry.
- Token plaintext lives only in: (a) `secrets.Service.Resolve` return, (b) the outbound HTTP request body, (c) an in-memory token cache keyed by binding (≤10min TTL) used by the refresher to avoid double-decrypting on every minute. The cache **does not persist**.
- Strategies, bindings, snapshots, and action logs are all `project_id`-scoped; `RequireProjectRole` middleware gates every endpoint per the existing RBAC matrix.
- One trigger = one binding = one workflow execution every tick. Per-binding Redis lock (`af:qc:lock:<binding_id>`, TTL = 2× tick interval) skips the tick if a previous run is still inside the DAG.

## 6. Data Model

Five new tables. Tokens go through Spec 1B's `secrets` table — they are not stored in the binding row.

### 6.1 `qianchuan_bindings` (migration 071)

```sql
CREATE TABLE qianchuan_bindings (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id           UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  employee_id          UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
  display_name         VARCHAR(128) NOT NULL,        -- author label, e.g. "店铺A 主直播间"
  provider_id          VARCHAR(32) NOT NULL DEFAULT 'qianchuan',
  advertiser_id        VARCHAR(64) NOT NULL,         -- Qianchuan 广告主 ID
  aweme_id             VARCHAR(64),                  -- 抖音号 ID（可选）
  scope                TEXT[] NOT NULL DEFAULT '{}', -- granted OAuth scopes
  access_token_secret  VARCHAR(160) NOT NULL,        -- secrets.name e.g. 'qianchuan.<id>.access_token'
  refresh_token_secret VARCHAR(160) NOT NULL,
  access_expires_at    TIMESTAMPTZ NOT NULL,
  refresh_expires_at   TIMESTAMPTZ NOT NULL,
  status               VARCHAR(24) NOT NULL DEFAULT 'active',
                       -- enum('active','paused','auth_expired','error')
  status_reason        TEXT,
  policy               JSONB NOT NULL DEFAULT '{}'::jsonb,
                       -- {max_bid_change_pct, max_budget_change_per_day,
                       --  require_human_approval_for: ['pause_ad', ...]}
  strategy_id          UUID REFERENCES qianchuan_strategies(id) ON DELETE SET NULL,
  tick_interval_sec    INT NOT NULL DEFAULT 60,      -- 10..3600
  trigger_id           UUID REFERENCES workflow_triggers(id) ON DELETE SET NULL,
  created_by           UUID NOT NULL,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (project_id, advertiser_id, COALESCE(aweme_id, ''))
);
CREATE INDEX qianchuan_bindings_project_idx ON qianchuan_bindings(project_id);
CREATE INDEX qianchuan_bindings_employee_idx ON qianchuan_bindings(employee_id);
CREATE INDEX qianchuan_bindings_status_idx   ON qianchuan_bindings(status)
  WHERE status IN ('active','auth_expired');
```

### 6.2 `qianchuan_strategies` (migration 070)

(Authored before §6.1 above to satisfy the `qianchuan_bindings.strategy_id` foreign key. The numbering reflects file-creation order; the spec presents bindings first only because it is the central concept readers care about.)

```sql
CREATE TABLE qianchuan_strategies (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id    UUID REFERENCES projects(id) ON DELETE CASCADE,  -- NULL = system seed
  name          VARCHAR(128) NOT NULL,
  version       VARCHAR(32)  NOT NULL DEFAULT 'v1',
  yaml_source   TEXT         NOT NULL,            -- canonical author input
  parsed_spec   JSONB        NOT NULL,            -- normalized form
  description   TEXT,
  created_by    UUID,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (project_id, name, version)
);
CREATE INDEX qianchuan_strategies_project_idx ON qianchuan_strategies(project_id);
```

### 6.3 `qianchuan_metric_snapshots` (migration 072) — minute-bucketed time series

```sql
CREATE TABLE qianchuan_metric_snapshots (
  id            BIGSERIAL PRIMARY KEY,
  binding_id    UUID NOT NULL REFERENCES qianchuan_bindings(id) ON DELETE CASCADE,
  bucket_at     TIMESTAMPTZ NOT NULL,             -- truncated to minute
  metrics       JSONB NOT NULL,
                -- normalized adsplatform.MetricSnapshot:
                --   { live: {viewers, gmv, clicks, ctr, ...},
                --     ads:  [{ad_id, spend, roi, cpm, status, bid, budget, ...}],
                --     materials: [{id, status, health: 0..1, ...}] }
  fetched_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (binding_id, bucket_at)
);
CREATE INDEX qianchuan_snapshots_binding_time_idx
  ON qianchuan_metric_snapshots(binding_id, bucket_at DESC);
```

Retention: rows older than 90 days are eligible for compaction (deferred — see §15).

### 6.4 `qianchuan_action_logs` (migration 073)

```sql
CREATE TABLE qianchuan_action_logs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  binding_id      UUID NOT NULL REFERENCES qianchuan_bindings(id) ON DELETE CASCADE,
  execution_id    UUID REFERENCES workflow_executions(id) ON DELETE SET NULL,
  strategy_id     UUID REFERENCES qianchuan_strategies(id) ON DELETE SET NULL,
  action_kind     VARCHAR(48) NOT NULL,
                  -- 'adjust_bid','adjust_budget','pause_ad','resume_ad',
                  -- 'apply_material','notify_im','record_event'
  target_ref      VARCHAR(96),                  -- e.g. ad_id
  request         JSONB,
  response        JSONB,
  outcome         VARCHAR(24) NOT NULL,
                  -- 'applied','blocked_by_policy','approved','rejected',
                  -- 'failed','noop'
  outcome_detail  TEXT,
  approved_by     UUID,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX qianchuan_actions_binding_idx ON qianchuan_action_logs(binding_id, created_at DESC);
CREATE INDEX qianchuan_actions_outcome_idx ON qianchuan_action_logs(outcome);
```

### 6.5 `qianchuan_oauth_states` (migration 074) — short-lived CSRF nonces

```sql
CREATE TABLE qianchuan_oauth_states (
  state         VARCHAR(64) PRIMARY KEY,
  project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  employee_id   UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
  initiated_by  UUID NOT NULL,
  redirect_uri  TEXT NOT NULL,
  expires_at    TIMESTAMPTZ NOT NULL,            -- now() + 10min
  consumed_at   TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX qianchuan_oauth_states_expiry_idx
  ON qianchuan_oauth_states(expires_at) WHERE consumed_at IS NULL;
```

**Migration numbering**: 070–074. Spec 1 plans claim 067–068; Spec 2 is expected to claim 069. If Spec 2 lands additional migrations between 069 and 070, this spec's numbers shift in lockstep — none of the migration content depends on the number itself.

## 7. API Endpoints

```
Bindings
  GET    /api/v1/projects/:project_id/qianchuan/bindings
  GET    /api/v1/employees/:employee_id/qianchuan/bindings
  POST   /api/v1/projects/:project_id/qianchuan/bindings/oauth/start
         body: {employee_id, display_name, redirect_uri}
         resp: {authorize_url, state}
  GET    /api/v1/qianchuan/oauth/callback?code=&state=
         (browser-facing — exchanges code → tokens → creates binding row,
          encrypts tokens via secrets, redirects FE)
  PATCH  /api/v1/qianchuan/bindings/:id          { status, policy, strategy_id, tick_interval_sec, display_name }
  POST   /api/v1/qianchuan/bindings/:id/pause
  POST   /api/v1/qianchuan/bindings/:id/resume
  DELETE /api/v1/qianchuan/bindings/:id

Strategies
  GET    /api/v1/projects/:project_id/qianchuan/strategies?include_system=true
  POST   /api/v1/projects/:project_id/qianchuan/strategies   { name, yaml_source }
  PATCH  /api/v1/qianchuan/strategies/:id                    { yaml_source, description }
  DELETE /api/v1/qianchuan/strategies/:id
  POST   /api/v1/qianchuan/strategies/:id/dry-run            { binding_id }
         → execute strategy against latest snapshot, do NOT apply, return {actions[]}

Snapshots & actions (FE charts)
  GET    /api/v1/qianchuan/bindings/:id/snapshots?since=&until=&granularity=
  GET    /api/v1/qianchuan/bindings/:id/actions?limit=&since=

Manual fire (useful for dry-run UX and on-demand check)
  POST   /api/v1/qianchuan/bindings/:id/fire
```

Error codes (FE renders localized text):

```
qianchuan:binding_not_found
qianchuan:advertiser_already_bound      ← UNIQUE (project, advertiser, aweme) violated
qianchuan:auth_expired                   ← refresh failed
qianchuan:oauth_state_invalid            ← consumed/expired/missing
qianchuan:strategy_parse_failed          ← yaml syntax / schema mismatch
qianchuan:strategy_not_found
qianchuan:policy_blocked                 ← action exceeded policy threshold
qianchuan:approval_required              ← action queued for human (not an error per se;
                                            returned by dry-run for FE preview)
qianchuan:upstream_rate_limited
qianchuan:upstream_unavailable
```

## 8. Provider-Neutral Interface

```go
// internal/adsplatform/provider.go
package adsplatform

import (
    "context"
    "time"
)

type Provider interface {
    ID() string  // 'qianchuan' | 'taobao' | ...

    // OAuth.
    OAuthAuthorizeURL(state, redirectURI string, scopes []string) string
    OAuthExchange(ctx context.Context, code, redirectURI string) (TokenSet, error)
    RefreshToken(ctx context.Context, refresh string) (TokenSet, error)

    // Data plane.
    FetchMetrics(ctx context.Context, t TokenSet, b BindingRef) (MetricSnapshot, error)
    ApplyAction(ctx context.Context, t TokenSet, b BindingRef, a Action) (ActionResult, error)
}

type TokenSet struct {
    AccessToken      string
    RefreshToken     string
    AccessExpiresAt  time.Time
    RefreshExpiresAt time.Time
    Scopes           []string
}

type BindingRef struct {
    AdvertiserID string
    AwemeID      string
}

type MetricSnapshot struct {
    BucketAt  time.Time              `json:"bucket_at"`
    Live      map[string]any         `json:"live"`       // viewers, gmv, ctr, ...
    Ads       []AdMetric             `json:"ads"`
    Materials []MaterialHealth       `json:"materials"`
    Raw       map[string]any         `json:"raw,omitempty"`
}

type AdMetric struct {
    AdID    string  `json:"ad_id"`
    Status  string  `json:"status"`
    Spend   float64 `json:"spend"`
    ROI     float64 `json:"roi"`
    CTR     float64 `json:"ctr"`
    Bid     float64 `json:"bid"`
    Budget  float64 `json:"budget"`
    CPM     float64 `json:"cpm"`
}

type Action struct {
    Kind      string         `json:"kind"`
    TargetRef string         `json:"target_ref"`  // ad_id or material_id
    Params    map[string]any `json:"params"`
}

type ActionResult struct {
    OK         bool           `json:"ok"`
    Detail     string         `json:"detail"`
    Provider   map[string]any `json:"provider,omitempty"`
}
```

Concrete `internal/qianchuan/provider.go` wraps the existing reference HTTP client patterns (URL builder for `api.oceanengine.com` / `ad.oceanengine.com`, retry on `51010/51011/40100` and Chinese-text retryables, signed `Access-Token` header) and translates Qianchuan's response shapes into `adsplatform.MetricSnapshot` / `ActionResult`. The translation layer (`internal/qianchuan/mapping.go`) is the only place Qianchuan-specific field names appear.

## 9. Strategy DSL

A strategy is a YAML document loaded the same way roles are loaded (`internal/role/parser.go` is the structural template). Schema:

```yaml
apiVersion: agentforge/v1
kind: QianchuanStrategy
metadata:
  name: "live-room-roi-guard"
  version: "v1"
  description: "Lower bid by 10% if ROI < 1.5 for 3 consecutive minutes; pause ad if ROI < 0.5"
inputs:
  # variable bindings extracted from snapshot for re-use in rules
  - name: roi_3m_avg
    expr: "ads[0].roi"          # reuses LookupPath; future: small avg() helper
when: "live.viewers > 100"      # gate; if false the whole strategy is no-op
rules:
  - name: roi-degradation
    when: "ads[0].roi < 1.5"
    actions:
      - kind: adjust_bid
        target: "ads[0].ad_id"
        params: { delta_pct: -10 }
      - kind: notify_im
        params:
          severity: "warn"
          summary: "ROI {{ads[0].roi}} below 1.5 — bid lowered 10%"
  - name: roi-collapse
    when: "ads[0].roi < 0.5"
    actions:
      - kind: pause_ad
        target: "ads[0].ad_id"
        params: { reason: "ROI < 0.5" }
```

**Evaluation** uses `nodetypes/EvaluateCondition` for `when:` and `nodetypes/ResolveTemplateVars` + `EvaluateExpression` for `expr:` / `target:` / template fields in `params:`. There is **no** dynamic code execution. Action `kind` values are a closed enum; unknown kinds reject at parse time. Expression scope is the metric snapshot only (no environment, no `process`, no filesystem).

## 10. End-to-End Data Flows

The single canonical DAG is `qianchuan_strategy_loop`:

```
schedule_trigger
  → http_call (FetchMetrics via Provider, write snapshot to dataStore + table)
  → function (load strategy, evaluate, emit []Action into dataStore.actions)
  → loop over actions:
       function (policy_gate.GateAction)
       branch:
         (a) pass        → http_call (ApplyAction) → record action_log row
         (b) block       → record action_log row (outcome='blocked_by_policy')
         (c) need_approve→ im_send (approval card) → wait_event → on resume:
                              http_call (ApplyAction) → record action_log row
  → im_send (summary card; only emitted if any action applied OR alert raised)
```

### Trace A — silent tick (no actions)

```
[scheduler] minute fires for binding B (advertiser=A1, aweme=W1)
[trigger.router] starts execution E with system_metadata.reply_target = nil  (schedule has none)
[Redis lock] af:qc:lock:B acquired (TTL 120s, tick=60s)

[http_call: fetch_metrics]
  resolve {{secrets.qianchuan.B.access_token}}
  GET https://api.oceanengine.com/open_api/v1.0/qianchuan/report/get
  resp 200 → mapping.go → adsplatform.MetricSnapshot{viewers:50, ads:[{roi:2.1}]}
  insert qianchuan_metric_snapshots(B, bucket=now-trunc-min, metrics)
  dataStore.snapshot = MetricSnapshot

[function: evaluate_strategy]
  strategy.when = "live.viewers > 100"  → false → emit []Action{}
  dataStore.actions = []

[loop: actions] iterations=0 → noop

[function: should_notify]
  len(dataStore.actions) == 0 && no alerts → false
  ⇒ short-circuit branch skips im_send

[execution complete] outbound_dispatcher: reply_target=nil → silent (Spec 1 §9 边界)
[Redis lock] released                                                            ✅
```

### Trace B — auto-applied action

```
[scheduler] minute fires for binding B
[http_call] fetch → snapshot{viewers:240, ads:[{ad_id:'AD7', roi:1.2, bid:5.0}]}

[function: evaluate_strategy]
  rule 'roi-degradation' when "ads[0].roi < 1.5" → true
  emit Action{kind:'adjust_bid', target_ref:'AD7', params:{delta_pct:-10}}
       Action{kind:'notify_im',  params:{severity:'warn', summary:'...'}}

[loop iter 1: adjust_bid]
  [policy_gate] policy.max_bid_change_pct = 20; |delta|=10 ≤ 20 → pass
  [http_call: apply] POST .../qianchuan/ad/update {ad_id:AD7, bid:4.5}
                     resp 200 → ActionResult{OK:true}
  insert qianchuan_action_logs{outcome:'applied', request, response}

[loop iter 2: notify_im]
  policy_gate noop for notify_im (always pass)
  insert action_log{outcome:'applied', request:{summary}}, no upstream call

[im_send: summary card]
  ProviderNeutralCard{
    title:'qianchuan binding "店铺A 主直播间" — 1 action applied',
    status:'success',
    summary:'ROI 1.2 < 1.5; bid AD7 4.50→4.50 (-10%)',
    fields:[{label:'binding', value:B.display_name}, {label:'tick', value:bucket_at}],
    footer:exec_id,
    actions:[{type:'url', label:'查看运行', url:'/employees/E/runs/<exec_id>'}]
  }
  POST IM Bridge /im/send {chat_id: binding.notify_chat_id, card}
  execution.system_metadata.im_dispatched = true   (Spec 1 §9)

[execution complete] outbound_dispatcher: im_dispatched=true → skip default
```

### Trace C — human approval required

```
[scheduler] minute fires for binding B
[http_call] fetch → snapshot{ads:[{ad_id:'AD7', roi:0.4}]}

[function: evaluate_strategy]
  rule 'roi-collapse' when "ads[0].roi < 0.5" → true
  emit Action{kind:'pause_ad', target_ref:'AD7', params:{reason:'ROI < 0.5'}}

[loop iter 1: pause_ad]
  [policy_gate] policy.require_human_approval_for includes 'pause_ad'
                → return need_approve{reason:'human_approval_required'}
  [im_send: approval card]
    create card_action_correlations: token T1 (approve), T2 (reject) for wait_event node
    card.actions = [
      {id:'approve', type:'callback', correlation_token:T1,
       payload:{action_kind:'pause_ad', target:'AD7'}},
      {id:'reject',  type:'callback', correlation_token:T2, payload:{}}
    ]
    POST IM Bridge → operator's Feishu group
    execution.system_metadata.im_dispatched = true
  [wait_event] park execution (status='waiting')
  insert action_log{outcome:'approval_required', request}

[Feishu user clicks "Approve"]
  IM Bridge inbound card_action → POST /api/v1/im/card-actions
  card_action_router.Lookup(T1) → hit, not consumed → wait_event_resumer.Resume(...)

[loop iter 1 resumes]
  branch (c) continues:
  [http_call: apply] POST .../qianchuan/ad/update {ad_id:AD7, status:'STATUS_DELETE'}
                     resp 200
  update action_log id=...: outcome='approved' approved_by=user_id

[im_send: summary card]
  status:'success', summary:'ROI 0.4 < 0.5; AD7 paused (approved by @user)'
[execution complete]                                                             ✅
```

If the user clicks Reject, the resumer routes to a `function` node that updates the action_log row to `outcome='rejected'` and skips the apply step.

## 11. Error Handling

| 场景 | 行为 |
|---|---|
| Token refresh 失败 (4xx) | binding.status='auth_expired'; 通过 admin 通知通道发 Feishu 卡片 (`severity:'error'`) 给 binding.created_by; 暂停所有以该 binding 为 trigger 的 schedule fire (Redis lock + scheduler skip) |
| Token refresh 失败 (5xx / 网络) | exponential backoff (1m / 5m / 15m), 三次后视同 4xx 走上面分支 |
| Qianchuan 速率限制 (40100) | binding 级 token bucket (本进程内 sync.Map)；trigger 自身的 dedupe_window 已经按 tick 间隔自然限速；额外触发时 http_call 节点返回 `qianchuan:upstream_rate_limited`，本 tick 失败但下 tick 正常 |
| Qianchuan 临时故障 (51010/51011/系统繁忙文案) | Provider 内部 retry 3 次（指数退避）；超过 → 节点 fail，execution status=failed，action_log 不写入 (这一 tick 没有快照即没有动作) |
| 网络 timeout | 默认 15s；超时 → 同上视为临时故障 |
| Strategy 表达式错误（解析期）| YAML 提交时拒绝；`qianchuan:strategy_parse_failed` 含错误位置 |
| Strategy 表达式错误（运行期，比如取了不存在的字段）| `EvaluateCondition` 返回默认 true 是 Spec 1 引擎的既有契约；本 spec 在 evaluator wrapper 中改为：未解析的 `{{...}}` 模板存在 → 整条 rule 视为 false 并写一行 `action_log{kind:'record_event', outcome:'noop', detail:'expression_unresolved:<path>'}`；不让模糊行为放过 |
| Policy gate 阻断 | action_log{outcome:'blocked_by_policy', outcome_detail:'max_bid_change_pct exceeded: requested 30, allowed 20'}；不通知（避免 noisy）；FE actions 表显式可见 |
| ApplyAction 4xx | action_log{outcome:'failed'}；继续后续 actions（不中止整轮 loop）；summary card 在 fields 中标红 |
| ApplyAction 5xx | retry once；二次失败 outcome:'failed'；同上 |
| 卡片 callback token 过期 / 已消费 | Spec 1E 既有路径：IM Bridge 410/409；Feishu toast"操作已过期 / 已处理" |
| Wait_event 等待超时 (> 24h 无人点) | wait_event 节点 `timeout_seconds: 86400`；超时分支写 action_log{outcome:'rejected', detail:'approval_timeout'} 并继续 |
| Redis lock 不可用 | `trigger.ErrIdempotencyStoreUnavailable` 抬起；scheduler 跳过本 tick 并 emit 一次 admin 告警；不 silently fall through (与 Spec 1 §一致：fail closed) |

## 12. Security

- **OAuth state**：CSRF nonce 写入 `qianchuan_oauth_states`，10 分钟 TTL，单次消费；callback 校验 state、redirect_uri 一致、未过期、未消费；任一失败 400 + `qianchuan:oauth_state_invalid`。
- **Token 加密**：access_token / refresh_token 仅以 secrets 表的密文形式落盘；binding 行只持有 secret 名称引用。明文生命周期与 Spec 1B §11 等同（仅 Resolve 调用栈 → outbound HTTP req 之间）。token refresh goroutine 在内存维护一个 ≤10min TTL 的解密缓存以避免每次 fetch 都做一次 GCM 解密——缓存随进程终止消失，不写盘。
- **Action 审计**：每个动作写一行 `qianchuan_action_logs`，含 request / response / outcome / approved_by；FE 在 binding 详情页提供完整审计视图；导出 CSV 留作未来运维需求（不在本 spec 范围）。
- **IP 白名单（可选）**：FE 在 binding policy 中可设置 `apply_only_from_ip_whitelist: [...]`；http_call 节点在出站前比对本机出口 IP（通过 `STUN`-like 探测或环境变量 `BACKEND_EGRESS_IP`），不匹配则视同 policy 阻断。默认空列表 = 不限制。
- **RBAC**：所有 `/api/v1/qianchuan/...` 端点走现有 `RequireProjectRole`：
  - `viewer` 可读 bindings / strategies / snapshots / actions
  - `editor` 可创建/修改 strategy、调整 binding policy 与 strategy_id
  - `admin` 可执行 OAuth bind / 删除 binding / pause / resume
  - `owner` 同 admin
- **secrets.X 引用**：仅 Spec 1B 的 HTTP 节点白名单字段允许 `{{secrets.X}}`；strategy YAML 里出现 `{{secrets.*}}` 在 parse 时拒绝（`qianchuan:strategy_parse_failed`），避免作者意外把策略写成 token 泄漏向量。

## 13. Testing Strategy

```
Unit Go
  ├─ adsplatform.Provider mock + registry round-trip
  ├─ qianchuan/mapping: snapshot + action result fixtures (golden JSON)
  ├─ qianchuan/oauth: state CSRF, expiry, double-consume
  ├─ qianchuan/client: retry on 51010/51011, on 429, on Chinese-text retryables
  ├─ strategy/manifest: parse → 8 fixture YAMLs (good + 4 bad)
  ├─ strategy/engine: snapshot × strategy × expected actions matrix (12 cases)
  ├─ strategy/policy_gate: pass / block / need_approve × 3 action kinds
  ├─ token_refresher: scheduling + 3-attempt backoff + auth_expired transition
  └─ binding repo: UNIQUE conflict; status filter index used (EXPLAIN)

Integration Go (PG + Redis + IM Bridge mock)
  ├─ Trace A: schedule fires → snapshot persisted → no action → no IM
  ├─ Trace B: schedule fires → 1 action applied → action_log row + IM card sent
  ├─ Trace C: pause_ad → policy holds for approval → IM card → callback → resume → applied
  ├─ Token refresh storm: 100 bindings expiring in same minute → goroutine rate-limits
  └─ Auth expired propagation: refresh fails → all triggers for binding stop firing within 1 tick

E2E smoke (extends src-im-bridge/scripts/smoke/)
  ├─ qianchuan-tick-silent.json
  ├─ qianchuan-tick-auto-apply.json
  └─ qianchuan-tick-approval-flow.json

FE Jest
  ├─ /employees/:id/qianchuan: bindings list + chart loading + actions table + WS update
  ├─ /projects/:id/qianchuan/strategies: YAML editor + schema-validate + dry-run preview
  └─ /projects/:id/qianchuan/bindings: OAuth init → callback redirect → list refresh

Manual QA checklist (because we can't hit Qianchuan from CI)
  ├─ Real OAuth bind in staging Qianchuan account
  ├─ One adjust_bid action applied + verified in Qianchuan console
  └─ One approval-required pause + verified
```

## 14. Spec Drifts (placeholder for plan-writing surprises)

To be filled by the plan-writer subagents per the same convention Spec 1 §13.1 uses. Currently expected drifts:

- The reference project's `EvaluateCondition` returns `true` on unrecognized expressions. This is *unsafe* for strategy authoring and Spec 3 needs an evaluator wrapper that rejects unresolved templates. If the wrapper turns out to require touching `nodetypes/expr.go` directly (rather than a thin shim), record here.
- The Qianchuan reference project parses very large numeric IDs (room_id, order_id) with a regex-based `safeJsonParse` to avoid JS Number precision loss. Go's `encoding/json` decodes numeric values into `float64` by default, which has the same precision ceiling. The Provider implementation must use `json.Number` or strict struct fields with `string` tags for ID fields; if this surfaces only during plan writing, document the fix here.
- Reference project uses Bun.cron at minute resolution. Spec 3 reuses Spec 1's existing scheduler (`internal/scheduler/`) instead. If that scheduler doesn't support sub-minute cadence (default 60s but range allows down to 10s), the plan must enable that — record actual gap here.

### 14.1 Plan 3A drifts (landed)

- **Binding row scope**: Plan 3A persists only the columns owned by binding lifecycle: `id`, `project_id`, `advertiser_id`, `aweme_id`, `display_name`, `status`, `acting_employee_id`, `access_token_secret_ref`, `refresh_token_secret_ref`, `token_expires_at`, `last_synced_at`, `created_by`, `created_at`, `updated_at`. The spec §6.1 schema additionally lists `policy`, `strategy_id`, `tick_interval_sec`, `trigger_id`; those columns are owned by Plan 3C (strategy CRUD) and Plan 3D (workflow loop) and will be added via `ALTER TABLE` migrations.
- **`employee_id` → `acting_employee_id`**: Spec §6.1 reads `employee_id NOT NULL`; Plan 3A persists `acting_employee_id UUID NULL` to align with the existing AgentForge convention used by `vcs_integrations` (072) and the workflow attribution guard. A binding may have no acting employee at creation time; the FE attaches one later.
- **No body-HMAC signing**: Spec §A3 phrasing implies "signed requests"; the upstream Qianchuan OpenAPI uses bearer-token auth (`Access-Token: <token>` header) with no body-HMAC, matching the reference project. The Provider implementation reflects bearer-token only.
- **Big-int IDs decoded with `json.Number`**: room_id / order_id / advertiser_id come back as 16+ digit numbers from Qianchuan. The client uses `json.Decoder.UseNumber()` and the mapping helpers stringify via `asString` so the precision is preserved end-to-end. Neutral structs (`AdMetric.AdID`, `LiveSession.RoomID`) carry these as `string`.
- **Migration number**: Plan 3A reserved 071 (after 070 qianchuan_strategies = Plan 3C). The audit `resource_type` CHECK list now includes `qianchuan_binding`; Plan 2A's migration 072 preserves both `qianchuan_binding` and `vcs_integration` in its replacement CHECK so applying 072 after 071 stays coherent.
- **Action surface bounded**: Provider exposes exactly 5 mutating primitives (AdjustBid, AdjustBudget, PauseAd, ResumeAd, ApplyMaterial). No raw `RawCall` / passthrough method exists. Adding a 6th remains a code-review gate per §11.
- **Integration-test deferred**: the `internal/testdb` helper does not yet exist on master. Plan 3A's GORM repo is exercised by the in-memory contract test only; round-trip integration test deferred until a testdb harness lands.
- **Project sub-nav deferred**: there is no shared `app/(dashboard)/projects/[id]/layout.tsx` on master, so the FE bindings page is reachable by direct URL only. Plan 1B's secrets page has the same constraint; both pages will gain sidebar entries when a project sub-nav layout lands.

## 15. Open Risks

- **Compaction job deferred**: `qianchuan_metric_snapshots` grows at 1 row/binding/minute = ~525k rows/binding/year. With 100 bindings the table is at ~50M rows in a year. PG will handle that with the existing index but query latency on the FE charts will start to hurt around the 6-month mark. Mitigation plan: a v1.1 worker that rolls minute → 5-minute (after 7 days) → hourly (after 90 days) buckets and prunes the source rows. **Not in v1**; the partition-friendly schema (always queried by `binding_id, bucket_at DESC`) makes the migration straightforward later.
- **Strategy expression injection**: even though we don't run JS, `EvaluateExpression` accepts arbitrary strings and the future-extension `len(path)` style may grow. Risk: an author writes a strategy that, via clever expression, exfiltrates fields the engine wasn't intended to expose. Mitigation: the evaluator scope is the snapshot only, and we explicitly forbid `{{secrets.*}}` / `{{system_metadata.*}}` references at parse time. Long term, consider a CEL or Expr-lang adoption with an explicit allowlist.
- **Qianchuan API breaking changes**: Qianchuan ships breaking changes to OpenAPI without long deprecation windows. Mitigation: every external response goes through `mapping.go`, and we keep a recorded golden response per supported endpoint as a fixture. CI doesn't catch this; on-call must.
- **OAuth refresh storm**: if 1000+ bindings expire in the same minute, the refresher hits Qianchuan's auth endpoint 1000 times. Mitigation: refresher uses a token bucket (default 5 req/sec) and spreads work across the next N minutes when the queue is large; bindings whose refresh slot lands after their access_expires_at fail soft and mark `auth_expired`.
- **Wait_event timeout default (24h)** is generous on purpose but leaves "approval cards" lingering in Feishu. If operator load grows we may need to age these out faster and cancel the workflow with a `cancelled` status. Tracked here as a future toggle on per-binding policy.
- **IP whitelist option** (§12) is documented but mostly aspirational in v1 — discovering the egress IP cleanly across web / desktop / Tauri sidecar contexts is non-trivial. If staging proves it's needed, expand the spec.

## 16. Successor / Future Work Hooks

- **Multi-platform**: `internal/taobao/`, `internal/jd_cloud/`, `internal/kuaishou/`, `internal/tiktok_ads/` — each implements `adsplatform.Provider`; the canonical workflow shape, the strategy DSL, and the FE surfaces stay unchanged. Each new provider's auth model (OAuth1, JWT-signed, or API key) determines how OAuth start/exchange specialize.
- **JS-sandbox strategy escape hatch**: for the small population of authors who really need imperative logic, ship a `kind: ScriptedStrategy` resource backed by a hardened sandbox (Goja with no globals, no module loader, fixed memory budget, hard 50ms wall-clock). YAML strategies remain the recommended default.
- **Strategy backtester**: snapshot table + a deterministic strategy engine = backtest is "read snapshots from window, replay through engine, count actions, compute counterfactual ROI". Surface as `POST /api/v1/qianchuan/strategies/:id/backtest`.
- **Compaction job** (see §15) and **partition-by-month** of the snapshots table.
- **Material upload pipeline**: drag-and-drop creative upload → Qianchuan asset → reference by ID in `apply_material` actions.
- **Cost integration**: pipe the `ads.spend` field from snapshots into AgentForge's existing cost-tracking surface so leaders can see "this employee burned ¥120k this month and saved ¥30k via auto-pause".
- **Live-stream room state machine**: the reference project tracks live-room state transitions (warming → live → ending). v1 ignores this; future work can drive different strategies per phase.
