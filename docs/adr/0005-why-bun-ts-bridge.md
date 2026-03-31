# ADR-0005: Why Bun For The TS Bridge / 为什么使用 Bun 作为 TS Bridge 运行时

- Status: Accepted
- Date: 2026-03-31
- Owners: AgentForge maintainers

## Context

The bridge owns runtime adapters for Claude, Codex, and OpenCode, plus plugin
SDK support for tool and review plugins. The project wants a TypeScript-centric
runtime that supports fast local iteration and compile-to-binary packaging for
the desktop bundle.

## Decision

Use Bun as the execution and distribution runtime for `src-bridge`. The bridge
source remains TypeScript, while release and desktop packaging use `bun build
--compile`.

## Consequences

- bridge builds can emit platform-specific single-file binaries for Tauri sidecars
- contributors need Bun installed for bridge work
- plugin tool/review scaffolds can reuse a single Bun-based local dev/test contract
