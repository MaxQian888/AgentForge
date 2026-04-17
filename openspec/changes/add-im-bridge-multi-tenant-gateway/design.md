## Context

这份 change 把 IM Bridge 从 "single-active-provider + single-project" 的 sidecar 升级为 "gateway"：同一进程跑多 provider，同一 provider 承载多 tenant，会话/历史持久化，命令插件化。这是 openclaw 的 architecture baseline，AgentForge 要走到企业多租户部署就绕不开。

设计原则：
- **把 "多" 引入为一等公民而不是事后套壳**：现在的 `selectProvider() → activeProvider` 返回单值；本 change 重写为 `selectProviders() → []*activeProvider`，每个 provider 有自己完整的上下文。不做 "进程 ID 散列 + 进程间共享" 的复杂做法。
- **Tenant 是路由键，不是承载容器**：IM Bridge 不持有 tenant 业务状态（那在后端），只需要在入站路径把消息打上 `TenantID` 标签、在出站路径把 delivery 按 `TenantID` 选择凭据。
- **Plugin registry 复用 marketplace 的分发链路**：不新造 plugin 分发机制，直接接入 marketplace；安装 marketplace plugin 时如果 manifest 声明 `im_commands`，bridge 的 plugin dir 就会收到对应 YAML。
- **避免 session "强一致"**：第一版只做单副本 persistent state；跨副本共享留到后续（需要外部 KV 时再说）。

## Goals / Non-Goals

**Goals**
1. 单一 bridge 进程同时承接 feishu + dingtalk + wecom 三家平台。
2. 同一 IM 群里不同用户可触发不同 tenant 的 project 命令，互不交叉。
3. 重启后 NLU history / intent cache / reply target binding 继续可用。
4. 自定义命令（`/jira`、`/deploy`）通过 YAML plugin 注入 bridge 而不需要改 Go 代码。
5. Marketplace 已有的 plugin 分发 + 权限机制天然覆盖 IM 命令插件。

**Non-Goals**
- 不做 multi-bridge 之间 session 同步。
- 不做 plugin 的 WASM 沙箱（复用 marketplace 的 HTTP/MCP 调用隔离模型）。
- 不在 bridge 侧做 tenant 自服务管理（动态注册/下线）。
- 不对 IM 账户做跨 tenant 复用（一个 IM user 可以在多个 tenant 里，但凭据走后端 `im_user_binding`）。

## Decisions

### Decision 1：Multi-Provider runtime

**配置模型**
```
IM_PLATFORMS=feishu,dingtalk,wecom
IM_TRANSPORT_MODE_FEISHU=live
IM_TRANSPORT_MODE_DINGTALK=live
IM_TRANSPORT_MODE_WECOM=stub

# 每 provider 凭据使用既有 env（FEISHU_APP_ID / DINGTALK_APP_KEY / WECOM_CORP_ID 等）不变
```

**Runtime 结构**
```go
type activeProvider struct {
  Descriptor     providerDescriptor
  Platform       core.Platform
  TransportMode  string

  // NEW
  Metadata       core.PlatformMetadata
  RateLimiter    *core.RateLimiter          // per-provider instance
  Receiver       *notify.Receiver           // per-provider HTTP mount (或共享 mux 路径前缀)
  ControlConn    *client.ControlPlaneConn   // per-provider 或共享
}

type Runtime struct {
  Providers    []*activeProvider
  Tenants      []*core.Tenant
  Resolver     core.TenantResolver
  PluginRegistry *core.PluginRegistry
  SessionStore *core.SessionStore
  AuditWriter  audit.Writer                 // 来自 change C
}
```

**HTTP port 分配**：
- 方案 A（推荐）：同一 NOTIFY_PORT 下按 path 前缀路由：`/feishu/*` / `/dingtalk/*` / `/wecom/*` / `/im/*`（generic）。
- 方案 B：每 provider 独立 port。第一版不选。

**Control plane port**：所有 provider 共用一条 `/ws/im-bridge` 连接；registration payload 里用 `providers[]` 声明所承载的平台集合。

### Decision 2：Tenant 抽象与路由

