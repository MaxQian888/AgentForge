## MODIFIED Requirements

### Requirement: Bridge execute requests accept normalized role execution profiles from Go
The TypeScript bridge SHALL treat `role_config` in execute requests as a normalized execution profile produced by the Go role-loading pipeline rather than as a raw Role YAML document. The bridge MUST apply the projected role persona, tool allowlist, bridge-consumable tool plugin identifiers, injected knowledge context, projected loaded skill context, available on-demand skill inventory, output filters, and permission constraints without needing to read YAML files, resolve inheritance, or interpret repo-local skill files locally.

#### Scenario: Expanded normalized role execution profile is honored
- **WHEN** the Go orchestrator submits an execute request with a valid normalized `role_config` that includes `allowed_tools`, `tools`, `knowledge_context`, loaded skill context, available skill inventory, and `output_filters`
- **THEN** the bridge uses that projected configuration when composing the effective system prompt, tool or plugin selection, and output filtering behavior for the runtime
- **THEN** execution uses the projected runtime-facing values instead of silently dropping the advanced fields

#### Scenario: Bridge does not need direct YAML or skill file access
- **WHEN** the bridge receives a valid execute or resume request whose role and skill tree were resolved from repo-local assets by Go
- **THEN** the bridge executes the task without reading the roles directory or parsing `skills/**/SKILL.md` itself
- **THEN** unsupported PRD-only sections remain outside the bridge request contract

#### Scenario: On-demand skills stay available without preloading full instructions
- **WHEN** the Go orchestrator submits a normalized role execution profile that contains available non-auto-load skills
- **THEN** the bridge preserves that inventory context for runtime prompt composition or diagnostics
- **THEN** the bridge does not inject the full instruction bodies for those on-demand skills unless the normalized profile explicitly marks them as loaded
