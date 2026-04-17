## Why

IM Bridge 当前是 "single-active-provider-per-process, single-project-per-process" 的 sidecar。这个形态在 AgentForge 当前内部测试阶段能用，但离 openclaw 的 "gateway" 形态有三条结构性差距，这三条差距会在多租户真实部署第一天就同时爆发：

- **多 provider 并发**：企业场景常在同一台 bridge 上同时服务 飞书 + 钉钉 + WeCom。当前 `selectProvider` 返回 1 个 `activeProvider`；要跑三家平台就得部署三套 bridge 进程，凭据/状态/审计/限速各自为营。
- **多租户路由**：`AGENTFORGE_PROJECT_ID` 是进程级 env，一个 bridge 进程绑死一个 project。一家公司若有多个 project（PRD 里明确定义的隔离单位），每个 project 都需要独立 bridge = 资源浪费 + 运维放大；更严重的是同一 IM 群里不同用户触发命令无法分派到各自的 project。
- **会话状态 + 命令插件化缺失**：
  - main.go 的 `historyBySession` 是进程内存 map，重启丢失，不跨副本。NLU history 依赖它做 intent 推理，冷启动每次都从零开始。
  - 命令全部硬编码进 binary（`commands/task.go`、`commands/agent.go` 等），新增自定义命令（`/jira`、`/deploy`、`/metrics`）必须改代码重编译；marketplace 买的 plugin 没法注入 IM 入口。openclaw 的 `/skills` 机制是这类场景的原型。

同时，单 `IM_CONTROL_SHARED_SECRET` + 单 bridge_id + 单 provider 的设计让 change C 的很多能力（per-tenant 限速、per-tenant 审计 scope）没有第二维度可用；B 不做，C 的多维 policy 实用价值打折。

## What Changes

### 核心架构升级

- **Multi-Provider per process**：`providerDescriptor` 从 "lookup → activeProvider" 升级为 "lookup → RegisteredProvider(s)"。`IM_PLATFORMS=feishu,dingtalk,wecom`（复数）在单进程里挂起三组 live transport，每组独立 metadata/ReplyPlan/RateLimit 实例。`cmd/bridge/main.go` 的 `selectProvider` 变为 `selectProviders(cfg) []*activeProvider`。
- **Tenant 抽象**：新增 `core.Tenant` 类型（`TenantID`、`ProjectID`、`Name`、`Credentials map[providerID]CredentialRef`）。一个 bridge 进程承载 N 个 tenant；每个 inbound message 通过 `provider.ResolveTenant(msg) (*Tenant, error)` 路由到对应 tenant 上下文。Resolver 策略：`by_chat_id` / `by_domain` / `by_workspace` —— 可插拔。
- **TenantAwareClient**：`AgentForgeClient` 从"一个实例绑一组凭据"升级为"工厂 + tenant 上下文"，`client.For(tenant).CreateTask(...)` 自动带上对应 project scope + API key。
- **Session state persistence**：复用 change C 的 `core/state` SQLite 存储，新增 `session` 表持久化 `historyBySession`、`intent_cache`、`reply_target_bindings`。重启后 history 保留；多 bridge 副本读写同一 state 需要在 K8s 里挂盘共享（第一版不做跨副本 session 同步，只做单副本持久化）。
- **Command plugin registry**：
  - 新增 `core.CommandPlugin` 接口：`Metadata() CommandMetadata` + `RegisterHandlers(engine *Engine)`。
  - 新增 `${IM_BRIDGE_PLUGIN_DIR}/*.yaml` 规范：plugin 用 YAML 声明命令入口 + 处理方式（`invoke: http` / `invoke: mcp` / `invoke: builtin`），避免直接动态加载 Go 代码（安全 + 简单）。
  - 内置命令全部迁移到该 registry（`task.go` / `agent.go` 等保留作为 builtin plugin 的实现）；外部插件通过 YAML + HTTP/MCP 接入。
  - 与 AgentForge marketplace 的 plugin 系统对齐（见 `src-marketplace/` 与 `internal/plugin` 控制平面），同一份 plugin manifest 可声明 IM 命令入口。
- **Multi-tenant control plane**：`/ws/im-bridge` 连接时带 `bridgeId`；新增 `ProviderBinding` 协议，让单一 bridge_id 下挂多 provider + 多 tenant 的 registration，backend 按 `(bridgeId, providerId, tenantId)` 三元组路由 delivery。
- **Tenant-scoped rate limit + audit**：change C 的多维 rate policy 的 `DimTenant` 填上真值；change C 的 audit event 里 `tenantId` 字段成真。

### 契约破坏

