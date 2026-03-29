## Why

TS Bridge 现在虽然把 `opencode` 注册成了可选 runtime，但实际执行仍依赖一个 Bridge 自定义的 `stdin -> JSON request / stdout -> ndjson events` 命令协议。OpenCode 官方当前面向自动化的真实集成面是 `opencode serve` HTTP API、`opencode run --format json` 和 `opencode acp`，因此现有实现更像占位适配器，无法真实保证 OpenCode 的会话、终止、恢复和诊断语义完整可用。

## What Changes

- 将 `opencode` runtime 从泛化命令占位适配器升级为基于 OpenCode 官方自动化接口的真实 Bridge 集成
- 为 OpenCode 执行引入真实的会话映射、异步运行、取消、暂停/恢复和状态同步语义，而不是仅靠本地快照重放原始 execute 请求
- 让 Bridge 对 OpenCode readiness 的判断从“命令是否存在”升级为“自动化 transport、服务可达性、provider/model 可用性、会话能力是否就绪”
- 保留现有 `/bridge/*` 对 Go 的 canonical contract，不把这次 change 扩成新的 Go/API/前端面板改造
- 补齐 OpenCode transport、事件归一化、失败恢复和本地验证文档与测试

## Capabilities

### New Capabilities
- `opencode-runtime-bridge`: 定义 TS Bridge 与 OpenCode 官方自动化接口之间的真实运行时桥接合同，包括 transport 选择、会话映射、事件归一化与生命周期控制

### Modified Capabilities
- `agent-sdk-bridge-runtime`: `opencode` 执行的 pause/resume/cancel/status 语义需要从本地占位快照升级为真实上游会话语义
- `bridge-agent-runtime-registry`: `opencode` catalog 诊断需要基于真实 OpenCode 集成前提而不是仅检查本地命令是否存在

## Impact

- **TS Bridge** (`src-bridge/`): OpenCode runtime adapter、transport client、session mapping、状态与恢复逻辑、测试
- **Bridge 文档/环境配置** (`README.md`, bridge docs, env examples): OpenCode server URL、认证、启动/连接与验证说明
- **Go 后端** (`src-go/internal/bridge/client.go`): 保持现有 `/bridge/*` contract，不引入新的上游 API，但需要兼容更真实的 OpenCode 运行时状态/恢复结果
- **External dependency**: 依赖 OpenCode 官方自动化接口（优先 `opencode serve`）而不是 Bridge 自定义 stdin 协议
