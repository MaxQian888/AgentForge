## Context

AgentForge 已经有一条桌面能力主线：Tauri v2 已接入 updater plugin，`src-tauri/src/lib.rs` 也提供了 `check_for_update` 命令，前端通过 `lib/platform-runtime.ts` 的 `checkForUpdate()` 暴露给 `app/(dashboard)/plugins/page.tsx`。但当前实现只覆盖“触发检查”这一步，缺少三个关键闭环：

- 运行时闭环不完整：前端拿不到版本、说明、下载/安装进度、安装完成后的“需要重启”状态，也没有正式的 restart handoff。
- 配置闭环不完整：`src-tauri/tauri.conf.json` 里的 `plugins.updater.endpoints` 与 `pubkey` 为空，`bundle.createUpdaterArtifacts` 未启用，意味着 release 产物本身不满足 Tauri updater 官方要求。
- 发布闭环不完整：现有 `.github/workflows/build-tauri.yml` 和 `.github/workflows/release.yml` 只构建并上传桌面包，没有生成 updater signatures，也没有发布 `latest.json` 或等价的静态更新清单。

官方 Tauri v2 updater 文档进一步限定了这条链路的实现方式：

- updater 不能关闭签名校验，必须有公私钥。
- release 构建必须启用 `bundle.createUpdaterArtifacts = true` 才会产出 updater 所需的 `.sig` 和更新 bundle。
- 前端可以通过 `@tauri-apps/plugin-updater` 使用 `check() / download() / install() / downloadAndInstall()`，并通过 `@tauri-apps/plugin-process` 的 `relaunch()` 完成安装后切换。
- 静态 JSON 清单必须包含版本、平台 URL 和内联 signature，而且 manifest 中不能夹带不完整平台记录。

这说明本次不是单点 UI 修补，而是跨 `src-tauri`、前端 facade、release workflow、签名与发布清单的一次交叉变更。

## Goals / Non-Goals

**Goals:**

- 把 AgentForge 桌面更新能力从“只检查”补齐为“检查、展示、下载、安装、重启切换”的完整产品合同。
- 保持共享平台能力 facade 仍是前端唯一入口，不让页面直接散落使用 updater/process 原始 API。
- 让 Tauri 配置、capability 权限、NPM/Rust 依赖和 release workflow 与官方 v2 updater 流程一致。
- 为 GitHub Releases 这条现有发布路径补上 updater artifacts、签名和静态 manifest 发布，使桌面客户端具备真实可消费的更新源。
- 保证 web 模式、本地未配置发布密钥的开发模式、以及 release 模式下的行为边界都显式可判断。

**Non-Goals:**

- 不在本次内设计动态 update server、灰度发布控制台或多 release channel 管理后台。
- 不把桌面更新状态接入所有页面；第一波只要求共享 facade 完整、至少一个现有桌面面板能消费完整状态。
- 不在本次内解决桌面包的商店分发、平台 notarization 细节或完整代码签名运营流程。
- 不把 updater 改造成通过 Go 或 TS Bridge 转发的业务主链路；它仍属于桌面壳的本地能力。

## Decisions

### 1. 继续保留“共享平台能力 facade”作为前端唯一入口，但桌面实现升级为官方 updater/process guest plugin 组合

当前 `platformRuntime.checkForUpdate()` 只是调用 Rust `check_for_update` 并返回布尔式结果，这不足以表达完整更新生命周期。新设计仍保留 `lib/platform-runtime.ts` / `hooks/use-platform-capability.ts` 作为唯一消费入口，但桌面分支要升级为统一封装：

- `checkForUpdate` 返回规范化的更新元数据，而不只是 `ok/failed`
- `installUpdate` 或等价方法负责下载/安装并汇报进度
- `relaunchAfterUpdate` 或等价方法负责安装完成后的重启切换

内部实现优先复用官方 `@tauri-apps/plugin-updater` 和 `@tauri-apps/plugin-process`，而不是继续在 Rust 侧自造一套下载/安装状态协议。原因是官方 guest API 已经提供 `check()`、`downloadAndInstall()` 和 `relaunch()`，并且天然支持下载进度。

备选方案：

- 继续用 Rust `#[tauri::command]` 包装完整 updater 生命周期。缺点是需要手工定义更多 IPC 与进度事件，重复官方 guest plugin 能力。
- 让页面直接 import updater/process API。缺点是会破坏已建立的平台能力统一入口，测试和 web fallback 也会变散。

### 2. 用显式“更新会话状态”替换当前粗粒度成功/失败消息

