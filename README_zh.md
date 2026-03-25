# AgentForge

AgentForge 是一个 Agent 驱动的开发管理平台，目标是把完整交付链路串起来：

`IM 收需求 -> AI 分解任务 -> Agent 执行编码 -> 自动审查 -> 交付落地`

根据最新项目文档，AgentForge 想做的不只是“代码助手”，而是让 AI Agent 成为真正可管理、可协作、可追踪成本与质量的团队成员。

[English Documentation](./README.md)

## 这个仓库现在包含什么

这个仓库已经不再只是一个通用 starter，而是一个持续演进中的 AgentForge 工作区，目前包含：

- `app/` 中的 Next.js 16 + React 19 Dashboard 与认证界面
- `src-go/` 中的 Go 后端基础实现
- `src-bridge/` 中的 TypeScript/Bun Agent Bridge 服务
- `src-im-bridge/` 中的 IM Bridge fork 工作区
- `src-tauri/` 中的 Tauri 桌面壳
- `docs/` 下的产品、架构、插件、审查与技术设计文档

## 产品方向

按照最新 PRD，AgentForge 的目标是：

- 成为一个面向人机混合研发团队的开源开发管理平台
- 能从 IM 接收需求，自动拆解任务，并把任务分配给 Agent 或人工成员
- 内建审查流水线、预算控制、进度追踪与插件扩展能力
- 打通团队沟通、开发执行、审查自动化与交付流程

## 架构总览

当前文档对 AgentForge 的核心分层大致如下：

- `Web Dashboard`：Next.js 16 前端，负责任务、项目、Agent、角色、成本等视图
- `Go Orchestrator`：负责 API、任务生命周期、调度、worktree、审查协调与实时分发
- `TS Agent Bridge`：所有后端 AI 调用的统一入口，负责 Agent 执行与轻量 AI 分析
- `IM Bridge`：基于 cc-connect 改造，连接飞书、钉钉、Slack、Telegram、Discord 等渠道
- `Review Pipeline`：分层自动审查 + 人工审批的质量闭环
- `Data Layer`：PostgreSQL、Redis、WebSocket / 事件流等基础设施

## 当前仓库状态

这个仓库正从早期 starter 基础迁移到 AgentForge，这一点非常重要：

- 产品文档和架构文档已经统一使用 `AgentForge`
- 一些代码、包名、模块名仍保留 `react-quick-starter` 或 `react-go-quick-starter` 这类历史命名
- 仓库里已经有真实实现工作区，但产品设计推进速度快于部分运行面
- 如果不同文档之间有冲突，请优先以 [`docs/PRD.md`](./docs/PRD.md) 为准

一个典型例子是：PRD v2 已说明 Go 与 TS Bridge 的通信方向更新为 `HTTP + WebSocket`，而部分较早的分册设计文档仍保留 `gRPC` 方案描述。出现冲突时应以 PRD 为最新口径。

## 仓库结构

```text
AgentForge/
├── app/                 # Next.js App Router：认证 + Dashboard 路由
├── components/          # 共享 UI 组件
├── hooks/               # 前端 hooks
├── lib/                 # 前端工具与领域/Mock 辅助代码
├── src-go/              # Go 后端基础实现
├── src-bridge/          # TypeScript/Bun Agent Bridge 服务
├── src-im-bridge/       # IM Bridge fork 工作区
├── src-tauri/           # Tauri 桌面壳
├── docs/                # PRD、调研、架构、设计文档
├── openspec/            # OpenSpec 变更产物
├── roles/               # 角色定义及相关资产
└── scripts/             # 构建辅助脚本，如后端 sidecar 编译
```

当前已经存在的主要前端路由组：

- `app/(auth)`：登录、注册
- `app/(dashboard)`：dashboard、agents、projects、roles、cost 等页面

## 文档导航

如果你想先理解最新项目叙事，建议按下面顺序阅读：

