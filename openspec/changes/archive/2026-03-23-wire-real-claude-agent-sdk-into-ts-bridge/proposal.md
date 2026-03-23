## Why

The TypeScript bridge is still simulating agent execution instead of running the real Claude Agent SDK, so the Go orchestrator cannot exercise the core runtime that AgentForge's architecture and product docs already depend on. We need to close that gap now so task execution, event streaming, cost tracking, and session handling can move from placeholder behavior to a production-facing contract.

## What Changes

- Replace the simulated execution path in `src-bridge` with a real Claude Agent SDK-backed execution flow.
- Align the Go bridge client and the TypeScript bridge HTTP contract so execute, status, cancel, and health requests target the same routes and payload shapes.
- Define how the bridge converts Claude Agent SDK outputs into the existing event stream that Go consumes, including status, output, tool activity, errors, snapshots, and cost updates.
- Introduce the bridge-side runtime and configuration responsibilities needed for API key handling, role injection, budget enforcement, and graceful cancellation/resume boundaries.
- Document the implementation boundary so follow-up apply work can land SDK wiring without expanding into unrelated IM, review, or frontend surfaces.

## Capabilities

### New Capabilities
- `agent-sdk-bridge-runtime`: Run real Claude Agent SDK sessions through the TypeScript bridge and stream normalized execution events back to the Go orchestrator.

### Modified Capabilities

## Impact

- Affected TypeScript bridge code in `src-bridge/src/server.ts`, `src-bridge/src/handlers/execute.ts`, runtime/session modules, and bridge package dependencies.
- Affected Go bridge integration in `src-go/internal/bridge/client.go` and any service code that assumes the current request/response contract.
- Affected runtime contracts for WebSocket event streaming, budget/cost reporting, cancellation, snapshots, and bridge health semantics.
- Expected new dependency and build considerations around the Claude Agent SDK and its required bridge runtime configuration.
