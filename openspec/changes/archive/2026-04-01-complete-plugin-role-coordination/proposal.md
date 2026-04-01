## Why

AgentForge 已经分别具备插件控制面、角色工作区、workflow role 校验和 role execution profile 投影，但这几条链路仍然各自为政。当前 workflow 插件只在后端做最小 role existence 校验，role 侧声明的 tool plugin / MCP 依赖也缺少 authoritative 的可用性判断，前端两边都看不到反向影响，导致“能保存、能安装、到真正执行才暴露断链”的问题持续存在。

## What Changes

- 为现有 plugin 和 role 链路补一个统一的 dependency truth：同一套后端规则同时评估 workflow 对 role 的引用、role 对 tool plugin / MCP server 的引用，以及这些引用当前是否 resolved、stale、blocking 或 warning。
- 扩展 plugin 管理面，让 workflow/plugin 详情可以显示关联 role、引用健康状态、缺失原因和跳转入口，而不是只把 role id 当静态字符串展示。
- 扩展 roles 工作区和角色库，让操作者在 authoring、preview/sandbox、保存和删除前看到 plugin/tool 依赖健康、下游 workflow/plugin 消费者，以及本次改动会影响哪些已安装能力。
- 对破坏性 role 变更增加影响保护：当 role 仍被已安装 workflow/plugin 能力消费时，系统必须返回明确阻断或 warning 语义，而不是允许静默删除后把失败推迟到运行时。

## Capabilities

### New Capabilities

### Modified Capabilities
- `plugin-management-panel`: 插件详情和管理动作需要展示 role 引用、dependency health、反向 consumer 信息，以及跳转到 roles 工作区的路径。
- `role-management-panel`: 角色库、工作区、context rail 和 destructive actions 需要暴露 plugin/tool 依赖健康与下游 workflow/plugin 影响。
- `workflow-plugin-runtime`: workflow 插件的 role 引用需要在详情、启用和执行链路里持续校验并暴露 drift，而不是只在首次注册时做一次存在性检查。
- `role-plugin-support`: role preview/sandbox/execution 与 role API 需要把 tool plugin / MCP 依赖健康和下游 consumer 影响纳入 authoritative 诊断与错误语义。

## Impact

- Affected backend seams: `src-go/internal/service/plugin_service.go`, `src-go/internal/service/workflow_execution_service.go`, `src-go/internal/handler/role_handler.go`, `src-go/internal/role/execution_profile.go`, `src-go/internal/role/tool_capabilities.go`, 以及 role/plugin 相关 model 与 tests。
- Affected frontend seams: `components/plugins/*`, `components/roles/*`, `lib/stores/plugin-store.ts`, `lib/stores/role-store.ts`, `lib/roles/role-management.ts`, 以及相关 dashboard route tests。
- Affected runtime contracts: role execution profile 对 role-scoped plugin IDs 的解释、workflow role reference diagnostics、plugin/role destructive action error semantics。
- Affected operator workflows: plugin detail inspection、role authoring review、preview/sandbox readiness、workflow/plugin enablement、role delete/update impact handling。
