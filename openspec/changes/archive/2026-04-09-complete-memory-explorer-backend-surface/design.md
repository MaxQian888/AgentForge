## Context

AgentForge 当前已经有两条与 memory explorer 直接相关、但尚未闭环的后端能力：

1. `src-go/internal/service/memory_service.go` + `src-go/internal/handler/memory_handler.go` 提供了项目 memory 的最小 `store/search/delete` API，但它只支持简单 query + limit，且 handler 仍读取 `q` 而不是当前前端 store 已发送的 `query`，也没有消费 `scope/category/roleId/date-range` 等 explorer 过滤条件。
2. `src-go/internal/service/episodic_memory_service.go` 已具备 episodic history、按时间范围过滤、retention cleanup、JSON export/import 等能力，但它没有接入 `routes.go`，也没有被当前 memory handler 复用。

与此同时，活跃 change `enhance-frontend-panel` 已把 `memory-explorer-panel` 列为真实产品面，要求 memory explorer 至少具备搜索、过滤、详情、清理、导出和统计等能力。若继续停留在当前最小 API，前端只能基于漂移参数、mock 数据或重复业务逻辑推进，后续 panel 无法做到 truthful backend integration。

约束条件：
- 保持当前 `/api/v1/projects/:pid/memory` 基础路径和 Bearer-token 行为，不破坏已有简单 consumer。
- 优先复用现有 repository 与 `EpisodicMemoryService`，避免另起并行 memory truth。
- 这次聚焦 Go backend contract，不扩成完整前端 panel 交付，也不顺手引入向量检索或长期学习架构。

## Goals / Non-Goals

**Goals:**
- 对齐现有 memory explorer consumer 与 Go handler 的查询契约，支持 `query/q` 兼容、scope/category/role/date-range/limit 等过滤条件。
- 提供 explorer-ready 的后端接口：列表、详情、统计、导出、批量删除、按条件清理。
- 将 `EpisodicMemoryService` 的 history/retention/export 能力接入当前 API surface，避免隐藏能力继续闲置。
- 为前端返回更稳定的 memory DTO/response shape，包含 explorer 渲染所需的时间与元信息摘要。
- 用 focused handler/service/repository tests 锁定 contract，防止再次出现前后端参数漂移。

**Non-Goals:**
- 不在本次 change 中交付完整 `memory-explorer-panel` UI。
- 不实现 semantic vector search、procedural learning、memory tagging、memory editing、CSV export。
- 不引入新的外部存储、搜索引擎或额外数据库表，除非实现中证明现有 schema 无法支撑。
- 不改变现有 short-term memory/token budget 语义。

## Decisions

### 1. 以现有 `/api/v1/projects/:pid/memory` 为根路径扩展 explorer API，而不是新开平行前缀

**Decision:** 保留当前 memory 路由根路径，在其下扩展 explorer 子资源与查询参数：继续兼容 `GET/POST/DELETE /memory`，并增加 detail / stats / export / bulk-delete / cleanup 等子路由。

**Rationale:**
- 当前前端 store、memory panel、鉴权与 API client 已经围绕 `/api/v1/projects/:pid/memory` 建立。
- 使用同一路径族可以把“简单 project memory”与“explorer-ready memory”保持在一个清晰控制面内，避免再出现双路径并存的 contract 漂移。
- 兼容读取旧参数 `q`，同时显式支持 `query`，可以让已有 consumer 无痛过渡。

**Alternatives considered:**
- 新建 `/api/v1/projects/:pid/memory-explorer/*`：语义更“新”，但会制造重复资源面，并让前端需要分支调用两套路径。
- 只改前端把 `query` 改回 `q`：能修当前 bug，但无法解决详情、统计、导出、清理等缺口。

### 2. 采用“通用 memory handler + episodic explorer sub-service”分层，而不是把所有逻辑堆进现有 `MemoryService`

**Decision:** 保留 `MemoryService` 负责通用 project memory store/search/delete；新增/扩展 explorer handler 层时，把 episodic history、retention、export 这类时间序列能力委托给 `EpisodicMemoryService`，再在 handler 或 façade service 聚合通用 DTO/统计结果。

**Rationale:**
- `MemoryService` 当前职责明确：存储、简单搜索、prompt injection、team learning；强行塞入 export/cleanup/history 会让其边界变糊。
- `EpisodicMemoryService` 已经实现了按时间范围查询、retention、export/import，直接复用比复制逻辑更稳。
- 这种分层允许 explorer API 对外呈现统一 contract，同时内部仍沿现有 service seam 演进。

**Alternatives considered:**
- 全部逻辑并回 `MemoryService`：短期文件更少，但会让 service 同时负责通用 memory 和 episodic explorer lifecycle，长期更难维护。
- 只暴露 `EpisodicMemoryService`：会丢掉 semantic/procedural/project memory 的统一入口，不利于 explorer 逐步扩展。

### 3. 这次统计与导出采用“现有数据可直接推导”的轻量模型，而不是先做新 schema

**Decision:** memory stats 先基于现有 repository 数据计算总数、分类分布、scope 分布、最近访问/创建时间、近似存储大小；导出先保证 JSON，且以当前过滤条件为准。

