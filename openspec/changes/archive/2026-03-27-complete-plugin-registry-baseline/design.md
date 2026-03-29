## Context

`complete-plugin-system-foundation`、`complete-go-plugin-control-plane` 与 `complete-plugin-operator-console` 已经把 AgentForge 的插件系统推进到“本地 catalog + source-aware install + trust gate + operator console”的阶段，但文档里单独承诺的 `Plugin Registry` 基础版仍未落地。当前仓库里最接近这条能力的真相是：

- `src-go/internal/service/plugin_service.go` 已经定义了 `RemoteRegistryClient`、`ListRemotePlugins()`、`InstallFromRemote()` 和对应路由，但没有默认 client 接入，也没有把远程 registry 状态转换成稳定的 operator-facing 合同。
- 当前插件页只消费 `/api/v1/plugins/marketplace`、`/api/v1/plugins/catalog` 和本地安装流，没有任何远程 registry UI 或 store action 接到 `/plugins/marketplace/remote`、`install-remote`。
- 现有 marketplace 区块仍以 catalog/browse-only 为主，符合此前 focused change 的边界，但还不等同于文档中的 `Plugin Registry 基础版`、远程 artifact 拉取器和可运营远程市场入口。
- 当前 trust/contracts 已覆盖 digest、signature、approval 与 lifecycle gate，因此新的 remote seam 必须复用现有信任语义，而不是绕过它们另造一套安装路径。

这意味着本次设计不是“再做一个更大的插件系统”，而是把已经预留但未闭环的远程 registry seam 变成一个最小可运行、可验证、不会漂移的能力层。

## Goals / Non-Goals

**Goals:**
- 定义远程 `Plugin Registry` 的最小可运行合同，覆盖 registry 列表读取、远程条目元数据、artifact 拉取与显式安装入口。
- 让 Go 控制面把远程 registry 错误从“client 未配置”转成稳定、可观测的 operator-facing 结果，包括 source reachability、空目录、下载失败与验证失败。
- 让远程安装复用现有 `plugin-distribution-and-trust` 路径，确保远程 artifact 同样经过 digest、signature、approval 和 lifecycle gate。
- 扩展插件管理控制台，使其能区分本地 catalog、已安装插件与远程 registry 条目，并对可安装/不可安装/不可达状态给出 truthful UI。
- 为后续更完整的 `plugins.agentforge.dev`、评分评论、发布门户或 OCI 化演进保留升级空间，但本次先定义基础版闭环。

**Non-Goals:**
- 不在本次内实现公开多租户运营后台、评分评论系统、开发者账号体系或完整 Web 发布门户。
- 不在本次内引入真正的 OCI registry、S3/MinIO 制品仓库、自动恶意扫描流水线或签名 CA 基础设施；这些保持为后续可替换实现。
- 不改变现有插件 runtime 宿主映射、workflow/review/runtime contracts、桌面能力或本地 scaffold/SDK 范围。
- 不把插件业务主链路从 Go 控制面迁移到 Tauri、TS bridge 或独立 marketplace 服务；远程 registry 仍由 Go control plane 对外收敛。

## Decisions

### 1. 用一个显式的 remote registry adapter 接到现有 `RemoteRegistryClient`，而不是把远程逻辑散落到 handler/store

Go 侧已经有 `RemoteRegistryClient` 抽象和两个对外方法，这说明当前最佳落点不是重开新 service，而是补一个明确的 adapter/implementation，并在 `PluginService` 初始化时注入配置化 registry URL 与 client。这样可以让：

- handler 保持简单，只消费稳定的 service 返回值。
- 远程 registry 的 fetch/download、错误分类、认证头或 future pagination 都封装在一个接缝内。
- 测试可以直接 mock client，也可以对 adapter 做独立 contract test。

备选方案：
- 直接在 `PluginService` 里用 `http.Client` 拼请求。缺点是会把 transport 细节和 install/trust 语义耦合在一起。
- 让前端直连远程 registry。缺点是会绕过 Go trust gate 和 source normalization，破坏当前 control-plane 架构。

### 2. 远程 marketplace 条目使用单独 capability 建模，但仍落到现有 normalized source model

新增 `plugin-registry-marketplace` capability，用于定义远程 registry browse/install contract；真正安装进 registry 后，记录仍继续复用现有 `PluginSource` 规范，标记为 `catalog` 或 `registry` 来源并保留 registry URL、entry、version、digest、signature、approval、release 元数据。

