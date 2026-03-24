## Why

AgentForge 的插件系统已经完成了统一 manifest、双宿主注册中心、Role YAML 支持，以及 Go WASM Integration runtime 的第一段落地，但这仍然只是“插件基础设施能跑起来”的阶段，不等于 PRD 和 `PLUGIN_SYSTEM_DESIGN.md` 承诺的完整插件体系已经可交付。当前最主要的缺口是：`WorkflowPlugin` 仍只有类型占位没有执行契约，`ReviewPlugin` 还没有真正接入 Layer 2 审查扩展面，TypeScript 侧插件 SDK 与 `create-plugin` 脚手架不存在，插件分发/签名/审核/市场只停留在文档蓝图。

现在需要把这些剩余能力收敛成一条连续、可实施的 OpenSpec 变更，避免插件系统长期停留在“部分类型可注册、部分宿主可执行、生态能力缺席”的半成品状态，也让后续 `/opsx:apply` 能沿着一个完整而非零散的计划推进。

## What Changes

- 为 `WorkflowPlugin` 增加正式执行契约，覆盖 manifest 结构、顺序编排最小执行面、角色引用、步骤输入输出、失败回退和运行态观测。
- 为 `ReviewPlugin` 增加正式扩展契约，覆盖插件注册、触发条件、规则声明、Layer 2 审查流水线挂载点、内置与自定义审查插件并行执行和结果聚合。
- 补齐插件开发者体验能力，定义 TypeScript 插件 SDK、扩展现有 Go WASM SDK 到 Integration/Workflow 双用途、提供 `create-plugin` 脚手架、样例模板与验证流程。
- 补齐插件分发与信任基础，定义本地之外的 Git/npm/registry 安装来源、签名与审核元数据、安装/更新/卸载流程，以及面向未来 Marketplace 的最小目录与发布契约。
- 扩展现有 `plugin-runtime` 与 `plugin-registry` 规格，使它们能覆盖工作流与审查插件、完整生命周期阶段（install/enable/activate/active/deactivate/disable/uninstall/update）以及运营可见的信任/来源状态。
- 将现有固定实现的 Layer 2 深度审查契约升级为“内置审查维度 + ReviewPlugin 扩展点”并存的模型，避免后续审查能力继续写死在桥接层。

## Capabilities

### New Capabilities
- `workflow-plugin-runtime`: 定义 Workflow 插件的 manifest、执行模式、步骤状态、失败补偿和最小可实施的顺序编排运行时。
- `review-plugin-support`: 定义 Review 插件的 manifest、触发条件、规则契约、执行结果模型以及与 Layer 2 审查流水线的挂载关系。
- `plugin-developer-experience`: 定义 TypeScript/Go 插件 SDK、`create-plugin` 脚手架、样例工程与开发验证流程。
- `plugin-distribution-and-trust`: 定义插件来源、发布、安装、更新、签名、审核和 Marketplace-ready 目录/发布契约。

### Modified Capabilities
- `plugin-runtime`: 从当前仅覆盖 Tool + Integration 的运行时路由，扩展为同时涵盖 Workflow/Review 的宿主归属、生命周期阶段和停用/更新语义。
- `plugin-registry`: 从当前仅覆盖内置/本地来源与基础运行态，同步扩展到多来源安装、信任元数据、发布目录和运营可见状态。
- `go-wasm-plugin-sdk`: 从当前以 Go 宿主 WASM Integration 为主的 SDK 契约，扩展到 Workflow 复用、打包约定和开发体验要求。
- `deep-review-pipeline`: 从当前固定四个审查维度的深度审查模型，扩展到支持内置维度与 ReviewPlugin 扩展点并行聚合。

## Impact

- Affected specs: new `workflow-plugin-runtime`, `review-plugin-support`, `plugin-developer-experience`, `plugin-distribution-and-trust`; modified `plugin-runtime`, `plugin-registry`, `go-wasm-plugin-sdk`, `deep-review-pipeline`
- Affected Go areas: `src-go/internal/plugin/*`, `src-go/internal/service/*`, plugin handler/repository models, workflow execution seams, review orchestration seams, `src-go/plugin-sdk-go`
- Affected TS areas: `src-bridge/src/plugins/*`, `src-bridge/src/review/*`, bridge install/activation/reporting seams, new TS SDK and scaffold packages/scripts
- Affected product surfaces: plugin install/manage flows, deep review execution path, workflow automation entrypoints, future plugin catalog/marketplace APIs and docs
- New dependency surface: plugin packaging/signature tooling, scaffold/template assets, multi-source plugin installation flow, and verification fixtures for workflow/review/plugin SDK scenarios
