# IM Bridge Provider Extensibility & Plugin System Alignment

**Date:** 2026-04-21
**Scope:** IM Bridge provider factory registry + Bridge registration contract enrichment + `feishu-adapter` demo cleanup
**Approach:** In-tree factory registration (no out-of-tree provider runtime), read-only unified inventory via the existing `/im/bridge/register` contract (no new WS channel / no new DB table), rename the misleading `feishu-adapter` demo

---

## 1. Background & Goals

### 1.1 Current state

- **Ten built-in IM providers** (`feishu`, `slack`, `dingtalk`, `discord`, `telegram`, `wechat`, `wecom`, `qq`, `qqbot`, `email`) are enumerated in `src-im-bridge/cmd/bridge/platform_registry.go` as a ~280-line `providerDescriptors()` switch. Adding a new provider requires modifying that central switch and recompiling the Bridge binary — this contradicts the product goal "IM providers should be easy to extend".
- **`core.Platform` + 15 optional interfaces** (`CardSender`, `AttachmentSender`, `ReactionSender`, `ThreadOpener`, `HotReloader`, ...) already encode a rich, well-designed provider surface. The *contract* is good; only the *registration shape* is rigid.
- **Two plugin systems coexist** today with no unified visibility:
  1. IM Bridge command plugins (`src-im-bridge/core/plugin/`, YAML manifests with http/mcp/builtin invokers, tenant-scoped) — extends slash commands only.
  2. Go orchestrator plugins (`src-go/internal/plugin/`, WASM + firstparty-inproc, Marketplace-distributed) — handles CI/CD events, notifications, tools, review, workflow.
- **Bridge registration contract** (`POST /api/v1/im/bridge/register` + `IMBridgeRegisterRequest`) already carries `CapabilityMatrix`, `CallbackPaths`, `Tenants`, `TenantManifest` — but only for a single `Platform` string. When a Bridge process runs `IM_PLATFORMS=feishu,slack,dingtalk`, only the primary provider's inventory reaches the Go orchestrator. Command-plugin inventory is not reported at all.
- **`plugins/integrations/feishu-adapter/`** exists as a builtin bundle entry. Its source code path in `scripts/plugin/plugin-dev-targets.js` maps to `./cmd/sample-wasm-plugin`: it is **not an actual Feishu integration**; it is the sample WASM plugin wearing a misleading Feishu label. The 2026-04-21 plugin-system-expansion spec (§9 audit notes) already identifies IM platform adapters as IM Bridge's exclusive responsibility, so the `feishu-adapter` entry is a stale demo.

### 1.2 Goals

1. **Provider extensibility** — adding a new IM provider must require (a) one new package under `platform/<id>/`, (b) one `init()`-registered factory, (c) one blank import — and no edits to the central switch.
2. **Plugin-system compatibility** — the Go orchestrator's inventory view must cover every active provider and every loaded command plugin on every online Bridge, for read-only display and audit.
3. **Demo honesty** — remove the `feishu-adapter` label that claims IM transport responsibility which the IM Bridge actually owns, while preserving a working end-to-end WASM plugin example.

### 1.3 Non-goals

- Out-of-process / gRPC provider runtime (reserved as a future option; only a `MetadataSource` field placeholder is introduced).
- WASM-runtime providers (WASI lacks the reverse I/O — HTTP listeners, long-lived WS — most provider adapters need).
- Dynamic hot-load of providers from the control plane; enable/disable of providers from the Go orchestrator side.
- Tenant-level command allowlist UI, `tenants.yaml` → DB migration, Marketplace publishing of command plugins.
- Restructuring `core.Platform` or any of the optional sender/receiver interfaces.
- Changes to `POST /api/v1/im/notify` / HMAC-signed delivery contract / `/ws/im-bridge` delivery protocol / rate limiter / audit writer / sanitization / attachment staging.

---

## 2. Design Overview

Three coordinated changes, each independently shippable and testable:

