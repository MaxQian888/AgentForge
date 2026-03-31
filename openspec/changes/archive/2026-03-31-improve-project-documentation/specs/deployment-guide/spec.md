## ADDED Requirements

### Requirement: Docker Compose 部署指南
系统 SHALL 在 `docs/deployment/docker.md` 中提供 Docker Compose 部署指南，包含：docker-compose.yml 配置说明、环境变量配置、数据持久化（PostgreSQL/Redis volume）、网络配置、健康检查。

#### Scenario: 从零部署 AgentForge 后端
- **WHEN** 运维人员执行部署
- **THEN** 按照文档能在 15 分钟内通过 `docker compose up` 启动完整的后端服务（Go API + PostgreSQL + Redis）

#### Scenario: 配置生产环境变量
- **WHEN** 运维人员准备生产部署
- **THEN** 文档列出所有必需的环境变量，包含推荐值、安全注意事项、多环境配置方案

### Requirement: Tauri 桌面应用打包指南
系统 SHALL 在 `docs/deployment/desktop-build.md` 中提供 Tauri 桌面应用打包指南，包含：前置依赖（Rust toolchain、系统库）、构建命令、跨平台构建（macOS/Windows/Linux）、代码签名、自动更新配置。

#### Scenario: 构建 Windows 安装包
- **WHEN** 开发者需要构建 Windows MSI/NSIS 安装包
- **THEN** 文档提供完整的构建步骤、签名配置和产物路径

#### Scenario: 配置自动更新
- **WHEN** 开发者需要启用桌面应用自动更新
- **THEN** 文档说明 Tauri updater 配置步骤、签名密钥管理和更新服务器搭建

### Requirement: 环境变量完整参考
系统 SHALL 在 `docs/deployment/environment-variables.md` 中提供环境变量完整参考，按服务分组（Go 后端、TS Bridge、Next.js 前端），标注必需/可选、默认值、安全等级。

#### Scenario: 快速查找某个环境变量
- **WHEN** 开发者需要了解 `JWT_ACCESS_TTL` 的作用和推荐值
- **THEN** 文档提供该变量的说明、类型、默认值和安全建议
