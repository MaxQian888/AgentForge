# TS Bridge Coverage Enhancement - Design Document

## Context

The AgentForge platform operates with a dual-runtime architecture:
- **Go Orchestrator**: Handles IM platform integration, task persistence, agent dispatching, and business logic
- **TypeScript Bridge**: Manages agent execution runtimes (Claude Code, Codex, OpenCode), AI capabilities, and plugin system

Currently, IM Bridge commands in Go call the Go API layer directly, which then orchestrates with TS Bridge for execution. However, several TS Bridge capabilities are not accessible from IM commands:

1. **AI Capabilities**: `/bridge/decompose`, `/bridge/generate`, `/bridge/classify-intent` are only used internally by Go backend
2. **Runtime Management**: `/bridge/pool`, `/bridge/runtimes`, `/bridge/health` are not exposed to IM users
3. **Plugin System**: `/bridge/tools/*` routes have no IM command surface
4. **Event Flow**: Bridge events don't flow back to IM platforms for visibility

This design addresses how to surface TS Bridge capabilities through IM commands while maintaining clear architectural boundaries.

### Current State

```
┌─────────────┐
│  IM Platform │
│ (Slack/Feishu)│
└──────┬──────┘
       │ /task, /agent commands
       ▼
┌─────────────────────┐
│   IM Bridge (Go)    │
│  - Command parsing  │
│  - Go API client    │
└──────┬──────────────┘
       │ HTTP to Go API
       ▼
┌─────────────────────┐
│  Go Backend API     │◄─── Some calls go to Bridge
│  - Business logic   │
│  - Persistence      │
└──────┬──────────────┘
       │ HTTP to /bridge/*
       ▼
┌─────────────────────┐
│  TS Bridge          │
│  - Agent runtimes   │
│  - AI capabilities  │
│  - Plugin mgmt      │
└─────────────────────┘
```

### Constraints

1. **Backward Compatibility**: Existing IM command syntax must remain functional
2. **No Duplicate Implementation**: If TS Bridge implements a feature, Go should proxy, not reimplement
3. **Clear Layer Boundaries**: Go handles auth/persistence, Bridge handles execution/AI
4. **Event Ordering**: IM notifications must maintain causal ordering (task created → agent spawned)

## Goals / Non-Goals

**Goals:**
- Expose TS Bridge AI capabilities through IM commands with full parameter support (provider/model selection)
- Surface runtime pool status and health through IM for operational visibility
- Provide IM command interface for Bridge plugin management
- Enable intelligent natural language routing using Bridge's classify-intent
- Establish bidirectional event flow: Bridge → Go → IM for significant events
- Ensure cross-feature invocation works end-to-end (task → agent → tools)

**Non-Goals:**
- Replacing Go orchestrator's role in auth, persistence, or business logic
- Moving all IM command handling to TypeScript
- Creating new Bridge capabilities (only surfacing existing ones)
- Modifying Bridge HTTP contract or WebSocket protocol
- Building IM-specific UI components (focus is on chat commands)

## Decisions

### 1. Routing Strategy: Go Backend Proxies Bridge Routes

**Decision**: Create Go API proxy endpoints that forward to TS Bridge, rather than having IM Bridge call TS Bridge directly.

**Rationale**:
- **Auth Consistency**: All external-facing API calls go through Go auth middleware
- **Audit Trail**: Go can log all Bridge operations for compliance
- **Rate Limiting**: Centralized rate limiting in Go backend
- **Circuit Breaking**: Go can implement fallbacks if Bridge is unavailable

**Alternatives Considered**:
- **IM Bridge → TS Bridge direct**: Would bypass auth/audit, create dual auth system
- **Move all logic to TS Bridge**: Violates architectural boundaries, loses Go's orchestration strengths

**Implementation**:
```go
// internal/handlers/bridge.go
func (h *Handler) GetBridgePool(c echo.Context) error {
    pool, err := h.bridgeClient.GetPool(c.Request().Context())
    if err != nil {
        return err
    }
    return c.JSON(200, pool)
}
```

### 2. Command Routing: Capability-Based Dispatch

**Decision**: IM commands check if a capability exists in Bridge before deciding whether to call Go API or Bridge proxy.

**Rationale**:
- **Extensibility**: New Bridge capabilities automatically surface in IM
- **Fail-Safe**: If Bridge is down, commands fall back to Go API where applicable
- **Clear Ownership**: Each capability has a single source of truth

