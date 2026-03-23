# IM Bridge

`src-im-bridge` is the Go IM Bridge that connects one active IM platform instance to the shared AgentForge command engine and backend API.

## Platform Selection

Set `IM_PLATFORM` to exactly one of:

- `feishu`
- `slack`
- `dingtalk`

The bridge validates credentials for the selected platform before startup:

- `feishu`: optional `FEISHU_APP_ID` and `FEISHU_APP_SECRET`
- `slack`: required `SLACK_BOT_TOKEN` and `SLACK_APP_TOKEN`
- `dingtalk`: required `DINGTALK_APP_KEY` and `DINGTALK_APP_SECRET`

Example:

```powershell
$env:IM_PLATFORM = "slack"
$env:SLACK_BOT_TOKEN = "xoxb-..."
$env:SLACK_APP_TOKEN = "xapp-..."
go run .\cmd\bridge
```

## Current Adapter Mode

The current implementation ships local stub adapters for all three platforms so the command and notification flows can be verified without a live third-party connection.

- `feishu`: local stub adapter with text replies and card support
- `slack`: local stub adapter with text replies
- `dingtalk`: local stub adapter with text replies

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

Notifications received on `POST /im/notify` must include a `platform` field matching the active bridge platform:

- matching platform + `CardSender` support: send structured card
- matching platform without `CardSender`: fall back to plain text
- mismatched platform: reject the notification request

## Local Verification

Run the IM bridge test suite from the package root:

```powershell
cd src-im-bridge
go test ./...
```
