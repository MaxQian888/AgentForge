## Why

AgentForge 已经有较完整的插件调研与目标架构，但仓库里还没有一份可落地的 OpenSpec 变更，把“先做哪一层、做到什么边界”收敛成实现契约。现在需要先把插件运行时和注册中心的最小闭环定下来，避免后续角色插件、工具插件、审查插件和集成插件各自演进、协议分叉。

## What Changes

- 定义 AgentForge 第一阶段插件系统的实现边界，只覆盖插件运行时和插件注册中心，不同时展开插件市场、可视化编排或完整 SDK。
- 为 Go Orchestrator 和 TS Agent Bridge 约定统一的插件元数据、生命周期状态、激活方式和能力暴露模型。
- 规定首批支持的插件类型与运行时映射，明确哪些能力走 Go 侧子进程运行时，哪些能力走 TS/MCP 运行时，以及跨边界调用方式。
- 定义插件注册中心的核心能力，包括插件清单、版本、安装来源、启停状态、权限声明和健康状态。
- 约束后续实现应优先支持本地安装与内置插件发现，为未来引入签名校验、远程分发和市场审核预留扩展点。

## Capabilities

### New Capabilities
- `plugin-runtime`: 定义 AgentForge 如何发现、加载、激活、停用并监控不同类型的插件运行时，包括 Go 侧与 TS 侧的职责边界。
- `plugin-registry`: 定义 AgentForge 如何存储、查询和管理插件元数据、安装状态、版本与权限声明，作为插件系统的统一真相源。

### Modified Capabilities

## Impact

- 受影响文档与设计基线包括 [docs/part/PLUGIN_SYSTEM_DESIGN.md](d:\Project\AgentForge\docs\part\PLUGIN_SYSTEM_DESIGN.md)、[docs/part\PLUGIN_RESEARCH_TECH.md](d:\Project\AgentForge\docs\part\PLUGIN_RESEARCH_TECH.md)、[docs/part\PLUGIN_RESEARCH_PLATFORMS.md](d:\Project\AgentForge\docs\part\PLUGIN_RESEARCH_PLATFORMS.md)。
- 后续实现预计会触达 `src-go/internal` 下的插件管理、配置和状态持久化模块，以及 `src-bridge` 中与 MCP/工具发现相关的桥接入口。
- 会影响插件 manifest、运行时进程管理、权限声明、安装来源和健康检查等跨模块契约。
- 该变更为后续角色插件、工具插件、审查插件和集成插件提供统一底座，但不直接要求这些高层能力在本次变更内全部实现。
