## Context

AgentForge now has a truthful official built-in plugin bundle, a Go control plane that can surface built-in discovery and catalog data, and a frontend plugin console that separates installed, built-in, local, and remote entries. The missing seam is not asset coverage anymore. It is readiness semantics.

Today the built-in bundle only gives the operator a coarse availability status and free-form message. The Go control plane forwards that metadata, and the plugin panel renders it as static text while still treating most built-ins as generic install candidates. This leaves several product gaps:

- Operators cannot reliably tell whether a built-in is ready now, installable but not activation-ready, or blocked on the current host.
- The system does not expose structured prerequisite or configuration guidance that the frontend can render consistently.
- Built-in installability and runtime readiness are conflated, so a button can imply "usable" when the real state is "installed but still blocked on setup".
- Verification can catch bundle drift, but it does not yet validate the readiness contract that operator-facing flows depend on.

This is a cross-cutting change because it touches repo-owned bundle metadata, Go DTOs and control-plane evaluation, the plugin management surface, and verification scripts. It is still intentionally narrow: the built-in inventory, plugin runtimes, and remote registry model stay as they are.

## Goals / Non-Goals

**Goals:**
- Define a machine-readable readiness contract for official built-in plugins that captures prerequisite checks, configuration requirements, docs references, and next-step guidance.
- Evaluate built-in readiness inside the Go control plane so discovery and catalog responses carry current operator-facing readiness state instead of only static prose.
- Make the plugin management panel distinguish installability from activation readiness and explain blocked states without forcing the operator to read raw manifest or bundle files.
- Add bounded verification for readiness metadata and deterministic preflight checks so official built-ins remain visible and actionable.

**Non-Goals:**
- Do not add new official built-in plugins or revisit the built-in inventory decided by `complete-documented-built-in-plugin-support`.
- Do not redesign plugin runtime hosts, activation contracts, remote registry behavior, or trust verification for external plugins.
- Do not introduce secret validation that requires calling third-party APIs during default readiness evaluation.
- Do not turn the plugin panel into a full wizard flow for every built-in; this change focuses on truthful readiness state and actionable guidance, not end-to-end secret management UX.

## Decisions

### 1. Extend the built-in bundle with structured readiness metadata instead of introducing a separate readiness registry

The official built-in bundle already acts as the repo-owned source of truth for built-in plugin identity, docs, verification profile, and coarse availability. Readiness metadata should live next to that existing truth source instead of in a second file or database table.

This keeps ownership clear:
- repo-maintained built-ins stay self-describing in one bundle file
- verification can validate both asset drift and readiness drift in one place
- the control plane can evaluate readiness directly from the bundle plus local environment

Alternative considered:
- Create a separate readiness registry file. Rejected because it splits built-in truth across two sources and makes drift more likely.

### 2. Separate installability from activation readiness in control-plane responses

The current platform contract already allows explicit install of built-in entries without browse side effects. That should remain true. What changes is that the control plane must separately report whether the built-in is activation-ready after install.

The response model should therefore distinguish:
- source installability: can this built-in be installed through the current supported flow
- readiness state: is it ready now, missing prerequisite, missing configuration, or unsupported on this host
- guidance: what the operator should do next

This avoids a false binary where every non-ready plugin becomes entirely hidden or entirely blocked from install.

Alternative considered:
- Collapse readiness into a single `installable` flag. Rejected because a built-in like `github-tool` can still be installed even when credentials are not configured yet.

### 3. Readiness evaluation stays deterministic by default and avoids secret-dependent live checks

Default readiness evaluation should inspect deterministic signals only: bundle metadata, local binaries or package-manager presence, supported host/runtime family, manifest paths, and the presence of required configuration values in known control-plane or bridge config surfaces. It should not attempt live third-party authentication or network probes during normal panel rendering.

This keeps readiness fast, reproducible, and aligned with the repo's current plugin verification philosophy.

Alternative considered:
- Perform live smoke checks whenever a built-in is discovered. Rejected because it would make plugin discovery noisy, slow, and dependent on secrets or external services.

### 4. The panel surfaces readiness on both availability cards and installed-plugin details

Readiness cannot live only in the built-in discovery section, because operators may install a built-in before finishing setup. The plugin detail surface for installed built-ins must continue to show the same blocking reasons and setup guidance so lifecycle actions stay understandable after installation.

This means the frontend should render:
- readiness badge and summary on built-in availability cards
- docs link and setup guidance on cards and details
- blocked activation explanations on installed built-ins
- stronger differentiation between "install now" and "usable now"

Alternative considered:
- Show readiness only before install. Rejected because the most confusing state is often an installed plugin that still cannot activate.

### 5. Verification should validate readiness contracts and preflight behavior by family

The existing built-in bundle verification already checks drift at the bundle level. This change should extend that workflow with readiness validation rather than create a brand-new monolithic script. Each maintained built-in family can expose deterministic readiness checks that prove the declared prerequisite or configuration contract still matches reality.

Alternative considered:
- Add one end-to-end readiness smoke for every built-in. Rejected because it would repeat the same brittleness that earlier family-based verification intentionally avoided.

## Risks / Trade-offs

- [Readiness metadata becomes another schema to maintain] -> Mitigation: validate readiness fields in `verify-built-in-plugin-bundle` and fail fast on malformed or incomplete entries.
- [Operators may misread installable-but-not-ready as a regression] -> Mitigation: explicitly separate installability from readiness in DTOs and UI copy.
- [Configuration checks may overfit current env var names] -> Mitigation: keep checks contract-based and minimal, tied to current supported config surfaces rather than provider-specific live auth.
- [Panel complexity can grow if every built-in adds bespoke setup text] -> Mitigation: constrain bundle metadata to structured reasons and short next-step guidance instead of arbitrary rich content.

## Migration Plan

1. Extend `plugins/builtin-bundle.yaml` with structured readiness metadata while preserving current docs and verification fields.
2. Update Go bundle parsing and built-in discovery/catalog DTO assembly to evaluate readiness and return structured guidance.
3. Extend frontend store types and plugin panel rendering to display readiness, setup guidance, and action gating on built-in plus installed surfaces.
4. Expand `verify-built-in-plugin-bundle` and related tests to validate readiness schema and bounded preflight behavior.
5. Roll back by dropping the readiness-specific fields from responses and bundle parsing while keeping the existing official built-in bundle intact if the UI or evaluation layer proves too noisy.

## Open Questions

- Which config surface should be treated as authoritative for readiness checks that depend on bridge-host secrets: persisted plugin config, environment variables, or both?
- Should a host-unsupported built-in remain visible with a disabled install action, or should the control plane classify it as browse-only with no install path at all?