| Section | Purpose | Surface affected |
|---------|---------|------------------|
| §3 Provider Factory Registry | Eliminate the `providerDescriptors()` switch; let each provider self-register | `src-im-bridge/core/`, `src-im-bridge/platform/<id>/`, `src-im-bridge/cmd/bridge/` |
| §4 Bridge registration inventory enrichment | Report every active provider + loaded command plugin to the Go orchestrator | `IMBridgeRegisterRequest` fields, `IMControlPlane.RegisterBridge`, Plugins page |
| §5 `feishu-adapter` rename to `sample-integration-plugin` | Remove the misleading IM-adapter label while keeping the WASM plugin demo | `plugins/integrations/`, `plugins/builtin-bundle.yaml`, dev-target scripts, fixture tests |

Order of implementation: §3 → §5 (parallel-safe with §3) → §4 (depends on `core.RegisteredProviders()` from §3).

---

## 3. Provider Factory Registry

### 3.1 New package-level API (`src-im-bridge/core/registry.go`)

```go
package core

import (
    "fmt"
    "strings"
    "sync"
    "time"
)

// ProviderFactory is the registration record a provider package publishes
// via init(). It must be registered exactly once per provider id.
type ProviderFactory struct {
    ID                      string
    Metadata                PlatformMetadata
    SupportedTransportModes []string
    // Features is an opaque hook for provider-specific capability advertising
    // (e.g. existing feishuProviderFeatures). Consumers type-assert when they
    // care; unknown feature shapes must be ignored.
    Features                any
    // EnvPrefixes declares the uppercase env-var namespaces this provider is
    // allowed to read via ProviderEnv. Cross-namespace reads panic in debug
    // builds and are refused in production (empty string returned).
    EnvPrefixes             []string

    ValidateConfig func(env ProviderEnv, mode string) error
    NewStub        func(env ProviderEnv) (Platform, error)
    NewLive        func(env ProviderEnv) (Platform, error)
}

// ProviderEnv is a read-only, namespace-gated view into the Bridge process
// environment. The adapter in cmd/bridge constructs one instance per provider
// factory invocation, populated with the current config struct.
type ProviderEnv interface {
    // Get returns the string value for key. key MUST begin with one of the
    // factory's declared EnvPrefixes; violations panic (debug) or return "".
    Get(key string) string
    BoolOr(key string, fallback bool) bool
    DurationOr(key string, fallback time.Duration) time.Duration
    // TestPort is a convenience accessor for the shared TEST_PORT value,
    // which every provider stub needs and is not namespaced.
    TestPort() string
}

var (
    providerRegistryMu sync.RWMutex
    providerRegistry   = map[string]ProviderFactory{}
)

// RegisterProvider records a factory. Panics on duplicate ID to surface
// misconfiguration at process start rather than at first provider lookup.
func RegisterProvider(f ProviderFactory) {
    id := strings.TrimSpace(f.ID)
    if id == "" {
        panic("core.RegisterProvider: empty provider id")
    }
    providerRegistryMu.Lock()
    defer providerRegistryMu.Unlock()
    if _, dup := providerRegistry[id]; dup {
        panic(fmt.Sprintf("core.RegisterProvider: duplicate provider id %q", id))
    }
    providerRegistry[id] = f
}

func LookupProvider(id string) (ProviderFactory, bool) {
    providerRegistryMu.RLock()
    defer providerRegistryMu.RUnlock()
    f, ok := providerRegistry[NormalizePlatformName(id)]
    return f, ok
}

// RegisteredProviders returns a stable-ordered snapshot for inventory /
// health-check callers.
func RegisteredProviders() []ProviderFactory {
    providerRegistryMu.RLock()
    defer providerRegistryMu.RUnlock()
    out := make([]ProviderFactory, 0, len(providerRegistry))
    for _, f := range providerRegistry {
        out = append(out, f)
    }
    // sorted by ID for deterministic tests / inventory
    sortProviderFactories(out)
    return out
}

// TransportMode constants (lifted from cmd/bridge) move here so providers can
// name them without importing cmd/bridge.
const (
    TransportModeStub = "stub"
    TransportModeLive = "live"
)
```

### 3.2 Per-provider registration (`src-im-bridge/platform/<id>/register.go`)

One file per provider. Example for `feishu`:

