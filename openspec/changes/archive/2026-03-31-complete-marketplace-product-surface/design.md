## Context

AgentForge 当前已经有一条名义上的 marketplace 产品链路，但它分散在多个并未真正闭环的 seam 上：

- 前端 `app/(dashboard)/marketplace/page.tsx` 与 `components/marketplace/*` 只覆盖基础 browse、详情、发布对话框、评价和安装确认，没有版本上传、审核/精选、作者管理、失败态治理，也没有把“已安装”和“已使用”区分成真实状态。
- `lib/stores/marketplace-store.ts` 同时依赖 `src-marketplace` 与 `src-go`，但只把已安装状态建模成 `Set<string>`，无法表达类型、消费位置、安装警告、版本或后续使用情况；多个读取动作还是 silent failure。
- `src-marketplace` 已经提供 item CRUD、version upload、review、admin feature/verify 等端点，但前端几乎没有消费 version/admin 作者链路。
- `src-go/internal/handler/marketplace_handler.go` 只真正把 marketplace 安装接到 plugin service；skill 和 role 目前只是把 artifact 落盘并留下注释“future implementation”，这与 README / PRD 中“插件、技能、角色均可发布、发现、安装”的表述不一致。
- 现有 repo 已经有真实的侧载/本地安装模型，但它们散落在插件控制面：`POST /api/v1/plugins/install` 支持 local path 与受支持 source，`POST /api/v1/plugins/catalog/install` 支持 catalog install，`components/plugins/plugin-install-dialog.tsx` 已经有文件选择和本地路径输入。
- 部署与运行时 contract 当前存在明显冲突：`src-marketplace` 默认端口是 `7779`，而 IM Bridge 的 `NOTIFY_PORT`、Tauri `IM_BRIDGE_PORT`、`dev:all` 文档与 runtime 也都占用 `7779`。这使“独立部署的 marketplace 服务”和“现有整栈运行”无法同时成立。
- role/skill 的真实消费 seam 已经存在但未接入 marketplace：角色工作区通过 `lib/stores/role-store.ts` 调 `GET /api/v1/roles` 和 `GET /api/v1/roles/skills`，后者由 `src-go/internal/handler/role_handler.go` 从 repo-local roles/skills 根目录发现内容。

这次 change 是跨前端、Go marketplace 服务、Go 主后端、以及角色/插件消费 seam 的 cross-cutting change，需要在 implementation 前先把权责和运行模型收紧。

## Goals / Non-Goals

**Goals:**

- 让 `/marketplace` 成为真实完整的产品工作区，而不是只会 browse 的演示页。
- 为 plugin、skill、role 三类 marketplace 条目定义真实可测试的“发布 -> 版本 -> 审核/精选 -> 安装 -> 已安装/已使用”闭环。
- 把 marketplace 与现有 repo 的侧载模型对齐，明确支持的本地导入/导出/安装方式，而不是再造一套平行协议。
- 让 `src-marketplace` 可以作为独立 Go 微服务稳定部署，并在 Web 模式、桌面模式、以及本地联调里都拥有不冲突的 runtime contract。
- 确保 marketplace 产物会进入现有产品面被实际消费：plugin 进入插件控制面，role 进入角色目录，skill 进入角色技能目录/选择器。

**Non-Goals:**

- 不在本次中引入新的公开云 marketplace、计费、支付、结算或 SaaS 多租户模型。
- 不重做现有 remote registry / plugin registry architecture；公开远程 registry 仍沿用既有 `plugin-registry-marketplace` 能力。
- 不引入全新的全局 skill runtime 或独立 role runtime，只把 marketplace 安装结果接入现有 repo-truthful consumer surfaces。
- 不把现有插件信任模型整体改写成新的签名体系；外部 source / trust gate 继续沿用既有 plugin distribution 规则。

## Decisions

### 1. Marketplace 元数据与“实际消费”分离建模

`src-marketplace` 继续作为 marketplace 条目、版本、评价、审核状态与 artifact 分发的权威来源；`src-go` 继续作为 repo-local plugin / role / skill 消费与安装状态的权威来源。Marketplace 前端不再用单纯的 `installedItemIds: Set<string>` 推测状态，而是改为消费一个 typed marketplace consumption contract，至少覆盖：

- item id / type
- selected version
- install state / warning / failure reason
- consumer surface（plugin / role / skill）
- installed record identity or local path/provenance
- “installed” 与 “used” 的区分

这样可以避免让 `src-marketplace` 直接操纵 repo-local roles/skills/plugins 目录，维持当前产品的责任边界。

