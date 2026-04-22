# IM Bridge

`src-im-bridge` 是 AgentForge 的 Go 语言即时通讯（IM）桥接服务，负责将一个或多个 IM 平台网关接入统一的 AgentForge 命令引擎与后端 API。

## 架构定位

后端拓扑是有意设计的：

- IM Bridge 调用 Go 后端提供的 `/api/v1/*` 接口
- Go 后端通过代理到 TS Bridge `/bridge/*` 来桥接 AI 与运行诊断
- TS Bridge 不直接发现或调用 IM Bridge 实例
- 异步进度与终端更新通过 Go 控制平面回传，确保原始的 `bridge_id` 与回复目标保持稳定

## 网关模式（多提供商 + 多租户）

单个 Bridge 进程可同时托管多个 IM 提供商与多个租户。通过以下环境变量配置：

- `IM_PLATFORMS` — 逗号分隔的提供商 ID（如 `feishu,dingtalk,wecom`）。未设置时回退到旧版单值 `IM_PLATFORM`。
- `IM_TRANSPORT_MODE` — 默认传输模式。可通过 `IM_TRANSPORT_MODE_<PROVIDER>` 按提供商覆盖。
- `IM_SECRET_<PROVIDER>` — 按提供商的 HMAC 共享密钥覆盖。未设置时回退到 `IM_CONTROL_SHARED_SECRET`。
- `IM_NOTIFY_PORT_<PROVIDER>` — 按提供商 HTTP 监听端口覆盖。多提供商并行时默认使用 `NOTIFY_PORT + index`。
- `IM_TENANTS_CONFIG` — 声明租户与解析器绑定的 YAML 文件路径。留空则禁用租户路由（旧版单租户模式）。
- `IM_TENANT_DEFAULT` — 入站消息无解析器匹配时使用的默认租户 ID。留空则拒绝未解析消息并返回显式回复。
- `IM_BRIDGE_PLUGIN_DIR` — 监视命令插件清单（`<plugin-id>/plugin.yaml`）的目录。留空则禁用插件加载。

### tenants.yaml 示例

```yaml
tenants:
  - id: acme
    projectId: 4a1e5c6f-0000-0000-0000-000000000001
    name: "ACME Corp"
    resolvers:
      - kind: chat
        platform: feishu
        chatIds: ["oc_abc123", "oc_def456"]
      - kind: workspace
        workspaceIds: ["T0ABC"]
    credentials:
      - providerId: agentforge
        source: env
        keyPrefix: ACME_
    plugins:
      - "@acme/jira"
  - id: beta
    projectId: 4a1e5c6f-0000-0000-0000-000000000002
    resolvers:
      - kind: chat
        chatIds: ["oc_ghi789"]
defaultTenant: acme
```

当前支持的解析器类型：`chat`（入站 `(platform, chatId)`）、`workspace`（`msg.Metadata["workspaceId"]`）、`domain`（`msg.Metadata["domain"]`）。发送 SIGHUP 可热重载配置文件。

### 插件清单示例

```yaml
id: "@acme/jira"
version: "1.0.0"
name: "Jira Commands"
commands:
  - slash: "/jira"
    subcommands:
      - name: "create"
        action_class: write
        invoke:
          kind: http
          url: http://localhost:9090/plugins/jira/create
          timeout: 10s
          headers:
            X-Tenant: "${TENANT_META_tenant_slug}"
tenants: ["acme"]
```

调用类型：`http`（已完全接入）、`mcp`（占位符 — 传输层待实现）、`builtin`（通过 `Registry.RegisterBuiltin` 注册的进程内处理器）。

## 平台选择（单提供商旧版路径）

启动时通过 Bridge 本地提供商契约解析 `IM_PLATFORM`，而非硬编码分支。内置提供商以标准化描述符暴露：

- 提供商 ID
- 支持的传输模式
- 健康/控制平面/通知路径消费的能力元数据
- 投递计划解析消费的可视化配置元数据
- 可选的提供商原生扩展声明

这样既保持了单进程单活跃提供商模型，又方便未来外置 IM 提供商或更丰富的提供商专属能力，无需重写 `main.go`。

将 `IM_PLATFORM` 设置为以下之一：

- `feishu`（飞书）
- `slack`
- `dingtalk`（钉钉）
- `telegram`
- `discord`
- `wecom`（企业微信）
- `qq`
- `qqbot`

