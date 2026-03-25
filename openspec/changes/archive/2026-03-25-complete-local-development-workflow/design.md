## Context

`docs/PRD.md` 与 `docs/part/PLUGIN_SYSTEM_DESIGN.md` 都把 AgentForge 描述成由 Next.js 前端、Go Orchestrator、TS Bridge、以及 PostgreSQL/Redis 等基础设施组成的多进程本地开发体验，但当前仓库只把其中一部分脚本化了：

- 根级 `package.json` 只有 `dev`、`plugin:dev`、`tauri:dev` 等局部入口，没有面向“前端 + Go + TS Bridge + infra”的统一命令。
- `README.md` / `README_zh.md` 仍要求开发者手工执行 `docker compose up -d`、`cd src-go && go run ./cmd/server`、`cd src-bridge && bun run dev`、`pnpm dev` 等多终端步骤。
- `.vscode/tasks.json` 只在 UI 任务上做了端口级重复启动保护，仓库命令面没有对应的 CLI 语义。
- `scripts/run-plugin-dev-stack.js` 已经证明“前置依赖检查 + 健康探测 + 复用已有服务”的模式可行，但它只覆盖 Go Orchestrator 和 TS Bridge，不能满足全栈本地开发。
- 历史运行中已经沉淀出 `.codex/runtime-logs` 作为日志落点，但当前没有统一的 pid/state 元数据，也没有与 `status` / `stop` 命令配套。
- 本仓库当前不存在 `.env.local.example` 和 `src-go/.env.example`，所以开发工作流不能依赖“先复制示例 env 文件”作为唯一入口，必须以现有代码默认值和可选覆盖为准。

这个 change 的目标不是重做 Tauri sidecar 监督、插件 authoring 流程或云端部署，而是补齐一个 repo-truthful 的根级本地开发控制面，让当前真实架构可以被一条命令稳定拉起、检查、复用、停止和诊断。

## Goals / Non-Goals

**Goals:**

- 为 AgentForge 提供统一的根级本地开发命令面，至少覆盖 `dev:all`、`dev:all:status`、`dev:all:stop` 和日志/诊断入口。
- 把前端、Go Orchestrator、TS Bridge、PostgreSQL/Redis 的启动顺序、健康探测、重复启动保护和失败提示变成一致的脚本行为。
- 复用现有 `plugin:dev`、`.vscode/tasks.json` 端口守卫和 `.codex/runtime-logs` 约定，而不是引入另一套孤立流程。
- 让 `status` / `stop` 能区分“本次脚本管理的进程”和“外部已存在但被复用的进程”，避免误杀用户自己启动的服务。
- 让 README 与开发脚本对齐，确保开发者从文档、终端脚本和实际端口观测到的是同一套流程。

**Non-Goals:**

- 不把 `dev:all` 扩展成 Tauri 桌面模式的统一编排器；`tauri:dev` 仍然保持独立职责。
- 不替换 `plugin:dev` 的插件 authoring 最小栈；本次只在需要时复用其已有 helper 或行为模式。
- 不要求本次引入新的进程管理依赖（如 PM2、concurrently、foreman）。
- 不在本次中解决所有业务级运行失败，例如缺少 API Key、数据库 schema 逻辑错误或某个子系统的业务 bug。
- 不把 Docker 化的 backend/bridge 作为默认开发模式；本次优先保留当前 repo truth 的“本地 frontend + 本地 Go + 本地 Bun + compose infra”路径。

## Decisions

### 1. 使用单一 Node 控制脚本承载根级开发控制面

本地开发工作流将继续沿用根目录 `scripts/` 下的 Node 脚本模式，由根级 `package.json` 暴露 `dev:all` 及其配套命令。这样可以与现有 `build-backend.js`、`build-bridge.js`、`run-plugin-dev-stack.js` 保持一致，避免把跨平台参数解析、端口探测和日志管理拆成多套 PowerShell/Bash 逻辑。

备选方案：
- 仅用 `concurrently` 或 shell 脚本拼接多条命令。问题是很难做健康探测、复用已有服务和 Windows 下的 detached 进程治理。
- 只依赖 VS Code tasks。问题是 CLI、CI、终端用户都无法共享同一套语义。

### 2. 定义显式的命令族，而不是把所有行为塞进一个 `dev:all` flag

根级控制面将以一组清晰的命令暴露，而不是要求用户记忆大量 flags：

- `pnpm dev:all`：启动或复用全栈本地开发服务
- `pnpm dev:all:status`：输出每个服务的来源、pid、端口、健康状态、日志位置
- `pnpm dev:all:stop`：停止由脚本托管的服务，并保留外部复用服务
- `pnpm dev:all:logs`：查看或指向最新日志文件

这种命令面更接近当前仓库已经采用的 `plugin:dev` / `plugin:verify` 习惯，也更容易让 README 和故障提示引用固定命令。

备选方案：
- 只实现 `dev:all`。问题是仍然缺少停止、状态和诊断闭环。
- 只实现一个多 flag 脚本。问题是 discoverability 和文档清晰度更差。

### 3. 用“服务定义 + 健康探测 + 运行态元数据”模型统一管理所有本地服务

脚本会把前端、Go、TS Bridge、Postgres、Redis 视为同一类 service definition：每个服务声明启动命令、工作目录、端口、健康 URL、依赖关系和日志目标。运行态则记录到 repo-local 状态文件，例如 `.codex/dev-all-state.json`，内容至少包含：

- `service name`
- `managed` 或 `reused` 来源
- `pid`（若由脚本启动）
- `port` / `healthUrl`
- `logPath`
- `startedAt`
- `lastKnownStatus`

