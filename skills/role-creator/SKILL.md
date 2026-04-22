---
name: Role Creator
description: Use when designing, generating, or provisioning AgentForge Role manifests for the project's multi-surface workspace. Covers role gap analysis, manifest authoring, inheritance design, skill binding, API-based creation, batch provisioning, and governance checks.
tools:
  - code_editor
  - terminal
---

# Role Creator

Design and provision AgentForge roles that match the project's actual architecture and team workflow. A well-designed role reduces repetitive setup and makes agent behavior predictable across surfaces.

## When to Use

- Bootstrapping roles for a new project or team onboarding.
- Adding a specialized role after a new surface or backend module lands.
- Refactoring existing roles to use inheritance or new skill bindings.
- Auditing the role library against the current codebase to find gaps.
- Batch-provisioning roles from a manifest directory or JSON array.

## AgentForge Architecture Context

AgentForge is a multi-surface workspace with these major seams:

| Surface | Tech Stack | Key Paths |
|---------|-----------|-----------|
| Web Frontend | Next.js 16 (React 19), Tailwind CSS v4, shadcn/ui | `app/`, `components/`, `lib/`, `hooks/` |
| Desktop | Tauri 2.9 (Rust) wrapping Next.js | `src-tauri/` |
| Bridge | TypeScript/Bun (Hono) — agent runtime | `src-bridge/` |
| IM Bridge | TypeScript/Bun — messaging integration | `src-im-bridge/` |
| Backend | Go (Echo) — orchestrator | `src-go/` |
| Marketplace | Go microservice — plugin/skill/role store | `src-marketplace/` |

Dashboard surfaces (each may need dedicated roles):
- `agents`, `employees`, `teams`, `reviews`, `cost`, `scheduler`
- `memory`, `roles`, `plugins`, `marketplace`, `settings`
- `im`, `docs`, `workflow`, `sprints`, `documents`, `skills`, `debug`
- `projects` (with VCS, secrets, qianchuan ads integrations)

Existing preset roles (do not duplicate without reason):
- `coding-agent` — general-purpose coding
- `bug-fixer` — extends coding-agent, diagnosis specialist
- `code-reviewer` — patch review and quality gates
- `default-code-fixer` — patch generator for review findings
- `doc-writer` — technical documentation
- `frontend-developer` — extends coding-agent, React/Next.js specialist
- `planner-agent` — task planning and breakdown
- `refactorer` — structural code improvement
- `test-engineer` — extends coding-agent, QA and test authoring

## Role Design Workflow

### 1. Analyze the Gap

Before creating a role, read the current state:
- List existing roles via `GET /api/v1/roles`.
- Identify the target surface (e.g., "Go backend workflow engine" or "IM bridge message handlers").
- Check whether an existing role can be extended instead of creating a new one.

### 2. Choose Inheritance or Standalone

- **Standalone** (`extends` omitted): Use for fundamentally different domains (e.g., a DevOps operator vs. a frontend developer).
- **Extension** (`extends: coding-agent` or similar): Use for specializations that share base tools and skills but need tighter constraints or extra knowledge.

Inheritance rules:
- Child overrides merge deeply with parent; explicit child values win.
- Cycles are rejected by the registry.
- Use `overrides` for single-field surgical changes without repeating the whole parent section.

### 3. Bind Skills

Available built-in runtime skills (check `/api/v1/roles/skills` for the live catalog):
- `skills/typescript` — TypeScript contract safety
- `skills/react` — React/Next.js surface work
- `skills/testing` — Regression coverage and verification
- `skills/css-animation` — Motion and transitions

Binding strategy:
- `auto_load: true` for skills the role cannot function without.
- `auto_load: false` for skills that are useful but optional (on-demand).
- Reference skills by canonical path (e.g., `skills/react`).

### 4. Author the Manifest

Follow the `agentforge/v1` schema. The manifest can be authored in YAML or JSON.

#### Minimal JSON Example

