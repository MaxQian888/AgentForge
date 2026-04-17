# IM Bridge

`src-im-bridge` is the Go IM Bridge that connects one active IM platform instance to the shared AgentForge command engine and backend API.

The backend topology is intentional:

- IM Bridge talks to Go backend-owned `/api/v1/*` surfaces
- Go backend mediates Bridge-backed AI and runtime diagnostics by proxying to TS Bridge `/bridge/*`
- TS Bridge does not directly discover or invoke IM Bridge instances
- asynchronous progress and terminal updates flow back through the Go control plane so the original `bridge_id` and reply target remain stable

## Platform Selection

Startup now resolves `IM_PLATFORM` through a bridge-local provider contract rather than a hard-coded startup branch. Built-in providers still ship in-tree, but they now expose a normalized descriptor with:

- provider id
- supported transport modes
- capability metadata consumed by health/control-plane/notify paths
- rendering profile metadata consumed by delivery-plan resolution
- optional provider-native extension declarations for richer message surfaces

This keeps the current single-active-provider-per-process model intact while making future externalized IM providers or richer provider-specific capabilities easier to add without rewriting `main.go`.

Set `IM_PLATFORM` to exactly one of:

- `feishu`
- `slack`
- `dingtalk`
- `telegram`
- `discord`
- `wecom`
- `qq`
- `qqbot`

Set `IM_TRANSPORT_MODE` explicitly:

- `stub`: local verification and offline development
- `live`: real provider transport, credential validation, and production delivery semantics

The bridge validates credentials for the selected platform before startup:

- `feishu`: `FEISHU_APP_ID` and `FEISHU_APP_SECRET` for live long connection; optional `FEISHU_VERIFICATION_TOKEN`, `FEISHU_EVENT_ENCRYPT_KEY`, and `FEISHU_CALLBACK_PATH` when the deployment also exposes a webhook callback endpoint
- `slack`: required `SLACK_BOT_TOKEN` and `SLACK_APP_TOKEN`
- `dingtalk`: required `DINGTALK_APP_KEY` and `DINGTALK_APP_SECRET`
- `wecom`: required `WECOM_CORP_ID`, `WECOM_AGENT_ID`, `WECOM_AGENT_SECRET`, `WECOM_CALLBACK_TOKEN`, and `WECOM_CALLBACK_PORT`; optional `WECOM_CALLBACK_PATH`
- `qq`: required `QQ_ONEBOT_WS_URL`; optional `QQ_ACCESS_TOKEN`
- `qqbot`: required `QQBOT_APP_ID`, `QQBOT_APP_SECRET`, and `QQBOT_CALLBACK_PORT`; optional `QQBOT_CALLBACK_PATH`, `QQBOT_API_BASE`, and `QQBOT_TOKEN_BASE`
- `telegram`: required `TELEGRAM_BOT_TOKEN`, optional `TELEGRAM_UPDATE_MODE=longpoll`, and no `TELEGRAM_WEBHOOK_URL`
- `discord`: required `DISCORD_APP_ID`, `DISCORD_BOT_TOKEN`, `DISCORD_PUBLIC_KEY`, and `DISCORD_INTERACTIONS_PORT`; optional `DISCORD_COMMAND_GUILD_ID`

Example:

```powershell
$env:IM_PLATFORM = "slack"
$env:IM_TRANSPORT_MODE = "live"
$env:SLACK_BOT_TOKEN = "xoxb-..."
$env:SLACK_APP_TOKEN = "xapp-..."
go run .\cmd\bridge
```

## Runtime Control Plane

Each running bridge instance now persists a stable `bridge_id`, registers itself with the backend, keeps a heartbeat alive, and opens a persistent control-plane WebSocket for targeted notification and progress replay.

Key environment variables:

- `IM_BRIDGE_ID_FILE`: local file used to load or create a stable `bridge_id`
- `IM_CONTROL_SHARED_SECRET`: shared secret used to sign compatibility `POST /im/send` and `POST /im/notify` deliveries
- `IM_BRIDGE_HEARTBEAT_INTERVAL`: how often the bridge refreshes backend liveness
- `IM_BRIDGE_RECONNECT_DELAY`: reconnect backoff for the control-plane WebSocket

Bridge startup now performs:

1. Load or create the stable `bridge_id`
2. Register `/im/send` and `/im/notify` callback capabilities with the backend
3. Start the heartbeat loop
4. Connect to `/ws/im-bridge` for targeted delivery replay and live progress events

