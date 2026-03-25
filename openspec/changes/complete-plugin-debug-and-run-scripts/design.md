## Context

`docs/PRD.md` 和 `docs/part/PLUGIN_SYSTEM_DESIGN.md` 都已经把插件开发者体验写成产品能力的一部分，包括 Go/TS 双 SDK、`create-plugin`、以及“运行 `npm run dev` 开始开发”这类明确的本地开发叙事。但当前仓库真相更窄：

- 根目录只有 `build:backend`、`build:bridge`、`build:plugin:wasm` 这类构建脚本，没有插件作者可直接复用的调试和运行脚本。
- `scripts/build-go-wasm-plugin.js` 目前硬编码 `./cmd/sample-wasm-plugin -> plugins/integrations/feishu-adapter/dist/feishu.wasm`，不支持按 manifest 或样例目标切换。
- `docs/GO_WASM_PLUGIN_RUNTIME.md` 只给出构建命令和几组测试命令，没有一个可复用的本地调试入口来复现 `AGENTFORGE_AUTORUN`、`AGENTFORGE_OPERATION`、`AGENTFORGE_CONFIG`、`AGENTFORGE_PAYLOAD` 这些真实运行时契约。
- 插件开发需要的最小宿主栈目前仍靠人工分别启动 `go run ./cmd/server` 和 `bun run dev`，缺少一个 repo-truthful 的统一入口和 readiness 输出。

这个 change 的目标不是重做整个 `complete-plugin-system-foundation`，而是把其中最阻塞实际使用的一段 DX 缺口单独落地：先让当前 SDK、样例插件和未来模板有一条真实可跑的本地 build/debug/run/verify 闭环。

## Goals / Non-Goals

**Goals:**
- 为维护中的插件样例和后续模板提供统一的根级脚本入口，而不是继续依赖文档里的手工步骤。
- 让 Go WASM 插件作者可以在不改脚本源码的前提下，按目标 manifest 或入口文件构建并调试插件。
- 提供一个面向插件开发的最小运行栈命令，统一报告 Go/TS 服务就绪状态、端口和失败原因。
- 把样例插件、脚本命令和文档说明纳入验证面，降低文档承诺与仓库真相继续漂移的风险。

**Non-Goals:**
- 不在本次中交付完整的 `@agentforge/create-plugin` 脚手架包。
- 不在本次中实现插件市场、远程 registry、签名校验或完整分发流程。
- 不在本次中重构插件运行时归属或更换 Go/TS 宿主架构。
- 不在本次中自动搭起全量桌面/Tauri/数据库联调链路；这里聚焦插件作者所需的最小开发栈。

## Decisions

### 1. 采用根级 `package.json` + Node 脚本作为唯一受支持的插件开发入口

插件开发脚本会继续放在根目录 `scripts/` 下，并由根级 `package.json` 暴露统一命令，而不是同时维护 Bash、PowerShell 和 README-only 手册流程。当前仓库已经用 Node 脚本承载 sidecar 构建逻辑，这条路径最符合现有模式，也最容易跨平台复用。

备选方案：
- 直接新增 `.ps1` / `.sh` 包装脚本。问题是会把参数解析、错误输出和跨平台维护拆成两套。
- 只写 README 命令，不新增脚本。问题是当前仓库已经证明这种方式会和真实行为漂移。

### 2. 所有插件构建与调试流程都走 “manifest-first” 解析

新脚本会优先接受 manifest 路径或插件标识，再从 manifest 解析运行时类型、入口、输出位置和必要元数据；只有在 manifest 不存在或显式覆盖时，才接受直接入口或输出参数。这样可以避免继续把 `dist/*.wasm`、`cmd/sample-wasm-plugin` 之类的路径硬编码进脚本源码。

备选方案：
- 继续只支持单一样例插件。问题是无法服务当前 SDK，也不利于未来模板验证。
- 全部靠 flags 传入入口和输出。问题是会让 manifest 与脚本参数重复维护，容易失配。

