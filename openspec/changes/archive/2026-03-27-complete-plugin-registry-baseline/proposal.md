## Why

`PLUGIN_SYSTEM_DESIGN.md` 和 `PRD.md` 已经把 `Plugin Registry` 定义为插件生态建设的下一阶段能力，但当前仓库真相仍停留在本地 catalog、trust gate 和 browse-only marketplace。Go 控制面虽然预留了远程 marketplace 路由与 remote registry client 接缝，前端也有 marketplace 区块，但远程目录、artifact 拉取、operator-facing 远程安装与 registry 元数据流都还没有形成可运行闭环，因此需要一个 focused OpenSpec change 把这条文档承诺补成 apply-ready 的实现契约。

## What Changes

- 为 AgentForge 定义 `Plugin Registry` 基础版能力，覆盖远程 registry 目录读取、远程插件元数据模型、artifact 拉取与最小安装闭环。
- 为远程插件分发定义真实的信任与制品验证边界，确保远程安装不会绕过当前 digest、signature、approval 与 lifecycle gate。
- 扩展现有插件控制台，使其能区分本地 catalog 与远程 registry 条目，并对远程可安装条目提供 truthful 的浏览、筛选、详情和安装语义。
- 补齐与远程 registry 对接所需的 operator-facing 状态，包括远程源可达性、安装结果、版本来源和失败原因，而不是继续暴露“client 未配置”的占位错误。
- 保持当前插件 runtime、workflow/review 扩展、桌面能力和本地 scaffold/SDK 能力不变；本次不进入评分评论、公开开发者门户、多租户运营后台或完整 OCI 发布基础设施。

## Capabilities

### New Capabilities
- `plugin-registry-marketplace`: define the remote AgentForge Plugin Registry browse, metadata, source selection, and operator-facing installation flow for marketplace entries.

### Modified Capabilities
- `plugin-distribution-and-trust`: extend distribution and trust requirements so remote registry artifacts are fetched, verified, and gated through the same operator-visible trust flow as other external sources.
- `plugin-management-panel`: extend the plugin operator console so remote registry entries, remote install actions, source reachability, and install failure states are rendered truthfully.

## Impact

- Affected backend/control-plane seams: `src-go/internal/service/plugin_service.go`, `src-go/internal/handler/plugin_handler.go`, `src-go/internal/server/routes.go`, and any new remote registry client or manifest-download integration.
- Affected frontend/operator surfaces: `app/(dashboard)/plugins/page.tsx`, `components/plugins/*`, `lib/stores/plugin-store.ts`, and related i18n/messages for remote marketplace browsing and installation.
- Affected product contracts: remote marketplace routes, plugin source metadata, installation error semantics, and trust/approval visibility for remote artifacts.
- Verification impact: focused Go service/handler tests for remote registry browse/install flows plus frontend plugin console tests for remote marketplace rendering, gating, and install behavior.