On graceful shutdown the bridge unregisters itself. If the WebSocket drops, the bridge reconnects with the last acknowledged cursor so pending deliveries can be replayed without duplicating already-acked messages.

## Current Provider Mode

Every supported platform currently ships as a built-in provider with both a local stub adapter and a live transport path.

- `feishu`: stub + live long connection, readiness tier `full_native_lifecycle`, native JSON/template card payload support, callback-token-aware delayed card update support
- `slack`: stub + live Socket Mode, Block Kit callbacks + `response_url`
- `dingtalk`: stub + live Stream mode, readiness tier `native_send_with_fallback`, ActionCard send + session-webhook-first completion path + explicit mutable-update fallback
- `telegram`: stub + live long polling, inline keyboard + callback query + message edit
- `discord`: stub + live HTTP interactions, deferred reply + follow-up + original-response edit
- `wecom`: stub + live callback-driven inbound flow, readiness tier `native_send_with_fallback`, `response_url`-first reply path, direct app-message send fallback, and explicit richer-update fallback
- `qq`: stub + live OneBot WebSocket intake, readiness tier `text_first`, slash-or-mention command normalization, conversation-scoped reply-target reuse, and explicit richer fallback
- `qqbot`: stub + live webhook intake, readiness tier `markdown_first`, QQ Bot OpenAPI markdown/keyboard-capable delivery, `msg_id`-aware reply-target reuse, and explicit mutable-update fallback

Stub adapters expose local test endpoints on `TEST_PORT`:

- `POST /test/message`
- `GET /test/replies`
- `DELETE /test/replies`

## Command and Notification Behavior

All supported platforms reuse the same command engine:

- `/task` (`create`, `list`, `status`, `assign`, `decompose`, `move`; 兼容 `transition`)
- `/agent` (`status`, `spawn`, `run`, `logs`, `pause`, `resume`, `kill`; 兼容 `list`)
- `/tools` (`list`, `install`, `uninstall`, `restart`)
- `/queue`
- `/team`
- `/memory`
- `/review`
- `/sprint`
- `/cost`
- `/help`
- `@AgentForge ...` fallback

Common examples:

- `/task decompose task-123 openai gpt-5`
- `/task decompose task-123` and then use the suggested `/agent run ...` or `/agent spawn ...` handoff commands
- `/agent spawn task-123` to start a run and immediately see available Bridge tools
- `/agent health` or `/agent runtimes` for Bridge runtime diagnostics
- `/tools list`
- `/tools install https://registry.example.com/web-search.yaml`
- `@AgentForge review the PR and create follow-up tasks for the fixes`

The bridge propagates the active message platform to the backend through `X-IM-Source`, so Slack, DingTalk, and Feishu traffic can be distinguished downstream.
This now also applies to Telegram, Discord, WeCom, QQ, and QQ Bot.

Notifications received on `POST /im/notify` must include a `platform` field matching the active bridge platform:

- matching platform + `NativeMessageSender` support: send provider-native payload first and report the actual delivery method plus any fallback reason
- matching platform + `CardSender` support: send structured card
- matching platform + `StructuredSender` support: send platform-native structured payload
- matching platform without native structured support: fall back to plain text
- mismatched platform: reject the notification request

The canonical outbound delivery contract now accepts the same typed fields on direct compatibility HTTP and replayed control-plane deliveries:

- `content`: plain-text fallback text
- `structured`: shared structured payload
- `native`: provider-native payload such as Feishu JSON/template cards
- `replyTarget`: preserved asynchronous reply/update context
- `metadata`: operator-visible delivery metadata such as `fallback_reason`, `delivery_method`, or `action_status`

`POST /im/send`, `POST /im/notify`, and `/ws/im-bridge` replay now resolve those fields through the same delivery helper, so queued/replayed traffic no longer drops structured/native payloads or fallback metadata just because it crossed the control plane.

Outbound delivery now resolves through a provider-aware rendering plan before transport execution. In practice this means:

