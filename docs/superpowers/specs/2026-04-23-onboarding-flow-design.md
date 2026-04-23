---
title: "Spec 8: Onboarding Flow"
date: 2026-04-23
status: draft
---

# Onboarding Flow

## Problem

AgentForge has basic registration/login but no first-run experience. New users land on an empty dashboard with no guidance. There are no sample projects, no interactive tutorials, and no contextual help. Enterprise onboarding (new employee joins an org) has no guided setup.

## Current State

- Registration creates an empty account
- Login redirects to dashboard with no data
- Project creation wizard exists but is not discoverable
- No tooltip/tour system
- No sample/demo data

## Design

### Onboarding Tiers

**Tier 1: Platform onboarding** (new user, no org)
1. Welcome screen with role selection (developer, PM, team lead, admin)
2. Create or join organization
3. Guided project creation with template suggestions
4. Quick tour of key features (3-5 steps)

**Tier 2: Org onboarding** (invited to existing org)
1. Welcome screen showing org name and inviter
2. Profile completion (display name, avatar)
3. Existing org projects listed with "star" recommendations
4. Brief tour of org-specific features

**Tier 3: Project onboarding** (first time in a project)
1. Project overview card (what this project does, active members)
2. Suggested first actions based on project state
3. Mini-tour of project-specific features

### Onboarding Data Model

```sql
CREATE TABLE onboarding_state (
  user_id     UUID PRIMARY KEY REFERENCES users(id),
  role        VARCHAR(32),                  -- selected role
  current_step VARCHAR(64),                 -- e.g. "create_org", "tour_dashboard"
  completed_steps JSONB DEFAULT '[]',
  dismissed   BOOLEAN DEFAULT false,        -- user skipped onboarding
  completed_at TIMESTAMPTZ,
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Welcome Wizard Flow

```
┌──────────────────────────────────────────┐
│  Welcome to AgentForge!                  │
│                                          │
│  What best describes your role?          │
│                                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│  │ Developer │ │   PM     │ │Team Lead │ │
│  │  focus:   │ │  focus:  │ │  focus:  │ │
│  │  coding,  │ │  tasks,  │ │  team,   │ │
│  │  reviews  │ │  sprints │ │  reports │ │
│  └──────────┘ └──────────┘ └──────────┘ │
│                                          │
│                    [Continue]             │
└──────────────────────────────────────────┘
```

**Step 2: Org setup**
- "Create a new organization" → org creation form (simplified)
- "Join an existing org" → enter invite code or wait for invitation
- "Skip for now" → personal workspace (no org)

**Step 3: First project**
- Show project templates based on role selection:
  - Developer: "Code Review Bot", "Bug Fix Agent", "Feature Branch Workflow"
  - PM: "Sprint Board", "Task Decomposition Pipeline"
  - Team Lead: "Team Dashboard", "Multi-Agent Sprint"
- Or blank project with name + description

**Step 4: Feature tour** (3-5 interactive highlights)
- Highlight: "This is your dashboard — you'll see agent activity here"
- Highlight: "Go to Agents to spawn your first AI coding agent"
- Highlight: "Check Settings to configure your coding agent"
- Each highlight has a "Got it" button and "Skip tour"

### Contextual Help System

Beyond the initial wizard, contextual help provides ongoing guidance:

**Tooltips:** First time a user sees a complex component, show a tooltip:
- Agent workspace: "Agents are AI employees. Spawn one to start a coding task."
- Workflow editor: "Drag nodes to build automation flows."
- Cost page: "Track how much each agent run costs in tokens and USD."

**Empty states:** When a page has no data, show actionable guidance:
```
┌──────────────────────────────────────────┐
│  No agents yet                           │
│                                          │
│  Agents are AI employees that execute    │
│  coding tasks. Create your first agent   │
│  to get started.                         │
│                                          │
│  [Spawn Your First Agent]                │
└──────────────────────────────────────────┘
```

**Help center link:** "?" icon in the bottom-right corner opens contextual help panel.

### Sample Data

For new personal workspace (no org), optionally seed:

```json
{
  "project": {
    "name": "My First Project",
    "description": "A sample project to explore AgentForge features"
  },
  "tasks": [
    { "title": "Try creating a task", "status": "inbox", "description": "Create a new task and assign it to an agent" },
    { "title": "Spawn your first agent", "status": "inbox", "description": "Go to Agents page and spawn a coding agent" },
    { "title": "Explore the workflow editor", "status": "inbox", "description": "Visit Workflows to see automation templates" }
  ]
}
```

Sample data is opt-in at onboarding ("Start with sample data?" checkbox).

### API Endpoints

```
GET    /api/v1/onboarding                — Get onboarding state
PUT    /api/v1/onboarding                — Update step
POST   /api/v1/onboarding/dismiss        — Skip onboarding
POST   /api/v1/onboarding/complete       — Mark finished
POST   /api/v1/onboarding/seed-data      — Create sample project + tasks
```

### Frontend Components

**New files:**
- `components/onboarding/welcome-wizard.tsx` — multi-step wizard
- `components/onboarding/role-selector.tsx` — role picking
- `components/onboarding/org-setup.tsx` — org creation/join
- `components/onboarding/project-setup.tsx` — first project with templates
- `components/onboarding/feature-tour.tsx` — interactive tour overlay
- `components/onboarding/contextual-tooltip.tsx` — smart tooltip system
- `lib/stores/onboarding-store.ts` — onboarding state management

**Modified:**
- Post-registration redirect → onboarding wizard (if not completed)
- Post-login redirect → onboarding wizard (if not completed and not dismissed)
- `app/(dashboard)/layout.tsx` — check onboarding state, show contextual help
- All empty-state components → add actionable guidance

### Testing

- Unit: onboarding state transitions, step completion logic
- Integration: seed data creation, role-based template suggestions
- E2E: register → welcome wizard → create org → create project → complete tour
- Accessibility: wizard keyboard navigation, focus management
