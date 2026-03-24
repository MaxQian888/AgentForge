## Why

AgentForge 的前端已经有 `Plugins`、`Roles`、`Workflow` 三个页面入口，但它们目前仍是最小可用壳层，和 `docs/PRD.md`、`docs/part/PLUGIN_SYSTEM_DESIGN.md` 中明确要求的“角色配置面板、工作流可视化、插件市场 UI”存在明显落差。现在补齐这组管理面板，可以把已经落地的插件注册、角色 YAML、工作流配置和运行时状态真正暴露为可操作的产品能力，而不是只停留在后端契约或基础 CRUD。

## What Changes

- 完成插件管理面板，补上已安装插件、内建插件、市场插件三类入口，以及按类型、生命周期、运行时宿主、来源的筛选与详情查看。
- 为插件卡片和插件详情补上运行时与运维信息展示，包括权限声明、runtime host、last health、restart count、source path、ABI/runtime metadata、错误状态和动作可用性说明。
- 将插件市场面板接入现有 `/api/v1/plugins/marketplace` 能力，用于浏览可用插件条目、区分本地安装与市场来源，并给出真实可执行或不可执行的安装状态提示。
- 扩展角色配置面板为结构化编辑器，覆盖基础身份信息、继承关系、allowed tools、permission mode、knowledge、路径约束、预算与审查开关等 PRD/Role YAML 关键字段。
- 在角色面板中加入模板/预设入口和角色摘要信息，让用户能从现有角色快速派生，而不是只能从空白表单硬填。
- 为工作流页面补上只读可视化视图，展示状态流转图、自动化触发规则和最近触发活动；同时保留当前配置编辑能力。
- 将工作流可视化接入现有配置接口和 `workflow.trigger_fired` 实时事件，提供最近触发记录、空状态、加载失败和实时降级提示。

## Capabilities

### New Capabilities
- `plugin-management-panel`: 提供面向操作员的插件管理前端，覆盖已安装/内建/市场插件浏览、筛选、详情、权限与运行时状态展示、以及与现有启用/禁用/激活/重启动作的安全联动。
- `role-management-panel`: 提供结构化的角色配置前端，覆盖模板选择、继承关系、身份与知识字段、工具与权限约束、预算配置和角色摘要。
- `workflow-visualization-panel`: 提供项目级工作流配置与只读可视化前端，覆盖状态流转图、触发规则列表、最近自动化活动和实时状态提示。

### Modified Capabilities
- None.

## Impact

- Affected frontend pages: `app/(dashboard)/plugins/page.tsx`, `app/(dashboard)/roles/page.tsx`, `app/(dashboard)/workflow/page.tsx`
- Affected UI components: `components/plugins/*`, `components/roles/*`, `components/workflow/*`
- Affected client stores: `lib/stores/plugin-store.ts`, `lib/stores/role-store.ts`, `lib/stores/workflow-store.ts`, `lib/stores/ws-store.ts`
- Affected APIs and realtime contracts: `/api/v1/plugins`, `/api/v1/plugins/discover`, `/api/v1/plugins/marketplace`, `/api/v1/roles`, `/api/v1/projects/:projectId/workflow`, `workflow.trigger_fired`
- No intended change to remote registry, plugin signing/review pipeline, or long-term WASM/marketplace distribution architecture
