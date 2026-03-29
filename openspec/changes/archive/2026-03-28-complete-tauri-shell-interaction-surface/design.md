## Context

AgentForge 当前的桌面壳已经覆盖了双 sidecar 监督、tray 更新、全局快捷键、结构化桌面通知和 updater 流程, 这些能力主要集中在 `src-tauri/src/lib.rs`、`lib/platform-runtime.ts` 和 `hooks/use-platform-capability.ts`。但剩余的 Tauri 缺口已经收敛到交互层:

- `hooks/use-platform-capability.ts` 只暴露文件选择、通知、tray、shortcut、updater 和 runtime 查询, 还没有窗口控制或 shell action API。
- `src-tauri/src/lib.rs` 目前只有 tray 点击唤醒窗口，没有原生菜单或统一的 shell action 事件合同，也没有桌面通知点击后的路由承接。
- `lib/stores/ws-store.ts` 已经消费大量业务 websocket 事件，但没有接入后端现有的 `plugin.lifecycle` 事件，因此插件桌面增强事件仍是缺口。
- `components/layout/dashboard-shell.tsx` 负责认证、通知桥和 websocket 连接，天然是承接桌面交互协调层的地方；相对地，页面级零散调用或 Rust 直接接管路由都会把 Web/桌面模式撕成两套协议。

这次 change 的目标不是再扩一轮泛化桌面框架，而是把剩余的交互闭环补齐：窗口与菜单控制、通知点击回跳、插件生命周期桌面事件。

## Goals / Non-Goals

**Goals:**

- 定义一个统一的桌面 shell action 合同，覆盖窗口控制、tray 或原生菜单快捷动作，以及通知点击后的动作承接。
- 把 `plugin.lifecycle` 等现有后端事件接入桌面增强事件流，让前端通过一个统一订阅入口观察 desktop-native 与 desktop-enhanced 交互。
- 保持桌面模式通过共享 platform facade 消费上述能力，并保留清晰的 web fallback 或 unsupported 结果。
- 保持插件生命周期变更、通知 read 状态和业务路由真相仍然留在现有 frontend + Go 控制面，而不是迁入 Rust。

**Non-Goals:**

- 不在本次内引入多窗口编排、工作区窗口沙箱或完整原生菜单体系。
- 不在本次内让 Rust 直接建立第二条带认证的后端 websocket 或 HTTP 控制通道。
- 不在本次内把插件 enable/disable/restart 等变更迁到 Tauri command；这些仍由既有后端 API 负责。
- 不在本次内引入移动端或非 Tauri 容器的专用交互协议。

## Decisions

### 1. 由前端壳层持有 shell coordination，而不是让 Rust 直接控制页面路由

桌面交互的统一协调层应挂在认证后的壳层，例如 `components/layout/dashboard-shell.tsx` 或同级桌面协调模块。这个协调层负责：

- 订阅 `platformRuntime.subscribeDesktopEvents(...)`
- 订阅现有 websocket 里的 `plugin.lifecycle`
- 根据规范化的 shell action 或 notification activation 决定 `router.push(...)`、窗口聚焦和后续 backend 调用

这样做的原因：

- 路由、认证态和现有 store 都已经在前端壳层，Rust 不需要重复理解页面路径、权限或 session。
- Web 与 desktop 可以共用同一套业务路由逻辑，只把“如何触发”差异限制在 facade。
- 避免 Rust 为了插件事件或通知回跳再建立第二条业务控制平面。

备选方案：

- 让 Rust 直接维护深链接与页面跳转。缺点是会把前端路由知识复制到 `src-tauri`，并让调试和测试更脆弱。
- 让每个页面自己订阅桌面事件。缺点是逻辑会碎裂，通知、插件、窗口行为难以统一去重。

### 2. 扩展 platform facade 为 shell action API，而不是开放原始 Tauri 窗口或菜单调用

`lib/platform-runtime.ts` 应扩展统一 API，例如：

- `showMainWindow()`
- `setMainWindowState("minimized" | "restored" | "focused")`
- `triggerShellAction(...)`
- `subscribeDesktopEvents(...)` 返回包含 `runtime.*`、`notification.*`、`shell.action.*`、`plugin.lifecycle.*` 的统一事件流

这样做的原因：

- 当前 repo 已经接受 “desktop capability 只能经由 facade” 的模式，继续沿用最稳。
- 前端不需要知道 `@tauri-apps/api/window`、tray 菜单或通知插件的原始细节。
- web fallback 可在同一层稳定返回 `unsupported` 或 `not_applicable`，不会让调用点散落 `try/catch`。

备选方案：

- 允许页面直接调用 Tauri window/menu API。缺点是会快速形成调用漂移，并破坏现有 capability contract。

### 3. 插件生命周期桌面事件走“前端合流”，不让 Rust 直接复制 backend websocket

