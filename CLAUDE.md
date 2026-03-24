# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

React + Tauri desktop application starter: Next.js 16 (React 19) + Tauri 2.9 + TypeScript + Tailwind CSS v4 + shadcn/ui + Zustand.

**Dual Runtime Model:**

- **Web mode** (`pnpm dev`): Next.js dev server at <http://localhost:3000>
- **Desktop mode** (`pnpm tauri dev`): Tauri wraps Next.js in a native window

## Development Commands

```bash
# Frontend
pnpm dev              # Start Next.js dev server
pnpm build            # Build for production (outputs to out/)
pnpm lint             # Run ESLint
pnpm lint --fix       # Auto-fix ESLint issues

# Testing
pnpm test             # Run Jest tests
pnpm test:watch       # Run tests in watch mode
pnpm test:coverage    # Run tests with coverage report

# Type checking
pnpm exec tsc --noEmit

# Desktop (Tauri)
pnpm tauri dev        # Dev mode with hot reload
pnpm tauri build      # Build desktop installer
pnpm tauri info       # Check Tauri environment

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

# Start Tauri desktop app with auto-compiled backend
pnpm tauri:dev

# Full production build: Go binary + Next.js export + Tauri installer
pnpm tauri:build
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

## Architecture

### Frontend Structure

- `app/` - Next.js App Router (layout.tsx, page.tsx, globals.css)
- `components/ui/` - shadcn/ui components using Radix UI + class-variance-authority
- `lib/utils.ts` - `cn()` utility (clsx + tailwind-merge)
- `__tests__/` - Jest tests with React Testing Library

### Tauri Integration

- `src-tauri/` - Rust backend
  - `tauri.conf.json` - Config pointing `frontendDist` to `../out`
  - `beforeDevCommand`: runs `pnpm dev`
  - `beforeBuildCommand`: runs `pnpm build`

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

## Critical Notes

- **Always use pnpm** (lockfile present)
- **Tauri production builds require static export**: Add `output: "export"` to `next.config.ts` for `pnpm tauri build` to work
- **Rust toolchain**: Requires v1.77.2+ for Tauri builds
- shadcn/ui configured with "new-york" style and RSC mode
