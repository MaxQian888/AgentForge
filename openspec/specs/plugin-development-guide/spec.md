# plugin-development-guide Specification

## Purpose
Define the plugin authoring documentation contract for AgentForge so contributors can build, test, debug, and reason about both Go-hosted WASM plugins and bridge-hosted MCP tool or review plugins using the maintained repo-local workflow.
## Requirements
### Requirement: 插件开发入门文档
系统 SHALL 在 `docs/guides/plugin-development.md` 中提供插件开发完整教程，包含：概念入门（什么是 Integration Plugin / Tool Plugin / Review Plugin / Workflow Plugin）、环境准备、Hello World 示例、调试技巧、发布流程。

#### Scenario: 开发者创建第一个 WASM 插件
- **WHEN** Go 开发者按教程步骤操作
- **THEN** 能在 30 分钟内完成一个最小化的 WASM Integration Plugin，并通过本地测试

#### Scenario: 开发者创建第一个 MCP Tool Plugin
- **WHEN** TypeScript 开发者按教程步骤操作
- **THEN** 能在 30 分钟内完成一个最小化的 MCP Tool Plugin，并在 Bridge 中注册和调用

### Requirement: WASM 插件开发路径文档
系统 SHALL 在 `docs/guides/plugin-wasm.md` 中提供 WASM Integration Plugin 的详细开发指南，包含：TinyGo/Wasm 构建工具链、插件接口规范、宿主函数调用、生命周期钩子、配置 schema 定义、错误处理、测试策略。

#### Scenario: 实现带配置的 WASM 插件
- **WHEN** 开发者需要创建一个接受用户配置的 WASM 插件
- **THEN** 文档展示配置 schema 定义方式和宿主如何注入配置

#### Scenario: 调试 WASM 插件
- **WHEN** 插件运行时出现错误
- **THEN** 文档提供调试方法：日志输出、宿主侧错误追踪、单元测试编写

### Requirement: MCP 插件开发路径文档
系统 SHALL 在 `docs/guides/plugin-mcp.md` 中提供 MCP Tool/Review Plugin 的详细开发指南，包含：MCP 协议基础、Tool 声明 schema、Review 插件接口、Bridge 注册流程、类型安全、测试方法。

#### Scenario: 创建带类型安全的 MCP Tool
- **WHEN** 开发者需要创建一个接受结构化输入的 MCP Tool
- **THEN** 文档展示 Zod schema 定义方式、类型推导和 Bridge 中的注册流程

#### Scenario: 创建 Review Plugin
- **WHEN** 开发者需要自定义代码审查规则
- **THEN** 文档展示 Review Plugin 接口实现方式、审查结果格式和与 Review Pipeline 的集成
