# IM Bridge Comprehensive Improvement Design

**Date:** 2026-04-23
**Status:** Approved
**Approach:** Layered progressive (Plan C)

## Background

Full audit of the IM Bridge (`src-im-bridge/`) via 10 parallel research agents covering: entry point, core routing, all 9 platform adapters, command system, client/backend integration, notify system, audit system, test coverage, config/deployment, backend integration, and frontend surfaces.

### Current State Summary

| Layer | Files | Coverage | Assessment |
|-------|-------|----------|------------|
| `cmd/bridge/` entry | 15 | 51% | Multi-provider supervision, hot-reload, signal handling |
| `core/` message routing | ~20 | 76% | Rate limiting, multi-tenant resolution, action classification |
| `platform/` adapters | ~80 | 65-77% | 9 platforms with live+stub implementations |
| `commands/` system | ~30 | 69% | 12 top-level commands, intent recognition, runtime mentions |
| `notify/` dispatch | ~15 | 65% | HMAC signing, dedup, degradation chain |
| `audit/` logging | ~6 | 81% | JSONL, PII hashing, rotation, retention |
| `client/` backend | ~12 | 62% | API client, control plane, heartbeat |
| Backend integration | ~15 | - | Dual-channel (HTTP+WS), delivery signing, event bus |
| Frontend UI | ~15 | - | Channel management, bridge health, message history |

**Total test cases:** 930+, **average coverage:** ~68%

### Platform Completeness

| Platform | Completeness | Cards | Threads | Async Update | Callback |
|----------|-------------|-------|---------|--------------|----------|
| Feishu | 100% | Template+JSON | Yes | Deferred card update | Yes |
| Discord | 95% | Embed+Components | Yes | Deferred update (15min) | Yes |
| Slack | 90% | BlockKit | Yes | Message edit | Yes |
| DingTalk | 85% | ActionCard | No | No | Stream |
| Telegram | 80% | InlineKeyboard | No | Message edit | Yes |
| WeCom | 75% | TemplateCard | No | No | Yes |
| QQ Bot | 50% | Keyboard | No | No | Yes |
| QQ | 40% | None | No | No | No |
| Email | 30% | None | No | No | No |

### Discovered Issues

**High Priority:**
1. Email platform feature-poor (30% completeness)
2. QQ/QQ Bot experience poor — no card updates, no async progress feedback
3. DingTalk has no card update API — task progress cannot refresh in-place
4. Test coverage gaps — `cmd/bridge` only 51%, no CI/CD pipeline, no race detection
5. Frontend has no real-time WebSocket updates — relies on 5s polling

**Medium Priority:**
6. Bridge lifecycle management has no UI — cannot operate bridges from frontend
7. Tenant mount management has no UI — YAML-only configuration
8. Command plugin management has no UI — read-only display
9. No performance benchmarks — throughput/latency baselines unknown
10. WeCom/WeChat adapter test coverage low — 55% and 51%

**Low Priority:**
11. Email IMAP inbound is stub only — no real mail receiving
12. WeChat platform adapter — lowest coverage (55%), limited functionality
13. No distributed tracing integration — has tracectx but no OpenTelemetry export
14. Hot-reload Unix only — Windows requires restart

---

## Phase 1: Critical Test Net + CI/CD Skeleton (1 week)

### 1.1 cmd/bridge Entry Test Hardening

**Target:** 51% → 70%+

- `main_test.go` — Add multi-provider concurrent start/stop tests, missing env var scenarios, signal handling
- `hotreload_test.go` (new) — Simulate SIGHUP verifying provider credential reload, tenant config hot-update
- `control_plane_test.go` — Add WebSocket disconnect/reconnect, cursor replay, heartbeat timeout scenarios
- `inventory_test.go` — Add multi-tenant manifest construction, plugin inventory reporting

### 1.2 golangci-lint + Race Detection

- New `.golangci.yml` enabling: `errcheck`, `govet`, `staticcheck`, `gosec`, `ineffassign`, `unconvert`
- CI runs `go test -race ./...`
- Fix existing lint warnings (estimated 20-40 items)

### 1.3 GitHub Actions Base Pipeline

```yaml
# .github/workflows/im-bridge-ci.yml
on: [push, pull_request]
paths: ['src-im-bridge/**']
jobs:
  test:
    - go test -race -coverprofile=coverage.out ./...
    - coverage threshold gate (65%)
  lint:
    - golangci-lint run ./...
  build:
    - go build ./cmd/bridge
```

---

## Phase 2A: Platform Alignment (1.5 weeks, parallel with 2B)

### 2.1 Email Platform (30% → 70%)

- **IMAP real-time inbound** — IMAP IDLE mode listening for new emails, parse `In-Reply-To` for session correlation
- **Email command parsing** — Recognize `/task list` etc. in email body, route to command engine
- **HTML card rendering** — `ProviderNeutralCard` → HTML table layout (title+fields+button links), plain text fallback
- **Attachment support** — Receive email attachments, stage to `IM_BRIDGE_ATTACHMENT_DIR`
- **Thread model** — `In-Reply-To` + `References` headers for email thread tracking
- **Capability matrix** — Declared as `text_first` tier; no reactions/threads/callbacks

### 2.2 QQ OneBot (40% → 70%)

- **CQ code rich text** — Forward message nodes (`[CQ:node]`) for card-like effects
- **Command parsing** — `/` prefix routes to command engine, `@bot` triggers intent recognition
- **User info** — `get_stranger_info` / `get_group_member_info` for sender resolution
- **Image/file** — CQ code `[CQ:image]` / `[CQ:file]` for attachment delivery
- **Capability matrix** — Declared as `text_first`; no card updates

