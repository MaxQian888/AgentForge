## Context

AgentForge 当前已经把 role + skill 这条链路拆成了几层成熟能力:

- `src-go/internal/role/skill_catalog.go` 能从 `skills/**/SKILL.md` 发现 repo-local skill catalog。
- `src-go/internal/role/skill_runtime.go` 已经会把 skill frontmatter 中的 `requires` 和 `tools` 解析进 runtime bundle，并在 execution profile 中保留 `origin`、`requires`、`tools` 等字段。
- `components/roles/*`、`lib/roles/role-management.ts` 和 `lib/stores/role-store.ts` 已经支持 catalog-backed skill selection、resolved/unresolved cues，以及 preview/sandbox 的 loaded/on-demand feedback。

但当前缺口也很明确:

- catalog entry 只暴露 label、description、available parts 等 authoring 元数据，没有把 `requires` / `tools` 这类 compatibility metadata 变成 authoring contract。
- preview/sandbox 和 role workspace 主要解释“skill 能不能解析”，还没有 authoritative 地解释“这个 role 的 tool inventory 能不能满足 skill tree”。
- runtime readiness 目前会阻断 missing auto-load skill，但没有把 skill 声明的工具需求和 role 实际可用工具做兼容性判断。
- 当前 repo 里 role YAML 的 built-in tools 仍大量使用 `Read/Edit/Write/Bash/...` 这类旧值，而 PRD 与 skill frontmatter 更接近 `code_editor/terminal/browser_preview/...` 这套 capability-oriented 标识，直接字符串比较会制造假阴性。

因此这次设计的重点不是重新发明 skill catalog 或 runtime projection，而是给现有 role-skill seam 增加一层 authoritative compatibility contract，并把它复用到 catalog、preview/sandbox、spawn readiness 和 role workspace 的解释面。

## Goals / Non-Goals

**Goals:**

- 让 skill catalog、execution profile、preview/sandbox 和 role workspace 对同一份 skill compatibility truth 使用同一套元数据和诊断语义。
- 让 role-skill compatibility 不只关心“skill file 是否存在”，还关心 direct dependency、transitive auto-load closure、以及声明工具需求是否被当前 role 覆盖。
- 在不破坏 manual skill path 和现有 role YAML 读写兼容性的前提下，支持 legacy role tool names 与 PRD-style capability IDs 的归一化比较。
- 明确 blocking vs warning 规则:
  - auto-load skill 或其依赖的 incompatibility 阻断 execution-facing projection；
  - on-demand inventory incompatibility 继续保留为 warning-only authoring/runtime cue。

**Non-Goals:**

- 不新增新的 skill root、global skill source 或另一套 role-skill storage model。
- 不改变当前 manual skill path fallback 语义，也不把 unresolved manual reference 自动改写成 catalog entry。
- 不在这次里重做整个 role workspace 布局或重新设计 role editor 信息架构。
- 不自动为 role 补齐或修改工具授权；系统只解释和阻断不兼容组合，不偷偷修正 role manifest。

## Decisions

### 1. 在 Go skill package 层统一产出 compatibility metadata

会把 compatibility 所需元数据统一建立在 `readSkillPackageDocument(...)` / `skillPackageDocument` 这一层，然后同时服务于 catalog entry 与 runtime bundle，而不是让 catalog 和 runtime 各自再解析一遍 `SKILL.md` frontmatter。

这意味着:

- `SkillCatalogEntry` 扩展出 direct `requires` 与 declared `tools`。
- runtime bundle 继续保留 `requires` / `tools`，但其来源与 catalog 对齐为同一份 shared parsing truth。

这样做的原因:

- compatibility metadata 本质上属于 skill package contract，而不是 authoring-only 或 runtime-only 的私有字段。
- 统一在 Go 侧做一次解析，可以避免 catalog、preview mapping 和 runtime bundle 长期漂移。

备选方案:

- 仅在 runtime bundle 中保留 `requires/tools`，前端需要时再通过 preview/sandbox 侧推导。缺点是 role workspace 需要先跑 preview 才能解释 skill compatibility，authoring 太晚。
- 让前端直接读取 `skills/**/SKILL.md`。缺点是破坏当前 backend-authoritative catalog seam，也不适合多运行时环境。

### 2. 用“归一化 tool capability set”而不是原始字符串直接比较 role 与 skill

compatibility evaluation 不会直接拿 role 的原始 built-in tool 值和 skill frontmatter `tools` 做裸字符串比较，而是先构建一个 canonical capability set。这个 set 会同时吸收:

- role 的 effective built-in tool grants；
- role 已声明的 external tool IDs / MCP server names；
- 对 legacy built-in names 的 alias normalization，例如把 `Read/Edit/Write/Glob/Grep/Bash` 投影到更接近 skill frontmatter 的 capability buckets（如 `code_editor`、`terminal`、`browser_preview` 的可比较集合）或明确定义它们不覆盖的能力。

这样做的原因:

