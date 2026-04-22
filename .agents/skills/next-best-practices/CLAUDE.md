# next-best-practices/CLAUDE.md

Next.js best practices skill for AgentForge.

## Purpose

Covers Next.js 16 App Router patterns: file conventions, RSC boundaries, async APIs, data patterns, error handling, route handlers, metadata, and hydration.

## Key References

| File | Content |
|------|---------|
| `SKILL.md` | Skill definition, entry point |
| `references/file-conventions.md` | Project structure, route segments, parallel/intercepting routes |
| `references/rsc-boundaries.md` | Invalid RSC pattern detection |
| `references/async-patterns.md` | Async params, searchParams, cookies, headers |
| `references/data-patterns.md` | Data fetching and caching patterns |
| `references/error-handling.md` | Error boundaries and handling |
| `references/route-handlers.md` | API route conventions |
| `references/suspense-boundaries.md` | Suspense usage guidance |
| `references/hydration-error.md` | Hydration mismatch debugging |
| `references/bundling.md` | Bundle optimization |
| `references/metadata.md` | Metadata API usage |
| `references/image.md` | Image optimization |
| `references/font.md` | Font loading |
| `references/parallel-routes.md` | Parallel routes deep-dive |
| `references/runtime-selection.md` | Node.js vs Edge runtime |
| `references/self-hosting.md` | Self-hosting notes |
| `references/scripts.md` | Script loading |
| `references/directives.md` | "use client" / "use server" guidance |
| `references/debug-tricks.md` | Debugging tips |
| `references/functions.md` | Server Functions patterns |

## Project Context

- Next.js 16, React 19
- App Router with static export (`output: "export"`)
- Middleware renamed to `proxy` in v16
