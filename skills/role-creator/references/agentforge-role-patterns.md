# AgentForge Role Patterns

Canonical patterns for designing roles in the AgentForge multi-surface workspace.

## Inheritance Patterns

### Base Specialist Pattern
A generalist base role with specialist children. Used when multiple roles share tools and conventions but differ in domain expertise.

```yaml
# Base: coding-agent
metadata:
  id: coding-agent
  name: Coding Agent
identity:
  role: Senior Software Engineer
capabilities:
  toolConfig:
    builtIn: [Read, Edit, Write, Bash, Glob, Grep]
  skills:
    - path: skills/typescript
      autoLoad: true
  maxTurns: 30

# Child: frontend-developer
extends: coding-agent
metadata:
  id: frontend-developer
  name: Frontend Developer
identity:
  role: Senior Frontend Developer
  goal: Build responsive, accessible, and performant UI
capabilities:
  packages:
    - web-development
    - design-system
  skills:
    - path: skills/react
      autoLoad: true
    - path: skills/css-animation
      autoLoad: false
overrides:
  identity.role: Principal Frontend Developer
```

### Task-Focused Pattern
A standalone role for a narrow, repeatable task that does not share behavior with coding roles.

```yaml
metadata:
  id: doc-writer
  name: Documentation Writer
identity:
  role: Technical Writer
  goal: Produce clear, accurate, and maintainable documentation
capabilities:
  toolConfig:
    builtIn: [Read, Edit, Write, Glob, Grep]
  maxTurns: 20
security:
  allowedPaths: ["docs/", "README.md", "CHANGELOG.md"]
```

## Surface-to-Role Mapping

| Surface | Suggested Role Base | Key Skills | Allowed Paths |
|---------|---------------------|------------|---------------|
| Web Frontend | `frontend-developer` | react, css-animation, typescript | `app/`, `components/`, `lib/`, `hooks/` |
| Go Backend | `coding-agent` + Go knowledge | typescript (for schemas) | `src-go/` |
| Bridge Runtime | `coding-agent` | typescript, testing | `src-bridge/` |
| IM Bridge | `coding-agent` | typescript, testing | `src-im-bridge/` |
| Docs/Wiki | `doc-writer` | — | `docs/` |
| Tauri Desktop | `frontend-developer` + Rust | react, typescript | `src-tauri/`, `app/`, `components/` |
| DevOps/CI | Standalone | — | `.github/`, `scripts/`, `docker/` |

## Skill Binding Recipes

### Frontend Recipe
```yaml
capabilities:
  skills:
    - path: skills/typescript
      autoLoad: true
    - path: skills/react
      autoLoad: true
    - path: skills/css-animation
      autoLoad: false
    - path: skills/testing
      autoLoad: false
```

### Backend Recipe
```yaml
capabilities:
  skills:
    - path: skills/typescript
      autoLoad: true
    - path: skills/testing
      autoLoad: false
```

### Full-Stack Recipe
```yaml
capabilities:
  skills:
    - path: skills/typescript
      autoLoad: true
    - path: skills/react
      autoLoad: false
    - path: skills/testing
      autoLoad: false
```

## Security Profiles

### Standard Web
```yaml
security:
  permissionMode: bypassPermissions
  allowedPaths: ["app/", "components/", "lib/", "styles/", "public/"]
  outputFilters: [no_credentials, no_pii]
```

### Backend
```yaml
security:
  permissionMode: bypassPermissions
  allowedPaths: ["src-go/", "src-bridge/", "src-im-bridge/", "src-marketplace/"]
  outputFilters: [no_credentials, no_pii]
```

### Restricted (docs only)
```yaml
security:
  permissionMode: bypassPermissions
  allowedPaths: ["docs/", "README.md"]
  outputFilters: [no_credentials, no_pii]
```

## Collaboration Patterns

