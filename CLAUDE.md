# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AgentForge — agent-driven development management platform. Multi-surface workspace with Next.js 16 (React 19) + Tauri 2.9 + TypeScript + Tailwind CSS v4 + shadcn/ui + Zustand frontend, Go orchestrator backend, Bun agent bridge, IM bridge, and Marketplace microservice.

**Subproject guides:** [Go backend](src-go/CLAUDE.md) · [Marketplace](src-marketplace/CLAUDE.md) · [Bridge](src-bridge/CLAUDE.md) · [IM Bridge](src-im-bridge/CLAUDE.md) · [Tauri](src-tauri/CLAUDE.md) · [Skills](.agents/CLAUDE.md)

**Dual Runtime Model:**

- **Web mode** (`pnpm dev`): Next.js dev server at <http://localhost:3000>
- **Desktop mode** (`pnpm tauri dev`): Tauri wraps Next.js in a native window with sidecar supervision

## Development Commands

```bash
# Frontend
pnpm dev              # Start Next.js dev server
pnpm build            # Build for production (outputs to out/)
pnpm start            # Serve the production build
pnpm lint             # Run ESLint
pnpm lint --fix       # Auto-fix ESLint issues

# Testing
pnpm test             # Run Jest tests (unit + integration)
pnpm test:watch       # Run tests in watch mode
pnpm test:coverage    # Run tests with coverage report
pnpm test:e2e         # Run Playwright end-to-end tests
pnpm test:tauri       # Run Tauri Rust unit tests
pnpm test:tauri:coverage  # Run Tauri Rust tests with coverage gates

# Type checking
pnpm exec tsc --noEmit

# Desktop (Tauri)
pnpm tauri dev        # Dev mode with hot reload
pnpm tauri build      # Build desktop installer
pnpm tauri info       # Check Tauri environment
pnpm desktop:dev:prepare    # Build backend + bridge + im-bridge for current platform
pnpm desktop:standalone:check   # Verify desktop sidecar health
pnpm desktop:standalone:dev     # Run desktop with sidecars in standalone mode
pnpm desktop:build:prepare      # Full cross-platform build for desktop release

# Full local stack
pnpm dev:all          # Start compose infra + Go + Bridge + IM Bridge + frontend
pnpm dev:all:status   # Check stack health
pnpm dev:all:stop     # Stop managed services
pnpm dev:all:logs     # Show log file paths
pnpm dev:all:watch    # Start with file-watch hot reload
pnpm dev:all:verify   # Full startup + health + smoke test
pnpm dev:all:restart  # Restart a single service

# Backend-only (no frontend)
pnpm dev:backend            # Start PG + Redis + Go + Bridge + IM Bridge
pnpm dev:backend:watch      # Same as above but with air hot-reload for Go
pnpm dev:backend:status     # Check backend service health
pnpm dev:backend:stop       # Stop backend services
pnpm dev:backend:restart    # Restart a single service: pnpm dev:backend:restart go-orchestrator
pnpm dev:backend:logs       # Show log file paths
pnpm dev:backend:verify     # Full startup + health + smoke test

# Plugin development
pnpm create-plugin          # Scaffold a new plugin
pnpm plugin:build           # Build Go WASM plugins
pnpm plugin:debug           # Debug Go WASM plugins
pnpm plugin:dev             # Run plugin dev stack
pnpm plugin:verify          # Verify plugin dev workflow
pnpm plugin:verify:builtins # Verify built-in plugin bundle

# Skill development
pnpm skill:sync:mirrors     # Sync internal skill mirrors
pnpm skill:verify:internal  # Verify internal skills
pnpm skill:verify:builtins  # Verify built-in skill bundle

# i18n
pnpm i18n:audit             # Audit missing i18n keys

# Add shadcn/ui components
pnpm dlx shadcn@latest add <component-name>
```

## Go Backend Commands

```bash
# Run backend directly (requires PostgreSQL + Redis running)
cd src-go && go run ./cmd/server

# Build for current platform
cd src-go && go build ./cmd/server

# Run Go tests
cd src-go && go test ./...

# Compile Go sidecar for current platform only (fast, for local dev)
pnpm build:backend:dev

# Cross-compile Go sidecar for all platforms
pnpm build:backend
```

### Backend Environment (src-go/.env)
Create `src-go/.env` if you need local overrides. Common auth-related values:
- `POSTGRES_URL` — PostgreSQL connection string
- `REDIS_URL` — Redis connection string used for refresh-token storage and revocation checks
- `JWT_SECRET` — Must be set in production (min 32 chars recommended)
- `JWT_ACCESS_TTL=15m`
- `JWT_REFRESH_TTL=168h`
- `ALLOW_ORIGINS=http://localhost:3000,tauri://localhost,http://localhost:1420`

Auth flow notes:
- Frontend auth persists `accessToken`, `refreshToken`, and `user`, then revalidates sessions through `GET /api/v1/users/me`.
- If the access token is no longer valid and a refresh token exists, the frontend attempts one `POST /api/v1/auth/refresh` before clearing the local session.
- Auth requests resolve the backend URL through the shared resolver: `NEXT_PUBLIC_API_URL` in web mode, Tauri `get_backend_url` in desktop mode, then `http://localhost:7777` as fallback.
- Refresh, logout revocation, and blacklist-backed protected-route checks now fail closed when Redis/token-cache state is unavailable; do not document or reintroduce silent success on cache failure.

