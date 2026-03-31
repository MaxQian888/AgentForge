## ADDED Requirements

### Requirement: 认证授权流程文档
系统 SHALL 在 `docs/security/authentication.md` 中提供完整的认证授权流程文档，包含：注册/登录流程、JWT Access Token + Refresh Token 双层机制、Token 刷新流程、Redis token 黑名单与吊销机制、CORS 策略配置。

#### Scenario: 理解完整认证生命周期
- **WHEN** 开发者阅读认证文档
- **THEN** 文档包含从用户注册到 token 刷新再到主动登出的完整流程图和说明

#### Scenario: 理解 token 黑名单机制
- **WHEN** 开发者需要理解登出后 token 如何失效
- **THEN** 文档说明 JWT 无状态特性下的黑名单方案：Redis 存储、TTL 对齐、fail-closed 策略

### Requirement: Tauri 权限模型文档
系统 SHALL 在 `docs/security/tauri-permissions.md` 中提供 Tauri 权限模型文档，包含：Capability 配置、权限范围（文件系统、网络、Shell）、安全边界、IPC 通信安全。

#### Scenario: 理解桌面端安全边界
- **WHEN** 开发者需要在前端调用系统 API
- **THEN** 文档说明 Tauri capability 配置方式和权限白名单机制

### Requirement: 安全最佳实践文档
系统 SHALL 在 `docs/security/best-practices.md` 中提供安全最佳实践文档，覆盖：密钥管理、输入验证、SQL 注入防护、XSS 防护、依赖审计。

#### Scenario: 新开发者了解安全规范
- **WHEN** 新贡献者加入项目
- **THEN** 安全最佳实践文档提供可操作的安全编码清单
