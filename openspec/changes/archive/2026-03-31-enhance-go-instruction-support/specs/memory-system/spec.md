# Memory System Specification

## ADDED Requirements

### Requirement: Short-term memory maintains scoped context within a token budget
The system SHALL provide an in-memory short-term memory store that keeps entries per scope, estimates token usage, enforces a configurable token budget, and returns recent context within a context token limit.

#### Scenario: Store entries within configured token budget
- **WHEN** entries are stored under the same short-term memory scope and total estimated tokens remain within the configured budget
- **THEN** the store keeps those entries in the scope snapshot

#### Scenario: Retrieve recent context within context budget
- **WHEN** the caller requests context for a scope without overriding the context token budget
- **THEN** the store returns the most recent entries that fit within the configured context token limit

### Requirement: Short-term memory supports configurable eviction policies
The short-term memory store SHALL evict entries when a scope exceeds its configured token budget using the configured eviction policy.

#### Scenario: LRU eviction keeps most recent entries
- **WHEN** the store uses the LRU eviction policy and a new entry would exceed the token budget
- **THEN** the least recently used entries are evicted first

#### Scenario: Importance eviction preserves higher-priority entries
- **WHEN** the store uses the importance-based eviction policy and a new entry would exceed the token budget
- **THEN** lower-importance entries are evicted before higher-importance entries when possible

### Requirement: Repository-backed memory service manages project memory records
The system SHALL provide a memory service that stores project memory records through the configured repository, defaults missing scope and category values, supports search, deletion, and access-count updates, and returns DTOs for API consumers.

#### Scenario: Store memory with default scope and category
- **WHEN** a memory record is stored without an explicit scope or category
- **THEN** the service defaults the scope to `project`
- **THEN** the service defaults the category to `episodic`

#### Scenario: Search memories updates access counts
- **WHEN** the caller searches project memories by query
- **THEN** the service returns matching memory DTOs from the repository
- **THEN** the service increments access counts for the returned records

#### Scenario: Delete memory by ID
- **WHEN** the caller deletes a memory entry by ID
- **THEN** the service delegates the delete operation to the configured repository

### Requirement: Memory service injects recent project context into runtime prompts
The system SHALL format recent project memories into system-prompt context for runtime execution, filtering role-scoped entries that do not match the requested role.

#### Scenario: Inject up to ten recent memories into system prompt context
- **WHEN** project memories exist for the requested project
- **THEN** the service formats up to ten memories into a prompt-ready project memory context block

#### Scenario: Exclude unrelated role-scoped memories
- **WHEN** the caller injects context for a specific role ID
- **THEN** role-scoped memory entries for other roles are excluded from the injected context

### Requirement: Team completion learnings are stored as episodic project memory
The system SHALL summarize completed team runs into project-scoped episodic memory entries.

#### Scenario: Record team learnings after team completion
- **WHEN** team completion data is recorded with team metadata and run summaries
- **THEN** the memory service stores a project-scoped episodic memory entry summarizing the team completion outcome

### Requirement: Current memory-system support is limited to short-term memory and repository-backed project memory services
The system SHALL treat the current memory capability as covering short-term scoped memory plus repository-backed project memory storage and prompt injection. Semantic vector search, procedural learning, import/export, and long-term compaction workflows are not yet guaranteed by this capability.

#### Scenario: Do not assume semantic or procedural memory from this capability
- **WHEN** a caller relies on the current memory-system capability definition
- **THEN** only short-term memory and repository-backed project memory behaviors are guaranteed
