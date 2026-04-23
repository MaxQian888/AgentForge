---
title: "Spec 7: Data Export / Import"
date: 2026-04-23
status: draft
depends_on: [1]
---

# Data Export / Import

## Problem

AgentForge supports only cost CSV export. There is no way to export projects, tasks, configurations, or any other entity in bulk. No import mechanism exists for migrating from other tools (Jira, Linear, Trello) or restoring from backup.

## Current State

- Cost CSV export: `components/cost/cost-csv-export.tsx`
- No other export capabilities
- No import of any kind
- No backup/restore functionality

## Design

### Export Architecture

**Format: JSON bundles + optional ZIP archive**

An export is a JSON manifest referencing entity data files, packaged as a ZIP:

```
agentforge-export-2026-04-23.zip
├── manifest.json              — export metadata, version, entity counts
├── projects.json              — project definitions
├── tasks.json                 — all tasks with comments, custom fields
├── sprints.json               — sprint definitions
├── workflows.json             — workflow definitions
├── roles.json                 — role definitions
├── knowledge/                 — wiki pages and documents
│   ├── pages.json
│   └── assets/                — file attachments
├── automations.json           — automation rules
└── settings.json              — project settings, custom fields, saved views
```

### Export Types

| Type | Scope | Content |
|------|-------|---------|
| `project` | Single project | All project data |
| `org` | Entire org | All org projects + org settings |
| ` selective` | Custom selection | User-picked entity types |

### Export API

```
# Trigger export
POST /api/v1/orgs/:orgId/exports
Body: {
  "type": "project" | "org" | "selective",
  "projectId": "uuid",           // for project type
  "entities": ["tasks", "wiki"], // for selective type
  "format": "json" | "csv",      // csv for tabular data only (tasks, sprints)
  "includeAttachments": true
}

# Response (async — export can be large)
{ "exportId": "uuid", "status": "processing" }

# Poll status
GET  /api/v1/orgs/:orgId/exports/:exportId

# Download when ready
GET  /api/v1/orgs/:orgId/exports/:exportId/download
```

**Export processing:**
1. Create export record with `processing` status
2. Background goroutine collects entities, writes to temp directory
3. ZIP the directory
4. Upload to storage (local filesystem or S3)
5. Update export record with `completed` status and download URL
6. Send notification to requesting user

### Import Architecture

**Source formats:**
1. **AgentForge bundle** — native format (from export)
2. **Jira** — CSV export from Jira
3. **Linear** — CSV export from Linear
4. **Trello** — JSON export from Trello
5. **GitHub Issues** — CSV export
6. **Generic CSV** — column-mapped import

### Import API

```
# Upload import file
POST /api/v1/orgs/:orgId/imports/upload
Body: multipart/form-data with file
Response: { "importId": "uuid", "sourceType": "auto-detected" }

# Configure mapping (after upload)
PUT /api/v1/orgs/:orgId/imports/:importId/mapping
Body: {
  "targetProjectId": "uuid",     // existing project or null to create
  "newProjectName": "Migrated Project",
  "fieldMapping": {              // source field → target field
    "Summary": "title",
    "Description": "description",
    "Status": "status",
    "Assignee": "assignee_email"
  },
  "statusMapping": {             // source status → AgentForge status
    "To Do": "inbox",
    "In Progress": "in_progress",
    "Done": "done"
  },
  "options": {
    "skipDuplicates": true,
    "dryRun": false
  }
}

# Preview import (dry run)
POST /api/v1/orgs/:orgId/imports/:importId/preview
Response: { "totalRows": 150, "validRows": 145, "errors": [...], "warnings": [...] }

# Execute import
POST /api/v1/orgs/:orgId/imports/:importId/execute
Response: { "created": 145, "skipped": 5, "errors": 0 }

# Get import status
GET /api/v1/orgs/:orgId/imports/:importId
```

### Import Flow

```
Upload file → Auto-detect source format → Show field mapping UI
  → User maps fields → Preview (dry run) → Show errors/warnings
  → User confirms → Execute import → Report results
```

### Field Mapping UI

The frontend shows a mapping interface:

```
┌─────────────────────────────────────────────────┐
│ Import from Jira CSV (150 rows)                  │
├─────────────────────────────────────────────────┤
│ Source Field    →  AgentForge Field              │
│ ─────────────────────────────────────────────── │
│ Summary        →  Task Title        [✓ mapped]  │
│ Description    →  Description       [✓ mapped]  │
│ Status         →  Status            [✓ mapped]  │
│ Assignee       →  Assignee (email)  [✓ mapped]  │
│ Priority       →  Custom: Priority  [▼ pick]    │
│ Labels         →  Tags              [▼ pick]    │
│ Story Points   →  ——— skip ———      [▼ pick]    │
├─────────────────────────────────────────────────┤
│ Status Mapping:                                  │
│ "To Do"        →  inbox              [▼ pick]   │
│ "In Progress"  →  in_progress        [▼ pick]   │
│ "Done"         →  done               [▼ pick]   │
│ "Blocked"      →  blocked            [▼ pick]   │
├─────────────────────────────────────────────────┤
│ Preview: 145 valid · 5 errors · 0 warnings      │
│                                    [Import]      │
└─────────────────────────────────────────────────┘
```

### Data Model

```sql
CREATE TABLE exports (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID NOT NULL REFERENCES organizations(id),
  user_id     UUID NOT NULL REFERENCES users(id),
  type        VARCHAR(16) NOT NULL,    -- project, org, selective
  status      VARCHAR(16) NOT NULL DEFAULT 'processing',
  config      JSONB NOT NULL DEFAULT '{}',
  file_path   TEXT,
  file_size   BIGINT,
  error       TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  expires_at  TIMESTAMPTZ NOT NULL     -- auto-cleanup after 7 days
);

CREATE TABLE imports (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID NOT NULL REFERENCES organizations(id),
  user_id     UUID NOT NULL REFERENCES users(id),
  source_type VARCHAR(32) NOT NULL,    -- agentforge, jira, linear, trello, github, csv
  status      VARCHAR(16) NOT NULL DEFAULT 'uploaded',
  file_path   TEXT NOT NULL,
  mapping     JSONB,
  config      JSONB NOT NULL DEFAULT '{}',
  result      JSONB,
  error       TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);
```

### Frontend

**New pages:**
- `/orgs/:orgId/data` — data management hub (export/import tabs)
- Or integrated into Org Settings → Data tab

**New components:**
- `ExportDialog` — select type, entities, download
- `ImportWizard` — multi-step: upload → detect → map → preview → execute
- `FieldMapper` — drag-and-drop field mapping interface
- `ImportProgress` — progress bar with per-entity counts

### Compliance & Security

- Exports are scoped to org; users can only export data they can read
- Imports require org admin permission
- Export files expire and are auto-deleted after 7 days
- All export/import actions are logged to audit trail
- Imported data inherits org's default permissions
- Duplicate detection: tasks with same title+project are flagged, not auto-skipped

### Testing

- Unit: export serialization, import parsing per source format, field mapping
- Integration: full export → import roundtrip (verify data fidelity)
- E2E: export project → create new project → import → verify all entities present
- Edge cases: circular task dependencies, missing fields, encoding issues, large files
