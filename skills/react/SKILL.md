---
name: React
description: Use when building or repairing AgentForge React and Next.js App Router surfaces, especially dashboard pages, shared UI components, client/server boundaries, and responsive product workflows.
requires:
  - skills/typescript
tools:
  - code_editor
  - browser_preview
---

# React

Build product-facing React surfaces the way this repository already works instead of inventing a parallel frontend architecture.

## Guardrails

- Start from the existing route, feature component, store, and design-system seams before adding wrappers, providers, or duplicate state.
- Prefer Server Components by default. Add `"use client"` only at the smallest boundary that truly needs browser APIs, effects, or local interaction state.
- Treat static export as a hard constraint when touching layout or provider code. Keep render paths server-safe and avoid browser-only work during initial render.
- Reuse the current dashboard shell, role workspace, settings workspace, and store/query patterns instead of creating one-off page architectures.
- Preserve accessibility, responsive behavior, empty states, error states, and loading states together. Do not ship only the happy path.
- Keep shared layout changes safe for both web and desktop shells when the surface is reused in Tauri.

## Working Loop

- Read the route, adjacent components, store, and nearby tests before editing so the change follows the established seam.
- Keep page files thin. Move durable view logic into feature components, stores, or helpers when a page starts mixing data, state, and layout concerns.
- Prefer extending an existing store or feature-local state over adding new global state.
- Reuse shared UI primitives from `components/ui` and existing dashboard patterns before adding new primitives.
- Keep data boundaries explicit: fetch/normalize near the boundary, render normalized state in the UI, and avoid shape rewrites inside presentational components.

## Verification

- Validate the touched surface with the narrowest useful check first, then rerun the relevant repo gate.
- When route-group paths are involved on Windows, prefer explicit test paths over fragile shell globs.
- Finish with the relevant lint, test, and build checks for the page or shared seam you changed.

## References

- Read [references/agentforge-react-surface-map.md](references/agentforge-react-surface-map.md) when the task touches a shared React seam and you need a quick map of the canonical UI, store, and verification entrypoints in this repository.
