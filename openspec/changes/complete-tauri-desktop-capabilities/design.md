## Context

AgentForge 的产品文档已经把桌面模式定义为正式交付面，而不是模板附带能力。`docs/PRD.md` 与 `docs/part/PLUGIN_SYSTEM_DESIGN.md` 对 Tauri 层的描述是一致的：

- Tauri 是 Layer 0 桌面壳，负责窗口、原生 OS 能力、sidecar 进程树和跨层事件广播。
- 桌面模式需要同时管理 Go Orchestrator 与 TS Bridge，而不是只起一个后端进程。
- 前端应通过统一的平台能力入口消费 `select_files`、`send_notification`、`update_tray`、`register_shortcut`、`check_update` 等桌面增强能力，并在 Web 模式下自动降级。
- Tauri 可以为桌面模式提供插件状态查询与事件转发，但不能取代 Frontend ↔ Go ↔ TS 的业务通信主链路。

当前仓库的 `src-tauri` 仍明显停留在模板阶段：

- `src-tauri/src/lib.rs` 只管理 `server` sidecar 和 `get_backend_url`。
- `src-tauri/tauri.conf.json` 仍保留 `react-quick-starter` 的产品名、标识和仅单 sidecar 打包配置。
- `src-tauri/capabilities/default.json` 只允许执行 `server` sidecar，没有通知、事件或其他桌面能力权限。
- 前端目前只有 `lib/backend-url.ts` / `hooks/use-backend-url.ts` 这条桌面接缝，没有统一的平台能力 hook，也没有桌面运行态或插件事件入口。

这个 change 的目标不是实现完整插件市场、工作流或云端部署，而是先把文档已经承诺的 Tauri 桌面层最小完整闭环定义清楚，避免后续桌面功能继续散落在模板残留、直接 `invoke` 调用和业务面板临时补丁里。

## Goals / Non-Goals

**Goals:**
- 定义 Tauri 对双 sidecar 的监督模型，包括启动顺序、运行态、就绪判定、退出处理和恢复语义。
- 定义桌面原生能力的统一访问模型，并明确每项能力在 Web 模式下的 fallback。
- 定义桌面事件与状态桥接模型，让前端能够消费 sidecar 状态、插件状态和桌面专属事件，而无需把 Tauri 变成业务 API 中枢。
- 收敛 Tauri 相关配置真相，包括产品标识、sidecar 打包产物、capability 权限和前端入口约定。
- 为后续 `/opsx:apply` 留下分阶段实现路径和可验证的验收边界。

**Non-Goals:**
- 不改写现有 Frontend ↔ Go ↔ TS 的业务主链路，不把插件业务调用改成走 Tauri IPC。
- 不在本次内定义完整的窗口编排、多窗口插件沙箱、菜单系统或移动端能力。
- 不在本次内实现远程更新服务器、签名分发、云端控制平面或桌面安装器细节。
- 不扩展插件市场、角色编辑器、工作流设计器等产品面板本身的业务范围，只补它们与桌面壳交互所需的契约。
- 不把所有未来可能的 Tauri 插件都纳入第一波，只覆盖文档已反复承诺的桌面增强能力。

## Decisions

### 1. 用“Desktop Runtime Manager”统一管理 Go 与 TS Bridge sidecar，而不是继续单点启动

Tauri 层应维护一个显式的桌面运行态对象，至少包含：

- `backend`: URL、pid/label、status、lastStartedAt、restartCount、lastError
- `bridge`: URL、pid/label、status、lastStartedAt、restartCount、lastError
- `overall`: `starting | ready | degraded | stopped`

启动顺序采用：

1. Tauri 启动 Go Orchestrator sidecar
2. 等待 Go 侧健康或就绪信号
3. 启动 TS Bridge sidecar，并将 Go 地址显式传入
4. 两者均 ready 后再把桌面壳状态标记为 `ready`

这样做的原因：

