## Context

`src-bridge` 里的 OpenCode runtime 已经完成了 execute、pause/resume continuity、基础 event normalize，以及一批 canonical interaction routes；但当前 control-plane 仍有四个关键断点。

第一，OpenCode provider readiness 现在主要停留在 `checkReadiness()` 的 blocking diagnostics：Bridge 知道 provider 未连接或需要认证，但还没有 Bridge-owned 的 auth handshake，所以 catalog 只能“报错”，不能把问题推进成可操作流程。第二，`/bridge/permission-response/:request_id` 现在只会解 `HookCallbackManager` 的 pending request，这条路本质上仍然是 Claude callback 语义；OpenCode transport 已经有 `respondToPermission()`，但没有真实的 request-id → session-id / permission-id 绑定闭环。第三，OpenCode 的 messages、diff、command、shell、revert 等 server-backed controls 当前都只从 active runtime pool 里找 task；任务 pause 之后 runtime 被释放，虽然 continuity 还在，但这些 control route 会退化成 `task not found`。第四，runtime catalog 对 OpenCode discovery 仍然是 best-effort enrichment：`agents`、`skills`、`providers` 取不到时会静默吞掉，导致上游无法区分“确实没有”与“Bridge 没拿到”。

这次 change 只收口 `src-bridge` 自己拥有的 OpenCode seam，不扩成 Go proxy、frontend、或新的 runtime 工作。重点是把现有 route、transport、snapshot 和 catalog 提升成真实、稳定、可诊断的 OpenCode control plane。

## Goals / Non-Goals

**Goals:**
- 让 OpenCode provider auth 从 blocking unavailable 提升为 Bridge 可引导、可完成、可回报结果的 control-plane handshake。
- 让 OpenCode permission request / response 形成真实 round-trip，避免继续借用 Claude-only callback 假象。
- 让 OpenCode 的 canonical interaction routes 在 pause 之后仍能通过 persisted continuity 找到原 upstream session。
- 让 OpenCode runtime catalog 对 discovery / auth / session-control 的缺失原因发布明确 degraded diagnostics，而不是静默 best-effort 缺字段。
- 保持当前 `/bridge/*` 主 contract 兼容：已有 route 不重命名，新增 surface 采用 additive 方式。

**Non-Goals:**
- 不修改 Go backend 的 proxy、缓存或 frontend 的 runtime selector 消费逻辑。
- 不扩展 Claude / Codex / 其他 CLI runtime 的能力矩阵，除非为了复用通用 error shape。
- 不在这次 change 内引入 Bridge-managed OpenCode server bootstrap、sidecar lifecycle、或新的长期凭据存储。
- 不把 paused OpenCode task 重新放回 active pool 才允许查询控制；目标是 session-backed control，而不是“恢复成运行态”。

## Decisions

### Decision 1: 为 OpenCode 引入 session-backed control resolver，而不是继续把所有 control route 绑死在 active runtime pool

当前 messages / diff / command / shell / revert 等 route 都通过 `pool.get(task_id)` 解析 runtime。这对正在运行的 task 足够，但对 paused OpenCode task 会直接丢掉已保存的 upstream session 绑定。新的 resolver 将按以下顺序解析：

1. active runtime（如果任务仍在 pool 中）；
2. persisted snapshot continuity（如果任务已 pause/release）；
3. 显式 continuity error（如果 task 存在但没有可用的 OpenCode session binding）。

这样 OpenCode server-backed controls 能继续围绕同一 upstream session 工作，而不是退化成“只有运行中任务才能看消息/执行命令”。

**Alternative considered:** pause 后强制先 resume，或为 control route 临时重建 active runtime。  
**Rejected because:** 这会把只读/轻量 control 操作错误地升级成一次新的运行生命周期，既不真实，也会放大状态复杂度。

### Decision 2: OpenCode permission / auth pending state 与 Claude hook callback state 分离

`HookCallbackManager` 当前的语义是“向外 POST 回调 URL，然后等待 `/bridge/permission-response/:request_id` 返回结果”。Claude hooks 适合这套模式，但 OpenCode permission response 还需要持有 upstream `session_id` / `permission_id`，provider auth 还可能需要保存 provider 标识、auth URL 与 callback payload。  

因此这次 change 会引入 OpenCode 自己的 pending interaction store，按 `request_id` 记录：

- `kind`: `permission` 或 `provider_auth`
- `session_id` / `permission_id` / `provider`
- timeout、created_at、清理状态

Bridge 继续保留统一的 request-id 驱动体验，但内部不会再把 OpenCode 事件硬塞进 Claude 的 callback manager。

**Alternative considered:** 扩展 `HookCallbackManager` 让它同时承载 Claude 与 OpenCode。  
**Rejected because:** 两者的上游语义不同；硬合并只会让 request lifecycle、payload shape 和错误处理继续混在一起。

### Decision 3: Provider auth 采用 Bridge-owned additive routes，而不是只靠 execute-time blocking errors

OpenCode transport 已经能发现 provider catalog 和 auth methods，但缺少让调用方真正完成认证的 canonical Bridge control surface。这次 change 采用 additive 的 Bridge-owned auth handshake：