桌面更新不是瞬时动作，UI 需要表达至少这些阶段：

- `idle`
- `checking`
- `available`
- `downloading`
- `installing`
- `ready_to_relaunch`
- `up_to_date`
- `failed`

共享 facade 应输出稳定的状态对象，包括版本、日期、说明、进度与错误摘要。这样 `plugins` 页或后续桌面设置页才能不依赖命令细节，直接渲染状态与 CTA。

备选方案：

- 把状态全部留在页面本地、只返回 Promise 成败。缺点是不同页面会重复建模，无法稳定测试，也难以支撑事件型 UI。

### 3. 发布链路采用“GitHub Releases + 静态 latest.json manifest”作为第一波 updater source of truth

仓库已经有 `build-tauri.yml` 与 `release.yml`，且 release 流程默认面向 GitHub Releases。第一波最稳的方案不是新增动态 update server，而是沿现有发布面继续走：

- Tauri 构建启用 `bundle.createUpdaterArtifacts = true`
- release job 在全部平台构建完成后收集 bundle 与 `.sig`
- 生成符合 Tauri v2 静态 JSON 结构的 `latest.json`
- 将 `latest.json` 与桌面包一起附加到 GitHub Release
- `tauri.conf.json` 的 updater `endpoints` 指向该静态 manifest

这样能在不引入新服务的前提下形成完整闭环，并与官方文档给出的静态 JSON 模式一致。

备选方案：

- 动态 update server：未来灵活性更高，但会把这次 change 扩成新的服务与运维面。
- 只上传 bundle，不生成 manifest：无法真正被 updater 消费，仍然是不完整实现。

### 4. 签名与公钥配置遵循“私钥只进环境，公钥与 endpoint 进公开配置”的分层

根据官方文档，签名私钥绝不能进仓库，且 `.env` 不能作为构建输入。因此本次设计采用：

- 私钥与可选密码只通过 CI secrets 或运行时环境变量注入
- 公钥作为公开校验材料进入 updater 配置
- release workflow 在缺少签名输入时显式失败，不发布伪完整 updater release
- 本地 dev 或未配置发布密钥的普通构建仍允许继续，但 updater 应表现为未配置或不可用，而不是假装可工作

备选方案：

- 把公私钥都留空，仅保留 UI 入口。缺点是会继续制造“表面可用、实际不可发布”的伪能力。

## Risks / Trade-offs

- [签名材料管理增加 CI 复杂度] -> 通过将 updater 完整发布限定在 tag/release 路径，并在 workflow 中输出明确的缺失项诊断，减少日常开发路径负担。
- [GitHub Release 静态 manifest 容易引用不完整平台产物] -> 只在所有目标构建和签名完成后生成 `latest.json`，并在发布前校验每个平台 URL 与 `.sig` 成对存在。
- [Windows 安装流程会打断当前应用实例] -> 在 facade 中显式建模 `ready_to_relaunch` 和安装前后的用户提示，必要时为 Windows 增加受控 install mode。
- [沿用现有 `check_for_update` 命令会与新 guest-plugin 流程并存一段时间] -> 在实现阶段要收敛为单一 source of truth，避免同一页面同时走两条 updater 路径。

## Migration Plan

1. 先补齐依赖与权限：添加 process guest plugin、必要 capability 权限，以及 updater 所需的公开配置字段来源。
2. 收敛前端 contract：把 `platform-runtime` 扩展为完整 updater facade，并让现有桌面入口消费规范化状态。
3. 对齐 Tauri 配置：启用 updater artifacts、补齐 pubkey/endpoints/config gating，明确本地未配置时的行为。
4. 改造 CI/release：在 tag release 流程里生成签名产物、静态 manifest，并附加到 GitHub Release。
5. 验证三条路径：web fallback、本地桌面未配置 updater 的安全退化、release 模式下的 updater artifact/manifest 完整性。

回滚策略：

- 如果发布链路未能稳定完成，可以先保留 facade 和 UI 的完整状态模型，但把 updater 标记为 `unconfigured`，而不是继续暴露“可检查但不可安装”的半成品行为。
- 如果官方 guest plugin 集成引入不可接受的不稳定性，可临时保留 Rust 侧检查命令用于只读检查，但不得继续宣称自动更新已完整。

## Open Questions

- 第一波是否只支持稳定通道 `latest.json`，把 beta/canary channel 留到后续 change？
- Windows 安装模式是否沿官方默认 `passive`，还是需要在 AgentForge 中暴露为可配置项？
