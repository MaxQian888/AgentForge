## Context

`src-im-bridge` 现在已经跨过了“只有单一飞书 stub”的阶段：Slack、Discord、Telegram、飞书、钉钉都具备 live adapter 或 control-plane 入口，Go 后端也已经有 `IMReplyTarget`、Bridge 注册和异步 delivery 语义。但当前系统仍把大部分平台差异压缩成少量布尔位和最低公共分母接口：

- `PlatformCapabilities` 只有 `SupportsRichMessages`、`SupportsDeferredReply`、`RequiresPublicCallback`、`SupportsSlashCommands`、`SupportsMentions` 五项，无法表达线程回复、消息编辑、交互按钮、卡片回调窗口、session webhook 等关键差异。
- `notify.Receiver` 仍基本只在“发卡片还是发纯文本”之间二选一，无法根据 reply target 和平台特色选择 thread reply、edit original、follow-up、callback response 或 session webhook。
- `ReplyTarget` / `IMReplyTarget` 已经预留 `InteractionToken`、`ResponseURL`、`SessionWebhook`、`PreferEdit` 等字段，但这些字段在大多数路径里仍停留在透传层，没有形成统一的更新策略。
- 现有 active OpenSpec 已经补上 provider coverage 和 runtime control-plane，意味着下一阶段真正缺少的是“平台原生体验层”，否则 PRD 中承诺的“每个平台保持原生交互语义”仍然只是 transport 层面的支持。

结合当前官方能力基线，可以确定几个必须面对的约束：

- Slack 官方强调 Socket Mode、Block Kit、`response_url`、线程上下文和 modal / dynamic menu。
- Discord 官方要求 interaction 在严格时限内先确认，再通过 follow-up 或 edit-original 完成异步响应。
- Telegram Bot API 提供 inline keyboard、callback query、`editMessageText`/`editMessageReplyMarkup` 等低噪音更新方式。
- 飞书卡片回调要求 3 秒内响应，并支持即时更新或 30 分钟窗口内的延时更新。
- 钉钉 live 路径虽然已接入 Stream 和 session webhook，但当前仓库还没有把 ActionCard / 交互回调收敛成统一 contract。

因此，这次设计不是再加一个 transport，而是把“平台差异”提升为正式的一层契约，使共享命令仍然共享，但用户可见体验使用平台原生能力。

## Goals / Non-Goals

**Goals:**

- 建立可驱动真实投递策略的 IM 平台能力矩阵，而不是继续依赖少量粗粒度布尔位。
- 让 Slack、Discord、Telegram、飞书、钉钉都能对同一条 AgentForge 流程采用该平台优先的 command / action / progress / completion 交互方式。
- 统一 reply target、structured message renderer、action callback normalizer 和 backend action contract，减少平台分支散落在命令层和通知层。
- 为未来接入 `wecom` 等已出现在 PRD 或模型层的平台预留明确扩展位和缺口清单。

**Non-Goals:**

- 不在本次变更中新增第六个 live adapter，也不把 `wecom` 直接实现为正式运行时。
- 不重写 `core.Engine` 的命令模型，不把业务命令改造成平台专属 handler。
- 不一次性覆盖每个平台全部高级能力，如 Slack Home Tab、Discord full modal workflow、Telegram payments、飞书多卡片模板管理等。
- 不改变现有 Bridge control-plane 的实例注册、cursor replay 和签名机制，只在其上补能力声明和交互策略。

## Decisions

### 1. 引入分层 capability matrix，而不是继续追加零散布尔位

现有 `PlatformCapabilities` 只能回答“能不能发富消息”，无法回答“应该优先 edit 还是 follow-up”“是否支持线程内更新”“回调有效期多久”“是否支持交互型 structured payload”。本次把能力模型扩成 5 个维度：

- `commandSurface`: slash / mention / interaction / callback_query / mixed
- `structuredSurface`: none / blocks / cards / inline_keyboard / action_card
- `asyncUpdateMode`: reply / thread_reply / edit / follow_up / session_webhook / deferred_card_update
- `actionCallbackMode`: none / webhook / socket_payload / interaction_token / callback_query
- `messageScope`: chat / thread / topic / interaction-scoped

保留布尔位作为快捷判断，但由 descriptor 输出更细的结构化 metadata，供 `notify`, control-plane registration, health payload 和 renderer 统一消费。

备选方案：

- 继续在布尔位上加字段。优点是改动少；缺点是很快退化成不可维护的半结构化能力表。
- 每个平台自行硬编码策略。优点是实现快；缺点是平台规则会继续散落在 adapter、receiver 和 command 里。

### 2. 保持共享命令层不变，在平台层新增三类策略组件

`core.Engine` 仍负责统一命令路由，平台差异通过三类组件吸收：

- `ReplyStrategy`: 决定同步回复、延迟确认、progress heartbeat、terminal summary 应该走 reply、edit、follow-up 还是 thread reply。
- `StructuredRenderer`: 把同一份 AgentForge 结构化通知映射为 Slack blocks、飞书 cards、Telegram inline keyboard、钉钉 ActionCard，或判断必须降级。
- `ActionNormalizer`: 把 Slack block actions / modal submit、Discord component/custom_id、Telegram callback query、飞书 card action、钉钉交互事件统一规约为同一份 backend action payload。

这样可以避免命令处理逻辑理解每个平台的交互细节，同时允许 adapter 按平台扩展。

备选方案：

- 把 platform-native 行为塞回 `Platform.Reply/Send`。优点是表面简单；缺点是 callback normalize、structured rendering 和 async state 会互相缠绕。
- 按平台复制命令 handler。优点是每个平台最灵活；缺点是共享命令语义会快速分叉。

### 3. reply target 继续作为唯一跨层会话凭证，但必须补齐“更新策略提示”

