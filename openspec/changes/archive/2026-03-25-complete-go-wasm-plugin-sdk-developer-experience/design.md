## Context

AgentForge 当前已经有一条可工作的 Go-hosted WASM 插件最小链路：`src-go/internal/plugin/runtime.go` 通过 `wazero` + WASI 环境变量驱动模块执行，`src-go/plugin-sdk-go/sdk.go` 提供一个极简 `Runtime` 包装，`src-go/cmd/sample-wasm-plugin` 和 `scripts/build-go-wasm-plugin.js` 则证明仓库可以构建并激活样例插件。但对照 `docs/PRD.md`、`docs/part/PLUGIN_SYSTEM_DESIGN.md` 和主规格 `openspec/specs/go-wasm-plugin-sdk/spec.md`，当前 SDK 仍明显不完整：

- 作者接口仍以 `map[string]any` 为主，缺少 manifest-aligned 的 typed descriptor、capability metadata、invocation context、result/error helper。
- 样例插件必须自己声明导出函数和 envelope 编排逻辑，SDK 没有把这些稳定 ABI 细节封装成可复用 helper。
- `Context` 只暴露配置访问和日志，缺少当前调用 operation、运行时约束、宿主允许能力等受边界保护的执行上下文。
- 样例、构建脚本、文档与主规格之间没有被统一验证，导致“规格说得更丰富、SDK 暴露得更少”的漂移已经发生。

这个 focused change 的目标不是重新设计整个插件系统，而是把 Go SDK 这一条作者体验链路补成一个可持续维护的闭环，并且不与正在进行中的更宽 `complete-plugin-system-foundation` umbrella 互相纠缠。

## Goals / Non-Goals

**Goals:**
- 在不推翻当前 `runtime: wasm` + `wazero` + WASI env transport 的前提下，把 Go SDK 升级为稳定的 typed authoring contract。
- 为 Go-hosted 插件补齐结构化 descriptor、capability、execution context、result/error helper 和统一导出辅助，减少模板样板代码。
- 把仓库内样例/模板、构建辅助、验证入口和文档说明收敛为一条受支持的作者工作流。
- 让主规格、SDK 暴露面、样例插件和验证策略重新对齐，避免后续 Go-hosted plugin 演进继续建立在漂移基础上。

**Non-Goals:**
- 不把底层 transport 从当前 JSON envelope + WASI env 改成更激进的线性内存 ABI 或 Extism host functions。
- 不在本次中引入 TypeScript SDK、公开 Marketplace、签名分发系统或完整 `create-plugin` 全量脚手架。
- 不在本次中把 WorkflowPlugin 全量实现到可执行状态；这里只保证 Go SDK 对未来 Go-hosted 扩展的契约可复用。
- 不重写整个 Go runtime manager；宿主执行路径只做为 SDK authoring contract 提供必要适配。

## Decisions

### 1. 保留当前宿主 transport，向上补 typed authoring layer，而不是重做底层 ABI

本次不会把 Go-hosted 插件协议从 `AGENTFORGE_*` 环境变量 + stdout/stderr JSON envelope 改成新的底层 ABI。当前 runtime 已经真实工作，风险更大的问题在于作者侧 contract 过薄，而不是宿主无法执行。设计会在 SDK 上层增加显式的 typed request/response、descriptor、capability 和 context helper，同时保持宿主看到的 ABI export 仍是 `agentforge_abi_version` 与 `agentforge_run`。

备选方案：
- 直接切到线性内存 ABI / host function bridge。优点是长期更强；缺点是 apply 复杂度大，会同时改 runtime、sample、tests。
- 保持现状不变。缺点是作者仍需手写大量样板，规格与代码持续漂移。

### 2. SDK 以 “descriptor + runtime helper + bounded context” 三层收敛

新的 Go SDK 需要同时解决“作者如何描述插件”“运行时如何导出稳定 ABI”“插件执行时可读取哪些宿主信息”三个问题，因此会拆成三层：
- `Descriptor`/`Capability`/manifest-aligned typed types：统一插件元数据、能力声明和支持操作的表达。
- `Runtime`/`Exports` helper：统一 `agentforge_abi_version`、`agentforge_run`、autorun 入口与 envelope 编排。
- `Context`/`Invocation` helper：暴露当前 operation、配置视图、运行时约束和结构化日志/结果/错误接口，但不泄露任意宿主内部对象。

备选方案：
- 继续让插件只返回 `map[string]any`。缺点是 IDE/编译期反馈差，也很难把样例和模板做稳。
- 直接为每类插件定义完全分叉接口。缺点是当前 repo 只落地 Go-hosted/WASM 一条线，过早分叉会带来重复维护。

