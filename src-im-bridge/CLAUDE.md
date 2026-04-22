# src-im-bridge/CLAUDE.md

Standalone Go service for IM (Instant Messaging) provider connectivity.

## Overview

Multi-provider supervision, hot-reload, control plane, and inventory for IM platforms.

## Quick Commands

```bash
# Run
go run ./cmd/bridge

# Test
go test ./...

# Build
go build ./cmd/bridge
```

## Structure

| Package | Responsibility |
|---------|---------------|
| `cmd/bridge/` | Entry point |
| `client/` | AgentForge backend client, control plane, reaction handling |
| `commands/` | IM command parsers and executors |
| `core/` | Core message routing and dispatch |
| `platform/` | Platform adapters (feishu, dingtalk, slack, telegram, discord, wecom, qq, qqbot) |
| `notify/` | Notification dispatch |
| `audit/` | Audit event logging |

## Notes

- Default port: `7779` (when run as Tauri sidecar).
- Rich delivery (attachments/reactions/thread-policy) is first-class; capability matrix truthfully advertises provider support.