将 `IM_TRANSPORT_MODE` 显式设置为：

- `stub`：本地验证与离线开发
- `live`：真实提供商传输、凭证校验与生产投递语义

Bridge 在启动前会校验所选平台的凭证：

- `feishu`：`FEISHU_APP_ID`、`FEISHU_APP_SECRET`（live 长连接必需）；如部署同时暴露 webhook 回调端点，可选 `FEISHU_VERIFICATION_TOKEN`、`FEISHU_EVENT_ENCRYPT_KEY`、`FEISHU_CALLBACK_PATH`
- `slack`：`SLACK_BOT_TOKEN`、`SLACK_APP_TOKEN`
- `dingtalk`：`DINGTALK_APP_KEY`、`DINGTALK_APP_SECRET`
- `wecom`：`WECOM_CORP_ID`、`WECOM_AGENT_ID`、`WECOM_AGENT_SECRET`、`WECOM_CALLBACK_TOKEN`、`WECOM_CALLBACK_PORT`；可选 `WECOM_CALLBACK_PATH`
- `qq`：`QQ_ONEBOT_WS_URL`；可选 `QQ_ACCESS_TOKEN`
- `qqbot`：`QQBOT_APP_ID`、`QQBOT_APP_SECRET`、`QQBOT_CALLBACK_PORT`；可选 `QQBOT_CALLBACK_PATH`、`QQBOT_API_BASE`、`QQBOT_TOKEN_BASE`
- `telegram`：`TELEGRAM_BOT_TOKEN`，可选 `TELEGRAM_UPDATE_MODE=longpoll`，无需 `TELEGRAM_WEBHOOK_URL`
- `discord`：`DISCORD_APP_ID`、`DISCORD_BOT_TOKEN`、`DISCORD_PUBLIC_KEY`、`DISCORD_INTERACTIONS_PORT`；可选 `DISCORD_COMMAND_GUILD_ID`

示例：

```powershell
$env:IM_PLATFORM = "slack"
$env:IM_TRANSPORT_MODE = "live"
$env:SLACK_BOT_TOKEN = "xoxb-..."
$env:SLACK_APP_TOKEN = "xapp-..."
go run .\cmd\bridge
```

## 运行时控制平面

每个运行的 Bridge 实例会持久化一个稳定的 `bridge_id`，向后端注册自身，保持心跳活跃，并打开一条持久的控制平面 WebSocket，用于定向通知与进度回放。

关键环境变量：

- `IM_BRIDGE_ID_FILE`：本地文件，用于加载或创建稳定的 `bridge_id`
- `IM_CONTROL_SHARED_SECRET`：用于签名兼容 `POST /im/send` 与 `POST /im/notify` 投递的共享密钥
- `IM_BRIDGE_HEARTBEAT_INTERVAL`：Bridge 刷新后端存活状态的间隔
- `IM_BRIDGE_RECONNECT_DELAY`：控制平面 WebSocket 重连退避

Bridge 启动流程：

1. 加载或创建稳定的 `bridge_id`
2. 向后端注册 `/im/send` 与 `/im/notify` 回调能力
3. 启动心跳循环
4. 连接 `/ws/im-bridge`，用于定向投递回放与实时进度事件

优雅关闭时 Bridge 会注销自身。若 WebSocket 断开，Bridge 会使用最后确认的光标重连，从而回放待投递消息而不重复已确认的消息。

## 当前提供商模式

每个支持的平台目前都作为内置提供商交付，同时带有本地 Stub 适配器与 Live 传输路径。

- `feishu`：stub + live 长连接，就绪等级 `full_native_lifecycle`，原生 JSON/模板卡片载荷，支持回调令牌感知的延迟卡片更新
- `slack`：stub + live Socket Mode，Block Kit 回调 + `response_url`
- `dingtalk`：stub + live Stream 模式，就绪等级 `native_send_with_fallback`，ActionCard 发送 + session-webhook 优先完成路径 + 显式可变更新回退
- `telegram`：stub + live 长轮询，inline keyboard + callback query + 消息编辑
- `discord`：stub + live HTTP interactions，延迟回复 + follow-up + 原始响应编辑
- `wecom`：stub + live 回调驱动入站流，就绪等级 `native_send_with_fallback`，`response_url` 优先回复路径、直接应用消息发送回退、显式富更新回退
- `qq`：stub + live OneBot WebSocket 摄入，就绪等级 `text_first`，斜杠或提及命令归一化，会话级回复目标复用，显式富回退
- `qqbot`：stub + live webhook 摄入，就绪等级 `markdown_first`，QQ Bot OpenAPI markdown/键盘能力投递，`msg_id` 感知回复目标复用，显式可变更新回退

