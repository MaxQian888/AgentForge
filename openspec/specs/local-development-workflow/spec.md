# local-development-workflow Specification

## Purpose
Define the supported root-level local web development workflow for AgentForge so developers can start, inspect, diagnose, and stop the repo-truthful frontend, backend, bridge, and local infrastructure stack without relying on ad-hoc multi-terminal steps.
## Requirements
### Requirement: Repository exposes a unified full-stack local development startup command
The repository SHALL provide a supported root-level full-stack development startup command that prepares the AgentForge local web development surface in one flow. The command MUST cover the repo-truthful frontend dev server, Go Orchestrator, TS Bridge, IM Bridge, and the local PostgreSQL/Redis infrastructure needed by the default development path, and MUST print the resulting service endpoints when startup succeeds.

#### Scenario: Full stack starts from a clean local environment
- **WHEN** a developer runs the supported full-stack startup command on a machine where the required toolchain and Docker runtime are available
- **THEN** the repository starts or prepares PostgreSQL, Redis, the Go Orchestrator, the TS Bridge, the IM Bridge, and the frontend dev server in a valid order
- **AND** it prints the reachable endpoints for each managed development service

#### Scenario: Startup reuses an already healthy service
- **WHEN** the supported full-stack startup command detects that one or more required services are already healthy on their expected local endpoints
- **THEN** the command reuses those services instead of starting duplicates
- **AND** it reports which services were reused, including the IM Bridge when its expected health surface is already available

#### Scenario: Managed IM Bridge startup uses repo-supported local defaults
- **WHEN** the full-stack startup workflow launches a managed IM Bridge process
- **THEN** it provides the backend connectivity, stable bridge identity path, and local notify or test port configuration required by the repo-supported local debug path
- **AND** the developer does not need to hand-edit a second startup command just to make the IM Bridge eligible for local control-plane registration

### Requirement: Full-stack development startup is health-aware and idempotent
The supported startup workflow SHALL validate service readiness before reporting success. The workflow MUST use repo-supported health probes or equivalent readiness checks for each service, including the IM Bridge health surface, MUST fail fast when a required dependency cannot be started or verified, and MUST distinguish missing prerequisites from unhealthy subprocesses or unknown external listeners.

#### Scenario: Startup waits for service readiness
- **WHEN** the startup workflow launches a managed frontend, Go, TS Bridge, or IM Bridge process
- **THEN** the command waits for the configured readiness checks to pass before marking that service ready

#### Scenario: Startup reports a missing prerequisite
- **WHEN** a required local dependency such as Docker, Go, Bun, Node.js, or pnpm is unavailable for a service that cannot be reused
- **THEN** the startup command exits non-zero and identifies the missing prerequisite and the affected service

#### Scenario: Startup reports an unknown listener conflict
- **WHEN** a required port is already occupied but the listener does not satisfy the expected health probe for the corresponding AgentForge service
- **THEN** the startup command exits non-zero and reports the port conflict as an external or unknown listener instead of assuming the service is healthy

### Requirement: Repository exposes status and stop commands for the managed local stack
The repository SHALL provide supported root-level status and stop commands for the same full-stack local development surface. The status command MUST report the current source, health, pid metadata when available, and log locations for each known service, including IM Bridge. The stop command MUST terminate only services previously marked as managed by the workflow and MUST leave reused or external services untouched.

#### Scenario: Status reports managed and reused services distinctly
- **WHEN** a developer runs the supported status command after a previous startup flow
- **THEN** the command reports each known service with its current health, whether it is managed or reused, and any available pid, endpoint, and log metadata

#### Scenario: Stop terminates only managed services
- **WHEN** a developer runs the supported stop command after the startup workflow managed some services and reused others
- **THEN** the command stops only the managed services, preserves the reused services, and reports which services were stopped versus preserved

### Requirement: Repository persists runtime metadata and diagnostics for the local stack
The supported local development workflow SHALL persist repo-local runtime metadata and diagnostic outputs needed by follow-up status and troubleshooting commands. The workflow MUST store per-service log locations and enough state to reconcile stale processes, including the managed IM Bridge process, and MUST direct developers to the relevant log or status surface when startup or health verification fails.

#### Scenario: Managed startup records runtime metadata and logs
- **WHEN** the supported startup workflow launches or reuses services for the local stack
- **THEN** it records repo-local runtime metadata for those services and ensures each managed service has a discoverable log location

#### Scenario: Status reconciles stale runtime state
- **WHEN** the stored runtime metadata references a process id or service state that no longer matches the current machine state
- **THEN** the status workflow detects the stale state, updates the reported status accordingly, and avoids treating the stale record as a healthy managed service

#### Scenario: Failure output points to diagnostics
- **WHEN** startup or readiness verification fails for a managed service
- **THEN** the workflow exits non-zero and reports the failing service, the reason category, and the log or status location a developer should inspect next

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

