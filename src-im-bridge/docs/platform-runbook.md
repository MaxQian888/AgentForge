# IM Bridge Platform Runbook

This runbook covers the live transport expectations, rollout steps, rollback steps, and manual verification matrix for the IM Bridge platforms currently supported in `src-im-bridge`.

The bridge now resolves providers through a local provider contract before startup. Built-in providers still live in-tree, but the runtime, `/im/health`, and control-plane surfaces now consume provider descriptor metadata instead of relying on startup-only hard-coded branches.

That descriptor now also carries provider rendering-profile metadata. Delivery paths resolve a rendering plan before transport execution, so formatting decisions, mutable-update rules, and provider-specific richer builders stay aligned across `/im/send`, `/im/notify`, `/im/action`, and replay.

For Chinese platforms, the runtime truth now also exposes a readiness tier in both `/im/health` and bridge-registration metadata:

- `full_native_lifecycle`: the provider can preserve callback context through synchronous acknowledgement and later native update flows
- `native_send_with_fallback`: the provider can send richer/native payloads, but mutable-update parity is not claimed and explicit fallback remains part of the contract
- `text_first`: the provider resolves richer requests into text or link-safe output first
- `markdown_first`: the provider resolves richer requests into markdown/keyboard-safe output first, with explicit fallback when mutable update is requested

Registration and health metadata for those providers also expose completion-mode truth through:

- `capability_matrix.preferredAsyncUpdateMode`
- `capability_matrix.fallbackAsyncUpdateMode`
- bridge-registration metadata keys such as `preferred_async_update_mode` and `fallback_async_update_mode`

## Preferred Live Transport

| Platform | Preferred transport | Required live credentials | Notes |
| --- | --- | --- | --- |
| Feishu | long connection | `FEISHU_APP_ID`, `FEISHU_APP_SECRET` | Optional webhook callback config uses `FEISHU_VERIFICATION_TOKEN`, `FEISHU_EVENT_ENCRYPT_KEY`, and `FEISHU_CALLBACK_PATH`; long connection remains the default callback intake. |
| Slack | Socket Mode | `SLACK_BOT_TOKEN`, `SLACK_APP_TOKEN` | Requires app-level token and Socket Mode enablement. |
| DingTalk | Stream mode | `DINGTALK_APP_KEY`, `DINGTALK_APP_SECRET` | Stream mode is the default live intake; structured notifications currently downgrade to text explicitly. |
| WeCom | callback-driven app messaging | `WECOM_CORP_ID`, `WECOM_AGENT_ID`, `WECOM_AGENT_SECRET`, `WECOM_CALLBACK_TOKEN`, `WECOM_CALLBACK_PORT` | `WECOM_CALLBACK_PATH` defaults to `/wecom/callback`; inbound callbacks can reply through `response_url` and later direct sends use app-message APIs. |
| QQ | OneBot WebSocket | `QQ_ONEBOT_WS_URL` | Optional `QQ_ACCESS_TOKEN` secures the websocket handshake when the OneBot server requires it. |
| QQ Bot | webhook callback + OpenAPI send | `QQBOT_APP_ID`, `QQBOT_APP_SECRET`, `QQBOT_CALLBACK_PORT` | `QQBOT_CALLBACK_PATH` defaults to `/qqbot/callback`; optional `QQBOT_API_BASE` and `QQBOT_TOKEN_BASE` override the official QQ Bot endpoints for tests or alternate deployments. |
| Telegram | long polling | `TELEGRAM_BOT_TOKEN` | Current live implementation supports `TELEGRAM_UPDATE_MODE=longpoll` only and rejects webhook config. |
| Discord | outgoing webhook interactions | `DISCORD_APP_ID`, `DISCORD_BOT_TOKEN`, `DISCORD_PUBLIC_KEY`, `DISCORD_INTERACTIONS_PORT` | Optional `DISCORD_COMMAND_GUILD_ID` scopes command sync to a guild for faster rollout. |

All platforms support `IM_TRANSPORT_MODE=stub` for local verification and `IM_TRANSPORT_MODE=live` for production traffic.

