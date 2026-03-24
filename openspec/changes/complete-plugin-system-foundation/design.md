## Context

AgentForge 当前已经把插件系统的最小控制面搭起来了：`plugin-runtime` 和 `plugin-registry` 约定了统一 manifest 与双宿主真相源，`role-plugin-support` 让 Role YAML 进入 Go 侧执行配置，`go-wasm-plugin-sdk` 与 Go WASM runtime 让 Integration 类插件第一次具备真实执行能力。但这条线只覆盖了“插件基础设施的第一段”，还没有把 PRD 与 `PLUGIN_SYSTEM_DESIGN.md` 里剩余的关键插件能力变成可实施 contract。

当前缺口集中在四类：
- `WorkflowPlugin` 还停留在 kind/schema 占位，没有正式运行时与状态模型。
- `ReviewPlugin` 还没有真实挂到 Layer 2 深度审查扩展面，当前四个维度仍是固定实现。
- TypeScript 侧插件 SDK、`create-plugin` 脚手架、模板和验证流程都还不存在，开发者体验与设计文档承诺不一致。
- 插件分发与信任模型仍然停留在“内置 + 本地安装”，签名、审核、Git/npm/registry 来源和 Marketplace-ready 目录都没有正式规格。

这次设计必须同时满足两个约束：
- 以当前仓库真相为准，而不是机械回到旧文档里的 `go-plugin` 假设。当前 Go 宿主真实方向已经是 `runtime: wasm` + `wazero`。
- 给 `/opsx:apply` 一个可以连续实施的边界，避免把 P2/P3 的远景能力一次性揉成无法验证的大变更。

## Goals / Non-Goals

**Goals:**
- 把 Workflow 和 Review 两类插件补成正式的一等 capability，而不是继续停留在 manifest 占位层。
- 让插件运行时、注册中心、深度审查流水线、SDK 与脚手架围绕同一组 manifest/schema 演进，避免各层自己发明协议。
- 把插件生命周期从“注册/激活”扩展到 install/enable/activate/active/deactivate/disable/uninstall/update 的完整运营语义。
- 为未来 Marketplace 做好 source/trust/release contract，但不要求这次直接交付公开市场产品。
- 让插件作者能够在当前仓库中真实生成、构建、验证 Tool/Review/Integration/Workflow 四类插件模板。

**Non-Goals:**
- 不在本次中建设公开可运营的 OCI Registry、支付体系、评分评论或完整市场前端。
- 不在本次中交付 Extism 双宿主运行时，也不把 TS 侧改成 WASM 插件执行器。
- 不在本次中交付完整的可视化工作流编辑器；工作流以 manifest/runtime/API 为主。
- 不在本次中实现 Event-Driven 全量事件编排和 Hierarchical 的完整调度器；这两类模式在本次只做到 schema 与显式支持边界，不做全量执行引擎。

## Decisions

### 1. 插件系统继续采用 “Go 控制面 + 宿主分治执行” 模型

这次不会再引入平行的插件真相源。Go 侧 registry 仍是唯一权威记录，负责安装来源、信任元数据、生命周期状态、版本与运行态聚合；TS Bridge 和 Go runtime 只负责执行与上报。这样 Workflow/Review/Tool/Integration 四类插件都能通过同一控制面暴露给 Dashboard、API 和任务编排层。

备选方案：
- 让 TS Bridge 自己持有 Review/Tool 插件真相。问题是状态与安装来源会再次分裂，和现有 registry-first 方向冲突。
- 为 Marketplace 单独做一套 catalog 存储。问题是会造成“目录一份、已安装一份、运行态一份”的三源分离。

### 2. WorkflowPlugin 本次以 Sequential 真实执行为主，其他模式显式保留为后续

`WorkflowPlugin` 会成为新的正式 capability，但首个可执行模式只实现 `process: sequential`。这样可以尽快复用现有 agent spawn、task/review service、worktree 与 role projection seams，形成一条能跑通的“角色绑定 -> 步骤输入输出 -> 失败重试/回退 -> 状态可见”的真实链路。`hierarchical` 与 `event-driven` 会保留在 manifest/schema 设计中，但 activation/execution 必须返回显式 unsupported error，而不是被错误地当成已支持。

备选方案：
- 一次性交付 Sequential + Hierarchical + Event-Driven。问题是需要同时引入 manager/worker 调度、事件总线订阅和更复杂的恢复模型，apply 风险过大。
- 完全不做 Workflow runtime，只先写 schema。问题是继续停留在“设计存在、行为缺席”的状态，无法兑现插件系统补全目标。

### 3. ReviewPlugin 作为 Layer 2 深度审查的标准扩展点，内置维度也走统一插件模型

当前深度审查的四个维度已经有固定 contract，但还不是插件化的。新设计会把 ReviewPlugin 变成标准扩展面：内置逻辑/安全/性能/合规维度以 internal plugin 的形式注册，自定义 ReviewPlugin 通过 TS Bridge 的 MCP runner 执行，最后统一聚合到 `deep-review-pipeline` 的 finding/recommendation 模型里。这样后续新增团队规范、架构规则或专项扫描时，不必继续修改桥接层核心代码路径。

