# ADR-0003: Why Use A WASM Plugin Runtime / 为什么引入 WASM 插件运行时

- Status: Accepted
- Date: 2026-03-31
- Owners: AgentForge maintainers

## Context

AgentForge needs plugin extensibility for workflow and integration features
without embedding arbitrary external code directly into the Go orchestrator
process. The project also wants a portable packaging model that works with the
current desktop sidecar distribution flow.

## Decision

Use a Go-hosted WASM runtime for the plugin classes that are owned by the Go
control plane, especially workflow and integration plugins. Keep the plugin
contract manifest-driven and ABI-versioned.

## Consequences

- plugin manifests declare runtime, ABI version, capabilities, and source explicitly
- workflow and integration starters can be scaffolded into predictable repo-local layouts
- ABI compatibility and runtime debugging become explicit maintenance concerns
