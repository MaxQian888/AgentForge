## Why

`src-im-bridge` 已经具备飞书 `card.action.trigger`、延时更新 token、以及 `/help` 富消息回复的基础能力，但当前链路还没有形成“规格、渲染、回调、验收”一致的完整闭环。`/help` 之类的命令说明仍可能走到与飞书当前卡片文档不完全对齐的 markdown/card 结构，callback 就绪条件与降级提示也只覆盖了部分场景。

飞书仍是 AgentForge 当前最强的原生卡片 IM 面，因此这些缺口会直接影响帮助指令可读性、快捷按钮可用性、以及 callback 更新链路是否真实可验。需要补一条 focused change，把飞书 callback 支持、命令卡片结构、以及 markdown 渲染契约收紧到当前官方文档和仓库真实实现之上。

## What Changes

- 完善飞书 callback 生命周期规格，覆盖长连接或 webhook callback intake 就绪、`schema: 2.0` 回调归一化、同步响应、延时更新、token 失效/耗尽降级、以及 callback 不可用时的真实行为。
- 收紧飞书 provider-owned 渲染契约，确保 `/help` 与类似命令说明消息通过飞书文档支持的 interactive 消息结构发送，而不是依赖模糊的 markdown/card 组装。
- 明确 `/help` 等命令在飞书下的两条路径：callback-ready 时展示可点击快捷动作；callback 未配置或不可用时展示可读、可执行的文本回退指引。
- 为飞书卡片与帮助指令补齐回归覆盖，验证 `msg_type=interactive`、`content` JSON 字符串结构、callback 响应体形状、延时更新时序约束、以及命令说明卡片的渲染结果。

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `feishu-rich-card-lifecycle`: 扩展飞书 callback、同步响应、延时更新与 interactive 消息结构的真实合约，覆盖 callback-ready 与 fallback 场景。
- `im-provider-rendering-profiles`: 收紧飞书 provider 渲染计划，要求命令帮助和结构化消息使用 provider-safe 的 markdown/card 结构与显式降级。
- `im-operator-command-surface`: 更新 `/help` 在飞书下的 discoverability 合约，要求 callback-ready 快捷按钮与未就绪时的文本指导都与真实平台能力一致。

## Impact

- 受影响代码主要位于 `src-im-bridge/platform/feishu/*`、`src-im-bridge/commands/help.*`、`src-im-bridge/core/*`、以及相关 `notify`/callback 测试。
- 会影响飞书 `/help` 与卡片交互的消息结构、callback 处理与降级结果，但不引入新的外部平台或新的公开 API。
- 需要补充或调整 `src-im-bridge` 下的单元测试、stub smoke/回调验收用例，以及必要的运行文档说明。
