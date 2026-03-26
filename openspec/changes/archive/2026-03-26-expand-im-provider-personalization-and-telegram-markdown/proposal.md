## Why

`src-im-bridge` 现在已经具备 provider contract、能力矩阵、飞书原生卡片生命周期、以及 Telegram inline keyboard / callback / message edit 这些基础能力，但“平台个性化支持”仍然停在 capability 宣告层，尚未形成稳定的 provider-owned 渲染与构建契约。结果是飞书 richer card/message 构建仍偏底层 payload 直出，Telegram 也还没有 repo-truthful 的 Markdown 渲染、转义、长度分段与回退语义。

现在推进这项变更的原因很明确：当前仓库已经具备多 provider IM Bridge 的控制面与 typed envelope，只差最后一层“按平台说人话”的渲染能力。若不先把这一层建成正式 contract，后续继续增强飞书卡片、消息模板、Telegram MarkdownV2、以及更多 provider-specific surface 时，就会重新退回 scattered branching 和一平台一套临时规则。

## What Changes

- 为 IM Bridge 增加 provider-personalization 渲染契约，让 provider 不仅声明 transport/callback/update 能力，还能声明文本格式、结构化消息构建、按钮/卡片降级和长度限制策略。
- 为飞书补齐 provider-owned message/card construction seam，明确何时使用普通文本、`lark_md` 卡片内容、JSON 卡片、模板卡片，以及这些路径的统一回退语义。
- 为 Telegram 增加正式的 Markdown-aware delivery contract，覆盖 `parse_mode` 选择、MarkdownV2 转义、文本长度限制、分段/编辑边界，以及 inline keyboard 场景下的安全降级规则。
- 让 typed outbound delivery、interactive action completion、以及 replay/compatibility HTTP 路径都走同一套 provider renderer 规则，而不是让通知层和平台适配器各自临时格式化文本。
- 更新 IM Bridge 运行说明和验证矩阵，使“provider personalization + Telegram markdown + Feishu card/message builders”成为后续继续扩展的平台基线。

## Capabilities

### New Capabilities
- `im-provider-rendering-profiles`: 定义 IM Bridge 的 provider-owned 渲染/构建契约，包括文本格式化能力、结构化内容降级规则、长度限制、按钮/卡片映射，以及 typed envelope 到最终 provider payload 的统一落地方式。

### Modified Capabilities
- `im-platform-plugin-contract`: 将 provider contract 从“启动与能力声明”扩展到“可声明渲染 profile、格式约束和 provider-specific builder surface”。
- `im-rich-delivery`: 让 typed outbound delivery 明确经过 provider-aware renderer，而不是仅在 native/structured/text 三种大类之间切换。
- `im-platform-native-interactions`: 扩展 Telegram text-first/native-edit 语义和 callback completion 路径，使其覆盖 Markdown-aware edit/reply/fallback 约束。
- `feishu-rich-card-lifecycle`: 扩展飞书 richer card lifecycle，使其覆盖 provider-owned message/card construction、`lark_md` 文本块生成、模板卡片输入约束和统一降级策略。

## Impact

- Affected code: `src-im-bridge/core`, `src-im-bridge/notify`, `src-im-bridge/platform/telegram`, `src-im-bridge/platform/feishu`, `src-im-bridge/cmd/bridge`, 以及相关 tests / runbook / README。
- Affected behavior: outbound typed delivery、interactive action completion、control-plane replay、compatibility `/im/send` / `/im/notify` 的最终渲染语义会更强地依赖 provider renderer。
- External references: Telegram Bot API 的 `sendMessage` / `editMessageText` / `CallbackQuery` / `Formatting options`，以及飞书开放平台的发送消息、消息卡片、模板卡片、`card.action.trigger`、长连接事件接收文档。
- Non-goals: 本次不要求新增新的 live provider，也不要求一次性补齐 Slack/Discord/DingTalk 的所有高级 rich surface；重点是先把 provider personalization contract、飞书构建面、Telegram Markdown 能力做成可持续扩展的真实基线。