- 与 PRD/插件系统文档里“Rust 壳 -> Go -> TS Bridge”的进程树一致。
- 避免当前只知道后端 URL、不知道 bridge 是否存在的半截桌面状态。
- 让前端与运维面板有统一运行态可消费，而不是从日志文本反推。

备选方案：

- 继续只由 Tauri 启动 Go，再由 Go 间接启动 TS Bridge，Tauri 不持有 bridge 状态。缺点是桌面壳无法判断 bridge 故障，也无法可靠做重启和 UI 告警。
- 把 Go 和 TS Bridge 都交给外部脚本启动。缺点是会破坏桌面安装包的一体化交付承诺。

### 2. 前端统一通过平台能力 facade 消费桌面能力，不允许页面零散直连 `invoke(...)`

桌面能力不应继续像 `get_backend_url` 一样各处散落。设计上应增加一个统一的平台能力入口，例如：

- `lib/platform-runtime.ts` 负责环境判断、事件 schema、基础命令调用
- `hooks/use-platform-capability.ts` 负责为 React 页面提供可直接使用的方法

该 facade 统一暴露：

- `selectFiles`
- `sendNotification`
- `updateTray`
- `registerShortcut`
- `checkForUpdate`
- `getDesktopRuntimeStatus`
- `subscribeDesktopEvents`

并且逐项定义 Web fallback：

- 文件选择 -> HTML file input
- 系统通知 -> Web Notification API
- 托盘提示 -> 页面标题闪烁或应用内 badge
- 全局快捷键 -> 明确返回 unsupported
- 自动更新 -> Web 环境直接返回 not-applicable

这样做的原因：

- 保持 Web 与桌面模式共享同一套前端调用面。
- 防止插件页、任务页、设置页未来各自发明新的桌面检测方式。
- 让能力缺失与降级语义统一，而不是分散在若干 `try/catch` 中。

备选方案：

- 允许各页面直接 `import("@tauri-apps/api/core")` 并手写降级。缺点是会快速形成调用风格漂移和错误语义不一致。

### 3. 桌面事件桥接只做“状态与增强能力”，不接管业务通信

Tauri 需要向前端暴露两类事件：

- sidecar/runtime 事件：启动、ready、degraded、restarting、terminated
- desktop-enhanced 事件：插件状态变化的桌面广播、原生通知确认、快捷键触发、更新检查结果

但它不应承载业务主数据流。设计约束如下：

- 任务、插件、角色、工作流的业务读写仍走 Frontend ↔ Go HTTP/WS
- Tauri 只提供只读状态查询、系统级事件广播和桌面能力命令
- 若某类信息既可从 Go API 获取，也可由 Tauri 转发，则 Tauri 只提供“更快的桌面态提示”，不成为唯一来源

这样做的原因：

- 与 PRD 中“插件系统必须在有无 Tauri 时都能工作”一致。
- 能防止桌面模式与 Web 模式出现两套业务协议。
- 保持测试与后续云端部署路径更简单。

备选方案：

- 把插件状态和运行时查询都改成先问 Tauri。缺点是 Web 模式会被迫实现另一套兼容层，且 Tauri 容易膨胀成新的 API 网关。

### 4. Tauri 配置、权限和品牌信息必须同步收敛到 AgentForge 真相

当前 `tauri.conf.json` 和 Cargo 元数据仍保留模板仓库痕迹，这在桌面分发阶段会直接暴露成错误产品身份。这个 change 明确要求：

- `productName`、窗口标题、`identifier`、描述和打包名收敛到 AgentForge
- `externalBin` 和相关 build 流程能够同时覆盖 `server` 与 `bridge`
- `src-tauri/capabilities/default.json` 只增加实现本 change 所需的最小权限
- sidecar 名称、Rust 注册、前端 guest package 与 capability 权限一一对应

这样做的原因：

- 避免桌面分发继续带着模板产品名和残缺 sidecar 列表进入后续实现。
- 把权限声明变成显式合同，便于后续做最小权限审计。