## Marketplace Service

The marketplace (`src-marketplace/`) is a standalone Go microservice for publishing, discovering, installing, and reviewing plugins, skills, and roles.

```bash
cd src-marketplace
go run ./cmd/server    # Runs on port 7781 by default
go test ./...          # Run marketplace tests
go build ./cmd/server  # Build marketplace binary
```

Frontend store: `lib/stores/marketplace-store.ts`
Frontend page: `app/(dashboard)/marketplace/page.tsx`
Components: `components/marketplace/`

Current marketplace delivery notes:

- Default standalone port is `7781`; do not reuse the IM Bridge port.
- The main Go backend now bridges marketplace installs and typed consumption state through `/api/v1/marketplace/install` and `/api/v1/marketplace/consumption`.
- Marketplace installs materialize into existing consumer seams: plugins go to the plugin control plane, roles go into the repo-local roles store, and skills go into the authoritative role skill catalog.
- Local side-load in the marketplace workspace currently reuses the plugin local-install seam. Unsupported role/skill side-load flows should stay explicitly blocked instead of pretending to succeed.

## Architecture

### Frontend Structure

- `app/` - Next.js App Router (layout.tsx, page.tsx, globals.css)
- `app/(auth)` - Login, registration, and invitation accept/decline pages
- `app/(dashboard)` - Dashboard route group covering: overview, agents, employees, teams, reviews, cost, scheduler, memory, roles, plugins, marketplace, settings, IM, docs, workflow, sprints, documents, skills, debug
- `app/(dashboard)/employees/[id]` - Per-agent profile, run history (`/runs/`), and trigger configuration (`/triggers/`)
- `app/(dashboard)/projects/[id]/integrations/vcs` - Per-project VCS provider connections
- `app/(dashboard)/projects/[id]/secrets` - Per-project secret management
- `app/(dashboard)/projects/[id]/qianchuan` - Ads-platform bindings and strategy surfaces
- `app/(dashboard)/project/templates` - Project template gallery
- `app/(dashboard)/docs/[pageId]` - Project wiki with BlockNote editor, comments, versions
- `app/(dashboard)/debug` - Observability surface with live tail and timeline views
- `app/forms/[slug]` - Standalone form pages
- `components/ui/` - shadcn/ui components using Radix UI + class-variance-authority (60+ components)
- `components/knowledge/` - Knowledge base UI (IngestedFilesPane, KnowledgeSearch, MaterializedFromPill, SourceUpdatedBanner)
- `components/im/` - IM control plane components (bridge inventory panel)
- `lib/stores/` - Zustand stores (70+ store files covering all domain surfaces, including test coverage)
- `lib/i18n/` - Internationalization (next-intl)
- `hooks/` - Frontend hooks (use-mobile, use-backend-url, use-breadcrumbs, use-breakpoint, use-keyboard-navigation, use-platform-capability, use-a11y-preferences, use-live-artifact-projections, use-plugin-enabled, use-project-role)
- `lib/utils.ts` - `cn()` utility (clsx + tailwind-merge)

### Backend Structure (src-go/)

Go orchestrator using Echo framework with layered architecture:
- `cmd/server` - Entry point
- `internal/handler` - HTTP handlers
- `internal/service` - Business logic
- `internal/repository` - Data access
- `internal/model` - Domain models
- `internal/middleware` - Auth, CORS, rate limiting
- `internal/ws` - WebSocket hub
- `internal/plugin` - Plugin control plane (WASM runtime, MCP tools, catalog, trust gates)
- `internal/scheduler` - Job scheduling
- `internal/role` - Role management
- `internal/worktree` - Git worktree management
- `internal/cost` - Cost tracking
- `internal/memory` - Project memory
- `internal/pool` - Agent pool management
- `internal/trigger` - Automation trigger engine (CRUD, idempotency, routing, schedule ticker, dry-run)
- `internal/automation` - Declarative automation rules evaluated by the trigger engine
- `internal/vcs` - VCS provider registry (GitHub, GitLab, Gitea) with webhook router
- `internal/knowledge` - Knowledge asset management, chunked ingestion, vector search, live-artifact materialization
- `internal/secrets` - Per-project secret storage
- `internal/employee` - Agent identity (employee) management
- `internal/adsplatform` - Ads-platform provider registry (Qianchuan bindings and strategies)
- `internal/queue` - Agent work queue and priority controls
- `internal/skills` - Governed skill catalog operations
- `internal/document` - Document management (global, distinct from project wiki)
- `internal/eventbus` - Internal event publish/subscribe bus
- `internal/instruction` - Agent instruction/prompt management
- `internal/storage` - Blob storage abstraction
- `internal/imcards` - IM rich-card payload formatters
- `internal/imconfig` - IM provider configuration management
- `internal/version` - Service version metadata
- `internal/integration` - External integration trigger flow tests
- `internal/workflow` - Workflow engine (node types, execution, templates, DAG evaluation)
- `internal/bridge` - Bridge integration and runtime coordination
- `internal/log` - Structured logging and observability