Stub 适配器在 `TEST_PORT` 暴露本地测试端点：

- `POST /test/message`
- `GET /test/replies`
- `DELETE /test/replies`

## 命令与通知行为

所有支持的平台复用同一命令引擎：

- `/task`（`create`、`list`、`status`、`assign`、`decompose`、`move`；兼容 `transition`）
- `/agent`（`status`、`spawn`、`run`、`logs`、`pause`、`resume`、`kill`；兼容 `list`）
- `/tools`（`list`、`install`、`uninstall`、`restart`）
- `/queue`
- `/team`
- `/memory`
- `/review`
- `/sprint`
- `/cost`
- `/help`
- `@AgentForge ...` 兜底

常用示例：

- `/task decompose task-123 openai gpt-5`
- `/task decompose task-123`，然后使用推荐的 `/agent run ...` 或 `/agent spawn ...` 交接命令
- `/agent spawn task-123` 启动运行并立即查看可用的 Bridge 工具
- `/agent health` 或 `/agent runtimes` 查看 Bridge 运行时诊断
- `/tools list`
- `/tools install https://registry.example.com/web-search.yaml`
- `@AgentForge review the PR and create follow-up tasks for the fixes`

Bridge 通过 `X-IM-Source` 将活跃消息平台传播给后端，以便下游区分 Slack、钉钉、飞书等流量。此机制同样适用于 Telegram、Discord、企业微信、QQ 与 QQ Bot。

在 `POST /im/notify` 接收的通知必须包含与当前 Bridge 平台匹配的 `platform` 字段：

- 平台匹配 + 支持 `NativeMessageSender`：优先发送提供商原生载荷，并报告实际投递方式与任何回退原因
- 平台匹配 + 支持 `CardSender`：发送结构化卡片
- 平台匹配 + 支持 `StructuredSender`：发送平台原生结构化载荷
- 平台匹配但无原生结构化支持：回退到纯文本
- 平台不匹配：拒绝通知请求

标准的出站投递契约现在在直接兼容 HTTP 与控制平面回放投递上接受相同的类型化字段：

- `content`：纯文本兜底文本
- `structured`：共享结构化载荷
- `native`：提供商原生载荷（如飞书 JSON/模板卡片）
- `replyTarget`：保留的异步回复/更新上下文
- `metadata`：操作者可见的投递元数据，如 `fallback_reason`、`delivery_method`、`action_status`

`POST /im/send`、`POST /im/notify` 与 `/ws/im-bridge` 回放现在都通过同一投递助手解析这些字段，因此队列/回放流量不会再因为经过控制平面而丢弃结构化/原生载荷或回退元数据。

出站投递现在会在传输执行前通过提供商感知的渲染计划解析。实际意味着：

- 提供商描述符声明渲染默认值，如支持的文本模式、结构化渲染偏好、文本长度限制
- 提供商元数据与 `/im/health` 暴露 `readiness_tier` 以及 `capability_matrix.readinessTier`、`capability_matrix.preferredAsyncUpdateMode`、`capability_matrix.fallbackAsyncUpdateMode`，以便操作者可见地区分完整原生生命周期与仅原生发送、文本优先、markdown 优先提供商
- 共享投递代码先选择渲染计划，再执行提供商传输 API
- Telegram 可通过投递元数据（如 `text_format=markdown_v2`）选择 MarkdownV2，在未请求或不受支持时仍回退到纯文本
- 飞书延迟更新与动作完成路径现在通过类型化助手构建更丰富的提供商原生卡片消息，而非在共享投递代码中手工组装原始卡片 JSON
- QQ、QQ Bot、钉钉与企业微信在保留的回复目标请求可编辑或延迟更新行为但无法实际兑现时，会报告显式的回退元数据
- 绑定进度、动作完成与回放投递现在通过 `reply_target_*` 元数据保留提供商完成提示，如 `reply_target_progress_mode`、`reply_target_session_webhook`、`reply_target_response_url`、`reply_target_conversation_id`

