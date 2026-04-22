# Tauri Permission Model / Tauri Capability 与 IPC 安全边界

This document describes the desktop capability files that currently gate native
access in `src-tauri/capabilities/`.

## Capability Files

- `src-tauri/capabilities/default.json`
- `src-tauri/capabilities/desktop.json`

## Current Capability Split

### `default.json`

Baseline permissions for the `main` window:

- `core:default`
- `dialog:default`
- `shell:allow-execute` for named sidecars only:
  - `server`
  - `bridge`
  - `im-bridge`

Implications:

- shell execution is scoped to bundled sidecars
- arbitrary shell access is intentionally not enabled

### `desktop.json`

Desktop-specific additive permissions:

- `notification:default`
- `global-shortcut:allow-register`
- `global-shortcut:allow-unregister`
- `global-shortcut:allow-is-registered`
- `process:default`
- `updater:default`

## Security Boundary

Current repo truth:

- desktop-native access is centralized through `lib/platform-runtime.ts`
- sidecar supervision is the preferred way to reach the Go backend and TS bridge
- permissions are scoped per capability file and window label (`main`)

## What Is Intentionally Not Enabled

The capability files do not currently grant broad:

- filesystem access
- unrestricted network access
- arbitrary shell execution

That aligns with the repo rule to keep Tauri permissions minimal.

## Operational Guidance

- add permissions only for the exact feature being introduced
- prefer named sidecars over general shell access
- if a feature can stay in web mode or go through the backend, prefer that
- verify both desktop and web fallback behavior whenever the platform facade changes
