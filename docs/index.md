# AgentForge

**Agent-driven development management platform.**

AgentForge is a multi-surface workspace that combines a Next.js 16 + React 19 frontend, Go orchestrator backend, Bun agent bridge, and Tauri 2 desktop shell into a unified environment for orchestrating AI agents across real software projects.

## Quick Navigation

| Section | Description |
|---------|-------------|
| [Product](product/prd.md) | Product requirements and platform overview |
| [Architecture](architecture/agent-orchestration.md) | System design and technical decisions |
| [API Reference](api/index.md) | REST and event-driven API documentation |
| [Guides](guides/plugin-development.md) | How-to guides for developers |
| [Deployment](deployment/docker.md) | Running AgentForge in production |
| [Security](security/authentication.md) | Auth, permissions, and best practices |
| [ADRs](adr/index.md) | Architecture Decision Records |

## Runtime Model

- **Web mode** — `pnpm dev` starts Next.js at `http://localhost:3000`
- **Desktop mode** — `pnpm tauri dev` wraps Next.js with Tauri; sidecars supervise Go (7777), TS Bridge (7778), IM Bridge (7779)
- **Full stack** — `pnpm dev:all` starts Postgres + Redis + all services