- provider descriptors declare rendering defaults such as supported text modes, structured rendering preference, and text length limits
- provider metadata and `/im/health` expose `readiness_tier` plus `capability_matrix.readinessTier`, `capability_matrix.preferredAsyncUpdateMode`, and `capability_matrix.fallbackAsyncUpdateMode`, so operator-visible truth distinguishes full native lifecycle from native-send-only, text-first, or markdown-first providers
- shared delivery code chooses a rendering plan first, then executes provider transport APIs
- Telegram can opt into MarkdownV2 through delivery metadata such as `text_format=markdown_v2`, while still falling back to plain text when formatted delivery is not requested or not supported
- Feishu delayed-update and action-completion paths now build richer provider-native card messages through typed helpers instead of hand-assembling raw card JSON inside shared delivery code
- QQ, QQ Bot, DingTalk, and WeCom now report explicit fallback metadata when a preserved reply target requests editable or deferred update behavior they cannot actually honor
- bound progress, action completion, and replayed deliveries now preserve provider completion hints through `reply_target_*` metadata such as `reply_target_progress_mode`, `reply_target_session_webhook`, `reply_target_response_url`, and `reply_target_conversation_id`

Interactive callbacks normalized into `/api/v1/im/action` now expect truthful backend outcomes instead of placeholder acknowledgements. The backend returns canonical action states such as:

- `started`: the action launched a task/agent workflow
- `completed`: the action finished synchronously, such as decomposition or review completion
- `blocked`: the entity exists but is stale, already completed, or otherwise cannot transition
- `failed`: the entity is missing, invalid, or the downstream workflow could not run

The bridge preserves that status in `metadata.action_status` while still returning the user-facing `result` text through the original reply target.

Bridge capability routing and fallback behavior now follows these rules:

- `@AgentForge ...` mentions call `/api/v1/ai/classify-intent` with candidate intents and recent session history
- low-confidence classifications return a three-item disambiguation menu instead of silently guessing
- `/task decompose` prefers Bridge decomposition, then falls back to the legacy Go decomposition endpoint when Bridge is unavailable
- `/task decompose` replies now include follow-up `/agent run ...` or `/agent spawn ...` suggestions for generated subtasks
- `/agent spawn` can return a queued outcome with a Bridge pool capacity reason before any runtime starts, and successful spawns include a Bridge tools summary
- completed reviews with findings now include follow-up `/task create ...` suggestions

For Feishu specifically, the native payload surface now supports:

- raw JSON interactive cards
- template cards with `template_id`, optional `template_version_name`, and `template_variable`
- provider-owned richer text/card builders for action completion and delayed-update content
- delayed card updates through preserved callback token context when the originating reply target supports it
- explicit `fallback_reason` reporting when delayed update cannot be used and the bridge has to fall back to a reply/send path
- `/help` quick actions only when the active runtime can actually receive `card.action.trigger` through long connection or an exposed webhook callback; otherwise the help card falls back to manual command guidance

For Telegram specifically, the rendering profile now supports:

- plain-text delivery by default
- optional MarkdownV2 delivery when the caller requests `text_format=markdown_v2`
- provider-side escaping before `sendMessage` / `editMessageText`
- segmented follow-up sends when a formatted update is too large to safely stay as a single in-place edit

Compatibility `POST /im/send` and `POST /im/notify` deliveries are now protected with:

- `X-AgentForge-Delivery-Id`
- `X-AgentForge-Delivery-Timestamp`
- `X-AgentForge-Signature`

When `IM_CONTROL_SHARED_SECRET` is configured, unsigned or invalidly signed compatibility requests are rejected, and duplicate `delivery_id` values are suppressed so retries do not fan out duplicate IM messages.

## Live Transport Summary

| Platform | Readiness tier | Preferred live transport | Structured surface | Native callback path | Async update preference | Current downgrade rule |
| --- | --- | --- | --- | --- | --- | --- |
| Feishu | `full_native_lifecycle` | long connection | interactive cards + template cards | card action callback | immediate toast/reply, delayed card update, native fallback reason reporting | falls back to reply/send when native card send or delayed update cannot be used |
| Slack | n/a | Socket Mode | Block Kit | interactive payloads via Socket Mode | thread reply, `response_url`, follow-up | falls back to plain text only if blocks cannot be rendered |
| DingTalk | `native_send_with_fallback` | Stream mode | ActionCard send + text fallback | Stream card callback | session webhook, then direct send | ActionCard or richer update requests degrade explicitly when mutable update is unavailable |
| Telegram | n/a | long polling | inline keyboard | callback query | reply or in-place edit | card-like content maps to text plus inline keyboard, and formatted text falls back to plain text when MarkdownV2 is not selected or safe |
| Discord | n/a | outgoing webhook interactions | message components | `/interactions` message component payloads | deferred ack, follow-up, original-response edit | unsupported component cases return explicit ephemeral ack |
| WeCom | `native_send_with_fallback` | callback-driven app messaging | template-card/markdown-compatible richer send with text fallback | webhook/callback message payload | `response_url` reply first, direct app send fallback | richer or mutable updates degrade to markdown/text with explicit fallback when the current path cannot honor them |
| QQ | `text_first` | OneBot WebSocket | text-first shared rendering | OneBot message event payload | reply in the same chat, optionally with reply segment metadata | structured, native, or mutable-update requests degrade to plain text or link output before send |
| QQ Bot | `markdown_first` | webhook callback + OpenAPI send | markdown/keyboard-first shared rendering | `/qqbot/callback` webhook payload | reply using preserved `msg_id` when present, otherwise direct follow-up send | mutable-update or incompatible keyboard requests degrade explicitly to supported text follow-up |

