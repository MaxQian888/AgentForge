## Context

AgentForge has already completed the first two skill-support steps:

- role manifests, inheritance, APIs, and dashboard authoring preserve structured `capabilities.skills`
- the repo now exposes a repo-local skill catalog and review cues for role authors

The remaining gap is the runtime seam. `docs/PRD.md`, `docs/part/PLUGIN_SYSTEM_DESIGN.md`, and `docs/part/PLUGIN_RESEARCH_ROLES.md` describe Skill-Tree as progressively loaded professional knowledge backed by `SKILL.md`, while the current execution profile intentionally omits skills and the Bridge prompt injector only consumes role prompt text plus `knowledge_context`. That means the product can describe, save, and preview a role's skills, but actual agent execution still behaves as if the skill tree does not exist.

This design has to close that gap without breaking the existing Go-owning-runtime model. Go must remain the only place that reads repo-local roles and skills, resolves inheritance, applies load policy, and produces a normalized execution profile. The Bridge should continue to consume a normalized `role_config` contract and never scan the repository or parse YAML or Markdown skill files directly.

## Goals / Non-Goals

**Goals:**

- Resolve repo-local `skills/**/SKILL.md` files into runtime-usable skill bundles from the Go side.
- Make `auto_load` role skills materially affect execution by projecting their instructions into the normalized execution profile and Bridge prompt composition.
- Preserve non-auto-load skills as visible runtime inventory instead of silently dropping them after authoring.
- Expose truthful preview and sandbox diagnostics for loaded skills, available skills, unresolved references, and blocking auto-load failures.
- Keep runtime projection deterministic across authoring preview, sandbox, spawn, and Bridge execution.

**Non-Goals:**

- Do not introduce a global skill marketplace, remote registry, or per-operator home-directory skill scanning.
- Do not make the Bridge parse `SKILL.md`, role YAML, or repo files directly.
- Do not silently expand `allowed_tools` or permissions based on skill frontmatter.
- Do not build a fully autonomous "load extra skill during the run" protocol in this change; on-demand skills stay inventory-like unless a later runtime control plane is added.

## Decisions

### 1. Go owns skill bundle resolution and produces a normalized runtime projection

Go will add a repo-local skill resolver alongside the existing skill catalog seam. The resolver will read `skills/**/SKILL.md`, parse frontmatter plus Markdown body, normalize the canonical role-facing path, and return runtime-ready bundles that include at least path, label, description, load policy, dependency metadata, and instruction content. Execution-profile construction, preview, sandbox, and spawn validation will all use this same resolver.

Why:

- It preserves the existing contract that Go owns repository truth and the Bridge consumes only normalized execution profiles.
- It avoids drift between preview, sandbox, spawn, and live execution.
- It keeps Windows and CI behavior stable because runtime skill resolution no longer depends on frontend heuristics or Bridge filesystem access.

Alternatives considered:

- Let the Bridge read `skills/**/SKILL.md` locally. Rejected because it breaks the current Go-owned runtime contract and makes Bridge behavior depend on local checkout shape.
- Keep skill resolution in the frontend only. Rejected because it cannot authoritatively govern spawn-time behavior or runtime blocking rules.

### 2. Execution profiles will distinguish loaded skill context from available on-demand inventory

The normalized execution profile will grow explicit runtime-facing skill projection fields instead of overloading `knowledge_context`. Auto-load skills will be resolved into prompt-ready skill context for Bridge injection. On-demand skills will be preserved as summarized inventory metadata so preview, sandbox, and runtime summaries can still explain what is available without preloading full instructions.

Why:

- Skill-Tree semantics depend on load policy; one flat string cannot express which skills were actually injected versus merely available.
- Dedicated fields make the Go to Bridge contract easier to test and evolve than hiding everything inside `knowledge_context`.
- Preview and sandbox need the same separation so operators can inspect runtime behavior before saving or launching.

Alternatives considered:

- Append skill text into `knowledge_context`. Rejected because it hides load policy and makes preview or runtime reasoning opaque.
- Preload every declared skill into the prompt. Rejected because it violates the documented progressive-disclosure model and creates unnecessary prompt bloat.

### 3. Blocking rules will follow load policy: auto-load failures block, on-demand gaps warn