**Pattern**:
```go
// In IM Bridge command handler
func handleTaskDecompose(ctx context.Context, p Platform, msg *Message, client *Client, taskID string) {
    // Try Bridge first
    if client.SupportsBridgeCapability("decompose") {
        result, err := client.DecomposeViaBridge(ctx, taskID)
        if err == nil {
            replyWithDecomposition(p, msg, result)
            return
        }
        // Log fallback
        log.Warn("Bridge decompose failed, falling back to Go API", "error", err)
    }
    // Fallback to Go API
    result, err := client.DecomposeViaGoAPI(ctx, taskID)
    // ...
}
```

### 3. Event Forwarding: Bridge → Go WebSocket → IM

**Decision**: TS Bridge forwards specific events to Go backend via WebSocket, Go backend then pushes to IM platforms through existing notification infrastructure.

**Events to Forward**:
- `permission_request`: User needs to approve/deny tool usage
- `budget_alert`: Approaching budget limit
- `agent_status_change`: Agent started/paused/completed/failed
- `tool_installed`: Plugin installation completed

**Rationale**:
- **Single WebSocket**: IM platforms already listen to Go WebSocket
- **Event Ordering**: Go can sequence events with other business events
- **Filtering**: Go can apply user preferences for which events to receive

**Alternatives Considered**:
- **Direct Bridge → IM WebSocket**: Would require IM platforms to maintain two WebSocket connections
- **Polling**: Higher latency, more resource intensive

**Implementation**:
```typescript
// In TS Bridge event handler
if (event.type === 'permission_request') {
  await forwardToGoBackend({
    type: 'bridge_event',
    event: event,
    target: runtime.task_id
  });
}
```

```go
// In Go WebSocket handler
case "bridge_event":
    // Forward to IM notification system
    h.notificationManager.SendToTaskWatchers(event.Target, event.Event)
```

### 4. Plugin Management: Full IM Command Surface

**Decision**: Implement complete `/tools` command group in IM Bridge for plugin management.

**Commands**:
- `/tools list` → `GET /api/v1/bridge/tools`
- `/tools install <manifest-url>` → `POST /api/v1/bridge/tools/install`
- `/tools uninstall <plugin-id>` → `POST /api/v1/bridge/tools/uninstall`
- `/tools restart <plugin-id>` → `POST /api/v1/bridge/tools/:id/restart`

**Rationale**:
- **ChatOps Workflow**: Ops teams can manage plugins from Slack/Feishu
- **Approval Flow**: Go backend can enforce approval workflows for plugin installation
- **Audit**: All plugin changes logged through Go API

**Security**:
- Plugin installation requires admin role
- Manifest URL validated against allowlist
- Plugin ID validated before uninstall/restart

### 5. Natural Language Routing: Bridge classify-intent

**Decision**: Use Bridge's `/bridge/classify-intent` for `@AgentForge` natural language mentions.

**Flow**:
1. User sends `@AgentForge show me the sprint status`
2. IM Bridge calls `POST /bridge/classify-intent` with text and candidate intents
3. Bridge returns `{ intent: "sprint_status", confidence: 0.92 }`
4. IM Bridge routes to `/sprint status` command handler

**Rationale**:
- **Unified AI**: Single AI model for intent classification (no Go implementation needed)
- **Consistency**: Same classification logic across all interfaces (desktop, IM, API)
- **Extensibility**: New intents added to Bridge automatically available in IM

**Fallback**:
- If Bridge is unavailable, fall back to keyword matching in Go
- If confidence < 0.7, show disambiguation menu

## Risks / Trade-offs

### Risk: Increased Bridge Dependency
**Impact**: IM commands fail when Bridge is down
**Mitigation**:
- Implement graceful degradation: `/agent health` shows "Bridge unavailable" instead of error
- Critical commands (task creation, assignment) use Go API directly, not Bridge
- Health check endpoint in Go backend reports Bridge availability

### Risk: Event Forwarding Latency
**Impact**: Users see delayed notifications for Bridge events
**Mitigation**:
- Use existing WebSocket infrastructure (low latency)
- Go backend prioritizes `bridge_event` type messages
- Monitor p95 latency for Bridge → Go → IM path

