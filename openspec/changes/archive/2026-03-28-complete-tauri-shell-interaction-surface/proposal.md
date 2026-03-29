## Why

AgentForge 的 Tauri 基础壳能力、桌面通知桥和 updater 发布链路已经基本成形，但 repo 里仍然缺少最后一段操作闭环: 插件生命周期的桌面增强事件、窗口与原生菜单控制、以及通知点击后的桌面回跳和动作承接。现有归档 change 也已经把这些列为剩余 backlog，所以现在需要用一个 focused change 把 Tauri 交互面收口，避免桌面模式继续停在“能运行、但交互不完整”的状态。

## What Changes

- 为桌面模式定义统一的 shell 交互面，覆盖主窗口显示/聚焦/最小化等原生窗口控制，以及 tray 或原生菜单触发的受支持快捷动作。
- 把现有后端插件生命周期事件与桌面增强层接通，向前端暴露规范化的插件桌面事件，但保持插件业务真相和生命周期变更仍由现有 Go 控制平面负责。
- 为桌面通知补齐点击回跳和动作承接语义，使通知在桌面模式下可以恢复窗口、导航到目标页面，并保留失败或不支持时的稳定降级结果。
- 扩展共享平台能力 facade 与桌面事件桥，使前端通过统一入口消费窗口、菜单、通知交互和插件桌面事件，而不是页面零散直连 Tauri 原始 API。

## Capabilities

### New Capabilities
- `desktop-shell-actions`: 定义桌面窗口控制、原生菜单或 tray 快捷动作，以及通知点击后的路由和动作承接合同。

### Modified Capabilities
- `desktop-native-capabilities`: 增加窗口控制与 shell 级快捷动作的统一 facade 和非桌面降级语义。
- `desktop-runtime-event-bridge`: 扩展为可向前端暴露插件生命周期桌面事件和 shell 交互结果事件，同时保持业务主链路仍走现有后端控制面。
- `desktop-notification-delivery`: 从“投递结果”扩展为支持桌面通知点击后的回跳、聚焦和目标承接语义。

## Impact

- Affected frontend: `lib/platform-runtime.ts`, `hooks/use-platform-capability.ts`, desktop-aware layout or shell components, plugin/runtime-facing surfaces that consume desktop events
- Affected desktop shell: `src-tauri/src/lib.rs`, `src-tauri/capabilities/*.json`, tray/menu wiring, notification interaction handling
- Affected backend integration: existing plugin lifecycle websocket or projection surfaces that feed additive desktop event summaries
- Affected tests: platform runtime tests, dashboard shell or plugin page desktop-event coverage, Tauri command tests, and notification interaction or route handoff assertions
