## Context

当前仓库的插件基础设施已经能解析并发现 manifest-backed built-ins，但真实内置集合仍然明显窄于项目文档承诺。Go 控制面的 built-in discovery 直接扫描 plugins 目录下的 manifest.yaml，当前仓库里只有 plugins/tools/web-search/manifest.yaml 和 plugins/integrations/feishu-adapter/manifest.yaml 两个真实 built-in 入口；ReviewPlugin 与 WorkflowPlugin 虽然已经有 parser、runtime、SDK、脚手架和测试样例，但 repo 并没有把它们作为官方内置插件随仓库交付。与此同时，PRD.md 和 PLUGIN_SYSTEM_DESIGN.md 已经把官方内置 ToolPlugin、1-2 个内置 ReviewPlugin，以及可运行的 workflow starter 写成产品叙述。

这意味着当前缺口不是再做一轮插件 runtime 或 registry 基础设施，而是补齐 官方内置插件包这层产品真相：哪些 built-ins 属于仓库交付面、它们的 manifest 和入口资产放在哪里、catalog 如何 truthfully 暴露它们、脚本如何验证它们，以及文档如何不再领先于仓库事实。另一个约束是 RolePlugin 已经通过 roles 目录和 role-plugin-support 单独演进，它虽然是插件类型的一部分，但不走当前 plugins 目录的 manifest-backed built-in discovery，因此不应和这次 change 混成一个大而散的内置资产整理任务。

## Goals / Non-Goals

**Goals:**
- 为官方内置插件建立单一 repo-owned 真相源，明确仓库实际随附的 built-in ToolPlugin、ReviewPlugin、WorkflowPlugin、IntegrationPlugin 集合。
- 让 built-in discovery、catalog install 和管理面板只暴露真实存在、可解析、可验证的官方 built-ins，并对缺少前置条件的条目返回 truthful availability。
- 把至少一条内置 ReviewPlugin 线和一条内置 Workflow starter 线从文档样例提升为真实 repo 资产，并纳入现有 lifecycle 与 provenance contract。
- 扩展现有 plugin build/debug/verify 工作流，使仓库维护的 built-in MCP review/tool plugins 与 workflow starter 都有稳定验证路径。
- 让项目文档中的官方内置插件示例、目录路径和仓库实际清单保持一致，避免后续再次漂移。

**Non-Goals:**
- 不在本次内扩展 remote registry、marketplace 发布、外部来源安装或新的信任模型；这些能力继续由现有 plugin-registry-baseline 等变更推进。
- 不把 RolePlugin 预设角色模板迁移进 plugins 目录，也不重做 role authoring、role registry 或 role marketplace。
- 不引入新的插件宿主类型或改变现有 kind/runtime 兼容矩阵；ToolPlugin 仍走 mcp，ReviewPlugin 仍走 mcp，WorkflowPlugin 和 IntegrationPlugin 仍走 Go-hosted wasm 路径。
- 不在本次内把所有文档中的示例插件一次性做满；先聚焦一组可运行、可验证、能代表文档承诺的官方 built-in bundle。

## Decisions

### 1. 用一个 repo-owned built-in bundle 清单作为官方内置插件真相源，而不是继续依赖扫到什么 manifest 就算什么 built-in

本次会增加一个显式的 built-in bundle 清单文件，记录官方随仓库交付的 plugin id、kind、manifest path、docs ref、verification profile 和 availability semantics。Go 控制面在 built-in discovery 和 catalog 组装时继续复用现有 manifest 扫描能力，但会把manifest 存在与属于官方 bundle区分开，只有 bundle 清单声明且 manifest 可解析的资产才算 docs-aligned official built-in。

这样做的原因：
- 仓库已经存在 parser fixtures、脚手架样例和未来可能新增的实验插件目录，仅靠扫描所有 manifest 容易把样例误当成产品交付面。
- 文档一致性需要一个能被验证的真相源，不能继续靠 PRD 文字和目录偶然同步。
- 后续若某个 built-in 暂时保留源码但从官方 bundle 下线，只改 bundle 清单即可，不需要破坏开发样例目录。

备选方案：
- 继续按目录约定推断官方 built-ins。缺点是无法稳定区分 maintained built-ins、实验样例和测试资产。
- 直接把 docs 当真相源。缺点是无法驱动 catalog/discovery，也不能做 CI 级 drift 校验。

### 2. 本次 bundle 只覆盖 manifest-backed plugins，Role 预设模板继续留在 roles 目录单独治理

官方 built-in bundle 会覆盖当前 plugins 目录语义下的 ToolPlugin、ReviewPlugin、WorkflowPlugin、IntegrationPlugin。现有 roles 目录中的内置角色模板仍由 role-plugin-support 与 role authoring 流程负责，不并入本次 bundle 清单。

这样做的原因：
- 当前 Go 插件服务的 built-in discovery 与 plugin catalog 都围绕 manifest-backed plugins 工作，Role YAML 不共享同一安装、启停、catalog 入口。
- 如果本次把 roles 也并进来，会把一个插件产品完整性 change 扩成 role marketplace/authoring 重整，范围会明显失真。
- 文档对 RolePlugin 的承诺已经有独立实现面，真正缺的是 review/workflow/tool 这类 manifest-backed official built-ins。

