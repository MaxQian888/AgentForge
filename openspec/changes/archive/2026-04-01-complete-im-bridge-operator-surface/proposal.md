## Why

IM Bridge 的平台适配、控制面投递、前端 `/im` 页面和单条 delivery retry 已经存在，但当前 operator-facing 产品面仍停留在“可看基础状态”的最小实现：`/api/v1/im/bridge/status` 只返回注册/心跳级摘要，前端无法看到队列积压、聚合投递指标、平台诊断，也没有把测试发送、失败重试和配置入口串成真实可用的操作链。与此同时，活跃中的 `enhance-frontend-panel` 虽然记录了更完整的 IM status panel 设想，但范围过宽且 IM 只是其中一个未落地子块；现在需要一个更聚焦的 change，把现有 IMBridge 能力真正收敛成可使用、可诊断、可操作的完整产品面。

## What Changes

- 扩展 IM Bridge 的 operator 数据契约，让后端状态接口除了基础 liveness 以外，还能返回按 provider 聚合的运行摘要、待投递/失败数量、最近 fallback 或错误信号、以及聚合投递指标。
- 为 IM operator 面补齐真实操作链：测试发送、失败/超时 delivery 重试、批量 retry 工作流，以及从状态视图直接跳转到现有 channel configuration 的上下文入口。
- 把当前 `/im` 页面升级为完整的 IM Bridge operator console，覆盖 summary metrics、provider diagnostics、filterable activity history、payload/detail drilldown、queue/backlog 指示和 test-send affordance，而不是继续停留在分散的基础 tabs。
- 复用现有 `/api/v1/im/channels`、`/api/v1/im/deliveries/:id/retry`、`/api/v1/im/send`、control-plane history/state 等共享 seam，不新增平行 debug UI 或单独的 IM 管理入口。
- 保持现有平台 adapter、rich delivery envelope、IM chat command grammar 和已有 `/im` 路由兼容；本次 change 聚焦 operator 完整性与功能落地使用，不重做平台基础设施。

## Capabilities

### New Capabilities
- `im-bridge-operator-console`: 定义 `/im` operator workspace 的产品契约，要求它暴露 IM Bridge 的 summary metrics、provider diagnostics、activity/history、test-send 和 config drill-through。

### Modified Capabilities
- `im-bridge-control-plane`: 将 Bridge status/operator snapshot 从基础注册心跳摘要扩展为包含 queue/backlog、recent delivery health、provider diagnostics metadata 和聚合运行概览的控制面契约。
- `im-delivery-diagnostics`: 将 delivery diagnostics 从单条 preview/retry 扩展为支持筛选、批量 retry、聚合 counters 和更完整 payload/detail drilldown 的 operator 工作流契约。

## Impact

- Affected frontend: `app/(dashboard)/im/page.tsx`, `components/im/*`, `components/shared/*`, `lib/stores/im-store.ts`, IM 相关 i18n 文案与 command palette / navigation 深链。
- Affected backend: `src-go/internal/model/im.go`, `src-go/internal/handler/im_control_handler.go`, `src-go/internal/service/im_control_plane.go`, 相关 route / handler / operator tests，以及必要的 operator action endpoints。
- Affected bridge runtime: `src-im-bridge/cmd/bridge` / control-plane diagnostics metadata 上报路径在需要平台诊断时可能补充更丰富的 provider health 信息。
- Affected APIs: 扩展 `GET /api/v1/im/bridge/status`、`GET /api/v1/im/deliveries` 的返回能力；复用或补充 operator action endpoints 以承载 test-send 与 bulk retry；不引入预期中的 breaking removal。