```go
type Tenant struct {
  TenantID       string
  ProjectID      string         // backend project scope
  Name           string
  Credentials    map[string]CredentialRef
  PluginScope    []string       // plugin ids enabled for this tenant
  Metadata       map[string]string
}

type TenantResolver interface {
  Resolve(msg *Message) (*Tenant, error)
}

// 内置 resolver
type ChatIDResolver struct { mapping map[chatID]*Tenant }
type WorkspaceResolver struct { mapping map[workspaceID]*Tenant }  // Slack workspace / Feishu tenant_key
type DomainResolver struct { mapping map[domain]*Tenant }          // email / teams domain
```

**tenants 配置文件**（`IM_TENANTS_CONFIG=./tenants.yaml`）：
```yaml
tenants:
  - id: acme
    projectId: 4a1e...  # UUID of backend project
    name: "ACME Corp"
    resolvers:
      - kind: chat
        chatIds: ["oc_abc123", "oc_def456"]  # Feishu
      - kind: workspace
        workspaceIds: ["T0ABC"]              # Slack
    credentials:
      - providerId: feishu
        source: env
        keyPrefix: FEISHU_ACME_
    plugins:
      - "@acme/jira-commands"
      - "@builtin/task"
```

**Inbound 路径**
```
  provider.onMessage(msg)
       │
       ▼
  runtime.Resolver.Resolve(msg) → Tenant
       │ 未命中
       ├─ 默认 tenant（若配置）→ fallthrough
       └─ 无默认 tenant → audit reject + 可选回复 "该会话未配置 tenant 绑定"
       ▼
  msg.TenantID = tenant.TenantID
  msg.ReplyTarget.TenantID = tenant.TenantID
  engine.HandleMessage(provider, msg)  // engine 在 tenant scope 下执行
```

**Outbound 路径**：`ClientFactory.For(tenant).CreateTask(...)` 自动注入 `projectId`、API key；`DeliveryEnvelope.TenantID` 在 `core.DeliverEnvelope` 调 `client.NotifyBackend(...)` 时携带。

### Decision 3：AgentForgeClient Factory

```go
type ClientFactory struct {
  baseURL       string
  defaultAPIKey string
  httpClient    *http.Client
}

func NewClientFactory(cfg *Config) *ClientFactory { ... }

func (f *ClientFactory) For(tenant *core.Tenant) *AgentForgeClient {
  return &AgentForgeClient{
    baseURL:   f.baseURL,
    projectID: tenant.ProjectID,
    apiKey:    resolveCredential(tenant, "api_key", f.defaultAPIKey),
    // ...
  }
}
```

所有 command handler 从 `func(engine, client)` 变为 `func(engine, factory)`，handler 内用 `factory.For(msg.TenantID)` 获取 scoped client。

### Decision 4：Session persistence

复用 change C 的 `core/state` SQLite 存储，新增 3 张表：

```sql
CREATE TABLE session_history (
  tenant_id   TEXT NOT NULL,
  session_key TEXT NOT NULL,
  content     TEXT NOT NULL,
  occurred_at INTEGER NOT NULL,
  PRIMARY KEY(tenant_id, session_key, occurred_at)
);
CREATE INDEX session_history_recent ON session_history(tenant_id, session_key, occurred_at DESC);

CREATE TABLE intent_cache (
  tenant_id  TEXT NOT NULL,
  text_hash  TEXT NOT NULL,
  intent     TEXT NOT NULL,
  confidence REAL NOT NULL,
  cached_at  INTEGER NOT NULL,
  PRIMARY KEY(tenant_id, text_hash)
);
CREATE INDEX intent_cache_expiry ON intent_cache(cached_at);

CREATE TABLE reply_target_binding (
  tenant_id      TEXT NOT NULL,
  binding_id     TEXT NOT NULL,  -- task id, agent run id, etc.
  reply_target   TEXT NOT NULL,  -- JSON-encoded ReplyTarget
  created_at     INTEGER NOT NULL,
  expires_at     INTEGER NOT NULL,
  PRIMARY KEY(tenant_id, binding_id)
);
```

TTL 策略：
- `session_history` 每 tenant 保留最近 100 条（LRU by `occurred_at`）。
- `intent_cache` 24h。
- `reply_target_binding` 与业务实体 TTL 对齐（task/agent run 一般 7 天），业务实体删除时 binding 也删。

### Decision 5：Command plugin registry

