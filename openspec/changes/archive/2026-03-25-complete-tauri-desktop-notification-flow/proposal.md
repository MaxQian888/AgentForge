## Why

AgentForge 现在已经有完整的后端通知模型、WebSocket 推送和前端通知 store，但 Tauri 桌面壳仍只暴露了一个手动 `send_notification` 命令，实际业务通知并不会自动进入原生系统通知。结果是桌面模式虽然有 Tauri 能力和通知数据，却还停留在“可以测试发通知”而不是“桌面客户端真的承接通知流”的半成品状态。

现在继续完善 Tauri，最有价值的下一步不是重复补 sidecar 或 updater 骨架，而是把现有通知系统真正接进桌面增强层，让任务进度、调度告警、评审结果等通知在桌面模式下具备原生提醒、点击回跳和已读同步的正式合同。

## What Changes

- 为 AgentForge 桌面模式定义原生通知投递能力，覆盖后端持久化通知、WebSocket 实时通知与 Tauri 系统通知之间的路由、去重、显示时机和失败降级。
- 为桌面通知定义上下文保留与后续处理语义，使原生通知能够携带任务/页面目标信息并与现有 in-app 通知中心保持一致，而不是退化成一条不可追踪的系统 toast。
- 为桌面通知定义前后台协同语义，覆盖前台抑制、托盘未读摘要、重复推送保护、以及桌面不可用时保留现有 in-app 通知链路。
- 扩展共享平台能力与桌面事件桥接，使前端能统一消费“通知已投递/被抑制/投递失败”这类桌面通知事件，而不需要页面直接依赖 Tauri 原始插件 API。

## Capabilities

### New Capabilities
- `desktop-notification-delivery`: 定义 AgentForge 如何把持久化或实时业务通知路由到 Tauri 原生系统通知，并保持与现有 in-app 通知流一致的目标、链接和去重语义。

### Modified Capabilities
- `desktop-native-capabilities`: 将桌面通知能力从“接受标题/正文的单次命令”扩展为支持业务通知元数据、明确降级语义和统一前端入口的正式合同。
- `desktop-runtime-event-bridge`: 扩展桌面事件桥接，使通知投递、抑制与失败事件能够以规范化桌面事件回传前端，而不替代现有后端业务真相来源。

## Impact

- Affected frontend: `lib/platform-runtime.ts`, `hooks/use-platform-capability.ts`, `lib/stores/notification-store.ts`, `lib/stores/ws-store.ts`, `components/layout/header.tsx`, `components/layout/dashboard-shell.tsx`, and any dashboard surface that reacts to notification href targets
- Affected desktop shell: `src-tauri/src/lib.rs`, `src-tauri/capabilities/*.json`, and any Tauri plugin wiring needed for notification action callbacks or desktop event emission
- Affected backend contracts: persisted notification payload shape, realtime notification envelopes, and notification href/data conventions exposed by `src-go/internal/model/notification.go` and related services/handlers
- Affected verification: platform runtime tests, notification-store/ws-store tests, dashboard notification UX tests, and desktop-mode notification delivery/click-through validation
