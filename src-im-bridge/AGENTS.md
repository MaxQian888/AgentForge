# src-im-bridge/AGENTS.md

## IM Bridge Guidelines

### Structure & Organization

- Go service with multi-provider supervision
- `cmd/bridge/` entry point
- Platform adapters in `platform/` (feishu, dingtalk, slack, telegram, discord, wecom, qq, qqbot)

### Commands

```bash
go test ./...
go run ./cmd/bridge
go build ./cmd/bridge
```

### Code Style

- Go conventions: lowercase packages, PascalCase exports
- Platform adapters implement a common interface; add new providers by conforming to the adapter contract
- Error wrapping with context: `fmt.Errorf("platform %s: %w", name, err)`

### Testing

- Mock platform APIs for adapter tests
- Test message routing and dispatch logic independently of platform adapters
- Coverage for reaction handling and control plane operations

### Provider Rules

- Capability matrix must truthfully advertise provider support
- Rich delivery (attachments, reactions, thread-policy) is first-class
- Hot-reload of provider configuration is supported; handle config changes gracefully

### Commit Style

- Conventional Commits: `feat:`, `fix:`, `refactor:`, `chore:`, `docs:`
