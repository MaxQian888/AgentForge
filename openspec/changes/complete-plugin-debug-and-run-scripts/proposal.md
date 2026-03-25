## Why

AgentForge 已经有可运行的 Go WASM 插件运行时和 SDK 文档，但当前仓库给插件作者的本地开发闭环仍然是不完整的：根目录只有构建脚本，没有面向插件开发的调试脚本、运行脚本和统一验收命令。现在需要把这条最实际的缺口补成一个独立 change，让当前 SDK、样例插件和后续脚手架工作都有可复用的本地工作流，而不是继续依赖手改脚本和多终端手工拼装。

## What Changes

- 为仓库增加面向插件作者的根级开发脚本契约，覆盖 build、debug、run、verify 四类本地工作流。
- 将当前硬编码在 `scripts/build-go-wasm-plugin.js` 里的单一样例路径，扩展为可按 manifest 或目标路径解析的参数化构建能力。
- 增加 Go WASM 插件的本地调试入口，使开发者可以按平台真实 `AGENTFORGE_*` 运行时契约执行 `describe`、`init`、`health` 和自定义 operation，而不必先安装进注册表。
- 增加最小插件开发运行栈命令，统一启动或复用 Go Orchestrator 与 TS Bridge，并输出健康检查、端口和后续操作指引。
- 增加脚本级验证面，确保维护中的样例插件、文档命令和后续模板输出不会继续和仓库真实行为漂移。

## Capabilities

### New Capabilities
- `plugin-development-scripts`: 定义仓库对插件作者提供的本地 build/debug/run/verify 脚本行为与错误语义。

### Modified Capabilities
- `go-wasm-plugin-sdk`: 将现有样例插件工作流从“单一硬编码构建脚本”扩展为“可参数化构建 + 本地调试 + 激活前验证”的支持流程。

## Impact

- Affected code: root `package.json`, `scripts/*.js`, `scripts/*.test.ts`, `src-go/plugin-sdk-go`, maintained plugin samples under `plugins/**`
- Affected docs: `README.md`, `README_zh.md`, `docs/GO_WASM_PLUGIN_RUNTIME.md`, plugin-related operator/developer guidance
- Affected verification: focused script tests, sample plugin smoke checks, local plugin development instructions
- No external API breaking changes are intended; this change is repo-local developer workflow completion