交互回调归一化为 `/api/v1/im/action` 后，现在期望真实的后端结果而非占位确认。后端返回标准动作状态：

- `started`：动作启动了任务/Agent 工作流
- `completed`：动作同步完成，如分解或审阅完成
- `blocked`：实体存在但已过期、已完成或无法转换
- `failed`：实体缺失、无效或下游工作流无法运行

Bridge 将该状态保留在 `metadata.action_status` 中，同时仍通过原始回复目标返回用户可见的 `result` 文本。

Bridge 能力路由与回退行为遵循以下规则：

- `@AgentForge ...` 提及调用 `/api/v1/ai/classify-intent`，附带候选意图与近期会话历史
- 低置信度分类返回三项消歧菜单，而非静默猜测
- `/task decompose` 优先使用 Bridge 分解，Bridge 不可用时回退到旧版 Go 分解端点
- `/task decompose` 回复现在包含后续 `/agent run ...` 或 `/agent spawn ...` 建议，用于生成的子任务
- `/agent spawn` 可在任何运行启动前返回排队结果及 Bridge 池容量原因，成功启动包含 Bridge 工具摘要
- 有发现项的已完成审阅现在包含后续 `/task create ...` 建议

### 飞书专属

原生载荷表面现在支持：

- 原始 JSON 交互卡片
- 带 `template_id`、可选 `template_version_name` 与 `template_variable` 的模板卡片
- 提供商自有的富文本/卡片构建器，用于动作完成与延迟更新内容
- 通过保留的回调令牌上下文进行延迟卡片更新（当原始回复目标支持时）
- 当延迟更新无法使用且 Bridge 必须回退到回复/发送路径时，显式报告 `fallback_reason`
- `/help` 快捷操作仅在活跃运行时能通过长连接或暴露的 webhook 回调接收 `card.action.trigger` 时显示；否则帮助卡片回退到手动命令指引

### Telegram 专属

渲染配置现在支持：

- 默认纯文本投递
- 调用方请求 `text_format=markdown_v2` 时的可选 MarkdownV2 投递
- `sendMessage` / `editMessageText` 前的提供商端转义
- 格式化更新过大无法安全作为单条原地编辑时，分段 follow-up 发送

兼容 `POST /im/send` 与 `POST /im/notify` 投递现在受以下保护：

- `X-AgentForge-Delivery-Id`
- `X-AgentForge-Delivery-Timestamp`
- `X-AgentForge-Signature`

配置 `IM_CONTROL_SHARED_SECRET` 后，未签名或签名无效的兼容请求会被拒绝，重复的 `delivery_id` 会被抑制，避免重试导致 IM 消息重复扩散。

## Live 传输汇总

| 平台 | 就绪等级 | 首选 live 传输 | 结构化表面 | 原生回调路径 | 异步更新偏好 | 当前降级规则 |
| --- | --- | --- | --- | --- | --- | --- |
| 飞书 | `full_native_lifecycle` | 长连接 | 交互卡片 + 模板卡片 | 卡片动作回调 | 即时 toast/回复、延迟卡片更新、原生回退原因报告 | 原生卡片发送或延迟更新无法使用时回退到回复/发送 |
| Slack | n/a | Socket Mode | Block Kit | Socket Mode 交互载荷 | 线程回复、`response_url`、follow-up | 仅 blocks 无法渲染时回退到纯文本 |
| 钉钉 | `native_send_with_fallback` | Stream 模式 | ActionCard 发送 + 文本回退 | Stream 卡片回调 | session webhook，然后直接发送 | ActionCard 或更富更新请求在可变更新不可用时显式降级 |
| Telegram | n/a | 长轮询 | inline keyboard | callback query | 回复或原地编辑 | 卡片类内容映射为文本加 inline keyboard；格式化文本在未选择或不安全时回退到纯文本 |
| Discord | n/a | 传出 webhook interactions | message components | `/interactions` message component 载荷 | 延迟 ack、follow-up、原始响应编辑 | 不支持的 component 场景返回显式 ephemeral ack |
| 企业微信 | `native_send_with_fallback` | 回调驱动应用消息 | 模板卡片/markdown 兼容富发送 + 文本回退 | webhook/callback 消息载荷 | `response_url` 优先回复，然后直接应用发送 | 更富或可变更新在当前路径无法兑现时显式降级到 markdown/文本 |
| QQ | `text_first` | OneBot WebSocket | 文本优先共享渲染 | OneBot 消息事件载荷 | 同群回复、私聊回复或显式文本 follow-up；无原生可变更新 | 结构化、原生或可变更新请求在发送前降级到纯文本或链接输出 |
| QQ Bot | `markdown_first` | webhook 回调 + OpenAPI 发送 | markdown/键盘优先共享渲染 | `/qqbot/callback` webhook 载荷 | 存在时复用保留的 `msg_id` 回复，否则直接 follow-up 发送 | 可变更新或不兼容键盘请求显式降级到支持的文本 follow-up |

