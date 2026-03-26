## Context

`src-im-bridge` 现在已经完成了多 provider 的基础运行面：`cmd/bridge` 能通过 provider descriptor 选择 active platform，`notify` 与 control-plane 能发送 typed outbound envelope，Feishu 已有 native card / delayed update seam，Telegram 已有 callback query / inline keyboard / edit message seam。真正缺的不是“再接一个平台”，而是“如何把同一条业务输出按不同 provider 的语言和约束说对”。

当前的薄弱点主要有两类：
- 渲染层缺位：`core.DeliverEnvelope(...)` 只在 `native / structured / text` 三大分支间切换，还没有 provider-owned rendering profile 来决定文本格式、parse mode、长度切分、按钮映射和安全降级。
- provider 个性化构建面不稳定：Feishu richer payload 仍然偏向原始 payload 直出，Telegram 则缺少正式的 MarkdownV2/HTML/纯文本策略与 escape contract，导致平台差异散落在 adapter 或 notify 层。

这次设计要解决的是“最后一公里”问题：让 provider contract 不只描述 transport 和 callback，还能描述最终消息如何被构建、渲染、分段、编辑和降级，同时保持当前 single-active-provider-per-process、typed envelope、以及现有 `/im/send` / `/im/notify` / `/im/action` 契约稳定。

## Goals / Non-Goals

**Goals:**
- 为 IM Bridge 定义 provider-owned rendering profile，使平台能力从“能不能发”扩展到“应该怎么发”。
- 让 `core.DeliverEnvelope(...)`、`notify` action completion、compatibility HTTP、control-plane replay 共享同一套 renderer 选择和 fallback 规则。
- 为 Telegram 建立正式的 Markdown-aware rendering contract，包括 parse mode、转义、长度限制、分段与 edit/reply 边界。
- 为 Feishu 建立 provider-owned message/card builders，使普通文本、`lark_md` 文本块、JSON 卡片、模板卡片和 delayed update 生命周期能够在同一条 provider 语义线上演进。
- 保持现有 provider、环境变量、控制面接口与单平台部署模型兼容。

**Non-Goals:**
- 不在本次把所有 provider 迁移成外部插件。
- 不要求一次性为 Slack / Discord / DingTalk 提供与飞书同级的高级渲染 surface。
- 不改变后端主业务接口，只在必要时为 outbound envelope 增加向后兼容的渲染意图字段或 metadata。
- 不把 provider-specific 原始 payload 暴露给共享业务层。

## Decisions

### 1. 在 provider descriptor 上新增 rendering profile，而不是在 notify/core 中堆平台分支

每个 provider descriptor 除了现有 capability metadata 与 native extension 外，再声明一个 rendering profile。它负责回答这些问题：
- 该平台支持哪些文本格式模式，例如 plain text、MarkdownV2、HTML、`lark_md`
- 文本最大长度、编辑限制、是否需要 escape、是否允许自动分段
- structured payload 应该优先映射成 cards / blocks / inline keyboard / text fallback 的哪一种
- provider 是否提供 provider-owned builders，例如 Feishu card builder、Telegram markdown formatter

上层 `notify`、`core`、`reply strategy` 只消费 profile，不再根据 provider 名称直接判断“Telegram 该不该 parse_mode”“Feishu 该不该造 card JSON”。

选择这个方案是因为 provider contract 已经是当前仓库的真实扩展边界，把渲染 profile 挂在同一层最利于保持 transport/callback/rendering 三者一致。另一种方案是把 renderer 单独做成全局 registry，但那会重新引入一套并行抽象，导致 provider 生命周期和渲染语义分离。

### 2. 交付路径统一为 `typed envelope -> rendering plan -> transport executor`

当前 `DeliverEnvelope` 更接近“分类派发器”。本次把它提升为两阶段流程：

1. `typed envelope -> rendering plan`
   - 根据 active provider 的 rendering profile、reply target、delivery metadata 和 payload 类型，生成一个 provider-aware rendering plan。
   - rendering plan 可以包含 text segments、parse mode、structured fallback、native payload、是否 prefer edit、是否必须降级等信息。

2. `rendering plan -> transport executor`
   - transport executor 只负责把已经做好的 plan 交给 provider adapter。
   - provider adapter 不再承担跨场景的策略决策，只负责执行对应 transport API，例如 Telegram `sendMessage` / `editMessageText`，Feishu `create message` / `reply` / `delayed update`。

这样做的原因是现在相同的业务输出会从 `notify action`、compatibility HTTP、WebSocket replay 等多个入口进入桥，如果每个入口各自做平台格式化，行为会继续漂移。

### 3. Telegram 采用“plain-first, markdown-when-safe”的 rendering 策略

Telegram 官方能力支持 `parse_mode`，但 MarkdownV2 的 escape 规则严格，文本还有 4096 字符限制，callback completion 与 `editMessageText` 也需要与 reply target 协同。因此本次不把 Telegram 渲染设计成“永远 MarkdownV2”，而是：

