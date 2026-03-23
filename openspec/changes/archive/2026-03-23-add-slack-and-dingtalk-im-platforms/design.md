## Context

当前 `src-im-bridge` 已经有 `core.Platform`、`core.Message`、命令注册和通知接收器这些基础抽象，但运行入口仍然是单平台、飞书优先的最小实现：

- `cmd/bridge/main.go` 只读取 `FEISHU_APP_ID/FEISHU_APP_SECRET`，并始终实例化 `platform/feishu` stub。
- `client/agentforge.go` 发送 Go 后端请求时固定写入 `X-IM-Source: feishu`，无法正确标记 Slack 或钉钉来源。
- `notify/receiver.go` 虽然通知载荷里已有 `platform` 字段，但当前 receiver 仅绑定一个平台实例，未对平台匹配、降级和错误语义做明确约束。
- `core.Engine` 与 `commands/*` 已经基本与平台无关，说明现阶段最合适的扩展点是平台适配层、配置层和平台元数据传播，而不是重写整个命令系统。

仓库现有研究文档已经把 Slack 与钉钉列为 IM Bridge 的高优先平台，也明确指出平台能力存在明显差异：Slack 倾向 Block Kit / Socket Mode，钉钉倾向 Stream / ActionCard，而当前 Bridge 只有 `CardSender` 这一类富消息抽象。因此，这次设计需要在不引入过早架构膨胀的前提下，为新增平台建立一条可落地、可测试、可继续演进到更多平台的路径。

## Goals / Non-Goals

**Goals:**

- 让 IM Bridge 能以单个平台实例模式启动 Slack 或钉钉，并保留当前飞书路径。
- 保证 `/task`、`/agent`、`/cost`、`/help` 和 `@AgentForge` fallback 在新增平台上继续复用同一套 `core.Engine` 与命令实现。
- 去掉平台来源的飞书硬编码，使 API 调用和通知投递能够感知真实平台。
- 定义富消息能力的最小兼容与降级规则，确保新增平台在不支持卡片时仍能可靠回复。
- 为后续继续接更多 IM 平台保留清晰 seam，但不把这次实现扩成完整插件注册系统。

**Non-Goals:**

- 不在本次变更中实现完整的自然语言意图识别或多轮上下文管理。
- 不一次性引入 `cc-connect` 的整套 registry / daemon / TOML 配置体系。
- 不要求 Slack 与钉钉在第一版就达到飞书同等级的交互卡片能力。
- 不在一个 Bridge 进程里同时托管多个平台实例；本次按单实例单平台部署。

## Decisions

### 1. 保留当前 `core.Platform` + `core.Engine` 主体，新增 Slack / 钉钉适配器而非整体重构

沿用当前 `core.Platform`、`core.Message`、`core.Engine` 和 `commands/*`，把改动集中在：

- `cmd/bridge/main.go` 的配置解析与平台选择
- `platform/slack` 与 `platform/dingtalk` 适配器
- `client` 的平台来源传播
- `notify` 的平台匹配与降级处理

这样可以最大化复用已经落地的命令链路，避免为了两个新增平台提前引入 `cc-connect` 级别的复杂度。

备选方案：

- 直接迁移到 `cc-connect` 式插件注册/多平台框架。优点是长远扩展性更强；缺点是当前仓库只有最小 Bridge，直接切换会让本次 scope 从“新增两个平台”膨胀成“重写 IM Bridge 基座”，不适合这次变更。

### 2. 明确采用“单 Bridge 进程 = 单活动平台”的运行模型，并把平台选择提升为一等配置

当前运行入口和通知接收器都天然绑定一个 `core.Platform` 实例，因此本次直接把平台类型做成显式配置，例如：

- `IM_PLATFORM=feishu|slack|dingtalk`
- 平台专属凭据各自独立，如 Slack bot/app token、钉钉 app key/secret、飞书 app id/secret

启动时根据 `IM_PLATFORM` 选择适配器并校验对应必需配置；缺参时返回明确错误，而不是静默退回飞书 stub。

备选方案：

- 继续用“是否存在某平台凭据”来隐式选择。实现简单，但会造成配置歧义。
- 在同一进程中同时启动多个平台。长期可能需要，但会要求 `Engine`、`Receiver` 和健康检查改成多实例路由，本次不做。

