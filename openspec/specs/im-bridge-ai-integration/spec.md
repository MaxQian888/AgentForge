# im-bridge-ai-integration Specification

## Purpose
Define how IM Bridge exposes Bridge AI capabilities for task decomposition, generation, and intent classification through Go proxy endpoints.
## Requirements
### Requirement: Task decomposition uses Bridge AI with provider/model selection
The IM Bridge `/task decompose` command SHALL call the TS Bridge `/bridge/decompose` endpoint instead of the Go API directly, enabling provider and model selection for AI-powered task decomposition.

#### Scenario: Decompose task with default provider
- **WHEN** user sends `/task decompose task-123` via IM platform
- **THEN** IM Bridge calls `POST /api/v1/ai/decompose` (Go proxy to Bridge)
- **THEN** Bridge executes decomposition using default provider and model
- **THEN** IM Bridge replies with subtask list in the chat

#### Scenario: Decompose task with specific provider
- **WHEN** user sends `/task decompose task-123 anthropic` via IM platform
- **THEN** IM Bridge calls `POST /api/v1/ai/decompose` with `{ provider: "anthropic" }`
- **THEN** Bridge uses Anthropic provider for decomposition
- **THEN** IM Bridge replies with subtask list

#### Scenario: Decompose task with provider and model
- **WHEN** user sends `/task decompose task-123 openai gpt-4` via IM platform
- **THEN** IM Bridge calls `POST /api/v1/ai/decompose` with `{ provider: "openai", model: "gpt-4" }`
- **THEN** Bridge uses specified provider and model
- **THEN** IM Bridge replies with subtask list

#### Scenario: Bridge unavailable during decomposition
- **WHEN** user sends `/task decompose task-123` and Bridge is unavailable
- **THEN** IM Bridge falls back to Go API decomposition endpoint
- **THEN** IM Bridge replies with subtask list from Go fallback
- **THEN** IM Bridge logs fallback event for monitoring

### Requirement: Text generation command through Bridge
The IM Bridge SHALL provide `/task ai generate` command to generate text using Bridge AI capabilities with configurable provider and model.

#### Scenario: Generate text with defaults
- **WHEN** user sends `/task ai generate Write a summary of the project` via IM platform
- **THEN** IM Bridge calls `POST /api/v1/ai/generate` with `{ prompt: "Write a summary..." }`
- **THEN** Bridge generates text using default provider
- **THEN** IM Bridge replies with generated text

#### Scenario: Generate text with specific model
- **WHEN** user sends `/task ai generate --model claude-opus-4 Write a summary` via IM platform
- **THEN** IM Bridge calls `POST /api/v1/ai/generate` with `{ prompt: "...", model: "claude-opus-4" }`
- **THEN** Bridge uses specified model for generation
- **THEN** IM Bridge replies with generated text

### Requirement: Intent classification command through Bridge
The IM Bridge SHALL provide `/task ai classify` command to classify text intent using Bridge AI capabilities.

#### Scenario: Classify intent with candidate list
- **WHEN** user sends `/task ai classify "show sprint status" sprint_view,sprint_burndown,task_list` via IM platform
- **THEN** IM Bridge calls `POST /api/v1/ai/classify-intent` with `{ text: "show sprint status", candidates: ["sprint_view", ...] }`
- **THEN** Bridge returns classified intent with confidence score
- **THEN** IM Bridge replies with `{ intent: "sprint_view", confidence: 0.95 }`

#### Scenario: Classify intent with low confidence
- **WHEN** user sends `/task ai classify "status"` with ambiguous candidates
- **THEN** Bridge returns `{ intent: "task_list", confidence: 0.45 }`
- **THEN** IM Bridge replies with classification and notes low confidence

### Requirement: Go backend proxies Bridge AI endpoints
The Go backend SHALL expose `/api/v1/ai/generate`, `/api/v1/ai/classify-intent`, and `/api/v1/ai/decompose` endpoints that proxy to TS Bridge with full parameter support.

#### Scenario: Generate endpoint proxies to Bridge
- **WHEN** authenticated client calls `POST /api/v1/ai/generate` with `{ prompt: "...", provider: "anthropic", model: "claude-sonnet-4-5" }`
- **THEN** Go backend forwards request to `POST http://localhost:7778/bridge/generate`
- **THEN** Go backend returns Bridge response to client

#### Scenario: Classify-intent endpoint proxies to Bridge
- **WHEN** authenticated client calls `POST /api/v1/ai/classify-intent` with `{ text: "...", candidates: [...] }`
- **THEN** Go backend forwards to `POST http://localhost:7778/bridge/classify-intent`
- **THEN** Go backend returns Bridge response to client

#### Scenario: Decompose endpoint ensures Bridge parameter support
- **WHEN** authenticated client calls `POST /api/v1/ai/decompose` with `{ task_id: "...", provider: "openai", model: "gpt-4" }`
- **THEN** Go backend forwards all parameters to `POST http://localhost:7778/bridge/decompose`
- **THEN** Go backend returns decomposition result from Bridge
