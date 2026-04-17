## ADDED Requirements

### Requirement: Live-artifact block category in the wiki editor

The system SHALL provide a new block category in the wiki editor named "Live Artifact." Live-artifact blocks SHALL be insertable only in `kind=wiki_page` knowledge assets. Supported initial kinds SHALL be `agent_run`, `cost_summary`, `review`, and `task_group`.

#### Scenario: Insert agent_run live block via slash menu

- **WHEN** user types `/` in the wiki editor and selects "Embed agent run (live)"
- **THEN** the editor shows a picker for the target agent run and inserts a `live_artifact` block with `live_kind=agent_run` and `target_ref={kind:"agent_run", id}` once selected

#### Scenario: Insert cost_summary live block

- **WHEN** user selects "Embed cost summary (live)" from the slash menu
- **THEN** the editor opens a filter dialog (time range, optional runtime/provider/member) and inserts a `live_artifact` block with `live_kind=cost_summary` and the filter as `target_ref.filter`

#### Scenario: Insert review live block

- **WHEN** user selects "Embed review (live)" from the slash menu
- **THEN** the editor shows a picker for the target review and inserts a `live_artifact` block with `live_kind=review` and `target_ref={kind:"review", id}`

#### Scenario: Insert task_group live block

- **WHEN** user selects "Embed task group (live)" from the slash menu
- **THEN** the editor shows a filter builder (saved view or inline filter) and inserts a `live_artifact` block with `live_kind=task_group` and the filter as `target_ref.filter`

#### Scenario: Reject insertion outside wiki_page kinds

- **WHEN** the editor attempts to insert a `live_artifact` block in a `kind=template` or `kind=ingested_file` asset
- **THEN** the insertion is rejected and the slash menu does not offer the live-artifact category for those kinds

### Requirement: Live-artifact block reference persistence

The system SHALL persist each live-artifact block in `content_json` as a BlockNote custom block with `type: "live_artifact"` and `props: { live_kind, target_ref, view_opts, view_opts_schema_version, last_rendered_at? }`. The block SHALL NOT store the projected content inside `content_json`.

#### Scenario: Block reference survives save

- **WHEN** an asset containing live-artifact blocks is saved and reloaded
- **THEN** each live block's `live_kind`, `target_ref`, `view_opts`, and `view_opts_schema_version` round-trip unchanged

#### Scenario: Block reference survives version snapshot and restore

- **WHEN** a version snapshot is created and later restored
- **THEN** live blocks in the restored content still hold their references and render live against current entity state (they do not render the state as of the snapshot time)

#### Scenario: Projected content not written to content_json

- **WHEN** a save is triggered
- **THEN** the serialized BlockNote JSON contains no embedded projection payload — only the reference props

### Requirement: Open-source action

Each live-artifact block SHALL provide an "Open source" affordance that navigates to the authoritative surface for its target.

#### Scenario: Open agent_run source

- **WHEN** user clicks "Open source" on an `agent_run` live block
- **THEN** the system navigates to `/agents/:id` for the referenced run

#### Scenario: Open cost_summary source

- **WHEN** user clicks "Open source" on a `cost_summary` live block
- **THEN** the system navigates to `/cost` with the block's filter encoded as URL parameters

#### Scenario: Open review source

- **WHEN** user clicks "Open source" on a `review` live block
- **THEN** the system navigates to `/reviews/:id` for the referenced review

#### Scenario: Open task_group source

- **WHEN** user clicks "Open source" on a `task_group` live block
- **THEN** the system navigates to `/tasks` with the block's filter (saved view id or inline filter) applied

### Requirement: Freeze-as-static action

The system SHALL provide a "Freeze" action that replaces a live-artifact block with static BlockNote blocks capturing the current projection. Freezing SHALL create an `AssetVersion` snapshot of the pre-freeze state.

#### Scenario: Freeze creates static fragment plus snapshot callout

- **WHEN** user clicks "Freeze" on a live-artifact block
- **THEN** the system calls the projector, replaces the live block with the returned BlockNote fragment preceded by a callout block "Frozen from {live_kind summary} on {ISO date}"
- **THEN** the system creates a new `AssetVersion` named "Frozen live artifact {block_id}" capturing the pre-freeze content

#### Scenario: Freeze reject on non-ok projection

- **WHEN** the projector returns `Status != ok` for the block
- **THEN** the freeze action is rejected with a user-visible error explaining that the block cannot be frozen in its current state

### Requirement: Orphan handling

A live-artifact block SHALL render an explicit degraded state when its target cannot be resolved, access is denied, or the projector fails.

#### Scenario: Target not found

- **WHEN** the projector returns `Status=not_found` because the referenced entity has been deleted
- **THEN** the block renders a "no longer available" state and exposes "Remove block" and (if a cached snapshot exists) "Freeze last known snapshot" actions

#### Scenario: Access forbidden

- **WHEN** the projector returns `Status=forbidden` for the current principal
- **THEN** the block renders "You do not have access to this live artifact" without revealing the target id, title, or any projected content

#### Scenario: Projector degraded

- **WHEN** the projector returns `Status=degraded`
- **THEN** the block renders a warning banner with the diagnostics string and, if a recent successful projection exists within the client-cached TTL hint, continues to show that last-known projection

### Requirement: Per-block RBAC gating

The system SHALL evaluate RBAC for each live-artifact block independently of the hosting asset. Read access to the asset SHALL NOT imply read access to the referenced entity.

#### Scenario: Reader has asset access but not entity access

- **WHEN** a viewer opens an asset containing a `cost_summary` live block and the viewer lacks cost-read permission
- **THEN** the `cost_summary` block renders `forbidden` state while the rest of the asset renders normally

#### Scenario: RBAC change takes effect on refresh

- **WHEN** the principal's role changes while an asset is open
- **THEN** on the next projection call (triggered by any subscription push or a manual refresh) the blocks re-evaluate RBAC and switch to or from `forbidden` accordingly
