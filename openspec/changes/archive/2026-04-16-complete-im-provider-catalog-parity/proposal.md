## Why

AgentForge 的 IM Bridge runtime 已经演进到比 operator surface 和文档更完整的 provider 集合：`src-im-bridge` 内建 registry 现在包含 `wechat` 和 `email`，但 README/runbook 仍停留在较早的 8 平台快照，缺少这两个平台的 transport 行和 manual verification 条目。继续保留这种 catalog drift，会让”多 IM 支持”在代码层看似完整、在文档和运维层却无法被准确验证。

需要把 docs 与当前 backend catalog + bridge registry 对齐，并通过 focused sync tests 防止再次漂移。

## What Changes

### 已完成 (backend + frontend)

- `src-go` 已有 authoritative IM provider catalog (`GET /api/v1/im/platforms`)，覆盖全部 10 个 operator-visible provider，区分 `interactive` 与 `delivery_only`，含 `wechat` 和 `email` 的配置字段 schema。
- Go backend surface-specific validation 已按 surface 拆分：interactive 入口（message / action / command）接受 `wechat`、拒绝 `email`；delivery surfaces（channel config / test-send）两者均接受。
- `lib/stores/im-store.ts` 已覆盖全部 10 平台的 `IMPlatform` 类型，`fetchProviderCatalog` 从 `/api/v1/im/platforms` 拉取，并在 store 中规范化存储。
- `components/im/im-channel-config.tsx` 已切换到 catalog-driven platform options，`PLATFORM_DEFINITIONS` 仅作 display-only fallback。
- `/im` 页面、`im-bridge-health.tsx`、`section-im-bridge.tsx` 均已消费 `providerCatalog`，delivery_only labeling 与 test-send affordance 均来自 catalog。
- `platform-badge.tsx` 的 `PLATFORM_DEFINITIONS` 已包含 `wechat` 和 `email` 的展示元数据（icon + config field fallback）。
- Backend Go tests 和 frontend tests 均已覆盖 `wechat` / `email` 的 catalog truth、delivery_only 行为与 operator 交互流程。

### 待完成 (docs + bridge sync)

- 同步 `src-im-bridge/README.md` 的 `IM_PLATFORM` 平台列表，补充 `wechat` 和 `email` 条目及对应凭证要求。
- 补充 `src-im-bridge/docs/platform-runbook.md` transport 表和 feature matrix，为 `wechat` 和 `email` 各加一行，使 manual verification guidance 与当前 registry 对齐。
- 新增 focused `src-im-bridge` 检查，将 built-in provider 集合（包含 `wechat`、`email`）锁定为测试契约，防止 docs/backend/bridge registry 再次漂移。

## Capabilities

### New Capabilities
- `im-provider-catalog-truth`: 定义 authoritative IM provider catalog 如何从当前 bridge runtime truth 投影到 backend/operator APIs 与 frontend operator surfaces，并区分 interactive chat provider 与 delivery-first provider 的可用 affordance。

### Modified Capabilities
- `additional-im-platform-support`: 将“支持的平台集合”从旧的 8 平台快照更新为与当前 built-in provider registry 一致的运行时真相，并要求 startup、health metadata 和文档对同一 provider set truthful 对齐。
- `im-frontend-full-platform-coverage`: 将 frontend 平台覆盖从硬编码静态枚举升级为 authoritative provider catalog 驱动，并要求 `/im` 与相关 shared components truthfully 处理新增 provider 及其配置差异。
- `im-bridge-operator-console`: 要求 `/im` operator console、provider 卡片、test-send 与 configure drill-through 反映 authoritative provider catalog 和 provider-specific operator constraints，而不是继续依赖 stale platform assumptions。

## Impact

- Affected IM Bridge runtime: `src-im-bridge/cmd/bridge/platform_registry.go`, `src-im-bridge/README.md`, `src-im-bridge/docs/platform-runbook.md`, 以及任何需要暴露 provider truth 的 metadata / docs seams。
- Affected Go backend: `src-go/internal/model/im.go`, `src-go/internal/service/im_control_plane.go`, `src-go/internal/handler/im_control_handler.go`, `src-go/internal/server/routes.go`, 以及新增或调整的 provider catalog API / validation tests。
- Affected frontend/operator surfaces: `lib/stores/im-store.ts`, `components/shared/platform-badge.tsx`, `components/im/*`, `app/(dashboard)/im/page.tsx`, `app/(dashboard)/settings/_components/section-im-bridge.tsx`，以及相关前端测试。
- Affected APIs/contracts: current `/api/v1/im/bridge/status`, `/api/v1/im/channels`, `/api/v1/im/test-send` consumers，以及新引入或扩展的 provider catalog payload contract。