```json
{
  "apiVersion": "agentforge/v1",
  "kind": "Role",
  "metadata": {
    "id": "backend-engineer",
    "name": "Backend Engineer",
    "version": "1.0.0",
    "description": "Go backend development specialist",
    "author": "AgentForge",
    "tags": ["backend", "go"]
  },
  "identity": {
    "role": "Senior Backend Engineer",
    "goal": "Build reliable, performant Go services",
    "backstory": "Expert in Go, PostgreSQL, Redis, and distributed systems.",
    "systemPrompt": "You are a Go backend engineer. Follow clean architecture principles. Write tests for all handlers and services. Use Echo framework patterns established in this codebase.",
    "persona": "Disciplined backend specialist",
    "goals": ["Keep handler layer thin", "Write table-driven tests"],
    "constraints": ["No business logic in handlers", "Always use parameterized queries"]
  },
  "capabilities": {
    "toolConfig": {
      "builtIn": ["Read", "Edit", "Write", "Bash", "Glob", "Grep"]
    },
    "skills": [],
    "languages": ["go"],
    "frameworks": ["echo"],
    "maxTurns": 30,
    "maxBudgetUsd": 5.0
  },
  "knowledge": {
    "repositories": ["src-go"],
    "documents": ["docs/PRD.md", "CLAUDE.md"],
    "patterns": ["layered-architecture", "repository-pattern"]
  },
  "security": {
    "permissionMode": "bypassPermissions",
    "allowedPaths": ["src-go/", "docs/"],
    "outputFilters": ["no_credentials", "no_pii"]
  }
}
```

#### Rich YAML Example (with all sections)

```yaml
apiVersion: agentforge/v1
kind: Role
metadata:
  id: frontend-developer
  name: Frontend Developer
  version: "1.0.0"
  author: AgentForge
  tags: [frontend, react, nextjs]
  description: Frontend development specialist for React and Next.js
extends: coding-agent
identity:
  role: Senior Frontend Developer
  goal: Build responsive, accessible, and performant UI components and pages
  backstory: You are a frontend engineer expert in React, Next.js, Tailwind CSS, and modern web standards.
  persona: Collaborative frontend specialist
  goals:
    - Keep product UI consistent
    - Improve dashboard authoring ergonomics
  constraints:
    - Reuse the existing design system before adding new primitives
    - Preserve accessibility and responsive behavior
  personality: patient
  language: zh-CN
  responseStyle:
    tone: professional
    verbosity: concise
    formatPreference: markdown
systemPrompt: |
  You are a frontend developer. Follow these practices:
  - Use the project's component library and design system
  - Write accessible markup with semantic HTML
  - Use Tailwind CSS with existing utilities and tokens
  - Implement responsive layouts that work across screen sizes
  - Prefer React Server Components when possible
  - Follow Next.js App Router conventions
capabilities:
  packages:
    - web-development
    - design-system
  toolConfig:
    builtIn: [Read, Edit, Write, Bash, Glob, Grep]
    external:
      - figma
    mcpServers:
      - name: design-mcp
        url: http://localhost:3010/mcp
  customSettings:
    approval_mode: guided
    review_depth: ui-critical
  skills:
    - path: skills/react
      autoLoad: true
    - path: skills/css-animation
      autoLoad: false
  languages: [typescript]
  frameworks: [nextjs, react]
  maxTurns: 30
  maxBudgetUsd: 5.0
knowledge:
  repositories:
    - app
    - components
    - lib
  documents:
    - docs/PRD.md
    - docs/part/PLUGIN_SYSTEM_DESIGN.md
  patterns:
    - responsive-layouts
    - dashboard-shell
  shared:
    - id: design-guidelines
      type: vector
      access: read
      description: Shared UI design guidance for AgentForge product surfaces
      sources:
        - docs/PRD.md
        - docs/part/PLUGIN_SYSTEM_DESIGN.md
  private:
    - id: frontend-playbook
      type: doc
      access: read
      description: Private implementation notes for frontend delivery.
      sources:
        - docs/role-authoring-guide.md
  memory:
    shortTerm:
      maxTokens: 64000
    episodic:
      enabled: true
      retentionDays: 30
    semantic:
      enabled: true
      autoExtract: true
security:
  profile: standard
  permissionMode: bypassPermissions
  allowedPaths: ["app/", "components/", "lib/", "styles/", "public/"]
  deniedPaths: ["src-go/", "src-tauri/"]
  outputFilters:
    - no_credentials
    - no_pii
  permissions:
    fileAccess:
      allowedPaths: ["app/", "components/", "lib/"]
      deniedPaths: ["src-go/", "src-tauri/"]
    network:
      allowedDomains: ["localhost", "api.agentforge.local"]
    codeExecution:
      sandbox: true
      allowedLanguages: ["javascript", "typescript"]
  resourceLimits:
    tokenBudget:
      perTask: 100000
      perDay: 500000
    costLimit:
      perTask: "$5.00"
      alertThreshold: 0.8
collaboration:
  canDelegateTo:
    - code-reviewer
  acceptsDelegationFrom:
    - coding-agent
  communication:
    preferredChannel: structured
    reportFormat: markdown
triggers:
  - event: pr_created
    action: auto_review
    condition: "labels.includes('frontend')"
    autoExecute: true
    requiresApproval: false
overrides:
  identity.role: Principal Frontend Developer
```

