# IM Bridge Platform Runbook

This runbook covers the live transport expectations, rollout steps, rollback steps, and manual verification matrix for the IM Bridge platforms currently supported in `src-im-bridge`.

The bridge now resolves providers through a local provider contract before startup. Built-in providers still live in-tree, but the runtime, `/im/health`, and control-plane surfaces now consume provider descriptor metadata instead of relying on startup-only hard-coded branches.

That descriptor now also carries provider rendering-profile metadata. Delivery paths resolve a rendering plan before transport execution, so formatting decisions, mutable-update rules, and provider-specific richer builders stay aligned across `/im/send`, `/im/notify`, `/im/action`, and replay.

## Preferred Live Transport

| Platform | Preferred transport | Required live credentials | Notes |
| --- | --- | --- | --- |
| Feishu | long connection | `FEISHU_APP_ID`, `FEISHU_APP_SECRET` | HTTP callback remains an explicit seam for callback types that still require it. |
| Slack | Socket Mode | `SLACK_BOT_TOKEN`, `SLACK_APP_TOKEN` | Requires app-level token and Socket Mode enablement. |
| DingTalk | Stream mode | `DINGTALK_APP_KEY`, `DINGTALK_APP_SECRET` | Stream mode is the default live intake; structured notifications currently downgrade to text explicitly. |
| WeCom | callback-driven app messaging | `WECOM_CORP_ID`, `WECOM_AGENT_ID`, `WECOM_AGENT_SECRET`, `WECOM_CALLBACK_TOKEN`, `WECOM_CALLBACK_PORT` | `WECOM_CALLBACK_PATH` defaults to `/wecom/callback`; inbound callbacks can reply through `response_url` and later direct sends use app-message APIs. |
| Telegram | long polling | `TELEGRAM_BOT_TOKEN` | Current live implementation supports `TELEGRAM_UPDATE_MODE=longpoll` only and rejects webhook config. |
| Discord | outgoing webhook interactions | `DISCORD_APP_ID`, `DISCORD_BOT_TOKEN`, `DISCORD_PUBLIC_KEY`, `DISCORD_INTERACTIONS_PORT` | Optional `DISCORD_COMMAND_GUILD_ID` scopes command sync to a guild for faster rollout. |

All platforms support `IM_TRANSPORT_MODE=stub` for local verification and `IM_TRANSPORT_MODE=live` for production traffic.

## Feature Matrix

| Platform | Structured surface | Action callback mode | Reply target restored | Preferred async update path | Explicit downgrade |
| --- | --- | --- | --- | --- | --- |
| Feishu | interactive cards, JSON cards, template cards | card callback | chat id, message id, callback token | immediate toast/reply first, delayed native card update when available | reply/send fallback with explicit fallback reason when delayed update cannot be used; shared action completions use provider-owned richer text/card builders |
| Slack | Block Kit | Socket Mode interactive payload | channel, thread, `response_url` | thread reply or `response_url` follow-up | plain text only if block rendering is unavailable |
| DingTalk | ActionCard planned, text fallback active | Stream card callback | session webhook, conversation id, conversation type | session webhook first, direct send fallback | structured payloads are sent as explicit downgraded text today |
| WeCom | card-compatible structured profile with text fallback | webhook callback payload | chat id, user id, `response_url` | `response_url` reply first, direct app send fallback | richer or mutable updates fall back to text when the current WeCom path cannot honor them |
| Telegram | inline keyboard | callback query | chat id, message id, `message_thread_id` topic | reply or `editMessageText` depending on target | card-like payloads collapse to text plus inline keyboard; optional MarkdownV2 delivery escapes content first and oversized formatted updates fall back to segmented replies |
| Discord | message components | `/interactions` component payload | channel id, interaction token, original response id | deferred ack, follow-up, original response patch | unsupported interaction types return explicit ephemeral failure |

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

## Rollout Checklist

1. Set `IM_PLATFORM` to a single platform and `IM_TRANSPORT_MODE=live`.
2. Populate only the credentials needed by that platform.
3. For Discord, expose `http://<host>:<DISCORD_INTERACTIONS_PORT>/interactions` and configure it as the interactions endpoint.
4. For Telegram, make sure no webhook configuration remains in the environment when long polling is enabled.
5. For WeCom, expose the configured callback endpoint and verify the callback token/path match the live deployment.
5. Start the bridge and confirm `/im/health` reports the expected `platform`, normalized `source`, and capability matrix fields.
6. Run a command path, a native action path, a control-plane replay path, and a notification path before promoting the deployment.
7. For Feishu, verify both JSON-card and template-card notifications if the deployment depends on richer card payloads.
8. For Telegram, verify one `text_format=markdown_v2` delivery and one oversized formatted completion so the segmented fallback path is exercised explicitly.

## Rollback Guidance

- If a live provider starts failing during rollout, switch `IM_TRANSPORT_MODE=stub` for local diagnosis instead of silently leaving the bridge in a broken live state.
- If the deployment needs to revert to the previous active platform, change only `IM_PLATFORM` and that platform's credentials; the bridge is still single-platform per process.
- If Discord command registration is causing rollout delays, set `DISCORD_COMMAND_GUILD_ID` to a development guild first, validate there, then remove it for global sync.
- If Telegram long polling needs to be disabled, stop the bridge before reconfiguring webhook-based infrastructure; the current implementation intentionally rejects mixed polling/webhook state.
- If WeCom callback delivery fails, verify the exposed callback URL, callback token, and direct-send credentials before falling back to stub mode for diagnosis.
- If DingTalk users need structured controls before ActionCard send is promoted, do not fake parity; keep the current explicit text downgrade and document the missing card-send step in rollout notes.
- If the new control-plane WebSocket path is unstable, keep the bridge registered but temporarily fall back to signed compatibility `POST /im/send` and `POST /im/notify` while investigating.
- If duplicate notifications appear, verify the bridge still uses a stable `IM_BRIDGE_ID_FILE` and confirm `delivery_id` headers are preserved by any reverse proxy.

