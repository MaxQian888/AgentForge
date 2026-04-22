# src-bridge/AGENTS.md

## Bridge Service Guidelines

### Structure & Organization

- Hono framework on Bun runtime
- `src/server.ts` entry point
- Runtime adapters in `src/runtime/`; ACP integration in `src/runtime/acp/`
- Handlers in `src/handlers/`

### Commands

```bash
bun run src/server.ts          # Start dev server
bun test                       # Run tests
bun run lint                   # Lint (if configured)
```

### Code Style

- TypeScript strict mode
- Prefer `type` over `interface` for data shapes unless declaration merging is needed
- Use `zod` for runtime validation of inbound payloads
- ACP handlers: keep capability checks explicit and log permission denials

### Testing

- Co-locate tests with source
- Mock ACP client and runtime adapters
- Cover error paths in handlers (malformed payloads, missing fields)

### Runtime Adapters

- ACP is the primary integration path; legacy adapters are deprecated
- New adapter work goes through `src/runtime/acp/adapter-factory.ts`
- Connection pooling and multiplexing are handled by `src/runtime/acp/connection-pool.ts`

### Commit Style

- Conventional Commits: `feat:`, `fix:`, `refactor:`, `chore:`, `docs:`
