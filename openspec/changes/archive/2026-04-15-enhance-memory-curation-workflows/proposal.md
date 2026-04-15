## Why

AgentForge 现有的 memory explorer backend 和 workspace 已经支持检索、详情、导出与清理，但记忆条目仍缺少真正的整理与维护能力。当前大而泛的 `enhance-frontend-panel` 还挂着 `memory tagging` 需求，角色编辑器和 IM `/memory note` 也已经暴露出更丰富的记忆使用场景，因此现在需要把剩余的记忆整理 seam 抽成一个 focused change，补齐标签、人工记录和受控编辑，而不是继续把它埋在宽泛前端大变更里。

## What Changes

- 为项目记忆增加 operator-facing curation contract：支持人工记录项目记忆、为记忆添加/移除标签、按标签过滤，并区分可维护的人工记忆与只读的系统生成记忆。
- 扩展 memory explorer API 与存储模型，使单条记忆可以暴露稳定的标签字段、受控更新语义和可编辑状态，而不是要求前端把标签塞进原始 metadata 字符串。
- 升级 `/memory` 工作区，补齐记忆 note 创建、标签 chip 管理、标签筛选、单条导出，以及仅对允许的记忆类型开放的编辑入口与反馈状态。
- 统一前后端对“可编辑记忆”的边界：人工记录的 operator note 可修改内容与标签，系统自动生成的 team learning、task decomposition、document chunk 等记忆保持只读，避免破坏可追溯性。
- 保持范围聚焦在 operator curation，不在本 change 中扩展 semantic vector search、procedural learning automation、长期压缩归档或跨项目知识图谱。

## Capabilities

### New Capabilities

### Modified Capabilities
- `memory-system`: 将当前 memory contract 从只读 explorer + bounded cleanup 扩展为支持标签化记忆、人工 operator note、以及受控的可编辑记忆边界。
- `memory-explorer-api`: 扩展记忆 API 以支持标签字段、标签过滤、人工 note 创建、单条导出，以及受控的记忆更新/标签维护语义。
- `memory-explorer-workspace`: 扩展 `/memory` 工作区，使其支持 note authoring、标签管理、标签筛选、单条导出与只在允许范围内出现的编辑流。

## Impact

- Backend seams: `src-go/internal/model/agent_memory.go`, `src-go/internal/repository/agent_memory_repo.go`, `src-go/internal/service/memory_service.go`, `src-go/internal/service/memory_explorer_service.go`, `src-go/internal/service/memory_api_service.go`, `src-go/internal/handler/memory_handler.go`, `src-go/internal/server/routes.go`, 以及相关 migration / tests。
- Frontend seams: `app/(dashboard)/memory/page.tsx`, `components/memory/*`, `lib/stores/memory-store.ts`, 相关组件测试与 store contract 测试。
- API impact: 现有 `/api/v1/projects/:pid/memory*` contract 需要增加标签与受控编辑/导出语义，并保持现有 Bearer token、项目隔离和错误响应约定。
- Product alignment: 从广泛的 `enhance-frontend-panel` 中抽离仍未闭环的 memory curation seam，让记忆功能按 focused change 继续推进，而不是继续堆进超大 change。