### 2.3 QQ Bot Official (50% → 75%)

- **Keyboard enhancement** — Multi-row button layout, URL + callback hybrid buttons
- **Markdown rendering** — `ProviderNeutralCard` → QQ Bot markdown (headings, field lists, dividers)
- **Message edit** — Use `PATCH /messages/{id}` API if platform supports for progress updates
- **Group message threading** — Sequence numbers for simple thread tracking
- **Capability matrix** — Declared as `markdown_first`

### Out of Scope for Phase 2

- **DingTalk** (85%) — sufficient, card updates blocked by platform API, explore in Phase 3
- **WeCom** (75%) — sufficient, same as above

---

## Phase 2B: Ops Experience Upgrade (1.5 weeks, parallel with 2A)

### 2.4 Frontend WebSocket Real-time Updates

- **Connection layer** — `im-store.ts` adds WebSocket connection to backend dashboard observer endpoint
- **Event types** — Listen for `bridge.status_changed`, `delivery.created`, `delivery.acked`, `delivery.failed`
- **State sync** — Events update store directly; remove polling timer, keep HTTP fetch for initial load
- **Degradation** — WS disconnect falls back to polling (exponential backoff, max 30s); WS recovery switches back to push
- **Backend** — `src-go/internal/ws/` adds `IMDashboardHandler` broadcasting bridge state change events

### 2.5 Bridge Lifecycle Management UI

- **Bridge detail drawer** — Click bridge card to expand: providers, capability matrix, recent heartbeats, active tenants
- **Diagnostic actions** — "Send test message", "Retry failed deliveries", "Clear queue" buttons
- **Heartbeat timeline** — Real-time display of last N heartbeats with latency
- **Excludes** start/stop bridge (bridge is independent process, frontend should not control lifecycle)

### 2.6 Tenant Mount Management UI

- **Tenant list page** — All registered tenants, associated projects, resolver rules
- **Tenant CRUD** — Create/edit/delete tenants, configure resolvers (chat ID, workspace ID, domain)
- **Credential binding** — Display per-tenant provider credential prefixes
- **Backend support** — `src-go` needs new tenant config CRUD API (currently tenant config lives bridge-side only)

### 2.7 Command Plugin Management UI

- **Plugin list** — Registered plugin ID, version, command list, associated tenants
- **Plugin detail** — Command signatures, manifest YAML preview, status
- **Excludes** online install/uninstall (plugins are filesystem-level, keep CLI operation)

### 2.8 i18n

- All new UI component translation keys (EN + ZH-CN)
- New `im-ops` translation namespace for ops-related strings

---

## Phase 3: Long-tail + Polish (1.5 weeks)

### 3.1 DingTalk/WeCom Async Update Exploration

- **DingTalk** — Explore interactive card update API (requires enterprise certification); fallback: send new message quoting old
- **WeCom** — Explore "update application message" API; same fallback
- **Generic fallback** — `append_reply` mode for platforms without update API: send new reply with `[Update]` prefix and timestamp

### 3.2 Performance Benchmarks

New `bench/` directory:
- Message parsing throughput — benchmark `Engine.HandleMessage()` P50/P99 latency
- Notification delivery chain — benchmark `notify.Receiver` full pipeline
- Audit write — benchmark JSONL append under concurrent 16/64/256 goroutines
- SQLite state store — benchmark dedupe query, rate-limit counters at 10k/100k entries

### 3.3 Distributed Tracing Integration

- `notify/middleware_trace.go` extension — extract/inject `X-Trace-ID` from request headers
- Audit event correlation — bind trace ID to each audit log entry
- Optional OpenTelemetry export — export spans when `OTEL_EXPORTER_OTLP_ENDPOINT` configured
- Control plane delivery tracing — full backend-to-bridge delivery chain trace

### 3.4 Windows Hot-Reload Alternative

- Watch `IM_BRIDGE_CONFIG_FILE` (new env var) for file changes
- Use `fsnotify` for cross-platform file watching replacing SIGHUP
- Trigger same reload logic as Unix SIGHUP: credential refresh, tenant reconfiguration, bridge re-registration

### 3.5 Final Polish

- All platform test coverage to 70%+
- README and CLAUDE.md updates reflecting new features
- Platform Runbook additions for new platform usage
- `.env.example` updates for new environment variables

---

## Timeline

| Phase | Duration | Content |
|-------|----------|---------|
| Phase 1 | 1 week | Test net + CI/CD + lint |
| Phase 2A | 1.5 weeks | Email/QQ/QQBot platform alignment |
| Phase 2B | 1.5 weeks | Frontend real-time + management UI |
| Phase 3 | 1.5 weeks | Long-tail + performance + tracing |
| **Total** | **~6 weeks** | |

Phase 2A and 2B run in parallel, actual wall-clock time: **~4-5 weeks**.

---

## Risks and Mitigations

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| QQ Bot message edit API unavailable | Medium | Low | Use `append_reply` fallback |
| DingTalk interactive card API requires enterprise cert | High | Medium | `append_reply` fallback sufficient for most use cases |
| Backend tenant CRUD API scope creep | Medium | Medium | Scope to minimal CRUD only, defer advanced features |
| WebSocket dashboard endpoint performance under load | Low | Medium | Rate-limit event emission, batch updates |
| fsnotify Windows compatibility edge cases | Low | Low | Fallback to manual file polling at 5s interval |

---

## Success Criteria

1. All platform adapters at 70%+ test coverage
2. Email/QQ/QQBot at 70%+ functional completeness
3. CI/CD pipeline running on all PRs with 65% coverage gate
4. Frontend shows real-time bridge status without polling
5. Tenant and plugin management accessible from UI
6. Performance benchmarks established for all hot paths
