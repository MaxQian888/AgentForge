## 1. Runtime catalog and shared contract baseline

- [x] 1.1 Extend `src-bridge` runtime catalog types, builders, and route responses to publish structured interaction capability metadata alongside legacy `supported_features`
- [x] 1.2 Add shared support-state and diagnostics primitives so lifecycle controls can report `supported` / `degraded` / `unsupported` consistently across catalog and route handlers
- [x] 1.3 Enforce callback-dependent request validation for hooks, tool permission callbacks, and other interaction inputs that require an upstream callback surface

## 2. Claude Code interaction alignment

- [x] 2.1 Expand Claude hook schemas and runtime launch wiring to cover the official hook events AgentForge needs for orchestration and callback forwarding
- [x] 2.2 Publish Claude live-control capability metadata from the active Query surface and add the missing Bridge control handling for supported Query methods
- [x] 2.3 Add focused Claude runtime tests covering hook validation, callback payloads, and truthful supported/unsupported live-control publishing

## 3. Codex config and approval alignment

- [x] 3.1 Introduce a Bridge-owned Codex config overlay builder for per-run MCP, approval, sandbox-intent, and runtime-default settings
- [x] 3.2 Update the Codex launcher and catalog publishing so config-governed concerns use the overlay while prompt/image/search inputs keep their direct CLI mappings
- [x] 3.3 Add focused Codex tests proving overlay generation, MCP approval metadata publishing, and explicit unsupported handling for interaction modes Codex does not truthfully support

## 4. OpenCode server control-plane completeness

- [x] 4.1 Extend OpenCode catalog publishing to include provider/auth readiness plus server-backed session control metadata from the official server surfaces
- [x] 4.2 Add the canonical Bridge shell route and proxy it through the official OpenCode session shell endpoint with capability-aware validation
- [x] 4.3 Tighten OpenCode diagnostics and tests around auth-required states, shell control support, and runtime catalog discovery for agents/skills/providers

## 5. Canonical routes, docs, and conformance proof

- [x] 5.1 Update canonical Bridge interaction routes to return capability-aware structured errors that stay aligned with `/bridge/runtimes`
- [x] 5.2 Add doc-grounded conformance fixtures/tests for Claude Code, Codex, and OpenCode request fields, route behavior, and published capability metadata
- [x] 5.3 Refresh the relevant Bridge runtime documentation/spec references so the repo describes the new capability matrix and canonical interaction controls truthfully
