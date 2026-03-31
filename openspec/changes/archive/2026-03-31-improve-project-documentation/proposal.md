## Why

AgentForge 已有 100+ spec 目录和 20+ 设计文档，但面向开发者/使用者的「入口级文档」存在明显缺口：没有 API 参考文档、没有数据库 Schema 说明、没有生产部署指南、没有安全模型文档、缺少架构决策记录（ADR）。随着功能模块持续增加，文档与实际代码之间的差距正在扩大，新贡献者上手成本高，功能覆盖不完整。

## What Changes

- 新增 **API Reference** 文档：覆盖 Go 后端所有 REST 端点（/api/v1/...），包含请求/响应 schema、认证要求、错误码
- 新增 **Database Schema** 文档：ER 图 + 表/字段说明，覆盖 PostgreSQL 和 Redis 的核心数据结构
- 新增 **Deployment Guide** 文档：生产环境部署流程，覆盖 Docker、Tauri 打包、环境变量、TLS 配置
- 新增 **Security Model** 文档：认证/授权流程、JWT 策略、token 黑名单、CORS、Tauri 权限模型
- 新增 **Architecture Decision Records (ADR)** 模板与首批记录：记录关键技术选型决策
- 改进 **README.md / README_zh.md**：补充 Quick Start 5 分钟教程、架构总览图、功能特性矩阵
- 改进 **CONTRIBUTING.md**：增加环境变量速查表、常见问题排查、本地调试技巧
- 改进 **TESTING.md**：补充 Bridge 测试矩阵、IM Bridge 端到端测试流程、覆盖率提升指南
- 新增 **Component Catalog** 文档：面向前端的组件使用指南，覆盖关键 UI 组件及其 props/用法
- 新增 **Plugin Development Guide**：从零开发一个插件的完整教程（WASM + MCP 两条路径）

## Capabilities

### New Capabilities

- `api-reference-docs`: Go 后端 REST API 完整参考文档，自动从代码注解生成骨架并人工补充
- `database-schema-docs`: PostgreSQL 表结构与 Redis 缓存策略文档，含 ER 图和字段说明
- `deployment-guide`: 生产环境部署指南，覆盖 Docker Compose、Tauri 打包、环境配置、TLS
- `security-model-docs`: 安全模型文档，覆盖认证授权、JWT/Redis token 管理、CORS、Tauri 权限
- `adr-records`: 架构决策记录体系，含 ADR 模板和首批 5 条关键决策
- `plugin-development-guide`: 插件开发完整教程，WASM Integration 与 MCP Tool 两条路径
- `frontend-component-catalog`: 前端关键组件使用文档，含 props、示例和最佳实践

### Modified Capabilities

- `local-development-workflow`: 扩展现有本地开发文档，增加 Quick Start 5 分钟教程和环境变量速查
- `im-bridge-control-plane`: 补充 IM Bridge 端到端测试流程到 TESTING.md
- `bridge-health-probe`: 补充 Bridge 测试矩阵到 TESTING.md

## Impact

- **文档文件**: 新增 ~8 个文档文件到 docs/ 目录，更新 README.md、CONTRIBUTING.md、TESTING.md
- **代码影响**: 仅文档变更，无代码修改；可能需要为 API 文档添加代码注解
- **CI/CD**: 可能新增文档 lint/sync 检查 workflow
- **依赖**: 无新依赖
- **维护成本**: 文档需要与代码同步更新，建议在 PR checklist 中加入文档更新检查项
