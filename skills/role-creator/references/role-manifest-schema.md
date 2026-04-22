# Role Manifest Schema Reference

Complete field reference for `agentforge/v1` Role manifests. All fields shown with their JSON path, YAML key, type, and usage notes.

## Top-Level Structure

| JSON Path | YAML Key | Type | Required |
|-----------|----------|------|----------|
| `apiVersion` | `apiVersion` | string | Yes |
| `kind` | `kind` | string | Yes |
| `metadata` | `metadata` | object | Yes |
| `identity` | `identity` | object | Yes |
| `capabilities` | `capabilities` | object | Yes |
| `knowledge` | `knowledge` | object | Yes |
| `security` | `security` | object | Yes |
| `extends` | `extends` | string | No |
| `overrides` | `overrides` | object | No |
| `collaboration` | `collaboration` | object | No |
| `triggers` | `triggers` | array | No |

---

## metadata

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique kebab-case identifier. Used in URLs and references. |
| `name` | string | Human-readable display name. |
| `version` | string | SemVer string, e.g. `"1.0.0"`. Bump on behavioral changes. |
| `description` | string | One-line purpose summary. |
| `author` | string | Creator or team name. |
| `tags` | string[] | Searchable labels, e.g. `["frontend", "react"]`. |
| `icon` | string | Optional icon identifier for UI display. |

---

## identity

| Field | Type | Description |
|-------|------|-------------|
| `role` | string | Job title, e.g. `"Senior Frontend Developer"`. |
| `goal` | string | What this agent achieves. |
| `backstory` | string | Persona context and expertise background. |
| `systemPrompt` | string | Primary behavioral contract. Multi-line supported. |
| `persona` | string | Brief persona label. |
| `goals` | string[] | Tactical objectives. |
| `constraints` | string[] | Hard boundaries and rules. |
| `personality` | string | e.g. `"patient"`, `"assertive"`. |
| `language` | string | ISO language code, e.g. `"zh-CN"`, `"en-US"`. |
| `responseStyle` | object | See below. |

### responseStyle

| Field | Type | Description |
|-------|------|-------------|
| `tone` | string | e.g. `"professional"`, `"casual"`. |
| `verbosity` | string | e.g. `"concise"`, `"verbose"`. |
| `formatPreference` | string | e.g. `"markdown"`, `"json"`. |

---

## capabilities

| Field | Type | Description |
|-------|------|-------------|
| `packages` | string[] | Capability packages, e.g. `["web-development", "design-system"]`. |
| `toolConfig` | object | Structured tool configuration. See below. |
| `skills` | RoleSkillReference[] | Bound skills. See below. |
| `languages` | string[] | Programming languages, e.g. `["typescript", "go"]`. |
| `frameworks` | string[] | Frameworks, e.g. `["nextjs", "echo"]`. |
| `maxConcurrency` | integer | Max parallel tasks. |
| `maxTurns` | integer | Max conversation turns (default ~30). |
| `maxBudgetUsd` | number | Max spend per task in USD. |
| `customSettings` | object | Key-value map for runtime-specific settings. |

### toolConfig

| Field | Type | Description |
|-------|------|-------------|
| `builtIn` | string[] | Core tools: `Read`, `Edit`, `Write`, `Bash`, `Glob`, `Grep`. |
| `external` | string[] | External tool names, e.g. `["figma"]`. |
| `pluginBindings` | RoleToolPluginBinding[] | Plugin function bindings. |
| `mcpServers` | RoleMCPServer[] | MCP server connections. |

#### RoleToolPluginBinding

| Field | Type | Description |
|-------|------|-------------|
| `pluginId` | string | Plugin identifier. |
| `functions` | string[] | Bound function names. |

#### RoleMCPServer

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Server display name. |
| `url` | string | MCP server endpoint URL. |

### RoleSkillReference

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | Canonical skill path, e.g. `"skills/react"`. |
| `autoLoad` | boolean | `true` = loaded at runtime; `false` = on-demand. |

---

## knowledge

| Field | Type | Description |
|-------|------|-------------|
| `repositories` | string[] | Source directories the agent should know. |
| `documents` | string[] | Key documentation files. |
| `patterns` | string[] | Named codebase patterns. |
| `shared` | RoleKnowledgeSource[] | Shared knowledge sources. |
| `private` | RoleKnowledgeSource[] | Private knowledge sources. |
| `memory` | RoleMemoryConfig | Memory subsystem config. |

