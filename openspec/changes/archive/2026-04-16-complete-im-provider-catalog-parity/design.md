## Context

当前 repo 的多 IM 基础设施并不是“没有实现”，而是“真相分叉”：

- `src-im-bridge/cmd/bridge/platform_registry.go` 已经把 built-in provider 扩展到 `feishu`、`slack`、`dingtalk`、`telegram`、`discord`、`wecom`、`qq`、`qqbot`、`wechat`、`email`
- frontend `/im` 与 settings 仍主要由 `lib/stores/im-store.ts` + `components/shared/platform-badge.tsx` 的硬编码平台集合驱动，缺少 `wechat`
- Go backend 某些 IM model / validation 仍停在旧快照，导致有的 surface 接受 `email` 这种 delivery-only provider 作为交互入口，却又没有把 `wechat` 纳入同一层的合法 interactive provider truth
- `src-im-bridge/README.md` 和 `src-im-bridge/docs/platform-runbook.md` 的平台矩阵也没有完全跟上当前 runtime registry

这次 change 的难点不在于“新增一个 provider adapter”，而在于如何把现有 provider truth 同步给 backend operator APIs、frontend operator surfaces 和文档，同时避免 scope 膨胀成跨 module 的共享 runtime package 重构。

## Goals / Non-Goals

**Goals:**
- 让 backend 拥有一套 authoritative 的 operator-facing IM provider catalog，可区分 interactive chat provider 与 delivery-only provider。
- 让 surface-specific validation truthful 对齐：例如 `wechat` 可用于 interactive command/callback surfaces，`email` 只用于 delivery/test/config 相关 surfaces，而不是伪装成完整 IM command provider。
- 让 `/im` 与 settings 页面从 authoritative catalog 派生 provider 列表、配置字段和 operator affordances，消除当前 stale hardcoded platform universe。
- 让 docs / runbook / manual verification matrix 与当前 built-in provider registry 对齐，至少补齐 `wechat`、`email` 与相应 operator constraints。
- 通过 focused tests 防止 bridge runtime、backend contract 与 frontend operator surface 再次出现 catalog drift。

**Non-Goals:**
- 不把单进程 IM Bridge 改成“一个进程同时挂多个 active provider”；当前仍保持 single-active-provider-per-process 模型。
- 不重写 `src-im-bridge` 的 provider registry 成为一个跨 Go backend / frontend / bridge 共用的多语言 runtime package。
- 不在本次 change 中重新设计所有 provider 的 native payload / callback / async completion 语义；这次只收紧 provider catalog truth 与 operator parity。
- 不引入新的身份映射系统（如 email address 到 AgentForge user 的 inbound identity mapping）。

## Decisions

### 1. 增加 backend-owned provider catalog endpoint，作为 operator surface 的 authoritative truth
- **Decision**: 由 Go backend 暴露一个新的 provider catalog contract（例如 `GET /api/v1/im/platforms`），返回当前 operator 需要的 provider metadata：`id`、display label、interaction class、是否支持 channel config、是否支持 test-send、配置字段 schema、transport/readiness 摘要。
- **Why**: `/im` 和 settings 已经以 backend 为中枢；继续让 frontend 维护独立硬编码平台集合，只会重复当前 drift。把 operator-facing catalog 收口到 backend，frontend 可以直接消费，不必再推断 bridge runtime 真相。
- **Alternatives considered**:
  - 继续仅靠 frontend hardcoded constants 手动同步：实现最小，但 drift 问题会重复出现。
  - 直接让 frontend 调 `src-im-bridge` runtime registry：破坏当前 Go backend 为 operator/control-plane 中枢的拓扑，也不适用于 bridge 未运行时的配置场景。

### 2. 按 surface 区分 provider affordance，而不是用一个平台枚举统治所有 IM API
- **Decision**: 将 provider truth 明确拆成至少两类 surface：
  - **interactive chat**：可作为 inbound message / action / command source 的 provider（例如 `feishu`、`slack`、`dingtalk`、`telegram`、`discord`、`wecom`、`qq`、`qqbot`、`wechat`）
  - **delivery-only**：可用于 outbound send / notify / test-send / channel config，但不宣称 inbound operator command parity 的 provider（例如 `email`）
- **Why**: 当前 `src-go/internal/model/im.go` 这种“某些 interactive request 允许 `email`、却漏掉 `wechat`”就是把所有 surface 混成一个 platform union 的结果。按 surface 切分后，backend validation 和 frontend affordance 都能 truthful 表达差异。
- **Alternatives considered**:
  - 所有 IM API 统一接受同一平台集合：简单，但会持续制造 `email`/`wechat` 这类 false positive / false negative。
  - 为每个平台写独立 handler/DTO：表达最精细，但 scope 太大，与当前 canonical `/api/v1/im/*` 结构不匹配。