China-platform registration metadata now also publishes:

- `preferred_async_update_mode`
- `fallback_async_update_mode`
- reply-target completion hints carried forward as `reply_target_*` metadata on compat send, compat notify, action results, bound progress, and replayed deliveries

## Native Interaction Matrix

| Platform | Command surface | Reply target context preserved | Native action support | Native update support |
| --- | --- | --- | --- | --- |
| Feishu | slash + mention | chat, message, callback token | `card.action.trigger` normalized into `/im/action` with delayed-update context | reply, native card send, or delayed card update after synchronous callback acknowledgement |
| Slack | slash + mention + interaction | channel, thread, `response_url` | block action and view submission normalized into `/im/action` | thread reply, `response_url`, follow-up |
| DingTalk | mention + chatbot text | session webhook, conversation id, conversation type | card callback normalized when action reference is present | session webhook reply or direct send; no mutable-card parity claim |
| Telegram | slash + mention | chat, message, topic | inline keyboard callback query normalized into `/im/action` | `sendMessage`, `editMessageText`, segmented follow-up sends for oversized formatted updates |
| Discord | slash + component | channel, interaction token, original response | message component `custom_id` normalized into `/im/action` | deferred ack, follow-up webhook, original response patch |
| WeCom | callback text + mention | chat, user, `response_url` | callback messages normalize into the shared command surface | `response_url` reply first, then direct app send; richer updates fall back explicitly |
| QQ | slash + mention | chat, message, sender | shared command engine via OneBot-compatible message payloads | group reply, private reply, or explicit text follow-up; no native mutable update |
| QQ Bot | slash + mention | group or user openid, `msg_id` | webhook events normalize into the shared command surface | markdown/keyboard send or reply-target reuse when supported; mutable update falls back explicitly |

## Rollout And Rollback

- Roll out one active platform per process by setting `IM_PLATFORM` and `IM_TRANSPORT_MODE=live`.
- Verify `/im/health`, backend bridge registration, one inbound command, one reply path, one control-plane replay, and one notification path before promoting a deployment.
- For Discord, verify command sync completed and the interactions endpoint is reachable before exposing the deployment broadly.
- For Telegram, remove webhook config before enabling long polling and verify callback queries can still be answered quickly enough to avoid stuck button spinners.
- For WeCom, expose the configured callback endpoint and verify both callback delivery and direct app-message send before promoting the deployment.
- For QQ, verify the OneBot websocket connects cleanly and that both inbound command handling and outbound send actions succeed before promoting the deployment.
- For QQ Bot, expose the configured callback endpoint and verify both webhook delivery and OpenAPI text send before promoting the deployment.
- For DingTalk, verify ActionCard send where supported, and treat mutable-update requests beyond the current `native_send_with_fallback` tier as explicit fallback rather than fake parity.
- To roll back, first disable the control-plane WebSocket or clear `IM_CONTROL_SHARED_SECRET` if compatibility HTTP fallback is needed, then switch the deployment back to the previous platform or move the current platform to `IM_TRANSPORT_MODE=stub` for local diagnosis.

## Security & Ops Hardening

The bridge runs a defense-in-depth stack for the security-sensitive surfaces exposed to the backend control plane and local operators.

### Durable state store (SQLite)

- Location: `${IM_BRIDGE_STATE_DIR}/state.db` (default `.agentforge/state.db`).
- Holds delivery dedupe, nonce history, rate-limit counters, and the `audit_salt` settings row.
- WAL mode with `busy_timeout=5s`; a single writer serializes concurrent updates.
- Background cleanup every 30s evicts expired rows; rate-limit retention defaults to 1h.
- Set `IM_DISABLE_DURABLE_STATE=true` to force in-memory fallback — **not recommended** in production; the bridge logs a warning on startup.

