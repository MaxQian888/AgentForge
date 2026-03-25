# IM Bridge

`src-im-bridge` is the Go IM Bridge that connects one active IM platform instance to the shared AgentForge command engine and backend API.

## Platform Selection

Startup now resolves `IM_PLATFORM` through a bridge-local provider contract rather than a hard-coded startup branch. Built-in providers still ship in-tree, but they now expose a normalized descriptor with:

- provider id
- supported transport modes
- capability metadata consumed by health/control-plane/notify paths
- optional provider-native extension declarations for richer message surfaces

This keeps the current single-active-provider-per-process model intact while making future externalized IM providers or richer provider-specific capabilities easier to add without rewriting `main.go`.

Set `IM_PLATFORM` to exactly one of:

- `feishu`
- `slack`
- `dingtalk`
- `telegram`
- `discord`

`wecom` currently remains a planned provider only. It appears in shared model enums for roadmap completeness, but `src-im-bridge` does not yet ship a runnable adapter or activation path for it.

Set `IM_TRANSPORT_MODE` explicitly:

- `stub`: local verification and offline development
- `live`: real provider transport, credential validation, and production delivery semantics

The bridge validates credentials for the selected platform before startup:

- `feishu`: `FEISHU_APP_ID` and `FEISHU_APP_SECRET` for live long connection
- `slack`: required `SLACK_BOT_TOKEN` and `SLACK_APP_TOKEN`
- `dingtalk`: required `DINGTALK_APP_KEY` and `DINGTALK_APP_SECRET`
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

- `feishu`: stub + live long connection, card-capable, native JSON/template card payload support, delayed card update support
- `slack`: stub + live Socket Mode, Block Kit callbacks + `response_url`
- `dingtalk`: stub + live Stream mode, session-webhook replies + explicit structured downgrade
- `telegram`: stub + live long polling, inline keyboard + callback query + message edit
- `discord`: stub + live HTTP interactions, deferred reply + follow-up + original-response edit
- `wecom`: planned only, intentionally not runnable until an adapter and declared capability matrix land

Stub adapters expose local test endpoints on `TEST_PORT`:

- `POST /test/message`
- `GET /test/replies`
- `DELETE /test/replies`

## Command and Notification Behavior

All supported platforms reuse the same command engine:

- `/task`
- `/agent`
- `/cost`
- `/help`
- `@AgentForge ...` fallback

The bridge propagates the active message platform to the backend through `X-IM-Source`, so Slack, DingTalk, and Feishu traffic can be distinguished downstream.
This now also applies to Telegram and Discord.

Notifications received on `POST /im/notify` must include a `platform` field matching the active bridge platform:

- matching platform + `NativeMessageSender` support: send provider-native payload first and report the actual delivery method plus any fallback reason
- matching platform + `CardSender` support: send structured card
- matching platform + `StructuredSender` support: send platform-native structured payload
- matching platform without native structured support: fall back to plain text
- mismatched platform: reject the notification request

For Feishu specifically, the native payload surface now supports:

- raw JSON interactive cards
- template cards with `template_id`, optional `template_version_name`, and `template_variable`
- delayed card updates through preserved callback token context when the originating reply target supports it
- explicit `fallback_reason` reporting when delayed update cannot be used and the bridge has to fall back to a reply/send path

Compatibility `POST /im/send` and `POST /im/notify` deliveries are now protected with:

- `X-AgentForge-Delivery-Id`
- `X-AgentForge-Delivery-Timestamp`
- `X-AgentForge-Signature`

When `IM_CONTROL_SHARED_SECRET` is configured, unsigned or invalidly signed compatibility requests are rejected, and duplicate `delivery_id` values are suppressed so retries do not fan out duplicate IM messages.

## Live Transport Summary

| Platform | Preferred live transport | Structured surface | Native callback path | Async update preference | Current downgrade rule |
| --- | --- | --- | --- | --- | --- |
| Feishu | long connection | interactive cards + template cards | card action callback | immediate toast/reply, delayed card update, native fallback reason reporting | falls back to reply/send when native card send or delayed update cannot be used |
| Slack | Socket Mode | Block Kit | interactive payloads via Socket Mode | thread reply, `response_url`, follow-up | falls back to plain text only if blocks cannot be rendered |
| DingTalk | Stream mode | ActionCard planned, text fallback active | Stream card callback | session webhook, then direct send | structured notifications explicitly degrade to text today |
| Telegram | long polling | inline keyboard | callback query | reply or in-place edit | card-like content maps to text plus inline keyboard |
| Discord | outgoing webhook interactions | message components | `/interactions` message component payloads | deferred ack, follow-up, original-response edit | unsupported component cases return explicit ephemeral ack |

