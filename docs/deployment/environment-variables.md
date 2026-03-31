# Environment Variables / 环境变量参考

This file consolidates the environment variables that are actively used by the
current repository surfaces.

## Web / Next.js

| Variable | Required | Purpose | Notes |
| --- | --- | --- | --- |
| `NEXT_PUBLIC_API_URL` | Optional | Override the backend URL used by the frontend | Falls back to `http://localhost:7777` |
| `NEXT_PUBLIC_*` | Optional | Safe client-visible configuration | Do not expose secrets through this prefix |

## Go Orchestrator (`src-go`)

Loaded from `src-go/.env` and process env by `src-go/internal/config/config.go`.

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `PORT` | No | `7777` | HTTP listen port |
| `ENV` | No | `development` | Runtime mode |
| `POSTGRES_URL` | Yes for durable auth/data | none | PostgreSQL DSN |
| `REDIS_URL` | No | `redis://localhost:6379` | Redis DSN |
| `JWT_SECRET` | Yes in production | insecure dev fallback | JWT signing secret |
| `JWT_ACCESS_TTL` | No | `15m` | Access-token TTL |
| `JWT_REFRESH_TTL` | No | `168h` | Refresh-token TTL |
| `ALLOW_ORIGINS` | No | `http://localhost:3000,tauri://localhost,http://localhost:1420` | CORS allowlist |
| `BRIDGE_URL` | No | `http://localhost:7778` | TS bridge base URL |
| `AGENTFORGE_TOKEN` | Optional | empty | Trusted automation token for review ingress |
| `WORKTREE_BASE_PATH` | No | `./data/worktrees` | Managed worktree root |
| `REPO_BASE_PATH` | No | `./data/repos` | Managed repo root |
| `ROLES_DIR` | No | `./roles` | Role catalog root |
| `PLUGINS_DIR` | No | `./plugins` | Plugin manifest root |
| `PLUGIN_REGISTRY_URL` | Optional | empty | Remote plugin registry URL |
| `SCHEDULER_EXECUTION_MODE` | No | `in_process` | Scheduler execution mode |
| `MAX_ACTIVE_AGENTS` | No | `20` | Agent pool cap |
| `DEFAULT_TASK_BUDGET` | No | `5.0` | Default task budget |
| `TASK_PROGRESS_WARNING_AFTER` | No | `2h` | Progress warning threshold |
| `TASK_PROGRESS_STALLED_AFTER` | No | `4h` | Progress stalled threshold |
| `TASK_PROGRESS_ALERT_COOLDOWN` | No | `30m` | Alert cooldown |
| `TASK_PROGRESS_DETECTOR_INTERVAL` | No | `1m` | Detector polling interval |
| `TASK_PROGRESS_EXEMPT_STATUSES` | No | `blocked,done,cancelled` | Progress exemption list |
| `IM_NOTIFY_URL` | Optional | empty | IM notification endpoint |
| `IM_NOTIFY_PLATFORM` | Optional | empty | Default IM platform |
| `IM_NOTIFY_TARGET_CHAT_ID` | Optional | empty | Target chat/channel for notifications |
| `IM_CONTROL_SHARED_SECRET` | Optional | empty | IM bridge delivery/auth secret |
| `IM_BRIDGE_HEARTBEAT_TTL` | No | `2m` | Bridge heartbeat TTL |
| `IM_BRIDGE_PROGRESS_INTERVAL` | No | `30s` | IM progress heartbeat interval |

## TS Bridge (`src-bridge/.env.example`)

| Variable | Required | Purpose |
| --- | --- | --- |
| `PORT` | No | Bridge HTTP port, example `7778` |
| `GO_WS_URL` | Yes | Go WebSocket endpoint, usually `ws://localhost:7777/ws/bridge` |
| `ANTHROPIC_API_KEY` | Required for Claude runtime | Claude-backed execution |
| `ANTHROPIC_AUTH_TOKEN` | Optional | Supplemental Anthropic auth token |
| `CLAUDE_CODE_RUNTIME_MODEL` | Optional | Default Claude model |
| `OPENAI_API_KEY` | Required for OpenAI-backed bridge helpers | OpenAI APIs |
| `GOOGLE_GENERATIVE_AI_API_KEY` | Optional | Google Generative AI support |
| `CODEX_RUNTIME_COMMAND` | Required for Codex runtime | Codex CLI command/path |
| `CODEX_RUNTIME_MODEL` | Optional | Default Codex model |
| `OPENCODE_RUNTIME_COMMAND` | Required for OpenCode runtime | OpenCode command/path |
| `OPENCODE_RUNTIME_MODEL` | Optional | Default OpenCode model |
| `MAX_CONCURRENT_AGENTS` | No | Bridge concurrency cap |
| `LOG_LEVEL` | No | Runtime log verbosity |

## IM Bridge (`src-im-bridge/.env.example`)

| Variable | Required | Purpose |
| --- | --- | --- |
| `AGENTFORGE_API_BASE` | Yes | Go API base URL |
| `AGENTFORGE_PROJECT_ID` | Yes | Project scope for the bridge |
| `AGENTFORGE_API_KEY` | Yes | API key used by the IM bridge |
| `IM_PLATFORM` | Yes | Active IM platform (`feishu`, `slack`, etc.) |
| `IM_TRANSPORT_MODE` | No | Transport adapter mode |
| `IM_BRIDGE_ID_FILE` | No | Bridge instance-id persistence file |
| `IM_CONTROL_SHARED_SECRET` | Optional | Shared secret for control-plane calls |
| `IM_BRIDGE_HEARTBEAT_INTERVAL` | No | Heartbeat interval |
| `IM_BRIDGE_RECONNECT_DELAY` | No | Reconnect backoff |
| `NOTIFY_PORT` | No | Shared notify listener port |
| `TEST_PORT` | No | Shared test port |

Platform-specific examples already present:

- Feishu: `FEISHU_APP_ID`, `FEISHU_APP_SECRET`
- Slack: `SLACK_BOT_TOKEN`, `SLACK_APP_TOKEN`
- DingTalk: `DINGTALK_APP_KEY`, `DINGTALK_APP_SECRET`
- WeCom: `WECOM_CORP_ID`, `WECOM_AGENT_ID`, `WECOM_AGENT_SECRET`, `WECOM_CALLBACK_*`
- QQ / OneBot: `QQ_ONEBOT_WS_URL`, `QQ_ACCESS_TOKEN`
- QQ Bot Official: `QQBOT_*`