### Signed delivery contract

`POST /im/send`, `POST /im/notify`, and control-plane replayed deliveries all carry:

- `X-AgentForge-Delivery-Id`
- `X-AgentForge-Delivery-Timestamp` (RFC3339 or Unix seconds)
- `X-AgentForge-Signature` (HMAC-SHA256 over `method | path | delivery_id | timestamp | body`)

When `IM_CONTROL_SHARED_SECRET` is configured the receiver enforces three layered checks, each with a classified error:

| Failure | Status | `error` body | `retryable` |
|---------|--------|--------------|-------------|
| Missing headers | 401 | `missing_signed_delivery_headers` | false |
| Invalid HMAC | 401 | `invalid_signature` | false |
| Outside skew window | 408 | `timestamp_out_of_window` | false |
| Duplicate delivery id | 409 | `duplicate_delivery` | false |

`IM_SIGNATURE_SKEW_SECONDS` (default `300`) bounds how old/new a signed timestamp may be. Dedupe TTL is `skew + 60s grace`, so replays outside the window are caught by the timestamp check rather than the dedupe store.

### Structured audit log

- Append-only JSONL at `${IM_BRIDGE_AUDIT_DIR}/audit.jsonl`.
- Rotation: `IM_AUDIT_ROTATE_SIZE_MB` (default 128 MB) or daily, whichever triggers first.
- Retention: `IM_AUDIT_RETAIN_DAYS` (default 14). The current `audit.jsonl` is never pruned.
- `chatId` / `userId` are HMAC-SHA256-hashed with `IM_AUDIT_HASH_SALT`. If unset, the bridge generates a random salt on first boot and persists it to `state.db.settings(key='audit_salt')` so later runs stay consistent.
- Set `IM_DISABLE_AUDIT=true` to disable audit locally; the log still records the disable decision on startup.

Event schema (`v=1`) includes `direction`, `surface`, `deliveryId`, `platform`, `bridgeId`, `chatIdHash`, `userIdHash`, `action`, `status`, `deliveryMethod`, `fallbackReason`, `latencyMs`, `signatureSource`, and `metadata`.

### Multi-dimensional rate limiting

Rate limits are expressed as a list of policies; each policy names the dimensions it buckets on (`tenant` / `chat` / `user` / `command` / `action_class` / `bridge`), a rate, and a window. Policies are evaluated top-to-bottom; the first that rejects wins. The durable state store is authoritative when wired, so counters survive restart.

Default policy set (override with `IM_RATE_POLICY` JSON):

| id | dimensions | rate / window | purpose |
|----|-----------|----------------|---------|
| `session-default` | chat + user | 20/min | legacy session envelope |
| `write-action` | user + action_class=write | 10/min | per-user writes (/task create, /agent spawn) |
| `destructive-action` | user + action_class=destructive | 3/min | /tools install/uninstall/restart |
| `per-chat` | chat | 60/min | aggregate chat ceiling |

`IM_RATE_POLICY` example:

```json
[{"id":"per-user","dimensions":["user"],"rate":30,"window":"1m"}]
```

`ActionClassForCommand` maps slash commands to read/write/destructive buckets. Unknown commands default to `read`.

### Egress sanitization

`IM_SANITIZE_EGRESS=strict|permissive|off` (default `strict`) controls how outbound text is rewritten before any provider call:

- Broadcast mentions (`@everyone`, `@here`, `@all`, Slack `<!channel>`, Telegram `@channel`) are replaced with `[广播已屏蔽]` in strict mode.
- Zero-width characters (`U+200B/200C/200D/FEFF`) and non-newline control bytes are stripped in permissive + strict modes.
- Oversized text is segmented when the provider declares `SupportsSegments=true`, otherwise truncated with `…[已截断]`.
- Warnings are appended to the reply plan's `FallbackReason` and captured in the audit event.

### Command allowlist

`IM_COMMAND_ALLOWLIST` is a coarse-grained kill-switch that short-circuits unpermitted commands without round-tripping to the backend. Grammar (comma-separated):

- `<platform-or-*>:<command-or-*>` — allow
- `!<platform>:<command>` — deny (beats allow)
- empty entry or empty env — disabled, admits everything

Example: `feishu:/task,feishu:/help,slack:/*,!slack:/tools`

The allowlist is intentionally coarse; it does not replace backend RBAC.