### RoleKnowledgeSource

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Source identifier. |
| `type` | string | `"vector"`, `"doc"`, `"code"`. |
| `access` | string | `"read"`, `"write"`, `"admin"`. |
| `description` | string | Human-readable purpose. |
| `sources` | string[] | File or directory paths. |

### RoleMemoryConfig

| Field | Type | Description |
|-------|------|-------------|
| `shortTerm.maxTokens` | integer | Context window size, e.g. `64000`. |
| `episodic.enabled` | boolean | Enable episodic memory. |
| `episodic.retentionDays` | integer | How long to retain episodes. |
| `semantic.enabled` | boolean | Enable semantic memory. |
| `semantic.autoExtract` | boolean | Auto-extract semantic facts. |
| `procedural.enabled` | boolean | Enable procedural memory. |
| `procedural.learnFromFeedback` | boolean | Learn from execution feedback. |

---

## security

| Field | Type | Description |
|-------|------|-------------|
| `profile` | string | Named security profile. |
| `permissionMode` | string | `"bypassPermissions"`, `"restricted"`, `"default"`. |
| `allowedPaths` | string[] | Whitelisted file paths. |
| `deniedPaths` | string[] | Blacklisted file paths. |
| `maxBudgetUsd` | number | Security-enforced budget cap. |
| `requireReview` | boolean | Require human review before execution. |
| `permissions` | RolePermissions | Granular permissions. See below. |
| `outputFilters` | string[] | e.g. `["no_credentials", "no_pii"]`. |
| `resourceLimits` | RoleResourceLimits | Resource consumption caps. See below. |

### RolePermissions

| Field | Type | Description |
|-------|------|-------------|
| `fileAccess.allowedPaths` | string[] | File read/write allowed paths. |
| `fileAccess.deniedPaths` | string[] | File access denied paths. |
| `network.allowedDomains` | string[] | Network request whitelist. |
| `codeExecution.sandbox` | boolean | Require sandboxed execution. |
| `codeExecution.allowedLanguages` | string[] | Permitted execution languages. |

### RoleResourceLimits

| Field | Type | Description |
|-------|------|-------------|
| `tokenBudget.perTask` | integer | Tokens per task. |
| `tokenBudget.perDay` | integer | Tokens per day. |
| `tokenBudget.perMonth` | integer | Tokens per month. |
| `apiCalls.perMinute` | integer | API call rate limit. |
| `apiCalls.perHour` | integer | API call hourly limit. |
| `executionTime.perTask` | string | Duration string, e.g. `"10m"`. |
| `executionTime.perDay` | string | Duration string, e.g. `"2h"`. |
| `costLimit.perTask` | string | Cost string, e.g. `"$5.00"`. |
| `costLimit.perDay` | string | Daily cost cap. |
| `costLimit.alertThreshold` | number | Alert at this fraction (0.0-1.0). |

---

## collaboration

| Field | Type | Description |
|-------|------|-------------|
| `canDelegateTo` | string[] | Role IDs this role may delegate to. |
| `acceptsDelegationFrom` | string[] | Role IDs that may delegate to this role. |
| `communication.preferredChannel` | string | e.g. `"structured"`, `"chat"`. |
| `communication.reportFormat` | string | e.g. `"markdown"`, `"json"`. |
| `communication.escalationPolicy` | string | When and how to escalate. |

---

## triggers

Array of trigger objects:

| Field | Type | Description |
|-------|------|-------------|
| `event` | string | Event name, e.g. `"pr_created"`. |
| `action` | string | Action to take, e.g. `"auto_review"`. |
| `condition` | string | Conditional expression string. |
| `autoExecute` | boolean | Run without human confirmation. |
| `requiresApproval` | boolean | Require approval before executing. |

---

## overrides

Map of dot-path field overrides for inherited roles. Example:

```yaml
overrides:
  identity.role: "Principal Frontend Developer"
  capabilities.maxBudgetUsd: 10.0
```

---

## Complete Minimal Example (JSON)

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

## Complete Rich Example (YAML)

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
