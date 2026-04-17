## Why

IM Bridge 的 `Platform` 接口和 `DeliveryEnvelope` 当前只覆盖文本 + 富卡片 + 平台原生 payload 三类，离真实 AI 编程协作场景还缺三块骨头：

- **附件（文件上下行）完全空白**：`Platform` 接口无 `SendFile` / `ReceiveAttachment`，`DeliveryEnvelope` 无 `Attachments` 字段。AI 生成的 diff、report、log、截图、构建产物只能靠 paste 文本或外链；业务价值很大但实现面是空白。
- **Reactions 未成一等公民**：后端已有 `internal/model/im_reaction_event.go` 与 `internal/repository/im_reaction_event_repo.go`，但 bridge 侧 `Platform` 接口从来没有 `SendReaction` / `ReceiveReaction` 原语。"收到 / 处理中 / 成功 / 失败" 这类高频轻量反馈只能走文本 reply，刷屏严重。
- **Thread 不是一等对象**：`Message.ThreadID` 存在于数据结构里，但 delivery ladder 里没有 "reply to thread" / "open new thread" 的语义选择；长任务的进度刷新只能在原 channel 刷，不能聚合到独立 thread。
- **Provider readiness tier 不均衡**：Feishu 做到 `full_native_lifecycle`，DingTalk/WeCom 停在 `native_send_with_fallback`，QQ/QQ Bot 停在 `text_first`/`markdown_first`。PRD 8.7 里提到的 "信息结构化、可审计" 目标要求这些 tier 可以逐步补齐，而不是永久停在低端。
- **对齐 openclaw**：openclaw 的 `/extensions` 有完整的媒体与附件通道，PRD 定位 AgentForge 不做通用 IM bot，但 rich delivery 的基线能力必须与它持平，否则"IM 驱动 AI 编程"的叙事落空。

## What Changes

### 核心扩展（Platform interface + DeliveryEnvelope）

- **Attachment 一等对象**：
  - 新增 `core.Attachment` 类型：`{ID, Kind, MimeType, Filename, Size, ContentRef}`（`ContentRef` 指向本地临时文件或 URL）。
  - 新增 `Platform` 可选接口 `AttachmentSender` / `AttachmentReceiver`：上行 `UploadAttachment(ctx, chatID, attachment) (ExternalRef, error)`；下行通过 `Message.Attachments []Attachment` 暴露（provider 在收到带附件的消息时填充，触发下载到临时目录并提供 `ContentRef`）。
  - `DeliveryEnvelope` 新增字段 `Attachments []Attachment`；Delivery ladder 在 text/structured/native 三条路径前多一步：如果存在 attachments，根据 provider 能力决定 "单发 / 附带 / 降级为外链文本"。
  - 命令侧新增 `/task upload-log <task-id>` 等附件相关子命令（最小集合，后续迭代）。

- **Reaction 一等对象**：
  - 新增 `Platform` 可选接口 `ReactionSender` / `ReactionReceiver`：`SendReaction(ctx, replyCtx, emoji) error` / `Message.Reactions []Reaction`（UserID / EmojiCode / ReactedAt）。
  - `DeliveryEnvelope.Metadata["ack_reaction"]` 可携带 emoji code 作为 "收到" 的轻量反馈（避免刷屏的额外文本回复）。
  - 入站 reaction 事件通过 `core.Message` 的新 `Kind` 字段（`text|reaction|attachment|system`）区分，默认 text 保持现状。
  - 后端 `im_reaction_event_repo` 与 bridge 侧的 reaction 管道接通，可用作 `/review` 审批 "👍 = approve / 👎 = request-changes" 捷径。

- **Thread 一等原语**：
  - `ReplyPlan` 增加 `DeliveryMethodOpenThread` 一项，语义为 "如果 provider 支持，开一个新 thread 承接长任务输出，后续进度用 `DeliveryMethodThreadReply` 保持在该 thread"。
  - `core.ReplyTarget` 增加 `ThreadParentID` / `ThreadPolicy`（`reuse | open | isolate`）字段，reply 策略解析时按策略选路。
  - Slack / Discord / Feishu（群中话题）优先接入；其他 provider 未支持时 `OpenThread` 降级为 `Reply` 并在 `fallback_reason` 记账 `thread_unsupported`。

- **Readiness tier 上抬**：
  - DingTalk：补全 `ActionCard update`（通过 OpenAPI 更新已发送卡片）+ `completion reply to session webhook`，从 `native_send_with_fallback` 升 `full_native_lifecycle`。
  - WeCom：补全 template-card mutable update + `response_url` 延迟回复，从 `native_send_with_fallback` 升 `full_native_lifecycle`。
  - QQ Bot：补全 `msg_id`-based markdown update，从 `markdown_first` 升 `native_send_with_fallback`。
  - QQ：OneBot 的 `delete+resend` 模拟 mutable update 明确标记为 "simulated"，写入 capability matrix，不虚报 tier。