现有 `ReplyTarget` 已经能承载 `InteractionToken`、`ResponseURL`、`SessionWebhook` 等上下文。本次不新增第二套状态对象，而是在 reply target 上补两类信息：

- 可序列化的 provider-native context，例如 Slack `thread_ts` / `response_url`，Discord original response identity，Telegram message/thread/topic 目标，飞书 callback/update token，钉钉 session webhook / conversation target。
- 策略提示，例如 `PreferEdit`、`UseReply`、`PreferredRenderer`、`ProgressMode`。

后端继续只存储可序列化结构，不缓存 provider SDK 对象；Bridge 收到 control-plane delivery 后通过 `ReplyTargetResolver + ReplyStrategy` 恢复为平台上下文。

备选方案：

- 在数据库中单独存每个平台的 reply session 表。优点是更“规范”；缺点是 schema 复杂，且会重复表达已有 reply target。
- 只在 Bridge 内存里保留原始上下文。优点是实现快；缺点是重启后无法恢复异步更新。

### 4. structured message 采用“同一逻辑 payload，多平台 renderer”而不是单一 `core.Card`

`core.Card` 适合飞书，但不足以完整表达 Slack blocks、Telegram inline keyboard、Discord component row、钉钉 ActionCard。设计上保留 `core.Card` 作为兼容输出，同时新增 bridge-internal 的 canonical structured payload，例如：

- title / body / fields / primary actions / secondary actions
- progress state / severity / links / assignee / task identifiers
- preferred dense / verbose variants

各平台 renderer 将它映射为本平台 native payload；若某平台不支持对应元素，则基于矩阵降级为文本摘要 + 链接。

备选方案：

- 直接把 `core.Card` 扩成超级 union。优点是少一个中间层；缺点是字段会很快变成“谁也不完全适合”的大杂烩。
- 每个平台自己从原始业务对象渲染。优点是灵活；缺点是跨平台一致性差，测试成本高。

### 5. action callback 统一落到现有 `/im/action` 语义，但补齐平台上下文和异步入口

当前后端已有 `/im/action`，但 payload 仍偏简单。本次保持该入口，扩充 action contract：

- `action`, `entityId`, `platform`, `replyTarget`, `bridgeId`, `metadata`
- 支持 callback buttons、modal submit、select menu、inline keyboard presses 等来源
- 允许 action 结果返回“同步 toast / message mutation / follow-up task binding”三类后果

这样能复用当前 backend seam，同时把平台特有的 callback 事件都压平为一套可验证 contract。

备选方案：

- 每个平台单独打一条 webhook 到后端。优点是原样透传；缺点是后端逻辑分叉且难测试。
- 只在 Bridge 本地消费 callback。优点是简单；缺点是无法与任务/审查/Agent 真实状态打通。

### 6. 对 `wecom` 采用“显式缺口，不伪装已支持”策略

PRD 和 `model.IMMessageRequest` 已经出现 `wecom`，但仓库没有当前 adapter。设计上不把它塞进本次 live provider 列表，而是在 capability matrix 和 tasks 中明确：

- `wecom` 属于“模型层已声明、adapter 未实现”的 future provider
- 本次交付 extension seam、占位文档和验收前提，而不是假设已经完整支持

这样能满足“不要遗漏路线图”的要求，同时避免 proposal 漂移成额外实现范围。

## Risks / Trade-offs

- [能力矩阵过细导致实现成本上升] → 先围绕 5 个当前平台提炼共性维度，禁止为单一平台私有特性创建无法复用的全局字段。
- [结构化 renderer 与 fallback 文案可能不一致] → 引入 canonical structured payload，并在测试中做 same-event 多平台快照断言。
- [回调与异步更新窗口差异大，容易出现超时或重复发送] → 把 deadline、edit window、follow-up token 等规则固定在 ReplyStrategy 内，并结合现有 control-plane 去重。
- [旧的 `/im/notify`/`/im/send` 兼容路径可能与新策略冲突] → 保留兼容路径，但让其只走最低能力路线；平台 native 交互优先走 control-plane + reply target。
- [未来 `wecom` 等平台加入时再次扩散] → 本次先把 extension seam 和 checklist 固化，新增平台必须先填 capability matrix 再进入 adapter 实现。

## Migration Plan

1. 扩展 `PlatformCapabilities` / platform metadata / health-registration payload，补齐 matrix 字段和兼容映射。
2. 在 `src-im-bridge/core` 新增 canonical structured payload、reply strategy 和 action normalizer seam，不改命令接口。
3. 逐个平台实现 native renderer / callback normalization / update strategy，并保留最小降级路径。
4. 更新 `notify`, `client`, `control_plane` 和 Go `IMReplyTarget` / `IMActionRequest` 模型，使异步更新与 action callbacks 可以带着完整平台上下文往返。
5. 为 Slack、Discord、Telegram、飞书、钉钉补 contract tests 和 smoke fixtures；为 `wecom` 增加显式 gap 文档和接入前 checklist。

回滚策略：

- 若某个平台 native renderer 不稳定，可退回文本 fallback，但 capability matrix 必须同步声明降级状态。
- 若统一 structured payload 层引发兼容问题，可暂时保留旧 `core.Card` 直通，同时只为单个平台启用新的 renderer。

## Open Questions

- Discord 第一版是否只要求 slash command + button components，还是要把 modal submit 一并纳入首批 parity。
- Telegram 是否需要在首批就覆盖 forum topic / `message_thread_id`，还是先限于普通 chat + inline keyboard。
- 飞书的 delayed card update token 是否要直接进入 backend 持久化，还是先只留在 Bridge reply target 中。
- 钉钉是否要在首批直接支持 ActionCard 回调，还是先以 session webhook + 文本确认完成最小闭环。