```go
package feishu

import (
    "fmt"
    "strings"

    "github.com/agentforge/im-bridge/core"
)

func init() {
    core.RegisterProvider(core.ProviderFactory{
        ID:       "feishu",
        Metadata: liveMetadata,  // existing package-level constant
        SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
        Features: feishuFeatures, // existing feishuProviderFeatures
        EnvPrefixes: []string{"FEISHU_"},
        ValidateConfig: func(env core.ProviderEnv, mode string) error {
            if mode == core.TransportModeLive {
                if env.Get("FEISHU_APP_ID") == "" || env.Get("FEISHU_APP_SECRET") == "" {
                    return fmt.Errorf("selected platform feishu requires FEISHU_APP_ID and FEISHU_APP_SECRET for live transport")
                }
            }
            return nil
        },
        NewStub: func(env core.ProviderEnv) (core.Platform, error) {
            return NewStub(env.TestPort()), nil
        },
        NewLive: func(env core.ProviderEnv) (core.Platform, error) {
            opts := make([]LiveOption, 0, 1)
            if strings.TrimSpace(env.Get("FEISHU_VERIFICATION_TOKEN")) != "" {
                opts = append(opts, WithCardCallbackWebhook(
                    env.Get("FEISHU_VERIFICATION_TOKEN"),
                    env.Get("FEISHU_EVENT_ENCRYPT_KEY"),
                    env.Get("FEISHU_CALLBACK_PATH"),
                ))
            }
            return NewLive(env.Get("FEISHU_APP_ID"), env.Get("FEISHU_APP_SECRET"), opts...)
        },
    })
}
```

Each of the 10 existing provider packages gets one `register.go`. The logic inside each `ValidateConfig` / `NewStub` / `NewLive` is **lifted verbatim** from the matching case in `providerDescriptors()` — no behavior change.

### 3.3 `cmd/bridge/platform_registry.go` contraction

- `providerDescriptors()` is deleted (~280 lines).
- `platformDescriptors()` becomes a thin shim returning the aggregated factories (legacy alias).
- `lookupProviderDescriptor(name)` becomes:
  ```go
  func lookupProviderDescriptor(name string) (providerDescriptor, error) {
      f, ok := core.LookupProvider(name)
      if !ok {
          return providerDescriptor{}, fmt.Errorf("unsupported IM_PLATFORM %q", name)
      }
      return adaptFactoryToDescriptor(f), nil
  }
  ```
- `cmd/bridge/main.go` gains blank imports that run the `init()` registration side effect:
  ```go
  import (
      _ "github.com/agentforge/im-bridge/platform/dingtalk"
      _ "github.com/agentforge/im-bridge/platform/discord"
      _ "github.com/agentforge/im-bridge/platform/email"
      _ "github.com/agentforge/im-bridge/platform/feishu"
      _ "github.com/agentforge/im-bridge/platform/qq"
      _ "github.com/agentforge/im-bridge/platform/qqbot"
      _ "github.com/agentforge/im-bridge/platform/slack"
      _ "github.com/agentforge/im-bridge/platform/telegram"
      _ "github.com/agentforge/im-bridge/platform/wechat"
      _ "github.com/agentforge/im-bridge/platform/wecom"
  )
  ```
- The `providerDescriptor` / `platformDescriptor` / `activeProvider` types stay — they are used by multi-provider wiring and tests. They become thin adapters over `core.ProviderFactory` rather than authoritative registration records.

### 3.4 `ProviderEnv` implementation

`cmd/bridge/provider_env.go` (new) produces a `ProviderEnv` backed by the current `config` struct + `os.Getenv` fallback, gated by the factory's `EnvPrefixes`. The implementation:

```go
type cfgProviderEnv struct {
    cfg      *config
    prefixes []string
}

func (e *cfgProviderEnv) Get(key string) string {
    upper := strings.ToUpper(strings.TrimSpace(key))
    for _, p := range e.prefixes {
        if strings.HasPrefix(upper, p) {
            return lookupCfgField(e.cfg, upper) // or os.Getenv fallback
        }
    }
    // debug build: panic("cross-namespace env read: " + upper)
    return ""
}
```