## Gateway deployment (multi-provider + multi-tenant)

A single bridge process can host multiple providers concurrently. Each provider gets its own engine, notify receiver, and control-plane connection; state store, audit writer, rate limiter, and client factory are shared process-wide.

### Rollout from single-provider to gateway

1. Retain `IM_PLATFORM=<current>` and deploy the gateway binary. Single-provider semantics are preserved end-to-end (`IM_PLATFORMS` falls back to `IM_PLATFORM`).
2. Create `tenants.yaml` describing your current chat → project mapping and set `IM_TENANTS_CONFIG=/etc/agentforge/tenants.yaml`. Keep `IM_TENANT_DEFAULT` pointing at your existing tenant so misses route cleanly.
3. Verify registration payload reports `tenants: [...]` by inspecting `POST /api/v1/im/bridge/register` traffic or the operator dashboard.
4. Switch to `IM_PLATFORMS=<current>,<new>` and deploy again. Confirm both providers' notify ports are bound (default `NOTIFY_PORT` and `NOTIFY_PORT+1`) or override via `IM_NOTIFY_PORT_<PROVIDER>`.
5. Narrow `tenants.yaml` resolvers to add the new provider's chats/workspaces per tenant.
6. Remove `IM_PLATFORM` once every deployment is on `IM_PLATFORMS`; the alias is retained only for transitional compatibility.

### Diagnostics

| Symptom | Likely cause | Where to look |
| --- | --- | --- |
| "该会话未配置 tenant 绑定" replies in chat | Resolver miss without `IM_TENANT_DEFAULT` | `tenants.yaml` resolver bindings; operator audit events `metadata.reason=tenant_unresolved` |
| One provider healthy, another silent | Per-provider credential missing; startup logged `provider <id>` fail | `journalctl`/container logs for the startup line identifying the offending provider |
| Notify port in-use error | Default offset clashes with another service | Set `IM_NOTIFY_PORT_<PROVIDER>` per provider |
| Plugin command not dispatching | Manifest invalid or watcher hasn't picked up the new file yet | Check `${IM_BRIDGE_PLUGIN_DIR}/<id>/plugin.yaml`; watcher polls every 30s |
| Tenant leaking rate limit counters | Policy does not declare `DimTenant` | Set `IM_RATE_POLICY` to include `tenant` dimension in at least one policy |

## Feature Matrix

| Platform | Readiness tier | Structured surface | Action callback mode | Reply target restored | Preferred async update path | Explicit downgrade |
| --- | --- | --- | --- | --- | --- | --- |
| Feishu | `full_native_lifecycle` | interactive cards, JSON cards, template cards | card callback | chat id, message id, callback token | immediate toast/reply first, delayed native card update when available | reply/send fallback with explicit fallback reason when delayed update cannot be used; shared action completions use provider-owned richer text/card builders |
| Slack | `full_native_lifecycle` | Block Kit | Socket Mode interactive payload | channel, thread, `response_url` | thread reply or `response_url` follow-up | plain text only if block rendering is unavailable |
| DingTalk | `full_native_lifecycle` (mutable_update_method=`openapi_only`) | ActionCard send with text fallback | Stream card callback | session webhook, conversation id, conversation type | session webhook first, OpenAPI update when the card originated via OpenAPI | ActionCard or editable-update requests for webhook-origin cards degrade explicitly |
| WeCom | `full_native_lifecycle` (mutable_update_method=`template_card_update`) | template-card/markdown-compatible richer send | webhook callback payload | chat id, user id, `response_url` | `response_url` reply first, template_card_update fallback | richer updates outside template_card scope degrade to markdown/text |
| QQ | `text_first` (mutable_update=`simulated`) | text-first shared rendering | OneBot message event payload | group id, user id, message id | reply in the same chat using reply-segment compatible sends | mutable-update is simulated via delete+resend with thread context; structured/native requests downgrade to text |
| QQ Bot | `native_send_with_fallback` (mutable_update_method=`openapi_patch`) | markdown/keyboard-first shared rendering | webhook callback payload | group openid or user openid, `msg_id` | reply when `msg_id` exists, OpenAPI PATCH when available | mutable-update or incompatible richer requests are sent as explicit downgraded text output |
| Telegram | n/a | inline keyboard | callback query | chat id, message id, `message_thread_id` topic | reply or `editMessageText` depending on target | card-like payloads collapse to text plus inline keyboard; optional MarkdownV2 delivery escapes content first and oversized formatted updates fall back to segmented replies |
| Discord | n/a | message components | `/interactions` component payload | channel id, interaction token, original response id | deferred ack, follow-up, original response patch | unsupported interaction types return explicit ephemeral failure |

