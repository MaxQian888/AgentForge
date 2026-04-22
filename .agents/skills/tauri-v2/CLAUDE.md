# tauri-v2/CLAUDE.md

Tauri v2 skill for AgentForge.

## Purpose

Guides Tauri v2 desktop development in `src-tauri/`: Rust entrypoints, IPC commands, capabilities/permissions, plugin setup, and platform packaging.

## Key References

| File | Content |
|------|---------|
| `SKILL.md` | Skill definition, symptom map, workflow |

## Key Inspection Points

- `src-tauri/src/lib.rs` — shared entrypoint, command registration
- `src-tauri/src/main.rs` — desktop entry
- `src-tauri/tauri.conf.json` — config, devUrl, frontendDist
- `src-tauri/capabilities/*.json` — permission capabilities
- `src-tauri/Cargo.toml` — crate dependencies

## Common Issues

| Symptom | Check |
|---------|-------|
| `command not found` | `invoke_handler` list vs frontend `invoke()` name |
| `permission denied` | capability file, permission ID, window label match |
| White screen | `devUrl`, `frontendDist`, `beforeDevCommand`, CSP |
| Plugin API fails | `.plugin(...)` registration, guest package import |

## Commands

```bash
pnpm tauri info        # confirm v2 state
pnpm tauri add <plugin> # add plugin with aligned scaffolding
```
