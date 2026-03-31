# AgentForge Verification Surfaces

Use this reference to choose the correct validation boundary before running broad repo commands.

## Root Web App

- Scope: Next.js App Router, shared UI, frontend stores, static export
- Commands:
  - `pnpm lint`
  - `pnpm exec tsc --noEmit`
  - `pnpm test`
  - `pnpm test:coverage`
  - `pnpm build`

## TS Bridge

- Scope: runtime registry, schemas, handlers, MCP, transport
- Commands:
  - `bun test ...`
  - `bun run typecheck`

## Go Backend

- Scope: `src-go` handlers, services, repositories, role parsing/runtime
- Command:
  - `go test ./...`

## IM Bridge

- Scope: `src-im-bridge` command routing, platform adapters, delivery behavior
- Command:
  - `go test ./...`

## Desktop Runtime

- Scope: `src-tauri` runtime logic and desktop-only behavior
- Commands:
  - `pnpm test:tauri`
  - `pnpm test:tauri:coverage`
