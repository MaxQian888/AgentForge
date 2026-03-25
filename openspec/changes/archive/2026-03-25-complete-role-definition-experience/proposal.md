## Why

AgentForge 现在已经有基础的 Role YAML、Go 侧 role store 和结构化角色页面，但这些能力只覆盖了 PRD 与 `PLUGIN_SYSTEM_DESIGN.md` 里角色定义蓝图的一小部分。当前角色作者仍然无法在产品内完整配置高级 schema、查看继承后的有效结果、获得字段说明，或在保存前做一次可靠的 dry-run 验证，这让“数字员工可定制”依然停留在半成品状态。

## What Changes

- 扩展角色定义合同，使角色管理与 YAML source of truth 支持 PRD 已承诺但当前未完整闭环的高级字段，包括更完整的 identity、capability packages/tool config、结构化知识与记忆、协作约束、触发器、override 和治理说明。
- 将角色管理页面升级为完整的 authoring workspace，补齐高级分区编辑、字段级说明、继承/override 可见性、原始 YAML 预览、以及面向执行的有效配置摘要，而不是继续依赖用户手写 YAML 或猜测最终结果。
- 新增角色 authoring sandbox / dry-run 能力，让操作者在保存或发布前验证角色 schema、查看解析后的 effective manifest / execution profile，并用受控测试输入检查角色行为是否符合预期。
- 补齐角色定义文档、样例角色和面向操作者的说明内容，确保 PRD、插件系统设计、`docs/role-yaml.md`、样例 YAML 与实际产品能力保持一致。
- 为角色高级字段与 sandbox 流程补齐 focused 测试和验证脚本，避免 role schema、UI draft、Go normalization 与 preview 结果再次漂移。

## Capabilities

### New Capabilities
- `role-authoring-sandbox`: 定义角色 dry-run、effective manifest / execution profile 预览、受控测试输入校验与发布前验证流程。

### Modified Capabilities
- `role-plugin-support`: 角色定义要求从当前的基础 YAML 与 execution-profile 投影扩展到更完整的 PRD-aligned schema、继承/override 解析、知识/协作/触发器字段保留与 preview 支持。
- `role-management-panel`: 角色管理面板从基础结构化编辑扩展为完整 authoring workspace，要求覆盖高级字段分区、字段说明、YAML 预览、有效配置摘要与 sandbox 入口。

## Impact

- Affected Go role surfaces: `src-go/internal/model/role.go`, `src-go/internal/role/*`, `src-go/internal/handler/role_handler.go`, role API contracts, normalization, persistence, and preview/dry-run handlers
- Affected frontend role authoring surfaces: `app/(dashboard)/roles/page.tsx`, `components/roles/*`, `lib/stores/role-store.ts`, `lib/roles/role-management.ts`, and any new preview/sandbox client helpers
- Affected sample assets and docs: `roles/*/role.yaml`, `docs/PRD.md`, `docs/part/PLUGIN_SYSTEM_DESIGN.md`, `docs/part/PLUGIN_RESEARCH_ROLES.md`, `docs/role-yaml.md`, and operator guidance for role authoring
- Affected verification surface: Go role parser/store/handler tests, role workspace tests, sandbox preview tests, and focused end-to-end validation for advanced role authoring flows
