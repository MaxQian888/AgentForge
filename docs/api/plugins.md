# Plugin API / 插件 API

This document covers the plugin control plane wired in
`src-go/internal/server/routes.go` and `src-go/internal/handler/plugin_handler.go`.

## Overview

Supported plugin kinds:

- `RolePlugin`
- `ToolPlugin`
- `WorkflowPlugin`
- `IntegrationPlugin`
- `ReviewPlugin`

Supported runtime families:

- `wasm`
- `mcp`
- `go-plugin`
- `declarative`

## Primary Record

Installed plugin responses are centered on `PluginRecord`, which includes:

- manifest metadata and spec
- permissions and source metadata
- `lifecycle_state`
- `runtime_host`
- `last_health_at`, `last_error`, `restart_count`
- `resolved_source_path`
- `runtime_metadata`
- `current_instance`
- `builtIn`

## Discovery, Catalog, And Marketplace

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/plugins/discover` | Discover built-in plugins |
| `POST` | `/api/v1/plugins/discover/builtin` | Refresh built-in discovery |
| `GET` | `/api/v1/plugins/catalog?q=<query>` | Search installable catalog entries |
| `POST` | `/api/v1/plugins/catalog/install` | Install a catalog entry |
| `GET` | `/api/v1/plugins/marketplace` | List marketplace entries |
| `GET` | `/api/v1/plugins/marketplace/remote` | Query remote registry availability and entries |
| `POST` | `/api/v1/plugins/marketplace/:id/install-remote` | Install from the remote registry |
| `GET` | `/api/v1/plugins` | List installed plugins |

`GET /api/v1/plugins` supports filters:

- `kind`
- `state`
- `source`
- `trust`

## Install, Update, And Uninstall

### `POST /api/v1/plugins/install`

Supported request shapes:

```json
{
  "path": "./plugins/tools/github-tool/manifest.yaml"
}
```

```json
{
  "entry_id": "catalog-entry-id"
}
```

```json
{
  "path": "https://registry.example/plugin.tgz",
  "source": {
    "type": "git",
    "repository": "https://github.com/org/plugin.git",
    "ref": "main"
  }
}
```

### `POST /api/v1/plugins/:id/update`

```json
{
  "path": "./plugins/tools/github-tool/manifest.yaml",
  "source": {
    "type": "local"
  }
}
```

### `DELETE /api/v1/plugins/:id`

Response:

```json
{
  "message": "plugin uninstalled"
}
```

## Lifecycle Operations

| Method | Path | Purpose |
| --- | --- | --- |
| `PUT` / `POST` | `/api/v1/plugins/:id/enable` | Enable a plugin |
| `PUT` / `POST` | `/api/v1/plugins/:id/disable` | Disable a plugin |
| `POST` | `/api/v1/plugins/:id/activate` | Activate the runtime instance |
| `POST` | `/api/v1/plugins/:id/deactivate` | Deactivate the runtime instance |
| `GET` | `/api/v1/plugins/:id/health` | Run a health check |
| `POST` | `/api/v1/plugins/:id/restart` | Restart the runtime instance |
| `PUT` | `/api/v1/plugins/:id/config` | Update plugin config |
| `POST` | `/api/v1/plugins/:id/invoke` | Invoke a plugin operation |
| `GET` | `/api/v1/plugins/:id/events` | List recent lifecycle events |

### `PUT /api/v1/plugins/:id/config`

```json
{
  "config": {
    "mode": "webhook"
  }
}
```

### `POST /api/v1/plugins/:id/invoke`

```json
{
  "operation": "health",
  "payload": {
    "target": "feishu"
  }
}
```

## MCP Interaction Surface

These endpoints matter for MCP-backed plugins:

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/plugins/:id/mcp/refresh` | Refresh tools/resources/prompts |
| `POST` | `/api/v1/plugins/:id/mcp/tools/call` | Call an MCP tool |
| `POST` | `/api/v1/plugins/:id/mcp/resources/read` | Read an MCP resource |
| `POST` | `/api/v1/plugins/:id/mcp/prompts/get` | Read an MCP prompt |

Request examples:

```json
{
  "tool_name": "echo",
  "arguments": {
    "text": "hello"
  }
}
```

```json
{
  "uri": "file://README.md"
}
```

```json
{
  "name": "review:run",
  "arguments": {
    "review_id": "review-123"
  }
}
```

## Workflow Runs

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/plugins/:id/workflow-runs` | Start a workflow-plugin run |
| `GET` | `/api/v1/plugins/:id/workflow-runs` | List runs for a workflow plugin |
| `GET` | `/api/v1/plugins/workflow-runs/:runId` | Fetch a single run |

Workflow start body:

```json
{
  "trigger": {
    "source": "manual"
  }
}
```

## Remote Registry Error Codes

Remote installs can return:

- `remote_registry_unconfigured`
- `remote_registry_unavailable`
- `remote_registry_download_failed`
- `remote_registry_invalid_artifact`
- `remote_registry_verification_failed`

Clients should branch on HTTP status first, then on `errorCode` when present.
