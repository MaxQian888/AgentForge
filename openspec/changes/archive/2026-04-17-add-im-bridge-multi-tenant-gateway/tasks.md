## 1. Tenant abstraction & ClientFactory refactor

- [x] 1.1 `core/tenant.go`：新增 `Tenant`、`CredentialRef`、`TenantResolver` 接口 + `ChatIDResolver` / `WorkspaceResolver` / `DomainResolver` 内置实现。
- [x] 1.2 `core/message.go`：`Message` 与 `ReplyTarget` 增加 `TenantID` 字段（零值兼容）。
- [x] 1.3 `client/`：`AgentForgeClient` 重构为 `ClientFactory.For(tenant) → AgentForgeClient`；全部现有调用点迁移。
- [x] 1.4 `commands/*.go`：handler 签名从 `(engine, client)` 改为 `(engine, factory)`；内部用 `factory.For(msg.TenantID)`。
- [x] 1.5 Tenant 配置加载：`IM_TENANTS_CONFIG` 指向 YAML，启动解析为 `[]*Tenant` + resolver 组合。
- [x] 1.6 Tests：resolver 命中/未命中、default tenant fallback、factory scope 正确。

## 2. Multi-provider runtime

- [x] 2.1 `cmd/bridge/provider_contract.go`：`selectProvider` → `selectProviders`；每 provider 独立 `activeProvider` 实例。
- [x] 2.2 `cmd/bridge/main.go`：启动多 provider（goroutine）、shutdown 汇合；`IM_PLATFORMS` 解析。
- [x] 2.3 HTTP mount：`notify.Receiver` 支持多 provider —— 第一版采用 per-provider 独立端口（`NOTIFY_PORT + offset`，可通过 `IM_NOTIFY_PORT_<PROVIDER>` 覆盖）。共享端口的 path-prefix 路由作为后续优化登记。
- [x] 2.4 `client/control_plane.go`：registration payload 新增 `tenants[]` + `tenantManifest[]`；每个 provider binding 带自己的 capability matrix。
- [x] 2.5 Tests：resolver / factory / tenant config 测试覆盖新路径；provider_contract 单测验证 selectProviders 的去重 + 错误。

## 3. Session persistence

- [x] 3.1 `core/state/sessions.go`（复用 change C 的 SQLite DB）：`SessionStore.Append(tenantID, sessionKey, content)`、`Recent(tenantID, sessionKey, n)`、`IntentCache.Get/Set`、`ReplyBinding.Put/Get/Delete`。
- [x] 3.2 `cmd/bridge/main.go`：`historyBySession` map 替换为 `SessionStore` 调用。
- [x] 3.3 TTL worker（LRU 保留 100 条 history per session；intent cache 24h）。
- [x] 3.4 Tests：`core/state/sessions_test.go`——重启后 history 恢复（`TestSessionStorePersistsHistoryAcrossRestart`）、LRU（`TestSessionStoreHistoryLimit`）、TTL（`TestIntentCacheTTL`）、tenant 隔离（`TestSessionStoreTenantScoped`）、binding delete（`TestReplyBindingDelete`）全部覆盖。

## 4. Command plugin registry

- [x] 4.1 `core/plugin/plugin.go`：`Manifest`、`Registry`、manifest loader（YAML）。
- [x] 4.2 invoke 适配器：HTTP POST、MCP stub（transport pending — 等 src-bridge MCP proxy 对接）、builtin key-based 注册。
- [x] 4.3 内置 `/task` `/agent` `/tools` 等继续以既有 `commands.Register*` 注册；builtin plugin 通过 `Registry.RegisterBuiltin(key, handler)` 注入。完整迁移（保留现有行为 + 改走 plugin manifest）作为后续 change 落地，避免与本次 gateway 重构同次发生重大 API churn。
- [x] 4.4 Plugin 目录轮询热加载（30s 间隔）。fsnotify 升级为后续 optimization。
- [x] 4.5 Marketplace 对齐：`src-marketplace/internal/model/model.go` 新增 `PluginIMCommand` / `PluginIMSubcommand` / `PluginIMInvokeSpec` / `PluginExtraMetadata` 类型；`src-go/internal/handler/marketplace_handler.go` 新增 `WithIMBridgePluginDir` + `shipIMCommandsToBridge` — install 完成后将 `extra_metadata.im_commands` 序列化为 `<bridgePluginDir>/<slug>/plugin.yaml`，bridge 的 fsnotify/poll 监听自动加载。
- [x] 4.6 Tests：YAML 解析、invoke 三路径、tenant scope 过滤、subcommand 路由。