- `IM_PLATFORM` env 改名为 `IM_PLATFORMS`（复数，逗号分隔）；旧名字作为兼容别名保留到本 change 合并后下一次，随后移除。
- `IM_CONTROL_SHARED_SECRET` 保留，但同一 bridge 进程里每个 provider 可以额外通过 `IM_SECRET_<PROVIDER>` 覆盖；fallback 到 `IM_CONTROL_SHARED_SECRET`。
- `activeProvider` 结构与 `selectProvider` 返回类型变化；所有内部调用点同步迁移。
- Bridge ↔ backend 控制面 registration payload 增加 `providers[]` 数组 + `tenants[]` 数组；backend `im_control_plane.go` 兼容旧单-provider 注册。
- `AgentForgeClient` API 重构为 `Factory → ClientFor(tenant)` 模式；所有 command handler 同步迁移。
- `DeliveryEnvelope` / `ReplyTarget` 增加 `TenantID` 字段（零值保持当前单租户行为）。

## Non-Goals

- 不在同一进程内跨 tenant 共享附件 staging dir —— 每 tenant 独立 `${IM_BRIDGE_STATE_DIR}/<tenantId>/attachments/`。
- 不做跨 bridge 副本的 session 同步（第一版只做单副本持久化）。
- 不把 command plugin 升级为 full WASM 沙箱 —— 复用 marketplace plugin 的 HTTP/MCP invoke 模式。
- 不对后端 `project-access-control` capability 做回改 —— tenant 与 project 的 1:1 或 N:1 绑定由后端决定，bridge 只负责路由。
- 不做 tenant 的动态注册/下线（改配置 + SIGHUP 即可完成）；未来若需 UI 动态管理再独立 change。
- 不做 chat 层 SSO / IM 用户 ↔ AgentForge user 强绑定 —— 沿用后端 `im_user_binding` 路径。

## Capabilities

### New Capabilities
- `im-bridge-multi-provider-runtime`：定义 multi-provider per-process 的注册、生命周期、独立 transport、独立 capability matrix 约束。
- `im-bridge-tenant-routing`：定义 tenant 类型、resolver 合约、inbound→tenant 路由语义、tenant scoped credentials。
- `im-bridge-session-persistence`：定义 session history / intent cache / reply target binding 的持久化契约。
- `im-bridge-command-plugin-registry`：定义 plugin manifest schema、invoke 模式（http/mcp/builtin）、与 marketplace plugin 管理的对齐。

### Modified Capabilities
- `im-bridge-control-plane`：registration payload 增加 `providers[]` + `tenants[]`；delivery 路由按 `(bridgeId, providerId, tenantId)`。
- `im-bridge-durable-state`（若与 C 合并）：新增 `session` 表的契约。
- `additional-im-platform-support`：provider descriptor 允许同一进程同时激活多 provider。
- `im-provider-catalog-truth`：catalog payload 额外展示一个 bridge 实例挂载的多个 provider bindings。

## Impact

- **运行时** (`src-im-bridge/`)
  - `cmd/bridge/`：`selectProvider` → `selectProviders`；startup 并行启动 N 个 provider；shutdown 并行停止。
  - `core/tenant.go`（新）：Tenant 类型 + Resolver 接口。
  - `core/`：`DeliveryEnvelope` / `ReplyTarget` 增 `TenantID`；routing helper 新增 tenant 维度。
  - `client/`：`AgentForgeClient` 重构为 Factory 模式；`ControlPlaneConn` 支持多 provider binding。
  - `core/plugin/`（新）：PluginRegistry、manifest loader、YAML schema。
  - 内置 commands 迁移为 builtin plugin。
- **配置**
  - `IM_PLATFORMS` / `IM_TENANTS_CONFIG`（YAML 文件路径）/ `IM_BRIDGE_PLUGIN_DIR` 等。
- **后端** (`src-go/`)
  - `internal/service/im_control_plane.go` + `internal/ws/im_control_handler.go`：支持 multi-provider / multi-tenant registration。
  - `internal/handler/im_handler.go`：delivery 路由按 `(bridgeId, providerId, tenantId)`。
  - `internal/model/im.go`：注册 payload schema 扩展。
- **Marketplace 对齐**
  - `src-marketplace/internal/model/` 插件 manifest 增加 `im_commands` 字段；`internal/service/consumption.go` 分发到 bridge plugin dir。
- **文档**
  - `src-im-bridge/README.md` 重写 "Platform Selection" 为 "Provider Set + Tenants"。
  - `src-im-bridge/docs/platform-runbook.md` 增加多 provider / 多 tenant 的部署模板。

## Dependencies on other changes

- **依赖 C（`2026-04-17-harden-im-bridge-security-ops`）**：
  - tenant-scoped rate / audit 要 C 的 `RateLimitPolicy` 和 `audit.Event` 作为底座。
  - session 持久化需 C 的 `core/state` SQLite foundation。
  - 强烈建议 C 合并后 2-4 周再启动 B，避免同时重构两条关键路径。
- **松耦合 A（`2026-04-17-deepen-im-bridge-rich-delivery`）**：A 扩展的 Attachment/Reaction/Thread 在本 change 里自动获得 per-tenant scoping；若 A 先合，本 change 只需补 `TenantID` 字段；若 A 后合，B 已预留字段。
