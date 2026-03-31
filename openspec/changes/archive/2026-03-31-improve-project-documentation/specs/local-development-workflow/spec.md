## ADDED Requirements

### Requirement: Quick Start 5 分钟教程
CONTRIBUTING.md SHALL 在顶部新增一个「Quick Start」章节，提供 5 分钟快速上手教程，包含：前置依赖安装（一行命令）、克隆与安装、启动开发环境（`pnpm dev:all`）、验证运行成功、打开浏览器查看效果。

#### Scenario: 新贡献者 5 分钟内启动项目
- **WHEN** 新贡献者按照 Quick Start 教程操作
- **THEN** 在 5 分钟内能在浏览器中看到 AgentForge 运行中的界面

### Requirement: 环境变量速查表
CONTRIBUTING.md SHALL 包含环境变量速查表，按类别列出开发环境常用的环境变量：数据库连接、JWT 配置、CORS、日志级别。包含 `.env.example` 文件引用。

#### Scenario: 快速配置本地开发环境
- **WHEN** 开发者首次设置开发环境
- **THEN** 速查表列出必需的环境变量和推荐值，无需查阅多个文档

### Requirement: 常见问题排查指南
CONTRIBUTING.md SHALL 包含 FAQ 章节，覆盖常见问题：端口冲突、Node 版本不匹配、Go 编译错误、Tauri 构建失败、数据库连接失败。

#### Scenario: 解决端口冲突
- **WHEN** 开发者遇到 `EADDRINUSE` 错误
- **THEN** FAQ 提供排查步骤：如何查找占用进程、如何更改默认端口