- **Provider 能力矩阵升级**：`PlatformMetadata.Capabilities` 新增：
  - `SupportsAttachments` + `MaxAttachmentSize` + `AllowedAttachmentKinds`
  - `SupportsReactions` + `ReactionEmojiSet` (openclaw-like unified set or platform-native subset)
  - `SupportsThreads` + `ThreadPolicySupport` (`reuse|open|isolate`)

### 契约破坏

- `core.Platform` 接口本身不变（新增全部通过可选 interface 满足）。
- `core.Message` 新增 `Kind`、`Attachments`、`Reactions` 字段——零值兼容旧代码。
- `core.DeliveryEnvelope` 新增 `Attachments`——零值兼容。
- `core.ReplyTarget` 新增 `ThreadParentID` / `ThreadPolicy`——零值回退为当前 "reply" 行为。
- `/im/send` `/im/notify` HTTP schema 同步新增可选字段；后端 forward-compatible。

## Non-Goals

- 不做 audio / video / sticker 媒体 —— 先聚焦文件、reaction、thread 三块骨头。
- 不做 bridge-side 附件内容扫描（病毒/机密） —— 只做路径/size/mime 校验；内容扫描交给后端或 plugin。
- 不对所有 8 个 provider 同步升级 readiness tier —— 本 change 明确点名升级 DingTalk/WeCom/QQ Bot/QQ 四个；其他保持现状。
- 不把 thread 扩展到 DM（1-on-1 私聊） —— 只在 group/channel 语境下生效。
- 不依赖 change B 的多租户 —— attachments 的临时文件目录仍按进程级配置。
- 不重写 `Feishu delayed_card_update` 现有流程 —— 保留所有已通过的测试与 capability。

## Capabilities

### New Capabilities
- `im-bridge-attachments`：定义 attachment 类型、上下行接口、临时文件生命周期、provider 能力矩阵约束。
- `im-bridge-reactions`：定义 reaction 事件接口与后端 reaction repo 的对接契约。
- `im-bridge-thread-lifecycle`：定义 thread 开/回/隔离三种 policy 的选路与降级语义。

### Modified Capabilities
- `im-rich-delivery`：`DeliveryEnvelope` 扩展 attachments；ladder 增加 attachment 决策层。
- `im-provider-rendering-profiles`：provider capability matrix 扩展 `SupportsAttachments/Reactions/Threads` 三组字段。
- `additional-im-platform-support`：DingTalk / WeCom / QQ Bot / QQ 的 readiness tier 升级与 simulated 更新明确标注。
- `im-platform-native-interactions`：Thread policy 与 reaction 事件纳入 native interaction matrix。

## Impact

- **运行时** (`src-im-bridge/`)
  - `core/platform.go`：新增 `AttachmentSender/Receiver`、`ReactionSender/Receiver`、thread method 常量。
  - `core/message.go`：扩展 `Message.Kind/Attachments/Reactions`。
  - `core/delivery.go`：attachments 决策层、thread policy 选路。
  - `core/rendering_profile.go` + `core/platform_metadata.go`：新增 capability 字段。
  - `notify/receiver.go`：HTTP schema 接收 attachments，负责临时文件 staging。
  - 各 provider：`platform/dingtalk/live.go` `platform/wecom/live.go` `platform/qqbot/live.go` `platform/qq/live.go` 升级 tier；`platform/slack/live.go` `platform/feishu/live.go` `platform/discord/live.go` 实现 thread policy；所有 live provider 实现可用的 `AttachmentSender`。
- **命令层** (`commands/`)
  - `/task upload-log`, `/task attach <task-id> <file>`, `/review approve-reaction` 等最小集合。
- **后端** (`src-go/`)
  - `internal/model/im.go` 扩展 attachment / reaction 字段；`internal/service/im_control_plane.go` 接纳新 envelope 字段。
  - `internal/repository/im_reaction_event_repo.go` 已存在 —— 本次打通 bridge→backend 管道。
- **文档**
  - `src-im-bridge/README.md` 增加 attachment/reaction/thread 使用段。
  - `src-im-bridge/docs/platform-runbook.md` 更新 capability matrix。

## Dependencies on other changes

- **依赖 C（`2026-04-17-harden-im-bridge-security-ops`）**：
  - Attachment 上下行需接入 sanitize（检查 filename 中的零宽字符、mime 白名单）→ C 的 `core/sanitize.go` 是前置。
  - Reaction 滥用需纳入 `destructive-action` 限速维度 → C 的多维 policy 是前置。
  - 建议 C 合并后再启动 A。
