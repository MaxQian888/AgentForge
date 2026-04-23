---
title: "Spec 9: Multi-Agent Collaboration"
date: 2026-04-23
status: draft
---

# Multi-Agent Collaboration

## Problem

AgentForge supports individual agent execution but lacks multi-agent collaboration patterns. The PRD describes a Planner/Coder/Reviewer model where multiple agents work together on a task. Currently, each agent operates independently with no inter-agent communication, shared context, or coordinated workflow.

## Current State

- Agent spawning: `POST /agents/spawn` creates a single agent with a task
- Agent pool: `GET /agents/pool` shows running agents
- Agent teams: `POST /teams/start` creates a team with multiple agents
- But: teams are fire-and-forget — no coordination protocol, no message passing between agents, no shared working memory
- Bridge handles individual agent sessions via ACP

## Design

### Collaboration Patterns

Three patterns, each increasing in complexity:

**Pattern 1: Pipeline (sequential)**
```
Planner → Coder → Reviewer
  Task decomposition → Implementation → Quality check
```
Each agent's output feeds into the next agent's input. Simple, predictable.

**Pattern 2: Parallel (concurrent)**
```
       ┌→ Coder A (frontend)
Task → ├→ Coder B (backend)
       └→ Coder C (tests)
All results → Reviewer
```
Agents work independently on sub-tasks, results are merged and reviewed.

**Pattern 3: Hierarchical (manager-worker)**
```
Manager Agent
  ├── Worker A (research)
  ├── Worker B (implementation)
  └── Worker C (review)
Manager coordinates, reassigns, and synthesizes
```
A manager agent orchestrates workers, can dynamically assign and reassign sub-tasks.

### Data Model

```sql
-- Collaboration sessions
CREATE TABLE collab_sessions (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID REFERENCES organizations(id),
  project_id  UUID NOT NULL REFERENCES projects(id),
  task_id     UUID REFERENCES tasks(id),     -- parent task, nullable for standalone
  pattern     VARCHAR(16) NOT NULL,          -- pipeline, parallel, hierarchical
  status      VARCHAR(16) NOT NULL DEFAULT 'planning',
  config      JSONB NOT NULL DEFAULT '{}',   -- pattern-specific config
  shared_context JSONB DEFAULT '{}',         -- shared working memory
  created_by  UUID NOT NULL REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);

-- Agent assignments within a collaboration
CREATE TABLE collab_agents (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id      UUID NOT NULL REFERENCES collab_sessions(id),
  agent_run_id    UUID REFERENCES agent_runs(id),
  role            VARCHAR(32) NOT NULL,      -- planner, coder, reviewer, manager, worker
  parent_agent_id UUID REFERENCES collab_agents(id), -- for hierarchical: parent manager
  input_from      UUID[],                    -- agent IDs whose output feeds into this agent
  output_to       UUID[],                    -- agent IDs that receive this agent's output
  status          VARCHAR(16) NOT NULL DEFAULT 'pending',
  config          JSONB DEFAULT '{}',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Inter-agent messages
CREATE TABLE collab_messages (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id  UUID NOT NULL REFERENCES collab_sessions(id),
  from_agent  UUID NOT NULL REFERENCES collab_agents(id),
  to_agent    UUID REFERENCES collab_agents(id), -- nullable = broadcast
  type        VARCHAR(32) NOT NULL,     -- status_update, output, request, error, coordination
  payload     JSONB NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Orchestration Engine

The Go orchestrator manages collaboration sessions:

```go
type CollaborationEngine struct {
    sessionRepo    repository.CollabSessionRepo
    agentRepo      repository.CollabAgentRepo
    messageRepo    repository.CollabMessageRepo
    bridgeClient   bridge.Client
    eventBus       *eventbus.EventBus
}

// StartSession creates a collaboration and begins execution
func (e *CollaborationEngine) StartSession(ctx context.Context, config CollabConfig) (*CollabSession, error)

