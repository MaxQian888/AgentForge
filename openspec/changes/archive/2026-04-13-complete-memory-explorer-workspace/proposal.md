## Why

`app/(dashboard)/memory/page.tsx` 现在只是挂载一个最小 `MemoryPanel`：它只支持搜索框、角色筛选、分类 tabs 和单条删除，仍停留在“扁平列表 + 截断摘要”的阶段。与此同时，`complete-memory-explorer-backend-surface` 已经把 memory explorer 的 detail / stats / export / bulk-delete / cleanup API 补到了真实后端契约上，当前限制已经从后端能力转移到前端工作区本身。

现在需要把 `/memory` 提升为真正可操作的 memory explorer workspace，让项目记忆能够被筛选、审阅、导出和清理，而不是继续把已经存在的后端能力闲置在最小 UI 后面。

## What Changes

- 将 `/memory` 从单列卡片列表升级为完整工作区，补齐 summary cards、可组合过滤器（query / scope / category / role / date window / result window）以及清晰的 loading / empty / error / degraded 状态。
- 增加 memory detail surface，展示完整内容、结构化 metadata、related context、访问/更新时间，并提供 copy / export 等针对单条记忆的操作入口。
- 增加 operator 管理流，包括多选批量删除、按条件清理旧 episodic memory、基于当前过滤条件的 JSON 导出，以及对应的确认与结果反馈。
- 扩展 `lib/stores/memory-store.ts`，让前端统一消费现有 memory explorer API 的 list / detail / stats / export / bulk-delete / cleanup，而不是把数据拼装逻辑散落在组件里。
- 为 memory workspace 补齐针对组件交互、store contract 和关键空态/管理流的前端测试与多语言文案。

## Capabilities

### New Capabilities
- `memory-explorer-workspace`: 定义前端 memory explorer 工作区的列表、过滤、详情、导出和清理体验，并要求这些操作直接消费现有 memory explorer API。

### Modified Capabilities

## Impact

- Frontend routes/components: `app/(dashboard)/memory/page.tsx`, `components/memory/*`
- Frontend state/contracts: `lib/stores/memory-store.ts` 以及围绕 selection / detail / export / cleanup 的 memory workspace helpers
- Localization/tests: `messages/en/memory.json`, `messages/zh-CN/memory.json`, `components/memory/*.test.tsx`, `lib/stores/memory-store.test.ts`
- API consumption: 复用现有 `/api/v1/projects/:pid/memory`, `/memory/:mid`, `/memory/stats`, `/memory/export`, `/memory/bulk-delete`, `/memory/cleanup`
- Dependencies: 优先复用现有 shadcn/ui 组件，不新增重型数据网格或状态库