### 3. 导出函数和 envelope 由 SDK 统一生成，样例和模板只负责业务逻辑

样例插件目前要自己写 `//go:wasmexport agentforge_abi_version`、`//go:wasmexport agentforge_run` 以及 `main()` 中的 autorun 逻辑，这会让每个作者重复接触稳定 ABI 细节。设计会把这些细节收敛成统一 helper 或模板约定，让样例/模板文件只保留业务实现、descriptor 和 operation handler，导出逻辑由 SDK 复用。这样 future templates 更容易保持一致，也更适合纳入仓库验证。

备选方案：
- 保留手写导出并只在文档里说明。缺点是漂移与低级错误会频繁重复出现。

### 4. 新 capability 单独定义“作者工作流”，避免把 DX 要求埋进实现备注

主规格 `go-wasm-plugin-sdk` 负责 SDK contract 本身，但构建辅助、样例/模板、验证入口、文档对齐属于仓库级开发者体验要求，单靠修改 SDK spec 会把“功能契约”和“维护约束”混在一起。因此新增 `go-plugin-sdk-authoring-workflow` capability，专门定义支持的样例/模板、build helper、repo verification 和文档真相要求。

备选方案：
- 全部塞进 `go-wasm-plugin-sdk`。缺点是 requirement 过载，不利于后续单独演进作者工作流。
- 并入 `plugin-developer-experience`。缺点是当前主 specs 里还没有这个 capability，本次 focused change 也不想扩大到 TS SDK 和全量 scaffold。

### 5. 验证策略以仓库内 sample/template 真相为中心，而不是只靠 README 命令

SDK 完整性不能只靠文档存在来证明。设计要求仓库维护的 Go sample 或 template 至少被以下链路验证：构建出 `.wasm` artifact、manifest 能通过 parser/registry 校验、runtime 能激活并调用至少一个声明 capability、SDK typed helper 的关键行为有聚焦测试。这样 future SDK 变更一旦破坏模板或 build helper，就能在 repo verification 阶段暴露出来。

备选方案：
- 只保留 sample build 脚本。缺点是无法保证 manifest/runtime/SDK 三方仍对齐。

## Risks / Trade-offs

- [保留当前 env/envelope transport 会限制短期 ABI 演进空间] -> 先把作者契约补齐，避免一次 change 同时重构 transport 和 DX；后续如需升级 ABI，再在已收敛的 SDK 外层平滑演进。
- [新增 typed helper 可能与现有 sample/runtime 的 map-based 路径并存一段时间] -> 通过统一模板和 targeted tests 逐步收敛，避免一次性强行删除所有旧辅助函数。
- [focused change 和 umbrella change 都触及 Go SDK 语义] -> 在 proposal 和 tasks 中明确本次只做 Go SDK authoring seam，不扩到 TS SDK、ReviewPlugin、distribution/trust 等其它 tracks。
- [文档中仍保留更宏大的 Go Plugin SDK 示例] -> 本次把 repo-supported truth 写清楚，必要时在文档里显式标注已支持与未支持边界，避免用户误判功能已完整交付。

## Migration Plan

1. 先更新 OpenSpec：新增 `go-plugin-sdk-authoring-workflow`，并修改 `go-wasm-plugin-sdk` 以明确新的 typed contract 和作者工作流要求。
2. 在 `src-go/plugin-sdk-go` 中补齐 typed descriptor/context/result/error/export helpers，同时保持当前 runtime 可兼容运行现有 sample。
3. 重构 sample/template 与构建辅助，使其改用新的 SDK helper，并补齐 manifest/runtime 对齐验证。
4. 增加 focused tests 和 repo verification，覆盖 SDK helper、sample build、manifest 校验和 runtime 激活路径。
5. 最后同步 `docs/GO_WASM_PLUGIN_RUNTIME.md` 与相关 PRD/设计文档引用，写清当前支持边界与推荐作者流程。

回滚策略：
- 如果 typed helper 收敛过程中影响现有 sample 激活，可临时保留兼容 wrapper，让 runtime 继续接受旧 envelope 形状，再逐步迁移 sample/template。
- 如果样例/模板验证链路阻塞 wider repo checks，可先保持 focused verification 命令和脚本独立，不影响当前其他并行工作线。

## Open Questions

- 仓库是维护一个“sample plugin”即可，还是同时维护一个更贴近未来脚手架输出的 template fixture？
- 执行上下文中是否需要在本次就暴露 deadline/request id 等字段，还是先聚焦 operation/config/capability 范围？
- 文档中的 Go SDK 示例路径是否要同步切换到未来公开模块名，还是继续以 repo 内模块路径作为当前真相？
