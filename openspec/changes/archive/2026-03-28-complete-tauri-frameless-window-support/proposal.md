## Why

AgentForge 的桌面壳已经具备 sidecar 监督、tray、通知、快捷键、updater 和基础窗口动作合同，但主窗口仍然依赖原生边框和 Web-first header。只要真正切到无边框窗口，当前仓库就缺少共享标题栏、拖拽安全区、最大化/关闭按钮状态同步，以及跨登录页与 dashboard 的一致窗体壳，所以现在需要把这条 seam 收敛成明确的 OpenSpec 契约，而不是继续做页面级临时补丁。

## What Changes

- 为 AgentForge 桌面主窗口定义共享的 frameless window chrome 能力，覆盖无边框窗口、自定义标题栏、拖拽区与系统按钮集，并要求这些能力在桌面主窗口的主要路由壳层中统一提供。
- 扩展共享平台能力 facade 与 shell action 合同，使前端能够通过统一入口完成最小化、最大化/还原、关闭，以及读取和订阅只读窗口状态，而不是在页面里直接导入原始 Tauri window API。
- 为桌面事件订阅面补齐窗口状态投影，确保自定义标题栏在 shell action、双击拖拽区或其他原生窗口手势触发状态变化后仍能保持同步。
- 约束 frameless 集成方式，使现有 `Header`、`DashboardShell`、认证页和其他主窗口路由在桌面模式下能共用统一窗体壳，同时保留 Web 模式的现有布局与降级行为。

## Capabilities

### New Capabilities
- `desktop-window-chrome`: 定义 AgentForge 桌面主窗口的无边框窗体壳、自定义标题栏、拖拽安全区和窗口控制状态同步合同。

### Modified Capabilities
- `desktop-native-capabilities`: 将共享 facade 从基础窗口动作扩展为支持 frameless chrome 需要的窗口状态查询、订阅与桌面降级语义。
- `desktop-shell-actions`: 将窗口控制合同扩展到自定义标题栏场景所需的最大化/还原与关闭动作，并要求结果保持规范化。
- `desktop-runtime-event-bridge`: 扩展桌面事件订阅面，使 frameless chrome 可消费窗口状态投影，而不需要页面级原生监听拼装。

## Impact

- Affected frontend: `app/layout.tsx`, auth/dashboard route shells, `components/layout/header.tsx`, `components/layout/dashboard-shell.tsx`, shared desktop frame/titlebar components, `lib/platform-runtime.ts`, and `hooks/use-platform-capability.ts`
- Affected desktop shell: `src-tauri/tauri.conf.json`, `src-tauri/capabilities/*.json`, and `src-tauri/src/lib.rs` shell action handling for frameless window control
- Affected verification: platform runtime tests, shell/layout integration tests, and Tauri-adjacent checks covering frameless control flows and window-state synchronization
