## Why

PRD.md 和 PLUGIN_SYSTEM_DESIGN.md 已经把 AgentForge 描述为提供一组可直接使用的官方内置插件，包括更多 ToolPlugin、至少 1-2 个内置 ReviewPlugin，以及可运行的 Workflow starter；但当前仓库里真正能被 built-in discovery 发现的只有 plugins/tools/web-search/manifest.yaml 和 plugins/integrations/feishu-adapter/manifest.yaml。这导致插件目录、管理面板和项目文档之间出现明显漂移，也让 内置插件支持更完整仍停留在文档样例和测试 fixture，而不是可实施的产品契约。

## What Changes

- 为 AgentForge 定义一组文档对齐的官方内置插件包，明确哪些 built-in ToolPlugin、ReviewPlugin、WorkflowPlugin 需要随仓库提供真实 manifest、入口资产和验证路径。
- 补齐当前缺失的内置 ReviewPlugin 与 Workflow starter 支持，让它们不再只存在于 PRD.md、PLUGIN_SYSTEM_DESIGN.md 或 parser/test 示例里，而是进入真实的 built-in discovery、catalog install 和 lifecycle 流程。
- 扩展现有插件目录和发现契约，要求 catalog 只暴露真正由仓库维护、可解析、可验证的官方内置插件，并对缺少依赖或暂不可执行的 built-in 条目给出 truthful 状态而不是文档占位。
- 扩展仓库级 plugin build/debug/verify 工作流，使新增的官方 built-in ToolPlugin、ReviewPlugin、WorkflowPlugin 都有可复用的 repo-owned 验证路径，避免 built-in 样例再次漂移成不可运行资产。
- 保持远程 registry/marketplace、外部来源安装、额外 IM 平台扩张和新的插件宿主模型为非目标；本次聚焦官方 built-in 插件包的完整性和文档一致性。

## Capabilities

### New Capabilities
- built-in-plugin-bundle: define the official repository-owned built-in plugin inventory, required asset layout, discoverability, and docs-alignment rules for shipped plugins.

### Modified Capabilities
- plugin-catalog-feed: require the catalog and built-in discovery responses to surface the official built-in plugin bundle from real repo assets only, with truthful availability and installability metadata.
- review-plugin-support: extend review-plugin requirements so official built-in review plugins are shipped as real manifest-backed plugins and participate in the same discovery, selection, and provenance model as custom review plugins.
- workflow-plugin-runtime: require at least one maintained built-in sequential workflow starter to be discoverable and executable through the same workflow runtime contract used by custom WorkflowPlugin definitions.
- plugin-development-scripts: extend repository-owned plugin build/debug/verify workflows beyond the current Go-hosted sample so maintained built-in tool, review, and workflow plugins have repeatable validation paths.

## Impact

- Affected repo-owned plugin assets: plugins/tools/*, new plugins/reviews/*, new plugins/workflows/*, and any related starter entrypoints, manifest files, or packaged runtime assets.
- Affected control-plane seams: src-go/internal/service/plugin_service.go, plugin handler/catalog flows, workflow validation paths, and tests that currently assume only tool plus integration built-ins.
- Affected bridge or plugin authoring seams: src-bridge/src/plugin-sdk/*, repo-local plugin starter scaffolds, and script contracts used to build or verify maintained MCP review or tool plugins.
- Affected product contracts: built-in plugin discovery, plugin catalog truthfulness, installability gating, workflow starter visibility, review-plugin provenance, and official-plugin docs alignment.
- Affected docs and verification: docs/PRD.md, docs/part/PLUGIN_SYSTEM_DESIGN.md, built-in plugin examples, and focused plugin verification commands that must stay aligned with shipped built-in assets.
