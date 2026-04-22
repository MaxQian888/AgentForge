# AgentForge Go Orchestrator

AgentForge 的核心后端编排服务，基于 Go + Echo 框架构建，提供 HTTP API、WebSocket 实时通信、任务调度、插件控制平面、VCS 集成、知识库管理等全栈能力。

## 技术栈

- **语言**: Go 1.25+
- **Web 框架**: Echo v4
- **数据库**: PostgreSQL (主存) + Redis (缓存/令牌撤销)
- **ORM**: GORM
- **消息**: 内部 EventBus (Pub/Sub)
- **WebSocket**: gorilla/websocket
- **插件运行时**: WebAssembly (wazero)

## 目录结构

```
cmd/
  server/                     # 主服务入口
  backfill-trigger-source/    # Trigger 来源回填工具
  email-adapter/              # 邮件适配器
  generic-webhook-adapter/    # 通用 Webhook 适配器
  github-actions-adapter/     # GitHub Actions 适配器
  migrate-once/               # 一次性迁移脚本
  plugin-debugger/            # 插件调试器
  review-escalation-flow/     # 审阅升级流程
  standard-dev-flow/          # 标准开发流程
  task-delivery-flow/         # 任务交付流程
  ...
internal/
  handler/      # HTTP 处理器（REST API）
  service/      # 业务逻辑层
  repository/   # 数据访问层
  model/        # 领域模型
  middleware/   # 认证、CORS、限流等中间件
  ws/           # WebSocket Hub
  plugin/       # 插件控制平面
  scheduler/    # 定时任务调度
  role/         # 角色管理
  worktree/     # Git Worktree 管理
  cost/         # 成本追踪
  memory/       # 项目记忆
  pool/         # Agent 池管理
  trigger/      # 自动化触发引擎
  automation/   # 声明式自动化规则
  vcs/          # VCS 提供商注册表（GitHub/GitLab/Gitea）
  knowledge/    # 知识资产管理与向量搜索
  secrets/      # 项目级密钥存储
  employee/     # Agent 身份（员工）管理
  adsplatform/  # 广告投放平台集成（千川）
  queue/        # Agent 工作队列与优先级控制
  skills/       # 受治理的技能目录
  document/     # 文档管理
  eventbus/     # 内部事件总线
  instruction/  # Agent 指令/提示词管理
  storage/      # 对象存储抽象
  imcards/      # IM 富卡片格式化
  integration/  # 外部集成触发流测试
  version/      # 服务版本元数据
pkg/            # 公共包
migrations/     # 数据库迁移脚本
plugins/        # 内置插件示例
plugin-sdk-go/  # Go 插件 SDK
```

## 快速开始

```bash
# 直接运行（需 PostgreSQL + Redis）
go run ./cmd/server

# 构建当前平台
go build ./cmd/server

# 运行测试
go test ./...

# 数据库迁移（使用 golang-migrate）
migrate -path migrations -database "$POSTGRES_URL" up
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `POSTGRES_URL` | PostgreSQL 连接字符串 | - |
| `REDIS_URL` | Redis 连接字符串 | - |
| `JWT_SECRET` | JWT 签名密钥（生产环境必须设置） | - |
| `JWT_ACCESS_TTL` | Access Token 有效期 | `15m` |
| `JWT_REFRESH_TTL` | Refresh Token 有效期 | `168h` |
| `ALLOW_ORIGINS` | CORS 允许的源 | `http://localhost:3000` |
| `SERVER_PORT` | HTTP 服务端口 | `7777` |

> 完整配置请参考 `src-go/.env.example`。

## 关键模块说明

### 认证与授权
- JWT 双令牌机制（Access + Refresh）
- Refresh Token 黑名单（Redis  backed）
- 项目级 RBAC：owner / admin / editor / viewer

### 插件系统
- 支持 WebAssembly 运行时插件
- 插件 SDK 位于 `plugin-sdk-go/`
- 控制平面提供安装、卸载、重启、状态查询能力

### 触发引擎 (`trigger`)
- 支持 CRON、Webhook、事件订阅等多种触发源
- 幂等性保证与调度路由
- Dry-run 模式支持

### 知识库 (`knowledge`)
- 分块摄入（chunked ingestion）
- 向量搜索
- 实时工件物化（live-artifact materialization）

### VCS 集成 (`vcs`)
- 支持 GitHub、GitLab、Gitea
- Webhook 路由与事件处理
- 项目级 VCS 提供商连接管理
