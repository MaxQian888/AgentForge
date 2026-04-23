---
title: "Spec 4: Global Search"
date: 2026-04-23
status: draft
depends_on: [1]
---

# Global Search

## Problem

AgentForge has domain-specific search (knowledge assets, plugins, marketplace) but no unified search across all entities. Enterprise users with dozens of projects, hundreds of tasks, and thousands of documents cannot find what they need without navigating to the correct page first.

## Current State

- Knowledge search: `GET /projects/:pid/knowledge/search`
- Plugin catalog search: `GET /plugins/catalog?q=`
- Marketplace search: frontend-side filter
- No cross-entity search
- No search UI outside individual pages

## Design

### Search Architecture

**Approach: PostgreSQL full-text search (FTS) with materialized index**

For enterprise scale, dedicated search infrastructure (Elasticsearch/Meilisearch) is the long-term target. But PostgreSQL FTS with a `search_index` table provides a working solution with zero additional infrastructure.

```sql
CREATE TABLE search_index (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type VARCHAR(32) NOT NULL,  -- project, task, agent, employee, wiki, document, role, plugin
  entity_id   UUID NOT NULL,
  org_id      UUID REFERENCES organizations(id),
  project_id  UUID REFERENCES projects(id),   -- nullable for org-level entities
  title       TEXT NOT NULL,
  body        TEXT,
  ts_vector   TSVECTOR GENERATED ALWAYS AS (
                 setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
                 setweight(to_tsvector('english', coalesce(body, '')), 'B')
               ) STORED,
  metadata    JSONB DEFAULT '{}',    -- type-specific fields for display
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_search_vector ON search_index USING GIN(ts_vector);
CREATE INDEX idx_search_org ON search_index(org_id) WHERE org_id IS NOT NULL;
CREATE INDEX idx_search_project ON search_index(project_id) WHERE project_id IS NOT NULL;
```

### Indexing

Each entity type has a indexer that populates `search_index`:

| Entity | Title | Body | Metadata |
|--------|-------|------|----------|
| Project | name | description | status, member_count |
| Task | title | description, comments (first 500 chars) | status, assignee, sprint |
| Agent/Employee | name | role description | status, runtime |
| Wiki Page | title | content (first 2000 chars) | space, project |
| Document | filename | extracted text (first 2000 chars) | type, project |
| Role | name | description | plugin_count |
| Plugin | name | description | type, status |
| Team | name | description | member_count, status |
| Sprint | name | goal | status, project |

**Index triggers:**
- Write-through: entity create/update/delete writes to `search_index` in the same transaction
- Backfill: one-time migration script to index all existing entities
- Reindex: admin endpoint to rebuild the entire index

### API

```
GET /api/v1/search?q=<query>&types=<entity_types>&org=<orgId>&project=<projectId>&limit=20&offset=0
```

**Query parameters:**
- `q` — search query (required, min 2 chars)
- `types` — comma-separated entity types to search (optional, defaults to all)
- `org` — restrict to org (optional)
- `project` — restrict to project (optional)
- `limit` — page size (default 20, max 50)
- `offset` — pagination offset

**Response:**

```json
{
  "query": "auth flow",
  "total": 47,
  "results": [
    {
      "entityType": "task",
      "entityId": "uuid-123",
      "title": "Implement OAuth2 auth flow",
      "snippet": "...add <em>OAuth2</em> <em>auth</em> <em>flow</em> to the login page...",
      "metadata": { "status": "in_progress", "assignee": "Alice", "projectName": "AgentForge" },
      "projectSlug": "agentforge",
      "updatedAt": "2026-04-23T10:00:00Z"
    }
  ],
  "facets": {
    "types": { "task": 23, "wiki": 12, "document": 8, "project": 4 },
    "projects": { "AgentForge": 30, "CLI Tool": 17 }
  }
}
```

### RBAC Integration

Search results are filtered by the user's permissions:
- User can only see entities in projects they're a member of
- Org members can see org-level entities (roles, plugins)
- Platform admins see everything

This filtering happens in the query: join `search_index` with the user's accessible project/org list.

### Frontend

**Search Trigger:**
- `Cmd/Ctrl + K` opens search palette (command-palette pattern using existing `cmdk` dependency)
- Search icon in header also opens palette
- Search bar in sidebar for persistent search

**Search Palette:**
```
┌─────────────────────────────────────────────┐
│ 🔍 Search everything...                     │
├─────────────────────────────────────────────┤
│ [All] Tasks Wiki Docs Projects Agents Teams │
├─────────────────────────────────────────────┤
│ 📋 Implement OAuth2 auth flow               │
│    Task · In Progress · AgentForge          │
│ 📄 Authentication Architecture              │
│    Wiki · AgentForge                        │
│ 📁 Auth Provider Integration                │
│    Document · AgentForge                    │
│ 🤖 Auth Review Agent                        │
│    Employee · Idle · AgentForge             │
├─────────────────────────────────────────────┤
│ ↑↓ Navigate  ↵ Open  Esc Close             │
└─────────────────────────────────────────────┘
```

**New files:**
- `components/search/search-palette.tsx` — CmdK palette
- `components/search/search-result-item.tsx` — individual result
- `lib/stores/search-store.ts` — search state, history, recent searches

**Modified files:**
- `app/(dashboard)/layout.tsx` — register CmdK shortcut, add search trigger
- Existing `cmdk` usage in command palette extended with search mode

### Search History

- Store last 20 searches per user in `localStorage`
- Recent searches shown when palette opens with empty query
- "Quick links" for frequently accessed entities (learned from click-through)

### Performance Considerations

- PostgreSQL FTS handles ~100K documents comfortably; beyond that, consider migrating to Meilisearch
- Index size grows with entity count; `search_index` is a denormalized table optimized for reads
- Search queries use `ts_rank` for relevance scoring with title-weighted boost
- Snippet generation via `ts_headline` for matched text highlighting

### Testing

- Unit: indexer functions for each entity type
- Integration: create entity → appears in search → update → reindexed → delete → removed
- E2E: CmdK → type query → see results → click → navigates to entity
- Performance: benchmark search latency with 10K, 50K, 100K indexed entities
- RBAC: verify user cannot search entities outside their projects