现有 `plugin.lifecycle` 事件已经由 Go websocket 暴露，但前端还没有消费。为了保持单一认证链路，设计上应由前端 websocket store 或 desktop coordination 层消费该事件，再将其合流进 `platformRuntime` 暴露的统一 desktop event 订阅面。

这样做的原因：

- 复用现有 websocket 客户端和认证 token，不需要 Rust 再建立一条与 backend 对等的业务连接。
- 插件生命周期事件本质上是 backend truth；desktop event bridge 只是 additive projection。
- 可以让插件页在 event bridge 不可用时继续依赖现有 API 和 websocket store，不被 Tauri 绑定。

备选方案：

- 让 Rust 直接连接 Go websocket。缺点是要引入 token 注入、重连、权限和状态同步的新复杂度。

### 4. 通知点击、tray/menu 选择和窗口动作共用一个 shell action envelope

所有桌面交互回调都应归一为一套 envelope，例如：

- `source`: `notification` | `tray` | `menu` | `window`
- `actionId`: `open_notification_target` | `open_plugins` | `focus_main_window` | `refresh_plugin_runtime`
- `href`: 可选目标路由
- `payload`: 可选实体上下文，如 `notificationId`、`pluginId`
- `status`: `triggered` | `completed` | `failed` | `unsupported`

这样做的原因：

- 通知点击和菜单动作最终都要落到“恢复窗口 + 路由/调用既有业务入口”。
- 统一 envelope 能让前端测试、日志和诊断面共享一套断言方式。
- 后续如果增加更多桌面交互，不需要再发明新事件格式。

备选方案：

- 每类来源使用独立事件格式。缺点是订阅方要写多套解析逻辑，未来演进困难。

### 5. Shell 快捷动作如果涉及插件变更，必须回到既有 backend control plane

如果 tray/menu 提供插件相关快捷动作，这些动作只能触发前端已有的 backend 请求或 store action，例如刷新 runtime summary、打开插件页、调用现有 enable/disable/restart API。Rust 不直接执行插件 lifecycle mutation。

这样做的原因：

- 现有 `desktop-runtime-event-bridge` 已经要求 Tauri helper 是 read-only projection。
- 后端保留审计、权限、持久化和 websocket 广播真相；Rust 只做壳层增强。
- 可以避免 desktop-only mutation 让 Web 与 desktop 的插件行为分叉。

备选方案：

- 提供 Tauri-only plugin mutation commands。缺点是会形成第二套不可审计的控制面。

## Risks / Trade-offs

- [前端合流 shell events 与 websocket plugin events 后，事件源变复杂] -> 用统一 envelope 和 `source` 字段区分 native shell 与 backend-projected desktop events。
- [跨平台通知点击回调支持度不一致] -> 规范要求暴露稳定的 `failed` 或 `unsupported` 结果，且通知本身的 delivered 语义不依赖点击回调成功。
- [tray/menu 快捷动作过宽会把 Tauri 变成业务入口] -> 第一波只允许少量 route-first 或 read-only quick actions，涉及 lifecycle mutation 的动作必须显式回到 backend API。
- [壳层协调器职责变重] -> 保持它只做订阅、路由承接和 facade 调用，把具体 UI 或 store 更新仍留在现有模块。

## Migration Plan

1. 为 `desktop-shell-actions` 创建新 spec，并为 `desktop-native-capabilities`、`desktop-runtime-event-bridge`、`desktop-notification-delivery` 增加对应 delta。
2. 扩展 `lib/platform-runtime.ts` 和 `hooks/use-platform-capability.ts`，加入窗口控制与 shell action contract。
3. 在 `src-tauri/src/lib.rs` 增加窗口控制、tray/menu action 与 notification activation 的统一事件发射。
4. 在前端 websocket 或 desktop coordination 层接入 `plugin.lifecycle`，并把它合流到统一 desktop event 订阅面。
5. 让认证后的壳层消费 shell action envelope，实现恢复窗口、路由跳转和既有 backend action 转发。
6. 用平台 runtime、dashboard shell、插件页与通知桥测试覆盖 desktop success、web fallback、notification activation、plugin lifecycle projection 与 failure diagnostics。

回滚策略：

- 如果通知点击或 tray/menu action 在某个平台不稳定，可先保留 shell action envelope 与失败结果，不阻断原有通知投递和 tray 更新。
- 如果 plugin lifecycle desktop projection 的前端合流实现过于噪声，可先只暴露插件页需要的最小事件子集，保留 summary 查询路径。
- 如果窗口控制 API 第一波边界不稳，可先收缩到 `show/focus main window` 与 `minimize`，不阻塞通知回跳合同。

## Open Questions

- 第一波 shell quick actions 是否只覆盖路由型动作和 runtime refresh，还是要同时提供少量 plugin lifecycle shortcut？
- 通知点击默认是否应自动跳转 `href`，还是先在壳层给出“恢复窗口 + 打开通知中心”的保守模式？
- tray 与原生菜单是否共用完全同一套 action registry，还是允许 tray-only 的最小动作子集？
