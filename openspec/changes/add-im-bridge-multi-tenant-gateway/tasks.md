## 1. Tenant abstraction & ClientFactory refactor

- [ ] 1.1 `core/tenant.go`：新增 `Tenant`、`CredentialRef`、`TenantResolver` 接口 + `ChatIDResolver` / `WorkspaceResolver` / `DomainResolver` 内置实现。
- [ ] 1.2 `core/message.go`：`Message` 与 `ReplyTarget` 增加 `TenantID` 字段（零值兼容）。
- [ ] 1.3 `client/`：`AgentForgeClient` 重构为 `ClientFactory.For(tenant) → AgentForgeClient`；全部现有调用点迁移。
- [ ] 1.4 `commands/*.go`：handler 签名从 `(engine, client)` 改为 `(engine, factory)`；内部用 `factory.For(msg.TenantID)`。
- [ ] 1.5 Tenant 配置加载：`IM_TENANTS_CONFIG` 指向 YAML，启动解析为 `[]*Tenant` + resolver 组合。
- [ ] 1.6 Tests：resolver 命中/未命中、default tenant fallback、factory scope 正确。

## 2. Multi-provider runtime

- [ ] 2.1 `cmd/bridge/provider_contract.go`：`selectProvider` → `selectProviders`；每 provider 独立 `activeProvider` 实例。
- [ ] 2.2 `cmd/bridge/main.go`：启动多 provider（goroutine）、shutdown 汇合；`IM_PLATFORMS` 解析。
- [ ] 2.3 HTTP mount：`notify.Receiver` 支持按 path 前缀 (`/feishu/*`, `/dingtalk/*` ...) 多 provider 共享 port；generic `/im/*` endpoints 依 `X-IM-Source` header 区分。
- [ ] 2.4 `client/control_plane.go`：registration payload 支持 `providers[]` + `tenants[]`；每个 provider binding 带自己的 capability matrix。
- [ ] 2.5 Tests：双 provider 并发接收（feishu + dingtalk stub）、startup/shutdown 顺序、注册 payload 结构。

## 3. Session persistence

- [ ] 3.1 `core/state/sessions.go`（复用 change C 的 SQLite DB）：`SessionStore.Append(tenantID, sessionKey, content)`、`Recent(tenantID, sessionKey, n)`、`IntentCache.Get/Set`、`ReplyBinding.Put/Get/Delete`。
- [ ] 3.2 `cmd/bridge/main.go`：`historyBySession` map 替换为 `SessionStore` 调用。
- [ ] 3.3 TTL worker（LRU 保留 100 条 history per session；intent cache 24h）。
- [ ] 3.4 Tests：`core/state/sessions_test.go`——重启后 history 恢复、LRU、TTL。

## 4. Command plugin registry

- [ ] 4.1 `core/plugin/registry.go`：`CommandPlugin`、`PluginRegistry`、manifest loader（YAML）。
- [ ] 4.2 invoke 适配器：`invoke/http.go`（HTTP POST）、`invoke/mcp.go`（复用 src-bridge MCP client via HTTP proxy）、`invoke/builtin.go`（引用 in-process handler）。
- [ ] 4.3 内置 `/task` `/agent` `/tools` 等迁移为 builtin plugin（保留现有 handler 实现，但通过 plugin manifest 注册）。
- [ ] 4.4 Plugin 目录扫描 + fsnotify 热加载。
- [ ] 4.5 Marketplace 对齐：`src-marketplace/internal/model/plugin.go` 增 `im_commands` 字段；`src-go/internal/plugin` 控制平面在 consumer 安装 plugin 时同步落盘到 `IM_BRIDGE_PLUGIN_DIR`。
- [ ] 4.6 Tests：YAML 解析、invoke 三路径、tenant scope 过滤、热加载路径。

## 5. Control plane & backend alignment

- [ ] 5.1 `src-go/internal/service/im_control_plane.go`：接收新 registration payload；兼容旧单-platform payload。
- [ ] 5.2 `src-go/internal/handler/im_handler.go`：delivery 路由索引改为 `(bridgeId, providerId, tenantId)`。
- [ ] 5.3 `src-go/internal/model/im.go`：registration / delivery payload schema 更新。
- [ ] 5.4 `lib/stores/im-store.ts` + `components/im/*`：operator 控制台显示 "bridge instance 挂载的 providers + tenants" 视图。
- [ ] 5.5 Tests：旧 payload 继续工作、新 payload 路由正确、multi-tenant 场景 isolation。

## 6. Tenant-scoped rate / audit / allowlist

- [ ] 6.1 与 change C 的 `RateLimitPolicy.DimTenant` 对接：rate key 加入 tenant 维度。
- [ ] 6.2 `audit.Event` 的 `tenantId` 字段成真。
- [ ] 6.3 `IM_COMMAND_ALLOWLIST` 支持 tenant 维度：`<tenant>:<platform>:<command>`。
- [ ] 6.4 Tests：每 tenant 独立限速、审计事件带 tenantId。

## 7. Docs & runbooks

- [ ] 7.1 `src-im-bridge/README.md` 重写：Platform Selection → Providers + Tenants。
- [ ] 7.2 `src-im-bridge/docs/platform-runbook.md`：多 provider / 多 tenant 部署模板；典型诊断路径。
- [ ] 7.3 `docs/PRD.md` 和 `docs/part/TECHNICAL_CHALLENGES.md` 中 IM Bridge 小节更新为 gateway 形态。

## 8. Verification & rollout

- [ ] 8.1 `scripts/smoke/`：新增 `multi-tenant-smoke.ps1`，同时起 feishu + dingtalk stub，验证两 tenant 分流。
- [ ] 8.2 `go test ./...` 全绿；新增的 plugin invoke 需要 HTTP mock server 基础设施。
- [ ] 8.3 Rollout runbook：从 single-provider 升级到 multi-provider 的迁移步骤（先加 IM_PLATFORMS 含旧单值 + tenant.yaml，再逐 tenant 加绑定）。
