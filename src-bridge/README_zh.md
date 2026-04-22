# AgentForge Bridge

AgentForge 的 TypeScript/Bun 运行时桥接服务，负责将各类 AI 运行时（Claude Code、Codex、OpenCode、Cursor、Gemini 等）接入 AgentForge 编排后端。

## 技术栈

- **Runtime**: Bun + TypeScript (ESM)
- **Web 框架**: Hono
- **核心协议**: MCP (Model Context Protocol)、ACP (Agent Client Protocol)
- **AI SDK**: `ai` SDK + Anthropic / OpenAI / Google 多模型适配

## 目录结构

```
src/
  server.ts           # 服务入口
  runtime/            # 运行时适配器（claude_code、codex、opencode、cursor、gemini、qoder、iflow）
  handlers/           # HTTP 请求处理器
  plugins/            # 插件宿主与管理
  mcp/                # MCP 集成
  session/            # 会话管理
  review/             # 审阅流水线
  schemas.ts          # 共享校验 schema（Zod）
  lib/                # 工具库与日志
  middleware/         # 中间件（trace 等）
tests/                # 测试用例
```

## 快速开始

```bash
# 安装依赖
pnpm install

# 开发模式
pnpm dev
# 或直接使用 bun
bun run src/server.ts

# 类型检查
pnpm typecheck

# 构建可执行文件
pnpm build

# 代码检查
pnpm lint
```

## 环境变量

| 变量 | 说明 | 示例 |
|------|------|------|
| `BRIDGE_PORT` | 服务监听端口 | `7778` |
| `ORCHESTRATOR_URL` | Go 后端地址 | `http://localhost:7777` |
| `BRIDGE_API_KEY` | 与后端通信的 API Key | - |

> 完整环境变量配置请参考项目根目录的 `.env.example`。

## 运行时适配器

Bridge 通过 `AgentRuntime` 接口统一接入多种 AI 编程助手：

- **claude_code** — Anthropic Claude Code CLI
- **codex** — OpenAI Codex CLI
- **opencode** — OpenCode 运行时
- **cursor** — Cursor IDE Agent 模式
- **gemini** — Google Gemini CLI
- **qoder** / **iflow** — 扩展运行时

新增运行时只需实现 `AgentRuntime` 接口并注册到 `RuntimeRegistry` 即可。

## 核心功能

- **任务执行**: 接收编排后端的执行请求，调度到对应运行时
- **会话管理**: 运行时会话生命周期与状态持久化
- **实时事件流**: WebSocket 向编排后端推送运行日志与进度
- **插件系统**: 动态加载桥接插件，扩展命令与工具集
- **审阅流水线**: 代码审阅结果收集与回传
- **模型切换**: 运行时内动态切换底层 LLM 模型