中国平台注册元数据现在还会发布：

- `preferred_async_update_mode`
- `fallback_async_update_mode`
- 回复目标完成提示，作为 `reply_target_*` 元数据携带在兼容发送、兼容通知、动作结果、绑定进度与回放投递上

## 原生交互矩阵

| 平台 | 命令表面 | 回复目标上下文保留 | 原生动作支持 | 原生更新支持 |
| --- | --- | --- | --- | --- |
| 飞书 | 斜杠 + 提及 | 聊天、消息、回调令牌 | `card.action.trigger` 归一化为 `/im/action`，附带延迟更新上下文 | 回复、原生卡片发送或同步回调确认后的延迟卡片更新 |
| Slack | 斜杠 + 提及 + 交互 | 频道、线程、`response_url` | block action 与 view submission 归一化为 `/im/action` | 线程回复、`response_url`、follow-up |
| 钉钉 | 提及 + 群聊机器人文本 | session webhook、会话 ID、会话类型 | 存在动作引用时卡片回调归一化 | session webhook 回复或直接发送；无可变卡片对等声明 |
| Telegram | 斜杠 + 提及 | 聊天、消息、话题 | inline keyboard callback query 归一化为 `/im/action` | `sendMessage`、`editMessageText`、过大格式化更新的分段 follow-up 发送 |
| Discord | 斜杠 + component | 频道、交互令牌、原始响应 | message component `custom_id` 归一化为 `/im/action` | 延迟 ack、follow-up webhook、原始响应 patch |
| 企业微信 | 回调文本 + 提及 | 聊天、用户、`response_url` | 回调消息归一化为共享命令表面 | `response_url` 优先回复，然后直接应用发送；更富更新显式回退 |
| QQ | 斜杠 + 提及 | 聊天、消息、发送者 | 通过 OneBot 兼容消息载荷的共享命令引擎 | 群回复、私聊回复或显式文本 follow-up；无原生可变更新 |
| QQ Bot | 斜杠 + 提及 | 群或用户 openid、`msg_id` | webhook 事件归一化为共享命令表面 | markdown/键盘发送或支持时复用回复目标；可变更新显式回退 |

## 附件、表情回应、线程

Bridge 在文本、结构化载荷与提供商原生表面之外，暴露三类一级富投递原语。每项按提供商可选，通过 `/im/health` 暴露的能力矩阵协商。

### 附件

- 出站：填充 `DeliveryEnvelope.Attachments`（或 `POST /im/send` 上的 `attachments` 数组）。投递阶梯在文本/结构化/原生之前添加附件层：附件通过 `AttachmentSender.UploadAttachment` + `SendAttachment/ReplyAttachment` 上传，按提供商的 `MaxAttachmentSize` / `AllowedAttachmentKinds` 进行大小/类型检查。
- 入站：`POST /im/attachments` 将文件（multipart 或 raw body）暂存到 `${IM_BRIDGE_ATTACHMENT_DIR}`（默认 `${IM_BRIDGE_STATE_DIR}/attachments`）并返回 `staged_id`。后端将该 id 通过 `attachments[].staged_id` 传回 `/im/send`。
- 生命周期：启动时清理残留文件；TTL 工作者（默认 1h）驱逐旧文件；容量 GC（默认 2 GB）在暂存目录超过阈值时按最旧优先移除。
- 回退：未实现 `AttachmentSender` 或声明 `SupportsAttachments=false` 的提供商收到文本摘要 "[attachments degraded to text…]" 及任何原始内容；`fallback_reason` 携带 `attachments_unsupported` 或 `attachments_sender_unavailable`。

