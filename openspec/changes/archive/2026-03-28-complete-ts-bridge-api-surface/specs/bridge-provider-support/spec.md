## ADDED Requirements

### Requirement: Go backend exposes generate and classify-intent as REST endpoints
Go backend SHALL proxy lightweight AI operations to Bridge, exposing `POST /api/v1/ai/generate` and `POST /api/v1/ai/classify-intent` behind authentication middleware.

#### Scenario: Generate with explicit provider
- **WHEN** client calls `POST /api/v1/ai/generate` with `{"prompt": "summarize", "provider": "openai", "model": "gpt-4o"}`
- **THEN** Go handler calls Bridge `/bridge/generate` with same payload and returns result

#### Scenario: Generate with default provider
- **WHEN** client calls `POST /api/v1/ai/generate` with `{"prompt": "summarize"}` (no provider)
- **THEN** Bridge resolves default provider from registry and returns result

#### Scenario: Classify intent for IM processing
- **WHEN** IM handler needs to classify user message intent
- **THEN** service calls `bridge.ClassifyIntent()` which routes to `/bridge/classify-intent`

### Requirement: Frontend RuntimeSelector includes provider and model selection
The shared `RuntimeSelector` component SHALL allow selecting provider and model in addition to runtime. Provider options SHALL be filtered based on selected runtime's compatible providers. Model options SHALL default to the provider's default model.

#### Scenario: Provider changes when runtime changes
- **WHEN** user selects runtime `codex` which only supports provider `openai`
- **THEN** provider dropdown auto-selects `openai` and model dropdown shows OpenAI models

#### Scenario: User overrides default model
- **WHEN** user selects provider `anthropic` and changes model from default to `claude-sonnet-4-20250514`
- **THEN** spawn request includes the overridden model value
