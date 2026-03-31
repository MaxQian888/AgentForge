## ADDED Requirements

### Requirement: ADR 模板与目录结构
系统 SHALL 在 `docs/adr/` 目录下提供 ADR 模板文件 `TEMPLATE.md` 和索引文件 `README.md`。模板 SHALL 包含：编号、标题、状态（Proposed/Accepted/Deprecated/Superseded）、上下文、决策、后果。

#### Scenario: 创建新的架构决策记录
- **WHEN** 开发者需要记录一个架构决策
- **THEN** 可复制 `docs/adr/TEMPLATE.md` 创建新文件，按编号递增命名（如 `0006-xxx.md`）

#### Scenario: 查看所有架构决策
- **WHEN** 开发者打开 `docs/adr/README.md`
- **THEN** 索引列出所有 ADR 的编号、标题、状态和简要说明

### Requirement: 首批 5 条关键 ADR
系统 SHALL 包含以下首批架构决策记录：
1. ADR-0001: 为什么选择 Tauri 而非 Electron
2. ADR-0002: 为什么使用 Go + Next.js 双栈架构
3. ADR-0003: 为什么引入 WASM 插件运行时
4. ADR-0004: 为什么使用 JWT + Redis 双层认证
5. ADR-0005: 为什么使用 Bun 作为 TypeScript Bridge 运行时

#### Scenario: 理解 Tauri 选型决策
- **WHEN** 开发者打开 `docs/adr/0001-why-tauri-not-electron.md`
- **THEN** 文档记录当时的技术评估上下文、选择 Tauri 的理由、以及已知的后果和权衡

#### Scenario: 理解认证架构决策
- **WHEN** 开发者打开 `docs/adr/0004-why-jwt-redis-auth.md`
- **THEN** 文档记录 JWT 无状态 + Redis 黑名单的组合选型理由和 fail-closed 策略的决策背景
