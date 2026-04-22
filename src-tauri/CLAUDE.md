# src-tauri/CLAUDE.md

Tauri v2.9 Rust backend for AgentForge desktop mode.

## Overview

Wraps the Next.js frontend in a native window and supervises three sidecars.

## Quick Commands

```bash
# From repo root
pnpm tauri dev        # Dev mode with hot reload
pnpm tauri build      # Build desktop installer
pnpm tauri info       # Check Tauri environment

# Rust tests
pnpm test:tauri
pnpm test:tauri:coverage
```

## Structure

| File/Dir | Responsibility |
|----------|---------------|
| `tauri.conf.json` | Config pointing `frontendDist` to `../out` |
| `src/bin/agentforge-desktop-cli.rs` | Desktop standalone CLI for sidecar health checks and local runs |

## Configuration

- `beforeDevCommand`: runs `pnpm dev`
- `beforeBuildCommand`: runs `pnpm build`
- `frontendDist`: `../out` (static export from Next.js)

## Sidecars

| Service | Port | Binary |
|---------|------|--------|
| Go orchestrator | 7777 | `agentforge-backend` |
| TS Bridge | 7778 | `agentforge-bridge` |
| IM Bridge | 7779 | `agentforge-im-bridge` |

## Notes

- Requires Rust toolchain v1.77.2+.
- Production builds require Next.js static export (`output: "export"` in `next.config.ts`).