`lookupCfgField` is a small switch that maps env names to the existing `config` struct fields (so callers never do a second `os.Getenv`).

### 3.5 New-provider workflow (the extensibility payoff)

1. `mkdir src-im-bridge/platform/rocketchat/`
2. Copy `platform/slack/stub.go` + `live.go` + `renderer.go` as starting templates (or use an optional `scripts/scaffold-im-provider.sh`, scoped out of this spec).
3. Write `register.go` with `ID: "rocketchat"`, `EnvPrefixes: []string{"ROCKETCHAT_"}`, and the provider-specific factory logic.
4. Add one blank import line to `cmd/bridge/main.go`.
5. Write `stub_test.go` + `live_test.go` per existing patterns.
6. Run `go test ./...`.

Zero edits to `cmd/bridge/platform_registry.go`. Zero edits to `core/`.

### 3.6 Risks & tradeoffs

- **`init()` side effects** are a standard Go pattern (`database/sql` drivers). Missing a blank import silently drops a provider from the registry. §3.7's `TestAllBuiltinProvidersRegistered` catches this in CI.
- **EnvPrefixes enforcement** is a safety rail, not a security boundary — any package compiled into the Bridge can still call `os.Getenv` directly. The rail exists to prevent accidental coupling ("I need one env var from another provider, let me just read it") from silently becoming permanent.
- **Email provider** is treated as an IM provider by the current Bridge (`platform/email/`). This spec preserves that status quo — the factory registry covers email unchanged. Whether email belongs in IM Bridge at all is an independent decision, explicitly out of scope.

### 3.7 Tests

- `core/registry_test.go`:
  - Register + lookup roundtrip
  - Duplicate registration panics
  - Unknown lookup returns `false`
  - `RegisteredProviders()` returns deterministic order
- `core/provider_env_test.go`:
  - Namespace hit returns env value
  - Cross-namespace read returns `""` (and panics under `-tags debug`)
- `cmd/bridge/all_providers_registered_test.go` (the critical guard):
  ```go
  func TestAllBuiltinProvidersRegistered(t *testing.T) {
      expected := []string{"dingtalk", "discord", "email", "feishu", "qq",
                            "qqbot", "slack", "telegram", "wechat", "wecom"}
      got := map[string]bool{}
      for _, f := range core.RegisteredProviders() {
          got[f.ID] = true
      }
      for _, id := range expected {
          if !got[id] {
              t.Errorf("provider %q not registered; blank import missing from cmd/bridge/main.go?", id)
          }
      }
  }
  ```
- Existing `multi_provider_test.go`, `platform_registry_test.go`, `provider_contract_test.go` keep passing unchanged.

---

## 4. Bridge Registration Inventory Enrichment

### 4.1 Extend `IMBridgeRegisterRequest` (additive, back-compat)

`src-go/internal/model/im.go`:

```go
type IMBridgeRegisterRequest struct {
    BridgeID         string
    Platform         string             // KEEP: primary provider, back-compat
    Transport        string             // KEEP: primary provider, back-compat
    ProjectIDs       []string
    Capabilities     map[string]bool
    CapabilityMatrix map[string]any     // KEEP: primary provider
    CallbackPaths    []string
    Metadata         map[string]string
    Tenants          []string
    TenantManifest   []IMTenantBinding

    // NEW
    Providers        []IMBridgeProvider       `json:"providers,omitempty"`
    CommandPlugins   []IMBridgeCommandPlugin  `json:"commandPlugins,omitempty"`
}

// IMBridgeProvider describes one IM provider active on the Bridge.
// When a Bridge hosts a single provider, Providers[0] content mirrors the
// top-level Platform/Transport/CapabilityMatrix/CallbackPaths/Tenants fields.
type IMBridgeProvider struct {
    ID               string            `json:"id"`
    Transport        string            `json:"transport"`
    ReadinessTier    string            `json:"readinessTier,omitempty"`
    CapabilityMatrix map[string]any    `json:"capabilityMatrix,omitempty"`
    CallbackPaths    []string          `json:"callbackPaths,omitempty"`
    Tenants          []string          `json:"tenants,omitempty"`
    // MetadataSource is "builtin" today. Reserved values: "oop-grpc" (future
    // out-of-process providers) — do not rely on any value other than "builtin"
    // for behavior decisions; this field is informational.
    MetadataSource   string            `json:"metadataSource"`
}

// IMBridgeCommandPlugin mirrors a loaded core/plugin manifest.
type IMBridgeCommandPlugin struct {
    ID         string   `json:"id"`
    Version    string   `json:"version"`
    Commands   []string `json:"commands"`
    Tenants    []string `json:"tenants,omitempty"`
    SourcePath string   `json:"sourcePath,omitempty"`
}
```

