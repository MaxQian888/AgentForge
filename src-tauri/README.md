# AgentForge Desktop (Tauri)

The desktop shell for AgentForge, built with Tauri 2.9 + Rust. Bundles the Next.js frontend into a native desktop application and supervises the Go backend, TS Bridge, and IM Bridge via sidecars.

## Tech Stack

- **Framework**: Tauri 2.9
- **Language**: Rust (edition 2021, minimum toolchain 1.77.2)
- **Frontend**: Next.js 16 (static export to `out/`)
- **Build Tool**: Cargo

## Directory Structure

```
src/
  main.rs                 # Desktop app entry point
  lib.rs                  # Tauri commands and state management
  bin/
    agentforge-desktop-cli.rs   # CLI entry point
  runtime_logic.rs        # Runtime logic
  process_cleanup.rs      # Process cleanup
  standalone_cli.rs       # Standalone CLI logic
tauri.conf.json           # Tauri configuration
Cargo.toml                # Rust project configuration
capabilities/             # Tauri capability declarations
icons/                    # Application icons
target/                   # Cargo build output (gitignored)
```

## Quick Start

```bash
# Development mode (hot reload)
pnpm tauri dev

# Build desktop installer (production)
pnpm tauri build

# Check Tauri environment
pnpm tauri info
```

## Key Configuration

Key fields in `tauri.conf.json`:

- `frontendDist`: `../out` — Next.js static export directory
- `beforeDevCommand`: `pnpm dev` — Starts Next.js first in dev mode
- `beforeBuildCommand`: `pnpm build` — Builds Next.js before Tauri build

> **Note**: Production builds require `output: "export"` in `next.config.ts`.

## Sidecar Supervision

The desktop app supervises three sidecar processes in the background:

| Sidecar | Service | Default Port |
|---------|---------|--------------|
| Go Orchestrator | Main backend API | `7777` |
| TS Bridge | Agent runtime bridge | `7778` |
| IM Bridge | Instant messaging bridge | `7779` |

Tauri handles launching, lifecycle management, and cleanup of these processes.

## Plugins

Official Tauri plugins in use:

- `tauri-plugin-global-shortcut` — Global shortcuts
- `tauri-plugin-log` — Logging
- `tauri-plugin-notification` — Native notifications
- `tauri-plugin-shell` — System shell calls
- `tauri-plugin-dialog` — System dialogs
- `tauri-plugin-process` — Process management
- `tauri-plugin-updater` — Auto-updater (desktop platforms)

## CLI Mode

In addition to the desktop window, the project also builds a CLI entry point `agentforge-desktop-cli` for headless or automation scenarios.
