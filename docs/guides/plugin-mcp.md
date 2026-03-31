# MCP Plugin Guide / MCP Tool 与 Review 插件指南

This guide covers the bridge-hosted plugin path used for `ToolPlugin` and
`ReviewPlugin`.

## Generated Layout

Running:

```bash
pnpm create-plugin -- --type tool --name echo-tool
```

creates:

- `plugins/tools/echo-tool/manifest.yaml`
- `plugins/tools/echo-tool/package.json`
- `plugins/tools/echo-tool/src/index.ts`
- `plugins/tools/echo-tool/src/index.test.ts`

Review plugins are generated under `plugins/reviews/<name>/`.

## Manifest Essentials

```yaml
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: echo-tool
  name: Echo Tool
  version: 0.1.0
spec:
  runtime: mcp
  transport: stdio
  command: bun
  args:
    - run
    - src/index.ts
permissions: {}
source:
  type: local
  path: ./plugins/tools/echo-tool/manifest.yaml
```

## TypeScript Entry Contract

The scaffolded entrypoint uses the bridge plugin SDK to:

- define the manifest
- create an MCP server
- expose tools or review entrypoints
- connect the server over stdio

Review plugins also declare:

- review entrypoint
- trigger events
- file patterns
- output format

## Test And Type Safety

Use the generated package scripts:

- `bun run src/index.ts`
- `bun test src/index.test.ts`

The scaffold routes through the local plugin harness so MCP shapes stay normalized.
