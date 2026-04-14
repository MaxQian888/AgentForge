## Context

AgentForge 当前的 IM Bridge 已经有比较完整的基础骨架：`src-im-bridge` 负责多平台 command/action/delivery，Go backend 负责 `/api/v1/im/*`、control-plane、task/review/wiki/automation 等业务工作流，Dashboard `/im` 已经提供 operator console、history、health、channel config 和 test-send UI。问题不在于“缺少一个新的基础设施层”，而在于几条跨模块集成链路仍然是半连通状态：`/im/test-send` handler 未注入真实 sender、channel/event subscription 被保存但没有驱动运行时路由、TS Bridge 的 event inventory 与 Go/IM 转发链不完全一致、以及后端 action executor 已支持 `save-as-doc` / `create-task` 但 IM 侧没有用户入口。

这次 change 是典型的 cross-cutting integration work：会同时触及 `src-go`、`src-im-bridge`、`src-bridge`、frontend `/im` 页面，以及若干已有 OpenSpec capability 的 source of truth。目标不是再造一条平行 IM 管理线，而是把现有 operator surface、channel registry、Bridge events 和 action executor 收束成同一个 runtime contract。

## Goals / Non-Goals

**Goals:**
- 让 `/im/test-send` 真正走 live sender 和 canonical delivery pipeline，并把 settlement 结果回到 `/im` 页面。
- 让 `IMChannel.Events` 和 channel registry 成为 channel-scoped IM 事件的 authoritative routing source，而不是只保存不消费。
- 让 TS Bridge → Go backend → IM Bridge 的 forwarded event inventory、过滤、顺序和 fallback 语义回到 repo truth，至少覆盖当前已经存在的 budget/status/permission 类事件。
- 把 backend 已支持的 `save-as-doc` / `create-task` action 通过 IM 的真实入口暴露出来，并保持 source message context、reply-target lineage 和用户可见结果。
- 保持 Go backend 作为中枢拓扑，不重新引入 TS Bridge 直接依赖 IM Bridge 的旁路描述或实现。

**Non-Goals:**
- 不新建第二套 IM 管理页面或 debug-only 端点；继续复用 `/im` 页面与现有 `/api/v1/im/*` surfaces。
- 不在本次 change 中引入全新的持久化订阅系统或把 `IMActionBinding` 从内存状态升级为跨 backend restart 持久恢复机制。
- 不重做各平台 adapter 的基础连接模型；平台 transport/provider contract 继续沿用现有实现，只补必要的 action exposure 和 delivery truth。
- 不把 wiki mention 直接扩展成完整的“按用户 IM 身份一对一映射”系统；如果当前 repo 缺少 user-to-IM identity 映射，本次 change 只定义 truthfully degraded behavior。

## Decisions

### 1. 继续复用现有 `IMControlHandler`，通过注入真实 sender 修复 `/im/test-send`
- **Decision**: 在现有 `handler.NewIMControlHandler(control, sender)` seam 上注入 `imSvc` 或等价 sender，而不是新增专门的 operator-test endpoint 或绕过 `IMService` 的独立实现。
- **Why**: `IMControlHandler.TestSend` 已经定义了正确的 bounded-wait settlement contract，缺的是 wiring，不是协议本身。复用现有 seam 可以让 test-send 和普通 send/notify 共享同一条 delivery pipeline、history、retry 与 metadata 逻辑。
- **Alternatives considered**:
  - 新增独立 debug sender endpoint：实现快，但会让 operator console 和真实 runtime 分叉。
  - 前端直接打 `/im/send`：拿不到 bounded settlement 结果，也会让 operator flow 失去 canonical feedback。

### 2. 把 IM 路由明确分成两类：bound reply-target routing 与 channel/event routing
- **Decision**: 异步进度、terminal update、permission request、review follow-up 继续以 `IMActionBinding` / reply-target lineage 为 authoritative source；wiki/doc events、automation-triggered IM messages、以及其他非绑定式 broadcast delivery 改由 configured channels + subscribed events 决定目标。
- **Why**: 这两类消息的目标解析逻辑本来就不同。把它们混成“都靠 watcher”或“都靠单一 channel”会持续制造 drift。明确双路由模型后，spec、operator console、backend delivery code 与 IM Bridge 的职责边界会更稳定。
- **Alternatives considered**:
  - 全部统一成 channel routing：会破坏已有 bound reply-target progress/control-plane contract。
  - 全部统一成 binding routing：无法解释 wiki/automation 这类没有单一 originating IM conversation 的广播事件。

