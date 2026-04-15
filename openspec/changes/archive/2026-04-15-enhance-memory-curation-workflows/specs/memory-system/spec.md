## MODIFIED Requirements

### Requirement: Repository-backed memory service manages project memory records
The system SHALL provide a memory service family that stores project memory records through the configured repository, defaults missing scope and category values, supports explorer-ready filtering and detail retrieval, supports deletion and bounded cleanup workflows, supports operator note creation plus tag-aware curation, updates access counts when records are read, and returns DTOs for API consumers.

#### Scenario: Store memory with default scope and category
- **WHEN** a memory record is stored without an explicit scope or category
- **THEN** the service defaults the scope to `project`
- **THEN** the service defaults the category to `episodic`
- **THEN** a legacy write that uses `category = operator_note` is normalized to the canonical stored shape for an operator-authored note instead of persisting an undefined category

#### Scenario: Explorer search honors aligned filters
- **WHEN** the caller searches project memories with query text plus optional `scope`, `category`, `roleId`, `tag`, `startAt`, `endAt`, or `limit` filters
- **THEN** the service returns matching memory DTOs from the repository instead of silently dropping those filter inputs
- **THEN** the service increments access counts for the records returned to the caller

#### Scenario: Memory detail is available for explorer consumers
- **WHEN** the caller requests a single project memory entry by ID within an allowed scope
- **THEN** the service returns the corresponding memory record with explorer-facing timestamps, metadata fields, normalized tags, and curation flags indicating whether content edits are allowed
- **THEN** the caller does not need to reconstruct editability or tags from raw metadata strings

#### Scenario: Operator note is updated within allowed curation boundary
- **WHEN** the caller updates an operator-authored note
- **THEN** the service persists the requested key, content, and tag changes
- **THEN** the service updates the record timestamp so explorer consumers can detect the curation change

#### Scenario: Tags can be curated without changing memory provenance
- **WHEN** the caller adds or removes tags for an accessible memory entry
- **THEN** the service stores a normalized, deduplicated tag set for that entry
- **THEN** the original content, category, and creation provenance remain intact unless the entry is explicitly allowed to edit content

#### Scenario: Delete memory by ID
- **WHEN** the caller deletes a memory entry by ID
- **THEN** the service delegates the delete operation to the configured repository

#### Scenario: Episodic cleanup removes old records within allowed scope
- **WHEN** the caller requests cleanup of episodic memories older than a specific cutoff or retention window
- **THEN** the service deletes only the matching old episodic records within the accessible project scope
- **THEN** the service returns the number of deleted records

### Requirement: Current memory-system support is limited to short-term memory and repository-backed project memory services
The system SHALL treat the current memory capability as covering short-term scoped memory, repository-backed project memory storage, prompt injection, explorer-oriented episodic history, export, retention workflows, operator note authoring, tag curation, and controlled editing of operator-authored notes. Semantic vector search, procedural learning automation, arbitrary editing of system-generated memories, and long-term compaction workflows are not yet guaranteed by this capability.

#### Scenario: Caller relies on explorer-oriented curation workflows
- **WHEN** a caller uses the current memory-system capability for project memory exploration and curation
- **THEN** filtered history queries, JSON export, bounded retention cleanup, operator note creation, tag curation, and controlled editing of operator-authored notes are guaranteed behaviors of the supported contract

#### Scenario: Do not assume unsupported long-term memory features
- **WHEN** a caller relies on the current memory-system capability definition
- **THEN** semantic vector search, procedural learning automation, arbitrary editing of system-generated memories, and long-term compaction are not implied by this capability
