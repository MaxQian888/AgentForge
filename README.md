# AgentForge

AgentForge is an agent-driven development management platform that connects the full delivery loop:

`IM request -> AI task decomposition -> agent execution -> automated review -> delivery`

The project vision, defined in the latest product documents, is to make AI agents first-class team members with identity, role, cost tracking, review workflows, and collaboration surfaces alongside human developers.

[中文文档](./README_zh.md)

## What This Repository Contains

This repository is no longer just a generic starter template. It is an evolving AgentForge workspace that currently includes:

- A Next.js 16 + React 19 dashboard and auth surface in `app/`
- A Go backend foundation in `src-go/`
- A TypeScript/Bun agent bridge service in `src-bridge/`
- An IM bridge fork workspace in `src-im-bridge/`
- A Tauri desktop wrapper in `src-tauri/`
- Product, architecture, plugin, review, and technical design documents in `docs/`

## Product Direction

According to the latest PRD, AgentForge aims to be:

- An open-source platform for managing mixed human + AI engineering teams
- A system that can receive work from IM tools, decompose tasks, assign work to agents or people, and track execution
- A platform with a built-in review pipeline, budget controls, progress tracking, and plugin extensibility
- A bridge between team communication, development workflows, review automation, and delivery

## Architecture At A Glance

The current documentation describes AgentForge around these major layers:

- `Web Dashboard`: Next.js 16 UI for task management, agent status, project views, cost views, and team operations
- `Go Orchestrator`: API, task lifecycle, scheduling, worktree management, review coordination, and realtime distribution
- `TS Agent Bridge`: the unified backend AI entry point for agent execution and lightweight AI analysis
- `IM Bridge`: a cc-connect-based service for Feishu, DingTalk, Slack, Telegram, Discord, and other messaging channels
- `Review Pipeline`: layered review flow covering fast checks, deep review, and human approval
- `Data Layer`: PostgreSQL, Redis, WebSocket/event flow, and related infra

## Current Repository Status

This codebase is in an active migration from an earlier starter foundation into AgentForge. That matters for anyone reading the repo:

- Product docs and architecture docs already use the `AgentForge` identity
- Some code/package/module names still retain starter-era names such as `react-quick-starter` or `react-go-quick-starter`
- The repo contains real implementation workspaces, but the product design is ahead of some runtime surfaces
- If documentation sections disagree, treat [`docs/PRD.md`](./docs/PRD.md) as the latest product source of truth

One important example: the PRD v2 notes that Go-to-TS communication has moved toward `HTTP + WebSocket`, while some older design parts still describe `gRPC`-based variants. The PRD should win when they conflict.

## Repository Map

```text
AgentForge/
├── app/                 # Next.js App Router: auth + dashboard routes
├── components/          # Shared UI components
├── hooks/               # Frontend hooks
├── lib/                 # Frontend utilities and mock/domain helpers
├── src-go/              # Go backend foundation
├── src-bridge/          # TypeScript/Bun agent bridge service
├── src-im-bridge/       # IM bridge fork workspace
├── src-tauri/           # Tauri desktop shell
├── docs/                # PRD, research, architecture, design docs
├── openspec/            # OpenSpec change artifacts
├── roles/               # Role definitions and related assets
└── scripts/             # Build helpers such as backend sidecar compilation
```

Notable frontend route groups already present:

- `app/(auth)` for login and registration
- `app/(dashboard)` for dashboard, agents, projects, roles, and cost views

## Documentation Guide

Start here if you want the latest project narrative:

- [`docs/PRD.md`](./docs/PRD.md): unified product requirements and latest overall direction
- [`docs/part/AGENT_ORCHESTRATION.md`](./docs/part/AGENT_ORCHESTRATION.md): orchestrator, bridge, agent pool, worktree, and execution model
- [`docs/part/REVIEW_PIPELINE_DESIGN.md`](./docs/part/REVIEW_PIPELINE_DESIGN.md): three-layer review architecture
- [`docs/part/PLUGIN_SYSTEM_DESIGN.md`](./docs/part/PLUGIN_SYSTEM_DESIGN.md): target plugin system design
- [`docs/part/PLUGIN_RESEARCH_TECH.md`](./docs/part/PLUGIN_RESEARCH_TECH.md): runtime and sandbox technology research for plugins
- [`docs/GO_WASM_PLUGIN_RUNTIME.md`](./docs/GO_WASM_PLUGIN_RUNTIME.md): current Go-side WASM plugin runtime, SDK, and local verification flow
- [`docs/part/PLUGIN_RESEARCH_PLATFORMS.md`](./docs/part/PLUGIN_RESEARCH_PLATFORMS.md): platform comparison for extension ecosystems
- [`docs/part/TECHNICAL_CHALLENGES.md`](./docs/part/TECHNICAL_CHALLENGES.md): key engineering risks and mitigation paths
- [`docs/part/DATA_AND_REALTIME_DESIGN.md`](./docs/part/DATA_AND_REALTIME_DESIGN.md): data model and realtime/event design
- [`docs/part/CC_CONNECT_REUSE_GUIDE.md`](./docs/part/CC_CONNECT_REUSE_GUIDE.md): IM bridge fork and reuse strategy

Supporting repository docs:

- [`AGENTS.md`](./AGENTS.md): repository working conventions
- [`CONTRIBUTING.md`](./CONTRIBUTING.md): contribution guide
- [`TESTING.md`](./TESTING.md): testing notes
- [`CI_CD.md`](./CI_CD.md): CI/CD overview
- [`CHANGELOG.md`](./CHANGELOG.md): project changelog

