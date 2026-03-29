## Context

当前 `src-im-bridge` 已经围绕 `provider_contract.go`、`platform_registry.go`、`control_plane.go`、typed delivery 与 reply-target 恢复机制形成了统一的 IM provider 接入面。`Feishu/Slack/DingTalk/Telegram/Discord/WeCom` 都已经走这条 seam，但 PRD 与 `docs/part/CC_CONNECT_REUSE_GUIDE.md` 仍把 `QQ (NapCat/OneBot)` 和 `QQ Bot 官方` 作为目标平台与复用目录的一部分，而当前代码中完全没有对应 provider、文档运行矩阵、smoke fixture 或主规格。

这意味着当前缺口不是再发明新的 IM runtime，而是把 QQ 系列平台补进已经存在的共享合同里，并保证它们在 provider selection、control-plane registration、source metadata、typed outbound delivery、reply-target replay、runbook 与测试矩阵上与现有平台保持一致。

## Goals / Non-Goals

**Goals:**
- 让 QQ 与 QQ Bot 通过现有 provider registry 成为可启动的 built-in platform，而不是文档中的占位平台。
- 复用现有 control-plane、health、notification replay、typed delivery、rendering profile 与 reply-target seam，不新增平行 runtime。
- 为 QQ 与 QQ Bot 定义 truthful capability matrix，只声明当前 provider 真正可支持的命令面、结构化输出、回调模式和异步更新路径。
- 补齐主规格、README、runbook、smoke fixture 与 focused verification，使代码、规格与项目文档设计重新对齐。

**Non-Goals:**
- 不在本次 change 内把 `line`、`wechat` 或其他更宽的 cc-connect 平台一起拉入 scope。
- 不把 QQ 或 QQ Bot 提升为“全平台 richer parity”；如果某些消息组件、原地更新或交互能力当前无法稳定支持，应显式降级而不是伪装支持。
- 不重写现有 shared delivery 或 control-plane 架构；本次只在既有 seam 上扩平台。
- 不引入新的跨进程插件框架或替换当前 built-in provider 模式。

## Decisions

### 1. 为 QQ 与 QQ Bot 各自新增 focused provider capability，而不是创建一个模糊的 “qq-family” 平台

QQ (NapCat/OneBot) 与 QQ Bot 官方在接入协议、凭据形态、消息/回调能力和部署方式上并不等价。设计上将二者拆成两个 capability：

- `qq-im-platform-support`
- `qqbot-im-platform-support`

这样主规格、provider descriptor、runbook、smoke fixtures 和测试都可以围绕各自真实 transport 约束编排，同时共享同一套 `additional-im-platform-support` / `im-platform-plugin-contract` / `im-rich-delivery` 上层合同。

备选方案：
- 只做一个 `qq-family` capability。优点是文档更短；缺点是很容易把 OneBot 与官方 QQ Bot 的 transport、callback 和 richer surface 混在一起，后续 apply 阶段会失真。

### 2. 继续沿用 descriptor-driven provider seam，不在 `main.go` 增加新的平台分支

QQ 与 QQ Bot 必须像现有六个平台一样，通过 `platform_registry.go` 中的 descriptor 声明：

- provider id
- supported transport modes
- config validation
- capability metadata
- rendering profile metadata
- stub/live factory

这样 `selectProvider`、`selectPlatform`、`/im/health`、control-plane registration、typed delivery 与 replay 才能自然复用现有逻辑。任何平台专有差异都收敛在 `platform/qq`、`platform/qqbot` 与各自 descriptor metadata 中，而不是散落到 shared delivery 或启动主路径。

备选方案：
- 在 `main.go` 再写 `switch` 分支直接装配 QQ。优点是短期更快；缺点是破坏近几次 IM Bridge change 刚建立好的可扩展 seam，也不符合“组件复用和代码强度”的用户要求。

### 3. QQ 与 QQ Bot 的首版目标是 “truthful runnable baseline”，不是强行追平所有 richer surface

当前已有 provider 的经验表明，新平台接入时最容易失控的点是过早承诺完整 richer parity。本次设计要求：

