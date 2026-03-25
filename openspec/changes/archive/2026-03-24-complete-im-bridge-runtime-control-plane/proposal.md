## Why

PRD、`PLUGIN_SYSTEM_DESIGN.md` 和 `CC_CONNECT_REUSE_GUIDE.md` 都把 IM Bridge 描述成 AgentForge 的双向控制面入口，而当前 `src-im-bridge` 主要只完成了“单平台收命令 + 直接 HTTP 发通知”的基础桥接。现在缺少 Bridge 实例注册、目标路由、受保护的回调入口、断线补发，以及长任务的进度回传闭环，这会让 IM Bridge 无法承接文档已经承诺的运行时职责，也会阻塞后续把 IM 能力接入更完整的插件和工作流体系。

## What Changes

- 为 IM Bridge 增加实例级控制面：启动注册、心跳/失活、`bridge_id` 标识、平台/项目元数据上报，以及后端按实例定向投递的基础契约。
- 补齐 Bridge 与后端之间的可靠回传通道，覆盖通知推送、长任务进度心跳、断线重连后的补发/续传，以及避免多实例重复消费的定向路由规则。
- 为 `/im/send`、`/im/notify`、回调或事件入口增加认证与签名校验约束，避免当前裸露 HTTP 端点被伪造请求直接驱动外部 IM 平台。
- 把 IM 长任务交互补全为“立即确认 + 运行中进度 + 最终结果”三段式体验，让 `/task assign`、`/agent run`、任务分解、审查等长时操作能在 IM 中保持可见、可追踪、可恢复。
- 对齐当前 IM Bridge 与插件/runtime 文档的实例模型，明确哪些能力仍由单平台进程承担，哪些状态和投递语义要以受控的 Bridge 运行时为准。

## Capabilities

### New Capabilities
- `im-bridge-control-plane`: 定义 Bridge 实例注册、心跳、认证、目标路由、断线恢复与补发的运行时契约。
- `im-bridge-progress-streaming`: 定义 IM 发起的长任务在确认、心跳、完成回传和恢复场景下的可见性契约。

### Modified Capabilities
- `additional-im-platform-support`: 现有平台支持能力需要补充受控实例元数据、可靠回传语义和长任务交互约束，而不仅是单次命令/通知收发。

## Impact

- Affected code: `src-im-bridge/cmd/bridge`, `src-im-bridge/core`, `src-im-bridge/client`, `src-im-bridge/notify`, 各平台 `live` 适配器，以及相关 smoke/test 代码。
- Affected backend surfaces: `src-go` 中 IM 相关 API、Bridge 注册与实例状态存储、定向通知分发、可能的 Redis/事件流支撑。
- Affected APIs: `/api/v1/im/send`, `/api/v1/im/notify`, 新增或补全的 `/api/v1/im/bridge/register`、心跳/注销、进度或事件回传接口。
- Affected operations: Bridge 部署方式、实例鉴权、重连策略、消息补发、长任务进度展示，以及多实例环境下的重复消费防护。