### Risk: Dual API Surface Confusion
**Impact**: Developers unclear whether to use `/api/v1/bridge/*` or `/bridge/*`
**Mitigation**:
- Clear documentation: `/api/v1/bridge/*` is for external clients (auth required), `/bridge/*` is internal (localhost only)
- All new code uses canonical routes per `bridge-http-contract` spec
- Go proxy endpoints delegate to Bridge, don't reimplement

### Trade-off: Go Backend as Proxy vs Direct Bridge Access
**Trade-off**: Extra network hop (IM → Go → Bridge) vs architectural clarity
**Chosen**: Proxy through Go for auth/audit/rate-limiting benefits
**Accepted Cost**: ~10-20ms additional latency per call, acceptable for chat interactions

### Trade-off: Command Fallback Complexity vs Simplicity
**Trade-off**: Each command needs fallback logic vs single path
**Chosen**: Implement fallbacks for resilience
**Accepted Cost**: More code in command handlers, but better UX when Bridge is degraded

## Migration Plan

### Phase 1: Go Backend Proxy Endpoints (Week 1)
1. Implement `GET /api/v1/bridge/pool`
2. Implement `GET /api/v1/bridge/health`
3. Implement `GET /api/v1/bridge/tools`
4. Implement `POST /api/v1/bridge/tools/install`
5. Implement `POST /api/v1/bridge/tools/uninstall`
6. Implement `POST /api/v1/bridge/tools/:id/restart`
7. Add integration tests for all proxy endpoints
8. **Rollback**: Remove routes, no data migration needed

### Phase 2: IM Bridge Command Enhancements (Week 2)
1. Add `/tools` command group with all subcommands
2. Enhance `/agent list` to show Bridge pool status
3. Add `/agent runtimes` command
4. Add `/agent health` command
5. Update `/task decompose` to use Bridge with fallback
6. Implement natural language routing with classify-intent
7. Add IM Bridge tests for new commands
8. **Rollback**: Revert to previous command handlers

### Phase 3: Event Forwarding (Week 3)
1. Configure Bridge to forward `permission_request`, `budget_alert`, `agent_status_change` events
2. Implement Go WebSocket handler for `bridge_event` type
3. Connect to existing IM notification infrastructure
4. Add end-to-end tests for event flow
5. **Rollback**: Disable event forwarding in Bridge config

### Phase 4: Cross-Feature Integration (Week 4)
1. Implement task → agent handoff after decomposition
2. Show available tools when agent spawns
3. Link review findings to task creation
4. Integration testing across all new features
5. **Rollback**: Feature flags for each integration point

### Rollback Strategy
- All phases are independently deployable and reversible
- No database schema changes (uses existing tables)
- Feature flags control new command behaviors
- Monitoring dashboard tracks Bridge availability and command success rates

## Open Questions

1. **Q: Should `/task ai generate` and `/task ai classify` be separate top-level commands instead?**
   - **Context**: `/task ai` groups AI utilities under task context, but could be `/generate` and `/classify` at top level
   - **Options**:
     - A) Keep under `/task ai` (proposed) - groups related commands
     - B) Make top-level `/generate` and `/classify` - more discoverable
   - **Recommendation**: Start with `/task ai`, gather user feedback, promote to top-level if heavily used

2. **Q: What's the approval workflow for `/tools install` in production?**
   - **Context**: Installing plugins in production could be risky
   - **Options**:
     - A) Admin-only (proposed) - requires admin role
     - B) Approval flow - create approval request, admin approves
     - C) Restricted environment - only allowed in dev/staging
   - **Recommendation**: Start with A (admin-only), add approval flow in future if needed

3. **Q: Should Bridge events be stored in Go database for audit?**
   - **Context**: Permission requests and budget alerts are significant events
   - **Options**:
     - A) Forward only (proposed) - no persistence
     - B) Store in event log table - full audit trail
   - **Recommendation**: Start with A, add persistence in future if compliance requires it

4. **Q: How to handle Bridge provider/model selection in IM commands?**
   - **Context**: `/task decompose` could specify provider/model, but IM UI is limited
   - **Options**:
     - A) Auto-select defaults (proposed) - use Bridge defaults
     - B) Optional params - `/task decompose <id> [provider] [model]`
     - C) Interactive prompt - ask user to select after command
   - **Recommendation**: Start with A, add B if users request control