- 优先实现可运行的 inbound command normalization、source metadata、reply-target 恢复、control-plane registration 与 outbound delivery baseline。
- richer/structured 发送能力通过 rendering profile 声明；若当前 transport 或 reply target 无法支持原地更新、按钮组件或更丰富消息形态，shared delivery 必须明确降级到平台支持的文本或链接表示，并带上 fallback metadata。

这与 DingTalk、WeCom 当前的真实处理方式一致，能保证新增平台不会因为过度承诺而削弱共享 delivery 层的可维护性。

备选方案：
- 把 QQ / QQ Bot 定义成 text-only 且不进入 typed delivery。优点是实现最简单；缺点是会把它们排除在现有 typed replay / control-plane / richer metadata 合同之外，后续又会形成新的一套特判。

### 4. 文档与规格同步必须和代码 seam 一起落盘

IM Bridge 最近几次变更已经证明，如果只补代码不补主规格、README、runbook、smoke fixture 与验证矩阵，平台矩阵会再次漂移。本次 change 设计上把以下内容视为同一交付的一部分：

- 主规格增量与新增 capability spec
- `src-im-bridge/README.md` 平台矩阵与命令/通知说明
- `src-im-bridge/docs/platform-runbook.md` 的 rollout、rollback、feature matrix 与 manual verification
- smoke fixture / script
- focused tests 与验证命令

备选方案：
- 把文档留到 apply 末尾顺手更新。优点是前期阻力小；缺点是容易再次形成“代码支持了，文档还停留在旧矩阵”的 repo drift。

## Risks / Trade-offs

- [Risk] QQ 与 QQ Bot 的官方/社区 transport 语义差异较大，若在 proposal 阶段写得过死，apply 时可能被真实接口约束打脸。
  Mitigation: 规格只要求 runnable baseline、truthful capability declaration 与 explicit fallback，不预先承诺超出 repo 文档已支持的 richer parity。

- [Risk] 新 provider 可能诱导 shared delivery 再次出现平台名特判。
  Mitigation: 设计要求所有平台差异优先进入 descriptor metadata、provider-owned rendering profile 与 platform-local implementation，shared 层只消费声明能力。

- [Risk] QQ 系列平台一旦接入，README/runbook/test matrix 会显著变长。
  Mitigation: 通过 “两条 focused capability + 三个共享主规格增量” 控制结构，不把所有未来平台都一次性并入。

- [Risk] 本次 scope 如果同时把 `line/wechat` 也纳入，会让变更重新失焦。
  Mitigation: 明确把 scope 限定为当前文档矩阵中最直接缺失、且与 QQ 生态相邻的 `qq` 与 `qqbot` 两个平台。

## Migration Plan

1. 新增 change-local proposal/design/specs/tasks，使变更进入 apply-ready。
2. apply 阶段先补 `src-im-bridge` provider descriptor、config、platform package、reply-target / delivery / control-plane seam 的最小实现。
3. 紧接着补 focused tests、smoke fixtures 与 runbook/README，同步主规格。
4. 验证 `openspec validate --specs`、`src-im-bridge` focused tests 与 scoped smoke path。
5. 若 live transport 在真实环境中不可用，回退到不启用该 provider 的部署配置，同时保留 stub + specs + docs；不影响现有六个平台运行。

## Open Questions

- QQ (NapCat/OneBot) 的首版 live seam 在本仓库中应以 WebSocket 事件 intake 为主，还是允许 HTTP callback 兼容入口；apply 时需要根据 repo 现有 client/control-plane 适配成本收窄。
- QQ Bot 官方的 richer surface 在首版中是否只承诺文本 + 链接 + 显式降级，还是已有足够 seam 支撑按钮/交互组件；apply 时需要根据 transport 与回复上下文真相决定。
- 两个平台的 reply-target 最小持久化字段是否可以完全复用当前 `IMReplyTarget.Metadata` 承载，还是需要新增结构化字段来避免 shared layer 再度出现 provider-specific string parsing。
