## ADDED Requirements

### Requirement: Go service layer calls Bridge status endpoint as execution fallback
Go agent service SHALL call `GET /bridge/status/:id` once after spawn to confirm execution started, as a fallback in case the WebSocket event was missed.

#### Scenario: Status confirms execution started
- **WHEN** agent is spawned and WS `agent.started` event was received
- **THEN** status check is skipped (WS event is authoritative)

#### Scenario: Status check recovers missed start event
- **WHEN** agent is spawned but no WS `agent.started` event arrives within 5 seconds
- **THEN** Go service calls `GET /bridge/status/:id` and updates agent state from response

### Requirement: Go backend exposes AI generation endpoints proxying to Bridge
Go backend SHALL expose `POST /api/v1/ai/generate` and `POST /api/v1/ai/classify-intent` endpoints that proxy to Bridge `/bridge/generate` and `/bridge/classify-intent` respectively.

#### Scenario: Generate text via API
- **WHEN** authenticated client calls `POST /api/v1/ai/generate` with `{"prompt": "...", "provider": "anthropic"}`
- **THEN** Go handler forwards to Bridge and returns generated text response

#### Scenario: Classify intent via API
- **WHEN** authenticated client calls `POST /api/v1/ai/classify-intent` with `{"text": "...", "candidates": [...]}`
- **THEN** Go handler forwards to Bridge and returns classification result

### Requirement: Go backend exposes runtime catalog endpoint
Go backend SHALL expose `GET /api/v1/bridge/runtimes` that proxies to Bridge `/bridge/runtimes` and returns the runtime catalog.

#### Scenario: Frontend fetches runtime catalog
- **WHEN** authenticated client calls `GET /api/v1/bridge/runtimes`
- **THEN** response contains array of runtime entries with key, display_name, default_provider, default_model, available, diagnostics