### Attachments / Reactions / Threads capability matrix

| Platform | Attachments (max size) | Reactions | Threads (policies) |
| --- | --- | --- | --- |
| Slack | ✅ (1 GB) | ✅ unified set | reuse / open / isolate |
| Discord | ✅ (25 MB) | ✅ unified set | reuse / open / isolate |
| Feishu | ✅ (20 MB) | ✅ unified set | reuse / open / isolate |
| Telegram | ✅ (50 MB) | ✅ unified set | reuse / isolate |
| DingTalk | ❌ | ❌ | prefix via isolate |
| WeCom | ❌ | ❌ | prefix via isolate |
| QQ | ❌ | ❌ | prefix via isolate |
| QQ Bot | ❌ | ❌ | prefix via isolate |

Unified reaction codes: `ack`, `running`, `done`, `failed`, `thumbs_up`, `thumbs_down`, `eyes`, `question`.

Unsupported attachment/reaction/thread requests degrade to a text summary or a `[session: ...]` prefixed reply and always carry a `fallback_reason` (`attachments_unsupported`, `thread_open_unsupported`, etc.) in the delivery receipt.

## Control-Plane Prerequisites

Before promoting a bridge deployment, make sure the runtime control plane is configured consistently on both the Go backend and the bridge process:

- `IM_CONTROL_SHARED_SECRET` must match on both sides when compatibility HTTP fallback is enabled
- `IM_BRIDGE_ID_FILE` should point to durable local storage so the bridge keeps the same `bridge_id` across restarts
- `IM_BRIDGE_HEARTBEAT_INTERVAL` should be comfortably lower than the backend TTL
- the backend must be reachable for both `POST /api/v1/im/bridge/*` and `GET /ws/im-bridge`

Operational expectations:

1. Bridge starts, registers `bridge_id`, and appears as online in backend state
2. Heartbeats keep the instance live while the process is healthy
3. Graceful shutdown unregisters the instance immediately
4. WebSocket reconnect resumes from the last acked cursor instead of replaying already-processed deliveries

Canonical delivery expectations:

- direct `POST /im/send` and `POST /im/notify` requests may now carry `content`, `structured`, `native`, `replyTarget`, and `metadata`
- replayed `/ws/im-bridge` deliveries preserve the same typed payload shape instead of flattening everything to text
- `metadata.fallback_reason` and similar operator diagnostics must survive queueing, replay, and compatibility fallback
- normalized `/im/action` callbacks now expect truthful backend states such as `started`, `completed`, `blocked`, or `failed`, rather than placeholder success text
- when delivery metadata requests a supported formatted mode such as Telegram `text_format=markdown_v2`, the provider renderer applies escaping and transport-specific limits before issuing `sendMessage` or `editMessageText`
- compat send, compat notify, action results, bound progress, and replayed deliveries preserve provider completion hints through `reply_target_*` metadata such as `reply_target_progress_mode`, `reply_target_session_webhook`, `reply_target_response_url`, and `reply_target_conversation_id`

Bridge event-forwarding preferences can also be expressed through bound
`replyTarget.metadata` values. Current per-event keys follow the form:

- `bridge_event_enabled.permission_request`
- `bridge_event_enabled.status_change`
- `bridge_event_enabled.budget.warning`

When these keys are present, the control plane suppresses disabled Bridge event
deliveries before they reach the IM transport while still keeping backend
delivery history truthful.

## Rollout Checklist

