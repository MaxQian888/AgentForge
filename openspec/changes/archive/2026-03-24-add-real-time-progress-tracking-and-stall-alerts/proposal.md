## Why

AgentForge 已经有任务状态机、WebSocket 广播、通知表和 IM 通知接收器，但现在只能在状态被手动修改后被动更新，缺少“任务是否在持续推进”的统一判断与自动预警链路。PRD 已经把“实时状态流转，自动检测停滞并预警”列为 P0，这次 change 要先把这条能力定义成可实现、可验证的产品合同。

## What Changes

- 为任务域增加统一的进度追踪能力，记录任务最近有效活动、最近状态变更、停滞状态与恢复状态，而不仅仅保存当前 `status`。
- 定义任务停滞检测规则，使系统能够基于任务状态、最近活动时间、指派情况和 Agent 运行信号自动识别需要关注的任务。
- 补齐预警投递链路，让停滞、恢复和关键进度风险能够同时进入 WebSocket 实时事件、站内通知和 IM 通知入口。
- 为前端任务列表与 Dashboard/看板消费方补充进度信号字段和事件，使页面能够实时展示风险状态、预警原因和后续处理入口。
- 明确本次 change 只覆盖任务进度信号与预警，不扩展 Sprint 燃尽图、完整绩效系统或全新的团队管理页面。

## Capabilities

### New Capabilities
- `task-progress-tracking`: 定义任务活动信号、实时进度状态、停滞检测和恢复语义。
- `task-progress-alerting`: 定义任务进度风险的站内/WebSocket/IM 预警投递与去重升级规则。

### Modified Capabilities
- None.

## Impact

- Affected Go backend code in `src-go/internal/model/task.go`, task-related repositories/services/handlers, notification service, WebSocket event definitions, and new persistence plus detector wiring for task progress signals.
- Affected database surface for persisting task progress metadata or signal snapshots beyond the current task `status` and timestamps.
- Affected frontend state and realtime handling in `lib/stores/task-store.ts`, `lib/stores/ws-store.ts`, notification state, and task/dashboard views that need to render progress health and alerts.
- Affected IM notification flow in `src-im-bridge/notify/receiver.go` and any backend-to-IM notification payloads for progress warnings.
- Affected product flow for task assignment, agent execution monitoring, review waiting, and dashboard risk visibility, while remaining narrower than the existing dashboard/team-management change.
