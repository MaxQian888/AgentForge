# im-ai-command-routing Specification

## Purpose
Enable natural language `@AgentForge` mentions in IM platforms to use Bridge's classify-intent capability for intelligent command routing instead of keyword matching, providing better user experience and discoverability.

 and reducing the need for users to memorize exact command syntax.

## ADDED Requirements

his requirement: Natural language mentions use Bridge classify-intent
The IM Bridge SHALL use Bridge `/bridge/classify-intent` endpoint to route natural language `@AgentForge` mentions to appropriate commands based on intent classification and candidate intents.

 and confidence scores.

#### Scenario: Natural language routes to sprint status
- **WHEN** user sends `@AgentForge show me the sprint status` via IM platform
- **THEN** IM Bridge calls `POST /api/v1/ai/classify-intent` with `{ text: "show me the sprint status", candidates: ["sprint_status", "sprint_burndown", "task_list", "sprint_settings"]`
- **THEN** Bridge returns `{ intent: "sprint_status", confidence: 0.95 }`
- **THEN** IM Bridge routes to `/sprint status` command handler
- **THEN** IM Bridge executes the sprint status command and displays the result

- **THEN** IM Bridge replies to user with sprint status card

#### Scenario: Natural language routes to task creation
- **WHEN** user sends `@AgentForge create a task for title: Fix the login bug"` via IM platform
- **THEN** IM Bridge calls `POST /api/v1/ai/classify-intent` with `{ text: "create a task with title: Fix the login bug", candidates: ["create_task", "agent_spawn", "decompose_task"] }`
- **THEN** Bridge returns `{ intent: "create_task", confidence: 0.88 }`
- **THEN** IM Bridge routes to `/task create Fix the login bug` command handler
- **THEN** IM Bridge creates the task and displays confirmation with task ID

- **Then** IM Bridge replies with task creation confirmation

#### Scenario: Low confidence triggers disambiguation menu
- **WHEN** user sends `@AgentForge status report` via IM platform
- **THEN** Bridge returns `{ intent: "status_report", confidence: 0.45 }`
- **THEN** IM Bridge detects low confidence
- **THEN** IM Bridge displays disambiguation menu:
  1. "Generate status report"
 (generates formatted status report)
  2. "Show task list" (lists tasks)
  3. "Something else..." (offers to rephrase)
- **THEN** IM Bridge prompts user to select or option
- **THEN** user selects "Show task list"
 or types "Generate status report" or number
- **THEN** IM Bridge executes the selected command and displays the result

- **Then** IM Bridge replies with the result
- **THEN** IM Bridge logs the classification result for analytics

 #### Scenario: Bridge unavailable uses keyword fallback
- **WHEN** user sends `@AgentForge create a task` and Bridge is unavailable
- **THEN** IM Bridge detects Bridge unavailability
- **THEN** IM Bridge falls back to keyword matching in Go
- **THEN** Go keyword matching identifies intent as "create_task" with moderate confidence
- **THEN** IM Bridge routes to `/task create` command handler
- **THEN** IM Bridge displays result with note "Using fallback matching (Bridge unavailable)"
 - **Then** IM Bridge logs the fallback event for monitoring

#### Scenario: Natural language spawns agent with context
- **WHEN** user sends `@AgentForge spawn an agent to fix the bug in task task-123` via IM platform
- **THEN** Bridge classifies intent as `agent_spawn` with high confidence (0.92)
- **THEN** IM Bridge routes to `/agent spawn task-123` command handler
- **THEN** IM Bridge spawns agent and displays confirmation with agent run ID
- **Then** IM Bridge replies with agent spawn confirmation and link to agent run details

- **Then** IM Bridge logs the classification for analytics

 #### Scenario: Context-aware intent classification
- **WHEN** user sends `@AgentForge decompose the authentication feature into smaller tasks` in context of recent discussion about authentication
- **THEN** IM Bridge includes conversation context in classify-intent request
 `{ text: "decompose the authentication feature into smaller tasks", context: "..." }`
