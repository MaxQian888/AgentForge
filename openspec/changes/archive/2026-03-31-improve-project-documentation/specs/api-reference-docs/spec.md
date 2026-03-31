## ADDED Requirements

### Requirement: API 文档覆盖所有 Go 后端 REST 端点
系统 SHALL 在 `docs/api/` 目录下提供按路由模块组织的 API 参考文档。每个模块文件 SHALL 列出该模块下所有 HTTP 端点，包含：HTTP 方法、路径、请求参数（query/path/body）、响应格式（成功 + 错误）、认证要求、示例请求/响应。

#### Scenario: 开发者查看认证模块 API 文档
- **WHEN** 开发者打开 `docs/api/auth.md`
- **THEN** 文档包含 `/api/v1/auth/login`、`/api/v1/auth/register`、`/api/v1/auth/refresh`、`/api/v1/auth/logout` 的完整参考，含请求/响应 JSON 示例

#### Scenario: 开发者查看任务模块 API 文档
- **WHEN** 开发者打开 `docs/api/tasks.md`
- **THEN** 文档包含 CRUD 端点、任务分解、任务状态变更、任务评论等端点的完整参考

### Requirement: API 文档包含统一错误码参考
系统 SHALL 在 `docs/api/errors.md` 中提供统一的错误码参考表，覆盖所有 HTTP 错误响应的 code、message 模板和含义。

#### Scenario: 开发者排查 API 错误
- **WHEN** 前端收到 HTTP 403 响应，body 中包含 `code: "FORBIDDEN_RESOURCE"`
- **THEN** 开发者能在 `docs/api/errors.md` 中查到该 code 的含义和排查建议

### Requirement: API 文档标注认证要求
每个端点的文档 SHALL 明确标注认证要求：无需认证、需要 Access Token、需要 Refresh Token。

#### Scenario: 识别公开端点与受保护端点
- **WHEN** 开发者阅读任意端点文档
- **THEN** 文档清晰标注该端点的认证要求，包含 Authorization header 格式示例
