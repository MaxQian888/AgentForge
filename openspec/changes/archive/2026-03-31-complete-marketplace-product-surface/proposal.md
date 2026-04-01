## Why

当前仓库已经有独立的 `src-marketplace` Go 微服务、`/marketplace` 前端页面、以及 `src-go` 上的 marketplace 安装桥接，但这条链路仍停留在“文档声称完整、产品实际未闭环”的状态。现在需要把市场功能收敛成真实可运行、可部署、可消费的产品面，避免继续出现前端只做 browse、后端只对 plugin 生效、skills/roles 无法真正落地、以及独立部署端口与运行模型互相冲突的漂移。

## What Changes

- 为独立 `/marketplace` 工作区定义真实完整的 operator/product surface，覆盖浏览、筛选、详情、发布、版本管理、评价、安装、审核与失败/空态/加载态，而不是只停在基础卡片和对话框壳。
- 把 marketplace 条目的“发布 -> 版本上传 -> 审核/精选 -> 安装 -> 已安装/已使用”链路补成闭环，并要求 plugin、skill、role 三类条目都具有明确且真实的消费语义。
- 将 marketplace 与现有 repo 的侧载能力对齐，支持把已有 local path / catalog / 受支持外部来源模型接入市场分发与导入流程，而不是只保留并行的插件安装入口。
- 为独立部署定义 marketplace 服务运行契约，覆盖专用端口、CORS、health、artifact 存储、环境变量与 web/desktop 分离模式，消除当前 marketplace 与 IM Bridge 共享 `7779` 等运行时冲突。
- 约束 marketplace 产物必须被现有产品面消费：安装后的 plugin 要进入插件控制面并保留 provenance，安装后的 skill / role 要进入对应的目录或工作区，而不是只在市场页里显示“已安装”。

## Capabilities

### New Capabilities
- `marketplace-operator-workspace`: 定义 `/marketplace` 独立工作区的完整前端体验、真实状态语义、以及发布/版本/审核/安装等操作面。
- `marketplace-item-consumption`: 定义 marketplace 条目从发布到安装再到被插件、技能、角色相关工作区实际消费的闭环合同，并纳入侧载/导入语义。
- `marketplace-service-deployment`: 定义 `src-marketplace` 作为独立 Go 微服务的部署、配置、端口、health 与 artifact 运行契约，以及它与 web/desktop 主应用的接入边界。

### Modified Capabilities
- `plugin-management-panel`: 调整已安装/市场来源插件的呈现与跳转要求，确保 standalone marketplace 安装结果在插件控制面保留来源、状态与后续运维入口。

## Impact

- Affected frontend/operator surfaces: `app/(dashboard)/marketplace/page.tsx`, `components/marketplace/*`, `lib/stores/marketplace-store.ts`, `components/layout/sidebar.tsx`, and marketplace-related tests.
- Affected backend/runtime surfaces: `src-marketplace/internal/{handler,service,repository,server,config}/*`, `src-go/internal/handler/marketplace_handler.go`, `src-go/internal/server/routes.go`, and any consumer seams for installed marketplace assets.
- Affected deployment/runtime contracts: `docker-compose.dev.yml`, environment variables such as `NEXT_PUBLIC_MARKETPLACE_URL` and `MARKETPLACE_URL`, standalone service port allocation, CORS, health/readiness, and artifact storage configuration.
- Affected product integrations: existing plugin local/catalog install flows, plugin provenance/trust surfaces, and the catalog/workspace seams that must consume marketplace-installed skills and roles.