- 渲染输入增加文本意图，例如 plain/status/message-with-actions/markdown-preferred。
- renderer 先判断当前文本是否适合 MarkdownV2；若存在难以安全表达的内容、escape 成本过高、或 edit path 容易触发无效更新，则退回 plain text。
- 当文本超过平台长度限制时，renderer 负责分段计划；若 reply target 要求 prefer edit，则只允许可编辑范围内的更新，超出范围转为 follow-up reply。
- inline keyboard 仍保留在 structured surface 中，但其 fallback text 与按钮布局也由 Telegram profile 统一生成。

备选方案是统一对 Telegram 使用 HTML 或统一对所有文本使用 MarkdownV2。前者会让现有内容与后续按钮文案混杂 HTML 语义，后者则会把 escape 失败和 message edit 失败风险扩大到所有文本路径。

### 4. Feishu 采用“builder-owned surface + lifecycle-owned update policy”

Feishu 现在已经有 JSON/template card、`card.action.trigger`、delayed update 这些基础能力。本次不让上游直接拼接更多原始 payload，而是在 Feishu provider 内引入两个一等 builder：

- text/card builder：根据业务意图选择普通文本、`lark_md` block、JSON card、template card
- update policy builder：根据 reply target、callback token、message id、template metadata 决定即时响应、原位更新还是回退到 reply/send

这样可以把 Feishu 特有的 message/card construction 维持在 provider 内部，同时让 shared layer 只看到稳定的 typed intent。

备选方案是继续在 shared layer 直接产出 Feishu raw payload。问题在于模板卡变量、版本、多语言、delayed update 上下文都会继续泄露到公共逻辑，后续越改越难维护。

### 5. 规格层只引入一项新 capability，其余通过现有 capability delta 扩展

这次 change 的新增能力聚焦在 `im-provider-rendering-profiles`，因为真正新增的是“provider-owned rendering profile”这个横切能力。Feishu lifecycle、typed delivery、native interaction 这些能力已经存在，不宜再平行新建近义 spec，而是通过 delta 明确它们如何消费 rendering profile 并扩充行为。

这样做能避免再次出现多个 capability 描述同一条 delivery/rendering seam 的问题，也方便后续 archive 时把新能力同步进更清晰的主 spec 结构。

## Risks / Trade-offs

- [Risk] rendering profile 抽象过厚，反而让简单 provider 实现成本上升。 → Mitigation：profile 设计成“最小必填 + 可选 richer builders”，基础 provider 只需声明 plain text 和简单 fallback。
- [Risk] Telegram MarkdownV2 escape 规则复杂，若默认路径过于激进，可能导致合法文本被误降级或非法文本发送失败。 → Mitigation：采用 plain-first 策略，并要求 renderer 在无法证明安全时显式降级到 plain text。
- [Risk] Feishu builder surface 一旦设计得过于贴近当前 payload 形态，后续 CardKit/模板版本能力升级时仍会受限。 → Mitigation：builder 输入围绕“业务意图 + 模板标识 + 变量”建模，不把当前 JSON 结构细节暴露为共享 contract。
- [Risk] `DeliverEnvelope` 重构可能影响 replay、notify、action completion 等多条路径。 → Mitigation：保持 transport executor 层签名尽量稳定，先在 plan 阶段引入新 seam，再逐步迁移入口。
- [Risk] 当前文档把 Telegram 描述为“inline keyboard + edit”即可，新增 Markdown contract 后若验证矩阵没跟上，容易文档漂移。 → Mitigation：把 Telegram formatting 验证加入 runbook 和 focused tests，同步更新 smoke / manual verification checklist。

## Migration Plan

1. 在 provider descriptor 层增加 rendering profile 与 provider-owned builder seam，并为现有 provider 提供默认 profile。
2. 重构 `core.DeliverEnvelope(...)` 为“rendering plan + executor”两阶段，但保持外部入口不变。
3. 为 Telegram renderer 落地 Markdown-aware plan，补齐 parse mode、escape、segment、edit/fallback 规则。
4. 为 Feishu provider 落地 builder-owned text/card construction 与 delayed-update-aware update policy。
5. 更新 `notify` action completion、compatibility HTTP、runbook、README 与 focused tests，使所有入口共享新规则。

回滚策略：
- 若 renderer plan 层引发回归，可先回退到 provider default plain-text profile，同时保留 descriptor/profile 结构以避免白做抽象。
- 若 Telegram Markdown 路径不稳定，可临时把 Telegram profile 的默认文本模式切回 plain text，不影响 callback/query/edit 基础能力。
- 若 Feishu builder surface 在真实卡片场景中不稳定，可保留现有 JSON native payload 直送路径作为短期 fallback。

## Open Questions

- Telegram 首版是否只支持 MarkdownV2 与 plain text，还是同时保留 HTML 作为受控备选模式。
- provider rendering intent 是否需要成为 typed envelope 的一等字段，还是先通过 metadata 与 profile 默认值过渡。
- Feishu template card 的版本与多语言变量是否在本次 contract 中显式出现，还是先只约束模板标识与变量 map。
