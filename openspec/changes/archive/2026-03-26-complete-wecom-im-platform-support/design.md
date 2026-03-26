## Context

当前 `src-im-bridge` 已经把 Feishu、Slack、DingTalk、Telegram、Discord 纳入统一 provider contract、typed delivery、control-plane replay 与 reply-target 生命周期，但 `wecom` 仍只存在于枚举、descriptor 占位和测试断言里。仓库文档明确写着它是 planned-only，这会导致几个持续问题：一是“现有内置 connector 已完整”的产品说法不成立；二是 health、registration、capability matrix、smoke fixtures 与 README/runbook 对同一平台集合给出互相矛盾的真相；三是后续若继续在 Go backend 或 UI 层暴露 `wecom` 作为可选值，会把运行时错误推迟到最晚阶段。

本次变更是一个跨 `src-im-bridge`、`src-go` 模型与 OpenSpec 的 focused completion。它不引入新的 connector 家族，而是把现有 repo 已经承认存在的 `wecom` 补成 runnable built-in provider，并要求控制面、兼容 HTTP 和文档矩阵都说同一种真话。

## Goals / Non-Goals

**Goals:**
- 将 `wecom` 从 planned-only provider 提升为通过共享 provider contract 启动的 runnable built-in platform。
- 为 WeCom 定义最小但完整的 live/stub 启动、消息归一化、通知投递、reply-target、health/registration 与 explicit fallback 合同。
- 保持 typed delivery、compatibility HTTP、control-plane replay 与现有 provider 的共用路径，不新增第二套 WeCom 专用控制面。
- 更新验证矩阵与文档，使“内置 IM connector 支持列表”在 spec、代码、README、runbook、smoke 中一致。

**Non-Goals:**
- 不在本次 change 中新增 `line`、`wechat`、`qq` 等新的 provider。
- 不要求一次性补齐 WeCom 的所有高级交互表面；首个落地版本只需覆盖可启动、可收发、可回放、可降级的真实基线。
- 不重写现有 Feishu/Slack/DingTalk/Telegram/Discord adapter；这些平台只做回归验证和必要的共享 seam 调整。

## Decisions

### 1. WeCom 作为现有 provider contract 下的内置 runnable provider 落地

`wecom` 不会单独引入平行启动入口，而是直接接入 `src-im-bridge/cmd/bridge/platform_registry.go` 现有 descriptor/factory 模式。这能复用当前 `IM_PLATFORM`、`IM_TRANSPORT_MODE`、config validation、health/register metadata 与 control-plane 语义，也避免再次出现 “main.go 可启动但 provider contract 仍认为它 planned-only” 的漂移。

备选方案是继续让 `wecom` 仅存在于共享模型枚举中，等未来 plugin 化再落地；这个方案的代价是现有产品面与模型面继续暴露一个不可运行值，风险已经高于实现成本，因此不采用。

### 2. Live transport 采用 WeCom 官方应用消息/回调模型，stub 继续作为本地验证入口

基于企业微信开发者中心当前文档，WeCom 支持应用消息发送、接收消息与事件回调，以及模板卡片消息与更新。设计上将 live transport 建模为 HTTP callback + access-token 驱动的发送/更新路径，而不是强行套用 Slack Socket Mode 或 Telegram long polling 风格。stub 模式仍保留统一本地入口，以便和现有 smoke 脚本一致。

备选方案是仅先实现 stub，不做 live。这样虽然更快，但不能解决“功能完整”与 runbook/live matrix 不真实的问题，因此不采用。

### 3. WeCom rich delivery 先定义“模板卡片优先、文本显式降级”的最小完整合同

WeCom 需要纳入 `im-rich-delivery`，否则控制面和兼容 HTTP 路径遇到 `wecom` 时仍会缺少规范行为。本次不要求把所有 richer surface 全做完，而是先定义一条可验证的基线：typed envelope 可映射到 WeCom 支持的文本或模板卡片；当 preserved reply target 或 payload 不满足更新条件时，Bridge 必须显式回退到文本并记录 fallback metadata。

备选方案是把 WeCom 暂时定义成 text-only provider。这样会降低首版复杂度，但会让 spec 与平台文档里已经存在的 richer app-message/card 能力无法表达，也会让后续升级继续破坏 typed delivery 一致性，因此只把“text-only fallback”作为降级路径而不是主合同。

### 4. 现有 connector 完整性通过 focused regression 保障，而不是重新打开旧 change

这次 change 虽然以 WeCom 为核心，但会把“现有 connector matrix 不回退”纳入任务与验证，覆盖 provider selection、health/register metadata、compatibility HTTP、typed replay 与 smoke fixtures。这样能满足“完善现有连接器”的用户目标，同时保持 change 聚焦。

## Risks / Trade-offs

- [Risk] WeCom 官方消息/回调细节与当前 repo 其他平台差异更大，live 适配可能引入更多 transport-specific 逻辑 -> Mitigation: 将差异收敛在 `platform/wecom` 和 descriptor metadata，不把差异散落到 shared delivery/control-plane 层。
- [Risk] 若一开始承诺过多 WeCom richer surface，可能导致 change 范围失控 -> Mitigation: spec 只要求“模板卡片优先、文本显式降级”的最小完整合同，把更高级的交互扩展留到后续 focused change。
- [Risk] 将 `wecom` 从 planned-only 改成 runnable 后，任何配置校验遗漏都会变成真实启动缺陷 -> Mitigation: 在 platform registry、main tests、provider contract tests 和 smoke docs 中同时加入 WeCom 的 positive/negative cases。
- [Risk] 修改 supported platform 集合会波及文档、README、health matrix、前后端模型 -> Mitigation: tasks 明确包含模型、文档、运行矩阵同步，避免只落代码不改契约。

## Migration Plan

1. 先扩展 OpenSpec delta，明确 `wecom` 从 planned-only 变为 supported provider 的边界。
2. 在 `src-im-bridge` 中新增 `platform/wecom` 和相应 registry/config/test seams，使 stub/live 都能通过 shared provider contract 解析。
3. 对齐 `src-go/internal/model/im.go`、compatibility HTTP payload、health/register metadata 与 replay semantics，确保 `wecom` 在 typed delivery 中有定义。
4. 更新 runbook、README、smoke fixtures 与 focused verification 命令，把 `wecom` 纳入支持矩阵并去掉 planned-only 描述。
5. 通过 scoped tests 验证 WeCom 和现有内置 provider 没有回退后，再推进 apply。

## Open Questions

- WeCom 首版 live transport 的 inbound surface 是否只覆盖应用回调消息/事件，还是同时覆盖更宽的会话工具栏/JS-SDK 入口；本次设计默认前者优先。
- WeCom 模板卡片更新在 reply-target 中需要保留哪些最小字段，是否能完全复用现有 `IMReplyTarget.Metadata` 承载；实现时需要按真实接口再精化。
