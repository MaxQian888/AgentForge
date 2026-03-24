## Why

The TypeScript bridge currently executes agent work through a single Claude-specific runtime, while AgentForge is already accumulating role definitions and orchestration contracts that need to target more than one coding agent backend. We need a unified runtime mechanism now so Claude Code, Codex, and OpenCode can plug into the same bridge surface without creating parallel execution paths or hard-coded backend branches in Go and TypeScript.

## What Changes

- Add a bridge-local agent runtime registry that resolves which coding runtime should handle an execute request, including Claude Code, Codex, and OpenCode.
- Extend the bridge execution contract so requests can select a runtime explicitly or fall back to a configured default, with consistent validation and unsupported-runtime errors.
- Isolate Claude-specific execution behind a runtime adapter and add equivalent adapter seams for Codex and OpenCode so new runtimes can be added without rewriting `handleExecute(...)`.
- Define shared runtime configuration, capability metadata, and launch requirements for agent backends, including environment variables, executable discovery, and model/runtime defaults.
- Preserve the existing bridge event, budget, cancellation, and session snapshot contract while allowing each runtime adapter to normalize its native output into the canonical AgentForge event stream.

## Capabilities

### New Capabilities
- `bridge-agent-runtime-registry`: Define the TypeScript bridge mechanism for registering, resolving, configuring, and validating multiple coding-agent runtimes behind one extensible execution surface.

### Modified Capabilities
- `agent-sdk-bridge-runtime`: Expand the execute runtime contract so the bridge selects an agent runtime through the shared registry and keeps Claude-backed execution as one adapter within that mechanism.

## Impact

- Affected TypeScript bridge code in `src-bridge/src/server.ts`, `src-bridge/src/schemas.ts`, `src-bridge/src/types.ts`, `src-bridge/src/handlers/execute.ts`, the current Claude runtime module, and new runtime-registry/adapter modules.
- Affected Go bridge integration in `src-go/internal/bridge/client.go` and any service-layer code that currently treats `provider` and `model` as sufficient to identify the execution backend.
- Affected runtime configuration and operational setup for Claude Code, Codex, and OpenCode credentials, binary discovery, and default runtime/model selection.
- Affected execution behavior for validation, runtime selection, error reporting, and normalized streaming of output, tool activity, cost, cancellation, and snapshots.
