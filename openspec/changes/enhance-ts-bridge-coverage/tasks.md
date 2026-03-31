# Implementation Tasks

## 1. Go Backend Bridge Proxy Endpoints

- [x] 1.1 Add `GET /api/v1/bridge/pool` endpoint in `internal/handlers/bridge.go` that proxies to `GET http://localhost:7778/bridge/pool`
- [ ] 1.2 Add `GET /api/v1/bridge/health` endpoint in `internal/handlers/bridge.go` that proxies to `GET http://localhost:7778/bridge/health`
- [x] 1.3 Add `GET /api/v1/bridge/tools` endpoint in `internal/handlers/bridge.go` that proxies to `GET http://localhost:7778/bridge/tools`
- [x] 1.4 Add `POST /api/v1/bridge/tools/install` endpoint in `internal/handlers/bridge.go` that proxies to `POST http://localhost:7778/bridge/tools/install`
 with manifest URL validation
- [x] 1.5 Add `POST /api/v1/bridge/tools/uninstall` endpoint in `internal/handlers/bridge.go` that proxies to `POST http://localhost:7778/bridge/tools/uninstall` with plugin ID validation
- [x] 1.6 Add `POST /api/v1/bridge/tools/:id/restart` endpoint in `internal/handlers/bridge.go` that proxies to `POST http://localhost:7778/bridge/tools/:id/restart`
 with plugin ID validation
- [x] 1.7 Ensure `POST /api/v1/ai/decompose` endpoint supports full Bridge parameters (provider, model, context) in `internal/handlers/ai.go`
- [x] 1.8 Add Bridge client methods in `internal/bridge/client.go`: `GetPool()`, `GetHealth()`, `ListTools()`, `InstallTool()`, `UninstallTool()`, `RestartTool()`
- [ ] 1.9 Write integration tests for all new Go proxy endpoints in `internal/handlers/bridge_test.go`

## 2. IM Bridge Command Enhancements

- [x] 2.1 Update `/task decompose` command handler in `src-im-bridge/commands/task.go` to call Bridge `/bridge/decompose` with provider/model parameter support
- [x] 2.2 Add fallback logic in `/task decompose` to use Go API if Bridge is unavailable
- [x] 2.3 Create `/task ai generate` subcommand handler in `src-im-bridge/commands/task.go` that calls `POST /api/v1/ai/generate`
- [x] 2.4 Create `/task ai classify` subcommand handler in `src-im-bridge/commands/task.go` that calls `POST /api/v1/ai/classify-intent`
- [x] 2.5 Enhance `/agent list` command in `src-im-bridge/commands/agent.go` to fetch and display Bridge pool status alongside Go agent pool
- [x] 2.6 Create `/agent runtimes` subcommand in `src-im-bridge/commands/agent.go` that calls `GET /api/v1/bridge/runtimes`
- [x] 2.7 Create `/agent health` subcommand in `src-im-bridge/commands/agent.go` that calls `GET /api/v1/bridge/health`
- [ ] 2.8 Add Bridge capability checking logic in IM Bridge core engine to determine routing (Bridge vs Go API)

## 3. IM Plugin Management Commands

- [x] 3.1 Create new file `src-im-bridge/commands/tools.go` for `/tools` command group
- [x] 3.2 Implement `/tools list` command that calls `GET /api/v1/bridge/tools` and displays formatted plugin list
- [x] 3.3 Implement `/tools install <manifest-url>` command with admin role check that calls `POST /api/v1/bridge/tools/install`
- [x] 3.4 Implement `/tools uninstall <plugin-id>` command with admin role check that calls `POST /api/v1/bridge/tools/uninstall`
- [x] 3.5 Implement `/tools restart <plugin-id>` command that calls `POST /api/v1/bridge/tools/:id/restart`
- [ ] 3.6 Add manifest URL validation against allowlist in `/tools install` handler
- [x] 3.7 Write unit tests for all `/tools` commands in `src-im-bridge/commands/tools_test.go`

## 4. Bridge Event Forwarding

