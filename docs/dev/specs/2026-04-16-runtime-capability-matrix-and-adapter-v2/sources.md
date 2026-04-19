# 对标证据来源表

每条矩阵 `✓` / `~` 单元格的脚注编号在此条目化。占位符——实际内容由 spec 填充阶段产生。

**格式**：

| ID | 项目 | URL | 访问日期 | 归档位置 |
|---|---|---|---|---|
| cc-1  | Claude CLI | https://... | 2026-04-18 | web.archive.org/... 或 sources/screenshots/cc-1.png |
| sdk-1 | Claude SDK | https://... | 2026-04-18 | ... |

## Claude CLI（前缀 `cc-`）

证据快照日：**2026-04-18**。基准 = Claude Code CLI（`claude` TTY 工具，不含 Agent SDK）。
所有 URL 为一手官方文档；`docs.claude.com/en/docs/claude-code/*` 已 301 迁移至 `code.claude.com/docs/en/*`，引用使用新域。

| ID | 项目 | URL | 访问日期 | 归档位置 |
|---|---|---|---|---|
| cc-1  | Claude CLI | https://code.claude.com/docs/en/overview | 2026-04-18 | sources/screenshots/cc-1.png (TODO) |
| cc-2  | Claude CLI | https://code.claude.com/docs/en/cli-reference | 2026-04-18 | sources/screenshots/cc-2.png (TODO) |
| cc-3  | Claude CLI | https://code.claude.com/docs/en/checkpointing | 2026-04-18 | sources/screenshots/cc-3.png (TODO) |
| cc-4  | Claude CLI | https://code.claude.com/docs/en/checkpointing | 2026-04-18 | sources/screenshots/cc-4.png (TODO) |
| cc-5  | Claude CLI | https://code.claude.com/docs/en/interactive-mode | 2026-04-18 | sources/screenshots/cc-5.png (TODO) |
| cc-6  | Claude CLI | https://code.claude.com/docs/en/hooks | 2026-04-18 | sources/screenshots/cc-6.png (TODO) |
| cc-7  | Claude CLI | https://code.claude.com/docs/en/cli-reference | 2026-04-18 | sources/screenshots/cc-7.png (TODO) |
| cc-8  | Claude CLI | https://code.claude.com/docs/en/interactive-mode | 2026-04-18 | sources/screenshots/cc-8.png (TODO) |
| cc-9  | Claude CLI | https://code.claude.com/docs/en/permission-modes | 2026-04-18 | sources/screenshots/cc-9.png (TODO) |
| cc-10 | Claude CLI | https://code.claude.com/docs/en/settings | 2026-04-18 | sources/screenshots/cc-10.png (TODO) |
| cc-11 | Claude CLI | https://code.claude.com/docs/en/memory | 2026-04-18 | sources/screenshots/cc-11.png (TODO) |
| cc-12 | Claude CLI | https://github.com/anthropics/claude-code | 2026-04-18 | sources/screenshots/cc-12.png (TODO) |
| cc-13 | Claude CLI | https://code.claude.com/docs/en/hooks | 2026-04-18 | sources/screenshots/cc-13.png (TODO) |
| cc-14 | Claude CLI | https://code.claude.com/docs/en/skills | 2026-04-18 | sources/screenshots/cc-14.png (TODO) |
| cc-15 | Claude CLI | https://code.claude.com/docs/en/sub-agents | 2026-04-18 | sources/screenshots/cc-15.png (TODO) |
| cc-16 | Claude CLI | https://code.claude.com/docs/en/commands | 2026-04-18 | sources/screenshots/cc-16.png (TODO) |
| cc-17 | Claude CLI | https://code.claude.com/docs/en/costs | 2026-04-18 | sources/screenshots/cc-17.png (TODO) |
| cc-18 | Claude CLI | https://code.claude.com/docs/en/mcp | 2026-04-18 | sources/screenshots/cc-18.png (TODO) |
| cc-19 | Claude CLI | https://code.claude.com/docs/en/output-styles | 2026-04-18 | sources/screenshots/cc-19.png (TODO) |

## Claude Agent SDK（前缀 `sdk-`）

证据快照日：**2026-04-18**。基准 = `@anthropic-ai/claude-agent-sdk@0.2.109`（TypeScript）+ `claude-agent-sdk` for Python（GitHub wiki `anthropics/claude-agent-sdk-python`）。两端保持版本同步（claudeCodeVersion 2.1.109）。
TS 类型权威：npm tarball 解压后的 `package/sdk.d.ts`（4688 行，本机校对路径 `/tmp/sdk-ts/package/sdk.d.ts`）。Python 权威：`ClaudeAgentOptions` dataclass、`ClaudeSDKClient` class、`SubprocessCLITransport`。

| ID | 项目 | URL | 访问日期 | 归档位置 |
|---|---|---|---|---|
| sdk-1 | Claude Agent SDK (TS) | https://registry.npmjs.org/@anthropic-ai/claude-agent-sdk/0.2.109 | 2026-04-18 | sources/screenshots/sdk-1.png (TODO); tarball at /tmp/sdk-ts/package/sdk.d.ts |
| sdk-2 | Claude Agent SDK (TS) | https://github.com/anthropics/claude-agent-sdk-typescript/blob/main/README.md | 2026-04-18 | sources/screenshots/sdk-2.png (TODO) |
| sdk-3 | Claude Agent SDK (Python) | https://github.com/anthropics/claude-agent-sdk-python | 2026-04-18 | sources/screenshots/sdk-3.png (TODO); deepwiki anthropics/claude-agent-sdk-python wiki 2.3/3.2/5.3/6.1/6.2 |
| sdk-4 | Claude Agent SDK (Python) | https://deepwiki.com/anthropics/claude-agent-sdk-python/6.2 | 2026-04-18 | sources/screenshots/sdk-4.png (TODO) — file checkpointing + rewind_files |
| sdk-5 | Claude Agent SDK (Python) | https://deepwiki.com/anthropics/claude-agent-sdk-python/3.2 | 2026-04-18 | sources/screenshots/sdk-5.png (TODO) — ClaudeSDKClient.set_model / set_permission_mode |
| sdk-6 | Claude Agent SDK (TS) | https://github.com/anthropics/claude-agent-sdk-typescript/blob/main/CHANGELOG.md | 2026-04-18 | sources/screenshots/sdk-6.png (TODO) — 0.2.76 planFilePath; 0.1.45 structured output; 0.2.21 reconnect/toggleMcpServer; 0.2.63 supportedAgents; 0.2.72 getSettings; 0.2.74 skills user-invocable |
| sdk-7 | Claude Agent SDK | https://docs.claude.com/en/api/agent-sdk/custom-tools | 2026-04-18 | sources/screenshots/sdk-7.png (TODO) — @tool decorator + create_sdk_mcp_server |
| sdk-8 | Claude Agent SDK | https://docs.claude.com/en/api/agent-sdk/overview | 2026-04-18 | sources/screenshots/sdk-8.png (TODO) — Agent SDK reference overview |

## Cursor（前缀 `cur-`）

_待填充。建议来源：_
- Cursor docs
- Cursor Release notes

## Aider（前缀 `aid-`）

_待填充。建议来源：_
- https://github.com/paul-gauthier/aider

## Codex（前缀 `cdx-`）

_待填充。建议来源：_
- OpenAI Codex CLI docs
- Codex CLI GitHub

## OpenCode（前缀 `oc-`）

_待填充。建议来源：_
- OpenCode docs
- OpenCode GitHub

## Gemini CLI（前缀 `gmi-`）

_待填充。建议来源：_
- Gemini CLI GitHub
- Gemini CLI docs
