## Context

当前 `src-im-bridge` 已经具备一套可复用的最小内核：`core.Platform` 负责平台抽象，`core.Engine` 负责命令路由，`commands/*` 负责对接 AgentForge API，`notify.Receiver` 负责通知回推。这条链路已经足够证明 IM Bridge 的命令语义可以跨平台复用，但现状仍有三个明显断层：

- `feishu`、`slack`、`dingtalk` 仍以本地 stub 为主，`cmd/bridge/main.go` 里的平台选择本质上还是“配置校验 + stub 启动”。
- 当前平台扩展仍然依赖入口层 `switch` 和零散约定，没有把“平台能力、官方传输方式、签名/ack 语义、富消息能力”抽成稳定 seam。
- 未来目标平台已经在仓库文档里多次出现，但 Telegram 与 Discord 仍未进入正式规格，导致后续扩平台时缺少统一实现模板。

官方文档也给出了更清晰的现实约束，直接决定了这次设计不能只停留在“再多加几个 stub”：

- Slack 推荐使用 Socket Mode，通过 `apps.connections.open` 获取动态 WebSocket URL，并要求对每个 envelope 做 ack，同时处理连接刷新。
- 钉钉文档当前明确推荐 Stream 推送，而不是 HTTP 推送。
- 飞书当前推荐企业自建应用优先使用长连接接收事件/回调，且事件 v2.0 是更推荐的版本。
- Telegram Bot API 明确规定 `getUpdates` 与 `setWebhook` 两种更新接收方式互斥。
- Discord Interactions 允许通过 Gateway 或 HTTP 接收，但初始响应必须在 3 秒内完成，后续 follow-up 使用交互 token。

因此，这次设计的核心不是“再接两个 SDK”，而是把 IM Bridge 从“平台名称可选”推进到“平台行为可落地”，并给后续继续增加平台提供稳定结构。

## Goals / Non-Goals

**Goals:**

- 让 `feishu`、`slack`、`dingtalk` 三个现有平台具备真实 transport、消息接收、回复发送、错误处理和运维文档，而不是继续默认落到 stub。
- 新增 `telegram` 和 `discord` 作为正式支持的平台类型，并让它们复用现有命令与通知链路。
- 为每个平台定义显式能力描述，包括接入模型、是否需要早期 ack、是否支持结构化消息、是否支持延迟回复、是否要求公网回调。
- 在不重写 `core.Engine` 和 `commands/*` 的前提下，收敛跨平台 command normalization、defer/ack、follow-up、rich fallback 等复杂度。
- 保留本地 stub/假实现作为测试与离线开发工具，但必须改成显式模式，而不是生产路径的默认行为。

**Non-Goals:**

- 不把本次改造成完整的 `cc-connect` 插件注册体系、守护进程体系或 TOML 配置体系。
- 不要求五个平台在第一版都交付完全等价的富消息交互能力。
- 不在本次变更中引入多活动平台同进程运行；仍然保持单进程单活动平台。
- 不同时解决所有群权限、组织目录同步、媒体上传、语音转文字、线程/Topic 深度映射等高级平台特性。
- 不扩展 Go 后端的业务协议，只在必要范围内确保来源标记、通知和命令语义准确。

## Decisions

### 1. 引入轻量 `PlatformDescriptor` 注册层，而不是继续在 `main.go` 里堆 `switch`

当前 `IM_PLATFORM` 选择逻辑已经开始膨胀，继续追加 Telegram / Discord 只会让配置校验、能力分支、健康日志和测试 seam 分散到更多条件分支里。这里采用轻量注册层：

- 每个平台提供一个 `PlatformDescriptor`
- 描述其 `Name`、`ValidateConfig`、`NewLive`、`NewStub`、`Capabilities`
- `cmd/bridge` 只负责读取配置、定位 descriptor、选择 live/stub 模式并启动