- start auth：Bridge 请求上游 authorize surface，产出 request id、provider、auth URL / metadata；
- complete auth：Bridge 接收 callback payload，转发到上游 callback surface，并刷新相关 provider/catalog 状态。

这样 execute readiness 与 runtime catalog 都可以从“未认证，无法启动”升级为“未认证，但存在可执行的恢复路径”。

**Alternative considered:** 继续把 provider auth 只表现为 diagnostics，让外部系统绕过 Bridge 自己处理 OAuth。  
**Rejected because:** 这样会让 TS Bridge runtime contract 缺一块关键控制面，也会继续制造 catalog truth 与实际可操作能力不一致的问题。

### Decision 4: OpenCode catalog truthfulness 优先于“尽量显示更多字段”

当前 `getCatalogDetails()` 中的 agents / skills / providers 获取失败时会静默吞掉异常。这次 change 改成：  

- OpenCode execute readiness 仍由 health / provider selection 决定；
- discovery failure 不必一律让 runtime 整体 unavailable；
- 但 catalog 必须发布明确的 degraded diagnostics / reason codes，让调用方知道是 provider catalog、agent discovery、skill discovery 还是 auth metadata 获取失败。

换句话说，catalog 可以“部分可用”，但不能“缺了也不说”。

**Alternative considered:** 任何 discovery failure 都把 runtime 整体标 unavailable。  
**Rejected because:** 这会把可执行但观测面不完整的情况和真正不可执行的情况混为一谈，反而降低上游决策质量。

### Decision 5: paused-session control 保持 task-oriented canonical routes，不新增平行的 session-id public API

对外仍然使用现有 task-oriented canonical routes，例如 `/bridge/messages/:task_id`、`/bridge/diff/:task_id`、`/bridge/command`、`/bridge/shell`。Bridge 内部负责从 active runtime 或 persisted continuity 找到对应 upstream session。  

这样可以保持 Go / operator / future frontend 消费面稳定，不要求调用方知道 OpenCode 的 session ID。

**Alternative considered:** 暴露新的 `/bridge/opencode/session/:session_id/*` 路由。  
**Rejected because:** 这会把 upstream transport 细节泄露给上游，也会削弱 Bridge 作为 canonical task-bound control plane 的价值。

## Risks / Trade-offs

- **[Risk] OpenCode OAuth callback payload在不同 provider 间差异较大** → **Mitigation:** Bridge 只规范外围 request lifecycle，callback payload 保持 opaque object 透传给 transport。
- **[Risk] paused snapshot 与真实 upstream session 发生漂移** → **Mitigation:** session-backed resolver 失败时返回显式 continuity / session lookup error，不回退成误导性的 `task not found`。
- **[Risk] catalog truthfulness 提升后，调用方会更频繁看到 degraded 状态** → **Mitigation:** 保持 execute readiness 与 discovery diagnostics 分层，避免把非阻塞 discovery 问题提升成整体 unavailable。
- **[Risk] 新的 pending interaction store 引入超时/泄漏风险** → **Mitigation:** 所有 pending request 带 TTL、显式完成、以及在 start/complete failure 时清理。
- **[Risk] 新增 OpenCode auth route 但当前 Go/frontend 还未消费** → **Mitigation:** route 采用 additive 方式；当前 change 先确保 Bridge contract 自洽，消费端可后续接入。

## Migration Plan

1. 先补 OpenSpec delta：明确 OpenCode auth handshake、paused-session controls、catalog truthfulness 与 canonical route contract。
2. 在 `src-bridge` 内引入 OpenCode session control resolver 与 pending interaction store，并把 OpenCode control routes 从“仅 active runtime”切换为“active runtime + persisted continuity”。
3. 扩展 `OpenCodeTransport` 的 provider auth surface，并把 permission response / auth completion 接入真实 upstream transport。
4. 收紧 runtime catalog：将当前 best-effort discovery failure 替换成显式 degraded diagnostics，同时保留 execute readiness 与 current API shape 的兼容性。
5. 增加 focused tests，验证 paused-session route behavior、permission/auth round-trip、catalog degraded diagnostics；必要时用 additive fields 保证旧消费者不立即断裂。

回滚策略：

- 若 auth handshake 的 canonical surface 实现期不稳定，可先保留 route 与 diagnostics contract，但将 start/complete 标为 explicit degraded，而不是恢复到静默 missing。
- 若 paused-session resolver 造成 route 回归，可回退到 active-only lookup，但必须保留结构化 continuity error，不允许重新回到 `task not found` 的假象。

## Open Questions

- OpenCode provider auth 的 canonical Bridge route 最终是否需要单独区分 start / complete / cancel，还是 start+complete 已足够支撑当前 provider 集合？
- 是否要在这次 change 一并暴露 `/bridge/todos/:task_id` 之类的 snapshot query route，还是先把现有 messages / diff / command / shell / revert 收口完成？
- catalog diagnostics 是否需要为 agents / skills / providers 分别暴露独立 reason code，还是先收敛成一组 OpenCode discovery degraded codes 更稳？
