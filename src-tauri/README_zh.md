# AgentForge Desktop (Tauri)

AgentForge 的桌面端外壳，基于 Tauri 2.9 + Rust 构建，将 Next.js 前端打包为原生桌面应用，并通过 sidecar 机制 supervision Go 后端、TS Bridge 与 IM Bridge。

## 技术栈

- **框架**: Tauri 2.9
- **语言**: Rust (edition 2021, 最低 toolchain 1.77.2)
- **前端**: Next.js 16 (静态导出到 `out/`)
- **构建工具**: Cargo

## 目录结构

```
src/
  main.rs                 # 桌面应用入口
  lib.rs                  # Tauri 命令与状态管理
  bin/
    agentforge-desktop-cli.rs   # CLI 入口
  runtime_logic.rs        # 运行时逻辑
  process_cleanup.rs      # 进程清理
  standalone_cli.rs       # 独立 CLI 逻辑
tauri.conf.json           # Tauri 配置
Cargo.toml                # Rust 项目配置
capabilities/             # Tauri 能力声明
icons/                    # 应用图标
target/                   # Cargo 构建输出（gitignored）
```

## 快速开始

```bash
# 开发模式（热重载）
pnpm tauri dev

# 构建桌面安装包（生产环境）
pnpm tauri build

# 检查 Tauri 环境
pnpm tauri info
```

## 关键配置

`tauri.conf.json` 关键字段：

- `frontendDist`: `../out` — Next.js 静态导出目录
- `beforeDevCommand`: `pnpm dev` — 开发模式先启动 Next.js
- `beforeBuildCommand`: `pnpm build` — 构建前先构建 Next.js

> **注意**: 生产构建需要在 `next.config.ts` 中设置 `output: "export"`。

## Sidecar 监督

桌面端在后台 supervision 三个 sidecar 进程：

| Sidecar | 服务 | 默认端口 |
|---------|------|---------|
| Go Orchestrator | 主后端 API | `7777` |
| TS Bridge | Agent 运行时桥接 | `7778` |
| IM Bridge | 即时通讯桥接 | `7779` |

Tauri 负责这些进程的启动、生命周期管理与清理。

## 插件

当前使用的 Tauri 官方插件：

- `tauri-plugin-global-shortcut` — 全局快捷键
- `tauri-plugin-log` — 日志记录
- `tauri-plugin-notification` — 原生通知
- `tauri-plugin-shell` — 系统 shell 调用
- `tauri-plugin-dialog` — 系统对话框
- `tauri-plugin-process` — 进程管理
- `tauri-plugin-updater` — 自动更新（桌面平台）

## CLI 模式

除桌面窗口外，项目同时构建一个命令行入口 `agentforge-desktop-cli`，用于无头（headless）或自动化场景。
