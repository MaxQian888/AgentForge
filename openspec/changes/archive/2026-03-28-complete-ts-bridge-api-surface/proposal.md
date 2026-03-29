## Why

TS Bridge 已有完整的路由定义和核心执行能力（Claude Agent SDK 集成、多 Runtime/Provider 支持、Plugin/MCP 管理），但 Go 后端对 Bridge 的调用覆盖不完整（18 个 client 方法中仅 7 个被 service 层实际调用），前端在单 Agent 执行路径缺少运行时配置 UI、Bridge 健康监控不充分、Plugin 生命周期管理缺乏 Bridge 侧可视化。需要补齐 Go↔Bridge 的调用链路、前端操作面板、以及测试覆盖，使整个 TS Bridge 功能面可用且可观测。

## What Changes

- Go service 层补齐对 Bridge `GetStatus`、`Health`、`Generate`、`ClassifyIntent`、`GetRuntimeCatalog` 的调用，暴露为 REST API
- Go 后端增加 Bridge 健康检查机制（启动探针 + 周期性心跳），失败时标记降级状态
- 前端单 Agent Spawn 增加 Runtime/Provider/Model/Budget 选择对话框，复用 `StartTeamDialog` 中已有的运行时选择组件
- 前端增加 Bridge 运行时状态面板（Runtime Catalog 展示、Pool 详情、健康状态），集成到 Agents 页面
- 前端 Agent 恢复（Resume）工作流补齐 UI：暂停 Agent 列表 + 恢复操作
- Bridge server.test.ts 补齐 `/bridge/active`、`/bridge/pool`、`/bridge/tools/*`、`/bridge/plugins/*` 路由的测试覆盖
- Go bridge client_test.go 补齐未覆盖方法的单元测试

## Capabilities

### New Capabilities
- `bridge-health-probe`: Go 后端对 TS Bridge 的健康探针机制——启动就绪检查、周期性心跳、降级状态传播
- `bridge-runtime-dashboard`: 前端 Bridge 运行时状态仪表盘——Runtime Catalog、Pool 详情、健康状态、Agent 恢复工作流

### Modified Capabilities
- `bridge-http-contract`: 补齐 Go service 层对已定义但未使用的 Bridge 路由（status、health、generate、classify-intent、runtimes）的调用链路
- `agent-sdk-bridge-runtime`: 补齐 Agent 执行生命周期中 status polling 和 resume 的端到端集成
- `bridge-agent-runtime-registry`: 前端暴露 Runtime Catalog 查询，供单 Agent spawn 和管理面板使用
- `bridge-provider-support`: 前端暴露 Provider/Model 选择，复用 team dialog 中已有的选择逻辑

## Impact

- **Go 后端** (`src-go/`): bridge client 调用补齐、新增 health probe service、handler 层新增 API 路由
- **TS Bridge** (`src-bridge/`): 测试补齐，无功能变更
- **前端** (`components/`, `lib/stores/`, `app/`): 新增 spawn 配置对话框、bridge 状态面板、agent resume UI；提取 team dialog 中的运行时选择为共享组件
- **API 契约**: 新增 `GET /api/v1/bridge/health`、`GET /api/v1/bridge/runtimes`、`POST /api/v1/ai/generate`、`POST /api/v1/ai/classify-intent` 等端点