- **THEN** Bridge returns `{ intent: "decompose_task", confidence: 0.87 }`
- **THEN** IM Bridge routes to `/task decompose task-123` command handler with context
- **THEN** IM Bridge executes decomposition and displays subtasks
- **Then** IM Bridge replies with decomposition results and context
- **Then** IM Bridge logs that context was classification success

 #### Scenario: Multi-step workflow via natural language
- **WHEN** user sends `@AgentForge review the PR and create follow-up tasks for the fixes` via IM platform
- **THEN** Bridge classifies as `review` intent
- **THEN** IM Bridge routes to `/review <pr-url>` command handler
- **THEN** IM Bridge starts code review and- **WHEN** review completes, Bridge classifies next intent as `create_followup_tasks`
 with high confidence
- **THEN** IM Bridge routes to `/task create` for each issue from review findings
- **Then** IM Bridge creates follow-up tasks and displays summary
- **THEN** IM Bridge replies with review completion and follow-up task links
 - **THEN** IM Bridge logs the multi-step workflow completion

### Requirement: Go backend exposes classify-intent proxy endpoint
The Go backend SHALL expose `POST /api/v1/ai/classify-intent` endpoint that proxies to Bridge `/bridge/classify-intent` with authentication and request validation.

#### Scenario: Classify-intent endpoint proxies to Bridge
- **WHEN** authenticated client calls `POST /api/v1/ai/classify-intent` with `{ text: "show sprint status", candidates: ["sprint_status", "task_list", "agent_spawn"] }`
- **THEN** Go backend validates authentication
- **THEN** Go backend validates request body
- **THEN** Go backend forwards to `POST http://localhost:7778/bridge/classify-intent` with same payload
- **THEN** Go backend returns Bridge response to client

#### Scenario: Classify-intent with context
- **WHEN** authenticated client calls `POST /api/v1/ai/classify-intent` with `{ text: "decompose this task", candidates: ["decompose_task"], context: "This task has authentication issues" }`
- **THEN** Bridge uses context to improve classification accuracy
- **THEN** Go backend returns enhanced classification result with reasoning

#### Scenario: Classify-intent with custom candidates
- **WHEN** authenticated client calls `POST /api/v1/ai/classify-intent` with custom candidate list
- **THEN** Go backend forwards request to Bridge as-is
- **THEN** Bridge returns classification with confidence score for each candidate
- **THEN** Go backend returns result to client

#### Scenario: Unauthenticated request rejected
- **WHEN** unauthenticated client calls `POST /api/v1/ai/classify-intent`
- **THEN** Go backend returns 401 Unauthorized
- **THEN** response includes `WWW-Authenticate` header challenge

#### Scenario: Invalid request body rejected
- **WHEN** authenticated client calls `POST /api/v1/ai/classify-intent` without required fields
- **THEN** Go backend returns 400 Bad Request
- **THEN** response includes validation errors

### Requirement: IM Bridge maintains candidate intent registry
The IM Bridge SHALL maintain a registry of candidate intents mapped to command handlers, allowing Bridge to classify user input against known commands.

#### Scenario: Registry includes standard commands
- **WHEN** IM Bridge initializes
- **THEN** registry includes intents for:
  - `create_task`: routes to `/task create`
  - `sprint_status`: routes to `/sprint status`
  - `sprint_burndown`: routes to `/sprint burndown`
  - `agent_spawn`: routes to `/agent spawn`
  - `decompose_task`: routes to `/task decompose`
  - `review`: routes to `/review`
  - `task_list`: routes to `/task list`
  - `status_report`: routes to `/cost` (or custom report command)

  - `unknown`: triggers disambiguation menu

#### Scenario: Registry supports extensibility
- **WHEN** new command `/workflow` is registered in IM Bridge
- **THEN** registry is updated with `{ intent: "workflow", handler: workflowHandler }`
- **THEN** Bridge can classify and route to the new workflow command

#### Scenario: Registry provides disambiguation candidates
- **WHEN** Bridge classifies user input
- **THEN** candidate list is built from registry keys
- **THEN** candidates include all registered intent names