### 3. frontend 改为“catalog 驱动 + 本地轻量展示元数据”模式
- **Decision**: `/im`、channel config、settings summary 从 backend provider catalog 读取 provider 列表和配置字段 schema；frontend 本地只保留轻量的展示元数据（例如 icon mapping、badge 样式 fallback），不再把 config schema / affordance 真相硬编码在 `PLATFORM_DEFINITIONS` 里。
- **Why**: 前端最常漂移的是“哪些平台存在、每个平台该显示哪些字段、哪些 affordance 应该可点”。这些都应该来自 authoritative contract；而 icon/配色属于纯展示 concern，留在前端即可。
- **Alternatives considered**:
  - 完全把 label/icon/schema 都交给 backend：最集中，但会把纯视觉细节也耦合进 API，不划算。
  - 继续让 `PLATFORM_DEFINITIONS` 负责全部 truth：无法解决 catalog drift，只是把 drift 集中到一个 TS 文件。

### 4. 用 focused sync tests 约束 backend catalog、bridge registry 与 docs，而不是抽象成共享运行时包
- **Decision**: 保持 `src-im-bridge` runtime registry 和 backend operator catalog 各自位于当前模块边界内，但增加 focused tests / snapshots / docs assertions，确保：
  - backend catalog 至少覆盖当前 built-in provider set 中对 operator 可见的 provider
  - `wechat` / `email` 的 affordance 分类与 runtime metadata 一致
  - README / runbook 的平台矩阵和 manual verification 不落后于 catalog
- **Why**: 这能在当前 repo 结构下控制复杂度，不需要把 bridge runtime 代码硬搬进 backend 或 frontend 共用模块。
- **Alternatives considered**:
  - 构建跨模块共享 registry 包：长期更统一，但会显著扩大这次 change 的 blast radius。
  - 只修代码不加 sync tests：会很快重新漂移。

## Risks / Trade-offs

- **[backend catalog 成为新的同步点]** → 通过 focused tests 将 catalog 与 `src-im-bridge` built-in provider set 做 contract 校验，并在 docs 任务里同步 runbook/README。
- **[frontend 从静态 schema 切到 API 驱动后，初始渲染逻辑更复杂]** → 保持 API payload 扁平清晰，前端只做最小转换，并允许在 catalog 未加载时降级为只读/加载态而不是伪造 provider 列表。
- **[surface-specific validation 可能暴露现有 tests 或调用方假设]** → 先锁定 `/api/v1/im/*` operator surfaces 和 IM model tests，明确哪些 DTO 是 interactive-only，哪些允许 delivery-only provider，再按 slice 修复。
- **[docs 与代码同步成本上升]** → 将 docs 更新纳入 change 主任务，而不是作为可选收尾项；手工验证矩阵至少覆盖新增 truth 差异最大的 `wechat` 与 `email`。

## Migration Plan

1. 先定义 backend provider catalog DTO 和 surface-affordance 分类，并补 Go tests 锁定 `wechat` / `email` 的 validation truth。
2. 再把 `/im` 与 settings 切到消费 catalog，同时保留现有 bridge status / channels / event-types / deliveries 数据流不变。
3. 收敛 shared frontend platform metadata，只保留展示层 icon/badge 映射，把 config schema 与 affordance 真相迁移到 catalog payload。
4. 更新 bridge README / runbook / manual verification matrix，使 operator docs 与 catalog 对齐。
5. 回滚策略：若 frontend catalog 消费造成 operator 页面异常，可暂时退回到旧的 local fallback list，但 backend surface-specific validation 与 docs truth 仍应保留，避免继续扩大 drift。

## Open Questions

- `email` 是否在 operator console 中允许 canonical test-send？从当前 runtime 能力看它支持 outbound send，设计默认允许，但 implementation 时需要确认 UI 文案是否要显式标注为 delivery-only smoke test。
- backend provider catalog 是否还需要暴露 docs/runbook 链接字段，还是先保持最小 payload，仅覆盖 operator 必需 metadata？
- `wechat` 当前是否只需要进入 catalog + validation + docs truth，还是还要同步补 `/im` 页面里的 provider-specific config schema；从 repo truth 看它更像是已有 runtime provider 但缺 operator schema，这次默认将其纳入。