// HandleAgentComplete is called when an agent finishes
func (e *CollaborationEngine) HandleAgentComplete(ctx context.Context, agentID uuid.UUID, result AgentResult) error

// RouteMessage passes output from one agent to the next
func (e *CollaborationEngine) RouteMessage(ctx context.Context, msg CollabMessage) error
```

**Pipeline execution:**
1. Start planner agent with original task
2. Planner completes → output becomes input for coder
3. Coder completes → output becomes input for reviewer
4. Reviewer completes → final result, session marked complete

**Parallel execution:**
1. Decompose task into N sub-tasks (manually or via planner)
2. Start N coder agents concurrently
3. Wait for all to complete (with timeout)
4. Merge results → start reviewer agent

**Hierarchical execution:**
1. Start manager agent with task
2. Manager decides sub-tasks and spawns workers via bridge
3. Workers report back to manager through message queue
4. Manager can reassign, add workers, or complete early
5. Manager produces final result

### Shared Context

Agents in a collaboration share a context object:

```json
{
  "taskDescription": "Implement user authentication",
  "projectContext": { "techStack": "Next.js + Go", "conventions": "..." },
  "previousOutputs": {
    "planner": { "plan": "1. Create auth middleware\n2. Add login endpoint\n3. ..." }
  },
  "decisions": [
    { "by": "planner", "decision": "Use JWT with refresh tokens", "rationale": "..." }
  ],
  "artifacts": {
    "files_created": ["auth/middleware.go", "auth/handler.go"],
    "files_modified": ["router.go"]
  }
}
```

Each agent reads the shared context at start and writes its output back. The orchestration engine manages context propagation between pipeline stages.

### Bridge Integration

The bridge's ACP protocol handles individual agent sessions. For collaboration:

1. Each agent in the collaboration gets its own ACP session
2. The orchestrator injects shared context into each agent's initial prompt
3. Agent outputs are captured by the orchestrator (not just returned to user)
4. The orchestrator routes outputs between agents via `collab_messages`

**Bridge protocol extension:**
```
POST /bridge/collab/start      — Start a multi-agent collaboration
GET  /bridge/collab/:id/status — Get collaboration status
POST /bridge/collab/:id/input  — Inject input into a specific agent
```

### API Endpoints

```
# Collaboration CRUD
POST   /api/v1/projects/:pid/collabs                — Start collaboration
GET    /api/v1/projects/:pid/collabs                — List collaborations
GET    /api/v1/collabs/:id                          — Get collaboration detail
POST   /api/v1/collabs/:id/cancel                   — Cancel collaboration
GET    /api/v1/collabs/:id/agents                   — List agents in collaboration
GET    /api/v1/collabs/:id/messages                 — Message log
GET    /api/v1/collabs/:id/context                  — Shared context

# Templates (predefined patterns)
GET    /api/v1/collab-templates                     — List templates
POST   /api/v1/collab-templates                     — Create template
```

### Frontend

**New pages:**
- Collaboration detail view (within task detail or standalone)
- Shows agent graph visualization (pipeline/parallel/hierarchy diagram)

**New components:**
- `CollabGraph` — visual representation of agent relationships (using `@xyflow/react`)
- `CollabStatusPanel` — real-time status of each agent in the collaboration
- `CollabConfigDialog` — pattern selection, role assignment, parameter config
- `CollabMessageFeed` — inter-agent message log

**Modified:**
- Task detail page → show collaboration if task has one
- Agent workspace → indicate if agent is part of a collaboration
- Team management → add collaboration pattern selection

### Testing

- Unit: orchestration logic for each pattern, context propagation, message routing
- Integration: full pipeline (planner → coder → reviewer) with mock bridge
- E2E: create task → start pipeline collaboration → watch agents execute → verify result
- Edge: agent failure mid-pipeline (retry logic), timeout handling, partial completion