### Hot reload (SIGHUP, Unix only)

`kill -HUP <pid>` causes the bridge to reload its environment and ask the active platform to reconcile credentials in place. Providers that implement `core.HotReloader` can refresh tokens / reconnect without process restart. Providers that don't implement it log `manual_restart_required`; at the time of writing per-provider reconcile implementations are not yet landed, so plan on a rolling-restart for credential rotation.

Windows installations have SIGHUP unavailable; the bridge logs that hot reload is not wired and operators should use a service restart.

## Smoke Tests

Local stub smoke fixtures are stored under [scripts/smoke](/d:/Project/AgentForge/src-im-bridge/scripts/smoke). Use [Invoke-StubSmoke.ps1](/d:/Project/AgentForge/src-im-bridge/scripts/smoke/Invoke-StubSmoke.ps1) with the matching platform fixture after starting the bridge in stub mode.

Recommended scoped validation after adapter changes:

```powershell
cd src-im-bridge
go test ./platform/slack ./platform/feishu ./platform/telegram ./platform/discord ./platform/dingtalk ./platform/wecom ./platform/qq ./platform/qqbot -count=1
go test ./core -run 'Test(ResolveReplyPlan_|DeliverText_|DeliverNative_|DeliverEnvelope_|MetadataForPlatform_|StructuredMessageFallbackText|ReplyTarget_JSONRoundTrip|NativeMessage_)' -count=1
go test ./client -run 'Test(HandleIMAction_SendsCanonicalPayloadAndParsesReplyTarget|HandleIMAction_ParsesCanonicalActionOutcome|WithSource_NormalizesHeaderValue|WithPlatform_UsesTelegramMetadataSource|WithPlatform_UsesWeComMetadataSource|WithPlatform_UsesQQMetadataSource|WithPlatform_UsesQQBotMetadataSource)' -count=1
go test ./notify -run 'TestReceiver_(ActionResponseUsesReplyTargetDelivery|HealthReportsNormalizedTelegramSourceAndCapabilities|HealthReportsNormalizedWeComSourceAndCapabilities|HealthReportsNormalizedQQSourceAndCapabilities|HealthReportsNormalizedQQBotSourceAndCapabilities|FallsBackToStructuredTextWhenNativeStructuredSenderUnavailable|PrefersNativePayloadWhenPlatformSupportsIt|UsesDeferredNativeUpdateWhenFeishuReplyTargetSupportsIt|ReportsFallbackReasonWhenDeferredUpdateContextMissing|SuppressesDuplicateSignedCompatibilityDelivery|RejectsUnsignedCompatibilityDeliveryWhenSecretConfigured)' -count=1
go test ./cmd/bridge -run 'Test(SelectProvider_|SelectPlatform_|LookupPlatformDescriptor_|BridgeRuntimeControl_)' -count=1
```

Detailed rollout, rollback, and manual verification guidance is documented in [platform-runbook.md](/d:/Project/AgentForge/src-im-bridge/docs/platform-runbook.md).

## Local Verification

Run the IM bridge test suite from the package root:

```powershell
cd src-im-bridge
go test ./...
```

## Operator Console

The dashboard `/im` workspace now relies on a richer operator contract instead of a liveness-only bridge status. The backend operator surface is expected to provide:

- `GET /api/v1/im/bridge/status` for overall health, pending backlog, recent failures, average settled latency, and provider diagnostics metadata
- `GET /api/v1/im/deliveries` with optional filters such as `deliveryId`, `status`, `platform`, `eventType`, `kind`, and `since`
- `POST /api/v1/im/deliveries/:id/retry` and `POST /api/v1/im/deliveries/retry-batch` for operator retry workflows
- `POST /api/v1/im/test-send` for bounded-wait operator test messages that return the delivery id plus `delivered`, `failed`, or `pending`

Provider diagnostics shown in the operator console are last-known metadata snapshots supplied by bridge registration and heartbeat refresh. When a provider cannot report extra diagnostics, the console should show diagnostics as unavailable instead of fabricating a healthy state.

## Delivery Lifecycle

Control-plane delivery history is now settlement-truthful:

1. backend queue acceptance records the delivery as `pending`
2. the bridge applies the delivery and returns a terminal ack payload
3. the backend updates history with terminal status, `processedAt`, `latencyMs`, and any `failureReason` or `downgradeReason`

This means operator-visible backlog, success rates, retries, and latency are derived from actual bridge settlement, not just from the fact that the backend enqueued a message.