### 表情回应

- 出站：设置 `DeliveryEnvelope.Metadata["ack_reaction"] = "<unified code>"`（如 `done`、`running`、`ack`）。主投递成功后，Bridge 调用 `ReactionSender.SendReaction`，使用 `core.ReactionEmojiMapFor(platform)` 解析的提供商原生表示。未声明表情回应支持的提供商静默跳过；表情失败不导致主投递失败。
- 入站：提供商适配器构建 `notify.ReactionEvent` 并分发到配置的 `ReactionSink`。生产环境中该 sink 通过 `POST /api/v1/im/reactions` 转发到 Go 后端，写入 `im_reaction_events` 并可能触发审阅快捷方式（见 `/review approve-reaction`）。
- 统一编码：`ack`、`running`、`done`、`failed`、`thumbs_up`、`thumbs_down`、`eyes`、`question`。调用方传递统一编码；`core.NativeEmojiForCode(platform, code)` 映射到提供商原生表示（Slack `white_check_mark`、Telegram `✅`、飞书 `DONE` 等）。

### 线程

- 策略位于 `ReplyTarget.ThreadPolicy`：`reuse`（默认；继续现有线程）、`open`（长任务创建新线程）、`isolate`（每条消息前缀 `[session: <short-id>]` — 适用于跨接收方的批量广播）。
- 有原生线程的提供商（Slack、Discord、飞书、Telegram 话题）携带 `SupportsThreads=true` + `ThreadPolicySupport` 列出其支持的模式。不支持的提供商降级到 `Reply` 并设置 `fallback_reason=thread_<policy>_unsupported`。

### 能力矩阵

调用 `GET /im/health` 获取实时 `capability_matrix`；新增字段：`supportsAttachments`、`maxAttachmentSize`、`allowedAttachmentKinds`、`supportsReactions`、`reactionEmojiSet`、`supportsThreads`、`threadPolicySupport`、`mutableUpdateMethod`。

## 上线与回滚

- 每进程上线一个活跃平台，设置 `IM_PLATFORM` 与 `IM_TRANSPORT_MODE=live`。
- 推广部署前验证 `/im/health`、后端 Bridge 注册、一条入站命令、一条回复路径、一条控制平面回放、一条通知路径。
- Discord：广泛暴露部署前验证命令同步完成且 interactions 端点可达。
- Telegram：启用长轮询前移除 webhook 配置，验证 callback query 仍能足够快地响应以避免卡住按钮转圈。
- 企业微信：暴露配置的回调端点，验证回调投递与直接应用消息发送后再推广部署。
- QQ：验证 OneBot WebSocket 干净连接，且入站命令处理与出站发送动作均成功后再推广部署。
- QQ Bot：暴露配置的回调端点，验证 webhook 投递与 OpenAPI 文本发送后再推广部署。
- 钉钉：在支持的地方验证 ActionCard 发送，并将超出当前 `native_send_with_fallback` 等级的可变更新请求视为显式回退而非虚假对等。
- 回滚：先禁用控制平面 WebSocket 或清除 `IM_CONTROL_SHARED_SECRET`（如需兼容 HTTP 回退），然后将部署切回上一个平台或将当前平台切换到 `IM_TRANSPORT_MODE=stub` 进行本地诊断。

## 安全与运维加固

Bridge 为暴露给后端控制平面与本地操作者的敏感表面运行纵深防御栈。

### 持久化状态存储（SQLite）

- 位置：`${IM_BRIDGE_STATE_DIR}/state.db`（默认 `.agentforge/state.db`）。
- 保存投递去重、nonce 历史、限流计数器与 `audit_salt` 设置行。
- WAL 模式，`busy_timeout=5s`；单写入器串行化并发更新。
- 后台每 30s 清理过期行；限流保留默认 1h。
- 设置 `IM_DISABLE_DURABLE_STATE=true` 可强制内存回退 — **生产环境不推荐**；Bridge 启动时会记录警告。

### 签名投递契约

`POST /im/send`、`POST /im/notify` 与控制平面回放投递均携带：

- `X-AgentForge-Delivery-Id`
- `X-AgentForge-Delivery-Timestamp`（RFC3339 或 Unix 秒）
- `X-AgentForge-Signature`（HMAC-SHA256，覆盖 `method | path | delivery_id | timestamp | body`）