1. Set `IM_PLATFORM` to a single platform and `IM_TRANSPORT_MODE=live`.
2. Populate only the credentials needed by that platform.
3. For Discord, expose `http://<host>:<DISCORD_INTERACTIONS_PORT>/interactions` and configure it as the interactions endpoint.
4. For Telegram, make sure no webhook configuration remains in the environment when long polling is enabled.
5. For WeCom, expose the configured callback endpoint and verify the callback token/path match the live deployment.
6. For QQ, verify the OneBot websocket connects cleanly and the bridge can receive one inbound command plus one outbound send action before promotion.
7. For QQ Bot, expose the configured callback endpoint and verify the OpenAPI text-send path can deliver one group or user follow-up before promotion.
8. Start the bridge and confirm `/im/health` reports the expected `platform`, normalized `source`, `readiness_tier`, and capability matrix fields.
9. Run a command path, a native action path, a control-plane replay path, and a notification path before promoting the deployment.
10. For Feishu, verify both JSON-card and template-card notifications if the deployment depends on richer card payloads.
11. For Telegram, verify one `text_format=markdown_v2` delivery and one oversized formatted completion so the segmented fallback path is exercised explicitly.

## Rollback Guidance

- If a live provider starts failing during rollout, switch `IM_TRANSPORT_MODE=stub` for local diagnosis instead of silently leaving the bridge in a broken live state.
- If the deployment needs to revert to the previous active platform, change only `IM_PLATFORM` and that platform's credentials; the bridge is still single-platform per process.
- If Discord command registration is causing rollout delays, set `DISCORD_COMMAND_GUILD_ID` to a development guild first, validate there, then remove it for global sync.
- If Telegram long polling needs to be disabled, stop the bridge before reconfiguring webhook-based infrastructure; the current implementation intentionally rejects mixed polling/webhook state.
- If WeCom callback delivery fails, verify the exposed callback URL, callback token, and direct-send credentials before falling back to stub mode for diagnosis.
- If QQ websocket delivery fails, verify the OneBot websocket URL, access token, and upstream adapter availability before falling back to stub mode for diagnosis.
- If QQ Bot callback delivery fails, verify the exposed callback URL, app credentials, and OpenAPI reachability before falling back to stub mode for diagnosis.
- If DingTalk users need mutable card updates or richer lifecycle parity beyond the current `native_send_with_fallback` tier, do not fake parity; keep the explicit fallback behavior and document the missing capability in rollout notes.
- If the new control-plane WebSocket path is unstable, keep the bridge registered but temporarily fall back to signed compatibility `POST /im/send` and `POST /im/notify` while investigating.
- If duplicate notifications appear, verify the bridge still uses a stable `IM_BRIDGE_ID_FILE` and confirm `delivery_id` headers are preserved by any reverse proxy.

## Manual Verification Matrix