备选方案：
- 继续保留固定四维实现，只在外围再加“自定义步骤”。问题是结果模型与执行计划会分叉。
- 让 ReviewPlugin 直接绕开 deep-review-pipeline 自己写结果。问题是审查结论与通知状态无法复用既有 review contract。

### 4. 插件开发者体验用 “共享 schema + 双 SDK + 单脚手架” 收敛

开发者体验层会被单独定义为 capability，而不是散落在 README、样例和脚本里。TypeScript SDK 负责 Tool/Review 的 manifest helper、MCP bootstrap、review finding formatter 和本地 test harness；Go SDK 延续现有 WASM 契约，补足对更通用 Go-hosted plugin 模板与打包验证的支持；`create-plugin` 作为统一脚手架，按插件类型生成对应模板、构建脚本、测试骨架和 manifest。

备选方案：
- 只提供文档与示例，不做 SDK/scaffold。问题是插件作者仍要手写大量样板代码，DX 与设计文档不符。
- 为每类插件做独立脚手架。问题是维护成本高，而且 manifest/schema 容易漂移。

### 5. 分发与信任先做 multi-source + verification contract，不直接做公开 Marketplace 产品

这次会先把插件来源与信任模型做实：built-in、local path、git、npm package/tarball、configured catalog/registry entry 都要有统一 source record；安装与更新前要有 digest/signature/approval 的校验路径；registry 需要暴露“已安装插件”和“可安装目录项”的明确边界。这样既能支撑真实安装流，也为未来 Marketplace 留出 contract，而不要求这次同步交付大而全的前端市场。

备选方案：
- 继续只支持本地与内置来源。问题是文档里承诺的分发/审核/市场能力没有落点。
- 一步做到 OCI Registry + 公共市场 UI。问题是超出当前 apply 的可控范围。

### 6. 生命周期与观测统一扩展到 install/deactivate/update，而不是新增一套状态机

现有状态值 `installed / enabled / activating / active / degraded / disabled` 足够表达多数运行态，但文档里的 install、deactivate、uninstall、update 这些运营动作还没有成为正式 contract。本次不会再增加一套新状态枚举，而是把这些动作建模为状态迁移事件与 registry metadata 变化：例如空闲 deactivation 回到 `enabled`，update 保持插件 identity 但替换 version/source/digest，再重新走 enable/activate 流程。

备选方案：
- 为每个动作单独加新状态。问题是会放大前后端状态分支，并与现有 UI/handler 语义冲突。

## Risks / Trade-offs

- [Sequential-first 会让 WorkflowPlugin 的长期蓝图分阶段落地] -> 在 spec 和错误语义里显式声明 unsupported modes，避免用户误判已支持。
- [ReviewPlugin 插件化会给深度审查 SLA 增加变量] -> 内置四维先迁移成 internal plugins，保持默认执行计划稳定，再引入可选外部插件。
- [多来源安装与签名校验会扩大控制面复杂度] -> 统一通过 registry 记录 digest/signature/approval，避免在 host 侧各自实现校验逻辑。
- [SDK、模板和 runtime 容易发生漂移] -> 让模板/样例进入仓库验证面，并复用同一 manifest/schema 校验器。
- [长文档里的 go-plugin / Extism 描述与当前 repo truth 存在漂移] -> 这次设计明确以已归档 OpenSpec 和当前代码为准，同时把文档对齐作为任务的一部分。

## Migration Plan

1. 先扩展 OpenSpec contract：新增 Workflow/Review/DX/Distribution 四个 capability，并修改 runtime/registry/review/sdk 相关主 spec。
2. 在 Go 侧补齐 workflow runtime 与 registry/source/trust model，在 TS 侧补齐 review plugin runner 与 TS SDK 基础包。
3. 用 internal review plugins 收敛现有四个深度审查维度，保持外部 API 兼容的同时把执行模型改成可扩展。
4. 增加 `create-plugin` 脚手架、模板、样例与 repo 验证脚本，让 SDK 输出能被持续验证。
5. 最后补齐安装/更新/catalog API 与相关文档，再对内置插件和样例插件执行回归验证。

回滚策略：
- Workflow/Review 新 runner 默认通过 capability-aware activation 进入，不改已有 Tool/Integration happy path；如果新 runner 不稳定，可保持新 plugin kinds 处于 disabled/unsupported 状态而不影响既有插件。
- 多来源安装与 trust metadata 采用向后兼容字段扩展；若 catalog/install 流出现问题，可暂时只保留 built-in/local source 路径。
- 内置 review dimensions 迁移到 internal plugins 时保留旧聚合 contract，确保 review API 输出形状不变。

## Open Questions

- Workflow 的首次可执行入口是单独的 “run workflow” API，还是直接绑定到既有 task/review 触发器？
- 签名与 approval 的信任根是 repo-local keyring、数据库配置，还是外部 KMS/CI 发布流程？
- npm/catalog 安装是以 tarball/digest 直装为主，还是允许在受控环境里委托包管理器执行安装？
