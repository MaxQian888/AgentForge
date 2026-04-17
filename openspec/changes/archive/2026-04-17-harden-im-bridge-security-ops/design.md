## Context

IM Bridge 是 AgentForge 与外部 IM 平台之间的签名网关。当前安全/运维面在"能工作"和"能运维"之间有明显 gap（见 proposal.md 的 Why）。这份 design 落实 six 个加固点的具体形状：持久化 dedupe、时间戳窗口、结构化审计、多维限速、出站净化、命令 allowlist、凭据热重载。

核心原则：
- **所有加固都作为可关闭开关**，缺省保持当前行为 or 稍微更严；通过 env 翻开。
- **持久化不依赖外部服务**：SQLite 文件 + 本地 JSONL。Bridge 本来就是 sidecar 进程，不再引入 Redis/Postgres 强依赖。
- **契约对后端 forward-compatible**：签名协议/header 不变，仅新增可选字段。
- **尊重 PRD 8.7 安全模型的 bridge 端面**：签名+幂等+审计+限速是 bridge 侧的责任边界；RBAC/secrets vault 是后端/平台面的责任。

## Goals / Non-Goals

**Goals**
1. 重启/多副本场景下 `deliveryId` 幂等成真。
2. 合法签名请求的重放窗口有明确时间上限。
3. 运维可以通过结构化审计流追溯每一次下发/回调/action 的来龙去脉。
4. 限速可以按 tenant、chat、user、action_class、command 多维组合。
5. 出站文本在不同 provider 有一致的净化语义（广播提及剔除、长度分段、零宽清理）。
6. 命令灰度可以在 bridge 侧前置（不是每次都要回到后端 RBAC）。
7. 凭据轮转不再强制全进程重启。

**Non-Goals**
- 不做 provider 级的 at-rest 加密。
- 不做 bridge 侧完整的 secrets vault（仅暴露接口让未来可对接）。
- 不做 per-user fine-grained RBAC 在 bridge 侧；继续依赖后端。
- 不改 HMAC 算法或签名字段布局（会破坏所有已部署控制面）。
- 不合并 dedupe/audit 到外部服务（Redis/Postgres）——bridge 作为 sidecar 应保持本地可运行。

## Decisions

### Decision 1：持久化存储用 SQLite，不用外部 KV

**选项**
- A. SQLite 本地文件（选中）
- B. 外部 Redis
- C. BoltDB / BadgerDB

**选择 A 的理由**
- Bridge 是 sidecar，运行拓扑不保证有 Redis。
- SQLite 的 WAL 模式足够 dedupe + rate 的并发量级（峰值 IM 事件 << 1000 QPS/bridge）。
- Go 生态成熟（`modernc.org/sqlite` 纯 Go，不依赖 CGO）。
- 查询语言方便做 TTL 清理（`DELETE WHERE created_at < ?`）。
- 文件位置可以通过 `IM_BRIDGE_STATE_DIR` 挂盘，K8s 可用 emptyDir，桌面可用户目录。

**拒绝 Redis 的理由**
- 在 Tauri 桌面 + Compose web 的混合拓扑下，强依赖 Redis 会让桌面场景部署成本抬高。
- PRD 认为 IM Bridge 应 "no public exposure"，不应把 Redis 也暴露给 bridge。

**Schema（单文件，三张表）**
```sql
CREATE TABLE dedupe (
  delivery_id TEXT PRIMARY KEY,
  surface TEXT NOT NULL,               -- /im/send | /im/notify | control_plane
  created_at INTEGER NOT NULL,         -- unix seconds
  expires_at INTEGER NOT NULL          -- created_at + ttl
);
CREATE INDEX dedupe_expires ON dedupe(expires_at);

CREATE TABLE nonce (
  nonce TEXT NOT NULL,
  scope TEXT NOT NULL,                 -- platform or 'control_plane'
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  PRIMARY KEY(nonce, scope)
);
CREATE INDEX nonce_expires ON nonce(expires_at);

CREATE TABLE rate (
  scope_key TEXT NOT NULL,             -- hashed composite key, see Decision 4
  policy_id TEXT NOT NULL,
  occurred_at INTEGER NOT NULL,
  PRIMARY KEY(scope_key, policy_id, occurred_at)
);
CREATE INDEX rate_eviction ON rate(policy_id, occurred_at);
```

清理由后台 goroutine 每 30s 执行 `DELETE WHERE expires_at < ?` / `DELETE WHERE occurred_at < ?`。

### Decision 2：时间戳窗口对齐 dedupe TTL