配置 `IM_CONTROL_SHARED_SECRET` 后，接收方执行三层检查，每层都有分类错误：

| 失败原因 | 状态码 | `error` 体 | `retryable` |
|---------|--------|------------|-------------|
| 缺少头 | 401 | `missing_signed_delivery_headers` | false |
| HMAC 无效 | 401 | `invalid_signature` | false |
| 超出时间偏窗 | 408 | `timestamp_out_of_window` | false |
| 重复 delivery id | 409 | `duplicate_delivery` | false |

`IM_SIGNATURE_SKEW_SECONDS`（默认 `300`）约束签名时间戳的允许新旧范围。去重 TTL 为 `skew + 60s grace`，因此窗口外的重放由时间戳检查捕获而非去重存储。

### 结构化审计日志

- 追加式 JSONL，位于 `${IM_BRIDGE_AUDIT_DIR}/audit.jsonl`。
- 轮转：`IM_AUDIT_ROTATE_SIZE_MB`（默认 128 MB）或按天，以先触发者为准。
- 保留：`IM_AUDIT_RETAIN_DAYS`（默认 14）。当前 `audit.jsonl` 永不清理。
- `chatId` / `userId` 使用 `IM_AUDIT_HASH_SALT` 做 HMAC-SHA256 哈希。未设置时，Bridge 首次启动生成随机盐并持久化到 `state.db.settings(key='audit_salt')`，以便后续运行保持一致。
- 设置 `IM_DISABLE_AUDIT=true` 可在本地禁用审计；日志仍会在启动时记录禁用决定。

事件 schema（`v=1`）包含 `direction`、`surface`、`deliveryId`、`platform`、`bridgeId`、`chatIdHash`、`userIdHash`、`action`、`status`、`deliveryMethod`、`fallbackReason`、`latencyMs`、`signatureSource` 与 `metadata`。

### 多维限流

限流以策略列表表达；每条策略命名其分桶维度（`tenant` / `chat` / `user` / `command` / `action_class` / `bridge`）、速率与窗口。策略自上而下评估；第一条拒绝即生效。接入持久化状态存储时计数器在重启后存活。

默认策略集（可通过 `IM_RATE_POLICY` JSON 覆盖）：

| id | 维度 | 速率 / 窗口 | 用途 |
|----|------|------------|------|
| `session-default` | chat + user | 20/min | 旧版会话信封 |
| `write-action` | user + action_class=write | 10/min | 每用户写入（/task create、/agent spawn） |
| `destructive-action` | user + action_class=destructive | 3/min | /tools install/uninstall/restart |
| `per-chat` | chat | 60/min | 聚合聊天上限 |

`IM_RATE_POLICY` 示例：

```json
[{"id":"per-user","dimensions":["user"],"rate":30,"window":"1m"}]
```

`ActionClassForCommand` 将斜杠命令映射到 read/write/destructive 桶。未知命令默认 `read`。

### 出站净化

`IM_SANITIZE_EGRESS=strict|permissive|off`（默认 `strict`）控制出站文本在调用任何提供商前的重写方式：

- 严格模式下，广播提及（`@everyone`、`@here`、`@all`、Slack `<!channel>`、Telegram `@channel`）替换为 `[广播已屏蔽]`。
- permissive 与 strict 模式下，零宽字符（`U+200B/200C/200D/FEFF`）与非换行控制字节被剥离。
- 提供商声明 `SupportsSegments=true` 时 oversized 文本被分段，否则截断并附加 `…[已截断]`。
- 警告附加到回复计划的 `FallbackReason` 并记录在审计事件中。

### 命令白名单

`IM_COMMAND_ALLOWLIST` 是一个粗粒度断路器，在未允许的命令往返后端之前短路拦截。语法（逗号分隔）：

- `<platform-or-*>:<command-or-*>` — 允许
- `!<platform>:<command>` — 拒绝（优先于允许）
- 空条目或空环境变量 — 禁用，放行所有命令

示例：`feishu:/task,feishu:/help,slack:/*,!slack:/tools`

白名单故意粗粒度；不替代后端 RBAC。

### 热重载（SIGHUP，仅 Unix）

