## ADDED Requirements

### Requirement: Codex launch semantics are assembled from a Bridge-owned config overlay
The Bridge SHALL assemble Codex runtime configuration from a Bridge-owned per-run config overlay that can express runtime defaults, MCP server definitions, MCP tool approval preferences, and approval or sandbox intent alongside any direct CLI flags. Config-governed concerns MUST flow through the overlay so the Bridge stays aligned with the official Codex config and MCP contract instead of depending on ad hoc flag combinations alone.

#### Scenario: Codex run needs MCP servers and approval metadata
- **WHEN** a Codex execute request includes active MCP plugins or approval-related runtime intent
- **THEN** the Bridge SHALL materialize a Codex-compatible configuration overlay that preserves the server definitions and any supported approval preferences for that run
- **THEN** the Codex launcher SHALL consume that overlay together with direct CLI inputs such as prompt, image attachments, or web search flags

#### Scenario: Requested approval interaction exceeds Codex's published support
- **WHEN** a caller requests an interactive approval or permission callback mode that the current Codex bridge contract cannot truthfully provide
- **THEN** the Bridge SHALL publish that interaction as unsupported or degraded in the runtime capability metadata
- **THEN** execution SHALL fail with an explicit configuration or unsupported error instead of pretending Claude-style callback parity exists
