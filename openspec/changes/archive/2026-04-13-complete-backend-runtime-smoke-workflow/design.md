## Context

`package.json` 和 `scripts/dev-all.js` 现在已经提供 `dev:backend` / `status` / `logs` / `stop`，说明仓库已经有一套 repo-truthful 的 backend-only 启停与 runtime metadata 机制；`README.md` / `README_zh.md` 也把 Go Orchestrator、TS Bridge、IM Bridge 的本地端口、日志目录和 managed/reused 语义写成了现状真相。与此同时，`TESTING.md` 明确说明“no single one-command test surface”，而 IM Bridge 的真实端到端联调还依赖 `src-im-bridge/scripts/smoke/Invoke-StubSmoke.ps1` 这条 PowerShell-only 手工路径。换句话说，仓库已经有“把后端拉起来”的能力，但还没有一条“把后端真正跑通并给出可诊断结论”的官方 backend smoke workflow。

这次 change 是典型的 cross-stack integration / workflow work：它会同时触及 root scripts、`src-im-bridge` 的 smoke assets、以及 README / TESTING 的 source of truth。目标不是再造一套新的 backend 管理器，而是把现有 `dev:backend`、health endpoints、IM stub fixtures 和 backend connectivity 合同收束成一条受支持的 smoke proof path。

## Goals / Non-Goals

**Goals:**

- 提供一条 root-level、backend-only、repo-supported 的 smoke verification 命令，语义上等价于“把 Go + TS Bridge + IM Bridge 真正跑通”。
- 复用现有 `dev:backend` 的 managed stack、repo-local state、runtime logs 和端口健康探针，而不是复制启动逻辑。
- 在零外部 provider 凭据前提下，验证至少一条 IM stub -> Go proxy -> TS Bridge -> IM reply 的 canonical flow。
- 让 smoke 输出按 stage 指明失败 hop，并给出下一个该看的 endpoint / log 路径。
- 把 README / README_zh / TESTING 对 backend smoke 的说明更新成同一份真相。

**Non-Goals:**

- 不覆盖前端 `pnpm dev`、`pnpm build` 或 Tauri `pnpm tauri:dev` 的验证语义。
- 不尝试替代 `src-go` / `src-bridge` / `src-im-bridge` 各自的单测、类型检查或 integration tests；smoke 只证明 backend stack 活着且 canonical hop 可跑通。
- 不引入 live IM platform 凭据依赖，也不把 smoke workflow 设计成必须连接 Slack / Feishu / Telegram 等外部服务。
- 不要求在这次 change 中覆盖所有 auth-protected operator APIs（例如完整的 `/api/v1/im/test-send` UI 路径）；重点是零凭据、稳定、可复现的 backend smoke proof。

## Decisions

### Decision 1: `dev:backend:verify` 基于现有 `dev:backend` runtime manager，而不是另起一套启动器

新的 smoke 命令将直接复用 `scripts/dev-all.js` 已暴露的 backend profile start/status/stop 能力，以及 `.codex/dev-backend-state.json` / `.codex/runtime-logs/` 这套 repo-local runtime metadata。verify 本身只负责 orchestration + staged assertions：先启动或复用后端栈，再按 stage 执行检查并输出结果。

**Why this over a standalone launcher?**

- `dev:backend` 已经处理了 infra reuse、managed/reused 区分、端口冲突、日志路径和状态落盘，再造一套启动器只会复制脆弱逻辑。
- 用户要的是“跑通整个后端”，不是维护两套互相漂移的 backend workflow。

### Decision 2: smoke proof 采用“直接 health + IM stub command roundtrip”的双层验证模型

验证阶段会分成两层：

1. **Service readiness layer**：直接检查 Go `/health`、TS `/bridge/health`、IM `/im/health`，确保三段 backend process 已就绪。
2. **Canonical flow layer**：复用 IM stub test endpoints 与 fixture，向受管 IM Bridge 注入一条 repo-supported command（优先 ` /agent health` 或 `/agent runtimes` 这类 Bridge-backed 命令），然后断言成功捕获非空 reply，从而证明 IM Bridge -> Go backend -> TS Bridge -> IM reply 这条 hop 真能跑通。