这样做可以把“平台名 -> 配置约束 -> transport 实现 -> 平台能力”收敛成一个稳定接口，同时仍明显比 cc-connect 的全量 registry 更轻。

备选方案：

- 继续沿用 `switch`。优点是改动最少；缺点是每新增平台都要重复扩散改入口、日志、校验和测试。
- 直接迁移到 cc-connect 全量注册/daemon 模型。优点是长期扩展性更强；缺点是本次 scope 会失控。

### 2. 把 stub 改成显式运行模式，而不是“有凭据也先跑 stub”

现状最大的问题不是 stub 存在，而是 stub 冒充了生产路径。设计上要求把 transport mode 提升为显式配置，例如 `IM_TRANSPORT_MODE=live|stub`：

- `live` 为默认生产模式，平台 descriptor 必须返回真实 transport 适配器
- `stub` 只用于本地验证、测试、离线开发
- 当 `live` 模式缺少必需凭据或公网配置时，启动直接失败，不再静默回退

这样既保留了高价值的本地可测性，也能让“支持某平台”这件事真正代表可上线。

备选方案：

- 删除 stub。这样会让本地回归和 CI 难度明显上升。
- 保留当前隐式 fallback。这样会继续制造“配置看起来通过了，但实际上没有真实连到平台”的假象。

### 3. 采用“平台 transport 负责 ack/defer，Engine 继续保持同步命令语义”

`core.Engine` 目前的优势是足够简单：收到 `core.Message`，匹配命令，然后调用 handler。Slack、Discord、飞书等平台真正复杂的是传输层时限和回复协议，而不是命令业务本身。因此本次不重写 engine，而是在平台适配层加一层“交互生命周期管理”：

- 入站 transport 先把原始平台 payload 规范化为 `core.Message`
- 若平台有强制 ack/defer 期限，由适配层先发送“已接收/处理中”响应
- 后续 `Platform.Reply/Send` 再被映射为 edit / follow-up / response_url / callback response 等平台动作

这意味着 Discord 的 3 秒响应窗口、Slack Socket Mode 的 envelope ack、飞书/钉钉回调确认，都由平台 transport 自己兜住，而 `commands/*` 仍然只关心业务回复内容。

备选方案：

- 在 `core.Engine` 中引入异步状态机和多阶段命令返回。优点是表达力强；缺点是会波及所有命令实现。
- 让各命令自行理解每个平台的 defer/follow-up 语义。这样会把平台细节污染到业务层。

### 4. 为每个平台选择与官方文档一致的首选接入模式

为了避免抽象过度，本次明确每个平台的第一优先 transport：

- `feishu`: 企业自建应用优先长连接；为不支持长连接的回调保留 HTTP callback seam
- `slack`: Socket Mode + app-level token + reconnect/refresh 处理
- `dingtalk`: Stream 模式作为默认实现；HTTP 推送仅保留为后续兼容路径
- `telegram`: 首版优先 long polling，因为它最符合当前单进程单平台模型；Webhook 留作后续扩展
- `discord`: 首版优先 HTTP Interactions + Application Commands，同步命令注册与签名校验；只有当后续需要更多 Gateway 事件时再扩展 WebSocket Gateway

这个选择兼顾了官方推荐路径和当前代码结构，避免为了一次变更同时引入多种 transport 风格。

备选方案：

- Telegram 一上来只做 webhook。优点是响应更直接；缺点是部署门槛更高。
- Discord 直接走 Gateway。优点是与长期 bot 形态一致；缺点是心跳、会话恢复和 HTTP 回应要一起做，复杂度更高。

### 5. 增加显式平台能力矩阵，统一 rich message / interaction / fallback 选择

当前代码只有 `core.CardSender` 一个可选接口，这对五个平台已经不够。设计上保留 `Reply/Send` 作为最低公共能力，并增加 descriptor 级 capability metadata，例如：

