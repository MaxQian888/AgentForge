---
title: "Spec 10: Cross-Session Memory"
date: 2026-04-23
status: draft
---

# Cross-Session Memory

## Problem

AgentForge agents start each session with no memory of previous work. They don't remember project conventions, past decisions, code style preferences, or prior debugging experiences. This makes every agent run start from scratch, wasting tokens and producing inconsistent results.

## Current State

- `memory` module exists: `internal/memory/` with CRUD, search, stats, export, cleanup
- Frontend: Memory explorer page (behind `MEMORY_EXPLORER` feature flag)
- But: memory is project-scoped, manually curated, and not injected into agent sessions
- No automatic memory capture from agent runs
- No memory relevance scoring or decay

## Design

### Memory Types

| Type | Description | Source | Retention |
|------|-------------|--------|-----------|
| `convention` | Code style, naming patterns, architecture decisions | Manual + auto-extracted from reviews | Permanent |
| `decision` | Technical decisions with rationale | Agent session summaries | Permanent |
| `experience` | What worked / didn't work (debugging, implementation) | Agent run outcomes | 90 days |
| `context` | Project facts (tech stack, dependencies, structure) | Auto-extracted from project analysis | Permanent |
| `preference` | User/org preferences for agent behavior | Manual configuration | Permanent |

### Data Model

Extend the existing `agent_memories` table:

```sql
-- Add columns to existing memories
ALTER TABLE agent_memories ADD COLUMN memory_type VARCHAR(32) NOT NULL DEFAULT 'context';
ALTER TABLE agent_memories ADD COLUMN source VARCHAR(32) NOT NULL DEFAULT 'manual';
-- 'manual', 'agent_run', 'review', 'project_analysis'
ALTER TABLE agent_memories ADD COLUMN relevance_score REAL NOT NULL DEFAULT 1.0;
ALTER TABLE agent_memories ADD COLUMN access_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_memories ADD COLUMN last_accessed_at TIMESTAMPTZ;
ALTER TABLE agent_memories ADD COLUMN expires_at TIMESTAMPTZ; -- nullable = permanent
ALTER TABLE agent_memories ADD COLUMN embedding VECTOR(1536); -- pgvector for semantic search

-- Agent run → memory linkage
CREATE TABLE agent_run_memories (
  agent_run_id UUID NOT NULL REFERENCES agent_runs(id),
  memory_id    UUID NOT NULL REFERENCES agent_memories(id),
  role         VARCHAR(16) NOT NULL, -- 'input' (read) or 'output' (written)
  PRIMARY KEY (agent_run_id, memory_id)
);

-- Memory tags for categorization
CREATE TABLE memory_tags (
  memory_id UUID NOT NULL REFERENCES agent_memories(id),
  tag       VARCHAR(64) NOT NULL,
  PRIMARY KEY (memory_id, tag)
);
```

### Memory Lifecycle

```
Capture → Index → Retrieve → Inject → Update → Decay
```

**1. Capture:**
- **Automatic from agent runs:** After each agent run, the orchestrator extracts:
  - Key decisions made (parsed from agent output)
  - Files created/modified (from git diff)
  - Errors encountered and solutions applied
  - Task completion summary
- **From reviews:** Review findings and decisions captured as `experience` memories
- **Manual:** Users can add memories via UI or API

**2. Index:**
- Content is tokenized and embedded using OpenAI embeddings API
- Embeddings stored in `agent_memories.embedding` column (pgvector)
- Tags extracted automatically from content (project name, language, framework)

**3. Retrieve:**
When an agent session starts, relevant memories are fetched:

```go
func (s *MemoryService) RetrieveForSession(ctx context.Context, params RetrieveParams) ([]Memory, error) {
    // 1. Keyword match on project + entity scope
    // 2. Semantic similarity via pgvector cosine distance
    // 3. Boost by relevance_score and recency
    // 4. Filter by memory_type and tags
    // 5. Return top-K (default 10, max 20)
}
```

**4. Inject:**
Memories are injected into the agent's initial prompt:

```
## Project Memory
The following are relevant memories from previous sessions:

### Convention
- This project uses TypeScript strict mode with explicit return types on all functions
- API responses follow the pattern `{ data: T, error?: string }`

### Decision (from session 2026-04-20)
- Chose Zustand over Redux for state management due to simpler API and smaller bundle

### Experience (from session 2026-04-19)
- The auth middleware had a race condition with concurrent refresh tokens; fixed by adding a mutex per user
```

