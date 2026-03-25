## Context

当前 `src-im-bridge` 的运行入口已经支持 `feishu/slack/dingtalk/telegram/discord` 五个平台，但平台选择仍通过 `cmd/bridge/platform_registry.go` 中的硬编码 `platformDescriptors()` 完成。这个模式让现有多平台能力可以工作，却把平台扩展点固定在主启动流程里：新增平台、增强某个平台的高级能力、或者把平台适配层逐步对齐仓库主系统的 plugin/runtime 词汇时，都必须继续改中心化注册表。

与此同时，仓库主系统已经有统一的 plugin manifest、Go WASM runtime、`IntegrationPlugin` 词汇和 `plugins/integrations/feishu-adapter` 示例；`src-im-bridge` 本身也已有 capability matrix、reply target、native interaction callback 与 control-plane 语义。这说明 IM Bridge 不需要重新发明一套完全独立的扩展框架，而是应当把现有平台适配层收敛成一个 bridge-local、plugin-compatible provider contract。

飞书侧的现状也说明需要进一步抽象。当前实现已经具备基础 interactive card、`card.action.trigger` 回调和 delayed update 路径，但 richer capability 仍缺少统一建模：模板卡片与变量、JSON 卡片与模板卡片的双发送面、3 秒回调响应窗口、30 分钟 delayed update token 约束、以及后续链接预览/卡片局部更新等更细粒度 surface 还没有成为明确 contract。官方文档则已经把这些能力整理为稳定 API，因此现在是把 Feishu surface 做成一等扩展面的合适时机。

## Goals / Non-Goals

**Goals:**
- 为 `src-im-bridge` 定义平台 provider contract，使平台发现、配置校验、能力声明与实例装配不再依赖硬编码启动分支。
- 保留当前“单进程单活平台”的部署与心智模型，不引入一次性的大规模运行时迁移。
- 为飞书建立 richer card lifecycle contract，覆盖 JSON 卡片、模板卡片、模板变量、交互回调、即时响应与 delayed update。
- 让 `notify`、control-plane 和未来平台扩展都能从 provider 能力声明中得知可用 renderer、callback 语义和降级策略。
- 让后续把某些平台外置成真正的 `IntegrationPlugin` 时，可以沿用同一组 descriptor/capability 语义，而不是再次重写桥内协议。

**Non-Goals:**
- 本次不把所有现有平台立即迁出为外部可安装插件。
- 本次不引入新的多平台并行运行模型，仍保持每个 Bridge 进程只承载一个 active provider。
- 本次不一次性覆盖飞书所有开放能力，例如完整链接预览、卡片局部更新、全文流式更新、多租户模板治理。
- 本次不要求修改仓库主系统的通用 plugin registry 语义来直接托管 `src-im-bridge` provider 生命周期。

## Decisions

### 1. 先落 bridge-local provider contract，而不是直接把 IM 平台改造成外部 IntegrationPlugin

设计上新增一个 bridge-local provider seam，用统一 descriptor 表示平台标识、支持的 transport mode、能力矩阵、配置校验、实例工厂、以及可选的 richer surface。`cmd/bridge` 不再直接维护平台条件分支，而是通过 provider registry/loader 按 `IM_PLATFORM` 解析目标 provider，再创建实际的 `core.Platform` 实例。

这样做的原因是当前仓库已经有 plugin/runtime 方向，但 IM Bridge 仍有大量现有内建平台代码与测试资产。若现在直接强推外部 `IntegrationPlugin` 激活、签名、安装与 runtime 管理，会把任务范围从“收敛扩展 seam”扩大到“重做运行模型”。bridge-local provider contract 则可以先统一边界，再逐步支持 externalized provider 输出同样的 descriptor。

备选方案：
- 继续保留 `platformDescriptors()` 硬编码 map。优点是最小改动，缺点是后续每加一个 provider 或 richer surface 仍会继续堆积启动分支。
- 直接把 Feishu/Slack/Telegram 全部改成外部 `IntegrationPlugin`。优点是词汇统一，缺点是与当前 `src-im-bridge` 的控制面、reply target 和 smoke/test 基线耦合过深，超出本次可控范围。

### 2. 保留通用 `StructuredMessage` 路径，但为 provider-native richer payload 引入独立扩展面

`core.StructuredMessage` 继续承担跨平台的最小公共 structured surface；它适合表达标题、字段、按钮这类可迁移结构，但不适合承载 Feishu 的模板卡片、模板变量、共享/独享更新语义、以及未来更细的 cardkit 生命周期。因此本次设计不把 Feishu 模板能力硬塞进 `StructuredMessage`，而是在 provider contract 之上引入 provider-native outbound extension。

对 Feishu 来说，这个 extension 需要至少区分两种 payload 形态：
- JSON card：直接发送或更新飞书卡片 JSON。
- template card：通过 `template_id`、可选版本与 `template_variable` 发送/更新模板卡片。

通用 notification/control-plane 路径可以优先尝试 provider-native payload；若当前 provider 或 reply target 不支持，再回落到现有 `StructuredMessage` 或纯文本路径。

