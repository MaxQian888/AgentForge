## ADDED Requirements

### Requirement: Claude callback hook coverage aligns with the official hook event taxonomy used by AgentForge
The Bridge SHALL accept and publish the current Claude Code hook events that AgentForge depends on for orchestration, including tool lifecycle, subagent lifecycle, prompt submission, notification, and session lifecycle events. The callback payload and validation rules MUST stay aligned with the official Claude Code / Agent SDK hook surface instead of freezing the Bridge to an older subset of hook names.

#### Scenario: Request enables newer Claude hook events
- **WHEN** an execute request includes Claude hook subscriptions for events such as `SessionStart`, `SessionEnd`, `Notification`, or `UserPromptSubmit`
- **THEN** the Bridge SHALL validate those hook declarations and preserve them through the Claude runtime launch path
- **THEN** the Bridge SHALL NOT reject them merely because an older local schema only recognized tool and subagent events

#### Scenario: Hook event is surfaced through the Bridge callback contract
- **WHEN** Claude emits a supported hook event that AgentForge forwards through its orchestrator callback path
- **THEN** the Bridge SHALL emit a callback payload that identifies the hook event, task context, and relevant runtime payload fields
- **THEN** downstream orchestrators SHALL be able to make policy decisions without parsing Claude-native transport details directly

### Requirement: Claude live control publishing reflects query method availability truthfully
The Bridge SHALL publish and enforce Claude live controls based on the methods actually available on the active Query object. Controls such as interrupt, model switching, thinking-budget control, and MCP status introspection MUST only be advertised as supported when the active runtime and SDK surface can execute them.

#### Scenario: Active Query exposes a live control
- **WHEN** the active Claude Query exposes methods such as `interrupt`, `setModel`, `setMaxThinkingTokens`, or `mcpServerStatus`
- **THEN** the runtime capability metadata SHALL publish the corresponding control as supported for that active runtime
- **THEN** the canonical Bridge control route SHALL invoke the matching Query method instead of simulating the result

#### Scenario: Live control is unavailable on the active Query
- **WHEN** a caller requests a Claude live control whose underlying Query method is unavailable in the current SDK/runtime combination
- **THEN** the Bridge SHALL return an explicit unsupported or degraded response that identifies the missing Query capability
- **THEN** the runtime capability metadata SHALL match that unsupported or degraded state