### 3. 平台来源必须从 `core.Message` / `Platform.Name()` 一路传到 Go API，而不是继续硬编码 `feishu`

当前 `client.AgentForgeClient` 把 `X-IM-Source` 固定写成 `feishu`，这会让新增平台的审计、通知路由和后续策略判断全部失真。设计上要求：

- 命令调用时，优先从入站 `msg.Platform` 取真实平台标识。
- 非消息上下文的主动请求，至少从当前平台实例名称映射出稳定的 source 值。
- source 值在 Slack、钉钉、飞书三个平台间保持一致、可枚举、可测试。

备选方案：

- 只在通知侧使用 `Notification.Platform`，API 请求头仍不变。这样实现最小，但后端看不到真实来源，无法形成闭环。

### 4. 富消息采用“能力矩阵 + 纯文本保底”的策略

`core.CardSender` 已经为富消息留出 seam，但 Slack、钉钉和飞书的实际消息模型不同。本次不追求统一抽象所有平台特性，而采用：

- 纯文本 `Reply/Send` 是所有平台必需能力。
- `CardSender` 继续代表“Bridge 可发送结构化卡片/块消息”的可选能力。
- `notify.Receiver` 在收到卡片通知时，若当前平台实现了 `CardSender` 就发送结构化消息；否则可靠回退到 `Notification.Content`。
- 对于平台不匹配的通知请求，显式返回错误，避免错误实例误投。

备选方案：

- 为 Slack Block Kit、钉钉 ActionCard、飞书 Card 分别建独立抽象。这样表达力最强，但在当前阶段会把简单通知链路复杂化。

### 5. 为每个平台保留本地可验证的测试入口，而不把验证完全依赖真实第三方网络

当前飞书只有 stub，这说明仓库已经接受“真实适配器之外还要有本地测试支撑”的策略。新增 Slack 和钉钉时也应保留可本地驱动的测试 seam，例如：

- 把消息解析与平台 SDK/HTTP/WebSocket 客户端分层
- 为入站事件和出站消息保留 fake/stub 覆盖
- 命令与通知路径尽量通过 `core.Platform` 接口做集成测试

备选方案：

- 直接在真实 Slack / 钉钉环境上做唯一验证。实现快，但 CI 和本地回归成本都过高。

## Risks / Trade-offs

- [平台认证与连接方式差异较大] → 先把适配器边界和配置校验固定，再分别落 transport 细节，避免命令层被平台 SDK 污染。
- [`CardSender` 抽象对 Slack / 钉钉表达力有限] → 第一版只承诺“能发结构化消息或可靠回退”，后续再按平台加更细能力接口。
- [单进程单平台意味着多平台部署要起多个 Bridge 实例] → 这是当前代码结构下最稳妥的切分，后续若确有需要再演进到多平台实例管理。
- [当前后端通知接口虽然有 `platform` 字段，但 Bridge 还没有严格消费它] → 本次必须把平台匹配校验列入实现与验证范围，避免新增平台后产生误投递。
- [真实 SDK 接入容易让本次范围失控] → 用“核心命令通路 + 通知通路 + 配置/健康检查”作为 MVP 交付边界，平台专属高级交互后续迭代。

## Migration Plan

1. 扩展 `cmd/bridge` 配置模型，引入显式的平台类型与每个平台的必需凭据校验。
2. 实现 Slack 与钉钉适配器，并保持飞书路径可继续启动。
3. 改造 `client.AgentForgeClient`，让请求头和必要上下文携带真实平台来源。
4. 改造 `notify.Receiver`，加入平台匹配、卡片能力检测和纯文本回退。
5. 增加本地/单元/集成验证覆盖，至少覆盖新增平台的命令路由、通知投递和错误场景。
6. 文档更新配置样例、运行说明和平台差异说明。

回滚策略：

- 配置层可直接切回 `IM_PLATFORM=feishu`，保持现有飞书路径继续运行。
- 若 Slack 或钉钉适配器出现问题，可在不移除主命令系统的前提下禁用对应平台类型。

## Open Questions

- Slack 与钉钉第一版是否都要交付真实线上 transport，还是允许其中一个先以受限测试模式落地；本设计默认目标是两者都具备真实接入能力，但实现阶段可以按风险拆分。
- 后端是否需要基于 `X-IM-Source` 做更细粒度审计或通知策略；本次先保证来源准确传播，不要求同时完成后端策略扩展。
