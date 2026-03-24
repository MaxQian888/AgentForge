## 1. Data Model And Catalog Contract

- [ ] 1.1 Extend project settings and related DTO/store types to carry a coding-agent runtime catalog plus default `runtime` / `provider` / `model` values.
- [ ] 1.2 Add explicit `runtime` persistence to agent run storage, DTOs, summaries, and any team summary payloads that currently only expose `provider` / `model`.
- [ ] 1.3 Introduce a backend resolver that merges project defaults with explicit launch overrides into one validated coding-agent launch tuple.

## 2. Bridge Runtime Catalog And Validation

- [ ] 2.1 Extend the bridge runtime registry to publish catalog metadata for `claude_code`, `codex`, and `opencode`, including compatible providers, default model metadata, and default runtime identity.
- [ ] 2.2 Add bridge-side readiness diagnostics that report missing credentials, missing executables, or incompatible runtime/provider combinations without starting execution.
- [ ] 2.3 Update bridge execute and status contracts so Go-managed runs use explicit runtime tuples and status payloads expose resolved `runtime` / `provider` / `model`.

## 3. Backend Launch Flows And Team Propagation

- [ ] 3.1 Update agent spawn and dispatch flows to resolve project defaults before calling the bridge and stop relying on provider-only runtime guessing in the Go-managed path.
- [ ] 3.2 Update team start, downstream planner/coder/reviewer spawning, and retry flows to preserve one resolved coding-agent selection across the full team lifecycle.
- [ ] 3.3 Ensure agent/team API responses, summaries, and WebSocket-facing payloads return the resolved runtime identity needed by frontend views and debugging flows.

## 4. Frontend Settings And Launch UX

- [ ] 4.1 Expand the project settings page and project store so users can view runtime readiness diagnostics and configure default coding-agent runtime/provider/model values.
- [ ] 4.2 Replace hard-coded Team launch defaults with catalog-driven runtime/provider/model selectors and surface incompatibility or readiness errors before submission.
- [ ] 4.3 Update agent or team detail consumers to display the resolved runtime identity consistently in lists, summaries, and run detail views.

## 5. Documentation And Verification

- [ ] 5.1 Update README, PRD-aligned runtime docs, and role/runtime guidance to describe Claude Code, Codex, and OpenCode support, required environment variables, and compatibility rules.
- [ ] 5.2 Add focused Go tests for project default resolution, agent spawn persistence, and team phase propagation of runtime/provider/model.
- [ ] 5.3 Add focused bridge tests for runtime catalog diagnostics, strict runtime/provider validation, and status payload runtime identity.
- [ ] 5.4 Add focused frontend tests for settings catalog rendering and Team launch selection behavior so Claude Code, Codex, and OpenCode remain covered end to end.
