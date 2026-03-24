## Context

AgentForge 现在已经具备基础的邮箱注册、密码登录、JWT 鉴权中间件、refresh token 轮换和登出黑名单能力，但这些能力还没有形成统一的前后端会话模型：

- 后端 `AuthResponse` 返回 `accessToken`、`refreshToken` 和 `user`，前端 `lib/stores/auth-store.ts` 却仍按单个 `token` 字段消费，契约已经偏离。
- `app/(dashboard)/layout.tsx` 只根据客户端持久化的 `isAuthenticated` 做跳转，页面刷新、过期令牌、跨端启动和服务端真实会话状态都没有被重新校验。
- `GET /api/v1/users/me` 目前只从 JWT claims 返回 `id` 和 `email`，不能作为前端恢复会话时的权威身份接口。
- Redis 缓存对 refresh token 和 access-token 黑名单都属于安全关键依赖，但当前 `IsBlacklisted` 在缓存不可用时会 fail-open，和“高安全性”的目标不一致。
- 仓库同时支持 web 与 Tauri 入口，auth 路径需要和后端 URL 发现机制兼容，而不是继续把鉴权流固定死在 `NEXT_PUBLIC_API_URL` 的静态值上。

这说明本次变更最合适的方向不是重做整套认证架构，而是在现有 JWT + refresh 模型上补齐前后端契约、会话恢复与安全边界。

## Goals / Non-Goals

**Goals:**

- 建立单一、明确的前后端鉴权契约，统一登录、注册、刷新、登出和身份恢复使用的字段与错误语义。
- 让前端在 web 和 Tauri 两种入口下都能完成会话初始化、身份校验、令牌刷新、失效清理和受保护页面跳转。
- 让 `/api/v1/users/me` 成为权威身份恢复接口，而不是依赖前端缓存的用户快照。
- 明确缓存/黑名单不可用时的安全行为，避免“令牌撤销依赖失效但请求仍继续放行”的静默风险。
- 为 auth store、关键 handler/service/middleware 和页面保护补齐针对性测试。

**Non-Goals:**

- 不在本次变更中引入第三方 OAuth、SSO 或角色权限系统重构。
- 不把整个仓库迁移到 httpOnly cookie + BFF 模式；本次继续沿用现有 Bearer token + refresh token 架构。
- 不顺带重写所有业务 store 的数据访问方式，只聚焦鉴权链路和直接依赖鉴权状态的入口。
- 不改变当前 PostgreSQL/Redis 可选启动的总体运维策略，但会重新定义安全关键鉴权路径在降级模式下的响应方式。

## Decisions

### 1. 以后端 `AuthResponse` 作为唯一权威的前端会话契约

前端会话状态统一以 `accessToken`、`refreshToken`、`user` 和一个显式的会话状态字段（如 `idle`/`checking`/`authenticated`/`unauthenticated`）为核心，而不是继续保留“单 token + 布尔值”这种容易失真的结构。登录、注册和刷新都必须产出同一份可持久化的会话载荷；登出和刷新失败则统一走清理路径。

这样做的原因：

- 可以直接复用后端已存在的 `AuthResponse`，避免再维护一层前端私有协议。
- 能让刷新、重载恢复、登出失效都围绕同一状态机工作，减少每个页面和 store 分别处理 401 的重复逻辑。

备选方案：

- 继续使用当前 `token` 字段并手动映射 `accessToken`。实现快，但会继续放大字段不一致和 refresh token 丢失的问题。
- 改成完全不持久化任何 token。安全性更高，但会直接破坏当前 web/Tauri 的实际使用方式，本次不做。

### 2. 使用集中式会话恢复流程，而不是让页面或业务 store 各自猜测登录态

前端在应用启动和进入受保护布局时执行统一的会话恢复流程：

1. 读取已持久化的 access/refresh token。
2. 若没有 token，直接进入未登录态。
3. 若有 access token，先调用 `/api/v1/users/me` 验证并获取权威用户信息。
4. 若 access token 无效但 refresh token 仍存在，则调用 `/api/v1/auth/refresh` 获取新 token 对，再重试 `/api/v1/users/me`。
5. 若刷新失败或身份验证失败，则清理本地会话并跳转登录页。

`app/(dashboard)/layout.tsx` 必须等待这个流程完成后再决定渲染或重定向，避免“闪一下再跳”或凭本地布尔值误放行。

备选方案：

- 让每个业务 store 在请求失败时自行决定是否刷新。这样会造成重复实现、竞态和不一致的错误处理。
- 继续仅靠 `isAuthenticated` 本地布尔值守卫 Dashboard。实现最简单，但无法处理令牌过期、跨端恢复和服务端会话失效。

### 3. `/api/v1/users/me` 改为权威身份接口，按 subject 重新加载用户