窗口 `IM_SIGNATURE_SKEW_SECONDS`（默认 300s）决定了：
- 入站请求必须 `|now − timestamp| ≤ skew`，否则 408 `timestamp_out_of_window`。
- Dedupe 记录 TTL = `skew + grace (60s)`，窗口关闭后同 deliveryId 即使能 replay 也会被 timestamp 挡掉，不靠 dedupe。
- Dedupe 记录不需要永久保留，每条记录寿命 ≈ 6 分钟，总存储量级可控（即使 1000 QPS × 6 min = 360K 条 × 小行宽 ≈ 20MB）。

状态机：

```
         bodyBytes+headers
                 │
                 ▼
       ┌─────────────────────┐
       │ HMAC 计算 / 比对    │
       └──────┬──────────────┘
              │ valid
              ▼
       ┌─────────────────────┐  out of window
       │ 时间戳窗口校验      │────────────────▶ 408
       └──────┬──────────────┘
              │ in window
              ▼
       ┌─────────────────────┐  hit
       │ SQLite dedupe 查   │────────────────▶ 200 {status:"duplicate"}
       └──────┬──────────────┘
              │ miss
              ▼
       ┌─────────────────────┐
       │ INSERT (deliveryId) │
       │ 处理业务             │
       └─────────────────────┘
```

### Decision 3：审计用 JSONL 本地文件，控制面回传为可选

**为什么是 JSONL 不是 log 包扩展**
- logrus 已经是半结构化，但审计事件 schema 要稳定可消费（外部 log collector、`filebeat`、运营查询）。
- 专属文件便于做轮转、保留策略、独立于业务日志的权限。

**文件布局**
- `${IM_BRIDGE_AUDIT_DIR}/audit.jsonl` — 当前写入文件。
- 按 `IM_AUDIT_ROTATE_SIZE_MB`（默认 128MB）或每日零点滚动 → `audit.YYYY-MM-DD-HHMM.jsonl`。
- `IM_AUDIT_RETAIN_DAYS`（默认 14）之外的文件清理。

**事件 schema**
```json
{
  "v": 1,
  "ts": "2026-04-17T12:34:56.789Z",
  "direction": "ingress" | "egress" | "action" | "internal",
  "surface": "/im/send" | "/im/notify" | "/im/action" | "control_plane" | "inbound_callback",
  "deliveryId": "<uuid-like>",
  "platform": "feishu",
  "bridgeId": "<uuid>",
  "chatIdHash": "<hex>",            // SHA-256(chatId + salt)[0:16]
  "userIdHash": "<hex>",
  "action": "/task create" | "task_completed" | "decompose" | ...,
  "status": "delivered" | "duplicate" | "rejected" | "rate_limited" | "downgraded" | "failed",
  "deliveryMethod": "send" | "reply" | "edit" | "deferred_card_update" | "...",
  "fallbackReason": "<string-or-empty>",
  "latencyMs": 123,
  "signatureSource": "shared_secret" | "unsigned",
  "metadata": {
    "sanitize_warnings": ["broadcast_mention_stripped"],
    "rate_policy": "write:10/min",
    "..."
  }
}
```

**盐值** `IM_AUDIT_HASH_SALT`（若未设置，bridge 启动生成并持久化到 state.db），防止 chatId/userId 的 hash 被跨部署相关联。

**控制面回传（可选）**：`client.ControlDeliveryAck.AuditEvent` 字段在 ack 里嵌入当次 event，后端若已实现消费端则记录入 history；未实现端忽略即可（forward-compatible）。开关：`IM_AUDIT_SHIP_VIA_CONTROL_PLANE=true|false`（默认 false，仅本地）。

### Decision 4：多维限速 policy

**类型**
```go
type RateDimension string
const (
  DimTenant      RateDimension = "tenant"       // projectId or im-user identity binding
  DimChat        RateDimension = "chat"
  DimUser        RateDimension = "user"
  DimCommand     RateDimension = "command"      // e.g. "/task"
  DimActionClass RateDimension = "action_class" // "read"|"write"|"destructive"
  DimBridge      RateDimension = "bridge"
)

type RateLimitPolicy struct {
  ID             string
  Dimensions     []RateDimension
  Rate           int
  Window         time.Duration
  Description    string
}

type Scope struct {
  Tenant, Chat, User, Command, ActionClass, Bridge string
}

type Decision struct {
  Allowed       bool
  Policy        string // id of the policy that denied
  RetryAfterSec int
}
```

**默认 policy 集合**（可被 `IM_RATE_POLICY` JSON 覆盖）：

| ID | Dimensions | Rate/Window | 作用 |
|----|-----------|-------------|------|
| `session-default` | chat+user | 20/min | 保持当前全局限流语义 |
| `write-action` | user+action_class=write | 10/min | 写操作（/task create, /agent spawn 等）防刷 |
| `destructive-action` | user+action_class=destructive | 3/min | 删除/kill 类二次约束 |
| `per-chat` | chat | 60/min | 群级总量闸门 |

