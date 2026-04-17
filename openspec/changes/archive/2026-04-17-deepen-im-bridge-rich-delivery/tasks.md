## 1. Attachment primitives

- [x] 1.1 `core/message.go`：新增 `Attachment`、`AttachmentKind`、`Message.Kind` 与 `Message.Attachments`。
- [x] 1.2 `core/platform.go`：新增可选接口 `AttachmentSender` / `AttachmentReceiver`。
- [x] 1.3 `core/delivery.go`：attachments 决策层 —— 有 attachment 时按 provider 能力选路；不支持则 fallback 到 "文本 + 外链" + `fallback_reason=attachments_unsupported`。
- [x] 1.4 `notify/receiver.go`：`/im/send` 接收 multipart 或 base64 attachments，staging 到 `${IM_BRIDGE_STATE_DIR}/attachments/<uuid>`。
- [x] 1.5 Staging dir 生命周期：启动清理 + TTL worker（默认 1h）+ 容量阈值触发（默认 2GB）。
- [x] 1.6 Tests：上下行双端；fallback 行为；超 size 拒绝；TTL 清理。

## 2. Reaction primitives

- [x] 2.1 `core/message.go`：新增 `Reaction` 与 `Message.Reactions`。
- [x] 2.2 `core/platform.go`：新增可选接口 `ReactionSender` / `ReactionReceiver`。
- [x] 2.3 unified emoji code 表在 `core/reaction_emoji.go` 常量 + 每 provider 的映射 map。
- [x] 2.4 `core/delivery.go`：消费 `DeliveryEnvelope.Metadata["ack_reaction"]`，provider 支持即发；否则跳过（不降级到文本）。
- [x] 2.5 bridge ↔ 后端管道：`notify/receiver.go` 收到 reaction 事件 → `client.AgentForgeClient.PostReaction(ctx, ReactionEvent)` → 后端 `POST /api/v1/im/reactions`（新建）→ `im_reaction_event_repo.Create`。
- [x] 2.6 Tests：各 provider reaction 往返；后端 repo 持久化；ack_reaction 自动发送链路。

## 3. Thread lifecycle

- [x] 3.1 `core/reply_strategy.go`：新增 `ThreadPolicy` (`reuse | open | isolate`) + `DeliveryMethodOpenThread`。
- [x] 3.2 `core/message.go`：`ReplyTarget` 新增 `ThreadPolicy` / `ThreadParentID` 字段。
- [x] 3.3 `core/delivery.go`：按 policy 选路；不支持时 degrade/prefix + `fallback_reason=thread_<mode>_unsupported`。
- [x] 3.4 Slack / Discord / Feishu 实现原生 thread；其他 provider 实现 prefix emulation。
- [x] 3.5 Tests：原生 thread 正常 reply；不支持 provider 降级；isolate prefix 行为。

## 4. Capability matrix expansion

- [x] 4.1 `core/rendering_profile.go` + `core/platform_metadata.go`：`Capabilities` 新增 `SupportsAttachments/MaxAttachmentSize/AllowedAttachmentKinds/SupportsReactions/ReactionEmojiSet/SupportsThreads/ThreadPolicySupport`。
- [x] 4.2 所有 provider 的 `MetadataForPlatform` 补齐新字段（不支持的明确为零值 + `SupportsX=false`）。
- [x] 4.3 `/im/health` 响应 + 后端 provider catalog 同步。
- [x] 4.4 Tests：`core/platform_metadata_test.go` 新增每 provider 期望值锁定。

## 5. Readiness tier upgrades

- [x] 5.1 DingTalk：`update_card` via OpenAPI；capability matrix `mutable_update_method=openapi_only`；readiness tier → `full_native_lifecycle`；新增/更新测试。
- [x] 5.2 WeCom：template card update via `template_card_update`；readiness tier → `full_native_lifecycle`；测试。
- [x] 5.3 QQ Bot：markdown message PATCH via OpenAPI；readiness tier → `native_send_with_fallback`；测试。
- [x] 5.4 QQ (OneBot)：simulated mutable update（delete + send + thread context）；capability matrix `mutable_update=simulated`；测试。

## 6. Commands surface

- [x] 6.1 `/task attach <task-id>` command that takes a staged attachment by id and persists to backend task.
- [x] 6.2 `/task upload-log <task-id>` 命令快捷入口。
- [x] 6.3 `/review approve-reaction` / `/review reject-reaction` 通过 reaction 事件驱动已绑定 review 的 approve/request-changes。
- [x] 6.4 Tests: command handler + bridge→backend round-trip。

## 7. Backend alignment

- [x] 7.1 `src-go/internal/handler/im_handler.go` + `internal/model/im.go`：入站 attachment 字段、reaction 事件 endpoint。
- [x] 7.2 `internal/service/im_action_execution.go`：消费 reaction 作为 review approve 捷径的 gate 条件。
- [x] 7.3 Tests。

## 8. Docs

- [x] 8.1 `src-im-bridge/README.md`：新增 "Attachments / Reactions / Threads" 章节。
- [x] 8.2 `src-im-bridge/docs/platform-runbook.md`：capability matrix 更新（按 phase 累积）。
- [ ] 8.3 PRD 7.x 或相关章节引用 attachment/reaction/thread capability ownership（可在 phase 5 之后）。
