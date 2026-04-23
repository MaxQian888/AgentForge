# Go 后端深度审计报告

> 审计日期：2026-04-23
> 审计范围：`src-go/` 全代码库（30+ 包，159 service 文件，122 handler 文件，105 repository 文件，78 迁移）
> 审计方法：6 个并行 agent + 逐行人工阅读关键文件

---

## 目录

- [1. 项目概览](#1-项目概览)
- [2. 总体评估](#2-总体评估)
- [3. P0 — 关键安全漏洞](#3-p0--关键安全漏洞)
- [4. P1 — 严重正确性问题](#4-p1--严重正确性问题)
- [5. P2 — 中等问题](#5-p2--中等问题)
- [6. P3 — 低优先级 / 技术债务](#6-p3--低优先级--技术债务)
- [7. agent_service.go 逐函数分析](#7-agent_servicego-逐函数分析)
- [8. WebSocket 安全详析](#8-websocket-安全详析)
- [9. 认证流程与 Auth 服务](#9-认证流程与-auth-服务)
- [10. Workflow 引擎正确性](#10-workflow-引擎正确性)
- [11. Plugin 系统（WASM 沙箱）](#11-plugin-系统wasm-沙箱)
- [12. Knowledge 引擎](#12-knowledge-引擎)
- [13. Trigger 引擎](#13-trigger-引擎)
- [14. 并发模式汇总](#14-并发模式汇总)
- [15. 各层质量评价](#15-各层质量评价)
- [16. 修复优先级与建议](#16-修复优先级与建议)

---

## 1. 项目概览

| 指标 | 数值 |
|------|------|
| Go 版本 | 1.25 |
| 框架 | Echo v4 + GORM + Redis + WebSocket + WASM (wazero) |
| 内部包 | 30+ |
| Service 文件 | 159 |
| Handler 文件 | 122 |
| Repository 文件 | 105 |
| Model 文件 | 60 |
| 数据库迁移 | 78 (up/down pairs) |
| 入口点 | 11 (cmd/) |

架构分层：`HTTP Request → Handler → Service → Repository → Model → Database`

---

## 2. 总体评估

| 层 | 评价 | 关键发现 |
|----|------|----------|
| **Repository** | 优秀 | 全参数化查询，资源清理规范，NULL 处理一致，无 SQL 注入 |
| **Model** | 良好 | 指针类型正确处理可空字段，与 repository 匹配 |
| **Service** | 中等 | 业务逻辑正确但存在 god object、静默错误、goroutine 泄漏 |
| **Handler** | 中等偏下 | 格式不统一、验证缺失、部分端点暴露内部错误信息 |
| **Middleware** | 良好 | RBAC 矩阵完善，JWT 验证规范，但服务 token 时序攻击风险 |
| **WebSocket** | 差 | 3 个端点中 2 个无认证，黑名单不检查，CORS 绕过 |
| **Workflow** | 中等 | DAG 评估基本正确，但存在非原子状态更新 |
| **Trigger** | 中等 | 竞态条件、无重试、dryrun 不完整 |
| **Plugin** | 中等偏下 | WASM 无沙箱限制、缓存无上限 |
| **Knowledge** | 中等 | 功能完整但无大文件防护 |
| **Auth** | 中等 | 流程正确但无审计日志，密码变更不使 token 失效 |

---

## 3. P0 — 关键安全漏洞

### 3.1 Bridge WebSocket 无认证

**文件**: `internal/ws/bridge_handler.go:28-80`
**严重性**: Critical

```go
func (h *BridgeHandler) HandleWS(c echo.Context) error {
    // 无 JWT 验证
    // 无 token 检查
    conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
    // ...直接处理消息...
}
```

**攻击场景**: 内网攻击者连接 `/ws/bridge`，注入伪造的 `BridgeAgentEvent`，可：
- 伪造 agent 成本报告
- 伪造 agent 完成状态
- 触发 budget alert 和 IM 通知
- 伪造权限请求

**修复**: 添加共享密钥认证（header 或首条消息握手）或 mTLS。

### 3.2 IM Control WebSocket 无认证

**文件**: `internal/ws/im_control_handler.go:35-165`
**严重性**: Critical

```go
func (h *IMControlHandler) HandleWS(c echo.Context) error {
    bridgeID := c.QueryParam("bridgeId")
    // 无 JWT 验证
    conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
```

**攻击场景**: 枚举 bridgeId，窃听 IM 消息，注入伪造 ACK。

**修复**: 同上，添加共享密钥认证。

### 3.3 用户 WebSocket 无 Token 黑名单检查

**文件**: `internal/ws/handler.go:60-71`
**严重性**: High

```go
// handler.go — 只验证 JWT 签名，不检查黑名单
_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
    return []byte(h.jwtSecret), nil
})
// 缺少：blacklist.IsBlacklisted(claims.JTI)
```

对比 `middleware/jwt.go:50-60` 中的 HTTP 中间件会检查黑名单：

```go
// middleware/jwt.go — HTTP 请求正确检查黑名单
blacklisted, err := blacklist.IsBlacklisted(c.Request().Context(), claims.JTI)
if blacklisted {
    return c.JSON(http.StatusUnauthorized, ...)
}
```

**攻击场景**: 用户 logout 后，已撤销的 access token 仍可建立 WebSocket 长连接并保持。

**修复**: 在 WS upgrade 流程中注入 `TokenBlacklist` 依赖并检查 `claims.JTI`。

### 3.4 内部端点无认证

**文件**: `internal/server/routes.go`
**严重性**: High

| 端点 | 行号 | 风险 |
|------|------|------|
| `/internal/scheduler/jobs` | 1434 | 任务枚举 |
| `/internal/scheduler/jobs/:jobKey/trigger` | 1435 | 未授权触发定时任务 |
| `/internal/plugins/runtime-state` | 1588 | 篡改插件运行时状态 |
| `/api/v1/internal/logs/ingest` | 1207 | 仅限频，可注入恶意日志 |

代码注释承认需要 "shared-secret or mTLS guard"，但尚未实现。

**修复**: 为所有 `/internal/*` 路由组添加共享密钥中间件。

### 3.5 CheckOrigin 全放行

**文件**: `internal/ws/handler.go:26-28`
**严重性**: High

```go
var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true // CORS handled by Echo middleware
    },
}
```

注释错误——WebSocket upgrade 发生在 Echo CORS 中间件之前，任何源都可建立 WebSocket 连接。

**修复**: 实现 `CheckOrigin` 校验 `Origin` header 是否在 `ALLOW_ORIGINS` 列表中。

### 3.6 Service Token 时序攻击

**文件**: `internal/middleware/review_trigger.go`
**严重性**: Medium

```go
if serviceToken != "" && tokenStr == serviceToken {
    return next(c)
}
```

使用 `==` 比较字符串，可通过时序攻击逐字节推测 token。

**修复**: 使用 `crypto/subtle.ConstantTimeCompare`。

---

## 4. P1 — 严重正确性问题

### 4.1 agent_service.go 后台 Goroutine 无超时

**文件**: `internal/service/agent_service.go:584-617`
**严重性**: High

```go
// L584-598: ProcessRunCompletion
go func(parentTrace string) {
    bgCtx := context.Background()  // 无取消机制
    // ... 无 timeout ...
    s.teamSvc.ProcessRunCompletion(bgCtx, run)  // 如果挂起则 goroutine 永久泄漏
}(applog.TraceID(ctx))

// L602-616: HandleAgentRunCompletion
go func(parentTrace string) {
    bgCtx := context.Background()  // 同上
    _ = s.dagWorkflowSvc.HandleAgentRunCompletion(bgCtx, run.ID, ...)
}(applog.TraceID(ctx))
```

两个独立 goroutine 均无 timeout、无 panic recovery、无 WaitGroup 跟踪。

**修复**:

```go
go func(parentTrace string) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    defer func() {
        if r := recover(); r != nil {
            log.WithField("panic", r).Error("ProcessRunCompletion panicked")
        }
    }()
    s.teamSvc.ProcessRunCompletion(ctx, run)
}(applog.TraceID(ctx))
```

### 4.2 事件发布错误静默丢弃

**文件**: `internal/service/agent_service.go:1438-1440` 及 15+ 处
**严重性**: High

```go
func (s *AgentService) broadcastEvent(ctx context.Context, eventType, projectID string, payload any) {
    _ = eventbus.PublishLegacy(ctx, s.bus, eventType, projectID, payload)
}
```

出现在：`agent_service.go:530,625,629,692-693,799` | `task_service.go:86,144,158,171` | `workflow_execution_service.go:371,792` 等。

**影响**: 前端 WebSocket 状态不同步且无法排查原因。

**修复**: 至少记录日志：

```go
if err := eventbus.PublishLegacy(ctx, s.bus, eventType, projectID, payload); err != nil {
    log.WithError(err).WithFields(log.Fields{"eventType": eventType, "projectID": projectID}).
        Warn("event publish failed")
}
```

### 4.3 Spawn TOCTOU 竞态

**文件**: `internal/service/agent_service.go:375-383`
**严重性**: Medium-High

```go
// 检查阶段
runs, err := s.runRepo.GetByTask(ctx, taskID)
for _, r := range runs {
    if r.Status == model.AgentRunStatusRunning || r.Status == model.AgentRunStatusStarting {
        return nil, ErrAgentAlreadyRunning
    }
}
// ... 中间有多个步骤 ...
// L448: 创建阶段
if err := s.runRepo.Create(ctx, run); err != nil {
```

两个并发 Spawn 请求可能同时通过检查，都创建 run。

**修复**: 使用数据库 advisory lock 或在 agent_runs 表上添加条件约束。

### 4.4 Loop 状态非原子更新

**文件**: `internal/workflow/nodetypes/applier.go:389-414`
**严重性**: Medium-High

```go
// 步骤 1: 删除节点执行记录
if err := a.NodeRepo.DeleteNodeExecutionsByNodeIDs(ctx, exec.ID, p.NodeIDs); err != nil {
    return err
}
// --- 如果这里崩溃，节点已删除但 counter 未更新 ---
// 步骤 2: 更新计数器
if err := a.ExecRepo.UpdateExecutionDataStore(ctx, exec.ID, updated); err != nil {
    return fmt.Errorf("update datastore: %w", err)
}
```

两个操作不在同一事务中。中间崩溃会导致 workflow 丢失迭代进度，可能卡在不可恢复状态。

**修复**: 使用数据库事务包裹两个操作。

### 4.5 Trigger Schedule Ticker 竞态

**文件**: `internal/trigger/schedule_ticker.go:140-193`
**严重性**: Medium

```go
// shouldFire 在锁内检查
func (t *ScheduleTicker) shouldFire(tr *model.WorkflowTrigger, minute time.Time) bool {
    t.mu.Lock()
    defer t.mu.Unlock()
    // ... 检查 lastFire ...
    return true  // ← 锁释放
}

// dispatchOne 在锁外执行
func (t *ScheduleTicker) dispatchOne(ctx context.Context, tr *model.WorkflowTrigger, minute time.Time) {
    // ... Route ...
    t.mu.Lock()
    t.lastFire[tr.ID.String()] = minute  // ← 更新 lastFire
    t.mu.Unlock()
}
```

`shouldFire` 返回 true 后锁释放，在 `dispatchOne` 执行 Route 之前，另一个 tick 可再次调用 `shouldFire` 并得到 true（因为 lastFire 未更新），导致同一 trigger 在同一分钟被触发两次。

**修复**: 将 lastFire 更新移到 dispatchOne 之前（在锁内），或使用 compare-and-swap 模式。

### 4.6 Auth 服务无审计日志

**文件**: `internal/service/auth_service.go`
**严重性**: Medium-High

`Login`（L86-99）、`Register`（L55-83）、`ChangePassword`（L245-267）、`Refresh`（L103-143）均无日志记录。无法检测暴力破解、异常登录模式或安全事件。

**修复**: 为所有认证操作添加结构化审计日志，包括成功/失败、IP 地址、时间戳。

### 4.7 Wait Event 无超时清理

**文件**: `internal/workflow/nodetypes/wait_event.go:17-56`
**严重性**: Medium

等待事件的节点如果事件永远不到达，workflow 永久停泊在 `waiting` 状态。无超时机制，无清理 job。

**修复**: 添加可配置的超时参数，以及定期扫描 `waiting` 状态节点的清理 job。

---

## 5. P2 — 中等问题

### 5.1 God Object 服务

| 服务 | 行数 | 建议拆分 |
|------|------|----------|
| `agent_service.go` | 2,697 | Lifecycle / Dispatch / Notification / Cost / Event |
| `plugin_service.go` | ~2,000+ | Registry / Lifecycle / Health / RemoteRegistry |
| `im_control_plane.go` | ~50KB | Bridges / Deliveries / Channels / Actions |

### 5.2 WASM 插件无资源限制

**文件**: `internal/plugin/runtime.go:200-204, 251-279`

```go
runtime := wazero.NewRuntime(ctx)  // 无内存限制
if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {  // 无目录限制
```

- 无 CPU/内存/执行时间限制
- WASI preview1 允许文件系统访问，但未配置目录限制
- 编译缓存无 LRU 淘汰（L183-231），无限增长

**修复**: 配置 wazero 内存限制和执行超时；实现 LRU 缓存淘汰。

### 5.3 Knowledge 大文件无防护

**文件**: `internal/knowledge/ingest_worker.go:77-92, 132-133`

```go
rc, err := w.blobs.Get(ctx, a.FileRef)  // 无大小限制
chunks, err := parser.Parse(rc)
// plainTextParser:
b, err := io.ReadAll(r)  // 整个文件加载到内存
```

GB 级文件可导致 OOM crash。失败 ingestion 不清理部分 chunks（L96 错误被忽略）。

**修复**: 添加文件大小上限（如 100MB），使用流式解析器替代 `io.ReadAll`。

### 5.4 Workflow 节点执行无重试

**文件**: `internal/service/dag_workflow_service.go:610-612`

```go
result, err := entry.Handler.Execute(ctx, req)
if err != nil {
    _ = s.nodeRepo.UpdateNodeExecution(ctx, nodeExec.ID, model.NodeExecFailed, nil, err.Error())
    return err  // 立即失败，无重试
}
```

网络抖动等瞬时故障导致永久 workflow 失败。

**修复**: 添加可配置的重试策略（指数退避 + 最大重试次数）。

### 5.5 Plugin ToolChain 重试风暴

**文件**: `internal/plugin/toolchain_executor.go:94-102`

```go
for attempt := 0; attempt <= maxRetries; attempt++ {
    callResult, callErr = e.caller.CallMCPTool(ctx, pluginID, step.Tool, resolvedInput)
    if callErr == nil {
        break
    }
    if attempt < maxRetries {
        continue  // 立即重试，无延迟
    }
}
```

**修复**: 添加 exponential backoff + jitter：

```go
if attempt < maxRetries {
    time.Sleep(time.Duration(100*(1<<attempt)) * time.Millisecond)
    continue
}
```

### 5.6 Budget 阈值检测竞态

**文件**: `internal/service/agent_service.go:695-746`

```go
previousRatio := task.SpentUsd / task.BudgetUsd
currentRatio := updatedTask.SpentUsd / updatedTask.BudgetUsd
if previousRatio < 0.8 && currentRatio >= 0.8 && currentRatio < 1 {
```

快速连续的 cost update burst 可能跳过 80% 阈值。Bridge Cancel 失败被忽略（L722），agent 继续消耗预算。

### 5.7 IM 控制面无 Shutdown

**文件**: `internal/service/im_control_plane.go:82`

```go
type IMControlPlane struct {
    mu sync.Mutex  // 注释承认应迁移到 RWMutex
    // 无显式 shutdown/cleanup 方法
}
```

Map 数据永不释放，长期运行可能内存泄漏。

### 5.8 硬编码值

| 位置 | 值 | 建议 |
|------|-----|------|
| `agent_service.go:1524` | MaxTurns = 50 | 移至配置 |
| `dashboard_widget_service.go:74` | Cache TTL = 60s | 移至配置 |
| `ws/handler.go:20` | maxMessageSize = 4096 | 移至配置 |

---

## 6. P3 — 低优先级 / 技术债务

| # | 问题 | 位置 |
|---|------|------|
| 1 | Handler 错误响应格式不统一（`map[string]string` vs `model.ErrorResponse`） | 多个 handler |
| 2 | Query param 无边界校验（page/limit） | `automation_handler.go:151-158` |
| 3 | 项目创建清理错误用 `_` 忽略 | `project_handler.go:208-217` |
| 4 | Secrets 端点在创建响应中回传明文值 | `secrets_handler.go:113-116` |
| 5 | `context.Background()` 绕过父 context 取消（PoolStats 等） | `agent_service.go:895,910,927-948` |
| 6 | Dispatch attempt 记录错误被忽略 | `agent_service.go:1860`, `task_dispatch_service.go:391-412` |
| 7 | Trigger dryrun 不验证目标存在 | `trigger/dryrun.go:31-56` |
| 8 | Idempotency store 在 Redis 不可用时行为不一致 | `trigger/idempotency.go:44-73` |
| 9 | Handler 中类型断言无 nil 检查 | `knowledge_asset_handler.go:641-655`, `role_handler.go:697-719` |
| 10 | Bridge API handler cache 双重检查锁定模式有微小竞态窗口 | `bridge_api_handler.go:60-76` |
| 11 | Secret 值在创建响应中回传（虽然有文档说明，但仍是安全风险） | `secrets_handler.go:113-116` |
| 12 | ChangePassword 不使现有 token 失效 | `auth_service.go:245-267` |

---

## 7. agent_service.go 逐函数分析

### 结构体与初始化（L1-238）

**`AgentService` 结构体（L150-181）**: 20+ 接口依赖通过 setter 注入，无构造时校验。`NewAgentService`（L211-238）只初始化 5 个 map，其余全部为 nil。

| Setter | 调用方风险 |
|--------|-----------|
| `SetPool` | L440, L1365 直接 `s.pool.Acquire/Release` — nil panic |
| `SetBridgeHealth` | L272 `s.bridgeHealth.Status()` — nil panic（但有 nil guard） |
| `SetQueueStore` | L954, L961 直接访问 — nil panic（但有 nil guard） |
| `SetTeamService` | L583, L597 调用 `ProcessRunCompletion` — nil panic（但有 nil guard） |
| `SetMemoryService` | L1648 `s.memorySvc.InjectContext` — nil panic（但有 nil guard） |
| `SetPluginCatalog` | L1948 `ListDependencyPlugins` — nil panic（但外部调用有 guard） |

**风险**: 如果 `cmd/server/main.go` 初始化顺序出错，运行时 panic。无编译期保障。

### `spawnWithContext`（L374-537）— 核心生成逻辑

**L375-383 — 活跃运行检查 TOCTOU**: 并发 Spawn 可能同时通过检查。参见 [4.3](#43-spawn-toctou-竞态)。

**L440-451 — Pool Acquire/Release**: Acquire 在 Create 之前。`failSpawn`（L1262-1287）的清理错误全部用 `_` 忽略，可能导致 pool slot 泄漏。

**L536 — 后台验证**: `verifySpawnStarted` 使用 `context.Background()` + 5s timeout，正确。无 panic recovery。

### `UpdateStatus`（L541-631）— 状态转换核心

**L549-559 — 状态机不完整**: 从 `BudgetExceeded` 恢复到 `Running` 允许（L556），但不重新检查 budget 是否仍然超限。

**L584-617 — 两个 goroutine 无超时**: 参见 [4.1](#41-agent_servicego-后台-goroutine-无超时)。

**L618 — promoteQueuedAdmission 同步调用**: 含 bridge 网络调用，阻塞 UpdateStatus 返回。

### `UpdateCost`（L634-753）— 成本追踪

**L649-678 — N+1 查询**: 每次 UpdateCost 产生 3 次 DB 查询（run by ID, task by ID ×2, runs by task），task 查询重复。

**L695-746 — Budget 阈值检测竞态**: 参见 [5.6](#56-budget-阈值检测竞态)。

**L722 — Bridge Cancel 错误忽略**: agent 继续运行，消耗更多预算。

### `PoolStats`（L889-951）— O(n²) 复杂度

L895, L910, L927-948 使用 `context.Background()`。L927-936 对每个 active run 的 task 做 `GetByID`，O(activeRuns²) 复杂度。

### `ProcessBridgeEvent`（L2012-2287）— 280 行巨型 switch

**L2109 — StructuredOutput 更新错误忽略**: 结构化输出可能丢失，且无日志。

**L2139 — Retryable 错误静默丢弃**: `Retryable=true` 的错误只广播事件但不做任何处理，无重试机制。

### `findQueueEntry`（L1669-1692）— 性能问题

为找单个 entry 拉取最多 1000 条记录然后遍历。应使用直接查询。

### `waitForBridgeActivity`（L2451-2485）— Channel 管理

超时清理使用 `waiters[:0]` 在原 slice 上过滤。锁保护下安全，但代码风格不理想。

---

## 8. WebSocket 安全详析

### 用户 WebSocket `/ws`

**文件**: `internal/ws/handler.go`

| 漏洞 | 行号 | 详情 |
|------|------|------|
| CheckOrigin 全放行 | L26-28 | 任何源可建立 WS 连接 |
| Token 在 query param | L48 | JWT 泄漏到日志/历史/Referer |
| 无黑名单检查 | L60-71 | 已撤销 token 可建连 |
| 无项目成员校验 | L45, L86 | 可订阅任意项目事件 |
| 无连接数限制 | — | 单用户可建立大量连接 |

### Bridge WebSocket `/ws/bridge`

**文件**: `internal/ws/bridge_handler.go`

完全无认证。任意客户端可注入 bridge 事件。参见 [3.1](#31-bridge-websocket-无认证)。

### IM Control WebSocket `/ws/im-bridge`

**文件**: `internal/ws/im_control_handler.go`

完全无认证。参见 [3.2](#32-im-control-websocket-无认证)。

### WebSocket Hub

**文件**: `internal/ws/hub.go`

Hub 的 register/unregister 通过 channel 序列化，无竞态问题。客户端清理正确（close send channel）。

---

## 9. 认证流程与 Auth 服务

### JWT 完整流程

```
Register/Login → bcrypt hash → issueTokens (access + refresh JWT)
    ↓
Access Token (15m TTL, HS256, contains sub/email/jti)
    ↓ HTTP Request → JWTMiddleware → ParseWithClaims → Check blacklist → Set claims in context
    ↓
Refresh Token (168h TTL, stored in Redis)
    ↓ POST /auth/refresh → Parse JWT → Compare with Redis stored → Rotate both tokens
    ↓
Logout → Blacklist access JTI (Redis, TTL=accessTTL) → Delete refresh token (Redis)
```

### 关键发现

| 问题 | 详情 |
|------|------|
| WS 不检查黑名单 | 已撤销 token 可建立 WebSocket |
| 无审计日志 | Login/Register/ChangePassword/Refresh 均无日志 |
| ChangePassword 不使 token 失效 | 旧 access token 仍可用至自然过期 |
| Logout 非原子 | 先黑名单 JTI 再删 refresh token，中间失败导致不一致 |
| 无 rate limiting (service 层) | Login/Register 无暴力破解保护 |
| JWT 在 URL query param | WS 连接时 token 暴露 |

---

## 10. Workflow 引擎正确性

### DAG 拓扑 — `FindNodesBetween`

**文件**: `internal/workflow/nodetypes/topology.go:15-43`

BFS 实现实际正确。`visited` 在入队时检查 `!visited[next]`，在出队时设置。不会无限循环。之前初步报告有误，已更正。

### Loop 状态管理

**文件**: `internal/workflow/nodetypes/applier.go:380-418`

节点删除和计数器更新不在同一事务中。参见 [4.4](#44-loop-状态非原子更新)。

### Parallel Split/Join

**文件**: `internal/workflow/nodetypes/parallel_split.go`, `parallel_join.go`

节点处理器本身是空操作（no-op），实际协调逻辑在 `dag_workflow_service.go` 中。`executeNode`（dag_workflow_service.go:577-672）中的并行波执行使用 `sync.Mutex` + `sync.WaitGroup`，基本正确。

### Wait Event

**文件**: `internal/workflow/nodetypes/wait_event.go:17-56`

无超时机制。参见 [4.7](#47-wait-event-无超时清理)。

### Node 执行错误处理

**文件**: `internal/service/dag_workflow_service.go:610-612`

立即标记 failed，无重试。参见 [5.4](#54-workflow-节点执行无重试)。

---

## 11. Plugin 系统（WASM 沙箱）

### 资源限制

**文件**: `internal/plugin/runtime.go`

| 问题 | 行号 | 详情 |
|------|------|------|
| 无内存限制 | L200 | `wazero.NewRuntime(ctx)` 默认无限制 |
| 无 CPU 时间限制 | L275 | `InstantiateModule` 无超时 |
| 无文件系统隔离 | L201 | WASI preview1 无目录限制 |
| 编译缓存无上限 | L183-231 | 只有 `DeactivatePlugin` 会主动删除 |
| 实例重用 | L251 | 缓存的 runtime 在实例间共享，崩溃后可能状态损坏 |

### ToolChain Executor

**文件**: `internal/plugin/toolchain_executor.go:94-102`

重试无退避。参见 [5.5](#55-plugin-toolchain-重试风暴)。

---

## 12. Knowledge 引擎

### 文件大小限制

**文件**: `internal/knowledge/ingest_worker.go`

| 问题 | 行号 | 详情 |
|------|------|------|
| 无大小限制 | L77-81 | `blobs.Get` 无大小检查 |
| io.ReadAll | L132-133 | 整个文件加载到内存 |
| 删除错误忽略 | L96 | `_ = w.chunks.DeleteByAssetID`，旧 chunks 残留 |
| 失败不清理 | L123-127 | `failIngest` 不清理已创建的部分 chunks |

### 向量搜索

**文件**: `internal/knowledge/search_provider.go:34-70`

使用参数化查询（`plainto_tsquery('english', ?)`），无 SQL 注入风险。但复杂查询字符串可能引起性能问题。

---

## 13. Trigger 引擎

### Schedule Ticker

**文件**: `internal/trigger/schedule_ticker.go`

| 问题 | 行号 | 详情 |
|------|------|------|
| Check-then-act 竞态 | L140-193 | shouldFire 和 dispatchOne 之间锁释放 |
| 失败不重试 | L187 | Route 失败后仍更新 lastFire |
| Binding checker 无错误处理 | L159-163 | IsBindingActive 失败时静默跳过 |

### Idempotency Store

**文件**: `internal/trigger/idempotency.go:44-73`

Redis 不可用时有两种行为：
- `nil` client → 返回 error
- `NoopIdempotencyStore` → 始终返回 `false`（允许重复执行）

不一致，可能导致重复 trigger 执行。

### Dryrun

**文件**: `internal/trigger/dryrun.go:31-56`

只测试 filter 匹配和 template 渲染，不验证目标 workflow/plugin 是否存在。dryrun 通过的 trigger 可能在运行时失败。

---

## 14. 并发模式汇总

### Goroutine 泄漏风险

| 位置 | 模式 | 问题 |
|------|------|------|
| `agent_service.go:584` | `go func()` | 无 timeout，无 panic recovery，无 WaitGroup |
| `agent_service.go:602` | `go func()` | 同上 |
| `agent_service.go:536` | `go func()` | verifySpawnStarted 有 timeout 但无 panic recovery |

### Context.Background() 使用

| 位置 | 应使用父 context? | 原因 |
|------|-------------------|------|
| `agent_service.go:584,602` | 应继承 + 添加 timeout | 后台处理不应无限运行 |
| `agent_service.go:895,910,927-948` | 可保留 | PoolStats 是统计查询，独立于请求 |
| `agent_service.go:1948` | 应使用传入 ctx | resolveRoleConfig 中的 plugin 列表查询 |
| `plugin/runtime.go:200` | 已使用传入 ctx | 正确 |

### Mutex 使用

| 位置 | 锁类型 | 评价 |
|------|--------|------|
| `agent_service.go:170-174` | `sync.Mutex` | bridgeLastActivity + bridgeActivityWaiters 共享，活动通知可能阻塞等待者清理 |
| `agent_service.go:173` | `sync.Mutex` | lastBudgetAlertByRun，读多写少，应考虑 RWMutex |
| `im_control_plane.go:82` | `sync.Mutex` | 代码注释承认应迁移到 RWMutex |
| `plugin/runtime.go:17` | `sync.RWMutex` | 正确使用 |
| `plugin_registry.go:17` | `sync.RWMutex` | 正确使用 |

---

## 15. 各层质量评价

### Repository 层 — 优秀

- 全部使用参数化查询，无 SQL 注入
- 所有 `*Rows` 使用 `defer rows.Close()` 正确关闭
- 错误统一使用 `fmt.Errorf("operation: %w", err)` 包装
- 模型使用指针类型正确处理可空字段
- 事务使用合理（`CreateWithOwner`, `SetDefault`, `Upsert`）

### Model 层 — 良好

- 字段类型与 SQL 类型匹配
- DTO 转换方法完整（`ToDTO()`）
- 枚举值使用字符串常量

### Service 层 — 中等

- 业务逻辑基本正确
- 结构化日志广泛使用
- 但存在 god object、静默错误、goroutine 泄漏

### Handler 层 — 中等偏下

- 输入验证不完整
- 错误响应格式不统一
- 部分端点暴露内部错误信息

### Middleware 层 — 良好

- RBAC 矩阵完善（`middleware/rbac.go`）
- JWT 验证正确
- Auth → Project → RBAC → ArchivedGuard 顺序正确

### WebSocket 层 — 差

- 3 个端点中 2 个完全无认证
- CheckOrigin 全放行
- JWT 黑名单不检查
- 无连接数限制

---

## 16. 修复优先级与建议

### 立即修复（本周）

| # | 问题 | 影响 | 工作量 |
|---|------|------|--------|
| 1 | Bridge WS 添加认证 | 防止内部事件注入 | 0.5d |
| 2 | IM Control WS 添加认证 | 防止 IM 消息窃听 | 0.5d |
| 3 | 用户 WS 添加黑名单检查 | 使 logout 真正生效 | 2h |
| 4 | CheckOrigin 实现真正 CORS | 防止 CSRF | 2h |
| 5 | 内部端点添加共享密钥 | 防止未授权内部操作 | 0.5d |

### 短期修复（2 周内）

| # | 问题 | 影响 | 工作量 |
|---|------|------|--------|
| 6 | 后台 goroutine 添加 timeout + panic recovery | 防止 goroutine 泄漏 | 1d |
| 7 | UpdateCost budget 阈值检测添加互斥 | 防止跳过 budget 阈值 | 4h |
| 8 | Spawn 活跃检查使用 advisory lock | 防止并发 Spawn | 4h |
| 9 | applyResetNodes 使用事务 | 防止 workflow 卡死 | 4h |
| 10 | WASM 添加内存/时间限制 | 防止恶意插件 DoS | 1d |
| 11 | Knowledge ingest 添加文件大小限制 | 防止 OOM | 4h |
| 12 | 统一事件发布错误处理 | 改善可观测性 | 1d |
| 13 | Auth 服务添加审计日志 | 安全合规 | 1d |

### 中期改进（1 月内）

| # | 问题 | 影响 | 工作量 |
|---|------|------|--------|
| 14 | 拆分 agent_service.go | 可维护性 | 3-5d |
| 15 | Plugin toolchain 添加 exponential backoff | 防止重试风暴 | 4h |
| 16 | Trigger schedule ticker 修复竞态 | 防止重复触发 | 4h |
| 17 | Workflow 节点添加重试机制 | 容错性 | 2d |
| 18 | Wait event 添加超时清理 | 防止 workflow 永久挂起 | 1d |
| 19 | Token 比较使用 constant-time | 防止时序攻击 | 2h |
| 20 | ChangePassword 黑名单现有 token | 安全加固 | 4h |