**Alternative considered:** 只做 health checks。  
**Rejected because:** 这样最多证明进程活着，不能证明 Go proxy、Bridge capability routing 和 IM command reply 真正连起来了。

### Decision 3: IM stub smoke runner 必须 cross-platform，并复用现有 fixture / test endpoints

仓库已经有 `src-im-bridge/scripts/smoke/Invoke-StubSmoke.ps1`、fixture JSON，以及 `POST /test/message` / `GET /test/replies` / `DELETE /test/replies` 这些 stub endpoints。新方案不会推翻它们，而是补一层 Node-based smoke helper，让 root-level verify 可以跨平台调用同一套 fixture 和 reply capture 机制，不再把“后端跑通”绑定到 PowerShell 手工脚本。

**Why this over keeping PowerShell-only smoke?**

- 仓库明确要求跨平台命令与工作流；root-level 官方 smoke 命令不能只在 PowerShell 环境下自然可用。
- 现有 fixture/test endpoint 已经是高价值 seam，应该复用而不是重写平台 stub 逻辑。

### Decision 4: failure reporting 采用 stage-first diagnostics，并默认保留已启动的 backend stack 供后续调试

verify 命令在失败时必须输出明确 stage（如 `startup`, `go-health`, `bridge-health`, `im-health`, `stub-command`, `reply-capture`），并附带对应 endpoint、state file 或 log path。默认情况下，若 verify 自己启动了 managed backend stack，也先保留服务继续运行，方便开发者马上跟进；如需清理，再通过显式 flag 或现有 `dev:backend:stop` 完成。

**Alternative considered:** 失败后自动 stop 全部服务。  
**Rejected because:** 这会直接丢掉最有价值的 live debugging 环境，与“跑通失败后马上定位”目标相冲突。

## Risks / Trade-offs

- **[Risk] smoke command 被误解为替代所有 backend tests** → **Mitigation:** 在 spec 和文档里明确它只是 runtime smoke proof，不替代 `go test ./...`、`bun test`、`bun run typecheck`、`go build ./...`。
- **[Risk] stage 设计和真实 canonical flow 漂移** → **Mitigation:** smoke 只复用已有 repo-supported endpoints、fixture 和 command surfaces，不引入 dev-only 私有协议。
- **[Risk] 平台 fixture 与 managed 默认 IM 平台不一致** → **Mitigation:** 默认跟随 `dev:backend` 的 managed IM 平台/端口，并允许显式传入 platform 覆盖，但仍复用同一套 fixture registry。
- **[Risk] 保留运行中的 stack 可能让用户以为 verify 自带 cleanup** → **Mitigation:** 输出中明确区分“managed services kept running”与显式 stop 命令。
- **[Risk] protected operator APIs 仍未被 smoke 覆盖** → **Mitigation:** 将这类更高阶 operator verification 保留给 scoped tests / manual operator workflows，不把 zero-credential smoke 扩成 auth 流程工程。

## Migration Plan

1. 先新增 backend smoke capability spec，并为 `local-development-workflow`、`backend-bridge-connectivity` 写 delta，固定 verify 命令与 smoke proof 的 contract。
2. 在 root scripts 中实现 `dev:backend:verify`（或等效 verify entrypoint），直接复用现有 `dev-all.js` backend profile 和 runtime metadata。
3. 为 IM stub smoke 补 cross-platform helper，接入现有 fixture/test endpoints，并把默认平台、端口与 managed backend stack 对齐。
4. 更新 README / README_zh / TESTING，使 backend-only 启动、verify、失败诊断、清理命令的说法统一。
5. 回滚策略：如 verify runner 本身不稳定，可以保留 `dev:backend` 现有 start/status/logs/stop 行为，临时回退新增 verify entrypoint，而不影响 backend stack 本身的既有调试路径。

## Open Questions

- verify 默认命中的 Bridge-backed stub command 应该固定为 `/agent health`，还是允许在 `health` / `runtimes` 之间切换并把默认值收敛到更稳定的那条？
- 是否需要为 verify 增加 `--stop-managed` 之类的显式清理选项，还是只保留“默认不自动 stop + 用户手动执行 `dev:backend:stop`”这一条简单语义？