**manifest schema** (`${IM_BRIDGE_PLUGIN_DIR}/<plugin-id>/plugin.yaml`)
```yaml
id: "@acme/jira-commands"
version: "1.0.0"
name: "Jira Commands"
commands:
  - slash: "/jira"
    subcommands:
      - name: "create"
        description: "Create a Jira issue"
        action_class: write
        invoke:
          kind: http
          url: "http://localhost:9090/plugins/jira/create"
          method: POST
          timeout: 10s
          headers:
            X-API-Key: "${ACME_JIRA_KEY}"
      - name: "link"
        description: "Link a task to a Jira issue"
        action_class: write
        invoke:
          kind: mcp
          serverId: "@acme/jira-mcp"
          tool: "link_task"
tenants:            # 可选。空 = 所有 tenant 可用。
  - acme
  - beta
```

**Invoke 类型**
- `http`：最简单，bridge 发 POST，body 结构化包含 tenant/user/chat/args，返回体解析为 delivery envelope。
- `mcp`：调用进程内或 sidecar 的 MCP 服务（复用 `src-bridge` 的 MCP 基础设施）。
- `builtin`：调用已编译进 bridge 的 handler（内置 `/task`、`/agent` 等迁移后的形态）。

**注册流程**：
1. 启动时扫 `IM_BRIDGE_PLUGIN_DIR`，读每个子目录的 `plugin.yaml`。
2. 对每条 `commands[]`，调 `engine.RegisterCommand(slash, buildHandler(plugin, subcmd))`。
3. handler 在执行前检查 `tenants` 列表，不包含当前 tenant → 拒绝 + 审计。

**与 marketplace 对齐**：`src-marketplace` 的 plugin manifest 新增可选字段 `im_commands`；安装 plugin 时，marketplace 的 consumer（`internal/plugin` 控制平面 + bridge consumer）负责把 plugin 文件写到 `IM_BRIDGE_PLUGIN_DIR`。Bridge 不主动拉 plugin，由后端推送或 volume 挂载。

### Decision 6：Control plane registration payload

**老 payload**
```json
{
  "bridgeId": "...",
  "platform": "feishu",
  "transportMode": "live",
  "projectId": "...",
  "capabilities": { ... }
}
```

**新 payload**
```json
{
  "bridgeId": "...",
  "providers": [
    {
      "platform": "feishu",
      "transportMode": "live",
      "capabilities": { ... },
      "tenants": ["acme", "beta"]
    },
    {
      "platform": "dingtalk",
      "transportMode": "live",
      "capabilities": { ... },
      "tenants": ["acme"]
    }
  ],
  "tenants": [
    { "id": "acme", "projectId": "..." },
    { "id": "beta", "projectId": "..." }
  ]
}
```

后端按 `(bridgeId, providerId, tenantId)` 索引 registration，delivery 路由时按这个三元组选目标。兼容层：若 payload 只有顶层 `platform` 字段（旧），backend 包装为 `providers=[{platform:..., tenants:[default]}]`。

## Architecture diagram

```
┌──────────────── IM Bridge Process (gateway mode) ────────────────┐
│                                                                  │
│    IM_PLATFORMS=feishu,dingtalk,wecom  IM_TENANTS_CONFIG=...     │
│                                                                  │
│  ┌─ Provider: Feishu ──┐ ┌─ Provider: DingTalk ─┐ ┌─ WeCom ─┐    │
│  │ live transport      │ │ live stream           │ │ callback│    │
│  │ inbound → msg       │ │ inbound → msg         │ │ inbound │    │
│  └─────────┬───────────┘ └──────────┬────────────┘ └────┬────┘    │
│            │                         │                    │        │
│            ▼                         ▼                    ▼        │
│        ┌──────── runtime.Resolver (chat/workspace/domain) ──────┐  │
│        │ msg → Tenant (acme / beta / ...)                       │  │
│        └────────────────────┬─────────────────────────────────┬─┘  │
│                             │                                 │    │
│                             ▼                                 │    │
│  ┌─ engine (per-tenant scoped) ────────────────────────┐      │    │
│  │ allowlist / rate / plugin registry / command route  │      │    │
│  └─────────────┬───────────────────────────────────────┘      │    │
│                │                                              │    │
│                ▼                                              │    │
│  ┌─ ClientFactory.For(tenant) → AgentForgeClient ──────────┐  │    │
│  │ backend POST with projectId + tenant API key             │  │    │
│  └─────────────┬────────────────────────────────────────────┘  │    │
│                ▼                                              ▼    │
│   ┌ Plugin Registry ─────┐    ┌ Session Store ─────────────────┐   │
│   │ YAML manifest + HTTP │    │ SQLite: history/intent/binding  │   │
│   │ / MCP / builtin      │    │ (shared with change C state.db) │   │
│   └──────────────────────┘    └─────────────────────────────────┘   │
│                                                                    │
│   ┌ ControlPlaneConn (one per bridge_id; providers[] + tenants[])  │
│   └────────────────────────┬───────────────────────────────────────│
└──────────────────────────────────────────────────────────────────── │
                             │
                             ▼
                       backend im_control_plane
                       (routes by bridgeId, providerId, tenantId)
```

