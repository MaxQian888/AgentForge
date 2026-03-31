# Architecture Decision Records / 架构决策记录

Use [TEMPLATE.md](./TEMPLATE.md) when adding a new ADR. New records should use a
zero-padded numeric prefix and a kebab-case slug.

## Index

| ADR | Status | Summary |
| --- | --- | --- |
| [0001](./0001-why-tauri-not-electron.md) | Accepted | Use Tauri 2 instead of Electron for the desktop shell |
| [0002](./0002-why-go-nextjs-dual-stack.md) | Accepted | Split the product into a Go orchestrator and Next.js dashboard |
| [0003](./0003-why-wasm-plugin-runtime.md) | Accepted | Use a WASM runtime for Go-hosted plugin execution |
| [0004](./0004-why-jwt-redis-auth.md) | Accepted | Combine JWT access tokens with Redis-backed revocation and refresh state |
| [0005](./0005-why-bun-ts-bridge.md) | Accepted | Use Bun for the TS bridge runtime and distribution path |
