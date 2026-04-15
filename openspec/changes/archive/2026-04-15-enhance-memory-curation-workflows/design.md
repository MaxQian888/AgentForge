## Context

AgentForge 在 2026-04-09 和 2026-04-13 已经分别完成了 `memory-explorer-api` 与 `memory-explorer-workspace` 的第一轮闭环，当前 `/memory` 可以做检索、详情、统计、导出与清理。但仍存在三处未闭环的真实断层：

1. 活跃 change `enhance-frontend-panel` 的 `memory-explorer-panel` 仍保留 `memory tagging` 需求与未完成任务，说明产品层面对“整理记忆”的预期还没有落地。
2. `src-im-bridge/client/agentforge.go` 的 `StoreProjectMemoryNote` 仍向 `/api/v1/projects/:pid/memory` 发送 `category: "operator_note"`，但当前 `agent_memory` migration 的 category check 只声明了 `episodic | semantic | procedural`，人工 note 的 canonical shape 没有被正式定义。
3. `/memory` 工作区目前仍偏只读运维面；虽然 `lib/stores/memory-store.ts` 已经保留 `storeMemory` seam，但 UI 没有提供 note authoring，也没有标签筛选、标签维护或受控编辑流。

这次 change 需要在不重开 semantic vector search、procedural learning automation 或长期知识图谱架构的前提下，把“记忆如何被人工整理和维护”这一层 contract 补齐。

## Goals / Non-Goals

**Goals:**
- 为项目记忆定义稳定的 curation shape，包括标签、operator note 类型与可编辑边界。
- 让现有 `/api/v1/projects/:pid/memory*` surface 支持标签筛选、人工 note 创建，以及受控的记忆更新。
- 让 `/memory` 工作区支持人工 note 录入、标签管理、标签筛选、单条导出，并对只读记录显式展示不可编辑状态。
- 保持现有 explorer list/detail/stats/export/cleanup contract 可兼容延续，不破坏已完成的 backend/workspace 功能。
- 修正 IM `/memory note` 与 canonical memory contract 的漂移，避免继续依赖未定义的 category 枚举。

**Non-Goals:**
- 不实现 semantic vector search、GraphRAG、跨项目知识图谱或长期 memory compaction。
- 不把角色配置中的 semantic/procedural 开关一次性接成完整 runtime automation。
- 不引入新的 memory table、tag join table 或单独的 note-only persistence subsystem。
- 不把所有系统生成记忆都变成可编辑内容；这次只定义受控 curation，而不是重写可追溯性模型。
- 不把 `/memory` 重做成新的全局信息架构或独立子路由群。

## Decisions

### 1. 将 operator note 归一化为 `episodic` memory + metadata kind，而不是新增 category 枚举

**Decision:** 人工 note 继续走现有 `/api/v1/projects/:pid/memory` POST surface，但服务端把 operator note 统一归一化为 `category=episodic`，并在 metadata 中记录 `kind=operator_note`、`editable=true`、`tags=[]` 等 curation 字段。现有 `category=operator_note` 输入作为兼容别名保留，由 handler/service 在写入前规范化。

**Rationale:**
- 当前 migration 与主 spec 都没有正式承诺 `operator_note` category；继续扩 category 会把一次 focused curation change 膨胀成 category schema 治理问题。
- `episodic` 本来就代表人工沉淀与跨会话经验记录，operator note 作为其可维护子类型更符合现有 memory-system 边界。
- 接受 legacy `operator_note` 可以直接修复 IM bridge 的现实漂移，而不要求调用方一次性同步升级。

**Alternatives considered:**
- 新增 `operator_note` category 到 schema 与所有分支逻辑：语义直接，但会扩大迁移面，并把类别体系继续碎片化。
- 保持当前客户端自定义 category 不变：会继续让 API / DB / spec 三方不一致。

### 2. 将 tags 持久化在 `metadata.tags`，但在 DTO 中提升为一等字段

**Decision:** 标签不使用独立表；继续复用 `agent_memory.metadata` JSONB 保存 `tags`，同时在 list/detail DTO 中增加显式 `tags`、`kind`、`editable`（或等价命名）字段，避免前端自己解析 metadata 字符串。repository/filter 层新增 tag-aware query，workspace 只消费显式字段。

**Rationale:**
- 当前 `agent_memory` 已经有 JSONB metadata，适合为现有记录渐进式补上 curation 注解，而不需要额外 join 结构。
- explorer UI 需要稳定字段来渲染 chip、筛选和只读提示；直接暴露 DTO 字段可以避免再次出现 contract 漂移。
- 这种方式对现有历史数据兼容最好，没有 tag 的旧记录可以自然退化为 `tags=[]`。

