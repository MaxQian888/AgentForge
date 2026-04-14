## ADDED Requirements

### Requirement: Runtime catalog publishes structured interaction capability metadata
The TypeScript bridge SHALL publish runtime catalog entries with a structured interaction capability matrix in addition to any legacy flat feature list. Each entry MUST describe the runtime's supported input surfaces, lifecycle controls, approval and permission pathways, MCP integration surface, and diagnostics state so upstream consumers can make runtime-aware decisions without inferring behavior from the runtime key alone.

#### Scenario: Catalog entry includes grouped interaction capabilities
- **WHEN** the backend or an equivalent upstream consumer requests Bridge runtime metadata
- **THEN** each runtime entry SHALL include machine-readable capability groups for `inputs`, `lifecycle`, `approval`, `mcp`, and `diagnostics`
- **THEN** the existing `supported_features` field MAY remain for compatibility, but it SHALL NOT be the only published interaction contract

#### Scenario: Capability is currently unavailable because prerequisites are missing
- **WHEN** a runtime capability such as Codex MCP approvals, Claude callback hooks, or OpenCode provider auth cannot currently run because required credentials or callback prerequisites are absent
- **THEN** the catalog SHALL publish that capability as degraded or unavailable together with actionable diagnostics
- **THEN** upstream consumers SHALL be able to distinguish missing prerequisites from permanent unsupported behavior
