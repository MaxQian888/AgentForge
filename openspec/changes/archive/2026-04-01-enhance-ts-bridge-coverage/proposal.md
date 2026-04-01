# TS Bridge Coverage Enhancement Proposal

## Why

The current TS Bridge exposes a rich set of execution, control, and AI capabilities through canonical `/bridge/*` routes, but the IM Bridge (Go) command layer doesn't fully leverage these features. IM commands like `/task`, `/agent`, and `/review` currently call the Go backend API directly, missing opportunities to use TS Bridge capabilities for:

1. **AI-powered task decomposition** (`/task decompose`) - currently calls Go API, could use Bridge's `/bridge/decompose` with provider/model selection
2. **Intent classification for natural language commands** - IM Bridge could use `/bridge/classify-intent` to route `@AgentForge` mentions to appropriate actions
3. **Agent pool and runtime status integration** - `/agent list` could show Bridge runtime pool status via `/bridge/pool` and `/bridge/runtimes`
4. **Tool/plugin management from IM** - no IM commands for managing TS Bridge plugins via `/bridge/tools/*` routes
5. **Workflow and team coordination** - specs exist for team handoff and workflow orchestration but no IM command surface

This creates a gap where:
- Users can't access powerful Bridge features from IM platforms (Slack, Feishu, etc.)
- Go orchestrator becomes a bottleneck for features already implemented in TS Bridge
- Feature completeness is fragmented across two systems with unclear boundaries

**Why now**: The recent completion of bridge HTTP contract, cross-runtime extensions, and plugin management specs provides a stable foundation. Completing the IM-to-Bridge integration now prevents further drift and ensures the dual-runtime model (Go + TS) feels cohesive to users.

## What Changes

### 1. IM Bridge command expansion to use TS Bridge capabilities

- **Enhance `/task decompose`**: Call TS Bridge `/bridge/decompose` instead of Go API directly, enabling provider/model selection and richer decomposition context
- **New `/task ai` subcommand**: Direct access to Bridge AI capabilities (`/task ai generate`, `/task ai classify`)
- **Enhance `/agent` commands**:
  - `/agent list` to include Bridge runtime pool status (`/bridge/pool`)
  - `/agent runtimes` to show available Bridge runtimes (`/bridge/runtimes`)
  - `/agent health` to check Bridge health (`/bridge/health`)
- **New `/tools` command group** for Bridge plugin management:
  - `/tools list` - list installed Bridge tools/plugins
  - `/tools install <manifest-url>` - install a Bridge plugin
  - `/tools uninstall <plugin-id>` - remove a plugin
  - `/tools restart <plugin-id>` - restart a plugin
- **Enhance `@AgentForge` natural language handling**: Use `/bridge/classify-intent` to route to appropriate commands

### 2. Go backend proxy endpoints for Bridge features

- **New `/api/v1/bridge/pool` endpoint**: Proxy to Bridge `/bridge/pool` for agent pool status
- **New `/api/v1/bridge/health` endpoint**: Proxy to Bridge `/bridge/health` for health checks
- **New `/api/v1/bridge/tools/*` endpoints**: Proxy to Bridge plugin management routes
- **Enhanced `/api/v1/ai/decompose` endpoint**: Already exists, ensure it uses Bridge `/bridge/decompose` with full parameter support

### 3. Bidirectional event streaming integration

- **Bridge → IM event forwarding**: Configure Bridge to forward significant events (agent status changes, permission requests, budget alerts) to IM platforms via Go backend
- **IM → Bridge command routing**: Ensure all IM commands that should use Bridge capabilities are routed through Bridge, not just Go API

### 4. Cross-feature invocation completeness

- **Task → Agent handoff**: When `/task decompose` creates subtasks, automatically offer to spawn agents for them via Bridge execution
- **Agent → Tool integration**: When agents are spawned, show which Bridge tools/plugins are available for that runtime
- **Review → Task linking**: When `/review` completes, offer to create follow-up tasks from review findings

## Capabilities

### New Capabilities