Key fields to get right:
- `metadata.id` — kebab-case, unique across the deployment.
- `identity.systemPrompt` — the primary behavioral contract; be explicit about conventions and boundaries.
- `capabilities.toolConfig.builtIn` — match the agent's actual needs; do not grant `Bash` unless necessary.
- `security.allowedPaths` — scope file access to the relevant directories.
- `knowledge.documents` — point to PRDs, design docs, or API specs the agent must respect.

For the complete field schema, read [references/role-manifest-schema.md](references/role-manifest-schema.md).

### 5. Validate Locally

Before hitting the API, run the local manifest validator:

```bash
bun run skills/role-creator/scripts/verify-manifest.ts path/to/role.yaml
```

This checks:
- Required fields (`apiVersion`, `kind`, `metadata.id`, `metadata.name`, etc.)
- Kebab-case conformance for IDs
- Array type correctness for `repositories`, `documents`, `patterns`, `allowedPaths`
- Inheritance reference validity
- Overrides type validity

### 6. Preview Before Provisioning

Use the preview endpoint to catch issues before saving:

```bash
# Preview a new role
curl -X POST http://localhost:7777/api/v1/roles/preview \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d @role-manifest.json

# Preview an existing role with draft changes
curl -X POST http://localhost:7777/api/v1/roles/preview \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "roleId": "coding-agent",
    "draft": { "capabilities": { "maxTurns": 50 } }
  }'
```

Preview response fields:
- `normalizedManifest` — canonical form after server-side normalization
- `effectiveManifest` — fully resolved form after inheritance
- `executionProfile` — runtime-ready compiled profile
- `validationIssues` — field-level validation errors
- `inheritance.parentRoleId` — confirmed parent role
- `readinessDiagnostics` — skill compatibility and runtime readiness issues

Check for:
- Inheritance resolution errors
- Missing skill dependencies
- Tool compatibility warnings
- Validation issues in the response

### 7. Create the Role

```bash
curl -X POST http://localhost:7777/api/v1/roles \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d @role-manifest.json
```

On success, returns HTTP 201 with the enriched manifest (resolved inheritance, plugin dependencies).

### 8. Verify After Creation

```bash
# Fetch the created role
curl http://localhost:7777/api/v1/roles/backend-engineer \
  -H "Authorization: Bearer $TOKEN"

# Check references (who is using this role)
curl http://localhost:7777/api/v1/roles/backend-engineer/references \
  -H "Authorization: Bearer $TOKEN"

# Run sandbox probe with a realistic task
curl -X POST http://localhost:7777/api/v1/roles/sandbox \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "roleId": "backend-engineer",
    "input": "Write a new Echo handler for listing workflow templates",
    "runtime": "claude_code",
    "provider": "anthropic",
    "model": "claude-sonnet-4-6"
  }'
```

## Complete API Reference

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/roles` | List all roles with inheritance resolved |
| GET | `/api/v1/roles/skills` | List available skill catalog entries |
| GET | `/api/v1/roles/:id` | Get single role by ID |
| GET | `/api/v1/roles/:id/references` | Get role reference inventory (consumers) |
| POST | `/api/v1/roles` | Create new role |
| PUT | `/api/v1/roles/:id` | Update existing role (preserves absent fields) |
| DELETE | `/api/v1/roles/:id` | Delete role (blocked if references exist) |
| POST | `/api/v1/roles/preview` | Preview normalized/effective manifest + execution profile |
| POST | `/api/v1/roles/sandbox` | Sandbox probe with runtime catalog + model generation |

All endpoints require Bearer token authentication except in unauthenticated dev mode.

## Frontend Integration

The frontend uses `useRoleStore` (`lib/stores/role-store.ts`) which wraps these APIs:

```typescript
import { useRoleStore } from "@/lib/stores/role-store";

