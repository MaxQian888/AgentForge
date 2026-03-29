## Context

AgentForge 当前的桌面能力已经把 Tauri 从模板壳推进到了可运行、可通知、可更新、可响应 shell action 的状态，但主窗口仍使用原生装饰边框。现有实现真相是：

- `src-tauri/tauri.conf.json` 里的 `main` 窗口仍是默认 decorated window，没有 frameless/window chrome 约束。
- `src-tauri/src/lib.rs` 已通过 `perform_shell_action` 支持 `focus/show/restore/minimize` 等动作，但还没有为自定义标题栏收口最大化/还原、关闭与窗体状态同步。
- `lib/platform-runtime.ts` 与 `hooks/use-platform-capability.ts` 已经是桌面能力 facade，当前只暴露基础窗口动作，不足以支撑 frameless titlebar 的持续状态同步。
- `components/layout/dashboard-shell.tsx` 与 `components/layout/header.tsx` 仍按普通 Web 顶栏布局组织；`app/layout.tsx` 还没有桌面主窗口级别的共享 frame，因此登录页、dashboard 与其他主窗口路由无法自然共享一套无边框窗体壳。

这次 change 的目标不是泛化多窗口体系，也不是重写页面导航，而是补齐 AgentForge 主窗口的 frameless seam：一个统一的桌面窗体壳、可同步的标题栏状态、以及不破坏现有 Web 与 dashboard 结构的集成路径。

## Goals / Non-Goals

**Goals:**

- 为 AgentForge 主窗口定义一个共享的 desktop window frame，使桌面模式可以安全关闭原生边框并渲染自定义标题栏。
- 保持窗口控制、状态查询和桌面事件仍然经过共享 platform facade，而不是让页面直接接触原始 Tauri window API。
- 确保登录页、dashboard 以及其他主窗口路由在桌面模式下获得一致的 drag region 和系统按钮，同时不破坏 Web 模式布局。
- 让 frameless chrome 在按钮点击、拖拽区双击、原生窗口手势等状态变化后保持可观测、可测试、可恢复。

**Non-Goals:**

- 不在本次内引入多窗口编排、独立插件子窗口、工作区窗口沙箱或原生停靠布局。
- 不在本次内重写现有 dashboard `Header` 的业务内容，只调整它与桌面窗体壳的职责边界和布局关系。
- 不在本次内追求每个平台全部原生标题栏特性的完全复刻，例如完整系统菜单、所有平台专属振动/材质或透明毛玻璃效果。
- 不在本次内把已有 tray、notification、plugin lifecycle 或 updater 业务路径迁出当前 facade / backend truth。

## Decisions

### 1. 在 app shell 边界引入共享 Desktop Window Frame，而不是把 frameless 逻辑缝进单个页面头部

frameless 支持的真实边界是“主窗口壳”，不是单个 dashboard 页面。设计上应新增一个共享的 `DesktopWindowFrame` / `DesktopTitlebar` 层，挂在 root layout 或同等级别的主窗口路由壳上，让 auth routes、dashboard routes 与其他主窗口页面都能复用同一套标题栏、拖拽区和按钮集。

这样做的原因：

- 用户的“功能完整”要求意味着不能只让 dashboard 有标题栏，而让登录页或其他主窗口路由失去关闭/拖拽入口。
- 现有 `DashboardShell` 与 `Header` 可以继续聚焦业务导航、通知和用户菜单，自定义窗体壳只处理 window chrome。
- 共享 frame 可以统一处理桌面与 Web 的布局差异，避免每个页面自己判断 Tauri 环境。

备选方案：

- 直接把无边框标题栏塞进 `components/layout/header.tsx`。缺点是只覆盖 dashboard，并把业务 header 与 window chrome 耦合在一起。

### 2. 继续沿用 facade 模式，但把 frameless 所需的“动作 + 只读状态”一起收口

窗口最小化、最大化/还原、关闭这类变更动作继续沿用共享 shell action contract，避免页面直接操作原始 Tauri command。与此同时，frameless chrome 需要额外的只读状态能力，例如当前窗口是否 maximized、minimized、visible 或 focused。设计上应把这些状态查询与订阅能力也包进 `platformRuntime`，由 facade 负责桌面与 Web 的统一返回。

这样做的原因：

- 当前 repo 已经接受“桌面能力必须经过 facade”的模式，frameless 继续沿用这一 seam 最稳。
- 自定义标题栏需要持续状态同步，只靠一次性按钮动作不够。
- 将原生 API 限制在 facade 内，可以避免页面级 `@tauri-apps/api/window` 导入扩散。