`IMBridgeInstance` mirrors the same two new fields. The existing `GetBridgeStatus` response is the inventory map — no new field on top, no new endpoint.

### 4.2 Bridge-side assembly

New file `src-im-bridge/cmd/bridge/inventory.go`:

```go
package main

import (
    "github.com/agentforge/im-bridge/core"
    "github.com/agentforge/im-bridge/core/plugin"
)

type RegistrationInventory struct {
    Providers      []bridgeProviderRecord
    CommandPlugins []bridgeCommandPluginRecord
}

func buildRegistrationInventory(providers []*activeProvider, pluginReg *plugin.Registry) RegistrationInventory {
    inv := RegistrationInventory{}
    for _, p := range providers {
        md := p.Metadata()
        inv.Providers = append(inv.Providers, bridgeProviderRecord{
            ID:               md.Source,
            Transport:        p.TransportMode,
            ReadinessTier:    md.Capabilities.ReadinessTier,
            // Reuse the same capability-matrix serialization emitted by the
            // existing /im/health handler so wire format stays canonical.
            // Extract it into a shared helper in core/platform_metadata.go if
            // one does not exist yet.
            CapabilityMatrix: core.CapabilityMatrixJSON(md.Capabilities),
            CallbackPaths:    collectCallbackPaths(p), // small helper; may reuse existing metadata fields
            Tenants:          append([]string(nil), p.Tenants...),
            MetadataSource:   "builtin",
        })
    }
    if pluginReg != nil {
        inv.CommandPlugins = pluginReg.Snapshot() // new method, §4.3
    }
    return inv
}
```

The records are serialized into the `Providers` + `CommandPlugins` fields of the register request that `cmd/bridge/control_plane.go` already sends.

### 4.3 `core/plugin.Registry.Snapshot()`

```go
// Snapshot returns a stable-ordered inventory of every loaded manifest.
// Intended for control-plane reporting; callers MUST NOT mutate the result.
func (r *Registry) Snapshot() []IMBridgeCommandPlugin {
    r.mu.RLock()
    defer r.mu.RUnlock()
    out := make([]IMBridgeCommandPlugin, 0, len(r.plugins))
    for _, p := range r.plugins {
        commands := make([]string, 0, len(p.Manifest.Commands))
        for _, c := range p.Manifest.Commands {
            commands = append(commands, c.Slash)
        }
        out = append(out, IMBridgeCommandPlugin{
            ID:         p.Manifest.ID,
            Version:    p.Manifest.Version,
            Commands:   commands,
            Tenants:    append([]string(nil), p.Manifest.Tenants...),
            SourcePath: p.Path,
        })
    }
    sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
    return out
}
```

The `IMBridgeCommandPlugin` struct is declared in a new file `src-im-bridge/core/plugin/inventory.go` and mirrors the Go orchestrator's model struct field-for-field (JSON wire compatibility).

### 4.4 Re-registration triggers (no new WS channel)

The Bridge calls `/im/bridge/register` on the following events, reusing the existing upsert semantics of `IMControlPlane.RegisterBridge`:

| Event | Already-existing hook | New behavior |
|-------|----------------------|--------------|
| Bridge startup | `cmd/bridge/control_plane.go` startup sequence | Include `Providers` + `CommandPlugins` in the payload |
| SIGHUP tenants.yaml reload | existing signal handler | Call `Register` again with refreshed `Tenants` per provider |
| `plugin.Registry.ReloadAll` success | existing polling watcher | Call `Register` again with refreshed `CommandPlugins` |
| `HotReloader.Reconcile` returns non-empty `Applied` | existing `Reconcile` path | Call `Register` again (provider capability may have changed) |