备选方案：
- 把 Role 预设模板也纳入 built-in bundle。优点是所有内置插件概念更统一；缺点是需要同时重构 role list/get/create path 与 plugin catalog contract，超出本次 focused scope。

### 3. 先交付一组最小但文档对齐的官方 built-ins，而不是追求所有示例一次落地

设计上的最小官方 bundle 是：保留现有 web-search 和 feishu-adapter，新增 docs-aligned GitHub 或数据库类 ToolPlugin、1-2 个 built-in ReviewPlugin（至少包含 architecture-check）、以及一个可执行的 sequential workflow starter（标准开发流或等价 starter）。这些条目都必须有真实 manifest、入口资产和 verification path，不能只补 spec 名称。

这样做的原因：
- 这组 built-ins 已经直接出现在 PRD 和 PLUGIN_SYSTEM_DESIGN 的叙述中，最能代表文档承诺 vs repo 真相的差距。
- 先交付最小 bundle 可以让 built-in discovery、catalog、verify 和 docs 对齐闭环跑通，再决定是否追加更多官方插件。
- Workflow starter 只要求 sequential mode，可以复用现有 runtime contract，而不会把 hierarchical/event-driven 一并拉进来。

备选方案：
- 只新增 review plugins，不碰 tool/workflow。缺点是仍然无法解释文档中的 workflow starter 与多内置工具示例。
- 一次补全所有文档插件示例。缺点是实现面过宽，验证成本和 drift 风险都过高。

### 4. Catalog 和管理面板用 truthful availability 表达 built-in 状态，而不是隐藏不够可运行的官方插件

官方 built-in bundle 中的条目即使依赖额外前置条件，也应在 catalog 或 built-in discovery 中以明确状态出现，例如 installable、requires local prerequisite、requires secret configuration、temporarily unavailable。只有文档样例或未进入官方 bundle 的资产才应被排除出 operator-facing built-in 列表。

这样做的原因：
- 用户请求明确要求功能完整并符合项目文档，隐藏条目会继续让 operator-facing 视图与文档脱节。
- 现有 plugin-management-panel 和 plugin-catalog-feed 已经强调 truthful source-aware flows，这次只是在 built-in bundle 维度把 truthfulness 做实。
- 对于 GitHub、DB 这类 ToolPlugin，真实问题通常是前置条件，而不是资产不存在；状态表达比静默缺席更符合产品语义。

备选方案：
- 仅在完全可执行时展示 built-ins。缺点是 operator 无法理解为什么文档写了但产品看不到。

### 5. 仓库验证按 plugin family 分层，而不是依赖一个全量大而脆的 built-in smoke

repo-owned validation 会分成 manifest/schema validation、targeted build or package validation、family-specific debug or smoke checks 三层。Go-hosted wasm plugins、MCP tool plugins、MCP review plugins、workflow starter 走各自最小验证路径；需要网络或凭据的 live smoke 保持 opt-in，本地和 CI 默认使用 fixture 或 bounded readiness checks。

这样做的原因：
- 当前 plugin-development-scripts 已经证明单个 Go sample的 build/debug/verify 可维护，但跨 host 的 built-ins 如果都绑进一个 live smoke，很容易被外部环境噪音拖垮。
- 用户接受 focused verification，只要边界真实可复现；分层验证更符合这类 repo 的实际维护方式。
- 这也便于后续新增官方 built-ins 时按 family 追加验证，而不是不断膨胀一个超大脚本。

备选方案：
- 为所有 built-ins 提供统一全量 live smoke。优点是表面简单；缺点是网络、凭据和第三方二进制会让 CI 与本地都非常脆弱。

## Risks / Trade-offs

- Built-in bundle 清单新增了一层元数据维护成本 -> 用 verification 检查 bundle、manifest、docs 引用三者的一致性，防止再次漂移。
- MCP 类型 built-ins 可能依赖外部二进制、网络或密钥 -> 默认验证采用 fixture 或 prerequisite check，live smoke 保持显式 opt-in。
- Workflow starter 会把 role id 和 step routing 的真实问题暴露出来 -> starter 只允许引用现有 roles 目录里已存在的 role ids，并复用现有 sequential runner 校验。
- 这条 change 与 active 的 plugin-registry-baseline 都会触碰 catalog/panel 真相 -> 通过 bundle metadata 和 built-in availability 聚焦当前 seam，不重复定义 remote registry browse/install contract。

## Migration Plan

1. 新增 built-in bundle 清单，并把现有 official built-ins（至少 web-search、feishu-adapter）纳入真相源。
2. 为新增的 review/tool/workflow official built-ins 落真实目录、manifest、入口资产和最小验证脚本。
3. 扩展 control-plane discovery 和 catalog 组装逻辑，使其读取 bundle metadata 并输出 truthful availability。
4. 更新 plugin docs/examples 与 focused verification，使文档路径、bundle 清单和仓库资产保持一致。
5. 如果某个新增 built-in 在验证中暴露不可接受风险，可先把它从 bundle 清单下线并保持相关 runtime contract 不变，作为最小回滚路径。

## Open Questions

- GitHub 或数据库类官方 ToolPlugin 应该直接依赖第三方 MCP binary，还是提供 repo-local wrapper/fake mode 以减少验证波动？
- 第二个官方 built-in ReviewPlugin 应该优先落性能分析还是规范合规，以更贴近当前产品优先级和现有审查数据模型？