const { createRole, previewRole, sandboxRole, fetchRoles } = useRoleStore();

// Create a role
await createRole({
  apiVersion: "agentforge/v1",
  kind: "Role",
  metadata: { id: "my-role", name: "My Role", version: "1.0.0", description: "...", author: "AgentForge", tags: [] },
  identity: { systemPrompt: "...", persona: "", goals: [], constraints: [] },
  capabilities: { toolConfig: { builtIn: ["Read", "Edit", "Write"] }, languages: [], frameworks: [] },
  knowledge: { repositories: [], documents: [], patterns: [] },
  security: { permissionMode: "bypassPermissions", allowedPaths: ["src/"] }
});

// Preview before saving
const preview = await previewRole({ draft: partialManifest });
if (preview.validationIssues?.length > 0) {
  console.warn("Validation issues:", preview.validationIssues);
}

// Sandbox test
const sandbox = await sandboxRole({
  roleId: "my-role",
  input: "Write a function that..."
});
```

## Batch Provisioning

For creating multiple roles at once, prepare a JSON array and script the creation:

```bash
#!/usr/bin/env bash
TOKEN="your-jwt-token"
API="http://localhost:7777/api/v1/roles"

for manifest in roles/provisioning/*.json; do
  echo "Creating role from $manifest..."
  response=$(curl -s -w "\n%{http_code}" -X POST "$API" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d @$manifest)
  http_code=$(echo "$response" | tail -n1)
  body=$(echo "$response" | sed '$d')
  if [ "$http_code" = "201" ]; then
    echo "  Created: $(echo $body | jq -r '.metadata.id')"
  else
    echo "  Failed ($http_code): $(echo $body | jq -r '.message // .error // "unknown error"')"
  fi
done
```

## Guardrails

- Do not create a role for every page. Prefer broader surface roles (e.g., "Backend Engineer" over "Scheduler Page Agent").
- Keep `systemPrompt` focused on conventions and boundaries, not encyclopedic API docs.
- Reuse existing skills before inventing new ones; if a gap is real, note it for skill creation separately.
- Set `security.allowedPaths` narrowly. A frontend role does not need access to `src-go/`.
- When extending, only specify the delta. Avoid copy-pasting the parent's entire manifest.
- Preserve role `version` discipline: bump on behavioral or capability changes.
- Always preview before creating. A validation error in preview costs nothing; a broken role in production disrupts workflows.
- Check reference inventory before deleting. A role bound to employees, plugins, or queue entries cannot be safely removed.

## Error Handling

Common API errors and resolutions:

| Status | Code | Cause | Resolution |
|--------|------|-------|------------|
| 400 | `invalid_request_body` | Malformed JSON or YAML | Run local validator, check field types |
| 400 | `role_id_or_name_required` | Missing `metadata.id` or `metadata.name` | Ensure both fields are present |
| 400 | `failed_to_save_role` | Store-level validation failure | Check preview response for details |
| 400 | `role_inheritance_cycle` | Circular `extends` chain | Fix inheritance graph |
| 404 | `role_not_found` | Role ID does not exist | Check spelling or create first |
| 409 | `role_has_references` | Role is bound to consumers | Unbind consumers first, or force delete |
| 500 | `failed_to_reload_role` | Inheritance or skill resolution error | Check parent role and skill paths |

## Verification Checklist

After creating or updating a role:

- [ ] Role appears in `GET /api/v1/roles` with correct `metadata.id`
- [ ] `GET /api/v1/roles/{id}` returns resolved manifest (inheritance applied)
- [ ] Preview shows zero validation issues
- [ ] Execution profile includes expected tools and skills
- [ ] Skill diagnostics show no blocking compatibility errors
- [ ] Sandbox probe produces contextually appropriate output
- [ ] Role appears in frontend RoleWorkspace catalog without errors
- [ ] Reference inventory is empty (for new roles) or expected (for updates)

## References

- Read [references/agentforge-role-patterns.md](references/agentforge-role-patterns.md) for canonical inheritance patterns, skill binding recipes, and surface-to-role mapping guidance.
- Read [references/role-manifest-schema.md](references/role-manifest-schema.md) for the complete field-level schema reference with all sub-objects and types.
