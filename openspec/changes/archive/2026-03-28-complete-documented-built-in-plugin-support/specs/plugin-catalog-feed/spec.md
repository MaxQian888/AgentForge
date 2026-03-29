## MODIFIED Requirements

### Requirement: Catalog entries are sourced from real plugin manifests
The system SHALL serve plugin catalog data from real built-in and installable plugin manifests the Go server can resolve, instead of returning hardcoded placeholder marketplace entries. Entries shown in the built-in availability section MUST correspond to manifest-backed assets that are declared in the repository's official built-in plugin bundle. Each catalog entry MUST include plugin identifier, name, version, kind, description, source metadata, an install action target the Go control plane can actually use, and truthful availability metadata when the entry belongs to the official built-in bundle.

#### Scenario: Official built-in plugin appears in the catalog
- **WHEN** the Go server resolves a built-in bundle entry whose manifest path exists and parses successfully
- **THEN** the catalog feed includes that plugin with built-in source metadata, official built-in provenance, and an install target derived from the real manifest path

#### Scenario: Broken or unlisted built-in asset is not exposed as an official plugin
- **WHEN** a manifest is missing, invalid, or not declared in the repository's built-in plugin bundle
- **THEN** the control plane does not expose that asset as an official built-in catalog entry
- **THEN** it does not synthesize a placeholder install target for the missing or unlisted asset

#### Scenario: Placeholder entries are not returned
- **WHEN** the catalog endpoint is requested
- **THEN** it does not return synthetic plugin entries that are unrelated to resolvable manifests on disk or configured install sources