备选方案：

- 先只改 Rust 代码，后面再补配置。缺点是桌面功能通常会在 build 时一起失败，配置与代码分离修改很容易遗漏。

### 5. 实施顺序按“配置基线 -> 运行时监督 -> 原生能力 -> 面板接入”推进

推荐实现顺序：

1. 清理 Tauri 品牌/配置/打包基线
2. 建立双 sidecar runtime state 与 readiness/restart 机制
3. 接入原生能力命令和前端 facade
4. 将插件或运行时面板接入桌面状态/事件
5. 补齐桌面与 Web 的验证矩阵

这样做的原因：

- 先把最底层配置和进程树收稳，后面接前端时不会反复改协议。
- 能将复杂度拆成清晰的纵切任务，便于 `tasks.md` 形成可执行计划。

备选方案：

- 先从前端 hook 开始模拟能力。缺点是会把很多关键协议留到后面，造成回填成本更高。

## Risks / Trade-offs

- [Risk] 双 sidecar 监督会增加 Tauri 壳复杂度和状态分支 -> Mitigation: 用单一 `DesktopRuntimeState` 统一表达，避免每个命令各自维护局部状态。
- [Risk] 原生能力与 Web fallback 容易出现行为不一致 -> Mitigation: 先定义统一 facade 和每项能力的失败/unsupported/not-applicable 语义，再做页面接入。
- [Risk] 插件状态既有 Go API 又有 Tauri 事件，可能造成短暂不一致 -> Mitigation: 明确 Go API 为业务真相，Tauri 事件仅作桌面增强提示与即时状态快照。
- [Risk] 自动更新和全局快捷键在不同平台差异较大 -> Mitigation: 在 spec 中先约束最低行为和失败语义，不把平台专有实现细节塞进第一波需求。
- [Risk] 桌面 build 需要新增 Rust/guest 依赖与 capability 权限，容易与现有配置不同步 -> Mitigation: 把配置、Cargo、capability、前端 guest package 视为同一任务族同步变更。

## Migration Plan

1. 基线阶段：对齐 `src-tauri` 产品标识、external binaries、capability 文件与当前 AgentForge 命名。
2. 运行时阶段：引入双 sidecar runtime state、ready 判定、退出捕获与重启策略，并向前端暴露 `get_desktop_runtime_status`。
3. 能力阶段：补齐文件选择、通知、托盘、快捷键、更新检查命令，并建立前端平台能力 facade 与 Web fallback。
4. 集成阶段：把插件面板或其他桌面敏感页面接入桌面运行态与事件订阅，不改变原有业务 API 来源。
5. 验证阶段：覆盖 `pnpm tauri dev`、前端 fallback、Tauri capability 权限、sidecar 丢失/重启、桌面事件与基本 UI 消费链路。

回滚策略：

- 如果双 sidecar 监督链路不稳定，可以先保留单 sidecar 启动，但桌面状态必须显式标记 `bridge_unmanaged` 或同等退化态，不能伪装成完整实现。
- 如果某个原生能力在当前平台不可用，可回退到 facade 中的 `unsupported` / `not_applicable` 返回，而不是删除整个平台能力入口。
- 如果插件事件桥接不稳定，可先保留桌面运行态查询和系统通知，延后插件事件流，但不回退配置与统一 facade。

## Open Questions

- TS Bridge 在桌面模式下最终是由 Tauri 直接拉起，还是允许 Go 作为 bridge 的次级 supervisor，同时把状态回报给 Tauri？
- 插件状态查询是否需要单独的 Tauri command，还是由前端继续读 `/api/v1/plugins`，Tauri 只补充事件流与运行态快照？
- 托盘第一波是否只需要状态徽标和点击唤醒，还是要同时包含任务/插件快捷入口菜单？
- 自动更新第一波是否只约束“检查与提示”，还是要求包含下载后重启安装流程？