| Platform | Startup check | Inbound check | Native action check | Reply/update check | Notification check | Stub smoke fixture |
| --- | --- | --- | --- | --- | --- | --- |
| Feishu | Bridge starts with `feishu-live`, registers a stable `bridge_id`, and `/im/health` source `feishu` | Send a message or mention to the app in a subscribed chat | Click a card button and confirm the callback reaches `/im/action` with message and callback metadata preserved, whether the deployment uses long connection only or an exposed webhook callback | Confirm the card callback returns an immediate toast or card update, and long-running work can later use the preserved callback token for delayed native card update | `POST /im/notify` with signed headers and `platform=feishu` can send JSON cards, template cards, builder-owned richer text cards, or fallback replies depending on reply-target context | `scripts/smoke/fixtures/feishu.json` |
| Slack | Bridge logs `slack-live`, registers, and Socket Mode connects cleanly | Trigger `/queue list` or an app mention | Click a Block Kit button or submit a modal and confirm `/im/action` receives channel, thread, and `response_url` context | Confirm a threaded or `response_url` reply arrives after the Socket Mode ack | Matching Slack notification reaches the target channel, mismatched platform is rejected, replay stays in the original thread | `scripts/smoke/fixtures/slack.json` |
| DingTalk | Bridge logs `dingtalk-live`, registers, and `/im/health` reports readiness tier `native_send_with_fallback` | Send `/agent list` as the compatibility alias or `/agent status` in a chat using Stream mode | Trigger a card callback payload and confirm it normalizes into `/im/action` with session webhook or conversation context | Confirm callback results use session webhook first, then conversation-scoped fallback when webhook is absent; editable-update requests should report explicit fallback | Structured notifications fall back to explicit text when richer send/update is unavailable and duplicate `delivery_id` values are suppressed | `scripts/smoke/fixtures/dingtalk.json` |
| WeCom | Bridge starts with `wecom-live`, registers, and `/im/health` reports readiness tier `native_send_with_fallback` | Post a callback payload or send a message through the configured WeCom bot/application path | Confirm the callback normalizes into shared commands with `wecom` source, chat id, user id, and `response_url` preserved | Confirm reply flows prefer `response_url`, while replayed or later notifications can fall back to direct app send when no callback reply is available; editable-update requests should report explicit fallback | Matching WeCom notifications use provider-owned structured/text resolution and report explicit fallback when a richer update path is unavailable | `scripts/smoke/fixtures/wecom.json` |
| QQ | Bridge starts with `qq-live`, registers, and `/im/health` reports readiness tier `text_first` | Send `/memory search release` or `/help` through the OneBot-compatible QQ transport | Confirm the inbound message normalizes into shared commands with `qq` source, chat id, user id, and message id preserved | Confirm replies stay in the originating group or private chat and any request for editable/native update reports explicit fallback instead of claiming rich-card parity | Matching QQ notifications use text delivery and explicit fallback when a richer payload cannot be honored | `scripts/smoke/fixtures/qq.json` |
| QQ Bot | Bridge starts with `qqbot-live`, registers, and `/im/health` reports readiness tier `markdown_first` | Post a QQ Bot webhook payload for `/team list` or another group or direct message command | Confirm the callback normalizes into shared commands with `qqbot` source, group or user openid, and `msg_id` preserved | Confirm replies use preserved `msg_id` when present, markdown/keyboard sends remain available when supported, and mutable-update requests fall back explicitly | Matching QQ Bot notifications use markdown/text delivery and explicit fallback when a richer payload cannot be honored | `scripts/smoke/fixtures/qqbot.json` |
| Telegram | Bridge starts with `telegram-live`, registers, and no webhook env configured | Send `/memory search release` or `/help` to the bot while polling is running | Tap an inline keyboard button and confirm callback query metadata reaches `/im/action` | Confirm `answerCallbackQuery` clears the spinner and later completion edits or replies to the original message/topic; verify oversized formatted completion degrades to segmented replies | Matching Telegram notification sends plain text or inline keyboard to the configured chat id, and `text_format=markdown_v2` deliveries send escaped MarkdownV2 with `parse_mode` | `scripts/smoke/fixtures/telegram.json` |
| Discord | Bridge starts with `discord-live`, registers, syncs commands, and listens on `/interactions` | Trigger `/agent status` or `/help` from a guild or DM | Click a message component and confirm `/im/action` receives `custom_id`, interaction token, and original response context | Confirm the deferred ack is immediate and later progress edits the original response or posts a follow-up as expected | Matching Discord notification sends a channel message using the bot token and replay does not duplicate the first ack | `scripts/smoke/fixtures/discord.json` |

## Stub Smoke Usage

Run the bridge in stub mode for the chosen platform, then execute the generic smoke script:

```powershell
cd src-im-bridge
$env:IM_PLATFORM = "telegram"
$env:IM_TRANSPORT_MODE = "stub"
go run .\cmd\bridge
```

In a second shell:

```powershell
cd src-im-bridge
.\scripts\smoke\Invoke-StubSmoke.ps1 -Platform telegram -Port 7780
```

The same script works for `feishu`, `slack`, `dingtalk`, `discord`, `wecom`, `qq`, and `qqbot` by switching the `-Platform` value.

Recommended focused verification after native interaction changes:

