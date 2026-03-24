## Context

AgentForge 已经通过 `plugin-runtime` 和 `plugin-registry` 基线 spec，把插件注册、TS ToolPlugin 运行时和 Go/TS 双宿主边界定了下来，但仓库里的 Go 侧执行面仍明显不完整。当前 `src-go/internal/service/plugin_service.go` 对 `IntegrationPlugin` 的 `Activate` 只是直接把状态标成 `active`，`CheckHealth` / `Restart` 也只支持 TS ToolPlugin；`src-go/internal/plugin/parser.go` 仍把 Go 侧执行插件约束在 `go-plugin` + `spec.binary`；仓库里也没有实际的 Go WASM runtime、SDK 或可验证样例插件。

PRD 和插件调研文档已经明确，Go 宿主长期方向是安全沙箱化的 WASM 插件，而不是继续停留在“仅注册、不执行”的占位层。这个变更要把那条设计线真正落到仓库里：Go 可以真实加载插件模块，插件作者有可用 SDK，注册表展示的是实际运行结果，而不是推断状态。

## Goals / Non-Goals

**Goals:**
- 为 Go 宿主引入真实的 WASM 插件加载、实例化、调用、健康检查和重启能力。
- 扩展插件 manifest 和运行时模型，使 Go 侧可声明和校验 `runtime: wasm` 所需的模块、ABI 和能力信息。
- 提供首版 Go WASM 插件 SDK，统一导出函数、宿主绑定、请求/响应 envelope 和错误语义。
- 让插件注册表持有来自真实运行时的 Go 侧状态，包括激活成功、初始化失败、健康退化和重启次数。
- 提供至少一个仓库内样例插件和验证路径，证明“注册 → 激活 → 调用 → 健康检查”全链路可运行。

**Non-Goals:**
- 不实现远程插件市场、签名验证、审核流或插件分发门户。
- 不重写 TS Bridge 的 ToolPlugin/MCP 运行时，只处理与 Go 侧注册表同步相关的最小改动。
- 不在本次中交付完整的 Workflow/Review 多类型执行引擎；首个落地对象以 Go 宿主可执行插件为主。
- 不引入 Extism、WIT Component Model 或跨语言统一 SDK 作为首版方案。
- 不把前端插件管理 UI 扩展成完整 Marketplace 体验。

## Decisions

### 1. Go 宿主首版真实运行时采用 `wazero`，并引入 `runtime: wasm`

本次变更会在 Go 侧引入新的 `wasm` 运行时声明，并使用 `wazero` 作为实际的模块执行引擎。之所以不直接走 Extism，是因为当前仓库更需要一个纯 Go、无 CGO、易于嵌入 sidecar/服务进程、且可精细控制宿主调用面的方案；之所以不继续沿用 `go-plugin`，是因为用户需求已经明确转向“真实 WASM 插件加载”，而现有仓库也没有 gRPC 子进程宿主实现基础。

备选方案：
- **Extism**：跨语言生态更友好，但会额外引入更厚的运行时抽象，当前实现面不需要这么大跨度。
- **HashiCorp go-plugin**：成熟，但方向不符，而且无法满足本次对 WASM 沙箱和 SDK 的目标。

### 2. PluginService 只保留编排职责，真实执行下沉到独立 Go WASM Runtime Manager

`PluginService` 负责注册表编排、启停入口和状态回写，但不直接承担模块加载细节。新增的 Go WASM Runtime Manager 负责：
- manifest 解析后的模块定位与校验
- `wazero` runtime 生命周期
- 插件实例表和活跃句柄管理
- 初始化、调用、健康检查与重启
- 向注册表回传真实运行状态

这样可以避免把执行细节继续堆进 `PluginService`，也让未来 Workflow/Review 等 Go 宿主插件复用同一个 runtime seam。

备选方案：
- **继续把激活/健康逻辑写进 `PluginService`**：短期快，但会让服务层同时承担注册表、运行时和实例管理三种职责。

### 3. WASM manifest 采用“显式模块 + ABI 版本 + 能力声明”契约

对于 `runtime: wasm` 的 Go 宿主插件，manifest 需要至少声明：
- 模块产物路径，例如 `spec.module`
- SDK/ABI 兼容版本，例如 `spec.abiVersion`
- 可选入口或能力声明，例如 `spec.capabilities` / `spec.operations`

