## 1. 文档目录结构与模板

- [x] 1.1 创建 `docs/api/`、`docs/schema/`、`docs/deployment/`、`docs/security/`、`docs/adr/`、`docs/guides/` 子目录
- [x] 1.2 创建 ADR 模板文件 `docs/adr/TEMPLATE.md`（含编号、标题、状态、上下文、决策、后果）
- [x] 1.3 创建 ADR 索引文件 `docs/adr/README.md`（占位，后续填充链接）

## 2. API Reference 文档

- [x] 2.1 创建 `docs/api/auth.md` — 认证模块 API（login/register/refresh/logout/me）
- [x] 2.2 创建 `docs/api/tasks.md` — 任务模块 API（CRUD + 分解 + 状态变更 + 评论）
- [x] 2.3 创建 `docs/api/projects.md` — 项目模块 API（CRUD + 成员管理）
- [x] 2.4 创建 `docs/api/users.md` — 用户模块 API（me/invite）
- [x] 2.5 创建 `docs/api/reviews.md` — Review 模块 API（创建/列表/详情/决策）
- [x] 2.6 创建 `docs/api/plugins.md` — 插件模块 API（catalog/install/config）
- [x] 2.7 创建 `docs/api/agents.md` — Agent 模块 API（spawn/session/health）
- [x] 2.8 创建 `docs/api/errors.md` — 统一错误码参考表

## 3. Database Schema 文档

- [x] 3.1 创建 `docs/schema/postgres.md` — PostgreSQL ER 图（Mermaid）+ 表/字段/索引/外键完整文档
- [x] 3.2 创建 `docs/schema/redis.md` — Redis key pattern 文档（token 黑名单、会话、缓存）

## 4. Deployment 文档

- [x] 4.1 创建 `docs/deployment/docker.md` — Docker Compose 部署指南（配置、持久化、网络、健康检查）
- [x] 4.2 创建 `docs/deployment/desktop-build.md` — Tauri 桌面打包指南（跨平台、签名、更新）
- [x] 4.3 创建 `docs/deployment/environment-variables.md` — 环境变量完整参考（按服务分组，必需/可选标注）
- [x] 4.4 创建 `docs/deployment/tls.md` — TLS 配置指南（证书、反向代理、安全头）

## 5. Security 文档

- [x] 5.1 创建 `docs/security/authentication.md` — 认证授权流程（JWT + Redis 黑名单 + CORS）
- [x] 5.2 创建 `docs/security/tauri-permissions.md` — Tauri 权限模型（Capability、IPC 安全边界）
- [x] 5.3 创建 `docs/security/best-practices.md` — 安全编码最佳实践清单

## 6. Architecture Decision Records

- [x] 6.1 创建 `docs/adr/0001-why-tauri-not-electron.md`
- [x] 6.2 创建 `docs/adr/0002-why-go-nextjs-dual-stack.md`
- [x] 6.3 创建 `docs/adr/0003-why-wasm-plugin-runtime.md`
- [x] 6.4 创建 `docs/adr/0004-why-jwt-redis-auth.md`
- [x] 6.5 创建 `docs/adr/0005-why-bun-ts-bridge.md`
- [x] 6.6 更新 `docs/adr/README.md` 索引，列出所有 ADR 链接

## 7. Plugin Development Guide

- [x] 7.1 创建 `docs/guides/plugin-development.md` — 插件开发入门（概念 + Hello World 双路径）
- [x] 7.2 创建 `docs/guides/plugin-wasm.md` — WASM 插件详细开发指南
- [x] 7.3 创建 `docs/guides/plugin-mcp.md` — MCP 插件详细开发指南

## 8. Frontend 组件与状态管理文档

- [x] 8.1 创建 `docs/guides/frontend-components.md` — 关键 UI 组件使用文档
- [x] 8.2 创建 `docs/guides/state-management.md` — Zustand 状态管理指南

## 9. 现有文档改进

- [x] 9.1 在 CONTRIBUTING.md 顶部新增 Quick Start 5 分钟教程章节
- [x] 9.2 在 CONTRIBUTING.md 新增环境变量速查表章节
- [x] 9.3 在 CONTRIBUTING.md 新增 FAQ / 常见问题排查章节
- [x] 9.4 在 TESTING.md 新增 IM Bridge 端到端测试流程章节
- [x] 9.5 在 TESTING.md 新增 Bridge 测试矩阵章节
- [x] 9.6 在 TESTING.md 新增覆盖率提升指南章节
- [x] 9.7 改进 README.md：补充架构总览图（Mermaid）、功能特性矩阵表格
- [x] 9.8 同步更新 README_zh.md（与 README.md 对齐）
