## ADDED Requirements

### Requirement: Asset content_json preserves live-artifact block references across lifecycle operations

The system SHALL round-trip `live_artifact` block props through every asset lifecycle operation — save, load, version snapshot, version restore, and copy-paste into another asset — without modifying, stripping, or flattening the block's reference props.

#### Scenario: Save then load preserves references

- **WHEN** an asset containing one or more live-artifact blocks is saved and then reloaded
- **THEN** each live-artifact block's `live_kind`, `target_ref`, `view_opts`, and `view_opts_schema_version` are byte-identical to what was saved

#### Scenario: Version snapshot captures references not projections

- **WHEN** a named version is created on an asset containing live-artifact blocks
- **THEN** the version's stored `content_json` contains the block references but no embedded projection payload

#### Scenario: Version restore keeps references live

- **WHEN** an asset is restored from a version that contains live-artifact blocks
- **THEN** the restored blocks remain live — they project against current entity state, not state as of the snapshot

#### Scenario: Copy-paste replicates the reference

- **WHEN** a live-artifact block is copied from one wiki-page asset and pasted into another
- **THEN** the destination asset's `content_json` contains a live-artifact block with identical reference props (the BlockNote id may be regenerated)