**5. Update:**
- When a memory is used in a session, increment `access_count` and update `last_accessed_at`
- After a run, verify if captured memories conflict with existing ones → merge or update

**6. Decay:**
- `experience` memories expire after 90 days (configurable)
- Memories not accessed in 180 days get `relevance_score` halved
- Cleanup job (existing scheduler) handles expiration and low-relevance pruning

### Memory Capture from Agent Runs

In `agent_service.go`, after a run completes:

```go
func (s *AgentService) captureMemories(ctx context.Context, run *AgentRun) error {
    summary := s.extractSummary(run.Output)

    // Extract decisions
    for _, decision := range summary.Decisions {
        s.memoryService.Create(ctx, Memory{
            ProjectID: run.ProjectID,
            Type:      "decision",
            Source:    "agent_run",
            Content:   decision.Text,
            Tags:      decision.Tags,
        })
    }

    // Extract experiences
    for _, exp := range summary.Experiences {
        s.memoryService.Create(ctx, Memory{
            ProjectID: run.ProjectID,
            Type:      "experience",
            Source:    "agent_run",
            Content:   exp.Text,
            Tags:      exp.Tags,
            ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
        })
    }
}
```

### Semantic Search (pgvector)

When pgvector is available, memory retrieval uses hybrid search:

```sql
-- Hybrid: keyword + semantic, weighted combination
WITH keyword_results AS (
    SELECT id, content, relevance_score,
           ts_rank(ts_vector, plainto_tsquery($1)) AS text_rank
    FROM agent_memories
    WHERE project_id = $2 AND ts_vector @@ plainto_tsquery($1)
    ORDER BY text_rank DESC LIMIT 50
),
semantic_results AS (
    SELECT id, content, relevance_score,
           1 - (embedding <=> $3) AS similarity
    FROM agent_memories
    WHERE project_id = $2 AND embedding IS NOT NULL
    ORDER BY embedding <=> $3 LIMIT 50
)
SELECT id, content, relevance_score,
       COALESCE(text_rank, 0) * 0.4 + COALESCE(similarity, 0) * 0.6 AS combined_score
FROM (SELECT * FROM keyword_results UNION SELECT * FROM semantic_results) combined
ORDER BY combined_score DESC
LIMIT $4;
```

If pgvector is not installed, fall back to keyword-only search (existing FTS).

### API Endpoints

```
# Memory CRUD (existing, extended)
GET    /api/v1/projects/:pid/memory                   — Search memories (supports semantic)
POST   /api/v1/projects/:pid/memory                   — Create memory
PATCH  /api/v1/projects/:pid/memory/:mid              — Update memory
DELETE /api/v1/projects/:pid/memory/:mid              — Delete memory

# New endpoints
GET    /api/v1/projects/:pid/memory/types             — List memory types with counts
POST   /api/v1/projects/:pid/memory/capture/:runId    — Trigger manual capture from run
GET    /api/v1/projects/:pid/memory/stats             — Memory statistics
POST   /api/v1/projects/:pid/memory/reindex           — Rebuild embeddings
GET    /api/v1/agents/:id/memories                    — Memories used/produced by agent

# Employee-level memory scope
GET    /api/v1/projects/:pid/employees/:eid/memory    — Employee-scoped memories
```

### Frontend

**Enhanced Memory Explorer** (currently feature-flagged):
- Remove `MEMORY_EXPLORER` flag, make always available
- Add memory type filters and tag sidebar
- Show memory source badge (manual, agent run, review, analysis)
- Add "Capture from Run" button on agent run detail pages
- Show related memories on task and agent detail pages

**New components:**
- `MemoryTypeBadge` — colored badge for memory type
- `MemorySourceBadge` — source indicator
- `MemoryTimeline` — chronological view of memories
- `MemoryRelevanceScore` — visual indicator of memory relevance

### Testing

- Unit: memory capture extraction, relevance scoring, decay logic
- Integration: agent run → memory captured → next run retrieves → injected into prompt
- E2E: run agent on task → verify memory created → run again → verify memory used
- Performance: benchmark retrieval with 1K, 10K, 100K memories
- Embedding: verify pgvector fallback when extension not installed