```powershell
cd src-im-bridge
go test ./platform/slack ./platform/feishu ./platform/telegram ./platform/discord ./platform/dingtalk ./platform/wecom ./platform/qq ./platform/qqbot -count=1
go test ./core -run 'Test(ResolveReplyPlan_|DeliverText_|DeliverNative_|DeliverEnvelope_|MetadataForPlatform_|StructuredMessageFallbackText|ReplyTarget_JSONRoundTrip|NativeMessage_)' -count=1
go test ./client -run 'Test(HandleIMAction_SendsCanonicalPayloadAndParsesReplyTarget|HandleIMAction_ParsesCanonicalActionOutcome|WithSource_NormalizesHeaderValue|WithPlatform_UsesTelegramMetadataSource|WithPlatform_UsesWeComMetadataSource|WithPlatform_UsesQQMetadataSource|WithPlatform_UsesQQBotMetadataSource)' -count=1
go test ./notify -run 'TestReceiver_(ActionResponseUsesReplyTargetDelivery|HealthReportsNormalizedTelegramSourceAndCapabilities|HealthReportsNormalizedWeComSourceAndCapabilities|HealthReportsNormalizedQQSourceAndCapabilities|HealthReportsNormalizedQQBotSourceAndCapabilities|FallsBackToStructuredTextWhenNativeStructuredSenderUnavailable|PrefersNativePayloadWhenPlatformSupportsIt|UsesDeferredNativeUpdateWhenFeishuReplyTargetSupportsIt|ReportsFallbackReasonWhenDeferredUpdateContextMissing|SuppressesDuplicateSignedCompatibilityDelivery|RejectsUnsignedCompatibilityDeliveryWhenSecretConfigured)' -count=1
go test ./cmd/bridge -run 'Test(ConfigurePlatformActionCallbacks_|SelectProvider_|SelectPlatform_|LookupPlatformDescriptor_|BridgeRuntimeControl_)' -count=1
```

## Manual Replay Verification

1. Start the bridge in stub mode with `IM_CONTROL_SHARED_SECRET` set.
2. Trigger a long-running command such as `/agent run` or `/task assign`.
3. Confirm the bridge binds the originating `reply_target` and receives at least one control-plane progress delivery.
4. Stop the bridge process before the task finishes.
5. Restart the bridge with the same `IM_BRIDGE_ID_FILE`.
6. Confirm the backend replays only deliveries after the last acked cursor and the user-visible IM target does not receive a duplicate acceptance message.

For Feishu delayed-update validation, also confirm:

1. A card action response returns within the callback window.
2. `/help` only shows callback-backed quick actions when the active runtime can actually intake `card.action.trigger`; otherwise it falls back to plain command guidance.
3. A follow-on native update uses the preserved callback token when available.
4. If the callback token is missing or unusable, `/im/notify` reports a `fallback_reason` instead of silently pretending the delayed update succeeded.

For China-platform parity checks, also confirm:

1. `/im/health` exposes the expected `readiness_tier` for Feishu, DingTalk, WeCom, QQ, and QQ Bot.
2. Bridge registration metadata carries the same readiness tier plus matching preferred/fallback async update mode hints.
3. `capability_matrix.readinessTier`, `capability_matrix.preferredAsyncUpdateMode`, and `capability_matrix.fallbackAsyncUpdateMode` match the provider's actual completion path.
4. Requests for unsupported editable/deferred update behavior produce explicit fallback metadata instead of silently degrading.

## Operator Snapshot Notes

The `/im` operator console now assumes the backend exposes a richer snapshot contract:

- `/api/v1/im/bridge/status` should include pending backlog, recent failures, average settled latency, and provider diagnostics metadata in addition to registration health
- `/api/v1/im/deliveries` should support explicit filtering so operators can isolate one delivery, platform, event type, or recent window
- `/api/v1/im/deliveries/retry-batch` is the preferred workflow for retrying multiple failed or timed-out deliveries
- `/api/v1/im/test-send` should reuse the canonical send pipeline and return the delivery id together with the current bounded settlement result

Delivery status in that operator view is settlement-truthful. A delivery remains `pending` until the bridge returns a terminal ack with processed timestamp and any failure or downgrade metadata. Operators should not treat queue acceptance alone as successful delivery.