- 当前 repo 里的 sample roles 仍大量使用旧工具值；如果不做 normalization，这次 change 会把现有 role 全部误诊为 incompatible。
- skill frontmatter 想表达的是“这个 skill 依赖哪些能力面”，而不是强迫角色作者只能使用一种历史工具命名。

备选方案:

- 强制所有 role 先迁移到 PRD-style built-in tool IDs，再做 compatibility。缺点是迁移面过大，会把本次 focused seam 扩成 role contract 重构。
- 完全不做 tool compatibility，只显示声明工具列表。缺点是仍然无法回答“这个 role + skill 组合是否完整可执行”。

### 3. Compatibility diagnostics 以 Go execution profile 为 authoritative 入口

compatibility evaluation 会放在 execution-profile construction 这一条 authoritative Go 路径里，而不是做成前端独占规则或仅在 spawn 前做一次额外检查。preview、sandbox、agent spawn、workflow role execution 都复用这条结果。

具体上:

- `BuildExecutionProfile(...)` 在已有 `resolveRuntimeSkills(...)` 之后生成 compatibility diagnostics。
- auto-load skill、以及被 auto-load closure 拉进来的 dependency，只要要求的 capability 不被当前 role 覆盖，就产生 blocking diagnostic。
- non-auto-load available skill 的 incompatibility 只产生 warning diagnostic，保留其 inventory 语义。

这样做的原因:

- execution profile 已经是 role runtime truth；compatibility 如果不挂在这里，preview/sandbox、spawn、workflow 很容易再次出现三套判断。
- 当前 Go 服务已经在 spawn/workflow 前消费 `SkillDiagnostics`；沿用同一条诊断通道最稳定。

备选方案:

- 前端先做 compatibility，再把结果展示出来。缺点是 authoring 上看起来快，但不是 authoritative，也无法约束 spawn/workflow。
- spawn 时才检查 compatibility。缺点是 authoring 与 sandbox 仍然看不到真实阻塞点，用户体验会退回“保存成功但执行失败”。

### 4. 复用现有 preview/sandbox 与 role workspace，而不是新增 compatibility API

不会新增单独的 “role-skill compatibility analysis” API。现有 `/roles/skills` catalog 与 `/roles/preview`、`/roles/sandbox` 已经具备足够的载体:

- catalog 负责 direct skill metadata 与 direct compatibility hints；
- preview/sandbox 负责 effective manifest 下的 transitive loaded closure、runtime-facing diagnostics 和 blocking state；
- role workspace / context rail / role library 只做这些 authoritative payload 的解释与聚合展示。

这样做的原因:

- 当前 repo 的角色作者体验已经围绕 role workspace + context rail + preview/sandbox 建立，新增一条 API 只会让 authoring seam 更碎。
- compatibility 既有“catalog 级别的 direct metadata”，也有“effective role 级别的 runtime truth”；让两类数据各归各位更自然。

备选方案:

- 新增专门的 compatibility endpoint。优点是概念清晰；缺点是和 preview/sandbox 的 effective role truth 重复。

## Risks / Trade-offs

- [Legacy tool aliases 解释不一致] → 在 design 中明确 canonical capability mapping，并用 focused tests 覆盖旧 role values 与 PRD-style values 的兼容结果。
- [catalog 与 runtime diagnostics 展示信息过多，反而增加 authoring 噪音] → role workspace 默认突出 blocking/warning summary，把 dependency/tool 明细放在 skill row detail 与 context rail 中，而不是一开始全部展开。
- [sample roles 因为历史工具值而大量报 warning] → 对可安全归一化的 legacy values 做 alias mapping；只有真实没有覆盖到的能力才报 incompatibility。
- [preview/sandbox 与 spawn 诊断语义再次漂移] → 所有 compatibility diagnostics 从 execution profile 同一条 Go 路径产出，前端只解释，不自己重算阻断级别。

## Migration Plan

1. 扩展 skill package parsing、catalog entry、store types 和前端 mapping，先让 authoring surfaces 能看到 declared dependencies / tools。
2. 在 Go execution profile 中加入 canonical capability normalization 与 compatibility diagnostics，并让 preview/sandbox、agent spawn、workflow role execution 统一消费。
3. 更新 role workspace、role library、context rail 和 focused tests，展示 direct vs transitive skill compatibility cues。
4. 对 sample roles、sample skills 和 docs 做同步，确保文档示例不会因为 vocabulary drift 持续误导作者。
5. 如需回退，先移除前端对 compatibility metadata 的依赖，再回退 Go diagnostics 生成，避免出现“前端期待字段但后端不再返回”的半回退状态。

## Open Questions

- legacy built-in tool names 到 canonical capability IDs 的映射粒度要多细: 是只覆盖当前 repo-local sample skills 用到的 `code_editor / terminal / browser_preview`，还是一次性把 PRD 中常见 capability IDs 都纳入规范；本 change 优先覆盖当前 repo truth 已经出现的映射集合。
- compatibility summary 是否需要在 role list 上显示 direct blocker 数量以外的 dependency count；本 change 默认先显示对比决策最有价值的 blocker/warning cues，再按实现成本决定是否补更多计数。