- [`docs/PRD.md`](./docs/PRD.md)：统一后的产品需求文档，也是当前最高优先级说明
- [`docs/part/AGENT_ORCHESTRATION.md`](./docs/part/AGENT_ORCHESTRATION.md)：编排层、Bridge、Agent 池、worktree、执行模型
- [`docs/part/REVIEW_PIPELINE_DESIGN.md`](./docs/part/REVIEW_PIPELINE_DESIGN.md)：三层审查流水线设计
- [`docs/part/PLUGIN_SYSTEM_DESIGN.md`](./docs/part/PLUGIN_SYSTEM_DESIGN.md)：插件系统目标设计
- [`docs/part/PLUGIN_RESEARCH_TECH.md`](./docs/part/PLUGIN_RESEARCH_TECH.md)：插件运行时与沙箱技术调研
- [`docs/GO_WASM_PLUGIN_RUNTIME.md`](./docs/GO_WASM_PLUGIN_RUNTIME.md)：当前仓库内 Go WASM 插件运行时、SDK 与本地验证说明
- [`docs/part/PLUGIN_RESEARCH_PLATFORMS.md`](./docs/part/PLUGIN_RESEARCH_PLATFORMS.md)：主流平台扩展生态对比
- [`docs/part/TECHNICAL_CHALLENGES.md`](./docs/part/TECHNICAL_CHALLENGES.md)：关键技术挑战与应对路径
- [`docs/part/DATA_AND_REALTIME_DESIGN.md`](./docs/part/DATA_AND_REALTIME_DESIGN.md)：数据模型与实时系统设计
- [`docs/part/CC_CONNECT_REUSE_GUIDE.md`](./docs/part/CC_CONNECT_REUSE_GUIDE.md)：IM Bridge 的 fork 与复用策略

配套仓库文档：

- [`AGENTS.md`](./AGENTS.md)：仓库协作约定
- [`CONTRIBUTING.md`](./CONTRIBUTING.md)：贡献指南
- [`TESTING.md`](./TESTING.md)：测试说明
- [`CI_CD.md`](./CI_CD.md)：CI/CD 说明
- [`CHANGELOG.md`](./CHANGELOG.md)：变更记录

## 环境要求

- Node.js 20+
- pnpm
- Go 1.25+，用于 `src-go/`
- Bun，用于 `src-bridge/`
- Rust 1.77.2+，用于 Tauri 桌面开发
- Docker Desktop 或其他 Docker 环境，用于本地 PostgreSQL / Redis

## 快速开始

### 1. 前端 Dashboard

```bash
pnpm install
pnpm dev
```

这会启动 Next.js 开发环境。

常用根目录命令：

- `pnpm dev`
- `pnpm build`
- `pnpm start`
- `pnpm lint`
- `pnpm test`
- `pnpm test:coverage`
- `pnpm plugin:build -- --manifest plugins/integrations/feishu-adapter/manifest.yaml`
- `pnpm plugin:debug -- --manifest plugins/integrations/feishu-adapter/manifest.yaml --operation health`
- `pnpm plugin:dev`
- `pnpm plugin:verify -- --manifest plugins/integrations/feishu-adapter/manifest.yaml`

### 2. Go 后端

如果需要本地基础设施，先在仓库根目录启动：

```bash
docker compose up -d
```

然后运行 Go 服务：

```bash
cd src-go
go run ./cmd/server
```

常用后端命令：

- `go test ./...`
- `go build ./cmd/server`
- `docker build -t agentforge-server .`

### 鉴权与会话说明

当前鉴权链路已经按前后端统一契约收敛：

- 前端持久化的标准会话载荷为 `accessToken`、`refreshToken` 和 `user`。
- Dashboard 受保护路由不再只信任本地布尔值。应用启动或进入受保护区域时，会先调用 `GET /api/v1/users/me` 校验当前 access token；如果 access token 已失效且仍有 refresh token，则只会尝试一次 `POST /api/v1/auth/refresh`；恢复失败时会清空本地会话。
- Web 模式优先使用 `NEXT_PUBLIC_API_URL` 作为后端地址，默认回落到 `http://localhost:7777`；Tauri 模式优先通过原生命令 `get_backend_url` 获取地址，再回落到同一个默认值。
- `POST /api/v1/auth/refresh` 现在和登录、注册一起受认证限流保护。