- `im-bridge-ai-integration`: IM Bridge commands leverage TS Bridge AI capabilities (decompose, generate, classify-intent) for enhanced task management and natural language routing
- `im-plugin-management-commands`: IM command surface for managing TS Bridge plugins/tools (`/tools list/install/uninstall/restart`)
- `im-runtime-status-commands`: IM commands for querying Bridge runtime pool, health, and available runtimes (`/agent list/runtimes/health`)
- `bridge-event-im-forwarding`: Bridge forwards significant events (permission requests, budget alerts, status changes) to IM platforms through Go backend
- `im-ai-command-routing`: Natural language `@AgentForge` mentions use Bridge `/bridge/classify-intent` for intelligent command routing

### Modified Capabilities

- `im-bridge-control-plane`: Enhanced to route IM commands through appropriate backend layer (Bridge vs Go API) based on capability location, ensuring feature completeness
- `agent-spawn-orchestration`: Modified to include Bridge pool and runtime status checks during agent spawn operations

## Impact

### Code Changes

**Go Backend (src-go/)**:
- `internal/bridge/client.go`: Add methods for pool, health, tools/* endpoints
- `internal/handlers/bridge.go`: New proxy handlers for `/api/v1/bridge/pool`, `/api/v1/bridge/health`, `/api/v1/bridge/tools/*`
- `internal/handlers/ai.go`: Ensure `/api/v1/ai/decompose` uses full Bridge capabilities
- `internal/im/commands/`: Update task.go, agent.go to use Bridge client where appropriate
- New file: `internal/im/commands/tools.go` for `/tools` command group

**IM Bridge (src-im-bridge/)**:
- `commands/task.go`: Update `handleTaskDecompose` to use Bridge client
- `commands/agent.go`: Add subcommands for `runtimes`, `health`, enhance `list` with pool status
- New file: `commands/tools.go` for `/tools` command group
- `core/engine.go`: Enhance natural language routing with Bridge classify-intent
- `client/agentforge_client.go`: Add Bridge API methods (DecomposeViaBridge, GetPoolStatus, GetHealth, etc.)

**TypeScript Bridge (src-bridge/)**:
- `src/handlers/events.ts`: Add event forwarding to Go backend WebSocket for IM delivery
- Configuration for which events to forward (permission requests, budget alerts, etc.)

### API Surface

**New Go API Endpoints**:
- `GET /api/v1/bridge/pool` - Bridge runtime pool status
- `GET /api/v1/bridge/health` - Bridge health check
- `GET /api/v1/bridge/tools` - List Bridge plugins
- `POST /api/v1/bridge/tools/install` - Install Bridge plugin
- `POST /api/v1/bridge/tools/uninstall` - Uninstall Bridge plugin
- `POST /api/v1/bridge/tools/:id/restart` - Restart Bridge plugin

**Enhanced Go API Endpoints**:
- `POST /api/v1/ai/decompose` - Ensure full Bridge parameter support (provider, model, context)

**New IM Commands**:
- `/tools list` - List Bridge tools/plugins
- `/tools install <url>` - Install Bridge plugin
- `/tools uninstall <id>` - Uninstall plugin
- `/tools restart <id>` - Restart plugin
- `/agent runtimes` - Show available Bridge runtimes
- `/agent health` - Check Bridge health
- `/task ai generate <prompt>` - Generate text via Bridge
- `/task ai classify <text>` - Classify intent via Bridge

**Enhanced IM Commands**:
- `/task decompose <id>` - Now uses Bridge with provider/model selection
- `/agent list` - Shows Bridge pool status alongside Go agent pool
- `@AgentForge <message>` - Uses Bridge classify-intent for routing

### Dependencies

- Existing specs: `bridge-http-contract`, `im-bridge-control-plane`, `agent-spawn-orchestration`, `plugin-runtime`, `plugin-management-panel`
- No new external dependencies required

### Systems Affected

- **IM Platforms**: Users on Slack, Feishu, DingTalk, WeCom, QQ will have access to Bridge features
- **Desktop App**: Runtime dashboard can query IM commands for status
- **Monitoring**: Bridge health and pool status accessible from IM for ops
- **Plugin Ecosystem**: Plugin management accessible from chatOps workflows
