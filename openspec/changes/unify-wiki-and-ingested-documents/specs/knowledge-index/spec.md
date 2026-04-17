## MODIFIED Requirements

### Requirement: Current knowledge-index support is declarative rather than repository-index-backed

The system SHALL treat the current knowledge-index capability as declarative knowledge reference support plus runtime context projection plus automatic enqueue of wiki and template content for indexing. Concrete repository cloning, symbol extraction, full-text provider swap, embedding generation, and graph traversal are out of scope for this capability — those concerns SHALL live in sibling capabilities (`knowledge-asset-search` for lexical search, and future capabilities for semantic search and repo indexing).

#### Scenario: Do not assume semantic search or repo cloning from this capability

- **WHEN** a caller relies on the current knowledge-index capability definition
- **THEN** the guaranteed behavior is limited to knowledge reference parsing, inheritance, runtime context projection, and automatic enqueue of content-changed events for wiki and template assets

## ADDED Requirements

### Requirement: Wiki content is enqueued for indexing on save

The system SHALL enqueue content-changed events for `kind=wiki_page` and `kind=template` knowledge assets whenever their content is saved. The enqueue mechanism SHALL be the `IndexPipeline` interface defined by the `knowledge-asset-wiki-bridge` capability.

#### Scenario: Wiki save enqueues for indexing

- **WHEN** a `wiki_page` asset is saved
- **THEN** the system enqueues the asset for indexing via the `IndexPipeline.Enqueue` call path

#### Scenario: Template save enqueues for indexing

- **WHEN** a `template` asset's content is saved
- **THEN** the system enqueues the asset for indexing via the `IndexPipeline.Enqueue` call path
