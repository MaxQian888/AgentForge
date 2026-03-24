## Why

当前仓库已经有 Go 后端的注册、登录、刷新、登出和 JWT 中间件，但前端会话层仍停留在最小实现，和后端 `accessToken`/`refreshToken` 契约并不一致，Dashboard 保护也主要依赖客户端持久化布尔值，导致鉴权链路既不完整也不够稳健。现在补齐这条前后端鉴权闭环，能把登录态、令牌刷新、登出失效和受保护页面访问统一到一套可验证、可安全演进的实现里。

## What Changes

- 对齐前端与后端的鉴权响应和存储契约，统一使用访问令牌、刷新令牌和用户身份信息，而不是继续依赖单一 `token` 字段的脆弱约定。
- 补全前端会话生命周期，包括登录后初始化、页面刷新后的身份恢复、令牌过期后的刷新、登出后的本地状态清理，以及受保护页面的可靠跳转与失效处理。
- 加固后端鉴权行为，明确刷新、登出、黑名单、缓存不可用和开发/生产配置差异下的安全边界，避免出现静默失效或错误语义不清的情况。
- 为鉴权相关接口、状态层和关键页面补充测试与文档，确保登录、注册、刷新、登出和 `/api/v1/users/me` 身份校验在 web 与 Tauri 入口下都具备一致的可验证行为。

## Capabilities

### New Capabilities
- `auth-session-management`: 定义 AgentForge 前后端统一的登录、注册、会话恢复、令牌刷新、登出失效和受保护访问行为。

### Modified Capabilities
- None.

## Impact

- Affected frontend routes: `app/(auth)/login/page.tsx`, `app/(auth)/register/page.tsx`, `app/(dashboard)/layout.tsx`, and any auth-aware navigation/header flows.
- Affected frontend state and transport: `lib/stores/auth-store.ts`, stores that read the auth token, `lib/api-client.ts`, and `hooks/use-backend-url.ts`.
- Affected backend auth surface: `src-go/internal/service/auth_service.go`, `src-go/internal/handler/auth.go`, `src-go/internal/middleware/jwt.go`, `src-go/internal/server/routes.go`, `src-go/internal/config/config.go`, and `src-go/internal/repository/cache.go`.
- Affected validation and ops: auth-related tests, environment/config examples, and any documentation that describes login/session behavior or degraded startup semantics.
