# ADR-0001: Why Tauri Instead Of Electron / 为什么选择 Tauri 而非 Electron

- Status: Accepted
- Date: 2026-03-31
- Owners: AgentForge maintainers

## Context

AgentForge needs a desktop shell that can supervise the Go orchestrator, the TS
bridge, and the IM bridge while reusing the same web UI. The project also wants
minimal native permissions and a smaller packaging footprint than a bundled
Chromium stack.

## Decision

Use Tauri 2 as the desktop shell and keep the product UI in the existing Next.js
frontend. Native integration is exposed through capability-scoped IPC plus named
sidecar execution for the backend services.

## Consequences

- smaller desktop bundle and lower idle memory than a full Electron runtime
- Rust and Tauri tooling become part of the build and test matrix
- native permissions stay explicit through capability JSON files instead of a
  broad Node.js runtime inside the window
