## Why

AgentForge 的插件控制面后端已经具备 catalog、trust、audit、MCP interaction 和 workflow run 等真实能力，但当前 `app/(dashboard)/plugins` 仍只暴露了其中一小部分，导致操作员看到的是一个“能看卡片、不能完整运维”的半成品控制台。现在需要把剩余的 operator-facing 缺口单独收敛成一个 focused change，避免重新打开已经完成归档的角色/工作流大面板变更，同时把插件来源、安装语义和运行诊断恢复成 repo-truthful 的状态。

## What Changes

- 将插件页从基础卡片列表补齐为真实的 operator console，覆盖已安装插件、内建可发现插件、catalog/marketplace 条目以及与当前平台能力一致的安装语义。
- 修正 built-in/catalog discovery 的产品合同，使“发现/浏览”不再隐式等同于“安装/注册”，避免插件来源分区和 installed state 被 discovery 请求污染。
- 为已安装插件补齐 operator-facing 诊断面，展示 trust/approval/release 元数据、审计事件、MCP capability snapshot 与 latest interaction、以及 workflow plugin run 历史。
- 为插件生命周期补齐前端动作入口和状态约束，覆盖 deactivate、update、catalog install 等现有后端合同，而不是继续把这些能力隐藏在 API 后面。
- 为插件安装和配置体验增加更真实的来源选择与风险表达，但不在本次中承诺公开远程 marketplace、自动签名链路或新的插件运行时基础设施。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `plugin-management-panel`: expand the operator console to cover truthful source-channel browsing, install flows, lifecycle actions, trust and release visibility, audit history, MCP diagnostics, and workflow run inspection.
- `plugin-catalog-feed`: change built-in and catalog discovery semantics so discovery remains browse/installable data instead of implicitly creating installed registry records.

## Impact

- Affected frontend: `app/(dashboard)/plugins/page.tsx`, `components/plugins/*`, `lib/stores/plugin-store.ts`, and likely new plugin diagnostics/install helper components.
- Affected backend/control-plane seams: `src-go/internal/service/plugin_service.go`, `src-go/internal/handler/plugin_handler.go`, and existing plugin APIs for catalog install, deactivate, update, events, MCP interaction, and workflow runs.
- Affected operator flows: built-in discovery, catalog install, trusted vs untrusted plugin review, installed plugin diagnostics, MCP troubleshooting, and workflow plugin execution visibility.
- Verification impact: focused plugin page/component tests plus targeted Go service/handler coverage for corrected discovery semantics and the operator-facing plugin control-plane paths.