## Native Interaction Matrix

| Platform | Command surface | Reply target context preserved | Native action support | Native update support |
| --- | --- | --- | --- | --- |
| Feishu | slash + mention | chat, message, callback token | `card.action.trigger` normalized into `/im/action` with delayed-update context | reply, native card send, or delayed card update |
| Slack | slash + mention + interaction | channel, thread, `response_url` | block action and view submission normalized into `/im/action` | thread reply, `response_url`, follow-up |
| DingTalk | mention + chatbot text | session webhook, conversation id, conversation type | card callback normalized when action reference is present | session webhook reply, conversation fallback |
| Telegram | slash + mention | chat, message, topic | inline keyboard callback query normalized into `/im/action` | `sendMessage`, `editMessageText` |
| Discord | slash + component | channel, interaction token, original response | message component `custom_id` normalized into `/im/action` | deferred ack, follow-up webhook, original response patch |

## Rollout And Rollback

- Roll out one active platform per process by setting `IM_PLATFORM` and `IM_TRANSPORT_MODE=live`.
- Verify `/im/health`, backend bridge registration, one inbound command, one reply path, one control-plane replay, and one notification path before promoting a deployment.
- For Discord, verify command sync completed and the interactions endpoint is reachable before exposing the deployment broadly.
- For Telegram, remove webhook config before enabling long polling and verify callback queries can still be answered quickly enough to avoid stuck button spinners.
- For DingTalk, treat structured notifications as an explicit text downgrade until ActionCard sending is promoted from planned to active.
- To roll back, first disable the control-plane WebSocket or clear `IM_CONTROL_SHARED_SECRET` if compatibility HTTP fallback is needed, then switch the deployment back to the previous platform or move the current platform to `IM_TRANSPORT_MODE=stub` for local diagnosis.

## Smoke Tests

Local stub smoke fixtures are stored under [scripts/smoke](/d:/Project/AgentForge/src-im-bridge/scripts/smoke). Use [Invoke-StubSmoke.ps1](/d:/Project/AgentForge/src-im-bridge/scripts/smoke/Invoke-StubSmoke.ps1) with the matching platform fixture after starting the bridge in stub mode.

Recommended scoped validation after adapter changes:

```powershell
cd src-im-bridge
go test ./platform/slack ./platform/feishu ./platform/telegram ./platform/discord ./platform/dingtalk -count=1
go test ./core -run 'Test(ResolveReplyPlan_|DeliverText_|DeliverNative_|MetadataForPlatform_|StructuredMessageFallbackText|ReplyTarget_JSONRoundTrip|NativeMessage_)' -count=1
go test ./client -run 'Test(HandleIMAction_SendsCanonicalPayloadAndParsesReplyTarget|WithSource_NormalizesHeaderValue|WithPlatform_UsesTelegramMetadataSource)' -count=1
go test ./notify -run 'TestReceiver_(ActionResponseUsesReplyTargetDelivery|HealthReportsNormalizedTelegramSourceAndCapabilities|FallsBackToStructuredTextWhenNativeStructuredSenderUnavailable|PrefersNativePayloadWhenPlatformSupportsIt|UsesDeferredNativeUpdateWhenFeishuReplyTargetSupportsIt|ReportsFallbackReasonWhenDeferredUpdateContextMissing|SuppressesDuplicateSignedCompatibilityDelivery|RejectsUnsignedCompatibilityDeliveryWhenSecretConfigured)' -count=1
go test ./cmd/bridge -run 'Test(SelectProvider_|SelectPlatform_|LookupPlatformDescriptor_|BridgeRuntimeControl_)' -count=1
```

Detailed rollout, rollback, and manual verification guidance is documented in [platform-runbook.md](/d:/Project/AgentForge/src-im-bridge/docs/platform-runbook.md).

## Local Verification

Run the IM bridge test suite from the package root:

```powershell
cd src-im-bridge
go test ./...
```