这样做的原因：
- 远程 browse/install 本身是一个新的 operator-facing 能力，值得独立 spec，而不是硬塞进已有本地 catalog spec。
- 安装后的 plugin record 仍应保持统一 source model，否则 installed state 会再次分叉。
- 这让未来从“基础版远程 registry”升级到真正 OCI/多源 registry 时，无需推翻当前 registry record 结构。

备选方案：
- 完全复用 `plugin-catalog-feed`，把 remote registry 当 catalog 的一种。缺点是会混淆“本地 manifest 发现”和“远程源读取”，错误语义与可用性状态也不同。
- 新建完全独立的数据模型。缺点是与现有 installed/plugin-management contracts 冲突。

### 3. 远程安装继续走现有 install/trust pipeline，不允许 remote route 成为 bypass path

`InstallFromRemote()` 应只负责下载并准备 artifact，再把结果交给现有 install/verification 流；trust、approval、enablement 规则仍由统一的 distribution/control-plane 逻辑裁决。远程安装失败必须明确区分：

- registry 不可达或返回无效目录
- 下载失败或 artifact 缺失
- manifest 解析失败
- digest/signature 不匹配
- approval/trust gate 未满足

这样做的原因：
- 文档虽然要求远程市场与 artifact 拉取器，但没有要求远程源拥有更弱的信任标准。
- 当前系统已经有 operator-visible trust state，如果 remote route 绕过它，会立即造成 installed panel 语义漂移。
- 统一安装管线更利于把后续 npm/git/OCI 源都收敛到相同 lifecycle gate。

备选方案：
- 对 remote source 直接安装并默认为 verified。缺点是破坏现有安全模型。
- 为 remote source 再定义一套 trust 状态。缺点是 UI 和 control plane 会被迫双轨运行。

### 4. 插件控制台把 remote registry 视为单独 source section，并显式呈现 reachability 与 blocked state

前端不应把 remote registry 条目塞进现有“marketplace”卡片后继续依赖隐式 installability。控制台应至少区分：

- installed
- built-in / local catalog
- remote registry marketplace

对于 remote registry section，需要有：
- browse/filter/search 结果
- registry source label 和 version/source metadata
- reachability/error banner
- install CTA、blocked reason、in-progress / failed state

这样做的原因：
- operator 需要区分“本机可发现内容”和“远程市场内容”，否则 troubleshooting 和来源判断不真实。
- 当前文档承诺的是 `Plugin Registry` 基础版，而不是一个静态列表；source health 是这个 seam 的一部分。
- UI 只消费 Go 提供的 normalized result，避免自己推断 remote trust/install semantics。

备选方案：
- 继续沿用现有 marketplace 区块，只加一个按钮。缺点是来源语义与错误状态不清楚，无法满足文档要求的 remote market 基础版。

## Risks / Trade-offs

- [Remote registry contract may drift before a real hosted service exists] → 先把 adapter contract 固定在最小字段集和错误语义上，避免提前承诺完整 OCI/评分评论协议。
- [Remote artifact packaging may differ from local manifest-only install assumptions] → 在 spec 中把“基础版 artifact contract”写清楚，要求下载结果必须能转换成当前 install pipeline 可消费的 manifest/package 结构。
- [UI complexity in the plugin page may grow again] → 将 remote registry section 设计成 source-specific extension，复用现有 store/detail components，而不是再开一套平行管理页。
- [Trust failures may look like generic install failures without careful error typing] → 要求 control plane 返回稳定的 failure categories，并在 panel 中显示 source reachability、verification 和 approval 阶段的差异。

## Migration Plan

1. 在 Go 控制面引入可配置的 remote registry adapter 和基础配置项，但默认保持可关闭或未配置状态。
2. 先补 service/handler contract 和测试，确保未配置、不可达、空目录、下载失败、验证失败、成功安装等路径都返回稳定语义。
3. 再扩展前端 store 与插件页，把 remote registry section、install CTA、error state 和 install result 接到现有 operator console。
4. 用 focused verification 覆盖 Go remote flows 和插件页 remote section，不要求一次拉通更宽的插件/runtime 全量验证。
5. 如果线上或本地 registry 接入不稳定，可回退到“remote registry disabled”状态，但保留显式 section/状态与配置接缝，避免再次退回隐式占位接口。

## Open Questions

- 基础版 remote registry 下载返回的是单 manifest、压缩包还是 manifest + artifact 元数据组合？本次 spec 应固定一个最小可验证格式。
- 是否需要在基础版就支持 registry authentication header/token，还是先以匿名只读 registry 为准？
- remote registry entries 安装后应统一记录为 `catalog` 还是新增更明确的 `registry` source subtype；如果新增 subtype，哪些现有 DTO/store 枚举需要同步？