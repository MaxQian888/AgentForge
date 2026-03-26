## Why

AgentForge 已经具备结构化角色工作区，但当前 `components/roles/role-workspace.tsx` 仍是长表单 + 侧边摘要的初级形态，信息架构拥挤、继承与预览链路不够清晰，在窄屏或中等宽度下也难以保持稳定可读。`docs/role-authoring-guide.md`、`docs/PRD.md` 和 `docs/part/PLUGIN_SYSTEM_DESIGN.md` 已经把“模板起步、继承可见、预览/沙盒闭环、文档化说明”的 authoring 体验定义得更完整，现在需要把现有编辑器补到这个产品真相。

## What Changes

- 重构角色编辑器的信息架构，把角色库、分区编辑、预览/说明上下文组织成更清晰的 authoring workspace，而不是继续依赖单条长滚动表单。
- 为角色编辑器补充响应式布局要求，确保桌面端、中等宽度和窄屏下都能保持可导航、可编辑、可预览的稳定体验，而不是简单堆叠导致上下文丢失。
- 强化模板起步、继承来源、保存前预览与沙盒入口的可见性，让操作者能按 `docs/role-authoring-guide.md` 推荐流程完成角色定制。
- 将字段说明、章节引导、YAML 预览和执行摘要对齐到现有角色文档和 PRD 术语，减少“字段存在但不知道何时填、为什么填”的作者负担。
- 补充角色工作区的前端测试与必要的文档更新，覆盖布局切换、关键 rail 展示和文档一致性。

## Capabilities

### New Capabilities

### Modified Capabilities
- `role-management-panel`: 角色管理工作区的要求从“提供结构化表单”提升为“提供文档对齐、流程清晰、可响应式工作的完整 authoring layout”。

## Impact

- Affected frontend surfaces: `app/(dashboard)/roles/page.tsx`, `components/roles/*`, `lib/roles/role-management.ts`, and related role workspace tests
- Affected operator guidance: `docs/role-authoring-guide.md` and any role-authoring wording mirrored in product copy
- Affected verification surface: focused role workspace rendering, responsive behavior, preview/sandbox entry visibility, and catalog-to-editor interaction tests
