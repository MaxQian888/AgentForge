# API Error Reference / 统一错误参考

This file documents the current error envelope and the domain-specific error
codes that already exist in AgentForge today.

## Standard Error Envelope

Most handlers return:

```json
{
  "message": "human-readable message",
  "code": 400
}
```

Current repository truth:

- `message` is the primary operator-facing error detail
- `code` is usually the HTTP status integer echoed into the body
- string business error codes exist only on some specialized responses

## Common HTTP Status Semantics

| HTTP status | Meaning in this repo | Typical examples |
| --- | --- | --- |
| `400` | malformed request or invalid IDs | bad UUID, malformed JSON, invalid transition |
| `401` | auth failed | invalid credentials, invalid token |
| `404` | resource missing | task, project, review, agent run |
| `409` | conflict / guardrail block | duplicate email, pool full, invalid review state |
| `422` | validation failure | missing required fields, enum mismatch |
| `500` | server failure | repository/service failure |
| `502` | bridge or downstream runtime failure | invalid decomposition, failed agent start |
| `503` | required dependency unavailable | DB down, Redis unavailable, bridge unavailable |

## Domain-Specific Codes Already In Use

### Remote Registry Install Errors

`POST /api/v1/plugins/marketplace/:id/install-remote` can return:

| `errorCode` | Meaning |
| --- | --- |
| `remote_registry_unconfigured` | remote registry URL not configured |
| `remote_registry_unavailable` | remote registry could not be reached |
| `remote_registry_download_failed` | artifact download failed |
| `remote_registry_invalid_artifact` | artifact shape or manifest invalid |
| `remote_registry_verification_failed` | digest/signature or trust verification failed |

Example response:

```json
{
  "ok": false,
  "pluginId": "plugin-id",
  "version": "latest",
  "errorCode": "remote_registry_verification_failed",
  "message": "signature check failed"
}
```

### Agent Bridge Availability

Some agent endpoints return:

```json
{
  "error": "bridge_unavailable"
}
```

### Runtime Diagnostics

Bridge/runtime catalog responses can contain diagnostic `code` values such as:

- `missing_executable`
- `missing_runtime_catalog`
- `unknown_runtime`
- `unsupported_probe_provider`
- `sunset_window`
- `runtime_sunset`
- `stale_default_selection`

These appear inside diagnostics arrays rather than top-level API error bodies.

## Client Guidance

Handle errors in this order:

1. Branch on the HTTP status code.
2. Read `message` for operator-facing context.
3. If present, branch on specialized fields such as `errorCode`, runtime
   diagnostic `code`, or the lightweight `error` string returned by bridge-unavailable paths.