**Alternatives considered:**
- 新建 `agent_memory_tags` 表：查询更正统，但对 focused change 来说过重，且要同时改 repo、migration、cleanup/export shape。
- 让前端直接读写 `metadata.tags`：实现快，但会把 contract 重新隐藏回原始 JSON。

### 3. 使用统一的 curation update path，并用服务端规则限制可编辑范围

**Decision:** 在现有 `/api/v1/projects/:pid/memory/:mid` 资源上增加受控更新语义，支持部分更新 `key`、`content`、`tags`。服务端规则为：
- `tags` 可用于所有当前调用者可见且可管理的记忆；
- `key` / `content` 仅允许更新被标记为 `kind=operator_note` 或 `editable=true` 的人工记忆；
- 系统生成记忆继续允许删除、导出、详情查看，但内容编辑返回明确错误。

**Rationale:**
- list/detail/delete 已经围绕同一资源路径建立，继续在同一路径补 PATCH/更新语义最稳定。
- 把边界收在 service 层，可以同时保护 UI、IM bridge 与未来其他调用方，不让“只读系统记忆”约束只靠前端文案维护。
- 标签与内容编辑共享 selection / confirmation / refresh 语义，统一 update path 更利于 store 收敛。

**Alternatives considered:**
- 分拆 `/tags` 与 `/notes` 子端点：语义更细，但会把当前 memory API 分成多条近似 surface。
- 允许任意记忆都修改内容：实现简单，但会破坏自动生成记忆的追溯性。

### 4. `/memory` 工作区继续以 store 为单一 truth，新增 composer + curation affordances，而不是新开独立 authoring 页

**Decision:** 继续让 `lib/stores/memory-store.ts` 持有 filters、entries、detail、selection 和 mutation state，并在现有 workspace shell 内补上 operator note composer、tag chips、tag filter、edit dialog/inline panel、single-entry export。未选项目 gate 和现有 stats/list/detail 骨架保持不变。

**Rationale:**
- 当前 memory workspace 已经围绕单项目上下文稳定运行，cure flows 属于同一工作区内的延展。
- 维持同一个 store 能保证 note 创建、标签更新、删除/清理后都走同一套 refresh semantics。
- 若再开 authoring 页面或 modal-only 路径，会把已经完成的 explorer workspace 再拆散。

**Alternatives considered:**
- 新开 `/memory/new` 或 `/memory/[id]/edit`：URL 更显式，但不符合当前 operator workspace 风格。
- 直接在 `MemoryPanel` 大组件里堆全部逻辑：短期快，长期会让现有组件再次失焦。

## Risks / Trade-offs

- [Legacy category alias 继续存在一段时间] → 在 spec 中把 `operator_note` 定义为兼容输入而非 canonical 存储值，并在 tasks 中包含 bridge/client 与 API 文档同步。
- [metadata-based tags 查询性能受限] → 当前先以单项目 explorer 范围为主；如后续数据量证明不足，再单独补 tag index/change，而不是提前做重迁移。
- [可编辑边界不清导致用户误改系统记录] → DTO 明确返回 editability，workspace 在按钮层和错误反馈层都显式区分只读/可编辑。
- [旧记录缺少 kind/tags 字段] → service 统一做默认值归一化，确保旧数据在 UI 中退化为无标签、只读或按既有规则可删除。
- [范围再次膨胀到 semantic/procedural runtime] → design 和 specs 明确把这次锁在 operator curation，不顺手承诺 role runtime automation。

## Migration Plan

1. 先补后端 canonical curation shape：请求/DTO、service 归一化、repository tag filter/update helpers，以及 legacy `operator_note` alias 兼容。
2. 再更新 `/memory` store 与 workspace，把 note authoring、tag filter、tag mutations、受控编辑和单条导出接到现有 API。
3. 同步 IM `/memory note`、OpenAPI / spec 文档与 targeted tests，确保兼容输入和 canonical 输出一致。
4. 若上线后需要回滚，优先回退 workspace curation affordances 与 update routes；保留已存在的 list/detail/stats/export/cleanup surface，不做数据销毁型回滚。

## Open Questions

- 标签筛选首版是否只支持单标签精确过滤，还是要同时支持多标签交集；当前建议先做单标签，减少 query shape 复杂度。
- 单条导出是否需要单独后端 endpoint；当前建议优先复用 detail payload 由前端本地导出，若后续需要审计式下载再补专门 API。
- 是否需要为 tags 引入额外审计字段（如 `taggedBy` / `taggedAt`）；当前 proposal 不要求，但如果实现中发现可追溯性不足，可能要在后续 focused change 补充。