模块路径相对于 manifest 解析，注册时会保存解析后的真实来源；激活前还会进行文件存在性和 ABI 兼容性校验。这样可以把“能否加载”和“加载哪个模块”的真相放进 manifest，而不是写死在服务代码里。

备选方案：
- **只靠约定文件名查找 `.wasm`**：实现简单，但不利于多产物插件和未来签名/校验。
- **把全部 ABI 细节藏到 SDK，不进 manifest**：会导致注册表和运维侧看不到兼容性信息。

### 4. SDK 使用稳定导出函数 + JSON Envelope，而不是直接暴露底层内存 ABI

首版 `go-wasm-plugin-sdk` 会把底层线性内存读写、host function 调用和错误编解码封装起来，对插件作者暴露稳定的 Go 接口。宿主与插件之间采用版本化 JSON envelope 传递 `Describe`、`Init`、`Health` 和 kind-specific invocation 数据，SDK 负责生成导出函数和内存桥接层。

这样虽然会比自定义二进制协议多一些序列化开销，但在当前插件调用频率下可接受，换来的是更低的 SDK 使用门槛和更稳的 ABI 演进空间。

备选方案：
- **直接约定多组裸导出函数和内存布局**：运行时更薄，但插件作者体验差，升级脆弱。
- **直接上 WIT/Component Model**：长期方向更优雅，但超出本次范围。

### 5. 仓库内置样例插件必须走真实构建与激活链路

这次不会只写 SDK 文档或伪代码示例，而是必须提供一个仓库内可构建的 Go WASM 样例插件，并把内置插件 manifest 指向真实模块产物。验证标准不是“单元测试通过”，而是仓库内能完成：
1. 构建样例插件模块
2. 通过 manifest 注册
3. 由 Go runtime 激活
4. 完成至少一次健康检查或实际调用

备选方案：
- **只提供 SDK 包不带样例**：无法证明 ABI 和 runtime 真正打通。
- **只保留文档中的伪代码示例**：不能满足用户对“完整真实加载”的要求。

## Risks / Trade-offs

- [Risk] 新增 WASM toolchain 和 `wazero` 依赖会提高 Go 部分构建复杂度 -> Mitigation: 把插件构建和服务构建分离，提供独立脚本/命令与最小样例验证。
- [Risk] JSON envelope 增加序列化开销 -> Mitigation: 首版只覆盖低频控制面和集成事件，后续高频场景再评估二进制 ABI。
- [Risk] 插件实例如果持有脏状态，重启和健康检查会变复杂 -> Mitigation: runtime manager 统一管理实例生命周期，并把失败恢复逻辑集中到单处。
- [Risk] 现有 `go-plugin` manifest/样例与新 `wasm` 方向会产生过渡成本 -> Mitigation: 在本仓库先迁移内置样例，并在 parser / registry 中明确错误信息和迁移提示。
- [Risk] SDK 一旦暴露过多宿主能力会扩大安全面 -> Mitigation: 首版只开放日志、配置读取、结构化响应等必要 host bindings，权限继续由 manifest 管控。

## Migration Plan

1. 扩展插件模型与 parser，增加 `wasm` 运行时、模块字段、ABI 版本字段以及相应校验。
2. 在 Go 侧引入 WASM runtime manager，并把 `PluginService` 的激活、健康检查、重启逻辑改为委托真实运行时。
3. 实现 `go-wasm-plugin-sdk`，提供导出函数包装、host bindings、错误/结果 envelope 和最小构建脚本。
4. 增加仓库内样例插件与内置 manifest，打通注册、激活、健康检查与状态同步测试。
5. 在 handler/API 和测试层补齐回归验证，确保 registry 返回的状态来自真实运行时。

回滚策略：
- 如果 WASM runtime 集成出现阻塞，可保留 manifest/registry 兼容和 SDK 目录结构，但暂时关闭 `runtime: wasm` 的激活入口，让插件继续停留在已注册未激活状态。
- 如果样例插件构建链路不稳定，可先保留运行时与 SDK，实现仓库内 fixture 模块装载验证，再恢复完整样例构建。

## Open Questions

- 是否需要在本次中保留对旧 `go-plugin` manifest 的只读兼容，还是直接要求仓库内 Go 宿主可执行插件迁移到 `runtime: wasm`？
- 样例插件应直接替换现有 `plugins/integrations/feishu-adapter`，还是新增一个更聚焦运行时验证的示例插件目录？
