## ADDED Requirements

### Requirement: Agent execution honors the Bridge provider capability contract
The TypeScript Bridge SHALL resolve `provider` and `model` for execute requests through the shared provider registry, and it MUST only start agent execution when the resolved provider supports the `agent_execution` capability.

#### Scenario: Execute request uses the default supported provider
- **WHEN** the Go orchestrator submits a valid execute request without an explicit provider override
- **THEN** the Bridge SHALL resolve the default `agent_execution` provider and model from the registry
- **THEN** the Bridge SHALL start the real Claude Agent SDK-backed runtime for that task

#### Scenario: Execute request asks for an unsupported execution provider
- **WHEN** the Go orchestrator submits an execute request whose resolved provider does not support `agent_execution`
- **THEN** the Bridge SHALL reject the request before creating a runtime entry
- **THEN** it SHALL return an explicit error instead of ignoring the provider field or silently switching runtimes