### Marketplace Structure (src-marketplace/)

Standalone Go microservice:
- `cmd/server` - Entry point
- `internal/handler` - HTTP handlers (items, versions, reviews, admin)
- `internal/service` - Business logic
- `internal/repository` - Data access
- `internal/model` - Domain models
- `internal/config` - Configuration
- `internal/i18n` - Internationalization
- `migrations/` - Database migrations

### Bridge Structure (src-bridge/)

TypeScript/Bun service using Hono:
- `src/server.ts` - Entry point
- `src/runtime/` - Runtime adapters (claude_code, codex, opencode, cursor, gemini, qoder, iflow)
  - `src/runtime/acp/` - ACP client integration (adapter-factory, connection-pool, multiplexed-client, capabilities, handlers for fs/terminal/permission/elicitation)
- `src/handlers/` - Request handlers (execute, legacy runtime adapters now migrated to ACP)
- `src/plugins/` - Plugin hosting
- `src/mcp/` - MCP integration
- `src/session/` - Session management
- `src/review/` - Review pipeline
- `src/schemas.ts` - Shared schemas

### IM Bridge Structure (src-im-bridge/)

Standalone Go service for IM provider connectivity:
- `cmd/bridge/` - Entry point (multi-provider supervision, hot-reload, control plane, inventory)
- `client/` - AgentForge backend client, control plane, reaction handling
- `commands/` - IM command parsers and executors
- `core/` - Core message routing and dispatch
- `platform/` - Platform adapters (feishu, dingtalk, slack, telegram, discord, wecom, qq, qqbot)
- `notify/` - Notification dispatch
- `audit/` - Audit event logging

### Tauri Integration

- `src-tauri/` - Rust backend
  - `tauri.conf.json` - Config pointing `frontendDist` to `../out`
  - `beforeDevCommand`: runs `pnpm dev`
  - `beforeBuildCommand`: runs `pnpm build`
  - Desktop supervises three sidecars: Go orchestrator (7777), TS Bridge (7778), IM Bridge (7779)
  - `src-tauri/src/bin/agentforge-desktop-cli.rs` - Desktop standalone CLI for sidecar health checks and local runs

### Styling System

- **Tailwind v4** via PostCSS (`@tailwindcss/postcss`)
- CSS variables for theme colors (oklch color space) in `globals.css`
- Dark mode: class-based (apply `.dark` to parent element)
- Custom variant: `@custom-variant dark (&:is(.dark *))`

### Path Aliases

`@/components`, `@/lib`, `@/utils`, `@/ui`, `@/hooks` - all configured in tsconfig.json and components.json

## Code Patterns

```tsx
// Always use cn() for conditional classes
import { cn } from "@/lib/utils"
cn("base-classes", condition && "conditional", className)

// Button composition with asChild
<Button asChild>
  <Link href="/path">Click me</Link>
</Button>
```

## Key Dependencies

- UI: `shadcn/ui` (Radix UI, 60+ components), `lucide-react` icons, `@hello-pangea/dnd` drag-and-drop, `@dnd-kit/core`, `@tanstack/react-table`, `react-grid-layout`, `recharts`
- Editor: `@blocknote/core` + `@blocknote/react` + `@blocknote/shadcn` for wiki/docs editing, `@monaco-editor/react` for code editing
- i18n: `next-intl`
- State: `zustand` (70+ store files)
- Charts: `recharts`
- Flow: `@xyflow/react` for workflow visual editor
- Forms: `react-hook-form` + `@hookform/resolvers` + `zod`
- CLI: `cmdk` for command palette
- Notifications: `sonner` for toast notifications
- Date: `date-fns`, `react-day-picker`
- Diagrams: `mermaid`, `katex`
- Carousel: `embla-carousel-react`
- OTP: `input-otp`
- Diff: `react-diff-viewer-continued`
- Markdown: `react-markdown`, `remark-gfm`
- Panels: `react-resizable-panels`
- Drawer: `vaul`
- Base UI: `@base-ui/react`

## Critical Notes

- **Always use pnpm** (lockfile present in root, src-bridge, and src-im-bridge)
- **Tauri production builds require static export**: `output: "export"` is already set in `next.config.ts`; `pnpm build` outputs to `out/` which Tauri loads from `src-tauri/tauri.conf.json`
- **Rust toolchain**: Requires v1.77.2+ for Tauri builds
- shadcn/ui configured with "new-york" style, RSC mode, `neutral` base color, `lucide` icon library
- **Git hooks**: Husky + lint-staged configured; staged JS/TS files auto-run `eslint --fix --max-warnings=0` on commit
- **Test runner**: Jest with jsdom environment, `next/jest` integration, coverage thresholds (branches/functions 60%, lines/statements 70%)
- **E2E tests**: Playwright configured (`pnpm test:e2e`)
- The repository includes both real implementation and design-stage artifacts
- When in doubt about project intent, prefer `docs/PRD.md` as the latest source of truth
