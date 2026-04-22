# AgentForge Bridge

The TypeScript/Bun runtime bridge service for AgentForge, connecting various AI runtimes (Claude Code, Codex, OpenCode, Cursor, Gemini, etc.) to the AgentForge orchestrator backend.

## Tech Stack

- **Runtime**: Bun + TypeScript (ESM)
- **Web Framework**: Hono
- **Core Protocols**: MCP (Model Context Protocol), ACP (Agent Client Protocol)
- **AI SDK**: `ai` SDK + Anthropic / OpenAI / Google multi-model adapters

## Directory Structure

```
src/
  server.ts           # Service entry point
  runtime/            # Runtime adapters (claude_code, codex, opencode, cursor, gemini, qoder, iflow)
  handlers/           # HTTP request handlers
  plugins/            # Plugin hosting and management
  mcp/                # MCP integration
  session/            # Session management
  review/             # Review pipeline
  schemas.ts          # Shared validation schemas (Zod)
  lib/                # Utilities and logging
  middleware/         # Middleware (trace, etc.)
tests/                # Test suite
```

## Quick Start

```bash
# Install dependencies
pnpm install

# Development mode
pnpm dev
# Or use bun directly
bun run src/server.ts

# Type checking
pnpm typecheck

# Build executable
pnpm build

# Linting
pnpm lint
```

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `BRIDGE_PORT` | Service listen port | `7778` |
| `ORCHESTRATOR_URL` | Go backend URL | `http://localhost:7777` |
| `BRIDGE_API_KEY` | API key for backend communication | - |

> For the full environment variable reference, see `.env.example` in the project root.

## Runtime Adapters

Bridge unifies access to multiple AI coding assistants through the `AgentRuntime` interface:

- **claude_code** — Anthropic Claude Code CLI
- **codex** — OpenAI Codex CLI
- **opencode** — OpenCode runtime
- **cursor** — Cursor IDE Agent mode
- **gemini** — Google Gemini CLI
- **qoder** / **iflow** — Extension runtimes

Adding a new runtime only requires implementing the `AgentRuntime` interface and registering it in the `RuntimeRegistry`.

## Core Features

- **Task Execution**: Receives execution requests from the orchestrator and dispatches them to the appropriate runtime
- **Session Management**: Runtime session lifecycle and state persistence
- **Real-time Event Streaming**: Pushes runtime logs and progress to the orchestrator backend via WebSocket
- **Plugin System**: Dynamically loads bridge plugins to extend commands and toolsets
- **Review Pipeline**: Collects and returns code review results
- **Model Switching**: Dynamically switches the underlying LLM model within a runtime