- [ ] 4.1 Configure TS Bridge to forward `permission_request` events to Go backend via WebSocket in `src-bridge/src/handlers/events.ts`
- [ ] 4.2 Configure TS Bridge to forward `budget_alert` events to Go backend in `src-bridge/src/handlers/events.ts`
- [ ] 4.3 Configure TS Bridge to forward `agent_status_change` events to Go backend in `src-bridge/src/handlers/events.ts`
- [ ] 4.4 Implement Go WebSocket handler for `bridge_event` type in `internal/websocket/bridge_handler.go`
- [ ] 4.5 Connect Go WebSocket handler to existing IM notification infrastructure in `internal/im/notification_manager.go`
- [ ] 4.6 Add event filtering based on user preferences in Go backend (enabled/disabled per event type)
- [ ] 4.7 Implement event ordering preservation in Go WebSocket handler
- [ ] 4.8 Write end-to-end tests for Bridge → Go → IM event flow

## 5. Natural Language Command Routing

- [ ] 5.1 Enhance IM Bridge `@AgentForge` handler to call `POST /api/v1/ai/classify-intent` with user message and candidate intents
- [ ] 5.2 Implement candidate intent registry in IM Bridge core engine that maps intents to command handlers
- [ ] 5.3 Add fallback keyword matching logic in Go when Bridge is unavailable for intent classification
- [ ] 5.4 Implement confidence threshold checking (reject if confidence < 0.7, show disambiguation menu)
- [ ] 5.5 Create disambiguation menu UI for low-confidence classifications that lists top 3 candidate intents
- [ ] 5.6 Add context-aware classification support (include conversation history in classify-intent requests)
- [ ] 5.7 Write unit tests for natural language routing with various intent scenarios

## 6. Cross-Feature Integration

- [ ] 6.1 Implement task → agent handoff: After `/task decompose` creates subtasks, prompt user to spawn agents for them
- [ ] 6.2 Add available tools display: When agent spawns, show which Bridge tools/plugins are available for that runtime
- [ ] 6.3 Implement review → task linking: When `/review` completes, offer to create follow-up tasks from review findings
- [ ] 6.4 Add multi-step workflow support for natural language commands that trigger multiple actions (e.g., "review PR and create tasks for issues")
- [ ] 6.5 Write integration tests for all cross-feature workflows

## 7. Agent Spawn Integration

- [ ] 7.1 Add Bridge pool capacity check before agent spawn in `internal/services/agent_service.go`
- [ ] 7.2 Display warning with options when Bridge pool is at capacity (Wait in queue / Proceed anyway)
- [ ] 7.3 Add Bridge health check before agent spawn execution
- [ ] 7.4 Implement retry logic when Bridge health check fails during spawn
- [ ] 7.5 Write tests for agent spawn with Bridge integration

## 8. Documentation and Monitoring

- [ ] 8.1 Update API documentation with new `/api/v1/bridge/*` endpoints
- [ ] 8.2 Add IM command reference for new `/tools` commands
- [ ] 8.3 Document enhanced `/agent` and `/task` commands with examples
- [ ] 8.4 Create monitoring dashboard for Bridge availability and command success rates
- [ ] 8.5 Add logging for all Bridge capability usage and fallback events
- [ ] 8.6 Document event forwarding configuration and user preference options

## 9. Testing and Quality Assurance

- [ ] 9.1 Write integration tests for Go backend proxy endpoints
- [ ] 9.2 Write unit tests for IM Bridge command handlers
- [ ] 9.3 Write end-to-end tests for Bridge event forwarding to IM platforms
- [ ] 9.4 Write tests for natural language routing with Bridge classify-intent
- [ ] 9.5 Write tests for cross-feature integration workflows
- [ ] 9.6 Perform manual testing on all IM platforms (Slack, Feishu, DingTalk, WeCom, QQ)
- [ ] 9.7 Verify backward compatibility of existing IM commands
- [ ] 9.8 Load testing for Bridge pool and health endpoints under concurrent spawn scenarios
