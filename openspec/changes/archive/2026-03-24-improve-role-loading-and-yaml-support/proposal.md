## Why

AgentForge 的 PRD 已经把 Role Plugin 定义为 MVP 核心能力，要求通过 YAML 角色定义驱动数字员工的身份、能力、知识和安全约束；但当前仓库里的角色加载仍停留在扁平旧结构、文件级 CRUD 和预设角色硬编码阶段，和文档约定存在明显断层。现在需要把角色加载、校验和执行投影补到一个可实现、可验证的最小闭环，避免后续 Agent 绑定、角色扩展和 YAML 资产继续沿着不兼容的数据模型演进。

## What Changes

- 将现有角色加载从扁平旧版 manifest 对齐到 PRD 定义的 Role YAML 结构，支持 `apiVersion`、`kind`、分层 `metadata/identity/capabilities/knowledge/security` 等核心字段。
- 为 Go 侧补齐统一的角色注册与加载链路，覆盖内置预设角色、磁盘 YAML 角色、目录化 `roles/{role_id}/role.yaml` 发现、基础校验与错误语义，而不是继续由 handler 各自直接读文件。
- 定义角色执行投影规则，把完整 Role YAML 归一化为 Agent 运行时实际需要的 `system_prompt`、工具权限、预算、并发和权限模式等执行配置。
- 为 Agent 启动链路补一个最小的角色引用入口，让 `POST /api/v1/agents/spawn` 可以通过 `roleId` 选择角色，并由 Go 在启动前解析、投影并注入 Bridge 执行请求。
- 扩展角色 API 与持久化约束，使创建、读取、更新角色时围绕 YAML 资产与规范化角色模型工作，并为后续继承/合并和更多角色能力字段预留稳定扩展点。
- 补充 PRD 对齐的样例角色、验证用例和文档说明，确保仓库内现有角色文件可以被真实加载、列出、查询并用于 Agent 执行配置生成。

## Capabilities

### New Capabilities
- `role-plugin-support`: 定义 AgentForge 如何发现、校验、归一化和管理基于 YAML 的 Role Plugin，包括 PRD 对齐的 schema、角色注册表、目录布局、API 读写语义，以及面向 Agent 执行的配置投影。

### Modified Capabilities
- `agent-sdk-bridge-runtime`: Agent 执行请求需要与 Go 侧归一化后的角色执行配置对齐，明确运行时如何消费由 Role YAML 派生的系统提示、工具权限、预算和权限模式。

## Impact

- Affected Go role code: `src-go/internal/model/role.go`, `src-go/internal/role/*`, `src-go/internal/handler/role_handler.go`, `src-go/internal/server/routes.go`, 以及角色相关测试。
- Affected agent execution contract: `src-go/internal/bridge/client.go`、`src-go/internal/service/agent_service.go`、`src-go/internal/handler/agent_handler.go`、`src-go/internal/model/agent_run.go`、`src-bridge/src/schemas.ts` 及相关运行时调用点需要和新的角色执行投影保持一致。
- Affected role assets and configuration: `roles/**` 的文件组织、内置预设角色来源、样例 YAML 与迁移兼容策略。
- Affected documentation: `docs/PRD.md` 对齐说明、角色使用文档和验证说明需要反映新的 YAML 加载与角色能力边界。
