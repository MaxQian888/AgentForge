## MODIFIED Requirements

### Requirement: Repository-backed memory service manages project memory records
The system SHALL provide a memory service family that stores project memory records through the configured repository, defaults missing scope and category values, supports explorer-ready filtering and detail retrieval, supports deletion and bounded cleanup workflows, updates access counts when records are read, and returns DTOs for API consumers.

#### Scenario: Store memory with default scope and category
- **WHEN** a memory record is stored without an explicit scope or category
- **THEN** the service defaults the scope to `project`
- **THEN** the service defaults the category to `episodic`

#### Scenario: Explorer search honors aligned filters
- **WHEN** the caller searches project memories with query text plus optional `scope`, `category`, `roleId`, `startAt`, `endAt`, or `limit` filters
- **THEN** the service returns matching memory DTOs from the repository instead of silently dropping those filter inputs
- **THEN** the service increments access counts for the records returned to the caller

#### Scenario: Memory detail is available for explorer consumers
- **WHEN** the caller requests a single project memory entry by ID within an allowed scope
- **THEN** the service returns the corresponding memory record with explorer-facing timestamps and metadata fields
- **THEN** the caller does not need to reconstruct detail state from a list-only response

#### Scenario: Delete memory by ID
- **WHEN** the caller deletes a memory entry by ID
- **THEN** the service delegates the delete operation to the configured repository

#### Scenario: Episodic cleanup removes old records within allowed scope
- **WHEN** the caller requests cleanup of episodic memories older than a specific cutoff or retention window
- **THEN** the service deletes only the matching old episodic records within the accessible project scope
- **THEN** the service returns the number of deleted records

### Requirement: Current memory-system support is limited to short-term memory and repository-backed project memory services
The system SHALL treat the current memory capability as covering short-term scoped memory, repository-backed project memory storage, prompt injection, and explorer-oriented episodic history, export, and retention workflows. Semantic vector search, procedural learning automation, memory tagging, memory editing, and long-term compaction workflows are not yet guaranteed by this capability.

#### Scenario: Caller relies on explorer-oriented episodic workflows
- **WHEN** a caller uses the current memory-system capability for project memory exploration
- **THEN** filtered episodic history queries, JSON export, and bounded retention cleanup are guaranteed behaviors of the supported backend contract

#### Scenario: Do not assume unsupported long-term memory features
- **WHEN** a caller relies on the current memory-system capability definition
- **THEN** semantic vector search, procedural learning automation, memory tagging, and arbitrary memory editing are not implied by this capability