备选方案：
- 继续只用 `core.Card` / `StructuredMessage`。问题是模板变量、多语言版本、共享/独享更新这些 Feishu 特性会被压扁成字符串或 metadata，长期不可维护。
- 对所有平台统一采用 `map[string]any` 原始 payload。问题是失去类型边界，调用方需要理解 provider 细节，无法形成可验证 contract。

### 3. Feishu 回调与更新生命周期采用“同步确认 + 异步更新”双阶段模型

飞书官方回调要求服务端在 3 秒内完成回调响应，而 delayed card update 必须在响应回调之后、且在 token 有效窗口内完成。现有代码已经有基础 token / callback 语义，但还缺少统一设计边界。本次明确采用双阶段模型：
- 第 1 阶段：收到 `card.action.trigger` 后，Bridge 先将交互规范化为共享 action contract，并在 3 秒内返回空响应、toast 或即时卡片更新。
- 第 2 阶段：若业务处理超过即时窗口，则以 preserved reply target 中的 update token / message identity 走 delayed update；若 token 失效或当前卡片模式不支持原位更新，再显式降级为 reply/plain text。

这保证上游后端不需要直接处理 Feishu 回调协议细节，但 Bridge 又不会错误地把 delayed update 当成普通 reply。

备选方案：
- 在回调 HTTP handler 内同步等待完整业务处理。问题是很容易违反 3 秒窗口，导致卡片交互失败。
- 把原始 Feishu callback payload 直接透传给后端。问题是会把 provider 细节泄露到共享 action contract，破坏平台无关层。

### 4. 能力声明由 provider 自身负责产出，Bridge/控制面只消费统一矩阵

provider descriptor 负责声明基础 capability matrix 与 provider-native extension surface。Bridge health、runtime registration、notify 路径、reply-plan 选择逻辑只消费规范化矩阵与 extension metadata，不再通过 provider 名称推断“飞书应该能怎样、Telegram 应该不能怎样”。

这样做可以让 Feishu 新增 template card 或 delayed update 语义时，只需要增强 Feishu provider 的 descriptor 和实现，而不需要在上层散落新的 `if platform == "feishu"` 逻辑。

## Risks / Trade-offs

- [Risk] provider contract 与仓库主系统 plugin/runtime 词汇可能出现“两套相似抽象”。 → Mitigation：contract 仅定义 `src-im-bridge` 所需最小 seam，并明确 descriptor 字段与现有 `IntegrationPlugin` 语义保持兼容，避免重复发明生命周期概念。
- [Risk] Feishu JSON card 与 template card 双模型会增加 notify/control-plane 负担。 → Mitigation：用 typed union 或显式 mode 字段区分两种 payload，禁止在单个 payload 中混合多种发送语义。
- [Risk] delayed update token 存在时效与次数限制，若业务编排处理不当会出现“更新失败又回退为重复消息”。 → Mitigation：把 token 生命周期显式建模到 reply target / metadata，并在失败时记录 provider-aware fallback 原因。
- [Risk] 保持单进程单活平台意味着短期内不能在一个 Bridge 进程里同时托管多个 provider。 → Mitigation：明确这仍是当前部署基线，本次的目标是先把 provider seam 做对，为未来多实例或外置 provider 演进铺路。
- [Risk] 旧有测试主要围绕硬编码 provider map 和基础 Feishu card happy path，重构 seam 时容易出现无声回归。 → Mitigation：为 provider resolution、Feishu callback schema 2.0、template payload、delayed update 和 fallback path 增补 focused tests 与 smoke fixtures。

## Migration Plan

1. 引入 provider descriptor / registry / loader，并为当前内建 `feishu/slack/dingtalk/telegram/discord` 提供一层兼容包装，确保 `IM_PLATFORM`、`IM_TRANSPORT_MODE` 和现有凭证环境变量保持不变。
2. 将 `cmd/bridge` 启动流程切换到 provider contract，但保留现有 `core.Platform`、命令引擎与 control-plane 外部接口，避免一次性改变运行模型。
3. 为 Feishu provider 新增 typed richer card lifecycle：统一 JSON card 与 template card 发送面，补齐 callback schema 2.0 规范化、即时响应与 delayed update 协调逻辑。
4. 让 `notify`、health、registration、README、runbook、smoke fixtures 与测试基线切换到新 seam，并记录显式降级行为。

回滚策略：
- 若 provider registry 引发启动回归，可在实现阶段保留一层 built-in provider fallback，使旧的 provider descriptor 集合继续覆盖现有平台。
- 若 Feishu richer lifecycle 在真实环境中不稳定，可临时退回仅使用现有 JSON card / plain-text fallback 的发送路径，而不撤销 provider contract 本身。

## Open Questions

- 后端通知契约是否应在本次就新增一等的 Feishu template-card 字段，还是先通过 bridge-local provider payload metadata 过渡一版。
- 第一个真正 externalized 的 IM provider 是否应该直接选择 Feishu，还是等 bridge-local contract 在第二个平台上跑稳后再迁出。
- 链接预览、卡片局部更新、CardKit 实体级 API 等更深的飞书能力，后续应继续挂在 `feishu-rich-card-lifecycle` 下演进，还是拆分成独立 capability。