## 5. Control plane & backend alignment

- [x] 5.1 `src-go/internal/service/im_control_plane.go`：接收新 registration payload (`Tenants` + `TenantManifest`)；旧 payload（无字段）继续工作 — 零值兼容。
- [x] 5.2 `src-go/internal/service/im_control_plane.go`：`resolveBridgeTenantLocked` 按 `(bridgeId, providerId, tenantId)` 过滤候选 bridge；`QueueDelivery` 传入 `TenantID`；`ErrIMTenantProviderMismatch` 在 explicit target 不匹配时拒绝。Legacy bridge（无 `Tenants[]`）继续接受所有 tenant 以保证向后兼容。测试覆盖见 `im_control_plane_tenant_test.go`。
- [x] 5.3 `src-go/internal/model/im.go`：registration (`Tenants` + `TenantManifest`) / delivery (`TenantID`) / reply target (`TenantID`) schema 更新。
- [x] 5.4 `lib/stores/im-store.ts` 扩展 `IMBridgeProviderDetail` 新增 `tenants` / `tenantManifest` / `bridgeId`；`components/im/im-bridge-health.tsx` 在每个 provider 卡片上渲染 "Tenant mounts" 徽章（含 projectId tooltip）。
- [x] 5.5 Tests：`go test ./...` 全绿——旧 payload 零值继续通过；新字段由 schema 测试 + service 测试覆盖。

## 6. Tenant-scoped rate / audit / allowlist

- [x] 6.1 `engine.HandleMessage` 在 `Scope.Tenant = msg.TenantID` — rate key 真正携带 tenant 维度。`core.DimTenant` 由 change C 提供。
- [x] 6.2 `audit.Event.TenantID` 字段在 egress 路径 `emitDeliveredTenant` / `emitFailedTenant` 通过（ingress 拒绝路径的 tenant 传播作为后续补丁）。
- [x] 6.3 `IM_COMMAND_ALLOWLIST` 新增 3-段 `<tenant>:<platform>:<command>` 语法；2-段旧语法保留。
- [x] 6.4 Tests：`TestAllowlist_TenantScopedRule` 覆盖 tenant-scoped allow/deny；rate / audit 字段由 schema-level 测试覆盖。

## 7. Docs & runbooks

- [x] 7.1 `src-im-bridge/README.md` 新增 "Gateway mode (multi-provider + multi-tenant)" 顶级章节，列出 env、tenants.yaml 样例和 plugin manifest 样例。
- [x] 7.2 `src-im-bridge/docs/platform-runbook.md` 新增 "Gateway deployment" + "Diagnostics" 章节。
- [x] 7.3 `docs/PRD.md` 的 cc-connect Fork 章节新增 "Gateway 模式（2026-04 起）" 小节；`docs/part/TECHNICAL_CHALLENGES.md` 当前实现快照新增 IM Bridge gateway 条目。

## 8. Verification & rollout

- [x] 8.1 `scripts/smoke/multi-tenant-smoke.ps1` — 写入样例 tenants.yaml 到 temp 目录后，对 bridge 的 gateway 包与 backend `IMControlPlane_Tenant*` 用例运行 `go test`。完整的 E2E（真实双 provider 同时起 + 模拟 inbound 消息）在 integration test job 继续演进。
- [x] 8.2 `go test ./...` 全绿（src-im-bridge 与 src-go）。plugin invoke HTTP mock 在 `core/plugin/plugin_test.go` 中覆盖。
- [x] 8.3 Rollout runbook：见 `src-im-bridge/docs/platform-runbook.md` "Rollout from single-provider to gateway" 6 步流程。
