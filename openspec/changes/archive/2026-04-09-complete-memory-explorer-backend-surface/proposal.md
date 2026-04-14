## Why

`enhance-frontend-panel` 已经把 `memory-explorer-panel` 作为活跃需求面，但当前 Go 后端对 memory explorer 仍只有最小 `store/search/delete` 表面：`lib/stores/memory-store.ts` 已经发送 `query/scope/category`，而 `src-go/internal/handler/memory_handler.go` 只读取 `q/limit`，导致当前搜索与筛选契约已经漂移。与此同时，Go 侧 `EpisodicMemoryService` 已经具备 date-range history、retention、export/import 等能力，却没有被 `src-go/internal/server/routes.go` 暴露为可供 explorer 使用的真实 API。

现在需要把 memory explorer 的 backend surface 单独补齐，否则前端只能继续建立在不对齐的查询参数、缺失的过滤能力和不可用的管理能力之上，后续 `memory-explorer-panel` 会被迫依赖 mock 或重复实现。

## What Changes

- 对齐并扩展 memory explorer 的查询契约：把现有项目 memory 查询从隐式 `q` 搜索扩展为明确支持 `query`、`scope`、`category`、`roleId`、date-range、pagination/limit 的 explorer-ready API，而不是静默忽略前端参数。
- 为 memory explorer 增加 Go 侧管理接口：支持单条详情、筛选列表、批量删除/按条件清理、导出 episodic memory，以及统计摘要等后端能力。
- 复用并接线现有 `EpisodicMemoryService` / repository helpers，把已存在的 history、retention、export 逻辑提升为受控 API，而不是继续把能力埋在未接线的 service 中。
- 统一 memory explorer 返回 DTO，使其能携带结构化 metadata、role/scope 信息和 explorer 所需的相关上下文摘要，供前端面板直接消费。
- 补齐 Go handler/service/repository 测试以及 consumer-facing contract 验证，确保 memory explorer backend 在 `src-go` 与前端 store 之间保持真实一致。

## Capabilities

### New Capabilities
- `memory-explorer-api`: 定义 memory explorer 的 Go 后端查询、详情、统计、导出与清理接口，以及这些接口的筛选/分页/错误语义。

### Modified Capabilities
- `memory-system`: 将当前 memory-system 从简单的 project memory store/search/delete 扩展为支持 explorer-ready 查询过滤、episodic history/retention/export，以及更完整的 API-facing DTO 语义。

## Impact

- Affected backend seams: `src-go/internal/handler/memory_handler.go`, `src-go/internal/service/memory_service.go`, `src-go/internal/service/episodic_memory_service.go`, `src-go/internal/repository/agent_memory_repo.go`, `src-go/internal/server/routes.go`, 以及相关 model/DTO 与 tests。
- Affected consumer seams: `lib/stores/memory-store.ts`, `components/memory/memory-panel.tsx`, 以及活跃 `enhance-frontend-panel` 里的 `memory-explorer-panel` 前端实现。
- API impact: 新增/扩展 authenticated `/api/v1/projects/:pid/memory*` explorer endpoints 与查询参数契约；需要保持 `message` 型错误响应与现有 Bearer-token 语义。
- Verification impact: 需要以 `src-go` handler/service tests 为主，并补 consumer contract 验证，避免再次出现 `query` vs `q` 这类前后端漂移。
