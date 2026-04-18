# API Reference / 接口参考

当前 Go 服务端的标准 OpenAPI 文档位于：

- `docs/api/openapi.json`
- `docs/api/openapi.yaml`

## Scope / 覆盖范围

该文档基于当前 `src-go/internal/server/routes.go` 的真实路由注册结果生成，覆盖：

- 健康检查 / Health
- Auth / Users
- Projects / Members / Tasks
- Custom Fields / Views / Forms / Automations
- Dashboards / Milestones / Sprints / Workflow
- Memory / Logs / Wiki
- Agents / Bridge / Teams / Notifications
- Scheduler / Reviews / Stats
- Roles / Plugins / Marketplace / IM
- Internal HTTP endpoints
- WebSocket 握手入口

## Notes / 注意事项

- OpenAPI 版本：`3.1.0`
- 默认本地服务地址：`http://localhost:7777`
- 大多数 `/api/v1/**` 端点需要 `Authorization: Bearer <token>`
- WebSocket **握手入口** 已记录在 OpenAPI 中，但握手后的消息帧协议不属于 OpenAPI 标准；若后续需要完整事件流规范，建议补充 AsyncAPI 文档
- 某些插件、Bridge、Workflow、IM 相关接口本身是动态 JSON / 插件驱动结构，因此在 OpenAPI 中使用了较宽松的 schema 来保持与当前后端真实行为一致，而不是伪造过度精确的固定字段

## Quick Use / 快速使用

### Swagger UI

可直接将 `openapi.json` 或 `openapi.yaml` 导入 Swagger UI / Redoc。

### Code Generation

可用于：

- OpenAPI Generator
- Swagger Codegen
- Postman import
- API Gateway / MCP / integration tooling

## Topical guides

Long-form guides covering cross-cutting concerns:

- [`rbac.md`](./rbac.md) — Project access control, action→role matrix,
  permissions endpoint, last-owner protection, error codes.
- [`audit.md`](./audit.md) — Project audit log: storage, query API,
  emission model, sink degradation signals, redaction denylist.
- [`project-templates.md`](./project-templates.md) — Project configuration
  snapshots: storage, `POST /projects` clone params, save-as-template
  endpoint, marketplace install seam, author guide.
- [`invitations.md`](./invitations.md) — Member invitation flow: state
  machine, endpoints, token semantics, identity matching, delivery
  fallback, error codes.

## Source of Truth / 真相源

如果代码和文档出现差异，以当前 Go 路由注册与 DTO 为准：

- `src-go/internal/server/routes.go`
- `src-go/internal/handler/`
- `src-go/internal/model/`


## AsyncAPI / 事件流规范

当前 Go 服务端的异步 / 实时消息文档位于：

- `docs/api/asyncapi.yaml`

### Scope / 覆盖范围

该文档覆盖当前后端的事件流与 websocket 通道：

- `/ws`：前端实时推送通道
- `/ws/bridge`：TS bridge -> Go 的 runtime event ingress
- `/ws/im-bridge`：Go -> IM bridge delivery stream，以及 bridge -> Go 的 websocket ack

### Relationship To OpenAPI / 与 OpenAPI 的关系

- `openapi.json` / `openapi.yaml`：描述 HTTP API
- `asyncapi.yaml`：描述 websocket / bridge / IM realtime message flow

如果某条能力既有 HTTP 控制面、又有 websocket 事件面：
- HTTP 路由以 OpenAPI 为准
- websocket / delivery / event envelope 以 AsyncAPI 为准