如果需要本地覆盖后端鉴权配置，请在 `src-go/.env` 中设置环境变量。常见值如下：

```env
PORT=7777
ENV=development
JWT_SECRET=change-me-in-production-at-least-32-chars
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h
ALLOW_ORIGINS=http://localhost:3000,tauri://localhost,http://localhost:1420
REDIS_URL=redis://localhost:6379
```

安全语义说明：为了兼顾本地开发弹性与鉴权安全，PostgreSQL / Redis 仍可在进程启动时缺席，但凡依赖令牌撤销状态的鉴权路径都不会再静默降级。只要 Redis 或 token cache 不可用，refresh、logout 撤销写入，以及基于黑名单的受保护路由校验都会 fail closed，而不是假装成功。

### 3. TypeScript Agent Bridge

```bash
cd src-bridge
bun install
bun run dev
```

常用 Bridge 命令：

- `bun run dev`
- `bun run build`
- `bun run typecheck`

运行时说明：

- `/bridge/execute` 现在支持可选的 `runtime` 字段，可用值为 `claude_code`、`codex`、`opencode`。
- 如果省略 `runtime`，Bridge 会默认回退到 `claude_code`，并继续兼容旧的 provider 提示，如 `anthropic`、`codex`、`opencode`。
- `claude_code` 使用内置的 Claude 运行时适配器，要求配置 `ANTHROPIC_API_KEY`。
- `codex` 和 `opencode` 使用基于命令的运行时适配器。可通过 `CODEX_RUNTIME_COMMAND` 或 `OPENCODE_RUNTIME_COMMAND` 指向 `PATH` 上的可执行文件，或直接配置绝对路径。
- 命令型 runtime 需要从 `stdin` 读取一份 JSON 请求，并从 `stdout` 输出按行分隔的 JSON 事件。
- 命令型 runtime 会把这些事件归一化为 Bridge 的统一事件流：`assistant_text`、`tool_call`、`tool_result`、`usage`、`error`。

针对 Bridge runtime 层的聚焦验证命令：

- `bun test src/schemas.test.ts src/handlers/execute.test.ts src/runtime/registry.test.ts src/server.test.ts`
- `bun run typecheck`

根目录也提供了：

```bash
pnpm build:bridge
```

### 3.5 插件作者本地工作流

针对当前维护中的 Go WASM 样例插件，仓库现在提供了一条受支持的根级循环：

```bash
pnpm plugin:build -- --manifest plugins/integrations/feishu-adapter/manifest.yaml
pnpm plugin:debug -- --manifest plugins/integrations/feishu-adapter/manifest.yaml --operation health
pnpm plugin:verify -- --manifest plugins/integrations/feishu-adapter/manifest.yaml
```

说明：

- `plugin:build` 会从 manifest 解析受维护样例的产物路径；如果你在调别的 Go 宿主插件，也可以显式传 `--source` / `--output` 覆盖。
- `plugin:debug` 不会发明另一套开发协议，而是直接复用真实的 `AGENTFORGE_AUTORUN`、`AGENTFORGE_OPERATION`、`AGENTFORGE_CONFIG`、`AGENTFORGE_CAPABILITIES`、`AGENTFORGE_PAYLOAD` 合同。
- `plugin:verify` 目前只覆盖受维护样例的 smoke 路径，也就是 `build -> debug health`，它是聚焦验证，不替代更广的 Go/Bridge 测试。
- `plugin:dev` 是最小插件开发栈命令，只负责 Go Orchestrator 和 TS Bridge；如果服务已经健康会直接复用，并通过 `http://127.0.0.1:7777/health` 与 `http://127.0.0.1:7778/health` 报告就绪状态。

### 4. IM Bridge 工作区

```bash
cd src-im-bridge
go run ./cmd/bridge
```

