## Context

当前仓库已经把桌面模式定义成一条独立能力线，但 Rust/Tauri 调试入口仍然只有“完整桌面启动”这一条：

- `src-tauri/src/main.rs` 只有默认桌面二进制入口，直接调用 `agentforge_desktop_lib::run()`；`src-tauri/Cargo.toml` 目前也没有额外的 CLI binary 目标。
- `src-tauri/src/lib.rs` 同时承载 Tauri builder、tray/menu、invoke handler、以及 `DesktopRuntimeManager` 的 sidecar 启动/健康检查/恢复逻辑，说明当前 Rust 运行时核心还没有被单独抽成可复用的调试宿主。
- `src-tauri/tauri.conf.json` 的 `beforeDevCommand` 仍然是 `pnpm desktop:dev:prepare && pnpm dev`，VS Code 的 `desktop:debug:prepare` 也会先走前端 guard；也就是说当前受支持的桌面调试路径默认都会把前端 boot ownership 一起带上。
- 仓库已经有 `pnpm dev:backend` 这条 Go/Bridge/IM Bridge 的后端调试栈，但没有与之对应的“前端已就绪时单独启动 Rust/Tauri 运行时”的官方入口。

这次 change 的目标不是重写桌面壳，也不是把 `pnpm tauri:dev` 废掉；而是在现有桌面工作流之外，补一条 repo-truthful 的 Rust 独立调试路径，让 `src-tauri` 的启动、sidecar 监督和端口/前端前置问题可以被单独验证。

## Goals / Non-Goals

**Goals:**

- 为 `src-tauri` 增加受支持的独立调试模式，在前端 dev server 或已准备好的前端入口存在时单独启动 Rust 桌面运行时。
- 提供一个对应的 Rust CLI 工具，覆盖前置检查与前台启动两类开发者动作。
- 让完整桌面工作流和独立 Rust 调试工作流共享同一套 current-host sidecar prepare contract，避免前置步骤漂移。
- 对齐根级脚本、VS Code 桌面调试入口与开发文档，使两条桌面调试路径的边界、前置条件和报错语义一致。

**Non-Goals:**

- 不替换或弱化现有 `pnpm tauri:dev`；完整桌面开发模式仍然保留。
- 不把独立 Rust 调试模式扩展成新的后台守护进程编排器，也不复刻 `dev:all`/`dev:backend` 的 detached state/log ownership 模式。
- 不改动 backend / TS bridge / IM Bridge 的业务协议、默认端口或桌面 ready/degraded 语义。
- 不在本次 change 中引入一个永久 headless 的桌面服务产品形态；这里的重点是调试路径，而不是新增业务运行模式。

## Decisions

### 1. 新增专用的 Rust CLI binary，而不是只做一个 Node 包装脚本

首版会在 `src-tauri` 包内增加一个额外的 CLI binary（例如 `agentforge-desktop-cli`），负责 `run` / `check` 这类开发者命令；根级 `package.json` 只暴露轻量 wrapper，例如：

- `pnpm desktop:standalone:check`
- `pnpm desktop:standalone:dev`

这样做的原因是：

- 前置检查（frontend reachability、sidecar binary 是否存在、端口冲突、运行时共享约束）本质上属于 Rust 桌面运行时自己的 truth，放在 Rust 侧最不容易和实际启动路径漂移。
- Cargo 官方支持一个包同时声明多个 `[[bin]]` 目标，这让我们可以保留当前 GUI 入口，同时新增专用 CLI，不需要把调试逻辑塞进默认 `main.rs`。
- 根级脚本仍然可以提供仓库一致的调用体验，但不必再承载桌面运行时语义本身。

备选方案：

- 只增加一个 Node 脚本 `scripts/*.js` 去 `spawn cargo run` 并手工做检查。问题是桌面前置语义会和 Rust 真实启动路径形成双份实现。
- 用环境变量或命令行参数复用当前 `main.rs` 做“隐式 standalone 模式”。问题是默认 GUI 入口会变得更难理解，IDE debug 配置也会更难保持清晰。

### 2. 把桌面运行时宿主从 Tauri builder 壳层里抽出来，让 GUI 与 CLI 共享同一套 supervision core

当前 `src-tauri/src/lib.rs` 里，`DesktopRuntimeManager` 的 sidecar 启动、健康检查、冲突检测和 shutdown 逻辑与 Tauri menu/tray/invoke wiring 混在一个文件里。为了支持独立 CLI 而不复制逻辑，首版会把“运行时宿主”抽成共享模块，覆盖：

- backend / bridge / IM bridge 的启动顺序与 health 检查
- 端口冲突和 sidecar binary 可用性检查
- ready / degraded / stopped 风格的状态计算
- 进程 shutdown 与失败上下文归档

GUI `run()` 和新的 CLI `run` / `check` 都复用这套 core；Tauri builder 仍然保留窗口、tray、invoke、menu 等桌面 UI 专属 wiring。

备选方案：

- 在 CLI 里复制一份简化版 sidecar 管理代码。问题是未来任何端口、健康探针或恢复策略调整都会产生漂移。
- 继续把所有逻辑留在 `lib.rs`，让 CLI 通过 feature flag 或条件分支绕进去。问题是文件复杂度和职责耦合会继续上升，后续维护更差。

### 3. 独立 Rust 调试模式默认复用“已存在的前端入口”，而不是继续接管前端启动

独立调试模式的核心价值就是把 Rust 这一层从前端 boot 链路里拆出来。因此首版 contract 约定：