## Prerequisites

- Node.js 20+
- pnpm
- Go 1.25+ for `src-go/`
- Bun for `src-bridge/`
- Rust 1.77.2+ for Tauri desktop development
- Docker Desktop or another Docker environment if you want local PostgreSQL/Redis

## Getting Started

### 1. Frontend Dashboard

```bash
pnpm install
pnpm dev
```

This starts the Next.js app in development mode.

Useful root commands:

- `pnpm dev`
- `pnpm build`
- `pnpm start`
- `pnpm lint`
- `pnpm test`
- `pnpm test:coverage`

### 2. Go Backend

From the repository root, start infrastructure if needed:

```bash
docker compose up -d
```

Then run the Go service:

```bash
cd src-go
go run ./cmd/server
```

Useful backend commands:

- `go test ./...`
- `go build ./cmd/server`
- `docker build -t agentforge-server .`

### Auth And Session Notes

The current auth flow is intentionally aligned across the frontend and Go backend:

- The frontend persists the canonical session payload: `accessToken`, `refreshToken`, and `user`.
- Protected dashboard routes do not trust a cached boolean alone. On bootstrap, the app validates the stored access token with `GET /api/v1/users/me`, attempts one `POST /api/v1/auth/refresh` when the access token is no longer authorized, and clears stale session state if recovery fails.
- Web mode resolves the backend from `NEXT_PUBLIC_API_URL` and falls back to `http://localhost:7777`. Tauri mode uses the native `get_backend_url` command first, then falls back to the same default.
- `POST /api/v1/auth/refresh` is rate-limited together with login and registration.

For local backend auth config, create `src-go/.env` if you need overrides. Typical values are:

```env
PORT=7777
ENV=development
JWT_SECRET=change-me-in-production-at-least-32-chars
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h
ALLOW_ORIGINS=http://localhost:3000,tauri://localhost,http://localhost:1420
REDIS_URL=redis://localhost:6379
```

Security note: PostgreSQL/Redis can still be absent at process startup for local development, but auth paths that depend on token revocation state do not silently degrade. If Redis or the token cache is unavailable, refresh, logout revocation, and blacklist-backed protected-route checks now fail closed instead of reporting success.

### 3. TypeScript Agent Bridge

```bash
cd src-bridge
bun install
bun run dev
```

Useful bridge commands:

- `bun run dev`
- `bun run build`
- `bun run typecheck`

Runtime notes:

- `/bridge/execute` now accepts an optional `runtime` field with `claude_code`, `codex`, or `opencode`.
- If `runtime` is omitted, the bridge defaults to `claude_code` and still maps legacy provider hints such as `anthropic`, `codex`, and `opencode`.
- `claude_code` uses the built-in Claude-backed adapter and expects `ANTHROPIC_API_KEY`.
- `codex` and `opencode` use command-based adapters. Set `CODEX_RUNTIME_COMMAND` or `OPENCODE_RUNTIME_COMMAND` to an executable on `PATH` (or an absolute path). Each command must read one JSON request from `stdin` and emit newline-delimited JSON events on `stdout`.
- Command adapters normalize these event types into the canonical bridge stream: `assistant_text`, `tool_call`, `tool_result`, `usage`, and `error`.

Focused verification for the bridge runtime layer:

- `bun test src/schemas.test.ts src/handlers/execute.test.ts src/runtime/registry.test.ts src/server.test.ts`
- `bun run typecheck`

From the repo root, there is also:

```bash
pnpm build:bridge
```

### 4. IM Bridge Workspace

```bash
cd src-im-bridge
go run ./cmd/bridge
```

Useful IM bridge commands:

- `go test ./...`
- `go build ./cmd/bridge`

### 5. Desktop Mode

If you are working on the desktop shell:

```bash
pnpm tauri:dev
```

Or build desktop artifacts:

```bash
pnpm tauri:build
```

## Key Root Scripts

| Command | Purpose |
| --- | --- |
| `pnpm dev` | Run the Next.js web app |
| `pnpm build` | Build the Next.js app |
| `pnpm start` | Start the built Next.js app |
| `pnpm lint` | Run ESLint |
| `pnpm test` | Run Jest |
| `pnpm test:coverage` | Run Jest with coverage |
| `pnpm build:backend` | Cross-compile Go sidecar binaries for Tauri |
| `pnpm build:backend:dev` | Build the Go sidecar for the current platform |
| `pnpm build:plugin:wasm` | Build the Go WASM sample plugin artifact |
| `pnpm tauri:dev` | Build backend sidecar and start Tauri dev mode |
| `pnpm tauri:build` | Build the desktop app |
| `pnpm build:bridge` | Install and build the TS/Bun bridge |

## Tech Stack Snapshot

- Frontend: Next.js 16, React 19, TypeScript, Tailwind CSS v4, shadcn/ui, Zustand
- Backend: Go 1.25, Echo, PostgreSQL, Redis
- Bridge: Bun, TypeScript, Hono, WebSocket
- Desktop: Tauri 2
- Tooling: ESLint, Jest, OpenSpec, MCP configs

## Working Notes

- Secrets should stay in local env files such as `.env.local` or service-specific `.env.example` copies
- `src-tauri/` should keep capability scope minimal
- The repository includes both implementation work and design-stage artifacts, so do not assume every documented module is fully production-ready yet
- When in doubt about project intent, prefer the PRD and architecture docs over the legacy starter phrasing still visible in some package/module names

## License

[MIT](./LICENSE)
