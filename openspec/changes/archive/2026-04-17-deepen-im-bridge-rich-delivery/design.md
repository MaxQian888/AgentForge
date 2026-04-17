## Context

本 change 把 IM Bridge 从 "文本 + 富卡片" 时代带到 "带附件、带反应、带 thread" 时代。这三块原语在 openclaw 里是 baseline（AgentForge PRD 同样把它们视为 "通用 IM bot 功能的底线"），在 AgentForge 当前 bridge 里是空白或残缺。

设计原则：
- **Platform interface 扩展用 "可选 interface"**：provider 不实现 = 当前行为；避免逐个 provider 强制升级。
- **DeliveryEnvelope 字段零值兼容**：现有调用点不需要改即可继续工作。
- **降级路径明确**：不支持 attachment 的 provider 走 "附外链 + 文本摘要"，`fallback_reason` 记账；不支持 thread 的 provider 退化为 `Reply`。
- **附件临时文件在进程内清理**：生命周期是 "入境 → staging dir → handler 消费 → 删除"，最长保留时间有 TTL，不依赖外部存储。

## Goals / Non-Goals

**Goals**
1. AI 生成的 patch/report/log 能以文件形式下发到 IM；用户在 IM 里丢一个 .zip 能被 handler 识别并消费。
2. 高频轻量反馈（"收到"、"处理中"、"完成"）可用 reaction 表达，降低刷屏。
3. 长任务进度可以聚集到独立 thread，不污染主频道。
4. DingTalk / WeCom 升到 `full_native_lifecycle`；QQ Bot 升到 `native_send_with_fallback`；QQ 明确 `simulated` tier。
5. Provider capability matrix 对所有新字段 truthful，不虚报。

**Non-Goals**
- 不做病毒扫描 / DLP / 敏感内容识别（交给后端或 plugin）。
- 不做大文件断点续传（`MaxAttachmentSize` 按 provider 原生上限定）。
- 不做跨 bridge 实例的 attachment 共享存储。
- 不引入新 capability 就直接 GA；新能力默认开启但运营可关。

## Decisions

### Decision 1：Attachment 类型与上下行流程

```go
type AttachmentKind string
const (
  AttachmentKindFile    AttachmentKind = "file"
  AttachmentKindImage   AttachmentKind = "image"
  AttachmentKindLogs    AttachmentKind = "logs"   // structured log bundle
  AttachmentKindPatch   AttachmentKind = "patch"  // git patch/diff
  AttachmentKindReport  AttachmentKind = "report" // md/html/pdf
)

type Attachment struct {
  ID            string          // uuid for tracking
  Kind          AttachmentKind
  MimeType      string
  Filename      string
  SizeBytes     int64
  ContentRef    string          // local file path (ingress) or URL (egress)
  ExternalRef   string          // provider-native id after upload
  Metadata      map[string]string
}
```

**Egress（bridge 给 IM 发文件）流程**
```
  command handler
       │ attaches local file to DeliveryEnvelope.Attachments
       ▼
  core.DeliverEnvelope
       │ decides: provider supports attachments?
       ├─ yes → provider.UploadAttachment → provider.SendAttachment
       └─ no  → fallback to text with link + sanitize
       ▼
  audit event includes attachmentCount + sizeBytesTotal
```

**Ingress（IM 用户发文件给 bot）流程**
```
  provider.onMessage
       │ discovers file payload
       ▼
  provider.DownloadToStaging(ctx, tempDir) (Attachment, error)
       │ writes to ${IM_BRIDGE_STATE_DIR}/attachments/<uuid>
       ▼
  core.Message.Attachments = [...]
       │
       ▼
  engine.HandleMessage → command handler consumes ContentRef
       │
       ▼
  deferred cleanup worker (per-ContentRef TTL, default 1h)
```

**Size/kind allowlist 由 provider 声明**：capability matrix 新增
- `MaxAttachmentSize int64`
- `AllowedAttachmentKinds []AttachmentKind`
- Sanitizer（来自 change C）拒绝任何不在 allowlist / 超 size / mime 不匹配的上传。

### Decision 2：Reaction 接口与后端 repo 打通