常用 IM Bridge 命令：

- `go test ./...`
- `go build ./cmd/bridge`

### 5. 桌面模式

如果你在做桌面壳相关工作，可以运行：

```bash
pnpm tauri:dev
```

或者构建桌面产物：

```bash
pnpm tauri:build
```

当前 Tauri 桌面壳的能力契约如下：

- Tauri 现在会同时监督两个必需 sidecar：`http://127.0.0.1:7777` 上的 Go Orchestrator，以及 `http://127.0.0.1:7778` 上的 TS Bridge。
- 只有两个 sidecar 都通过健康检查后，桌面运行态才会被标记为 `ready`。异常退出会先做有界重启，超过阈值后才进入 `degraded`。
- 前端桌面能力统一收敛到 `lib/platform-runtime.ts` 与 `hooks/use-platform-capability.ts`。当前已接入后端地址解析、桌面运行态查询、原生文件选择、系统通知、tray 更新、全局快捷键注册、更新检查，以及只读 runtime summary。
- Web 模式下保留显式 fallback 语义：文件选择回退到浏览器 input，通知回退到 Web Notification API，tray 更新回退到页面标题更新，全局快捷键返回 `unsupported`，更新检查返回 `not_applicable`。
- 插件页中的桌面运行态面板只是增强层。插件清单和生命周期操作仍然走现有后端控制面，后端仍是唯一业务真相来源。

当前限制：

- 桌面事件流目前归一化了 runtime、tray、shortcut、notification 和 updater 事件，但它不替代后端插件业务数据。
- 更新检查目前只覆盖检测与事件上报，还没有在 Dashboard 中开放下载并安装流程。

## 根目录关键脚本

| 命令 | 作用 |
| --- | --- |
| `pnpm dev` | 启动 Next.js Web 应用 |
| `pnpm build` | 构建 Next.js 应用 |
| `pnpm start` | 启动构建后的 Next.js 应用 |
| `pnpm lint` | 运行 ESLint |
| `pnpm test` | 运行 Jest |
| `pnpm test:coverage` | 运行带覆盖率的 Jest |
| `pnpm build:backend` | 为 Tauri 交叉编译 Go sidecar |
| `pnpm build:backend:dev` | 仅为当前平台构建 Go sidecar |
| `pnpm build:plugin:wasm` | 构建 Go WASM 样例插件产物 |
| `pnpm plugin:build` | 按 manifest 构建受维护的 Go 宿主插件目标 |
| `pnpm plugin:debug` | 通过真实 runtime envelope 本地调试 Go WASM 插件 |
| `pnpm plugin:dev` | 启动或复用最小插件开发栈：Go Orchestrator + TS Bridge |
| `pnpm plugin:verify` | 运行受维护样例的 smoke 工作流：build -> debug health |
| `pnpm tauri:dev` | 构建后端 sidecar 并启动 Tauri 开发模式 |
| `pnpm tauri:build` | 构建桌面应用 |
| `pnpm build:bridge` | 安装并构建 TS/Bun Bridge |
| `pnpm build:desktop` | 构建 backend + bridge sidecar 并打包桌面应用 |

## 技术栈快照

- 前端：Next.js 16、React 19、TypeScript、Tailwind CSS v4、shadcn/ui、Zustand
- 后端：Go 1.25、Echo、PostgreSQL、Redis
- Bridge：Bun、TypeScript、Hono、WebSocket
- 桌面：Tauri 2
- 工具链：ESLint、Jest、OpenSpec、MCP 配置

## 使用说明

- 密钥与敏感配置应放在本地环境文件中，例如 `.env.local` 或各服务目录下的 `.env.example` 副本
- `src-tauri/` 应保持最小权限范围
- 仓库同时包含真实实现与设计阶段文档，不应默认认为所有文档中的模块都已经完全落地
- 如果你对项目意图有疑问，应优先看 PRD 和架构文档，而不是仍残留在部分包名/模块名里的旧 starter 表述

## License

[MIT](./LICENSE)
