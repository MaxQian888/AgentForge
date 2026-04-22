# src-tauri/AGENTS.md

## Tauri Desktop Guidelines

### Structure & Organization

- `src/lib.rs` — shared entrypoint, command registration, plugin setup
- `src/main.rs` — desktop entry
- `src/bin/agentforge-desktop-cli.rs` — standalone CLI
- Capabilities in `capabilities/`

### Commands

```bash
pnpm tauri info        # Check environment
pnpm tauri dev         # Dev mode
pnpm tauri build       # Build installer
cargo test             # Rust unit tests
```

### Code Style

- Rust: `snake_case` for functions/variables, `PascalCase` for types/traits
- Tauri commands: `#[tauri::command]` with explicit input types
- Keep `lib.rs` platform-agnostic; desktop/mobile differences branch in `main.rs` or platform modules

### Testing

- Rust tests via `cargo test`
- Command handler tests: mock state/manage objects
- Run with `pnpm test:tauri` or `pnpm test:tauri:coverage`

### Security

- Minimize capabilities in `capabilities/*.json`
- Avoid broad filesystem access; scope paths narrowly
- Never expose internal APIs without permission checks

### Sidecars

- Go orchestrator (7777), TS Bridge (7778), IM Bridge (7779)
- Sidecar lifecycle managed in `lib.rs`; handle crashes and restarts

### Commit Style

- Conventional Commits: `feat:`, `fix:`, `refactor:`, `chore:`, `docs:`