`kill -HUP <pid>` 使 Bridge 重载环境并要求活跃平台就地协调凭证。实现 `core.HotReloader` 的提供商可在不重启进程的情况下刷新令牌/重连。未实现的提供商记录 `manual_restart_required`；目前各提供商的协调实现尚未落地，因此凭证轮换请计划滚动重启。

Windows 环境无 SIGHUP；Bridge 会记录热重载未接入，操作者应使用服务重启。

## 冒烟测试

本地 stub 冒烟夹具存放于 [scripts/smoke](/d:/Project/AgentForge/src-im-bridge/scripts/smoke)。在 stub 模式下启动 Bridge 后，使用 [Invoke-StubSmoke.ps1](/d:/Project/AgentForge/src-im-bridge/scripts/smoke/Invoke-StubSmoke.ps1) 与匹配的平台夹具。

适配器变更后的推荐范围验证：

```powershell
cd src-im-bridge
go test ./platform/slack ./platform/feishu ./platform/telegram ./platform/discord ./platform/dingtalk ./platform/wecom ./platform/qq ./platform/qqbot -count=1
go test ./core -run 'Test(ResolveReplyPlan_|DeliverText_|DeliverNative_|DeliverEnvelope_|MetadataForPlatform_|StructuredMessageFallbackText|ReplyTarget_JSONRoundTrip|NativeMessage_)' -count=1
go test ./client -run 'Test(HandleIMAction_SendsCanonicalPayloadAndParsesReplyTarget|HandleIMAction_ParsesCanonicalActionOutcome|WithSource_NormalizesHeaderValue|WithPlatform_UsesTelegramMetadataSource|WithPlatform_UsesWeComMetadataSource|WithPlatform_UsesQQMetadataSource|WithPlatform_UsesQQBotMetadataSource)' -count=1
go test ./notify -run 'TestReceiver_(ActionResponseUsesReplyTargetDelivery|HealthReportsNormalizedTelegramSourceAndCapabilities|HealthReportsNormalizedWeComSourceAndCapabilities|HealthReportsNormalizedQQSourceAndCapabilities|HealthReportsNormalizedQQBotSourceAndCapabilities|FallsBackToStructuredTextWhenNativeStructuredSenderUnavailable|PrefersNativePayloadWhenPlatformSupportsIt|UsesDeferredNativeUpdateWhenFeishuReplyTargetSupportsIt|ReportsFallbackReasonWhenDeferredUpdateContextMissing|SuppressesDuplicateSignedCompatibilityDelivery|RejectsUnsignedCompatibilityDeliveryWhenSecretConfigured)' -count=1
go test ./cmd/bridge -run 'Test(SelectProvider_|SelectPlatform_|LookupPlatformDescriptor_|BridgeRuntimeControl_)' -count=1
```

详细的上线、回滚与手动验证指南参见 [platform-runbook.md](/d:/Project/AgentForge/src-im-bridge/docs/platform-runbook.md)。

## 本地验证

从包根目录运行 IM Bridge 测试套件：

```powershell
cd src-im-bridge
go test ./...
```

## 操作者控制台

Dashboard `/im` 工作空间现在依赖更丰富的操作者契约，而非仅存活状态的 Bridge 状态。后端操作者表面期望提供：

- `GET /api/v1/im/bridge/status`：整体健康、待处理积压、近期失败、平均稳定延迟与提供商诊断元数据
- `GET /api/v1/im/deliveries`：可选过滤 `deliveryId`、`status`、`platform`、`eventType`、`kind`、`since`
- `POST /api/v1/im/deliveries/:id/retry` 与 `POST /api/v1/im/deliveries/retry-batch`：操作者重试工作流
- `POST /api/v1/im/test-send`：有界等待的操作者测试消息，返回投递 id 及 `delivered`、`failed` 或 `pending`

操作者控制台中显示的提供商诊断是 Bridge 注册与心跳刷新提供的最新已知元数据快照。当提供商无法报告额外诊断时，控制台应显示诊断不可用，而非编造健康状态。

## 投递生命周期

控制平面投递历史现在是结算真实的：

1. 后端队列接受将投递记录为 `pending`
2. Bridge 应用投递并返回终端 ack 载荷
3. 后端使用终端状态、`processedAt`、`latencyMs` 及任何 `failureReason` 或 `downgradeReason` 更新历史

这意味着操作者可见的积压、成功率、重试与延迟均来源于 Bridge 的实际结算，而非仅后端将消息入队这一事实。