If a role declares an `auto_load` skill that cannot be resolved, or if one of its required dependencies fails to resolve, preview or sandbox readiness and spawn-time execution profile construction will treat that as blocking. Unresolved non-auto-load skills will remain preserved references and warning-level diagnostics unless another runtime-facing contract explicitly requires them.

Why:

- Once skills affect execution, silently skipping an auto-load skill would be a correctness bug.
- Warning-only handling still makes sense for staged or future on-demand references because they are not promised to be injected immediately.
- This preserves the useful flexibility of manual references without letting runtime-critical knowledge disappear silently.

Alternatives considered:

- Keep all unresolved skills warning-only. Rejected because `auto_load` would still be operationally meaningless.
- Block every unresolved skill regardless of load policy. Rejected because it would over-constrain authoring and drift away from progressive loading.

### 4. Skill dependencies are normalized in Go, but tool hints stay advisory

The resolver will recognize dependency metadata such as `requires` and normalize it into canonical skill identities so execution can load auto-load dependency closure deterministically. However, skill-declared tool hints or similar metadata will remain informational in this change; they may appear in preview or runtime summaries, but they will not automatically widen `allowed_tools`, plugin selection, or permission mode.

Why:

- Dependency closure is part of making skill content actually usable at runtime.
- Permission and tool expansion must remain controlled by explicit role capability and security settings, not implicit text metadata inside a skill file.

Alternatives considered:

- Ignore dependency metadata entirely. Rejected because it leaves documented `SKILL.md` structure under-supported.
- Auto-merge skill tool hints into runtime permissions. Rejected because it weakens the current governance boundary.

### 5. Preview and sandbox will expose the same skill projection the runtime would use

Preview or sandbox payloads will surface the execution-facing skill projection directly: loaded auto-load skills, available on-demand skills, blocking versus warning diagnostics, and any dependency-derived load decisions. Frontend review surfaces should render that returned projection rather than recomputing skill runtime behavior locally.

Why:

- Operators need to see the exact runtime truth before save or launch.
- A shared projection avoids duplicating dependency or blocking logic across Go and frontend helpers.

Alternatives considered:

- Recompute loaded versus available skills in the frontend. Rejected because it can drift from runtime truth and repeats resolution logic.

## Risks / Trade-offs

- [Auto-loaded skill content makes prompts too large] -> Mitigation: keep on-demand skills as summaries only, deduplicate dependency closure, and define deterministic formatting so later size limits can be enforced without changing the contract shape.
- [Repo-local skill files contain inconsistent metadata or weak instructions] -> Mitigation: expand sample fixtures and docs, and test both metadata parsing and body projection instead of assuming every `SKILL.md` is well-formed.
- [Auto-load blockers surprise operators who are used to warning-only skills] -> Mitigation: expose blocking rules clearly in preview, sandbox, docs, and launch diagnostics before apply.
- [Runtime contract grows more complex] -> Mitigation: keep Bridge file-agnostic and add explicit normalized fields rather than leaking raw role or skill documents.

## Migration Plan

1. Add Go-side skill bundle resolution and execution-profile projection with focused tests.
2. Extend the Go to Bridge `role_config` contract, Bridge schema, and prompt injector to consume projected skill context and available inventory.
3. Expose the same loaded or available skill projection and blocking diagnostics through preview and sandbox flows.
4. Update sample skills, sample roles, and docs so auto-load versus on-demand semantics match actual runtime behavior.
5. Verify with focused Go and Bridge tests before implementation sign-off.

Rollback strategy:

- If runtime skill projection proves unstable, revert to the previous execution-profile contract and keep role skills catalog- and authoring-only.
- Preview or sandbox can continue returning skill diagnostics even if Bridge-side injection is temporarily rolled back, because the resolver is additive on the Go side.

## Open Questions

- What size guard should apply to auto-loaded skill context before prompt assembly starts truncating or refusing overly large bundles?
- Should skill dependency metadata accept only canonical role-style paths such as `skills/typescript`, or also normalize shorthand names in frontmatter?
- Should Bridge expose loaded and available skill metadata in runtime snapshots or status payloads for operator diagnostics, or is prompt-composition behavior sufficient for the first implementation pass?