- `desktop:standalone:dev` 或 CLI `run` 在启动前先检查 configured frontend surface；开发模式默认检查 `tauri.conf.json` 当前的 `devUrl`。
- 如果使用者明确选择静态产物模式，CLI 也可以检查 `frontendDist` 是否存在；但它不会负责执行 `pnpm build`。
- 缺少 frontend surface 时，CLI 直接 fail-fast，并提示应该先跑哪条受支持命令（例如 `pnpm dev` 或其它明确的前置步骤），而不是偷偷帮用户启动新的前端进程。

这样可以保证“Rust 独立调试”是真正的单独 ownership，而不是另一条隐形的 all-in-one 命令。

备选方案：

- 让 standalone CLI 直接触发 `pnpm dev` 或复用 `beforeDevCommand`。问题是这会回到当前痛点：Rust 调试仍然被前端启动噪音绑住。
- 完全忽略 frontend preflight，交给 Tauri 在窗口打开时报错。问题是失败信息会变得含糊，调试效率差。

### 4. 独立 CLI 采用 foreground-first 的 `check` / `run` 子命令，而不是新增一整套 detached state/stop/logs 家族

这条能力的核心场景是“我要调试 Rust”，因此前台输出和真实退出码比 background workflow 更重要。首版 CLI 只承诺两类入口：

- `check`：执行前置检查并给出可执行诊断
- `run`：在前台启动 Rust 桌面运行时，并把启动进度、ready/degraded 过渡、异常退出原因直接打印到 stdout/stderr

这样既适合 `cargo run --bin ... -- run`，也适合 VS Code / LLDB 直接挂这条命令。它不需要新建 `.codex/desktop-*.json` 之类的 detached state 文件，也不需要复制 `dev-all` 的 stop/status 语义。

备选方案：

- 为桌面 standalone 模式再造一套 `status` / `stop` / `logs`。问题是 scope 会迅速膨胀，而且与“调试前台 Rust 进程”的主要价值不匹配。
- 只提供 `run`，没有 `check`。问题是前置失败仍然会被压成一次模糊的启动失败，排障体验不好。

### 5. IDE 和根级脚本明确区分“完整桌面模式”与“独立 Rust 模式”

当前仓库里完整桌面模式已经有 `pnpm tauri:dev`、`desktop:dev:prepare`、`desktop:debug:prepare` 这一组入口。首版会把双路径边界讲清楚：

- `pnpm tauri:dev` 继续代表“完整桌面开发模式”，保留 `beforeDevCommand` 对前端 boot 的 ownership。
- `pnpm desktop:standalone:check` / `pnpm desktop:standalone:dev` 代表“Rust 独立调试模式”，默认假定前端已由外部命令或 IDE 单独准备好。
- VS Code 新增或调整单独的 Rust desktop debug 配置，让它可以直接走 standalone CLI/二进制，而不是总是依赖完整桌面 preLaunchTask。

这样可以避免未来出现“CLI 一套、IDE 一套、文档一套”三份说法。

备选方案：

- 只加 CLI，不改 IDE。问题是维护中的 VS Code 调试入口仍然会强绑前端启动，仓库真相继续分裂。
- 只加 IDE launch，不提供根级脚本。问题是仓库不会有 CLI 级 source of truth，自动化和文档都难以复用。

## Risks / Trade-offs

- [从 `lib.rs` 抽共享 runtime host 时，可能会影响现有桌面壳行为] → 用 focused Rust tests 覆盖现有 sidecar startup/health/restart contract，确保 GUI 入口和 CLI 共用逻辑后仍保持同一语义。
- [standalone 模式默认只检查现有 frontend surface，首次使用者可能误以为它会自动拉起前端] → 在 CLI 帮助文本、README 和命令报错里明确写清“不会隐式启动前端”。
- [新增 CLI binary 后，Cargo/Tauri 调试入口可能出现默认二进制选择歧义] → 保持现有 GUI binary 作为默认桌面入口；VS Code 和根级脚本显式指定 standalone CLI binary 名称。
- [如果 `check` 和 `run` 的前置逻辑写成两份实现，未来会继续漂移] → 把前置检查抽成共享函数，`run` 在真正启动前复用同一套 preflight 结果。

## Migration Plan

1. 在 `src-tauri` 内拆分共享 runtime host / preflight 逻辑，并新增 standalone CLI binary 与对应 Rust tests。
2. 在根级 `package.json`、`.vscode/tasks.json`、`.vscode/launch.json` 中增加受支持的 standalone Rust 调试入口，同时保留现有 `tauri:dev` 路径。
3. 更新 `README.md`、`README_zh.md` 与桌面调试文档，说明两条桌面工作流的前置条件与适用场景。
4. 运行 focused Rust/JS 验证，确保完整桌面模式未回归、standalone CLI 可被构建和调用。

回滚策略：

- 如果 standalone CLI 或 runtime host 抽离不稳定，可以先移除新增 CLI binary、根级 wrapper 和 IDE standalone launch，保留当前 `pnpm tauri:dev` 作为唯一受支持的桌面入口。
- 如果共享 runtime host 引入行为漂移，可以把 Rust CLI 暂时降级为只做 preflight，不接管真实运行，直到共享宿主抽离稳定为止。

## Open Questions

- 首版是否需要正式支持“静态 `frontendDist` 调试模式”，还是先把 contract 锁定在 `devUrl` 已就绪的开发模式。当前建议是首版以 `devUrl` 为主，静态产物只保留为显式可扩展选项，不把它做成默认路径。