备选方案 A：让 `src-marketplace` 直接负责把 artifact 安装到 repo 目录。拒绝原因：它会绕过 `src-go` 已有的 plugin trust / lifecycle / role catalog seam，形成双写 source of truth。  
备选方案 B：继续只返回字符串 item ID 集合。拒绝原因：它无法支持 skills/roles，也无法表达安装警告、版本、消费位置和已使用状态。

### 2. 为 plugin / skill / role 引入分类型安装适配器，而不是只下载 artifact

安装桥接需要显式按条目类型分流：

- `plugin`：继续复用 `PluginService` 与现有 provenance/trust 模型，把 marketplace 安装记录纳入插件控制面。
- `role`：将 artifact 解包或投影进现有 role 存储根，并刷新 `GET /api/v1/roles` 可见目录，使其在 `app/(dashboard)/roles/page.tsx` 中可见。
- `skill`：将 artifact 解包或投影进现有 `roles` 邻接 skills 根，使其进入 `GET /api/v1/roles/skills` 的权威 catalog，并在角色工作区 skill selector 中可选。

安装成功的定义不再是“artifact 已下载到磁盘”，而是“对应 consumer surface 已能发现并使用该资产”。

备选方案 A：保留 skills/roles 的“download only”语义。拒绝原因：这正是当前文档与实现漂移的核心，不满足“确保功能都被使用”。  
备选方案 B：把 skills/roles 当 plugin 的子类型强行走 plugin registry。拒绝原因：仓库已有角色与技能独立的 authoring/catalog seam，强行混入 plugin registry 会放大范围并破坏现有边界。

### 2.5 Role 与 skill artifact contract 冻结为 canonical zip package，而不是任意原始文件

为避免 role/skill 安装在 apply 阶段继续猜测包格式，本次冻结如下 contract：

- `role` marketplace version artifact 必须是一个 zip archive。
- archive 解压后的包根必须包含 `role.yaml`，其内容必须满足当前 repo 的 canonical role manifest 规则。
- 安装时系统将该包物化到 `roles/<role-id>/`，并以 `roles/<role-id>/role.yaml` 作为权威发现入口；`role-id` 默认取 marketplace item slug，若 `role.yaml` 内 metadata.id 存在且与 slug 不一致，则安装必须失败而不是静默改名。
- `skill` marketplace version artifact 也必须是一个 zip archive。
- archive 解压后的包根必须包含 `SKILL.md`，并允许附带 `references/`、`scripts/`、`assets/`、子技能目录等 repo-local skill package 结构。
- 安装时系统将该包物化到 `skills/<skill-id>/`，并要求 `DiscoverSkillCatalog(...)` 能从 `skills/<skill-id>/SKILL.md` 发现该条目；`skill-id` 默认取 marketplace item slug，若解压结果需要依赖其他顶层布局才成立，则安装必须失败。

这保持了 marketplace 分发与当前 repo-local consumer seam 一致：role 仍是 `roles/<id>/role.yaml`，skill 仍是 `skills/<id>/SKILL.md`，只是通过 zip archive 作为传输格式。

备选方案 A：允许任意 tar/zip/raw file 并在安装端自动猜测。拒绝原因：Windows/桌面/浏览器联调下调试成本太高，且错误分类会漂移。  
备选方案 B：要求 role/skill 直接上传单文件 `role.yaml` / `SKILL.md`。拒绝原因：会丢失 skill package 的 companion files，也不适合 role 附加资产或后续扩展。

### 3. 侧载采用“复用现有 source model”的定义，而不是新发明 marketplace-only 协议

本次把“侧载”定义为两类受支持流：

- 作者侧载：授权用户可从本地文件、repo-local path 或受支持 artifact 上传生成 marketplace draft/version，走 marketplace service 的 publish/version contract。
- 操作员侧载：在 marketplace 工作区内复用现有 local path / catalog / supported external source install seam，把私有或尚未公开发布的资产导入本机环境，并保留来源元数据。

这意味着 marketplace 页不会重新发明一套本地导入协议，而是复用现有插件安装对话框、平台文件选择能力，以及统一 source/provenance 模型；对 roles/skills 则提供与其 consumer seam 一致的导入语义。

备选方案 A：把侧载限定为 `/plugins` 页已有的 local install，不在 marketplace 中体现。拒绝原因：用户要求市场支持侧载，且现有市场页需要承载完整产品能力。  
备选方案 B：要求所有侧载都必须先 publish 到 marketplace 再 install。拒绝原因：这会把本地私有导入路径强制变成服务端 round-trip，不符合现有 repo 的本地 authoring/dev 模式。