**命令到 action_class 的映射**（命令 catalog 里静态声明）：
- `/task create/decompose/assign/move` → write
- `/task list/status` → read
- `/agent run/spawn/resume` → write
- `/agent pause/kill` → destructive
- `/tools install/uninstall/restart` → destructive
- `/review` → 根据子命令细分
- 未声明 → `read`（保守）

Engine 路由时在解析命令后、执行前调用 `RateLimiter.Allow(ctx, scope)`，被拒 → `Decision.Policy` 进审计 event。

### Decision 5：出站净化策略

**三级**
- `off`：透明放行（调试/兼容场景）。
- `permissive`（未来扩展）：只剔除零宽字符。
- `strict`（默认）：全面净化。

**规则**（`strict`）
| 规则 | 行为 |
|------|------|
| Broadcast mention | 剔除 `@everyone` / `@here` / `@all` / 平台特定（Slack `<!channel>`、Telegram `@channel`）→ 换成可见明文 `[广播已屏蔽]` |
| 长度 | 按 provider `RenderingProfile.TextLengthLimit` 截断；若设置 `RenderingProfile.SegmentOversized=true`（Telegram 已有）则分段 |
| 零宽 | 剔除 `U+200B/200C/200D/FEFF` |
| 控制字符 | 剔除 `U+0000-001F` 除 `\n\t\r` 外 |

**集成点**：`core.deliverRenderedText` 与 `core.DeliverText` 在写出前调用；净化后的 `[]Warning` 进 `DeliveryEnvelope.Metadata["sanitize_warnings"]`（以 `|` 分隔），再进审计。

**关于富消息（card/native）**：本期仅处理 plain text path。Structured/Native payload 内嵌的文本先不做净化，由 provider-specific renderer 决定（未来 change A 再对齐）。

### Decision 6：命令 allowlist 格式

**语法**：`IM_COMMAND_ALLOWLIST="<entry>,<entry>,..."`
- `<entry>` = `<platform-or-*>:<command-or-*>`，例如：
  - `*:*` — 允许所有（等同未设置）
  - `feishu:/task,feishu:/help` — 飞书仅放 /task /help
  - `slack:/*` — Slack 全放
  - `feishu:!/admin` — 前缀 `!` = 拒绝（黑名单）；先匹配 deny，后匹配 allow，都不中则默认 allow（未设置时完全不拦）

**未授权行为**：bridge 侧直接回 `"该命令在此平台未启用，请联系管理员。"`，不进入后端 round-trip。审计 event `status=rejected`, `metadata.rate_policy=command_allowlist`。

### Decision 7：凭据热重载

**触发**：`SIGHUP`（Unix）/ Windows Service Control（桌面场景暂缓，打印 `manual_restart_required`）。

**流程**
```
  SIGHUP
    │
    ▼
  重新 loadConfig()  ──▶  计算 cfg diff
                          │
                  ┌───────┴─────────┐
                  │                 │
        无变化：忽略           有变化：
                                  │
                                  ▼
                          provider.Reconcile(ctx, newCfg)
                                  │
                          ┌───────┼────────┐
                          ▼       ▼        ▼
                     credential   transport   webhook
                     rotate       reconnect   port (不支持 → 警告)
```

**接口扩展** `core.Platform`（新增可选接口）：
```go
type HotReloader interface {
  Reconcile(ctx context.Context, cfg ReconcileConfig) ReconcileResult
}

type ReconcileConfig struct {
  Credentials map[string]string
  // 未来扩展：RenderingProfile 覆写、capability 开关
}

type ReconcileResult struct {
  Applied    []string   // 字段名列表
  Deferred   []string   // 需要手动重启的字段
  Errors     []error
}
```

provider 可选实现；不实现 = 收到 SIGHUP 时直接打印 `hot_reload_unsupported` warning。

**先实现的 provider**：feishu long-connection（可重连）、slack socket mode（可重连）、telegram long-poll（可重换 token）、qq onebot（ws 可重连）。
**暂不实现**：依赖 callback port 的（wecom/qqbot/wechat/discord）——打印 `manual_restart_required` 并记录审计。

## Architecture diagram