## Alternatives considered

- **保留 single-active-provider，靠 Docker-compose 起多个 bridge**：目前做法。放弃是因为部署成本放大 + tenant scope 在 bridge 外靠 chatId 白名单维护极难管。
- **Tenant 作为 provider 的参数**：比如 `IM_TENANTS_FEISHU=acme:oc_abc,beta:oc_def`。易写但难扩展（domain resolver / workspace resolver 无位置）；改用 YAML 文件。
- **Plugin 用动态 .so 加载**：强能力但跨平台构建成本 + 安全面广。放弃，复用 HTTP/MCP invoke 模式。
- **Session 跨副本共享走 Redis**：和 change C 同一考量——bridge 应保持无外部依赖。留到后续 change。

## Risks

| 风险 | 触发 | 缓解 |
|------|------|------|
| Multi-provider HTTP port 冲突 | 各 provider 原本就想独占 NOTIFY_PORT | 方案 A 按 path 前缀路由；第一版 hard-code 前缀；长期允许运营声明 |
| Tenant resolver 未命中→消息丢 | 配置错/新 chatId 还没写 | 默认 tenant fallback（可关）+ 明确审计 + 给群里回 "该会话未绑定 tenant" 提示 |
| Plugin invoke HTTP 超时抖 bridge | 某 plugin 10s 超时把 event loop 占了 | 每 invoke 独立 goroutine + context timeout；慢 plugin 触发限速 |
| Session TTL 竞争 | 同一 tenant 同一 session 被频繁 UPSERT | 每 INSERT 附带 LRU 维护；tests 锁定不会膨胀 |
| Marketplace plugin 分发到 bridge dir 延迟 | bridge 不重启就没发现新 plugin | 目录扫描 + `inotify`/`fsnotify` 热加载；failure tolerant |
| Provider 凭据 per-tenant 不同 | 需要 per-tenant credential ref | 配置模型允许 env prefix 或 secret file path；明确不靠 hardcoded |

## Migration

- **Phase 1**：Tenant 抽象 + ClientFactory 重构；保持 single-provider 行为（`IM_PLATFORMS` 只允许 1 项），但命令层已通过 factory 取 client。
- **Phase 2**：Multi-provider runtime：`IM_PLATFORMS` 接受多项；HTTP mux 按前缀路由；control plane `providers[]` 注册。
- **Phase 3**：Session persistence 上线；`historyBySession` map 切成 `SessionStore` 调用；重启保留 history。
- **Phase 4**：Command plugin registry：内置命令迁入 plugin 形态；YAML 配置驱动；marketplace 管道对齐。
- **Phase 5**：运维文档 + 多 tenant 部署 runbook + 后端路由校验。

## Open questions

1. Tenant 的 "默认 fallback" 行为默认开还是关？倾向默认关（显式配置），避免消息被静默路由到错误 tenant。
2. Plugin 的 `invoke.kind=mcp` 需要 bridge 有 MCP client 能力——是复用 `src-bridge` 已有 MCP 客户端还是在 im-bridge 里内置？倾向复用，通过 HTTP 代理把 bridge 变成 MCP client 代理。
3. Multi-provider 场景下 `notify_port` 是共享还是每 provider 独占？方案 A（共享 + 前缀）是第一选择，但若 provider 需要监听 WebHook 的特定 Host header 可能要改方案 B。
4. Plugin 如何做 per-tenant 凭据传递？第一版通过 `tenant.metadata` 里注入自定义字段，plugin manifest 支持 `${TENANT_META_KEY}` 引用。
