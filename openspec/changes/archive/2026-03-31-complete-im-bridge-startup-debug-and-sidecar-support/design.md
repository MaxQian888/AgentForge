## Context

当前仓库的启动与调试链路在三个关键位置缺失 IM Bridge：

- 根级 `package.json` 已提供 `dev:all`、`tauri:dev`、`build:desktop` 等入口，但 `dev:all` 只管理 PostgreSQL、Redis、Go Orchestrator、TS Bridge 和前端；`tauri:dev` / `build:desktop` 也只构建 backend 与 TS bridge sidecar。
- `src-tauri/tauri.conf.json` 的 `externalBin` 当前只有 `binaries/server` 和 `binaries/bridge`，Rust 侧 `DesktopRuntimeManager` 也只跟踪 `backend` 与 `bridge` 两条 sidecar，因此桌面运行时快照、ready/degraded 状态和恢复逻辑都天然忽略 IM Bridge。
- `src-im-bridge/cmd/bridge/main.go` 已经把 IM Bridge 定义成需要 backend control plane、稳定 `bridge_id`、`/im/health`、`NOTIFY_PORT` / `TEST_PORT` 的真实运行时，但根级开发命令和 VS Code 桌面调试前置还没有把它纳入支持范围。

这次 change 不是重做 IM Bridge 的平台协议，也不是把所有开发流合并成一个巨型脚本；目标是补齐当前仓库已经承诺但尚未真正成立的“完整启动与调试命令 + 完整桌面 sidecar 拓扑”。

## Goals / Non-Goals

**Goals:**

- 让 `pnpm dev:all` 及其 `status` / `stop` / `logs` 把 IM Bridge 作为受支持的本地调试服务统一管理。
- 为桌面开发与打包补齐 IM Bridge 构建链路，使 `tauri:dev`、`build:desktop`、Tauri pre-command 与 VS Code 桌面调试入口不再遗漏 IM Bridge。
- 让 Tauri/Rust 运行时把 IM Bridge 视为必需 sidecar，补齐启动顺序、健康检查、事件、快照和故障恢复语义。
- 保持文档、命令输出、日志落点和调试入口与仓库真实行为一致。

**Non-Goals:**

- 不改写 IM Bridge 的 provider/live transport/control-plane 业务协议。
- 不把 `plugin:dev`、`dev:all`、`tauri:dev` 压成一条单一命令面；三者继续保留不同职责。
- 不把本地开发流改成全量容器化，也不要求新增 PM2、foreman 之类的进程管理依赖。
- 不在本次 change 中新增桌面专属的 IM UI 功能；这里只补齐运行与调试链路。

## Decisions

### 1. 继续沿用现有 Node dev-workflow 控制面，把 IM Bridge 加入 `dev:all` 服务定义

`dev:all` 已经有 repo-local state、日志、端口冲突保护、健康探测和 managed/reused 语义；扩展它比再造一个 IM 专用启动脚本更稳。IM Bridge 会作为新的 application service 纳入同一套 service definition，使用 `src-im-bridge` 目录、`go run ./cmd/bridge` 启动方式、`http://127.0.0.1:<NOTIFY_PORT>/im/health` 健康探针、repo-local 日志文件，以及稳定的 `IM_BRIDGE_ID_FILE` 路径。

首版默认策略：

- 维持 `IM_TRANSPORT_MODE=stub` 的本地调试默认值。
- `AGENTFORGE_API_BASE` 指向本地 Go backend。
- `NOTIFY_PORT` / `TEST_PORT` 使用仓库默认端口，允许环境变量覆盖。
- `IM_BRIDGE_ID_FILE` 指向 repo-local 持久路径，避免每次重启生成新的 bridge instance 身份。

备选方案：

- 继续要求开发者手工在 `src-im-bridge` 下单独启动。问题是文档与脚本会继续漂移，`status` / `stop` 也永远无法覆盖完整栈。
- 为 IM Bridge 单独增加另一套 root 命令。问题是又会复制一套 state/log/health 逻辑。

### 2. 新增独立的 IM Bridge 构建脚本，并把桌面 prepare 命令收敛成共享别名

当前 backend 和 TS bridge 都有独立 build helper，IM Bridge 没有；导致 `tauri:dev`、`build:desktop`、`beforeBuildCommand`、IDE 调试入口都很难保持一致。解决方案是增加与 `build-backend.js` / `build-bridge.js` 对齐的 `build-im-bridge` helper，并通过 root script alias 明确区分当前主机开发构建与桌面打包构建。

命令分层如下：

- `build:im-bridge` / `build:im-bridge:dev`
- `desktop:dev:prepare`：构建当前主机所需的 backend、TS bridge、IM Bridge sidecar
- `desktop:build:prepare`：构建打包所需的 backend、TS bridge、IM Bridge sidecar 并配合前端生产构建
- `tauri:dev` / `build:desktop` / Tauri pre-command / VS Code 桌面调试入口统一复用这些 alias

备选方案：

