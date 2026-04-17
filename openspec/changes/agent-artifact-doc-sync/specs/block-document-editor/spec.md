## ADDED Requirements

### Requirement: Live-artifact block type

The block editor SHALL support a `live_artifact` custom BlockNote block type whose visual and interactive behavior is owned by the `live-artifact-blocks` capability. The editor SHALL treat live-artifact blocks as opaque — it serializes and deserializes the block's `props` without inspecting them, renders a placeholder during loading, and defers content rendering to the live-artifact components.

#### Scenario: Editor serializes live blocks by props alone

- **WHEN** a document containing live-artifact blocks is saved
- **THEN** the BlockNote JSON persists only the block's `type`, `id`, and `props` — no content array or projected data

#### Scenario: Editor renders a loading placeholder before projection resolves

- **WHEN** a wiki page with live-artifact blocks is first rendered
- **THEN** each live-artifact block shows a skeleton placeholder until the projection endpoint resolves the block's content

#### Scenario: Editor defers actions to the live-artifact component

- **WHEN** the user interacts with a live-artifact block (freeze, open source, remove)
- **THEN** the editor passes the interaction through to the live-artifact component and does not treat those as regular block-editing operations

### Requirement: Entity-card block remains distinct from live-artifact blocks

The editor SHALL continue to offer the `entity-card` block (from the existing `Embedded entity card block` requirement) for inline single-entity references to tasks, agents, and reviews. The slash menu SHALL label entity-card and live-artifact entries distinctly so operators can choose between a compact inline card and a richer projection.

#### Scenario: Slash menu distinguishes the two

- **WHEN** the user opens the slash menu
- **THEN** the menu shows "Embed task card" (entity-card) and "Embed task group (live)" as distinct entries, and similarly for agent and review options vs their live-artifact equivalents where both exist
