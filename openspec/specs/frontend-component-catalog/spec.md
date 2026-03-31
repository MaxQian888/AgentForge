# frontend-component-catalog Specification

## Purpose
Define the frontend component and state-management documentation contract for AgentForge so contributors can discover the maintained workspace components and Zustand store patterns before adding duplicate UI or state seams.
## Requirements
### Requirement: 前端关键组件使用文档
系统 SHALL 在 `docs/guides/frontend-components.md` 中提供前端关键组件的使用文档，按功能分类列出核心组件：Layout（Sidebar、Header、Breadcrumb）、表单（TaskForm、RoleForm）、数据展示（TaskBoard、ReviewDetail）、反馈（Toast、Dialog）。每个组件 SHALL 包含：props 列表、使用示例、设计约束。

#### Scenario: 开发者使用 TaskBoard 组件
- **WHEN** 开发者需要在页面中添加看板视图
- **THEN** 文档提供 TaskBoard 组件的 props 说明、数据格式要求和示例代码

#### Scenario: 开发者查找可用的 UI 组件
- **WHEN** 开发者需要选择合适的反馈组件
- **THEN** 文档列出 Toast、Dialog、Alert 等组件的适用场景和选择指南

### Requirement: 状态管理文档
系统 SHALL 在 `docs/guides/state-management.md` 中提供 Zustand 状态管理文档，包含：Store 组织结构、slice 模式、持久化策略、与 Tauri IPC 的集成模式。

#### Scenario: 创建新的 Zustand store
- **WHEN** 开发者需要管理新的页面状态
- **THEN** 文档提供 store 创建模板和与现有 slice 集成的最佳实践

#### Scenario: 持久化状态到 Tauri
- **WHEN** 桌面应用需要持久化用户偏好
- **THEN** 文档说明 Zustand persist middleware 与 Tauri store 的集成方式