当前 `GetMe` 只从 claims 回显 `id` 和 `email`，无法返回稳定的用户资料，也无法保证前端恢复的是数据库中的最新身份信息。本次设计要求它以 JWT subject 为入口重新读取用户仓库，并返回完整 `UserDTO`，使其成为前端会话恢复和身份重建的权威接口。

这样做可以：

- 让登录后、刷新后、重载后都能拿到同一份用户资料结构。
- 避免前端长期依赖过期的本地用户对象。
- 为后续增加用户资料字段或账户状态校验留下 seam。

备选方案：

- 继续只回显 claims，并把用户对象完全交给前端持久化。这样无法校正过期资料，也不利于后续账户状态检查。

### 4. 对撤销和刷新依赖采取 fail-closed 策略，但保持整体服务可启动

为了满足“安全性高”，本次把 Redis 相关鉴权依赖分成两层：

- 服务整体仍可在 PostgreSQL/Redis 不可用时启动，以保留现有开发与运维弹性。
- 但凡涉及 refresh token 校验、access token 撤销判断或登出撤销写入的路径，都必须在缓存不可用时显式拒绝，而不是静默放行或假装成功。

这意味着：

- `refresh` 在无法读取已保存 refresh token 时必须返回明确的失败语义，不得签发新 token。
- `logout` 在无法完成关键撤销步骤时必须返回失败，由前端清理本地状态并提示用户重新登录。
- 受 JWT 保护的路由在无法完成黑名单校验时不得把请求当成“未撤销”继续放行。

备选方案：

- 保留当前 blacklist fail-open 语义。运维弹性更强，但被撤销 token 可能在 Redis 异常时继续访问受保护接口，不符合这次目标。
- 把 Redis 变成强制启动依赖。安全边界更简单，但会改变仓库整体启动策略，超出本次范围。

### 5. 抽出可复用的 backend URL 解析能力，供鉴权链路统一使用

鉴权请求不能继续只依赖 `NEXT_PUBLIC_API_URL`，否则 Tauri 模式下登录、刷新和身份恢复会命中错误地址。本次设计采用“抽出一个可在 store 和组件中共享的 backend URL resolver，再由 auth 流程统一调用”的方式，让 auth 相关网络请求与现有 `useBackendUrl` 行为一致。

备选方案：

- 保持 auth store 使用静态 URL，只在页面组件里处理动态 URL。这样 store 内的 login/register/refresh 仍无法正确运行。
- 将所有 store 一次性切换到新的 resolver。长期是合理方向，但本次先聚焦 auth 链路，避免 scope 膨胀。

## Risks / Trade-offs

- [fail-closed 会让 Redis 异常时更多鉴权请求直接失败] → 用明确错误响应、日志和文档说明这是安全优先的显式策略，而不是偶发故障。
- [前端会话恢复增加一次或两次启动请求] → 通过仅在存在持久化 token 时执行，并把逻辑集中到 layout/bootstrap，避免页面级重复请求。
- [`/users/me` 改为查库后会增加后端依赖] → 这是恢复权威身份所必需的成本；通过复用现有 user repository 保持实现集中。
- [auth store 契约调整会波及多个业务 store] → 保持对外暴露稳定的“获取当前 access token”接口，优先减少业务 store 改动面。
- [web 与 Tauri backend URL 统一可能牵出更广的 transport 重构] → 仅抽 auth 所需的 resolver seam，本次不把所有数据请求都纳入改造。

## Migration Plan

1. 调整前端 auth state 结构与辅助方法，对齐后端 `AuthResponse`。
2. 引入统一的 session bootstrap/refresh 流程，并接入 `app/(dashboard)/layout.tsx`。
3. 扩展 backend URL resolver，确保 login/register/refresh/getMe/logout 在 web 与 Tauri 下都走一致地址。
4. 更新后端 `GetMe`、refresh/logout/cache 相关实现与错误映射，收紧黑名单和 refresh 的安全语义。
5. 为前端 store、布局守卫和后端 service/handler/middleware 增加针对性测试。
6. 更新开发配置与文档，说明 token 字段、恢复流程和缓存不可用时的行为。

回滚策略：

- 前端可回退到旧 auth store 契约和原有 Dashboard 守卫。
- 后端若需快速回退，可恢复当前 `GetMe` 和缓存降级语义，但应同时回退对应测试与文档，避免行为失配。

## Open Questions

- 前端是否需要在本次变更中加入“单飞中刷新”去重机制，避免并发请求同时触发多次 refresh；当前设计默认可先用串行化或单例 promise 解决。
- 登出在撤销写入失败时，前端提示文案应该偏“已清理本地会话，请重新登录”还是偏“服务暂时不可用”；本次先约束安全语义，不强制具体文案。
- 是否要把 refresh endpoint 也纳入单独的速率限制；这属于合理硬化项，但可以在实现时根据现有测试和风控需求决定是否一起落地。