Heartbeat payload is **not** extended — heartbeat remains minimal and frequent; inventory is pushed only on change via registration re-entry.

### 4.5 Go orchestrator side

- `IMControlPlane.RegisterBridge`: copy the two new fields into `IMBridgeInstance`. ~2 added lines.
- **Resolution precedence**: when a request carries a non-empty `Providers` slice, that slice is authoritative for inventory display. The top-level `Platform` / `Transport` / `CapabilityMatrix` remain populated (for delivery routing code paths that already read them), but the frontend reads `Providers`. When `Providers` is empty (older Bridge), the orchestrator synthesizes a single-entry `Providers` from the top-level fields at response time so downstream consumers see a uniform shape.
- `GetBridgeStatus`: unchanged; new fields flow through automatically because the response serializes the whole `IMBridgeInstance`.
- No new endpoint, no migration, no new table. `IMControlPlane.instances` stays in-memory.

### 4.6 Frontend

- New component `components/im/bridge-inventory-panel.tsx`, read-only:
  - Groups by `bridgeId`
  - For each bridge: list of providers (id, transport, readiness tier, capability matrix summary, tenant badges) + list of command plugins (id, version, commands).
- `app/(dashboard)/plugins/page.tsx` inserts the panel under a new "IM Bridge Providers" section header. The panel is suppressed when no bridge is online.
- Data source is the existing `GET /api/v1/im/bridge/status` — no new HTTP route, no new Zustand store (selector-only add if needed).

### 4.7 Duplication audit

Explicitly verified **not** duplicated:
- ❌ No new WS message type — reuses `/im/bridge/register` upsert.
- ❌ No new HTTP route — reuses `GET /im/bridge/status`.
- ❌ No new DB table / migration — reuses the existing in-memory `IMControlPlane.instances` map.
- ❌ No new Zustand store — selector-only integration with existing IM/plugin stores.
- ❌ No parallel bridge registry — `IMBridgeRegisterRequest` is **extended** with additive optional fields.

### 4.8 Risks & tradeoffs

- **Stale inventory on crashed Bridge**: existing heartbeat-expiry marks the bridge offline; the frontend should grey out offline bridges' inventory rather than delete it. This is the same behavior as today for single-provider bridges.
- **Multi-bridge divergence**: two Bridges running the same provider + same tenant both appear in `GetBridgeStatus` — the frontend lists them as separate rows. No arbitration; aligns with current delivery-routing behavior (first match wins).
- **Re-register storm on repeated SIGHUP / plugin reloads**: `RegisterBridge` is an O(1) in-memory upsert; an occasional burst is inconsequential. If a pathological case emerges, add a 1 s debounce in `cmd/bridge/control_plane.go`; out of scope.
- **Wire-format skew**: an older Bridge binary against a newer orchestrator omits the new fields — orchestrator treats `Providers == nil` as "single-provider legacy" and derives a synthetic single-entry inventory from `Platform` + `CapabilityMatrix`. A newer Bridge against an older orchestrator: extra fields ignored at unmarshal. Back-compat in both directions.

### 4.9 Tests

- `core/plugin/inventory_test.go`: `Snapshot()` ordering, empty registry, tenant cloning.
- `cmd/bridge/inventory_test.go`: multi-provider + command plugin composition.
- Extend `src-go/internal/service/im_control_plane_test.go`:
  - Register a bridge with `Providers: [{id: "feishu"}, {id: "slack"}]` + two command plugins; assert `GetBridgeStatus` returns them intact.
  - Register with legacy payload (no `Providers` field); assert `GetBridgeStatus` synthesizes a single-provider entry from `Platform`.
- Frontend: render test for `bridge-inventory-panel.tsx` with three scenarios (single provider, multi provider, offline bridge).

---

## 5. `feishu-adapter` → `sample-integration-plugin` Rename

### 5.1 Rationale

