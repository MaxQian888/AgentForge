## Why

AgentForge 现在已经有完整的业务通知真相链路: Go 后端持久化通知、WebSocket `notification` 事件、前端 `useNotificationStore` 归一化与 header/in-app 展示都已存在; 但桌面壳仍停留在手动调用 `send_notification` 的测试级能力，真实业务通知不会自动进入原生通知、托盘摘要或桌面结果事件。文档和现有 desktop specs 已经把桌面通知定义为正式增强层合同，而当前代码还没有把这条合同真正接通，所以需要用一个 focused change 把剩余缺口收敛清楚。

## What Changes

- 把现有持久化通知 hydration 与实时 `notification` 事件正式接入桌面通知桥，使桌面模式能够从现有通知真相来源触发原生通知，而不是继续依赖页面手动测试调用。
- 将共享平台能力里的桌面通知入口从仅支持 `title`/`body` 的薄封装升级为结构化业务通知 payload，保留通知标识、类型、标题、正文、时间戳、可选 `href` 与投递策略。
- 为桌面通知补齐去重、前台抑制、托盘未读摘要同步和投递结果事件，使桌面增强层可观测、可诊断，但不改变现有通知 API、通知 store 和已读语义作为业务真相来源。
- 收敛本次调研确认但暂不纳入实现的剩余 Tauri 缺口边界，包括插件生命周期事件桥、窗口/菜单原生控制和桌面通知交互回调，避免把本 change 扩大成宽泛的桌面总重构。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `desktop-native-capabilities`: 桌面通知能力从手动 title/body 调用升级为结构化业务通知投递合同，并明确非桌面降级与 tray 同步语义。
- `desktop-runtime-event-bridge`: 增加桌面通知投递结果事件，要求 delivered/suppressed/failed 等结果以规范化桌面事件回传前端。
- `desktop-notification-delivery`: 把桌面通知来源固定到现有通知 hydration 和 websocket 流，并补齐去重、前台抑制、未读摘要与失败降级要求。

## Impact

- Affected frontend: `lib/platform-runtime.ts`, `hooks/use-platform-capability.ts`, `lib/stores/notification-store.ts`, `lib/stores/ws-store.ts`, `components/layout/dashboard-shell.tsx`, `components/layout/header.tsx`
- Affected desktop shell: `src-tauri/src/lib.rs`, `src-tauri/capabilities/default.json`, `src-tauri/capabilities/desktop.json`
- Affected contracts and tests: desktop runtime/event tests, notification store/ws integration tests, dashboard-shell notification bridge coverage, and any desktop notification result assertions