**Rationale:**
- 当前 `AgentMemory` 已包含 `category`、`scope`、`accessCount`、`lastAccessedAt`、`createdAt`、`metadata`，足以支持 explorer 基础统计与导出。
- 使用现有字段可避免这次 change 变成数据库迁移项目，降低 apply 风险。
- JSON export 能直接复用 `EpisodicMemoryExport`，也最接近现有代码真相。

**Alternatives considered:**
- 先加聚合表或 materialized stats：更快，但需要 migration 和额外一致性维护，不适合当前 focused scope。
- 同时支持 CSV：对面板下载更友好，但当前 Go 侧没有对应导出结构，容易把变更做散。

### 4. 详情/批量删除/清理接口默认遵循项目隔离与 role-scope 限制

**Decision:** explorer API 继续以项目 ID 为主隔离边界；role-scoped episodic memory 在 detail/list/export/cleanup 中都必须遵守现有 `MemoryAccessRequest` 规则，不允许无差别跨 role 读取或清理。

**Rationale:**
- `EpisodicMemoryService` 已经有 `ensureMemoryAccess` / `filterAccessibleMemories` 语义，说明 role-scope 是既有 contract，而不是新发明的限制。
- memory explorer 是 operator-facing 面板，但仍不应该绕过已有 role-scoped 访问边界。
- 统一沿用现有访问规则，能避免“列表能看，详情被拒”之类的隐式行为不一致。

**Alternatives considered:**
- explorer 面板默认拥有 project 内全部 memory 读权限：实现更简单，但会破坏现有 role-scope 语义。
- 完全忽略 role-scoped memory：会让 explorer 数据不完整，也浪费已有 episodic memory 结构。

### 5. 把 DTO 扩展控制在 explorer 真实需要范围内，不预支 tags/edit 未来模型

**Decision:** 仅为当前 explorer contract 补充必要字段，例如 `updatedAt`、`lastAccessedAt`、structured metadata summary / related context hints（若可从 metadata 推导），不在这次 change 中引入 tags、editable annotations 或独立 related-link 表。

**Rationale:**
- 当前 `AgentMemoryDTO` 过于瘦，无法支撑详情、排序和统计展示；补充少量时间/metadata 字段是必要的。
- `memory-explorer-panel` 广义需求里提到 tags，但现有 `AgentMemory` schema 不包含 tags；硬塞进去会让本次 change 膨胀成 schema 改造。
- 通过 metadata 推导 related task/session hints 可以覆盖大部分 explorer 场景，同时不破坏现有存储模型。

**Alternatives considered:**
- 一次性加 tags 与 full metadata schema：会大幅扩大范围，并需要前后端联合迁移。
- 维持当前 DTO 不变：前端只能继续解析 raw string metadata，无法稳定渲染 explorer 详情。

## Risks / Trade-offs

- **[Explorer API 同时覆盖通用 memory 与 episodic memory]** → 通过 façade/handler 分层和清晰的 endpoint 命名，避免 service 责任继续混杂。
- **[保留 `q` 与新增 `query` 的兼容期会增加分支]** → 仅在 handler 参数解析层兼容，service/repository 内部统一用一个标准 query 对象。
- **[统计基于运行时聚合，数据量大时可能偏慢]** → 首版限制为 project-scoped explorer queries + 合理 `limit`，如后续性能证据不足再升级聚合策略。
- **[metadata 结构不统一导致 related context 提示不稳定]** → 将 related context 定义为 best-effort 字段；无法推导时返回空而不是猜测。
- **[活跃 `enhance-frontend-panel` 与本 change 都触及 memory explorer]** → 本 change 只定义 backend contract 和 consumer 对齐，不重做 panel 视觉/UI；前端 change 后续直接消费该 contract。

## Migration Plan

1. 先扩展 memory request/response model 与 handler 参数解析，兼容现有 `/memory` 列表接口并修正 `query`/`q` 漂移。
2. 在不破坏现有 `MemoryService` 职责的前提下，引入 explorer-specific query/service composition，把 `EpisodicMemoryService` 接到详情、history、export、cleanup 路径。
3. 增加 stats / bulk-delete / cleanup / export 路由，并补充相应 handler/service/repository tests。
4. 最后更新 `lib/stores/memory-store.ts` 到新 contract，确保当前 memory panel 和后续 `memory-explorer-panel` 可以从真实 API 获取数据。

**Rollback strategy:**
- 该 change 预期不依赖新外部基础设施；若需要回滚，可回退新增 explorer 子路由与 DTO 扩展，同时保留现有 `store/search/delete` 旧行为。
- 若实现过程中引入 schema 调整，则必须提供向后兼容读路径或显式回滚脚本；当前设计默认避免这一步。

## Open Questions

1. stats 接口中的“storage size”是返回近似逻辑字节数，还是需要数据库真实占用值？首版建议返回近似值。
2. export 是否需要立即支持 CSV？当前设计建议先保证 JSON，待前端明确需要再扩。
3. bulk-delete 是只支持显式 ID 列表，还是也支持“按当前过滤条件删除”？首版建议两者都支持，但条件删除必须要求显式确认字段。
4. related context hint 是否需要直接返回任务/会话 DTO，还是只返回轻量 ID/type 摘要？当前设计偏向轻量摘要，避免 memory API 横向膨胀。
