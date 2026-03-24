# IM Bridge

`src-im-bridge` is the Go IM Bridge that connects one active IM platform instance to the shared AgentForge command engine and backend API.

## Platform Selection

Set `IM_PLATFORM` to exactly one of:

- `feishu`
- `slack`
- `dingtalk`
- `telegram`
- `discord`

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

## Current Adapter Mode

Every supported platform ships both a local stub adapter and a live transport path.

- `feishu`: stub + live long connection, card-capable
- `slack`: stub + live Socket Mode, rich reply fallback to blocks
- `dingtalk`: stub + live Stream mode, text fallback for notifications
- `telegram`: stub + live long polling, text-only notifications
- `discord`: stub + live HTTP interactions, deferred reply + follow-up

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

- matching platform + `CardSender` support: send structured card
- matching platform without `CardSender`: fall back to plain text
- mismatched platform: reject the notification request

## Live Transport Summary

| Platform | Preferred live transport | Public callback required | Rich notifications |
| --- | --- | --- | --- |
| Feishu | long connection | Only for callback types that still require HTTP handling | Yes |
| Slack | Socket Mode | No | Yes |
| DingTalk | Stream mode | No | Fallback to text |
| Telegram | long polling | No | Fallback to text |
| Discord | outgoing webhook interactions | Yes, `/interactions` endpoint | Fallback to text |

## Rollout And Rollback

- Roll out one active platform per process by setting `IM_PLATFORM` and `IM_TRANSPORT_MODE=live`.
- Verify `/im/health`, one inbound command, one reply path, and one notification path before promoting a deployment.
- For Discord, verify command sync completed and the interactions endpoint is reachable before exposing the deployment broadly.
- For Telegram, remove webhook config before enabling long polling.
- To roll back, switch the deployment back to the previous platform or move the current platform to `IM_TRANSPORT_MODE=stub` for local diagnosis.

## Smoke Tests

Local stub smoke fixtures are stored under [scripts/smoke](/d:/Project/AgentForge/src-im-bridge/scripts/smoke). Use [Invoke-StubSmoke.ps1](/d:/Project/AgentForge/src-im-bridge/scripts/smoke/Invoke-StubSmoke.ps1) with the matching platform fixture after starting the bridge in stub mode.

Detailed rollout, rollback, and manual verification guidance is documented in [platform-runbook.md](/d:/Project/AgentForge/src-im-bridge/docs/platform-runbook.md).

## Local Verification

Run the IM bridge test suite from the package root:

```powershell
cd src-im-bridge
go test ./...
```