`scripts/plugin/plugin-dev-targets.js` maps `feishu-adapter` to `./cmd/sample-wasm-plugin` — it has never been a real Feishu integration. Its Feishu-themed manifest (`FEISHU_APP_ID`, `FEISHU_APP_SECRET` configuration entries, `integration, im` tags) misleadingly suggests IM transport responsibility that belongs exclusively to the IM Bridge. The spec `2026-04-21-plugin-system-expansion-design.md` §9 already calls out the ChannelAdapter anti-pattern and removes all other IM WASM adapters from scope; the `feishu-adapter` label is the last remnant.

### 5.2 Rename plan

**Filesystem:**
- `plugins/integrations/feishu-adapter/` → `plugins/integrations/sample-integration-plugin/`
- `plugins/integrations/sample-integration-plugin/dist/feishu.wasm` → `.../dist/sample-integration.wasm`

**Manifest (`plugins/integrations/sample-integration-plugin/manifest.yaml`):**
- `metadata.id`: `sample-integration-plugin`
- `metadata.name`: `Sample Integration Plugin`
- `metadata.description`: `Reference Go WASM integration plugin demonstrating the plugin ABI, health check, and echo operation. Use as a starting template for new Integration Plugins.`
- `metadata.tags`: `[builtin, sample, demo, wasm]` (no `integration`, no `im`)
- `spec.module`: `./dist/sample-integration.wasm`
- `spec.capabilities`: `[health, echo]` (replacing `send_message`, which implied IM transport)
- `spec.config`: remove `mode: webhook`; optional sample config only
- `permissions.network.required`: `false` (the sample plugin does not call out)
- Remove `FEISHU_APP_ID` / `FEISHU_APP_SECRET` configuration entries entirely

**`plugins/builtin-bundle.yaml`:**
- Update the `feishu-adapter` entry (lines 186–214): `id` → `sample-integration-plugin`, `manifest` path, `docsRef` → `docs/guides/plugin-wasm.md`, readiness messages describe a reference demo rather than a Feishu integration, remove `FEISHU_*` env configuration.

**`scripts/plugin/plugin-dev-targets.js`:**
- `DEFAULT_GO_WASM_MANIFEST_PATH` → `plugins/integrations/sample-integration-plugin/manifest.yaml`
- `MAINTAINED_GO_WASM_TARGETS`: rename the `"feishu-adapter"` key to `"sample-integration-plugin"` (source path unchanged: `./cmd/sample-wasm-plugin`).

**Test fixtures:**
- `scripts/plugin/build-go-wasm-plugin.test.ts` — replace literal `"feishu-adapter"` / `"feishu.wasm"` tokens.
- `scripts/plugin/debug-go-wasm-plugin.test.ts` — same.
- `scripts/plugin/verify-plugin-dev-workflow.test.ts` — same.
- `scripts/plugin/verify-built-in-plugin-bundle.test.ts` — same.

**Documentation (narrative edits, minimal):**
- `plugins/README.md`, `docs/architecture/wasm-plugin-runtime.md`, `docs/guides/plugin-wasm.md`, `docs/guides/plugin-development.md`, top-level `README.md` / `README_zh.md`, `docs/product/prd.md`: any "we ship a Feishu adapter" phrasing becomes "IM platform support lives in IM Bridge; the plugin system ships `sample-integration-plugin` as a WASM example."
- Archived OpenSpec changes (`openspec/changes/archive/2026-03-25-pluginize-im-bridge-feishu-capabilities/`) are **not touched** — archives are historical facts.

### 5.3 Scope guardrail

- **Do not** modify `src-go/cmd/sample-wasm-plugin/main.go` — keep the source code unchanged; the rename is purely label + manifest. Mixing source-code evolution with the rename confuses the history.
- **Do not** delete the artifact outright — losing the end-to-end example (manifest + build path + bundle entry + dev-workflow tests) costs more for new plugin authors than the renaming effort.

### 5.4 Tests

No new tests. The existing four fixture-based TS tests are the verification: they pass against the renamed artifact iff the rename is complete.

---

## 6. Implementation Order