- `SupportsRichBlocks`
- `SupportsCardReply`
- `SupportsDeferredAck`
- `RequiresPublicCallback`
- `SupportsSlashLikeCommands`
- `SupportsMessageMentions`

命令和通知不直接判断平台名，而是依据 capability 选择：

- 能延迟响应的平台先 defer，再 follow-up
- 能发结构化消息的平台用 block/card/component 方案
- 不能发结构化消息的平台统一退回纯文本

这样新增平台时只需要补 descriptor 与 renderer，而不是在命令层加更多 `if platform == ...`。

备选方案：

- 继续只靠 `CardSender`。优点是改动少；缺点是无法覆盖 Discord deferred interactions、Slack blocks、Telegram text-only 等差异。

### 6. 保留双层验证：live transport contract tests + stub/local tests

本次变更涉及多个外部平台，单纯依赖真实第三方环境会让回归代价过高，因此保留双层验证：

- descriptor / config / normalize / fallback 等逻辑通过本地单元测试覆盖
- transport contract tests 使用 fake server、fixture payload、签名样本、ack/retry 模拟来验证关键协议
- stub 模式继续服务于命令与通知端到端验证
- 文档层提供每个平台的最小 smoke-test 脚本和手工验收步骤

这样既能保持 CI 可跑，也能证明 live transport 不是“只写了 SDK client”。

备选方案：

- 只跑 live sandbox 测试。优点是真实；缺点是脆弱且成本高。
- 只跑 stub 测试。优点是便宜；缺点是无法保证真实 transport 合同。

## Risks / Trade-offs

- [第三方 SDK 和平台规则变化快] → 通过 descriptor 隔离 SDK 绑定点，并在文档中记录官方入口和升级策略。
- [不同平台的 ack/defer 语义差异很大] → 把时限语义固定在 transport 层，不让业务 handler 直接承担平台生命周期管理。
- [Discord/Feishu/部分 webhook 路径可能需要公网入口] → 首版优先选择更贴合当前部署模型的 transport，并把需要公网的能力明确写进平台文档与配置校验。
- [增加五个平台后入口和测试面会显著变宽] → 通过 descriptor registry、capability matrix 和 contract tests 把平台差异集中管理。
- [从隐式 stub 切到显式 live/stub 会暴露现有“看似支持”的空洞] → 这是有意为之，短期会增加启动失败场景，但长期能避免错误部署。

## Migration Plan

1. 为 `src-im-bridge` 增加平台 descriptor、transport mode 与 capability metadata，不改变现有命令接口。
2. 把现有 `feishu`、`slack`、`dingtalk` 目录重构为“descriptor + live adapter + stub adapter + renderer / mapper”结构。
3. 落地 Feishu、Slack、DingTalk 的真实 transport，并保留显式 stub 模式。
4. 新增 Telegram 与 Discord 平台目录、配置校验、命令映射、通知发送和最小 smoke tests。
5. 改造 `notify` 与 `client`，确保 source metadata、rich fallback 和 deferred reply 在所有支持平台上闭环。
6. 更新 `src-im-bridge/README.md` 与相关文档，加入平台凭据、接入方式、测试方式、排障步骤。

回滚策略：

- 若某个平台 live transport 不稳定，可保持 descriptor 存在但临时禁用其 `live` 模式创建入口。
- 若整体变更造成上线风险，可将部署配置切回原有单平台路径，并使用显式 `stub` 模式保留本地验证能力。

## Open Questions

- Feishu 首版是否需要同时覆盖“长连接事件 + HTTP 卡片回调”双路径，还是允许先只交付长连接和文本命令路径。
- Telegram 是否需要在首版就支持 webhook，以便和统一公网入口部署到同一台服务。
- Discord 第一版是否需要消息组件/Modal，只交付 slash commands 是否足够满足当前 AgentForge 命令面。
- 是否需要在 Go 后端或 Dashboard 暴露更正式的平台能力清单，帮助运维确认某个环境启用了哪些 provider。
