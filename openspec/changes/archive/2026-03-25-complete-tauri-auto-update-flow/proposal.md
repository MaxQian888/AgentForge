## Why

AgentForge 现在的 Tauri 桌面壳只实现了“检查更新”这一步：`src-tauri/src/lib.rs` 暴露了 `check_for_update`，但 `src-tauri/tauri.conf.json` 里的 updater `endpoints` 和 `pubkey` 仍为空，构建流程也没有生成或发布可供 updater 消费的签名产物与更新清单。结果是桌面模式在产品层看起来已经有 updater 入口，但实际上还没有形成“发现更新 -> 下载/安装 -> 重启切换 -> 后续版本继续可信更新”的完整闭环。

PRD 和插件系统设计都把 Tauri 定义为正式交付面，而不是只供本地调试的壳。既然桌面分发已经进入主路径，就需要把自动更新从“占位能力”补成可验证、可发布、可持续演进的正式能力。

## What Changes

- 将桌面端更新能力从“仅检查更新”扩展为完整 updater 生命周期，覆盖更新元数据读取、下载/安装进度、安装完成后的重启选择，以及非桌面环境下的稳定降级语义。
- 为桌面发布链路定义 updater 分发合同，覆盖签名密钥输入、`createUpdaterArtifacts` 产物生成、静态更新清单或等价 release metadata 的发布，以及缺少必需配置时的显式失败。
- 补齐共享平台能力 facade 与桌面面板的更新状态表达，使前端可以展示可用版本、说明、进度、安装结果和“需要重启”状态，而不再只有“check succeeded/failed”的粗粒度消息。
- 对齐 Tauri 配置、能力权限、依赖与 CI/release 工作流，使 updater 所依赖的 guest package、process relaunch 权限和发布工件生产方式与官方 v2 文档一致。

## Capabilities

### New Capabilities
- `desktop-update-distribution`: 定义 AgentForge 桌面发布链路如何生成、签名、校验并发布 updater 所需的更新产物与清单。

### Modified Capabilities
- `desktop-native-capabilities`: 将桌面更新能力从检查可用更新扩展为完整的检查、下载、安装、重启切换与降级合同。

## Impact

- Affected code: `src-tauri/src/lib.rs`, `src-tauri/Cargo.toml`, `src-tauri/tauri.conf.json`, `src-tauri/capabilities/*.json`, `lib/platform-runtime.ts`, `hooks/use-platform-capability.ts`, `app/(dashboard)/plugins/page.tsx`
- Affected tooling: root `package.json`, desktop build/release workflows under `.github/workflows/`, any helper scripts that package or publish desktop artifacts
- Affected verification: platform runtime tests, plugin/desktop panel tests, desktop build validation, updater artifact or release-manifest validation
- External dependencies: Tauri updater/process official guest-plugin flow, updater signing keys, release hosting strategy for static JSON or equivalent endpoint-backed metadata