- 直接把 IM Bridge `go build` 内联进 `tauri:dev` 和 `build:desktop`。问题是 CLI、Tauri config、IDE task 会继续各写各的。
- 只更新 `tauri.conf.json`，不增加独立 helper。问题是当前 host-only 开发构建和全平台打包构建无法清晰区分。

### 3. 桌面 sidecar 拓扑升级为 “backend + TS bridge + IM Bridge”，并保持 backend-first 依赖

IM Bridge 依赖 backend control-plane，但不依赖 TS bridge，因此桌面启动顺序采用：

1. backend 先启动并完成健康检查；
2. backend ready 后，分别尝试启动 TS bridge 与 IM Bridge；
3. overall runtime 只有在三条 sidecar 都 ready 时才进入 ready；
4. 任一必需 sidecar degraded/stopped 都会把 overall 拉回 degraded。

Rust runtime 状态、事件与恢复逻辑将按三条 sidecar 独立建模，而不是把 IM Bridge 藏在 “bridge” 或 “overall” 的隐式状态里。这样前端 runtime 观察面和故障诊断才有真实边界。

备选方案：

- 把 IM Bridge 挂在 TS bridge 后面作为隐式子进程。问题是 sidecar 生命周期和错误来源会失真。
- 只把 IM Bridge 打包进 `externalBin`，但不纳入 Rust supervision。问题是桌面运行时快照和恢复语义仍然不完整。

### 4. 桌面调试入口必须复用与 CLI 相同的准备步骤，避免 IDE 漂移

当前 `.vscode/launch.json` 的 `Tauri Development Debug` 只依赖 `ui:dev-if-needed`，没有任何 IM Bridge build/prepare 语义。首版将用共享 prepare 命令修正 CLI 与 IDE 漂移：

- CLI `tauri:dev` 使用 `desktop:dev:prepare`
- `build:desktop` 与 `beforeBuildCommand` 使用 `desktop:build:prepare`
- VS Code 桌面调试 preLaunchTask 改为调用同一套 desktop prepare 入口，并继续复用前端 dev server guard

这样不要求开发者记住 “CLI 一套、IDE 一套、Tauri config 一套” 的三套命令链。

备选方案：

- 只修 CLI，不修 `.vscode`。问题是调试入口仍然会跑出“桌面命令支持 IM Bridge、IDE 调试不支持”的双重真相。

### 5. 运行时对外合同同步扩展到 `desktop-runtime-event-bridge`

桌面 sidecar 增加到三条后，`get_desktop_runtime_status` 和 runtime event payload 也必须把 IM Bridge 暴露为独立 runtime；否则前端仍然看不到 IM Bridge 的状态变化，排障边界会继续缺失。该能力仍然保持 additive projection，不引入任何新的桌面专属业务 mutation。

备选方案：

- 只改 `desktop-sidecar-supervision`，不改事件桥。问题是规范和前端观察面会继续把 IM Bridge 隐藏掉。

## Risks / Trade-offs

- [IM Bridge 当前主机构建或跨平台构建与 Tauri triple 命名不一致] -> 复用现有 backend/bridge target matrix 约定，并把输出命名统一到 `src-tauri/binaries/<label>-<triple>`。
- [IM Bridge 启动早于 backend 就绪会触发 control-plane 注册失败] -> 明确采用 backend-first 启动顺序；`dev:all` 和桌面 runtime 都只在 backend ready 后再拉起 IM Bridge。
- [桌面 runtime snapshot 扩展后，前端或测试仍按两条 sidecar 假设] -> 在同一 change 中同步修改 runtime event spec、Rust tests、以及依赖 snapshot 的前端契约测试。
- [多管理一个进程会增加 stop/reuse/conflict 复杂度] -> 继续使用现有 runtime-state 与 managed/reused 区分，不引入第二套进程治理逻辑。

## Migration Plan

1. 增加 IM Bridge build helper、root script alias 与相关测试。
2. 扩展 `dev:all` 服务定义、状态文件与日志约定，把 IM Bridge 纳入统一控制面。
3. 更新 `package.json`、`src-tauri/tauri.conf.json`、`.vscode/tasks.json`、`.vscode/launch.json`，让 CLI、Tauri config、IDE 调试复用相同的 prepare 命令。
4. 扩展 Rust desktop runtime manager、runtime snapshot/event bridge 和对应测试，使 IM Bridge 成为第三条必需 sidecar。
5. 更新 README / README_zh 与开发说明，明确 IM Bridge 已进入支持的启动与调试命令面。

回滚策略：

- 如果桌面 prepare 命令或 IM Bridge sidecar 集成不稳定，可以先移除新增 alias、`externalBin` 和 Rust sidecar wiring，恢复到当前两条 sidecar 的桌面模式。
- `dev:all` 的 IM Bridge 集成可单独回滚，不影响现有 frontend/Go/TS bridge 启动路径。

## Open Questions

- 首版是否需要额外提供 `desktop:status` 一类只面向 IDE/脚本的桌面准备状态命令。当前结论是先不新增，复用现有 `dev:all:status` 与桌面 runtime snapshot 即可。