```
┌──────────────────── IM Bridge Process ─────────────────────┐
│                                                            │
│   SIGHUP ──▶ HotReloader.Reconcile  ─────┐                 │
│                                          ▼                 │
│   ┌──── notify.Receiver ────┐   ┌── provider.Platform ──┐  │
│   │  HTTP signature gate    │   │  Reconcile()          │  │
│   │  ├─ HMAC verify         │   │  Send/Reply/...       │  │
│   │  ├─ skew window check   │   └──────────┬────────────┘  │
│   │  ├─ dedupe (SQLite)     │              │               │
│   │  ├─ rate policy check   │              ▼               │
│   │  └─ audit emit ─────────┼──────▶ audit.Writer          │
│   └──────────┬──────────────┘             │                │
│              ▼                             │  JSONL append  │
│   ┌── core.Engine ──────────┐              ▼                │
│   │  allowlist gate         │     state.db / audit.jsonl    │
│   │  rate policy (command,  │                               │
│   │    action_class)        │                               │
│   │  egress sanitize        │                               │
│   └──────────┬──────────────┘                               │
│              ▼                                              │
│   ┌── core.DeliverEnvelope ────┐                            │
│   │  SanitizeEgress(...)  ─────┼─▶ Warning[] ─▶ audit.emit  │
│   │  DeliverText/Native/...    │                            │
│   └────────────────────────────┘                            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
         ▲
         │ control plane ack (with optional AuditEvent)
         │
         ▼
   backend (im_control_plane)
```

## Alternatives considered

- **把 dedupe 放后端 control plane**：后端已经有 settlement history，但去 `/im/send` 那条"前置幂等"放不到后端——它是 bridge 入口，只能在 bridge 本地做第一道。后端仍会做自己的 idempotency（那是另一层），两者叠加防御没问题。
- **用 rotating log 写审计**：第一版考虑直接扩展 logrus 到带 hook 的 structured JSONL。放弃是因为 logrus 的 formatter 和轮转策略与审计需求混在一起会相互干扰；专属 writer 简单清晰。
- **限速 policy 用 DSL/表达式**：例如 `user AND command=="/task" AND ratelimit(10, 1m)`。放弃是因为当前只需覆盖四五个场景，list 形式足够；若未来扩展到十几个再引入 DSL。
- **allowlist 放到后端返回 deny**：放弃是因为目标就是"bridge 侧前置"减少后端压力；allowlist 本身就是粗粒度灰度工具，不冲突后端 RBAC。

## Risks

| 风险 | 触发条件 | 缓解 |
|------|---------|------|
| 审计 JSONL 涨爆磁盘 | 高峰期事件 + 保留过长 | 默认 128MB 轮转 + 14 天清理；env 可调 |
| SQLite busy 阻塞 notify 路径 | 高并发 write 碰撞 | WAL 模式 + `busy_timeout=5s`；dedupe 检查写入放到 goroutine 异步落盘，但关键路径 INSERT 阻塞 ≤ 5s |
| 时间戳 skew 与后端/bridge 时钟漂移 | NTP 失效 / 容器时区错 | 默认 300s 窗口容忍大多数漂移；bridge 启动时与后端 `/api/v1/health` 对时记录 delta 做诊断字段 |
| SIGHUP 热重载重连失败 | provider 新凭据无效 | Reconcile 返回 error 时保留旧 transport 在线，写审计 `status=rotate_failed` + 操作者 console 告警 |
| 净化误伤合法广播 | 运维确实想 `@channel` 推送 | 净化受 `IM_SANITIZE_EGRESS=permissive` 控制；命令侧可传 `metadata.allow_broadcast=true` 显式豁免（下一期再做） |
| 持久化 nonce/dedupe 抹平后端期望 | 后端若在 skew 窗口内重试 | 对 bridge 而言这是预期行为：duplicate 就是返回 `{status:"duplicate"}`；后端看到这个 ack 应视为已交付 |

## Migration

- **Phase 1：持久化 dedupe + 时间戳窗口**（foundation，所有其他项目依赖这个）。默认开启，env 缺省用 `.agentforge/state.db`。已有部署启动时会看到空表 + 第一次请求正常走流程，无需数据迁移。
- **Phase 2：审计 + sanitize**。默认开启 strict sanitize；审计 writer 本地化。
- **Phase 3：多维限速 + allowlist**。限速切换到 policy API（删除旧 `Allow(key string)`）；allowlist 默认未设 = `*:*`。
- **Phase 4：SIGHUP 热重载**。可选实现，不 block 其他 phase。

Rollback：每个 phase 都有 env 开关（`IM_DISABLE_DURABLE_STATE=true` 等），异常时回到内存行为。

## Open questions

1. `IM_AUDIT_HASH_SALT` 生成策略：首次启动生成持久化到 `state.db`；若运维要跨实例对齐 hash 用于联合查询，需提供手动设置入口。暂定为首次自动 + 可覆盖。
2. 控制面 `ack.audit_event` 的后端消费落在 `im-bridge-control-plane` 还是新 `im-bridge-audit-trail` capability 的后端 mirror？倾向后者，保持 control-plane 的职责聚焦。
3. `action_class` 是 bridge 侧静态 map 还是由后端命令 catalog 下发？第一版 bridge 侧静态，后续若 marketplace 引入自定义命令再转为 catalog 下发。
