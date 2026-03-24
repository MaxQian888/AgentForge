# IM Bridge Platform Runbook

This runbook covers the live transport expectations, rollout steps, rollback steps, and manual verification matrix for the IM Bridge platforms currently supported in `src-im-bridge`.

## Preferred Live Transport

| Platform | Preferred transport | Required live credentials | Notes |
| --- | --- | --- | --- |
| Feishu | long connection | `FEISHU_APP_ID`, `FEISHU_APP_SECRET` | HTTP callback remains an explicit seam for callback types that still require it. |
| Slack | Socket Mode | `SLACK_BOT_TOKEN`, `SLACK_APP_TOKEN` | Requires app-level token and Socket Mode enablement. |
| DingTalk | Stream mode | `DINGTALK_APP_KEY`, `DINGTALK_APP_SECRET` | Stream mode is the default live intake. |
| Telegram | long polling | `TELEGRAM_BOT_TOKEN` | Current live implementation supports `TELEGRAM_UPDATE_MODE=longpoll` only and rejects webhook config. |
| Discord | outgoing webhook interactions | `DISCORD_APP_ID`, `DISCORD_BOT_TOKEN`, `DISCORD_PUBLIC_KEY`, `DISCORD_INTERACTIONS_PORT` | Optional `DISCORD_COMMAND_GUILD_ID` scopes command sync to a guild for faster rollout. |

All platforms support `IM_TRANSPORT_MODE=stub` for local verification and `IM_TRANSPORT_MODE=live` for production traffic.

## Rollout Checklist

1. Set `IM_PLATFORM` to a single platform and `IM_TRANSPORT_MODE=live`.
2. Populate only the credentials needed by that platform.
3. For Discord, expose `http://<host>:<DISCORD_INTERACTIONS_PORT>/interactions` and configure it as the interactions endpoint.
4. For Telegram, make sure no webhook configuration remains in the environment when long polling is enabled.
5. Start the bridge and confirm `/im/health` reports the expected `platform`, normalized `source`, and `supports_rich_messages` state.
6. Run a command path and a notification path before promoting the deployment.

## Rollback Guidance

- If a live provider starts failing during rollout, switch `IM_TRANSPORT_MODE=stub` for local diagnosis instead of silently leaving the bridge in a broken live state.
- If the deployment needs to revert to the previous active platform, change only `IM_PLATFORM` and that platform's credentials; the bridge is still single-platform per process.
- If Discord command registration is causing rollout delays, set `DISCORD_COMMAND_GUILD_ID` to a development guild first, validate there, then remove it for global sync.
- If Telegram long polling needs to be disabled, stop the bridge before reconfiguring webhook-based infrastructure; the current implementation intentionally rejects mixed polling/webhook state.

## Manual Verification Matrix

| Platform | Startup check | Inbound check | Reply check | Notification check | Stub smoke fixture |
| --- | --- | --- | --- | --- | --- |
| Feishu | Bridge starts with `feishu-live` and `/im/health` source `feishu` | Send a message or mention to the app in a subscribed chat | Confirm the reply lands in the same chat or thread | `POST /im/notify` with `platform=feishu` sends text or card depending on capability | `scripts/smoke/fixtures/feishu.json` |
| Slack | Bridge logs `slack-live` and Socket Mode connects cleanly | Trigger `/task list` or an app mention | Confirm a threaded or channel reply arrives after the Socket Mode envelope ack | Matching Slack notification reaches the target channel, mismatched platform is rejected | `scripts/smoke/fixtures/slack.json` |
| DingTalk | Bridge logs `dingtalk-live` and Stream intake starts | Send a bot message in a chat using Stream mode | Confirm webhook reply or direct send is delivered | Structured notifications fall back to text when rich send is unavailable | `scripts/smoke/fixtures/dingtalk.json` |
| Telegram | Bridge starts with `telegram-live` and no webhook env configured | Send `/task list` or `/help` to the bot while polling is running | Confirm `sendMessage` replies in the originating chat | Matching Telegram notification sends plain text to the configured chat id | `scripts/smoke/fixtures/telegram.json` |
| Discord | Bridge starts with `discord-live`, syncs commands, and listens on `/interactions` | Trigger `/agent` or `/help` from a guild or DM | Confirm the deferred ack is immediate and the follow-up message arrives after command execution | Matching Discord notification sends a channel message using the bot token | `scripts/smoke/fixtures/discord.json` |

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

The same script works for `feishu`, `slack`, `dingtalk`, and `discord` by switching the `-Platform` value.