备选方案：

- 允许 titlebar 组件直接使用原始 Tauri window API。缺点是会绕开既有 contract，使 Web fallback 与测试边界变脆。

### 3. 采用“Rust 负责规范化 shell action，facade 负责窗口状态投影”的混合模型

对最小化、最大化/还原、关闭等显式动作，Rust 侧 `perform_shell_action` 仍应作为规范化入口，以保持 shell action event 与已有订阅面一致。对双击拖拽区、系统手势或被动状态变化，则由 facade 在桌面模式下消费 Tauri window 状态与事件，再把结果投影回共享订阅面，供 frameless chrome 与 shell 协调层消费。

这样做的原因：

- 现有 Rust shell action 路径已经稳定，不值得为 frameless 另造一条 mutation 通道。
- 仅靠 Rust action 事件无法覆盖所有被动状态变化；frameless titlebar 仍需要被动同步。
- 共享订阅面继续作为唯一消费入口，测试与诊断更集中。

备选方案：

- 所有窗体状态都由 Rust 主动推送。缺点是需要把更多平台事件监听与状态缓存搬进 `src-tauri/src/lib.rs`，复杂度更高。

### 4. Drag region 必须与交互控件显式分区，不能靠“整个 header 可拖拽”的粗粒度方案

frameless titlebar 应只把明确的标题栏空白区或品牌区标记为拖拽区；按钮、输入框、popover trigger、tabs、通知入口、语言切换和用户菜单都必须是 no-drag 区域。dashboard 现有 `Header` 继续保留为业务 header，不直接作为整个 drag 区。

这样做的原因：

- 当前 header 内已有 notification popover、dropdown trigger 和 mobile sidebar 入口，粗暴整条 header 设 drag 会直接吞掉点击。
- 拖拽边界不清是无边框桌面壳最常见的回归来源，必须在设计阶段明确。
- 只在 compact titlebar 上处理 drag 能让 dashboard 主 header 继续保持 Web 与 desktop 统一语义。

备选方案：

- 让整个顶部区域都可拖拽，再逐个排除控件。缺点是高噪声、回归面大，而且 auth/dashboard 两套顶部结构不一致。

## Risks / Trade-offs

- [Drag region 抢占点击或 hover 事件] -> 将 drag 区限制在 titlebar 专用区域，并为按钮、popover、dropdown、输入控件添加显式 no-drag 约束与测试。
- [最大化状态在原生手势后与自定义按钮图标不同步] -> 在 facade 中统一做状态查询与事件订阅，并在 shell action 完成后立即刷新状态投影。
- [只覆盖 dashboard 会让登录页或其他主窗口路由残缺] -> 通过共享 Desktop Window Frame 放到 app shell 边界，避免路由级补丁。
- [不同平台对 frameless 行为存在差异] -> 规范先要求稳定的最小集合：无边框、拖拽、最小化、最大化/还原、关闭和可观测状态；不把更深的材质/系统菜单复刻纳入第一波。

## Migration Plan

1. 先补 OpenSpec deltas，明确 `desktop-window-chrome` 新 capability 以及 facade、shell action、event bridge 的 requirement 变更。
2. 在 Tauri 配置层启用 frameless 主窗口基线，并为相关 window state 查询或动作声明最小必要权限。
3. 引入共享 `DesktopWindowFrame` / `DesktopTitlebar`，把桌面主窗口公共 chrome 提升到 root-level shell，再调整 dashboard/auth shells 的布局衔接。
4. 扩展 `platformRuntime` 和 `usePlatformCapability()`，补齐 frameless 所需动作、状态快照和订阅语义，同时保持 Web fallback。
5. 通过 focused tests 与桌面验证确认 drag/no-drag 边界、状态同步和非桌面降级稳定，再决定是否继续扩展更多平台细节。

回滚策略：

- 如果 frameless 集成在某个平台上不稳定，可先恢复 `decorations` 基线并保留 facade 中新增的窗口状态 contract，不让页面再次散落原始 API 调用。
- 如果被动窗口状态同步噪声过大，可先保留显式按钮动作与状态刷新，再缩减被动事件覆盖范围，但不能回退到页面直接控制窗口。

## Open Questions

- frameless 标题栏第一波是统一显示 `AgentForge` 品牌名，还是应允许按当前路由展示次级页面标题而不破坏 drag 区稳定性？
- 未来若引入独立工作区窗口或插件窗口，是否复用同一套 `desktop-window-chrome` primitives，还是定义更窄的 secondary-window 规范？
