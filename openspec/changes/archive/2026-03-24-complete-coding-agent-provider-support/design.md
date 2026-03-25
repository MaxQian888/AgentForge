## Context

AgentForge 现在已经有两条相关基础能力，但还没有形成完整产品闭环。

- `src-bridge` 已具备 `claude_code`、`codex`、`opencode` 三类 runtime 适配器，并且 `README.md` 也记录了对应环境变量与命令约定。
- `src-go` 与前端 store 已能传递 `runtime` / `provider` / `model`，但项目设置、启动入口、团队流水线和运行摘要仍没有统一消费这些字段。
- 当前产品层存在几个真实断点：`components/team/start-team-dialog.tsx` 把 provider 锁死为 `anthropic`；`TeamService` 只把 provider/model 传给 planner，coder/reviewer 又丢回空值；`resolveBridgeRuntime(...)` 仍通过 provider 推断 runtime；`projects.settings` 与设置页没有 coding-agent provider catalog/defaults；`agent_runs` 也没有单独持久化 runtime，导致 Codex/OpenCode 运行身份在摘要与调试链路中不稳定。

这次 change 是跨前端、Go、Bridge、数据模型和文档的横切变更，而且需要把现有“兼容逻辑”收敛成更明确的产品合同，因此需要先写清设计。

## Goals / Non-Goals

**Goals:**
- 给项目级配置增加统一的 coding-agent provider catalog 与默认值语义，覆盖 Claude Code、Codex、OpenCode。
- 让设置页、单 Agent 启动和 Team 启动使用同一套 runtime/provider/model 选择与校验结果。
- 让 planner、coder、reviewer 在同一个 Team run 中继承一致的 runtime/provider/model，而不是只有首个阶段带配置。
- 在 Go 与 Bridge 之间建立明确的 runtime/provider/model 一致性合同，并把 resolved runtime 持久化到运行记录和摘要中。
- 为运维和调试暴露“可用/不可用”的真实原因，例如缺少 API Key、缺少 CLI 命令、配置组合不兼容。

**Non-Goals:**
- 在本次变更中新增 Claude Code、Codex、OpenCode 之外的更多 coding-agent runtime。
- 重写 `src-bridge` 的事件模型或改变 Codex/OpenCode command adapter 的底层协议。
- 一次性建设完整的多租户凭据托管系统；本次只定义项目配置与运行时可用性诊断，不引入新的秘密存储架构。
- 扩展任务分解、意图识别等轻量 text-generation provider 的能力矩阵；本次聚焦 coding-agent execution。

## Decisions

### 1. 项目级 provider catalog 作为产品默认值真相源，Bridge/Go 负责把它解析为运行时合同
项目配置需要新增 `coding_agent`（命名可在实现时最终确认）结构，至少覆盖：

- 支持的 runtime 列表及展示元数据
- 每个 runtime 的兼容 provider 标识与推荐默认 model
- 项目默认 runtime/provider/model
- 可用性状态与诊断摘要

前端设置页和启动表单只消费这个 catalog，而不是自己硬编码 `anthropic` 或模型列表。Go 负责把项目默认值和用户显式选择合并成一次完整的启动参数，再发往 Bridge。

这样做的原因是当前 drift 的根源之一就是默认值分散在前端、Go、Bridge 三层。把“产品级默认值”上收到项目设置后，才能让单 Agent、Team、后续批量 dispatch 和 dashboard 读到同一份配置语义。

备选方案：
- 继续把默认值放在前端组件里。否决，因为会继续出现 `StartTeamDialog` 这种只支持 Claude 的分叉。
- 让 Bridge 直接承担所有项目默认值。否决，因为项目设置、UI 展示和后端校验仍需要同一份上游真相源。

### 2. `agent_runs` 必须显式持久化 resolved runtime，而不是只保存 provider/model
当前运行记录只有 `provider` 和 `model`，但这不足以稳定表达 Claude Code、Codex、OpenCode 的运行身份，尤其是 provider alias、legacy fallback 或未来同 provider 多 runtime 并存时会失真。

因此本次设计要求运行记录、API summary、Team summary 和相关 WebSocket payload 统一保留 `runtime`、`provider`、`model` 三元组，其中：

- `runtime` 表示真实执行后端（`claude_code` / `codex` / `opencode`）
- `provider` 表示用户选择或项目默认的 provider 语义
- `model` 表示最终执行使用的模型名

这样可以让详情页、团队摘要、故障排查和成本统计看到稳定的运行身份，也能避免把 `provider=codex` 这种历史兼容写法继续误当成完整真相。

备选方案：
- 只在内存或日志里保留 runtime。否决，因为刷新页面、恢复会话和事后审计都会丢失执行身份。
- 继续把 runtime 从 provider 反推。否决，因为这正是当前歧义来源。

### 3. Team run 必须把同一组 provider/runtime/model 贯穿 planner、coder、reviewer
Team 服务当前只把 provider/model 传给 planner，后续 coder/reviewer 都回落为空，这会让一次 Team 执行中不同阶段跑在不同默认值上，直接破坏 Claude Code/Codex/OpenCode 的“完整支持”。