## Phase 1 Feishu Callback Closure Smoke

Requires:
- Feishu live bridge running with `FEISHU_APP_ID`, `FEISHU_APP_SECRET`, and (optional) callback webhook config
- Go backend running with migration 054 applied
- A card published in a Feishu test chat that contains one instance of each element type

Steps:

1. **Button (existing behavior)** — click approve button on a card with action ref `act:approve:<reviewID>`. Expect review transitions to approved, toast "Review … was approved".
2. **Select** — click a `select_static` whose value is `{"action": "act:transition-task:<taskID>"}` and options `["inbox", "triaged", …]`. Pick "done". Expect task transitions, toast success.
3. **Multi-select** — pick two agents on a multi_select with value `{"action": "act:assign-agent:<taskID>"}`. Expect task assigned to the first agent, `selected_options` in backend logs.
4. **Date picker** — pick a date. Expect Blocked toast "Due-date workflow is not configured; received YYYY-MM-DD …".
5. **Overflow** — pick an option whose value is `act:decompose:<taskID>`. Expect task decomposed.
6. **Checker** — toggle on a card whose value is `act:toggle:<taskID>`. Expect task moves to done; toggle back → task moves to in_progress.
7. **Input** — type "please reconsider" and submit on a card whose value is `act:input_submit:<taskID>`. Expect comment appended.
8. **Form** — submit a form with `name="create-task-form"` and fields `title`, `body`, `priority`. Expect task created.
9. **Reaction** — react with 👍 on a task notification message. Expect row in `im_reaction_events` with `emoji="THUMBSUP"` and `event_type="created"`. Remove the reaction — expect row with `event_type="deleted"`.

Each step must produce a deterministic toast or status — none should show "Unknown action".

## Security & ops troubleshooting

### Timestamp window rejections (`408 timestamp_out_of_window`)

- Confirm NTP is healthy on the bridge host: a 10+ minute clock drift immediately triggers rejections even for freshly signed requests.
- Check the control-plane emitter's timezone: timestamps must be UTC and valid RFC3339 or Unix seconds.
- If a planned maintenance window produced a burst of `408`s across many deliveries, widen `IM_SIGNATURE_SKEW_SECONDS` temporarily and investigate why the emitter's clock drifted.

### Duplicate delivery spam (`409 duplicate_delivery`)

- Expected behavior for retries; verify the backend's retry policy does not loop on 409s (they are `retryable=false`).
- If a restart triggered `409`s on deliveries the backend thought were new, the durable state store is working as designed — the retry found the prior success in SQLite.

### Rate limit 429-equivalent responses

- The bridge reply text includes the policy id (`policy=write-action`). Map that to the JSON override in `IM_RATE_POLICY` if you need to widen the gate.
- `audit.jsonl` carries `status=rate_limited` with `metadata.rate_policy` — the quickest way to spot noisy actors.

### Audit file ingestion

- The writer is append-only JSONL; existing log-shipper pipelines (Fluent Bit / Filebeat / Vector) that tail-follow JSONL files work without config beyond path.
- Rotation produces `audit.YYYY-MM-DD-HHMM.jsonl`; ensure your shipper picks up the rotated files or run the cleanup cadence high enough to avoid data loss during backpressure.

### Hot reload (SIGHUP)

- `kill -HUP <pid>` triggers a reload. Non-reloadable providers log `manual_restart_required`; check the log for the exact field names deferred.
- Windows installations do not receive SIGHUP; rotate credentials by restarting the service.

### Rolling back the hardening

Each capability has an explicit disable switch; prefer disabling the narrowest offender first:

| Capability | Disable |
|------------|---------|
| Durable state store | `IM_DISABLE_DURABLE_STATE=true` |
| Skew window | `IM_SIGNATURE_SKEW_SECONDS=86400` (24h, effectively off) |
| Audit log | `IM_DISABLE_AUDIT=true` |
| Egress sanitizer | `IM_SANITIZE_EGRESS=off` |
| Command allowlist | Unset `IM_COMMAND_ALLOWLIST` |
| Rate policies | Unset `IM_RATE_POLICY` to return to defaults |

