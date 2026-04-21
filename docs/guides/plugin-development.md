# Plugin Development Guide / 插件开发总览

This guide explains the current repo-local plugin authoring flow in AgentForge.

## Choose The Right Plugin Kind

| Kind | Runtime path | Best for |
| --- | --- | --- |
| `ToolPlugin` | MCP / TypeScript | Agent tools exposed through the bridge |
| `ReviewPlugin` | MCP / TypeScript | Review rules and findings generation |
| `WorkflowPlugin` | WASM / Go | Multi-step orchestration owned by the Go control plane |
| `IntegrationPlugin` | WASM / Go | External-system adapters owned by Go |
| `RolePlugin` | Declarative | Role metadata and prompt/skill composition |

## Current Starters

Use the repo-local scaffold:

```bash
pnpm create-plugin -- --type tool --name echo-tool
pnpm create-plugin -- --type review --name architecture-check
pnpm create-plugin -- --type workflow --name standard-dev-flow
pnpm create-plugin -- --type integration --name sample-integration-plugin
```

Generated locations:

- TypeScript/MCP plugins: `plugins/tools/<name>/` or `plugins/reviews/<name>/`
- Go/WASM plugins: plugin manifest under `plugins/.../<name>/` and entrypoint
  under `src-go/cmd/<name>/`

## Hello World Paths

### Tool / Review (MCP)

1. Run `pnpm create-plugin -- --type tool --name hello-tool`
2. Inspect `plugins/tools/hello-tool/manifest.yaml`
3. Implement behavior in `plugins/tools/hello-tool/src/index.ts`
4. Run `bun test` inside the plugin directory
5. Install through the control plane or plugin UI

### Workflow / Integration (WASM)

1. Run `pnpm create-plugin -- --type workflow --name hello-flow`
2. Inspect `plugins/workflows/hello-flow/manifest.yaml`
3. Implement the Go entrypoint in `src-go/cmd/hello-flow/main.go`
4. Build with `pnpm plugin:build -- --manifest ...`
5. Debug with `pnpm plugin:debug -- --manifest ... --operation health`

## Authoring Rules

- keep manifests as the primary source of plugin metadata
- declare permissions explicitly
- prefer the maintained root scripts instead of ad-hoc local commands
- keep starter tests updated so template drift is visible
