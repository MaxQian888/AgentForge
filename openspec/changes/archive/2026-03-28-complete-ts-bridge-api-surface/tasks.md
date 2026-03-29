## 1. Go 后端 Bridge 健康探针

- [x] 1.1 新增 `BridgeHealthService`（`src-go/internal/service/bridge_health_service.go`）：启动 readiness probe（10 次重试，2s 间隔），周期性心跳（30s），维护 `ready`/`degraded` 状态，3 次连续失败降级
- [x] 1.2 在 `server.go` 启动流程中初始化 `BridgeHealthService`，注入 bridge client
- [x] 1.3 新增 handler `GET /api/v1/bridge/health`（`bridge_health_handler.go`），返回 `{status, last_check, pool}` JSON
- [x] 1.4 在 agent spawn/pause/resume handler 中检查 bridge 状态，`degraded` 时返回 503
- [x] 1.5 编写 `bridge_health_service_test.go` 单元测试：startup probe 成功/失败、状态转换、降级恢复

## 2. Go 后端补齐 Bridge 调用链路

- [x] 2.1 新增 handler `GET /api/v1/bridge/runtimes`，代理 `bridge.GetRuntimeCatalog()` 并缓存 60s
- [x] 2.2 新增 handler `POST /api/v1/ai/generate`，代理 `bridge.Generate()`，接收 `{prompt, provider?, model?}`
- [x] 2.3 新增 handler `POST /api/v1/ai/classify-intent`，代理 `bridge.ClassifyIntent()`，接收 `{text, candidates}`
- [x] 2.4 在 `routes.go` 注册上述 3 个路由，挂载 auth middleware
- [x] 2.5 Agent service 中 spawn 后增加 post-spawn status verification：5s 内未收到 WS event 时 fallback 调用 `bridge.GetStatus()`
- [x] 2.6 补齐 `client_test.go` 中 `GetStatus`、`Health`、`Generate`、`ClassifyIntent`、`GetRuntimeCatalog` 的 httptest mock 测试

## 3. 前端共享组件提取

- [x] 3.1 从 `StartTeamDialog` 中提取 `RuntimeSelector` 组件到 `components/shared/runtime-selector.tsx`，接收 catalog 数据、emit `{runtime, provider, model}`
- [x] 3.2 重构 `StartTeamDialog` 使用提取的 `RuntimeSelector`，确保行为不变
- [x] 3.3 编写 `RuntimeSelector` 组件测试：选项过滤、不可用运行时禁用、provider 联动

## 4. 前端 SpawnAgentDialog

- [x] 4.1 新增 `SpawnAgentDialog` 组件（`components/tasks/spawn-agent-dialog.tsx`），使用 `RuntimeSelector` + budget 输入，调用 agent store spawn
- [x] 4.2 修改 `task-detail-content.tsx` 中"Start Agent"按钮打开 `SpawnAgentDialog` 而非直接 spawn
- [x] 4.3 在 `agent-store.ts` 中添加 `fetchRuntimeCatalog()` 方法，从 `GET /api/v1/bridge/runtimes` 获取并缓存 catalog

## 5. 前端 Bridge 运行时仪表盘

- [x] 5.1 在 `agent-store.ts` 中添加 `fetchBridgeHealth()` 方法，从 `GET /api/v1/bridge/health` 获取状态
- [x] 5.2 在 Agents 页面（`app/(dashboard)/agents/page.tsx`）增加 Bridge 健康状态 banner（ready/degraded 指示）
- [x] 5.3 在 Agents 页面增加 Runtime Catalog 展示区域（使用 catalog 数据渲染运行时卡片）
- [x] 5.4 在 Agents 页面增加 Paused Agents 区域：过滤 paused 状态 agent，每行提供 Resume 按钮

## 6. TS Bridge 测试补齐

- [x] 6.1 在 `server.test.ts` 中补齐 `/bridge/active` 端点测试：有/无运行中 agent 场景
- [x] 6.2 补齐 `/bridge/pool` 端点测试：不同 slot 分配场景、空池场景
- [x] 6.3 补齐 `/bridge/tools/*` 端点测试：install、uninstall、list、restart 流程
- [x] 6.4 补齐 `/bridge/plugins/*` 端点测试：register、enable、disable、health、MCP refresh、tool call

## 7. 集成验证

- [ ] 7.1 端到端验证：前端 SpawnAgentDialog → Go API → Bridge execute → WS 事件回传 → 前端状态更新
- [ ] 7.2 验证 Bridge 降级时前端展示 degraded banner 且 spawn 按钮禁用
- [ ] 7.3 验证 Agent pause → Agents 页面显示 paused agent → Resume → 恢复运行