```go
type Reaction struct {
  UserID     string
  EmojiCode  string   // openclaw-like unified code: "ack" | "done" | "failed" | "thumbs_up" | "thumbs_down" | ...
  ReactedAt  time.Time
  RawEmoji   string   // platform-native (e.g. "✅", Feishu custom id)
}

type ReactionSender interface {
  SendReaction(ctx context.Context, replyCtx any, emoji string) error
}

type ReactionReceiver interface {
  // provider 在捕获 reaction 事件时调用 engine 上的新 dispatch 入口
  OnReaction(ctx context.Context, msg *Message) error
}
```

**统一 emoji 代码表**（跨 provider 常用对齐）：
| Unified code | 用途 | Feishu | Slack | DingTalk | Telegram |
|--------------|------|--------|-------|----------|----------|
| `ack` | 已收到 | 👀 | eyes | 👀 | 👀 |
| `running` | 处理中 | ⚙️ | gear | ⏳ | ⏳ |
| `done` | 成功 | ✅ | white_check_mark | ✅ | ✅ |
| `failed` | 失败 | ❌ | x | ❌ | ❌ |
| `thumbs_up` | 赞同 | 👍 | +1 | 👍 | 👍 |
| `thumbs_down` | 反对 | 👎 | -1 | 👎 | 👎 |

`DeliveryEnvelope.Metadata["ack_reaction"]` 可携带这些 code，handler 不需要额外逻辑，delivery 层自动调 provider 的 `SendReaction`；若 provider 不支持则跳过（不降级到文本）。

**后端管道**：`notify/receiver.go` 收到 reaction 事件后，调 `POST /api/v1/im/reactions`（新增 endpoint）写入 `im_reaction_event_repo`，同时可触发已绑定 reply target 的逻辑（如 `/review` 的 👍 → approve）。

### Decision 3：Thread policy

**三档**
- `reuse`：若 message 已在 thread 内，继续 reply 到该 thread；否则 reply 到 parent channel（当前默认行为）。
- `open`：强制开一个新 thread；parent message 作为 thread root。
- `isolate`：若 provider 不支持 thread，降级为 "带 `[session: <short-id>]` 前缀的独立消息"，让用户可以用该 id 定位。

**适用场景**
| Policy | 触发 |
|--------|------|
| `reuse` | 默认；人用 command 交互 |
| `open` | 长任务启动时（`/agent spawn` 成功后的第一次 progress），把后续进度聚合到新 thread |
| `isolate` | 任何 "多发给不同用户但主题相同" 的批量广播 |

**ReplyTarget 扩展**
```go
type ReplyTarget struct {
  // ... existing fields
  ThreadPolicy    ThreadPolicy   // reuse | open | isolate
  ThreadParentID  string         // 当 policy=reuse 且已在 thread 内时填充
}
```

**Provider 支持矩阵**
| Provider | 原生 thread | `reuse` | `open` | `isolate` |
|----------|-----------|---------|--------|-----------|
| Slack | ✅ | native | native | emulate |
| Discord | ✅ | native | native | emulate |
| Feishu | 话题 group | native | native | emulate |
| Telegram | topic groups (limited) | native | degrade-to-reply | prefix |
| DingTalk | 群 topic（受限） | degrade | degrade | prefix |
| WeCom | 无 | prefix | prefix | prefix |
| QQ/QQ Bot | 无 | prefix | prefix | prefix |

"prefix" = `[session: xyz789] <message>` 模式。

### Decision 4：Readiness tier 升级路径

**DingTalk → full_native_lifecycle**
- 补完 `update_card(message_id, card)` 通过 OpenAPI（仅对通过 OpenAPI 发出的卡片有效，webhook 发出的不支持）→ 标记 `mutable_update_method=openapi_only`。
- session webhook reply 已存在；append `completion_marker` 状态管理，让 ack 之后的完成状态能回写到原消息。

**WeCom → full_native_lifecycle**
- template card update 通过 `template_card_update` API 实现。
- `response_url` reply 已存在；扩充到支持 markdown card 的更新。