这会成为 `status`、`stop` 和失败诊断的唯一共享数据源；`.codex/runtime-logs` 继续作为 stdout/stderr 的固定落点。

备选方案：
- 只靠端口探测推断一切状态。问题是无法区分“脚本托管”与“外部复用”，也无法给出日志路径。
- 用临时 lock 文件。问题是表达不了多个服务的细粒度状态和 pid 元数据。

### 4. 基础设施与应用进程采用不同的启动/复用策略

`dev:all` 会把本地运行面拆成两层：

- 基础设施层：优先通过 `docker compose up -d` 保障 PostgreSQL/Redis 可用，并在必要时验证 compose 服务健康；如果端口已被外部实例占用，则以“外部复用/非托管”状态报告，而不是盲目覆盖。
- 应用层：前端、Go、TS Bridge 通过 detached 子进程启动，并用健康 URL 探测 readiness；若健康检查已通过，则直接复用，不重复启动。

这与当前 repo truth 一致：应用在本机源码目录运行，数据库和 Redis 主要通过 compose 提供。

备选方案：
- 全部容器化启动。问题是与当前 README 和实际开发路径不一致。
- 所有服务都只做端口探测，不主动拉起 infra。问题是第一次运行体验仍然不完整。

### 5. 健康检查语义以 repo 现有端点为准，并显式处理缺失示例 env 文件

就绪判断将复用当前仓库已有端点：

- frontend: `http://127.0.0.1:3000`
- Go: `http://127.0.0.1:7777/health` 与需要时的 `http://127.0.0.1:7777/api/v1/health`
- TS Bridge: `http://127.0.0.1:7778/health`（兼容 `/bridge/health`）

脚本不会把 `.env.local.example` 或 `src-go/.env.example` 作为硬前置条件，因为这两个文件当前并不存在。相反，脚本将优先沿用代码默认值，并允许开发者通过已有 env 变量覆盖。

备选方案：
- 强制检查示例 env 文件。问题是会把一个当前仓库自己不满足的条件变成启动阻塞。
- 只用端口存活作为健康。问题是无法区分“端口打开但服务未 ready”的情况。

### 6. `stop` 仅终止“脚本托管”的进程，`status` 要把来源说清楚

为了避免误杀用户手动启动的服务，`dev:all:stop` 只会读取状态文件中标记为 `managed` 的 pid 并尝试终止；对 `reused` / `external` 服务只做提示，不主动 kill。`status` 则必须明确展示每个服务的来源与可停止性。

备选方案：
- 根据端口一律 kill。问题是极易终止不属于当前仓库的进程。
- 完全不提供 stop。问题是仓库会继续累积孤儿进程和端口冲突。

### 7. 验证与文档要把“完整开发闭环”作为合同，而不是只测脚本工具函数

这次 change 的测试面不应只覆盖 helper 函数，还要覆盖：

- 服务定义与状态文件读写
- 复用已有健康服务的幂等行为
- 缺失前置依赖/健康失败时的错误输出
- `status` / `stop` 对 managed vs reused 服务的差异语义
- README / README_zh 中公开命令与脚本支持面一致

备选方案：
- 只更新 README。问题是很快会再次与脚本漂移。
- 只做脚本单测，不覆盖命令合同。问题是无法保证最终 CLI 体验一致。

## Risks / Trade-offs

- [Windows detached 进程行为不稳定] → 复用已有 `plugin:dev` 中可工作的 `child_process.spawn` 模式，并通过状态文件与日志落盘减少“无输出假成功”。
- [端口已占用但并非 AgentForge 服务] → 在复用前增加健康探测与响应校验，无法确认时把状态标成 `external-unknown` 并给出人工处理提示。
- [Docker 不可用会让首次启动失败] → 在前置依赖检查里尽早暴露 Docker/Compose 不可用，并说明哪些服务可继续、哪些服务需要手动补齐。
- [状态文件可能陈旧] → `status` 每次都重新执行健康探测，发现 pid 不存在或健康失败时自动修正 stale state。
- [新增命令面会与未来更复杂的桌面/云开发流程交叉] → 在文档和规格里明确本次仅覆盖 repo-local full-stack web development，不把 Tauri 或远程部署混入首版。

## Migration Plan

1. 定义共享 service-definition 与 runtime-state helper，抽出日志目录、状态文件、pid/health 探测的公共逻辑。
2. 实现 `dev:all` 启动主流程，先补齐 infra 探测与 compose 启动，再统一 frontend/Go/TS Bridge 的启动与复用逻辑。
3. 实现 `dev:all:status`、`dev:all:stop`、`dev:all:logs`，并让它们共用同一份状态元数据。
4. 复查 `plugin:dev` 与 `.vscode/tasks.json` 的可复用部分，尽量下沉为共享 helper，避免两套健康探测与端口守卫继续分叉。
5. 更新 README / README_zh 与开发说明，补上支持矩阵、端口、日志目录、已知限制和缺失 env example 的真实说明。
6. 补齐 focused tests，并用最终 `openspec status` 与命令存在性检查确认变更 apply-ready。

回滚策略：

- 新增命令以增量方式加入 `package.json`，不替换现有 `dev`、`plugin:dev`、`tauri:dev`。若新工作流不稳定，可先移除 `dev:all*` 暴露面而不影响既有开发路径。
- 日志与状态文件都放在 repo-local `.codex/`，回滚时删除相关脚本和状态文件即可，不涉及业务数据迁移。

## Open Questions

- `dev:all:logs` 首版是否只输出各服务最新日志文件路径，还是直接支持 tail/follow 模式。当前建议先保证路径与最近日志定位，再把交互式 tail 作为后续增强。
