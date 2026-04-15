## Context

2026-03 的 `pluginize-im-bridge-feishu-capabilities` 已经把飞书 native card、`card.action.trigger`、以及延时更新 token 接进了 `src-im-bridge`。当前真实代码路径主要集中在 `src-im-bridge/platform/feishu/live.go`、`renderer.go`、`native_payload.go`、以及 `src-im-bridge/commands/help.go`。这些基础能力已经可用，但还存在三类契约漂移：

- callback 生命周期和验收边界只在 provider 内部与少量测试中体现，缺少对“长连接或 webhook 何时算 callback-ready、何时必须降级”的清晰规格。
- `/help` 这类结构化命令说明依赖通用 `StructuredMessage -> Feishu card` 渲染路径，缺少“哪些 markdown/card 元素可以安全用于飞书 interactive 消息”的约束。
- 仓库已经声明飞书具备 richest native lifecycle，但 `/help` 快捷按钮、callback 回退提示、以及 interactive payload 结构还没有被文档化为同一条行为契约。

官方飞书文档当前明确要求：`card.action.trigger` 为 `schema: 2.0` 回调；服务端必须在 3 秒内响应；延时更新必须在同步响应成功后执行；延时更新 token 有 30 分钟有效期且最多可用 2 次；消息发送使用 `msg_type: "interactive"`，`content` 为 JSON 字符串。这些约束需要回落到 AgentForge 的 OpenSpec 和实现验收里。

## Goals / Non-Goals

**Goals:**

- 让飞书 callback 生命周期、interactive 消息结构、以及 `/help` 命令卡片的 discoverability 合约在 OpenSpec 中对齐。
- 把飞书 provider 的渲染路径收紧为 provider-owned、doc-safe 的 card/message 结构，而不是让共享层或帮助命令隐式拼装飞书 payload。
- 为 callback-ready、callback-missing、同步响应、延时更新、和 `/help` 渲染建立可测试的 requirement 与任务边界。

**Non-Goals:**

- 不重做整个 IM Bridge 的共享结构化渲染模型。
- 不新增飞书之外的平台能力，也不顺手修改 Slack、DingTalk、WeCom 的 card/rendering 合约。
- 不引入新的外部依赖、可视化卡片搭建层，或新的 operator command 家族。

## Decisions

### 1. 在现有 capability 上补 requirement，而不是再开新的 Feishu 子 capability

本次问题属于现有飞书生命周期、provider 渲染、和 operator help surface 的契约补完，不是一个全新的产品面。继续沿用 `feishu-rich-card-lifecycle`、`im-provider-rendering-profiles`、`im-operator-command-surface`，可以避免把 callback、渲染、帮助说明拆成互相脱节的新 spec。

备选方案：
- 新建独立 capability 专门描述 Feishu help cards。放弃，因为 `/help` 只是 operator command surface 在飞书下的一种 provider-aware 呈现，不值得变成平行 capability。

### 2. 把飞书 interactive 消息结构视为 provider-owned 渲染责任

`/help`、命令说明、以及 callback 后的 richer 更新都必须由 Feishu provider 决定最终使用哪种 interactive card 结构，而不是让共享 StructuredMessage 层默认“只要能拼出 JSON 就算正确”。这意味着 requirement 要求 provider 渲染计划在 `plain_text`、`lark_md`、interactive card、template/raw card 之间做真实选择，并记录无法满足时的显式降级。

备选方案：
- 对飞书始终退回纯文本 `/help`。放弃，因为仓库已经把飞书定义为 richest native lifecycle，这会让 `/help` 与现有 callback/card 能力脱节。

### 3. 把 callback readiness 提升为 operator affordance gate

飞书快捷按钮本质上依赖 `card.action.trigger` intake 真实可用。当前仓库同时存在长连接 intake 和 webhook intake 两条线，但 `/help` 的 gating 还没有把这两条路径统一成同一条 truth model。设计上把它提升为显式 gate：只要当前 runtime 的 callback intake 真实可用，就显示 callback-backed affordance；否则显示真实可执行的 plain command guidance。

备选方案：
- 始终展示按钮，点击失败后再提示。放弃，因为这会让帮助信息对能力就绪状态撒谎。

### 4. 验收以“文档约束 + 仓库真实路径”双锚定

本次 change 不靠抽象描述收尾。任务和测试要同时锚定飞书官方约束与仓库现有路径：

- 官方约束：3 秒响应、延时更新后置执行、30 分钟/2 次 token、interactive envelope。
- 仓库路径：`src-im-bridge/platform/feishu/live.go`、`renderer.go`、`native_payload.go`、`commands/help.go`、以及现有 unit/stub smoke 入口。

这样能避免只修文案或只补单测，而没有覆盖真实 callback + card message 契约。

## Risks / Trade-offs

- [飞书文档对 `markdown` 组件与 `text.tag = lark_md` 的能力边界不同] → 规格只要求 provider 选择“文档支持且当前发送路径可接受”的结构，不在 proposal 阶段强绑某个单一元素实现。
- [问题容易外溢到共享 StructuredMessage 抽象] → 任务明确限制在 Feishu renderer、help surface、callback 验收，不做跨平台重构。
- [callback-ready 在本地与生产环境的可达条件不同] → requirement 只要求 bridge 按实际 handler/path readiness 决定 affordance，不把公网可达性误写成总是可用。
- [已有飞书 rich-card spec 已覆盖部分延时更新语义，新增 requirement 可能重复] → 新 requirement 只补“interactive envelope / callback response shape / help discoverability”这几个未被明确规定的 seam。

## Migration Plan

1. 在 delta specs 中补齐飞书 callback 响应结构、provider-safe 渲染、与 help affordance readiness。
2. 实现阶段收紧 `src-im-bridge/platform/feishu/*` 与 `commands/help.go` 的构造逻辑，并补单元测试。
3. 使用 `go test ./... -count=1` 与必要的 Feishu stub/webhook smoke 验证 callback 和 `/help` 输出。
4. 若出现回归，优先回退本次 change 内对 Feishu renderer/help gating 的改动，不影响其他平台的 shared delivery 逻辑。

## Open Questions

- `/help` 长文本主体最终应继续使用 `div + lark_md` 还是切到专用 `markdown` 组件，由实现阶段结合当前飞书文档支持面与现有测试决定。
- webhook callback 的端到端 smoke 是否在当前本地 harness 中足够稳定，还是需要先以单元测试 + stub smoke 为主、把公网 webhook 验收保留为 scoped verification。