**QQ Bot → native_send_with_fallback**
- OpenAPI 的 markdown message 支持 `msg_id` 关联 edit（QQ Bot 文档确认 OpenAPI 有 `/messages/{id}` PATCH 子路径），实现 mutable_update。

**QQ (OneBot) → text_first with simulated mutable**
- OneBot 本身不支持 edit；用 "delete old + send new + 保留 thread context" 模拟。`capability_matrix` 显式声明 `mutable_update=simulated`，运营侧可识别。

### Decision 5：Provider capability matrix 扩展

```go
type Capabilities struct {
  // ... existing
  SupportsRichMessages  bool
  ReadinessTier         ReadinessTier

  // NEW
  SupportsAttachments   bool
  MaxAttachmentSize     int64    // bytes; 0 iff not supported
  AllowedAttachmentKinds []AttachmentKind

  SupportsReactions     bool
  ReactionEmojiSet      []string  // unified codes; empty iff not supported

  SupportsThreads       bool
  ThreadPolicySupport   []ThreadPolicy  // subset of {reuse, open, isolate}
}
```

`MetadataForPlatform` 更新所有 provider 的 capability 声明；`/im/health` 返回值里补充。

## Alternatives considered

- **用 single "rich-delivery" 大 change 把 C+A 合在一起**：拒绝。安全加固和新 surface 的风险面不一样，应分次上线。
- **用 openclaw A2UI/Live Canvas 那种 unified UI DSL**：过度。Feishu 已有 full_native_lifecycle 的 card；复用现有 StructuredMessage + Native payload 比引入新 DSL 代价小。
- **reaction 走全新独立 endpoint（`/im/reaction`）**：拒绝。直接复用 `/im/notify` 的 envelope metadata 方向足够；独立 endpoint 徒增接口面积。
- **attachment 内容走 S3/外部 object storage**：拒绝。本期只做进程内 staging dir，未来若有跨 bridge 实例需求再升级。

## Risks

| 风险 | 触发 | 缓解 |
|------|------|------|
| 大文件传输拖住 bridge event loop | 20MB 以上文件串行上传 | 每 provider 声明 `MaxAttachmentSize`；sanitize 侧提前拒；上传放 goroutine 超时 2min |
| staging dir 磁盘打爆 | 大量入境附件未被 handler 清理 | TTL 1h + 启动清理 + 容量阈值（默认 2GB）触发 oldest-first GC |
| reaction 刷 rate-limit | 某 AI agent 每步都发 reaction | 纳入 change C 的 `write-action` policy 分支（或新增 `reaction-action` 限速维度） |
| thread 语义在不同 provider 不对称导致用户困惑 | Feishu 的 topic vs Slack 的 thread UX 差异大 | UI 文案上把 "thread" 统一翻成 "会话"，`fallback_reason` 让运营可见 |
| DingTalk `update_card` 仅对 OpenAPI 消息生效 | webhook 发的卡无法 update | capability matrix 里显式标记 `mutable_update_method=openapi_only`；delivery ladder 根据 reply target 里的 origin hint 自动选对 |

## Migration

- **Phase 1**：`Attachment` 类型 + 可选 interface；至少 2 个 provider 实现（feishu + slack）；其他 fallback。
- **Phase 2**：Reaction 接口 + 后端管道打通；feishu/slack 实现 SendReaction；`ack_reaction` metadata 自动走。
- **Phase 3**：Thread policy + 选路；slack/discord/feishu 原生支持；其他 prefix/degrade。
- **Phase 4**：DingTalk / WeCom readiness tier 升级。
- **Phase 5**：QQ Bot / QQ 升级与 simulated 标注。

每个 phase 可独立 ship 而不破坏前序。

## Open questions

1. Attachment kind 的 allowlist 是否需要 per-project？可能：市场团队想发图片，但开发团队禁止图片。第一版先 provider 级统一，后续配合 change B 的多租户再分层。
2. Reaction 事件是否纳入 `control-plane.AuditEvent` shipping？倾向于是，与 action 等价。
3. Thread `open` 策略的 trigger 是命令侧显式还是 bridge 侧自动？第一版倾向显式（`/agent spawn --thread=open`），自动策略后续迭代。
