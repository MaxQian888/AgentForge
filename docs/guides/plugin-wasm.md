# WASM Plugin Guide / WASM 插件开发指南

This guide covers the Go-hosted plugin path used for `WorkflowPlugin` and
`IntegrationPlugin`.

## Generated Layout

Running:

```bash
pnpm create-plugin -- --type integration --name my-integration
```

creates:

- `plugins/integrations/my-integration/manifest.yaml`
- `plugins/integrations/my-integration/package.json`
- `src-go/cmd/my-integration/main.go`
- `src-go/cmd/my-integration/main_test.go`

Workflow plugins use the same shape under `plugins/workflows/`.

## Manifest Essentials

```yaml
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: my-integration
  name: My Integration
  version: 0.1.0
spec:
  runtime: wasm
  module: ./dist/my-integration.wasm
  abiVersion: v1
  capabilities:
    - health
permissions: {}
source:
  type: local
  path: ./plugins/integrations/my-integration/manifest.yaml
```

## Go Entrypoint Contract

The starter Go entrypoint implements:

- `Describe`
- `Init`
- `Health`
- `Invoke`

and exports:

- `agentforge_abi_version`
- `agentforge_run`

through the plugin SDK runtime wrapper.

## Build And Debug Loop

```bash
pnpm plugin:build -- --manifest plugins/integrations/feishu-adapter/manifest.yaml
pnpm plugin:debug -- --manifest plugins/integrations/feishu-adapter/manifest.yaml --operation health
pnpm plugin:verify -- --manifest plugins/integrations/feishu-adapter/manifest.yaml
```