1. **§3 Provider Factory Registry** — one PR / commit:
   1. Add `core/registry.go` + `core/provider_env.go` + their tests.
   2. Add 10 `platform/<id>/register.go` files.
   3. Contract `cmd/bridge/platform_registry.go`; add blank imports in `main.go`.
   4. Add `all_providers_registered_test.go`.
   5. Verify `go test ./...` green.

2. **§5 `feishu-adapter` rename** — independent commit, safe to land in parallel with §3:
   1. Rename directory + module artifact.
   2. Update manifest, builtin bundle, `plugin-dev-targets.js`, four TS test fixtures.
   3. Documentation narrative edits.
   4. Verify `pnpm test` + plugin workflow scripts pass.

3. **§4 Bridge registration inventory** — depends on §3 (`core.RegisteredProviders`):
   1. Extend `IMBridgeRegisterRequest` / `IMBridgeInstance` models (`src-go`).
   2. Add `core/plugin.Registry.Snapshot` + inventory struct.
   3. Add `cmd/bridge/inventory.go` + tests.
   4. Wire registration payload construction.
   5. Add SIGHUP / plugin-reload / reconcile re-register triggers.
   6. Extend orchestrator control plane tests.
   7. Ship frontend panel last.

---

## 7. What This Does Not Change

- `core.Platform` and its 15 optional interfaces: stable.
- `/api/v1/im/notify` delivery contract, HMAC signing, dedupe, sanitization, rate limiting, audit writer, attachment staging: stable.
- `/ws/im-bridge` delivery / ack / progress messages: stable.
- `POST /api/v1/im/bridge/register` / `/heartbeat` / `/unregister`: unchanged methods; `/register` payload gains optional fields only.
- `IMControlPlane.instances` in-memory storage: unchanged.
- `notification-fanout` firstparty-inproc plugin: unchanged; it still calls into IM Bridge via the existing `/im/notify` path.
- Plugin control plane in Go orchestrator (`src-go/internal/plugin/`), Marketplace, plugin permissions: unchanged.
- Review pipeline, workflow executor routing, tool-chain executor, scheduler: unchanged.
- `tenants.yaml` schema, SIGHUP reload, tenant resolver, command allowlist: unchanged.
- Rate limiter policies / egress sanitization / reaction emoji map / thread policy semantics: unchanged.

---

## 8. Explicit Out-of-Scope

- **Out-of-process provider runtime (gRPC / HTTP adapter).** The `IMBridgeProvider.MetadataSource` field reserves the `"oop-grpc"` value for a future spec; no implementation in this scope.
- **Hot-load / dynamic enable-disable of providers from the Go orchestrator control plane.** Bridge's authoritative source for provider loading remains `IM_PLATFORMS` + env vars.
- **Tenant-level command allowlist UI / Marketplace publishing of command plugins.**
- **`tenants.yaml` → database migration.**
- **Email provider re-classification** (currently treated as IM; whether it belongs elsewhere is independent).
- **WASM-runtime IM providers.**
- **Rewriting `core.Platform` or any optional sender/receiver interface.**
- **Multi-bridge provider-conflict arbitration** beyond the existing heartbeat + first-match routing.

---

## 9. Audit Notes

- The existing `2026-04-21-plugin-system-expansion-design.md` §9 explicitly identifies IM platform adapters as IM Bridge's responsibility and removes the ChannelAdapter abstraction. This spec reinforces that boundary by (a) making IM Bridge's own provider extensibility story coherent and (b) purging the last surviving "IM adapter as WASM plugin" artifact (`feishu-adapter`).
- The existing `IMBridgeRegisterRequest` already carries half of the inventory story (`CapabilityMatrix`, `CallbackPaths`, `Tenants`). The extensions in §4 fill the two remaining gaps (multi-provider bridges and command-plugin visibility) without introducing any parallel bridge-tracking mechanism.
- The `core.Platform` optional-interface pattern (`CardSender`, `AttachmentSender`, `ReactionSender`, `ThreadOpener`, ...) is treated as load-bearing and untouched. Providers are extensible along one axis (registration); the capability surface they implement is extensible along the interface-satisfaction axis — both are existing Go idioms.