### 3. Channel registry 成为 channel-scoped event routing 的主入口，legacy env target 只保留为 compatibility fallback
- **Decision**: 当存在 active channel 且 event subscription 匹配时，运行时必须优先使用 channel registry；只有在没有匹配 channel 且 repo 仍配置 legacy env target 时，才允许走现有的单一 fallback target。
- **Why**: 这能让 `/im` 配置真正生效，同时不破坏仍依赖 `cfg.IMNotifyPlatform` / `cfg.IMNotifyTargetChatID` 的老环境。
- **Alternatives considered**:
  - 立即移除 env fallback：更干净，但会让未完成 channel 配置的环境直接退化。
  - 继续让 env fallback 始终优先：会使 `/im/channels` 继续沦为只读展示数据。

### 4. 以 canonical forwarded event inventory 收口 TS/Go/UI 的事件类型真相
- **Decision**: 把 forwarded Bridge events 视为显式 inventory：TS event types、Go `BridgeEvent*` 常量、`/api/v1/im/event-types`、channel event subscriptions 和 operator console 共享一套可验证的事件目录。对于 repo 中已实际产生但尚未接线的事件（例如 TS `budget_alert`），补齐 Go 侧解码与转发；对于 repo 中尚不存在的 fancy watcher semantics，不在本次 change 中虚构实现。
- **Why**: 当前最大问题是 inventory drift，不是缺少更多事件名。先让“已经存在的事件”在三层之间一致，才谈更高级的 user preference 模型。
- **Alternatives considered**:
  - 继续维护静态前端事件列表：短期简单，但会重复现在的 drift。
  - 一次性引入完整 watcher/subscription system：范围过大，且与现有 binding-centric repo truth 不对齐。

### 5. 复用 backend action executor，补 IM 可达入口，而不是再造新命令路径
- **Decision**: `save-as-doc` / `create-task` 继续由 `BackendIMActionExecutor` 执行；`src-im-bridge` 只负责把这些 action 变成真实 card/button/interaction entrypoints，并把 source message metadata 带回 `/im/action`。
- **Why**: 后端 workflow 已经存在，缺的是入口。重复造 `/task from-message` 或 `/doc from-message` 命令只会制造平行实现。
- **Alternatives considered**:
  - 新增 slash commands：可见性高，但会复制 message conversion logic。
  - 前端 dashboard 侧单独实现 message conversion：绕过 IM action contract，不符合当前拓扑。

## Risks / Trade-offs

- **[跨模块改动较多]** → 通过 capability 拆分、focused tests 和现有 seam 复用控制 blast radius，避免把 change 扩成平台基础设施重构。
- **[legacy env fallback 与 channel routing 共存可能让行为更复杂]** → 在 spec 中明确优先级：configured channel first，legacy env 仅在无匹配 channel 时兜底，并在 delivery metadata/diagnostics 中保持可见。
- **[Bridge event inventory 仍可能再次漂移]** → 把 TS constants、Go constants、`/im/event-types` 与 frontend 消费加入 focused verification，防止只改一层。
- **[message conversion entrypoints 可能依赖平台能力差异]** → action id 和 backend result contract 保持统一，平台仅在 UI affordance 和 callback normalization 上做最小差异化处理。
- **[用户会期待更强的个人 IM 映射或 backend 重启恢复]** → 在 spec 中显式声明本次只补 runtime integration completeness，不把 scope 擴大到 identity mapping 或 durable binding persistence。

## Migration Plan

1. 先修复 backend wiring：把 `/im/test-send` 接到真实 sender，并补齐相关 handler/service tests。
2. 引入 channel/event routing helpers，让 wiki/automation/channel-scoped broadcast delivery 先消费 channel registry，再兼容 legacy env fallback。
3. 收口 forwarded event inventory，补齐 TS `budget_alert` 等现有 event type 的 Go 侧接线，并让 `/im/event-types` 与 routing inventory 同源。
4. 在 `src-im-bridge` 暴露 `save-as-doc` / `create-task` 的用户入口，保持 `/im/action` payload normalization 与 reply-target context 不变。
5. 更新 `/im` 页面、store 与 focused tests，确保 operator console 的 test-send/history/health/event inventory 都反映新的 runtime truth。
6. 回滚策略：若 channel routing 或 event parity 变更导致行为异常，可保留 legacy env fallback 和既有 bound progress 路径，逐段关闭新增 routing consumption，而不需要回退整个 IM Bridge 基座。

## Open Questions

- 是否在本次 change 内，把 wiki comment mention 的 IM 行为限定为“全局 configured route + in-app notification”即可，还是要为未来的 user-to-IM identity mapping 预留更明确的 extension point？
- `/api/v1/im/event-types` 是否只暴露 channel-scoped broadcast events，还是也需要区分 bound-progress-only event types 以避免 operator 在 channel config 中看到无意义选项？
