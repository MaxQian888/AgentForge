## Context

AgentForge 是一个全栈桌面+Web 应用，技术栈横跨 Next.js 16 / React 19、Go 1.25、Tauri 2、Bun。当前已有：
- 根目录 8 个文档文件（README.md, CLAUDE.md, AGENTS.md, CONTRIBUTING.md, TESTING.md, CI_CD.md, CHANGELOG.md, README_zh.md）
- docs/ 目录下 20+ 设计文档（PRD、架构设计、技术挑战等）
- openspec/specs/ 下 100+ spec 目录

但缺少面向日常开发者的「操作性文档」：API 参考、数据库 Schema、部署流程、安全模型。现有文档偏向「设计意图」而非「操作指南」。

## Goals / Non-Goals

**Goals:**
- 补全 API Reference、Database Schema、Deployment、Security、ADR 五类核心文档
- 提供 Quick Start 5 分钟教程，降低新贡献者上手门槛
- 创建 Plugin Development Guide 完整教程（WASM + MCP 路径）
- 建立 ADR 体系，记录关键架构决策
- 改进 CONTRIBUTING.md 和 TESTING.md，补充实操细节

**Non-Goals:**
- 不自动生成 API 文档（如 Swagger UI）——本阶段以手写 Markdown 为主，后续可引入工具链
- 不创建 Storybook 组件目录——仅做 Markdown 格式的组件使用指南
- 不重写现有设计文档——仅更新入口级文档
- 不涉及国际化文档（i18n）——所有新文档使用中文+英文双语标题
- 不建立文档站点（如 Docusaurus）——仅仓库内 Markdown

## Decisions

### D1: 文档文件组织策略

**决策**: 新文档统一放入 `docs/` 目录，按类型分子目录：`docs/api/`、`docs/schema/`、`docs/deployment/`、`docs/security/`、`docs/adr/`、`docs/guides/`。

**理由**: 现有 `docs/` 已有 20+ 文件直接平铺，继续平铺会导致混乱。子目录分类让开发者快速定位。ADR 使用编号前缀（`NNNN-kebab-case.md`）方便排序。

**替代方案**: 创建独立 `documentation/` 目录——增加导航层级但不改善组织性。

### D2: API 文档以端点模块为粒度

**决策**: 按 Go 后端路由模块拆分文件：`docs/api/auth.md`、`docs/api/tasks.md`、`docs/api/projects.md` 等。每个文件覆盖该模块所有端点的请求/响应格式。

**理由**: 按模块拆分比单一大文件更易维护；与 Go 代码中的 handler 分组对齐。

### D3: ADR 格式采用轻量 Markdown 模板

**决策**: 使用业界标准 ADR 模板（Status / Context / Decision / Consequences），Markdown 格式，存放在 `docs/adr/`。

**理由**: ADR 的价值在于记录决策上下文，不需要复杂工具。轻量模板降低撰写门槛。

**首批 ADR 清单**:
1. ADR-0001: 为什么选择 Tauri 而非 Electron
2. ADR-0002: 为什么使用 Go + Next.js 双栈架构
3. ADR-0003: 为什么引入 WASM 插件运行时
4. ADR-0004: 为什么使用 JWT + Redis 双层认证
5. ADR-0005: 为什么使用 Bun 作为 TypeScript Bridge 运行时

### D4: Plugin Development Guide 双路径结构

**决策**: 指南分为两条独立路径：WASM Integration Plugin（Go 端）和 MCP Tool/Review Plugin（TypeScript 端），共享一个「概念入门」章节。

**理由**: 两种插件模型的技术栈、工具链、生命周期完全不同，混合讲解会增加认知负担。

### D5: 文档语言策略

**决策**: 所有新增文档使用英文撰写，关键标题附中文注释。README 更新保持中英双语同步。

**理由**: 项目已有英文为主的代码库和国际化的 README。混合语言会增加维护成本，但中文注释帮助中文使用者快速定位。

## Risks / Trade-offs

- **[文档与代码不同步]** → 在 CONTRIBUTING.md 的 PR checklist 中增加「文档是否需要更新」检查项；建议后续引入 doc-lint 工具
- **[大量文档增加仓库体积]** → 文档为纯 Markdown，体积影响可忽略；图片等二进制资源使用 Git LFS 或外部链接
- **[手写 API 文档维护成本高]** → 后续可引入 swaggo/swag 等工具从 Go 注解自动生成；本阶段先建立骨架和范例
- **[ADR 数量可能快速增长]** → 仅记录跨模块的技术决策，避免为每个小选择写 ADR
