# im-ai-command-routing Specification

## Purpose
Define the classify-intent proxy and candidate intent registry that route natural-language IM commands through Bridge capabilities.
## Requirements
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