## Manual Verification Matrix

| Platform | Startup check | Inbound check | Native action check | Reply/update check | Notification check | Stub smoke fixture |
| --- | --- | --- | --- | --- | --- | --- |
| Feishu | Bridge starts with `feishu-live`, registers a stable `bridge_id`, and `/im/health` source `feishu` | Send a message or mention to the app in a subscribed chat | Click a card button and confirm the callback reaches `/im/action` with message and callback metadata preserved | Confirm the card callback returns an immediate toast, and long-running work can later use the preserved callback token for delayed native card update | `POST /im/notify` with signed headers and `platform=feishu` can send JSON cards, template cards, builder-owned richer text cards, or fallback replies depending on reply-target context | `scripts/smoke/fixtures/feishu.json` |
| Slack | Bridge logs `slack-live`, registers, and Socket Mode connects cleanly | Trigger `/task list` or an app mention | Click a Block Kit button or submit a modal and confirm `/im/action` receives channel, thread, and `response_url` context | Confirm a threaded or `response_url` reply arrives after the Socket Mode ack | Matching Slack notification reaches the target channel, mismatched platform is rejected, replay stays in the original thread | `scripts/smoke/fixtures/slack.json` |
| DingTalk | Bridge logs `dingtalk-live`, registers, and Stream intake starts | Send a bot message in a chat using Stream mode | Trigger a card callback payload and confirm it normalizes into `/im/action` with session webhook or conversation context | Confirm callback results use session webhook first, then conversation-scoped fallback when webhook is absent | Structured notifications fall back to explicit text when rich send is unavailable and duplicate `delivery_id` values are suppressed | `scripts/smoke/fixtures/dingtalk.json` |
| WeCom | Bridge starts with `wecom-live`, registers, and exposes the configured callback path | Post a callback payload or send a message through the configured WeCom bot/application path | Confirm the callback normalizes into shared commands with `wecom` source, chat id, user id, and `response_url` preserved | Confirm reply flows prefer `response_url`, while replayed or later notifications can fall back to direct app send when no callback reply is available | Matching WeCom notifications use provider-owned structured/text resolution and report explicit fallback when a richer update path is unavailable | `scripts/smoke/fixtures/wecom.json` |
| Telegram | Bridge starts with `telegram-live`, registers, and no webhook env configured | Send `/task list` or `/help` to the bot while polling is running | Tap an inline keyboard button and confirm callback query metadata reaches `/im/action` | Confirm `answerCallbackQuery` clears the spinner and later completion edits or replies to the original message/topic; verify oversized formatted completion degrades to segmented replies | Matching Telegram notification sends plain text or inline keyboard to the configured chat id, and `text_format=markdown_v2` deliveries send escaped MarkdownV2 with `parse_mode` | `scripts/smoke/fixtures/telegram.json` |
| Discord | Bridge starts with `discord-live`, registers, syncs commands, and listens on `/interactions` | Trigger `/agent` or `/help` from a guild or DM | Click a message component and confirm `/im/action` receives `custom_id`, interaction token, and original response context | Confirm the deferred ack is immediate and later progress edits the original response or posts a follow-up as expected | Matching Discord notification sends a channel message using the bot token and replay does not duplicate the first ack | `scripts/smoke/fixtures/discord.json` |

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

The same script works for `feishu`, `slack`, `dingtalk`, `discord`, and `wecom` by switching the `-Platform` value.

Recommended focused verification after native interaction changes:

```powershell
cd src-im-bridge
go test ./platform/slack ./platform/feishu ./platform/telegram ./platform/discord ./platform/dingtalk ./platform/wecom -count=1
go test ./core -run 'Test(ResolveReplyPlan_|DeliverText_|DeliverNative_|MetadataForPlatform_|StructuredMessageFallbackText|ReplyTarget_JSONRoundTrip|NativeMessage_)' -count=1
go test ./client -run 'Test(HandleIMAction_SendsCanonicalPayloadAndParsesReplyTarget|HandleIMAction_ParsesCanonicalActionOutcome|WithSource_NormalizesHeaderValue|WithPlatform_UsesTelegramMetadataSource|WithPlatform_UsesWeComMetadataSource)' -count=1
go test ./notify -run 'TestReceiver_(ActionResponseUsesReplyTargetDelivery|HealthReportsNormalizedTelegramSourceAndCapabilities|HealthReportsNormalizedWeComSourceAndCapabilities|FallsBackToStructuredTextWhenNativeStructuredSenderUnavailable|PrefersNativePayloadWhenPlatformSupportsIt|UsesDeferredNativeUpdateWhenFeishuReplyTargetSupportsIt|ReportsFallbackReasonWhenDeferredUpdateContextMissing|SuppressesDuplicateSignedCompatibilityDelivery|RejectsUnsignedCompatibilityDeliveryWhenSecretConfigured)' -count=1
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
2. A follow-on native update uses the preserved callback token when available.
3. If the callback token is missing or unusable, `/im/notify` reports a `fallback_reason` instead of silently pretending the delayed update succeeded.