### Review Chain
```yaml
collaboration:
  can_delegate_to:
    - code-reviewer
  accepts_delegation_from:
    - coding-agent
```

### Event Trigger
```yaml
triggers:
  - event: pr_created
    action: auto_review
    condition: "labels.includes('frontend')"
```

## Memory Patterns

### Context-Heavy Role (e.g., architecture reviews)
```yaml
knowledge:
  memory:
    shortTerm:
      maxTokens: 128000
    episodic:
      enabled: true
      retentionDays: 90
    semantic:
      enabled: true
      autoExtract: true
    procedural:
      enabled: true
      learnFromFeedback: true
```

### Lightweight Role (e.g., simple utilities)
```yaml
knowledge:
  memory:
    shortTerm:
      maxTokens: 32000
    episodic:
      enabled: false
    semantic:
      enabled: false
    procedural:
      enabled: false
```

## Resource Limit Patterns

### Conservative (default)
```yaml
security:
  resourceLimits:
    tokenBudget:
      perTask: 50000
      perDay: 200000
    apiCalls:
      perMinute: 30
      perHour: 500
    executionTime:
      perTask: "10m"
      perDay: "2h"
    costLimit:
      perTask: "$2.00"
      perDay: "$10.00"
      alertThreshold: 0.8
```

### Generous (complex tasks)
```yaml
security:
  resourceLimits:
    tokenBudget:
      perTask: 200000
      perDay: 1000000
    apiCalls:
      perMinute: 120
      perHour: 2000
    executionTime:
      perTask: "30m"
      perDay: "4h"
    costLimit:
      perTask: "$10.00"
      perDay: "$50.00"
      alertThreshold: 0.75
```

## Knowledge Source Patterns

### Shared Guidelines
```yaml
knowledge:
  shared:
    - id: ui-guidelines
      type: vector
      access: read
      description: Product UI/UX guidelines
      sources:
        - docs/PRD.md
        - docs/design-system.md
```

### Private Playbook
```yaml
knowledge:
  private:
    - id: team-playbook
      type: doc
      access: read
      description: Internal team conventions
      sources:
        - docs/team-conventions.md
```

## Trigger Patterns

### Auto-Review on PR
```yaml
triggers:
  - event: pr_created
    action: auto_review
    condition: "labels.includes('frontend') || files.some(f => f.startsWith('app/'))"
    autoExecute: true
    requiresApproval: false
```

### Notify on Failure
```yaml
triggers:
  - event: test_failure
    action: notify_team
    condition: "branch === 'main'"
    autoExecute: true
    requiresApproval: true
```

## Override Patterns

### Surgical Override (preferred)
```yaml
extends: coding-agent
overrides:
  identity.role: "Principal Backend Engineer"
  capabilities.maxBudgetUsd: 10.0
```

### Section Override
```yaml
extends: coding-agent
overrides:
  "identity.responseStyle.tone": "formal"
  "security.allowedPaths": ["src-go/", "docs/", "proto/"]
```

## Anti-Patterns to Avoid

- **Role explosion**: One role per dashboard page leads to unmaintainable manifests. Group by surface or team.
- **Overly permissive paths**: Granting `Bash` and `/` access to a doc-writing role.
- **Shallow inheritance**: Extending a role but overriding 90% of its fields. Prefer standalone.
- **Missing version bumps**: Changing behavior without updating `metadata.version` breaks traceability.
- **Skill mismatch**: Binding `skills/react` to a pure Go backend role wastes resolution time and confuses diagnostics.
- **Empty arrays without defaults**: Leaving `repositories: []` without understanding the knowledge resolution fallback.
- **Ignored preview errors**: Creating a role with validation issues that preview already caught.
- **Circular collaboration**: Role A delegates to B, B delegates to C, C delegates to A.
- **Overly broad triggers**: `condition: "true"` with `autoExecute: true` creates noise.
- **Missing output filters**: Roles with network access should always filter credentials and PII.