### 4. `/marketplace` 需要补齐版本与审核运营面，而不是只保留 API 存在

前端工作区应把已有后端 contract 用起来，而不是继续停在“可发布 item 元数据”的浅层：

- 作者可在 item 详情或作者工作区上传/查看/yank 版本。
- 管理员可执行 verify / feature，并看到审核状态。
- 条目详情可显示版本状态、审核状态、可安装性、消费状态、告警与 downstream deep link。
- 空态/错误态/不可安装态必须是 operator-facing 文案，而不是 silent failure 或 generic toast。

备选方案：保留作者管理与审核为纯 API-only 能力。拒绝原因：这会让 `/marketplace` 继续只是一张目录页，不符合“确保前端都完整的界面实现”。

### 5. 独立部署 contract 采用专用 marketplace 端口，并与现有 Web/Desktop runtime 解耦

`src-marketplace` 必须拥有独立端口与稳定配置契约。推荐将默认 marketplace 服务端口迁移到 `7781`，并同步：

- `src-marketplace/internal/config/config.go`
- `docker-compose.dev.yml`
- `NEXT_PUBLIC_MARKETPLACE_URL` / `MARKETPLACE_URL` 文档与默认值
- 本地联调、静态前端 separated mode、以及桌面模式下的 marketplace URL 注入方式

保留 `7777` 作为 Go orchestrator、`7778` 作为 TS bridge、`7779` 作为 IM bridge，可避免重写现有 Tauri 与 IM runtime 约定。

备选方案 A：继续让 marketplace 使用 `7779`。拒绝原因：它已与 IM Bridge 的真实 runtime contract 冲突。  
备选方案 B：改动 IM Bridge 端口为 marketplace 让路。拒绝原因：IM Bridge、Tauri runtime、`dev:all` 文档和测试都已经围绕 `7779` / `7780` 成型，改动面更大且与本 change 主目标无关。

## Risks / Trade-offs

- [Marketplace 安装桥接需要触达 plugin、role、skill 三类 consumer seam，范围横跨多个模块] → Mitigation: 保持“marketplace metadata”与“consumer install/use”分层，按类型实现独立 adapter 与 targeted tests。
- [role/skill artifact 结构可能不统一，安装时需要额外校验或解包规则] → Mitigation: 在 spec 中要求明确 artifact contract 和失败分类，先支持 repo-truthful manifest/package 形态，不承诺任意 archive 自动推断。
- [把 marketplace 状态从 `Set<string>` 升级为 typed contract 会影响现有前端 store 和组件 props] → Mitigation: 先集中在 `marketplace-store` 做归一化，组件只消费稳定 UI model。
- [端口迁移会影响已有本地环境和文档] → Mitigation: 提供兼容期 env override，允许旧环境通过显式 `MARKETPLACE_URL` 运行，但默认配置与文档统一切到新端口。
- [“确保功能都被使用”容易被扩张成重做所有下游工作区] → Mitigation: 本次只要求安装结果进入既有 consumer seam 并可被发现、选择、管理，不重做这些工作区的全部 UX。

## Migration Plan

1. 先在 spec 中固定新的 marketplace runtime contract、安装语义和 consumer handoff。
2. 为 `src-marketplace` 与 `src-go` 引入新的 typed consumption/install contract，并保持旧 `installed` endpoint 在短期内可兼容或可映射。
3. 将 marketplace 默认端口从 `7779` 迁移到新端口，并同步 compose、env、README、dev/runtime wiring。
4. 更新 `/marketplace` 前端工作区以消费新 contract，并逐步接上版本、审核、side-load 与 deep-link UX。
5. 最后补齐 plugin / role / skill consumer seam 的 targeted verification，确认安装结果在各自工作区真实可见。

回滚策略：

- 若 consumer 安装适配器不稳定，可先保留 browse/publish/version/review UX 与 deploy contract 变更，但将 `skill` / `role` install action 显式标记为 blocked with reason，而不是伪装成成功安装。
- 端口迁移回滚时可通过 env 覆盖恢复旧 URL，但文档与默认值应保持单一真相，避免长期双默认。

## Open Questions

- skill/role 安装是否需要独立的“卸载/更新” contract，还是第一阶段仅要求 install + discover + replace 已有内容？
- 管理员审核状态是否只在 marketplace service 内部维护，还是需要同步到主应用的 operator surfaces 作为全局 provenance 标签？