### 3. 调试脚本必须复用真实宿主 envelope，而不是另起一套开发协议

Go WASM 本地调试会直接复用现有 `AGENTFORGE_AUTORUN`、`AGENTFORGE_OPERATION`、`AGENTFORGE_CONFIG`、`AGENTFORGE_PAYLOAD` 契约，并输出结构化 stdout/stderr 与退出码摘要。这样调试脚本既能服务 SDK 作者，也能作为宿主运行时问题的最短复现路径。

备选方案：
- 为 debug 模式单独设计一套 CLI 参数协议。问题是开发态和真实运行态会再次分叉。
- 强制所有调试都走完整注册表安装 + 激活。问题是反馈回路太长，不适合 SDK/模板迭代。

### 4. 运行脚本聚焦 “插件开发最小栈”，不吞掉所有平台依赖

统一运行脚本只负责插件开发最常用的 Go Orchestrator + TS Bridge 组合，并提供健康检查、端口信息、缺失前置依赖提示，以及可复用正在运行服务的 attach 语义。数据库、Docker、Tauri 等更重依赖保持显式可选，不把一次插件调试变成全量系统启动。

备选方案：
- 默认启动整套桌面模式或 Docker 基础设施。问题是过重，而且对 Tool/Review/Go SDK 调试并非总是必要。
- 完全不做 run 脚本，只保留 build/debug。问题是插件开发仍缺一个 repo-truthful 的统一宿主入口。

### 5. 验证面以 “受维护样例 + 命令合同” 为核心

新增脚本必须进入自动化验证面，包括参数解析、manifest 解析、样例插件 smoke、以及文档中对外暴露的命令合同。验证目标不是全量插件生态，而是保证仓库现在维护的样例、脚本和文档能持续对齐。

备选方案：
- 只测脚本工具函数。问题是无法发现样例产物或文档命令已经失效。
- 只做端到端人工验证。问题是太脆弱，下一轮改动很容易再次漂移。

## Risks / Trade-offs

- [最小运行栈仍可能依赖外部服务状态] -> 运行脚本需要明确区分“缺数据库/Redis 但可继续开发”和“关键 sidecar 未就绪必须失败”的错误语义。
- [manifest-first 解析会暴露历史清单不一致问题] -> 在脚本里优先给出可操作的 validation 错误，而不是把失败留到编译或运行时。
- [新增根级命令可能和未来 `create-plugin` 职责重叠] -> 本次只定义 repo 内维护样例和模板所需的开发闭环，把发布级脚手架留给后续 change。
- [脚本数量增加会带来维护成本] -> 通过共享 helper、统一参数风格和脚本测试来压缩重复逻辑。

## Migration Plan

1. 先定义并实现共享的 manifest/target 解析辅助逻辑，确保 build/debug/run/verify 使用同一套输入模型。
2. 在不破坏现有 `pnpm build:plugin:wasm` 别名的前提下，扩展参数化构建能力并增加本地 debug 脚本。
3. 增加最小插件开发栈运行命令与就绪输出，优先支持 Go Orchestrator + TS Bridge 的 repo-truthful 组合。
4. 补齐脚本测试、样例 smoke 和文档更新，把 README 与 Go WASM runtime 文档切换到新命令。

回滚策略：
- 保留现有 `build:plugin:wasm` happy path 作为兼容入口；若参数化流程不稳定，可先退回默认样例目标而不影响当前样例构建。
- 新增 run/debug 命令采用增量方式，不替换已有手工命令；必要时可以先禁用新命令而不影响既有服务启动方式。

## Open Questions

- 首版 `plugin-development-scripts` 是否只要求 Go WASM debug runner 提供正式 smoke，还是同时为 Tool/Review 插件提供占位型 root command。当前设计建议先把 Go WASM 做完整，把 TS 插件栈纳入统一 run/verify 入口但不强制首版 watcher。
