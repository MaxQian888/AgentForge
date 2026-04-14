# Desktop Build Guide / Tauri 桌面打包指南

This document describes the current Tauri desktop packaging flow defined by:

- `src-tauri/tauri.conf.json`
- `package.json`
- `scripts/build-backend.js`
- `scripts/build-bridge.js`
- `scripts/build-im-bridge.js`
- `docs/desktop-updater-release.md`

## Prerequisites

- Node.js 20+
- pnpm
- Rust toolchain with `cargo`
- Go 1.25+ for the backend sidecar
- Bun for the TS bridge build
- platform-specific Tauri dependencies

## Build Stages

### 1. Prepare sidecars

```bash
pnpm desktop:build:prepare
```

That runs:

- `pnpm build:backend`
- `pnpm build:bridge`
- `pnpm build:im-bridge`
- `pnpm build`

Sidecar outputs are written to `src-tauri/binaries/`:

- `server-<triple>`
- `bridge-<triple>`
- `im-bridge-<triple>`

### 2. Package the app

```bash
pnpm tauri:build
```

`src-tauri/tauri.conf.json` declares:

- product name: `AgentForge`
- identifier: `dev.agentforge.desktop`
- bundle targets: `all`
- external binaries:
  - `binaries/server`
  - `binaries/bridge`
  - `binaries/im-bridge`

## Development Mode

For desktop development:

```bash
pnpm tauri:dev
```

The Tauri config uses:

- `beforeDevCommand`: `pnpm desktop:dev:prepare && pnpm dev`
- `devUrl`: `http://localhost:3000`

## Standalone Rust Debug Mode

When you want to debug only the Rust/Tauri runtime and keep frontend startup separate:

```bash
pnpm dev
pnpm desktop:standalone:check
pnpm desktop:standalone:dev
```

Notes:

- `desktop:standalone:check` reuses `pnpm desktop:dev:prepare` and validates the current `devUrl`, current-host sidecar binaries, and known runtime ports.
- `desktop:standalone:dev` reuses the same current-host sidecar prepare contract, but it does **not** start `pnpm dev` for you.
- The maintained VS Code `Tauri Standalone Rust Debug` launch entrypoint follows the same rule: current-host sidecars are prepared automatically, frontend availability remains an external prerequisite.

## Cross-Platform Notes

### Go sidecars

`scripts/build-backend.js` and `scripts/build-im-bridge.js` cross-compile for:

- Linux x64 / arm64
- Windows x64
- macOS x64 / arm64

### TS bridge

`scripts/build-bridge.js` uses `bun build --compile` for:

- `bun-linux-x64`
- `bun-linux-arm64`
- `bun-windows-x64`
- `bun-darwin-x64`
- `bun-darwin-arm64`

## Signing And Updater

Current updater config:

- endpoint:
  `https://github.com/Arxtect/AgentForge/releases/latest/download/latest.json`
- Windows install mode: `passive`
- `pubkey` is blank until release signing is configured

Windows bundle config exposes:

- `certificateThumbprint`
- `digestAlgorithm`
- `timestampUrl`

Populate these with real signing values before shipping signed builds.

## Release Validation

```bash
pnpm build:updater-manifest
pnpm verify:updater-artifacts
```

Use them together with [../desktop-updater-release.md](../desktop-updater-release.md)
when preparing desktop releases.
