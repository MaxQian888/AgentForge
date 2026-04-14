## Why

AgentForge 的 IM Bridge 已经具备多平台 provider、控制面投递、operator `/im` 页面、Bridge-backed 命令和 backend action executor 等关键基础，但当前仍存在几处会直接破坏“功能完整性”的 runtime integration gap：`/im/test-send` 没有接到真实 sender、已保存的 channel/event subscription 没有进入真实事件路由、TS Bridge 发出的部分事件类型与 Go/IM 转发链不对齐，以及后端已支持的 message-to-doc / message-to-task action 还没有被 IM 入口真正暴露出来。继续保持这些断点，会让 IM Bridge 看起来“界面和 spec 都有”，但实际仍停留在局部演示和半连通状态。

现在需要用一条聚焦 change 把这些跨层断点收口成真实可用的产品契约：让 operator console 的配置和测试操作真正驱动运行时，让事件转发语义回到 repo truth，让已有 action 能力变成用户可达能力，而不是继续依赖隐藏后端能力或单一 env fallback。

## What Changes

- 修复 IM operator console 的关键运行时接线，确保 `/im/test-send` 通过真实 sender 走 canonical IM delivery pipeline，并返回可追踪的 settlement 结果而不是停在未接线状态。
- 新增一条 authoritative 的 channel/event routing 契约，让 `IMChannel.Events` 和已配置 channel 真正参与 wiki/document events、automation-triggered IM messages、以及其他 channel-scoped broadcast delivery 的目标选择，而不是只做保存展示。
- 补齐 TS Bridge → Go backend → IM Bridge 的 event forwarding parity，至少覆盖当前 repo 中已存在的 Bridge event types、预算预警类事件、reply-target preference metadata、以及 control-plane 内的顺序与过滤语义，避免 spec 写满而 runtime 只接住部分事件。
- 把 backend 已支持但 IM 侧尚未暴露的 action 能力补成用户可达入口，重点覆盖 `save-as-doc` 和 `create-task` 这类 message conversion 流程，并保证 source message context、reply-target 和 action outcome 在 IM 中可见。
- 收紧 IM event inventory 与 operator-facing contract，使 `/im/event-types`、`/im` 页面、wiki/doc IM forwarding、automation IM send、以及 IM action entrypoints 围绕同一套 runtime truth 工作，而不是继续依赖静态列表、单一 env channel 或隐式后门能力。

## Capabilities

### New Capabilities
- `im-channel-event-routing`: 定义配置化 IM channel 与 event subscription 的 authoritative runtime contract，覆盖哪些非绑定式 IM 事件应按 channel 配置路由、如何解析 platform/channel 目标、以及如何与 control-plane delivery 和 operator surfaces 保持一致。

### Modified Capabilities
- `im-bridge-operator-console`: `/im` operator workspace 的 test-send、config drill-through 和 operator action 必须接到真实 backend sender 与 settlement feedback，而不是停留在只读或未接线状态。
- `bridge-event-im-forwarding`: Bridge event forwarding 必须与当前 TS Bridge event inventory、Go 中枢拓扑、reply-target preference metadata 和真实过滤/顺序语义对齐，补齐 repo 中仍然缺失的 runtime parity。
- `im-bridge-progress-streaming`: document event streaming、automation-triggered IM delivery、以及 message-to-doc / message-to-task 相关的 IM follow-up 必须走真实入口并保持与 bound progress / terminal delivery 语义一致。
- `im-action-execution`: IM action execution 需要把 backend 已有的 message conversion 和 follow-up workflow 能力变成真实 IM 可达能力，并保留 source context、reply-target lineage 和用户可见结果。

## Impact

- Affected Go backend: `src-go/internal/server/routes.go`, `src-go/internal/handler/im_control_handler.go`, `src-go/internal/service/im_service.go`, `src-go/internal/service/im_control_plane.go`, `src-go/internal/service/wiki_service.go`, `src-go/internal/service/automation_engine_service.go`, `src-go/internal/service/im_action_execution.go`, `src-go/internal/service/agent_service.go`, 以及相关 handler/service tests。
- Affected IM Bridge: `src-im-bridge/commands/*`, card/action entrypoint seams, notify/action payload normalization, and any provider-specific action exposure needed for message conversion or follow-up flows。
- Affected TS Bridge: Bridge event type handling and any focused tests/docs needed to make forwarded event inventory and payloads match the Go/IM side truth。
- Affected frontend/operator surfaces: `app/(dashboard)/im/page.tsx`, `components/im/*`, `lib/stores/im-store.ts`, 以及 `/im` 页面中与 event inventory、test-send、history refresh、provider configure drill-through 相关的交互。
- Affected APIs/contracts: `/api/v1/im/test-send`, `/api/v1/im/event-types`, IM channel configuration semantics, Bridge event forwarding payload expectations, and IM action entrypoint expectations; 目标是收紧现有增量契约，不引入新的 breaking removal。
