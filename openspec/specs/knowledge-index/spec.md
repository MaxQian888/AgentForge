# knowledge-index Specification

## Purpose
Define the current declarative knowledge contract for role knowledge references, inherited knowledge merge, and runtime knowledge-context projection.
## Requirements
### Requirement: Role manifests preserve declarative knowledge references
The system SHALL preserve declarative role knowledge references for repositories, documents, patterns, shared knowledge sources, private knowledge sources, and memory settings in the normalized role manifest.

#### Scenario: Parse declarative repository and document knowledge
- **WHEN** a role manifest includes repositories, documents, and patterns in its knowledge section
- **THEN** the parser preserves those declarative knowledge references in the normalized role manifest

#### Scenario: Parse shared and private knowledge sources
- **WHEN** a role manifest includes shared or private knowledge source entries
- **THEN** the parser preserves those entries in the normalized role manifest

### Requirement: Effective role knowledge is merged through inheritance
The system SHALL merge inherited knowledge references across parent and child roles when resolving an effective role manifest.

#### Scenario: Merge inherited knowledge references
- **WHEN** a child role extends a parent role that defines repositories, documents, patterns, or knowledge sources
- **THEN** the effective role includes the inherited knowledge references together with child additions

### Requirement: Runtime execution profiles project declarative knowledge into prompt context
The system SHALL project the effective role knowledge section into the runtime execution profile as prompt-ready knowledge context.

#### Scenario: Build runtime knowledge context from effective role
- **WHEN** an effective role includes repositories, documents, patterns, shared knowledge sources, or private knowledge sources
- **THEN** the execution profile includes a formatted knowledge-context string derived from those declarative references

### Requirement: Current knowledge-index support is declarative rather than repository-index-backed

The system SHALL treat the current knowledge-index capability as declarative knowledge reference support plus runtime context projection plus automatic enqueue of wiki and template content for indexing. Concrete repository cloning, symbol extraction, full-text provider swap, embedding generation, and graph traversal are out of scope for this capability — those concerns SHALL live in sibling capabilities (`knowledge-asset-search` for lexical search, and future capabilities for semantic search and repo indexing).

#### Scenario: Do not assume semantic search or repo cloning from this capability

- **WHEN** a caller relies on the current knowledge-index capability definition
- **THEN** the guaranteed behavior is limited to knowledge reference parsing, inheritance, runtime context projection, and automatic enqueue of content-changed events for wiki and template assets

### Requirement: Wiki content is enqueued for indexing on save

The system SHALL enqueue content-changed events for `kind=wiki_page` and `kind=template` knowledge assets whenever their content is saved. The enqueue mechanism SHALL be the `IndexPipeline` interface defined by the `knowledge-asset-wiki-bridge` capability.

#### Scenario: Wiki save enqueues for indexing

- **WHEN** a `wiki_page` asset is saved
- **THEN** the system enqueues the asset for indexing via the `IndexPipeline.Enqueue` call path

#### Scenario: Template save enqueues for indexing

- **WHEN** a `template` asset's content is saved
- **THEN** the system enqueues the asset for indexing via the `IndexPipeline.Enqueue` call path
