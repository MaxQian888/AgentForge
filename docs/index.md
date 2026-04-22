# AgentForge

**Agent-driven development management platform.**

AgentForge is a multi-surface workspace that combines a Next.js 16 + React 19 frontend, Go orchestrator backend, Bun agent bridge, and Tauri 2 desktop shell into a unified environment for orchestrating AI agents across real software projects.

## Quick Navigation

| Section | Description |
|---------|-------------|
| [Product](product/prd.md) | Product requirements and platform overview |
| [Architecture](architecture/agent-orchestration.md) | System design and technical decisions |
| [API Reference](api/index.md) | REST, WebSocket, and AsyncAPI documentation |
| [Guides](guides/plugin-development.md) | Plugin, role, skill, and frontend development guides |
| [Deployment](deployment/docker.md) | Docker, desktop build, and TLS guides |
| [Security](security/authentication.md) | Auth model, permissions, and best practices |
| [Reference](reference/postgres-schema.md) | PostgreSQL and Redis schema reference |
| [ADRs](adr/index.md) | Architecture Decision Records |

### Featured Guides

- [Frontend Components](guides/frontend-components.md) — Component catalog and contracts
- [State Management](guides/state-management.md) — Zustand store patterns
- [Role Authoring](guides/role-authoring.md) — Role workspace flow and sandbox
- [Plugin Development](guides/plugin-development.md) — Authoring workflow for all 5 plugin kinds
- [Internal Skill Governance](guides/internal-skill-governance.md) — Skill provenance, verification, and mirror sync

## Runtime Model

- **Web mode** — `pnpm dev` starts Next.js at `http://localhost:3000`
- **Desktop mode** — `pnpm tauri dev` wraps Next.js with Tauri; sidecars supervise Go (7777), TS Bridge (7778), IM Bridge (7779)
- **Full stack** — `pnpm dev:all` starts Postgres + Redis + all services