本次要求 Team 层在 team record 或等价上下文里保存启动时选择的 runtime/provider/model，并让后续阶段显式继承，而不是重新走空值默认。对用户来说，一次 Team 启动选择的是一个完整执行策略，而不是只配置第一个 planner。

备选方案：
- 允许每个 team role 自动回到各自默认值。否决，因为这会让同一次 team run 内部运行身份漂移。
- 给每个 team role 单独配置 runtime/provider/model。暂不采用，因为会把本次 change 扩展成更大规模编排设计。

### 4. Go 正式链路不再依赖 `provider -> runtime` 猜测；Bridge 只保留最小 legacy 兼容
Go 到 Bridge 的正式执行链路必须发送显式 `runtime`，并在发送前完成 runtime/provider/model 兼容性校验。Bridge 仍可以对极少数 legacy 直接调用保留 provider-only 兼容，但这种兼容不再作为 Go 服务链路的主路径，也不应影响项目设置和 Team 启动的明确语义。

这能让错误更早暴露，例如：

- 选择 `runtime=codex` 却给出不兼容 provider
- 选择 `opencode` 但缺少可执行命令
- 选择 `claude_code` 但项目或环境没有有效凭据

备选方案：
- 完全移除 Bridge 的 legacy 兼容。暂不采用，因为会增加已有测试和手工调用面的破坏性。
- 继续允许 Go 通过 provider 推断 runtime。否决，因为这会让产品级 catalog 失去约束力。

### 5. Runtime catalog 的可用性诊断来自真实 Bridge 能力，而不是前端静态推断
Claude Code、Codex、OpenCode 的 readiness 不仅取决于项目选择，还取决于 Bridge 当前环境是否具备 API Key、Auth Token、CLI 可执行文件和默认 model。前端如果只靠静态列表展示，会让用户在点下启动之后才看到含糊错误。

因此需要由 Bridge 或 Go 聚合层提供面向 catalog 的 availability 结果，至少包含：

- runtime 是否当前可用
- 缺失的是凭据、命令还是组合不兼容
- 推荐默认 model / provider
- 兼容的 provider alias 或选择项

备选方案：
- 只在启动失败后返回错误。否决，因为设置页和启动表单无法在事前提示真实问题。
- 让前端直接检查环境变量。否决，因为前端拿不到可信的服务端运行环境信息。

## Risks / Trade-offs

- [Risk] `agent_runs` 增加 `runtime` 字段会带来迁移和 DTO 改动面 -> Mitigation: 采用向后兼容迁移，先补字段与默认填充，再更新 API/store/UI 消费。
- [Risk] 项目 settings schema 扩展后，如果前后端没有统一默认值解析，可能再次出现 drift -> Mitigation: 只允许 Go 侧做最终默认值合并，并为 settings/catalog 增加 focused tests。
- [Risk] Team 继承统一 runtime/provider/model 后，某些旧测试可能仍假定 coder/reviewer 走空默认 -> Mitigation: 在 team service tests 中显式覆盖 planner/coder/reviewer 一致性场景。
- [Risk] Bridge 暴露 catalog diagnostics 可能与现有 execute-time 校验重复 -> Mitigation: 把 diagnostics 复用现有 runtime registry/provider registry 的同一套 metadata 与错误分类，避免双份逻辑。
- [Risk] legacy provider-only 兼容若保留太久，可能继续被误用 -> Mitigation: 在文档和错误消息中把它标为 compatibility-only path，并让 Go 正式链路始终发送显式 runtime。

## Migration Plan

1. 先扩展 spec 与数据模型，给项目 settings 与 agent run 记录补齐 runtime/provider/model 的完整合同。
2. 在 Go 服务层实现 catalog/default 解析、运行时三元组持久化，以及 Team 全链路继承。
3. 在 Bridge 侧补 catalog diagnostics 与更严格的 runtime/provider/model 校验，但保留最小 legacy 兼容。
4. 最后再接前端设置页和启动入口，确保 UI 只消费新 catalog，而不是保留硬编码选项。
5. 验证通过后再更新 README / PRD / role-yaml 等文档，保证运维和开发入口与最终实现一致。

回滚策略：

- 如果前端设置页或 Team 入口出现问题，可先回滚 UI 消费层，同时保留后端/Bridge 的兼容字段。
- 如果 runtime diagnostics 接口阻塞上线，可暂时保留 execute-time 严格校验，但不发布设置页中的 readiness 提示。

## Open Questions

- 项目 settings 顶层键最终使用 `coding_agent`、`agent_runtime` 还是复用现有 `agent_defaults` 扩展子结构，需在实现前定稿。
- Team 是否需要在本次就支持“同一个 team 内不同角色选择不同 runtime”，当前设计默认不支持，优先保证整条链路一致。
- catalog diagnostics 是直接由 Bridge 暴露新接口，还是由 Go 聚合 Bridge 结果后再统一对前端提供，需要结合现有 API 边界最终确定。
