# IM Bridge Provider Extensibility & Plugin System Alignment — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make IM providers self-register via a `core.ProviderFactory` registry (removing the 280-line central switch), surface multi-provider + command-plugin inventory through the existing `/im/bridge/register` contract (no new endpoint / table / WS channel), and rename the misleading `feishu-adapter` WASM demo to `sample-integration-plugin`.

**Architecture:** Three phases, each independently testable. Phase A adds `core.RegisterProvider` + `init()`-based registration per platform. Phase B renames the demo plugin (no source-code change, pure label cleanup). Phase C extends `IMBridgeRegisterRequest` with `Providers []IMBridgeProvider` + `CommandPlugins []IMBridgeCommandPlugin`, hoists the register call to once per Bridge process (fixing a latent multi-provider overwrite), and ships a read-only frontend panel.

**Tech Stack:** Go 1.23+, wazero (unchanged), Echo (backend), Next.js 16 / React 19 (frontend), existing `src-im-bridge/client` AgentForge HTTP client, existing `src-im-bridge/core/plugin` manifest registry.

**Spec:** `docs/superpowers/specs/2026-04-21-im-bridge-provider-plugin-alignment-design.md`

---

## File Map

**Created:**
- `src-im-bridge/core/registry.go` — `ProviderFactory`, `ProviderEnv` interface, `RegisterProvider`, `LookupProvider`, `RegisteredProviders`
- `src-im-bridge/core/registry_test.go`
- `src-im-bridge/cmd/bridge/provider_env.go` — `cfgProviderEnv` backed by `*config` + env var fallback
- `src-im-bridge/cmd/bridge/provider_env_test.go`
- `src-im-bridge/platform/{feishu,slack,dingtalk,discord,telegram,wechat,wecom,qq,qqbot,email}/register.go` — 10 files, each calling `core.RegisterProvider` via `init()`
- `src-im-bridge/cmd/bridge/all_providers_registered_test.go`
- `src-im-bridge/core/plugin/inventory.go` — `IMBridgeCommandPlugin` struct + `Registry.Snapshot()`
- `src-im-bridge/core/plugin/inventory_test.go`
- `src-im-bridge/cmd/bridge/inventory.go` — `buildRegistrationInventory(providers, pluginReg)` + `registerBridgeOnce(ctx, client, bridgeID, inv, cfg)` one-shot call
- `src-im-bridge/cmd/bridge/inventory_test.go`
- `components/im/bridge-inventory-panel.tsx`
- `components/im/bridge-inventory-panel.test.tsx`

**Modified:**
- `src-im-bridge/cmd/bridge/platform_registry.go` — `providerDescriptors()` switch removed; replaced by thin adapter over `core.LookupProvider` / `core.RegisteredProviders`
- `src-im-bridge/cmd/bridge/main.go` — blank imports for 10 provider packages; registration moved out of `bridgeRuntimeControl`
- `src-im-bridge/cmd/bridge/control_plane.go` — `bridgeRuntimeControl.Start()` no longer calls `RegisterBridge`; re-register triggers added on SIGHUP + plugin reload
- `src-im-bridge/core/plugin/plugin.go` — add `Snapshot()` method
- `src-im-bridge/client/agentforge.go` — `BridgeRegistration` + `BridgeInstance` gain `Providers` + `CommandPlugins` fields
- `src-go/internal/model/im.go` — `IMBridgeRegisterRequest` + `IMBridgeInstance` gain the same fields
- `src-go/internal/service/im_control_plane.go` — `RegisterBridge` copies new fields; `GetBridgeStatus` synthesizes `Providers[0]` for legacy bridges
- `src-go/internal/service/im_control_plane_test.go` — multi-provider + command-plugin cases
- `plugins/integrations/feishu-adapter/` → `plugins/integrations/sample-integration-plugin/` (renamed directory)
- `plugins/integrations/sample-integration-plugin/manifest.yaml` — id/name/tags/capabilities rewritten
- `plugins/integrations/sample-integration-plugin/dist/feishu.wasm` → `sample-integration.wasm`
- `plugins/builtin-bundle.yaml` — `feishu-adapter` entry renamed
- `scripts/plugin/plugin-dev-targets.js` — `DEFAULT_GO_WASM_MANIFEST_PATH` + `MAINTAINED_GO_WASM_TARGETS`
- `scripts/plugin/build-go-wasm-plugin.test.ts`
- `scripts/plugin/debug-go-wasm-plugin.test.ts`
- `scripts/plugin/verify-plugin-dev-workflow.test.ts`
- `scripts/plugin/verify-built-in-plugin-bundle.test.ts`
- `app/(dashboard)/plugins/page.tsx` — render `<BridgeInventoryPanel />` section
- Narrative docs: `plugins/README.md`, `docs/architecture/wasm-plugin-runtime.md`, `docs/guides/plugin-wasm.md`, `docs/guides/plugin-development.md`, `README.md`, `README_zh.md`, `docs/product/prd.md`

---

# Phase A — Provider Factory Registry

## Task A1: Core Provider Registry primitives

Introduce `ProviderFactory`, `ProviderEnv` interface, and the registration functions. These are pure in-process state — no consumers yet.

**Files:**
- Create: `src-im-bridge/core/registry.go`
- Create: `src-im-bridge/core/registry_test.go`

- [ ] **Step 1: Write the failing test**

Create `src-im-bridge/core/registry_test.go`:

```go
package core_test

import (
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func resetRegistry(t *testing.T) {
	t.Helper()
	// TestOnly_ResetProviderRegistry is only exposed to _test.go via a
	// companion file; provided to let tests run in any order.
	core.TestOnly_ResetProviderRegistry()
}

func TestRegisterAndLookupProvider(t *testing.T) {
	resetRegistry(t)

	f := core.ProviderFactory{
		ID:                      "unittest-platform",
		SupportedTransportModes: []string{core.TransportModeStub},
		EnvPrefixes:             []string{"UNITTEST_"},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return nil, nil
		},
	}
	core.RegisterProvider(f)

	got, ok := core.LookupProvider("unittest-platform")
	if !ok {
		t.Fatalf("LookupProvider unittest-platform: not found")
	}
	if got.ID != "unittest-platform" {
		t.Errorf("ID = %q, want unittest-platform", got.ID)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	resetRegistry(t)
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "dup",
		SupportedTransportModes: []string{core.TransportModeStub},
		NewStub:                 func(env core.ProviderEnv) (core.Platform, error) { return nil, nil },
	})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "dup",
		SupportedTransportModes: []string{core.TransportModeStub},
		NewStub:                 func(env core.ProviderEnv) (core.Platform, error) { return nil, nil },
	})
}

func TestRegisterEmptyIDPanics(t *testing.T) {
	resetRegistry(t)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on empty id")
		}
	}()
	core.RegisterProvider(core.ProviderFactory{ID: "   "})
}

func TestLookupUnknownReturnsFalse(t *testing.T) {
	resetRegistry(t)
	if _, ok := core.LookupProvider("nope"); ok {
		t.Error("LookupProvider nope = ok, want false")
	}
}

func TestRegisteredProvidersStableOrder(t *testing.T) {
	resetRegistry(t)
	for _, id := range []string{"c-platform", "a-platform", "b-platform"} {
		core.RegisterProvider(core.ProviderFactory{
			ID:                      id,
			SupportedTransportModes: []string{core.TransportModeStub},
			NewStub:                 func(env core.ProviderEnv) (core.Platform, error) { return nil, nil },
		})
	}
	list := core.RegisteredProviders()
	if len(list) != 3 {
		t.Fatalf("got %d providers, want 3", len(list))
	}
	want := []string{"a-platform", "b-platform", "c-platform"}
	for i, f := range list {
		if f.ID != want[i] {
			t.Errorf("index %d = %q, want %q", i, f.ID, want[i])
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-im-bridge && go test ./core/ -run TestRegister -v
```

Expected: compile error — `TestOnly_ResetProviderRegistry`, `RegisterProvider`, `LookupProvider`, `RegisteredProviders`, `TransportModeStub`, `ProviderFactory`, `ProviderEnv` undefined.

- [ ] **Step 3: Implement `registry.go`**

Create `src-im-bridge/core/registry.go`:

```go
package core

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Transport mode string constants used by provider factories and the
// cmd/bridge selection code. These live in core so provider packages do
// not need to import cmd/bridge.
const (
	TransportModeStub = "stub"
	TransportModeLive = "live"
)

// ProviderFactory is the self-registering descriptor each IM provider
// package publishes via init(). Each provider id must be registered
// exactly once — duplicate registration panics.
type ProviderFactory struct {
	// ID is the normalized provider identifier (e.g. "feishu", "slack").
	ID string
	// Metadata mirrors what the provider's stub/live adapters report via
	// Platform.Metadata(). Used by inventory reporting.
	Metadata PlatformMetadata
	// SupportedTransportModes enumerates the transport modes this factory
	// can build, e.g. ["stub", "live"].
	SupportedTransportModes []string
	// Features is an opaque, provider-specific capability record. Consumers
	// that care (e.g. Feishu card rendering helpers) type-assert; others
	// ignore the field.
	Features any
	// EnvPrefixes declares the uppercase env-var namespaces this factory is
	// allowed to read through ProviderEnv. Cross-namespace reads return ""
	// to prevent silent coupling between providers.
	EnvPrefixes []string

	ValidateConfig func(env ProviderEnv, mode string) error
	NewStub        func(env ProviderEnv) (Platform, error)
	NewLive        func(env ProviderEnv) (Platform, error)
}

// ProviderEnv is a read-only, namespace-gated view into the Bridge process
// environment. cmd/bridge constructs one per factory invocation, backed by
// the loaded config struct.
type ProviderEnv interface {
	// Get returns the string value for key. The key must begin with one of
	// the factory's declared EnvPrefixes; violations return "".
	Get(key string) string
	BoolOr(key string, fallback bool) bool
	DurationOr(key string, fallback time.Duration) time.Duration
	// TestPort is the shared TEST_PORT value every stub adapter needs.
	// It is intentionally not namespace-gated.
	TestPort() string
}

var (
	providerRegistryMu sync.RWMutex
	providerRegistry   = map[string]ProviderFactory{}
)

// RegisterProvider records a factory. Panics on empty or duplicate ID so
// misconfiguration surfaces at process startup.
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

// LookupProvider returns the factory registered for id, if any. The lookup
// normalizes id the same way NormalizePlatformName does so callers can
// pass raw env values.
func LookupProvider(id string) (ProviderFactory, bool) {
	providerRegistryMu.RLock()
	defer providerRegistryMu.RUnlock()
	f, ok := providerRegistry[NormalizePlatformName(id)]
	return f, ok
}

// RegisteredProviders returns a deterministic snapshot sorted by ID.
func RegisteredProviders() []ProviderFactory {
	providerRegistryMu.RLock()
	defer providerRegistryMu.RUnlock()
	out := make([]ProviderFactory, 0, len(providerRegistry))
	for _, f := range providerRegistry {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
```

- [ ] **Step 4: Add the test-only reset helper**

Create `src-im-bridge/core/registry_testhelpers.go`:

```go
package core

// TestOnly_ResetProviderRegistry clears the global provider registry.
// Intended for tests that register fixtures and need a clean slate. The
// `TestOnly_` prefix ensures calls are grep-able; using this in production
// code is a bug.
func TestOnly_ResetProviderRegistry() {
	providerRegistryMu.Lock()
	defer providerRegistryMu.Unlock()
	providerRegistry = map[string]ProviderFactory{}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd src-im-bridge && go test ./core/ -run TestRegister -v
cd src-im-bridge && go test ./core/ -run TestLookup -v
cd src-im-bridge && go test ./core/ -run TestRegisteredProviders -v
```

Expected: all PASS (5 sub-tests in the file).

- [ ] **Step 6: Commit**

```bash
git add src-im-bridge/core/registry.go src-im-bridge/core/registry_testhelpers.go src-im-bridge/core/registry_test.go
git commit -m "feat(im-bridge): add core.ProviderFactory registry"
```

---

## Task A2: cmd/bridge ProviderEnv implementation

Back `core.ProviderEnv` with the existing `*config` struct so factories get a namespace-gated view into loaded env values.

**Files:**
- Create: `src-im-bridge/cmd/bridge/provider_env.go`
- Create: `src-im-bridge/cmd/bridge/provider_env_test.go`

- [ ] **Step 1: Write the failing test**

Create `src-im-bridge/cmd/bridge/provider_env_test.go`:

```go
package main

import (
	"testing"
	"time"
)

func TestCfgProviderEnv_NamespaceHit(t *testing.T) {
	cfg := &config{FeishuApp: "cli_abc", FeishuSec: "sec_xyz", TestPort: "7780"}
	env := newCfgProviderEnv(cfg, []string{"FEISHU_"})
	if got := env.Get("FEISHU_APP_ID"); got != "cli_abc" {
		t.Errorf("Get FEISHU_APP_ID = %q, want cli_abc", got)
	}
	if got := env.Get("FEISHU_APP_SECRET"); got != "sec_xyz" {
		t.Errorf("Get FEISHU_APP_SECRET = %q, want sec_xyz", got)
	}
	if got := env.TestPort(); got != "7780" {
		t.Errorf("TestPort = %q, want 7780", got)
	}
}

func TestCfgProviderEnv_CrossNamespaceReturnsEmpty(t *testing.T) {
	cfg := &config{FeishuApp: "cli_abc", SlackBotToken: "xoxb-xxx"}
	env := newCfgProviderEnv(cfg, []string{"FEISHU_"})
	if got := env.Get("SLACK_BOT_TOKEN"); got != "" {
		t.Errorf("cross-namespace Get SLACK_BOT_TOKEN = %q, want empty", got)
	}
}

func TestCfgProviderEnv_BoolOr(t *testing.T) {
	cfg := &config{EmailSMTPTLS: "false"}
	env := newCfgProviderEnv(cfg, []string{"EMAIL_"})
	if got := env.BoolOr("EMAIL_SMTP_TLS", true); got != false {
		t.Errorf("BoolOr EMAIL_SMTP_TLS = %v, want false", got)
	}
	if got := env.BoolOr("EMAIL_SMTP_UNKNOWN", true); got != true {
		t.Errorf("BoolOr EMAIL_SMTP_UNKNOWN fallback = %v, want true", got)
	}
}

func TestCfgProviderEnv_DurationOr(t *testing.T) {
	cfg := &config{HeartbeatInterval: 45 * time.Second}
	env := newCfgProviderEnv(cfg, []string{"IM_"})
	if got := env.DurationOr("IM_BRIDGE_HEARTBEAT_INTERVAL", 30*time.Second); got != 45*time.Second {
		t.Errorf("DurationOr IM_BRIDGE_HEARTBEAT_INTERVAL = %v, want 45s", got)
	}
	if got := env.DurationOr("IM_UNKNOWN", 10*time.Second); got != 10*time.Second {
		t.Errorf("DurationOr IM_UNKNOWN fallback = %v, want 10s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-im-bridge && go test ./cmd/bridge/ -run TestCfgProviderEnv -v
```

Expected: compile error — `newCfgProviderEnv` undefined.

- [ ] **Step 3: Implement `provider_env.go`**

Create `src-im-bridge/cmd/bridge/provider_env.go`:

```go
package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agentforge/im-bridge/core"
)

// cfgProviderEnv is a ProviderEnv backed by the parsed config struct.
// Env keys that begin with one of prefixes are resolved from config
// fields (preferred, already parsed) or fallback to os.Getenv. Keys
// that do not match any prefix return the zero value.
type cfgProviderEnv struct {
	cfg      *config
	prefixes []string
}

func newCfgProviderEnv(cfg *config, prefixes []string) *cfgProviderEnv {
	return &cfgProviderEnv{cfg: cfg, prefixes: prefixes}
}

var _ core.ProviderEnv = (*cfgProviderEnv)(nil)

func (e *cfgProviderEnv) inNamespace(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	for _, p := range e.prefixes {
		if strings.HasPrefix(upper, strings.ToUpper(p)) {
			return true
		}
	}
	return false
}

func (e *cfgProviderEnv) Get(key string) string {
	if !e.inNamespace(key) {
		return ""
	}
	if v := lookupCfgField(e.cfg, key); v != "" {
		return v
	}
	return os.Getenv(key)
}

func (e *cfgProviderEnv) BoolOr(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(e.Get(key)))
	switch raw {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func (e *cfgProviderEnv) DurationOr(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(e.Get(key))
	if raw == "" {
		return fallback
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return time.Duration(n) * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	return fallback
}

func (e *cfgProviderEnv) TestPort() string { return e.cfg.TestPort }

// lookupCfgField maps env-var names to already-parsed config fields. This
// avoids re-reading os.Getenv for values loadConfig has already parsed,
// and crucially keeps the field-to-env mapping in one place for review.
func lookupCfgField(cfg *config, key string) string {
	upper := strings.ToUpper(strings.TrimSpace(key))
	switch upper {
	// --- Feishu ---
	case "FEISHU_APP_ID":
		return cfg.FeishuApp
	case "FEISHU_APP_SECRET":
		return cfg.FeishuSec
	case "FEISHU_VERIFICATION_TOKEN":
		return cfg.FeishuVerificationToken
	case "FEISHU_EVENT_ENCRYPT_KEY":
		return cfg.FeishuEventEncryptKey
	case "FEISHU_CALLBACK_PATH":
		return cfg.FeishuCallbackPath
	// --- Slack ---
	case "SLACK_BOT_TOKEN":
		return cfg.SlackBotToken
	case "SLACK_APP_TOKEN":
		return cfg.SlackAppToken
	// --- DingTalk ---
	case "DINGTALK_APP_KEY":
		return cfg.DingTalkAppKey
	case "DINGTALK_APP_SECRET":
		return cfg.DingTalkAppSecret
	case "DINGTALK_CARD_TEMPLATE_ID":
		return cfg.DingTalkCardTemplateID
	// --- WeCom ---
	case "WECOM_CORP_ID":
		return cfg.WeComCorpID
	case "WECOM_AGENT_ID":
		return cfg.WeComAgentID
	case "WECOM_AGENT_SECRET":
		return cfg.WeComAgentSecret
	case "WECOM_CALLBACK_TOKEN":
		return cfg.WeComCallbackToken
	case "WECOM_CALLBACK_PORT":
		return cfg.WeComCallbackPort
	case "WECOM_CALLBACK_PATH":
		return cfg.WeComCallbackPath
	// --- WeChat ---
	case "WECHAT_APP_ID":
		return cfg.WeChatAppID
	case "WECHAT_APP_SECRET":
		return cfg.WeChatAppSecret
	case "WECHAT_CALLBACK_TOKEN":
		return cfg.WeChatCallbackToken
	case "WECHAT_CALLBACK_PORT":
		return cfg.WeChatCallbackPort
	case "WECHAT_CALLBACK_PATH":
		return cfg.WeChatCallbackPath
	// --- QQ (OneBot) ---
	case "QQ_ONEBOT_WS_URL":
		return cfg.QQOneBotWSURL
	case "QQ_ACCESS_TOKEN":
		return cfg.QQAccessToken
	// --- QQ Bot ---
	case "QQBOT_APP_ID":
		return cfg.QQBotAppID
	case "QQBOT_APP_SECRET":
		return cfg.QQBotAppSecret
	case "QQBOT_CALLBACK_PORT":
		return cfg.QQBotCallbackPort
	case "QQBOT_CALLBACK_PATH":
		return cfg.QQBotCallbackPath
	case "QQBOT_API_BASE":
		return cfg.QQBotAPIBase
	case "QQBOT_TOKEN_BASE":
		return cfg.QQBotTokenBase
	// --- Telegram ---
	case "TELEGRAM_BOT_TOKEN":
		return cfg.TelegramBotToken
	case "TELEGRAM_UPDATE_MODE":
		return cfg.TelegramUpdateMode
	case "TELEGRAM_WEBHOOK_URL":
		return cfg.TelegramWebhookURL
	// --- Discord ---
	case "DISCORD_APP_ID":
		return cfg.DiscordAppID
	case "DISCORD_BOT_TOKEN":
		return cfg.DiscordBotToken
	case "DISCORD_PUBLIC_KEY":
		return cfg.DiscordPublicKey
	case "DISCORD_INTERACTIONS_PORT":
		return cfg.DiscordInteractionsPort
	case "DISCORD_COMMAND_GUILD_ID":
		return cfg.DiscordCommandGuildID
	// --- Email ---
	case "EMAIL_SMTP_HOST":
		return cfg.EmailSMTPHost
	case "EMAIL_SMTP_PORT":
		return cfg.EmailSMTPPort
	case "EMAIL_SMTP_USER":
		return cfg.EmailSMTPUser
	case "EMAIL_SMTP_PASS":
		return cfg.EmailSMTPPass
	case "EMAIL_FROM_ADDRESS":
		return cfg.EmailFromAddress
	case "EMAIL_SMTP_TLS":
		return cfg.EmailSMTPTLS
	case "EMAIL_IMAP_HOST":
		return cfg.EmailIMAPHost
	case "EMAIL_IMAP_PORT":
		return cfg.EmailIMAPPort
	case "EMAIL_IMAP_USER":
		return cfg.EmailIMAPUser
	case "EMAIL_IMAP_PASS":
		return cfg.EmailIMAPPass
	// --- Shared tunables (reachable via IM_ prefix) ---
	case "IM_BRIDGE_HEARTBEAT_INTERVAL":
		return cfg.HeartbeatInterval.String()
	}
	return ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd src-im-bridge && go test ./cmd/bridge/ -run TestCfgProviderEnv -v
```

Expected: all 4 sub-tests PASS.

- [ ] **Step 5: Commit**

```bash
git add src-im-bridge/cmd/bridge/provider_env.go src-im-bridge/cmd/bridge/provider_env_test.go
git commit -m "feat(im-bridge): add cfgProviderEnv backing for core.ProviderEnv"
```

---

## Task A3: Register all 10 providers via init()

Each provider package gains one `register.go` that declares its `ProviderFactory` via `init()`. Behavior must be byte-identical to the current `providerDescriptors()` switch in `platform_registry.go`.

**Files:**
- Create: `src-im-bridge/platform/feishu/register.go`
- Create: `src-im-bridge/platform/slack/register.go`
- Create: `src-im-bridge/platform/dingtalk/register.go`
- Create: `src-im-bridge/platform/discord/register.go`
- Create: `src-im-bridge/platform/telegram/register.go`
- Create: `src-im-bridge/platform/wechat/register.go`
- Create: `src-im-bridge/platform/wecom/register.go`
- Create: `src-im-bridge/platform/qq/register.go`
- Create: `src-im-bridge/platform/qqbot/register.go`
- Create: `src-im-bridge/platform/email/register.go`

- [ ] **Step 1: Create `platform/feishu/register.go`**

```go
package feishu

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "feishu",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"FEISHU_"},
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

- [ ] **Step 2: Create `platform/slack/register.go`**

```go
package slack

import (
	"fmt"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "slack",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"SLACK_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode == core.TransportModeLive {
				if env.Get("SLACK_BOT_TOKEN") == "" || env.Get("SLACK_APP_TOKEN") == "" {
					return fmt.Errorf("selected platform slack requires SLACK_BOT_TOKEN and SLACK_APP_TOKEN for live transport")
				}
			}
			return nil
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			return NewLive(env.Get("SLACK_BOT_TOKEN"), env.Get("SLACK_APP_TOKEN"))
		},
	})
}
```

If the `slack` package does not expose `liveMetadata` as a package-level identifier, locate the existing metadata constant (likely declared inside `live.go`) and reference it directly; if it is an unexported local variable, export it for the registration file. For this plan, assume each platform package has or can expose a package-level `liveMetadata` — if the symbol name differs, use the existing one verbatim.

- [ ] **Step 3: Create `platform/dingtalk/register.go`**

```go
package dingtalk

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "dingtalk",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"DINGTALK_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode == core.TransportModeLive {
				if env.Get("DINGTALK_APP_KEY") == "" || env.Get("DINGTALK_APP_SECRET") == "" {
					return fmt.Errorf("selected platform dingtalk requires DINGTALK_APP_KEY and DINGTALK_APP_SECRET for live transport")
				}
			}
			return nil
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			opts := make([]LiveOption, 0, 1)
			if tid := strings.TrimSpace(env.Get("DINGTALK_CARD_TEMPLATE_ID")); tid != "" {
				opts = append(opts, WithAdvancedCardTemplate(tid))
			}
			return NewLive(env.Get("DINGTALK_APP_KEY"), env.Get("DINGTALK_APP_SECRET"), opts...)
		},
	})
}
```

- [ ] **Step 4: Create `platform/discord/register.go`**

```go
package discord

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "discord",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"DISCORD_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			if env.Get("DISCORD_APP_ID") == "" || env.Get("DISCORD_BOT_TOKEN") == "" || env.Get("DISCORD_PUBLIC_KEY") == "" {
				return fmt.Errorf("selected platform discord requires DISCORD_APP_ID, DISCORD_BOT_TOKEN, and DISCORD_PUBLIC_KEY for live transport")
			}
			if env.Get("DISCORD_INTERACTIONS_PORT") == "" {
				return fmt.Errorf("selected platform discord requires DISCORD_INTERACTIONS_PORT for live transport")
			}
			return nil
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			opts := make([]LiveOption, 0, 1)
			if gid := strings.TrimSpace(env.Get("DISCORD_COMMAND_GUILD_ID")); gid != "" {
				opts = append(opts, WithCommandGuildID(gid))
			}
			return NewLive(
				env.Get("DISCORD_APP_ID"),
				env.Get("DISCORD_BOT_TOKEN"),
				env.Get("DISCORD_PUBLIC_KEY"),
				env.Get("DISCORD_INTERACTIONS_PORT"),
				opts...,
			)
		},
	})
}
```

- [ ] **Step 5: Create `platform/telegram/register.go`**

```go
package telegram

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "telegram",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"TELEGRAM_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			if env.Get("TELEGRAM_BOT_TOKEN") == "" {
				return fmt.Errorf("selected platform telegram requires TELEGRAM_BOT_TOKEN for live transport")
			}
			return telegramValidateUpdateMode(env.Get("TELEGRAM_UPDATE_MODE"), env.Get("TELEGRAM_WEBHOOK_URL"))
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			return NewLive(env.Get("TELEGRAM_BOT_TOKEN"))
		},
	})
}

// telegramValidateUpdateMode is lifted verbatim from the telegramValidateConfig
// helper in cmd/bridge/platform_registry.go.
func telegramValidateUpdateMode(updateMode, webhookURL string) error {
	normalized := strings.ToLower(strings.TrimSpace(updateMode))
	if normalized == "" {
		normalized = "longpoll"
	}
	if normalized != "longpoll" {
		return fmt.Errorf("telegram live transport currently supports only longpoll update mode")
	}
	if strings.TrimSpace(webhookURL) != "" {
		return fmt.Errorf("telegram long polling cannot be combined with webhook configuration")
	}
	return nil
}
```

- [ ] **Step 6: Create `platform/wechat/register.go`**

```go
package wechat

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "wechat",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"WECHAT_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			if env.Get("WECHAT_APP_ID") == "" || env.Get("WECHAT_APP_SECRET") == "" {
				return fmt.Errorf("selected platform wechat requires WECHAT_APP_ID and WECHAT_APP_SECRET for live transport")
			}
			return nil
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			opts := make([]LiveOption, 0, 2)
			if p := strings.TrimSpace(env.Get("WECHAT_CALLBACK_PORT")); p != "" {
				opts = append(opts, WithCallbackPort(p))
			}
			if path := strings.TrimSpace(env.Get("WECHAT_CALLBACK_PATH")); path != "" {
				opts = append(opts, WithCallbackPath(path))
			}
			return NewLive(env.Get("WECHAT_APP_ID"), env.Get("WECHAT_APP_SECRET"), env.Get("WECHAT_CALLBACK_TOKEN"), opts...)
		},
	})
}
```

- [ ] **Step 7: Create `platform/wecom/register.go`**

```go
package wecom

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "wecom",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"WECOM_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			switch {
			case strings.TrimSpace(env.Get("WECOM_CORP_ID")) == "":
				return fmt.Errorf("selected platform wecom requires WECOM_CORP_ID for live transport")
			case strings.TrimSpace(env.Get("WECOM_AGENT_ID")) == "":
				return fmt.Errorf("selected platform wecom requires WECOM_AGENT_ID for live transport")
			case strings.TrimSpace(env.Get("WECOM_AGENT_SECRET")) == "":
				return fmt.Errorf("selected platform wecom requires WECOM_AGENT_SECRET for live transport")
			case strings.TrimSpace(env.Get("WECOM_CALLBACK_TOKEN")) == "":
				return fmt.Errorf("selected platform wecom requires WECOM_CALLBACK_TOKEN for live transport")
			case strings.TrimSpace(env.Get("WECOM_CALLBACK_PORT")) == "":
				return fmt.Errorf("selected platform wecom requires WECOM_CALLBACK_PORT for live transport")
			default:
				return nil
			}
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			return NewLive(
				env.Get("WECOM_CORP_ID"),
				env.Get("WECOM_AGENT_ID"),
				env.Get("WECOM_AGENT_SECRET"),
				env.Get("WECOM_CALLBACK_TOKEN"),
				env.Get("WECOM_CALLBACK_PORT"),
				env.Get("WECOM_CALLBACK_PATH"),
			)
		},
	})
}
```

- [ ] **Step 8: Create `platform/qq/register.go`**

```go
package qq

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "qq",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"QQ_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			if strings.TrimSpace(env.Get("QQ_ONEBOT_WS_URL")) == "" {
				return fmt.Errorf("selected platform qq requires QQ_ONEBOT_WS_URL for live transport")
			}
			return nil
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			return NewLive(env.Get("QQ_ONEBOT_WS_URL"), env.Get("QQ_ACCESS_TOKEN"))
		},
	})
}
```

- [ ] **Step 9: Create `platform/qqbot/register.go`**

```go
package qqbot

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "qqbot",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"QQBOT_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			switch {
			case strings.TrimSpace(env.Get("QQBOT_APP_ID")) == "":
				return fmt.Errorf("selected platform qqbot requires QQBOT_APP_ID for live transport")
			case strings.TrimSpace(env.Get("QQBOT_APP_SECRET")) == "":
				return fmt.Errorf("selected platform qqbot requires QQBOT_APP_SECRET for live transport")
			case strings.TrimSpace(env.Get("QQBOT_CALLBACK_PORT")) == "":
				return fmt.Errorf("selected platform qqbot requires QQBOT_CALLBACK_PORT for live transport")
			default:
				return nil
			}
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			return NewLive(
				env.Get("QQBOT_APP_ID"),
				env.Get("QQBOT_APP_SECRET"),
				env.Get("QQBOT_CALLBACK_PORT"),
				env.Get("QQBOT_CALLBACK_PATH"),
				WithAPIBase(env.Get("QQBOT_API_BASE")),
				WithTokenBase(env.Get("QQBOT_TOKEN_BASE")),
			)
		},
	})
}
```

- [ ] **Step 10: Create `platform/email/register.go`**

```go
package email

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "email",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"EMAIL_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			switch {
			case strings.TrimSpace(env.Get("EMAIL_SMTP_HOST")) == "":
				return fmt.Errorf("selected platform email requires EMAIL_SMTP_HOST for live transport")
			case strings.TrimSpace(env.Get("EMAIL_FROM_ADDRESS")) == "":
				return fmt.Errorf("selected platform email requires EMAIL_FROM_ADDRESS for live transport")
			default:
				return nil
			}
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			opts := make([]LiveOption, 0, 1)
			if strings.EqualFold(strings.TrimSpace(env.Get("EMAIL_SMTP_TLS")), "false") {
				opts = append(opts, WithTLS(false))
			}
			return NewLive(
				env.Get("EMAIL_SMTP_HOST"),
				env.Get("EMAIL_SMTP_PORT"),
				env.Get("EMAIL_SMTP_USER"),
				env.Get("EMAIL_SMTP_PASS"),
				env.Get("EMAIL_FROM_ADDRESS"),
				opts...,
			)
		},
	})
}
```

- [ ] **Step 11: Expose `liveMetadata` in each platform package**

For each platform package (feishu/slack/dingtalk/discord/telegram/wechat/wecom/qq/qqbot/email), grep its `live.go` and `stub.go` for the current metadata constant name:

```bash
grep -n "liveMetadata\|platformMetadata\|Metadata:" src-im-bridge/platform/*/live.go src-im-bridge/platform/*/stub.go
```

If a package uses a different name (e.g. `metadata` or `stubMetadata`) or the value is inline, lift it into a package-level `var liveMetadata = core.PlatformMetadata{...}` declaration near the top of `live.go`. Do not change field values; just change scope.

- [ ] **Step 12: Verify each register.go compiles**

```bash
cd src-im-bridge && go build ./platform/...
```

Expected: clean build. Fix any reference errors (missing exported symbol, wrong helper name) per platform package's actual API before continuing.

- [ ] **Step 13: Commit**

```bash
git add src-im-bridge/platform/*/register.go src-im-bridge/platform/*/live.go
git commit -m "feat(im-bridge): self-register each provider via core.ProviderFactory"
```

---

## Task A4: Contract platform_registry.go + blank imports + CI guard

Replace `providerDescriptors()` with a thin adapter over `core.LookupProvider`. Add blank imports in `main.go` so the provider `init()` side effects run. Add a test that catches missing blank imports.

**Files:**
- Modify: `src-im-bridge/cmd/bridge/platform_registry.go`
- Modify: `src-im-bridge/cmd/bridge/provider_contract.go`
- Modify: `src-im-bridge/cmd/bridge/main.go`
- Create: `src-im-bridge/cmd/bridge/all_providers_registered_test.go`

- [ ] **Step 1: Write the failing guard test**

Create `src-im-bridge/cmd/bridge/all_providers_registered_test.go`:

```go
package main

import (
	"sort"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

// TestAllBuiltinProvidersRegistered verifies every in-tree provider
// package is blank-imported from main.go and its init() has run. If a new
// provider package is added without an accompanying `import _ "..."` in
// main.go, this test fails loudly in CI with the missing id.
func TestAllBuiltinProvidersRegistered(t *testing.T) {
	expected := []string{
		"dingtalk",
		"discord",
		"email",
		"feishu",
		"qq",
		"qqbot",
		"slack",
		"telegram",
		"wechat",
		"wecom",
	}
	sort.Strings(expected)

	got := map[string]bool{}
	for _, f := range core.RegisteredProviders() {
		got[f.ID] = true
	}

	var missing []string
	for _, id := range expected {
		if !got[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		t.Errorf("providers not registered (likely missing blank import in main.go): %v", missing)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-im-bridge && go test ./cmd/bridge/ -run TestAllBuiltinProvidersRegistered -v
```

Expected: test **fails** with all 10 providers listed as missing (blank imports not added yet).

- [ ] **Step 3: Add blank imports to `cmd/bridge/main.go`**

At the top of `src-im-bridge/cmd/bridge/main.go`, append to the existing import block (order alphabetically in a dedicated blank-import group at the end):

```go
import (
	// ... existing imports ...

	// Register all built-in IM providers. Each import's init() calls
	// core.RegisterProvider with that provider's factory.
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

Remove the now-redundant non-blank imports of these packages that were used only for `platform_registry.go`'s switch — but keep any import still used by other `cmd/bridge/*.go` files. Run `go build ./cmd/bridge/...` after editing to surface unused-import errors the compiler will flag.

- [ ] **Step 4: Contract `platform_registry.go`**

Replace the entire contents of `src-im-bridge/cmd/bridge/platform_registry.go` with:

```go
package main

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

const (
	transportModeStub = core.TransportModeStub
	transportModeLive = core.TransportModeLive
)

type platformFactory func(env core.ProviderEnv) (core.Platform, error)

func normalizeTransportMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return transportModeStub
	}
	return normalized
}

// platformDescriptors is retained as a name for tests that enumerate
// every built-in provider descriptor. It now sources from the global
// registry instead of a hand-maintained switch.
func platformDescriptors() map[string]platformDescriptor {
	return providerDescriptorsFromRegistry()
}

func providerDescriptors() map[string]providerDescriptor {
	return providerDescriptorsFromRegistry()
}

func providerDescriptorsFromRegistry() map[string]providerDescriptor {
	out := map[string]providerDescriptor{}
	for _, f := range core.RegisteredProviders() {
		out[f.ID] = adaptFactoryToDescriptor(f)
	}
	return out
}

func adaptFactoryToDescriptor(f core.ProviderFactory) providerDescriptor {
	cast := func(newFn func(env core.ProviderEnv) (core.Platform, error)) platformFactory {
		return newFn
	}
	return providerDescriptor{
		ID:                      f.ID,
		Metadata:                f.Metadata,
		SupportedTransportModes: append([]string(nil), f.SupportedTransportModes...),
		Features:                adaptFeatures(f),
		ValidateConfig: func(cfg *config, mode string) error {
			if f.ValidateConfig == nil {
				return nil
			}
			return f.ValidateConfig(newCfgProviderEnv(cfg, f.EnvPrefixes), mode)
		},
		NewStub: func(cfg *config) (core.Platform, error) {
			if f.NewStub == nil {
				return nil, fmt.Errorf("provider %s has no stub factory", f.ID)
			}
			return f.NewStub(newCfgProviderEnv(cfg, f.EnvPrefixes))
		},
		NewLive: func(cfg *config) (core.Platform, error) {
			if f.NewLive == nil {
				return nil, fmt.Errorf("provider %s has no live factory", f.ID)
			}
			return f.NewLive(newCfgProviderEnv(cfg, f.EnvPrefixes))
		},
		envPrefixes: append([]string(nil), f.EnvPrefixes...),
	}
}

func adaptFeatures(f core.ProviderFactory) providerFeatureSet {
	if fs, ok := f.Features.(providerFeatureSet); ok {
		return fs
	}
	return providerFeatureSet{}
}
```

- [ ] **Step 5: Update `provider_contract.go` to match the new signatures**

Modify `src-im-bridge/cmd/bridge/provider_contract.go`:

1. Replace the existing `providerDescriptor` struct field definitions to use the factory-style signatures:

```go
type providerDescriptor struct {
	ID                      string
	Metadata                core.PlatformMetadata
	SupportedTransportModes []string
	Features                providerFeatureSet
	PlannedReason           string
	ValidateConfig          func(cfg *config, mode string) error
	NewStub                 func(cfg *config) (core.Platform, error)
	NewLive                 func(cfg *config) (core.Platform, error)
	// envPrefixes is carried here so legacy callers that need the factory's
	// namespace (e.g. for error messages) can reach it without re-consulting
	// core.RegisteredProviders.
	envPrefixes []string
}
```

2. Leave `activeProvider`, `lookupProviderDescriptor`, `selectProvider`, `selectProviderForPlatform`, `selectProviders`, `providerTransportOverride` unchanged — they call through the new adapter transparently.

3. The existing `platformDescriptor = providerDescriptor` type alias stays.

- [ ] **Step 6: Run all Bridge tests**

```bash
cd src-im-bridge && go test ./...
```

Expected: all tests PASS, including the new `TestAllBuiltinProvidersRegistered` (10 providers registered), plus the pre-existing `multi_provider_test.go`, `platform_registry_test.go`, `provider_contract_test.go` suites.

If any pre-existing test breaks because a provider package's exported helper (e.g. `WithAdvancedCardTemplate`) changed signature, either restore the helper or update the test — do not change the factory behavior.

- [ ] **Step 7: Commit**

```bash
git add src-im-bridge/cmd/bridge/platform_registry.go src-im-bridge/cmd/bridge/provider_contract.go src-im-bridge/cmd/bridge/main.go src-im-bridge/cmd/bridge/all_providers_registered_test.go
git commit -m "feat(im-bridge): route provider selection through core.ProviderFactory registry"
```

---

# Phase B — Rename feishu-adapter → sample-integration-plugin

## Task B1: Rename directory + rewrite manifest

**Files:**
- Rename: `plugins/integrations/feishu-adapter/` → `plugins/integrations/sample-integration-plugin/`
- Rename: `plugins/integrations/sample-integration-plugin/dist/feishu.wasm` → `sample-integration.wasm`
- Modify: `plugins/integrations/sample-integration-plugin/manifest.yaml`

- [ ] **Step 1: Rename the directory and WASM artifact**

```bash
git mv plugins/integrations/feishu-adapter plugins/integrations/sample-integration-plugin
git mv plugins/integrations/sample-integration-plugin/dist/feishu.wasm plugins/integrations/sample-integration-plugin/dist/sample-integration.wasm
```

- [ ] **Step 2: Rewrite the manifest**

Overwrite `plugins/integrations/sample-integration-plugin/manifest.yaml` with:

```yaml
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: sample-integration-plugin
  name: Sample Integration Plugin
  version: 0.1.0
  description: Reference Go WASM integration plugin demonstrating the plugin ABI, health check, and echo operation. Use as a starting template for new Integration Plugins.
  tags:
    - builtin
    - sample
    - demo
    - wasm
spec:
  runtime: wasm
  module: ./dist/sample-integration.wasm
  abiVersion: v1
  capabilities:
    - health
    - echo
permissions:
  network:
    required: false
source:
  type: builtin
  path: ./plugins/integrations/sample-integration-plugin/manifest.yaml
```

- [ ] **Step 3: Commit**

```bash
git add plugins/integrations/sample-integration-plugin/
git commit -m "refactor(plugin): rename feishu-adapter demo to sample-integration-plugin"
```

---

## Task B2: Update builtin-bundle + dev-targets script

**Files:**
- Modify: `plugins/builtin-bundle.yaml`
- Modify: `scripts/plugin/plugin-dev-targets.js`

- [ ] **Step 1: Update `plugins/builtin-bundle.yaml`**

Replace the `feishu-adapter` entry (lines 186–214) with:

```json
{
  "id": "sample-integration-plugin",
  "kind": "IntegrationPlugin",
  "manifest": "integrations/sample-integration-plugin/manifest.yaml",
  "docsRef": "docs/guides/plugin-wasm.md",
  "verificationProfile": "go-wasm",
  "readiness": {
    "readyMessage": "Sample Integration Plugin is ready for install.",
    "blockedMessage": "Requires the bridge host to support the Go WASM runtime.",
    "nextStep": "Install the built-in and confirm the WASM runtime is ready.",
    "installable": true
  },
  "availability": {
    "status": "ready",
    "message": "Bundled reference WASM integration plugin."
  }
},
```

Preserve surrounding JSON punctuation — the file is JSON despite the `.yaml` extension.

- [ ] **Step 2: Update `scripts/plugin/plugin-dev-targets.js`**

Change the file to:

```js
/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const path = require("node:path");

const DEFAULT_GO_WASM_MANIFEST_PATH = path.join(
  "plugins",
  "integrations",
  "sample-integration-plugin",
  "manifest.yaml",
);

const MAINTAINED_GO_WASM_TARGETS = {
  "sample-integration-plugin": {
    sourcePath: "./cmd/sample-wasm-plugin",
  },
  "standard-dev-flow": {
    sourcePath: "./cmd/standard-dev-flow",
  },
  "task-delivery-flow": {
    sourcePath: "./cmd/task-delivery-flow",
  },
  "review-escalation-flow": {
    sourcePath: "./cmd/review-escalation-flow",
  },
};

// ... rest of the file unchanged ...
```

The helpers `getRepoRoot`, `normalizePath`, `extractScalar`, `parseManifestFields`, `resolveMaintainedGoWASMSourcePath`, `resolveBuildTarget`, and the `module.exports` block are unchanged.

- [ ] **Step 3: Commit**

```bash
git add plugins/builtin-bundle.yaml scripts/plugin/plugin-dev-targets.js
git commit -m "refactor(plugin): point builtin bundle and dev targets at sample-integration-plugin"
```

---

## Task B3: Update four TS test fixtures

**Files:**
- Modify: `scripts/plugin/build-go-wasm-plugin.test.ts`
- Modify: `scripts/plugin/debug-go-wasm-plugin.test.ts`
- Modify: `scripts/plugin/verify-plugin-dev-workflow.test.ts`
- Modify: `scripts/plugin/verify-built-in-plugin-bundle.test.ts`

- [ ] **Step 1: Replace `feishu-adapter` string references**

Within each of the four files, substitute the following literal strings:

| Old | New |
|-----|-----|
| `"feishu-adapter"` | `"sample-integration-plugin"` |
| `"feishu.wasm"` | `"sample-integration.wasm"` |
| `plugins/integrations/feishu-adapter/` | `plugins/integrations/sample-integration-plugin/` |
| `plugins\\integrations\\feishu-adapter\\` (Windows path literals in debug-go-wasm-plugin.test.ts) | `plugins\\integrations\\sample-integration-plugin\\` |

Use targeted replacements rather than global regex, and confirm each test still makes sense semantically after rename. If a test asserts `summary["feishu-adapter"]` it becomes `summary["sample-integration-plugin"]`; the expected `["build", "debug-health"]` list stays unchanged.

- [ ] **Step 2: Run the affected test suites**

```bash
pnpm exec vitest run scripts/plugin/build-go-wasm-plugin.test.ts
pnpm exec vitest run scripts/plugin/debug-go-wasm-plugin.test.ts
pnpm exec vitest run scripts/plugin/verify-plugin-dev-workflow.test.ts
pnpm exec vitest run scripts/plugin/verify-built-in-plugin-bundle.test.ts
```

Expected: all four suites pass.

- [ ] **Step 3: Commit**

```bash
git add scripts/plugin/*.test.ts
git commit -m "test(plugin): update fixture literals for sample-integration-plugin rename"
```

---

## Task B4: Update narrative docs

**Files:**
- Modify: `plugins/README.md`
- Modify: `docs/architecture/wasm-plugin-runtime.md`
- Modify: `docs/guides/plugin-wasm.md`
- Modify: `docs/guides/plugin-development.md`
- Modify: `README.md`
- Modify: `README_zh.md`
- Modify: `docs/product/prd.md`

- [ ] **Step 1: Find every narrative mention of the old id**

```bash
grep -rn "feishu-adapter\|Feishu Adapter\|Feishu adapter" plugins/README.md docs/architecture/wasm-plugin-runtime.md docs/guides/plugin-wasm.md docs/guides/plugin-development.md README.md README_zh.md docs/product/prd.md
```

- [ ] **Step 2: Rewrite each match to reflect the split of responsibilities**

For every match returned in Step 1, replace the surrounding prose with wording along these lines:

- "We ship a Feishu adapter integration plugin" → "We ship `sample-integration-plugin` as a reference WASM integration plugin. IM platform transport (Feishu, Slack, DingTalk, …) is owned by the IM Bridge, not by Integration Plugins."
- Bullets that list `feishu-adapter` as an example become `sample-integration-plugin`.
- If a doc uses the plugin as a walkthrough example with concrete `FEISHU_*` env vars, replace the env vars with the generic sample manifest (no env vars needed).

**Do not** edit archived specs under `openspec/changes/archive/`.

- [ ] **Step 3: Verify no residual references outside archives**

```bash
grep -rn "feishu-adapter" docs/ plugins/ README.md README_zh.md | grep -v "openspec/changes/archive"
```

Expected: empty output.

- [ ] **Step 4: Commit**

```bash
git add plugins/README.md docs/architecture/wasm-plugin-runtime.md docs/guides/plugin-wasm.md docs/guides/plugin-development.md README.md README_zh.md docs/product/prd.md
git commit -m "docs: rewrite feishu-adapter mentions as sample-integration-plugin"
```

---

# Phase C — Bridge registration inventory enrichment

## Task C1: Extend `IMBridgeRegisterRequest` / `IMBridgeInstance` (server-side model)

**Files:**
- Modify: `src-go/internal/model/im.go`

- [ ] **Step 1: Add new types and extend the register request**

In `src-go/internal/model/im.go`, locate the existing `IMBridgeRegisterRequest` / `IMBridgeInstance` definitions (around line 418 / 443) and add the following types immediately after `IMTenantBinding`:

```go
// IMBridgeProvider describes one IM provider active on a Bridge process.
// When a Bridge hosts a single provider, Providers[0] content mirrors the
// top-level Platform/Transport/CapabilityMatrix/CallbackPaths/Tenants fields.
type IMBridgeProvider struct {
	ID               string            `json:"id"`
	Transport        string            `json:"transport"`
	ReadinessTier    string            `json:"readinessTier,omitempty"`
	CapabilityMatrix map[string]any    `json:"capabilityMatrix,omitempty"`
	CallbackPaths    []string          `json:"callbackPaths,omitempty"`
	Tenants          []string          `json:"tenants,omitempty"`
	// MetadataSource is "builtin" today. Reserved values: "oop-grpc".
	MetadataSource   string            `json:"metadataSource"`
}

// IMBridgeCommandPlugin mirrors a core/plugin manifest loaded by a Bridge.
// Read-only; the Go orchestrator cannot enable/disable command plugins via
// this channel.
type IMBridgeCommandPlugin struct {
	ID         string   `json:"id"`
	Version    string   `json:"version"`
	Commands   []string `json:"commands"`
	Tenants    []string `json:"tenants,omitempty"`
	SourcePath string   `json:"sourcePath,omitempty"`
}
```

Extend `IMBridgeRegisterRequest`:

```go
type IMBridgeRegisterRequest struct {
	BridgeID         string            `json:"bridgeId"`
	Platform         string            `json:"platform"`
	Transport        string            `json:"transport"`
	ProjectIDs       []string          `json:"projectIds,omitempty"`
	Capabilities     map[string]bool   `json:"capabilities,omitempty"`
	CapabilityMatrix map[string]any    `json:"capabilityMatrix,omitempty"`
	CallbackPaths    []string          `json:"callbackPaths,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Tenants          []string          `json:"tenants,omitempty"`
	TenantManifest   []IMTenantBinding `json:"tenantManifest,omitempty"`

	// New fields — optional, back-compat preserved.
	Providers      []IMBridgeProvider      `json:"providers,omitempty"`
	CommandPlugins []IMBridgeCommandPlugin `json:"commandPlugins,omitempty"`
}
```

Extend `IMBridgeInstance`:

```go
type IMBridgeInstance struct {
	BridgeID         string            `json:"bridgeId"`
	Platform         string            `json:"platform"`
	Transport        string            `json:"transport"`
	ProjectIDs       []string          `json:"projectIds,omitempty"`
	Capabilities     map[string]bool   `json:"capabilities,omitempty"`
	CapabilityMatrix map[string]any    `json:"capabilityMatrix,omitempty"`
	CallbackPaths    []string          `json:"callbackPaths,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Tenants          []string          `json:"tenants,omitempty"`
	TenantManifest   []IMTenantBinding `json:"tenantManifest,omitempty"`
	LastSeenAt       string            `json:"lastSeenAt,omitempty"`
	ExpiresAt        string            `json:"expiresAt,omitempty"`
	Status           string            `json:"status,omitempty"`

	// New fields
	Providers      []IMBridgeProvider      `json:"providers,omitempty"`
	CommandPlugins []IMBridgeCommandPlugin `json:"commandPlugins,omitempty"`
}
```

- [ ] **Step 2: Verify the model package still builds**

```bash
cd src-go && go build ./internal/model/...
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add src-go/internal/model/im.go
git commit -m "feat(im): add Providers/CommandPlugins fields to IM bridge registration model"
```

---

## Task C2: Mirror the new fields on the Bridge-side client types

**Files:**
- Modify: `src-im-bridge/client/agentforge.go`

- [ ] **Step 1: Add matching types and extend `BridgeRegistration` + `BridgeInstance`**

In `src-im-bridge/client/agentforge.go`, near `BridgeRegistration` (line 1871), add:

```go
// BridgeProvider describes one IM provider active on this Bridge process.
// Serialization is identical to the backend model.IMBridgeProvider.
type BridgeProvider struct {
	ID               string         `json:"id"`
	Transport        string         `json:"transport"`
	ReadinessTier    string         `json:"readinessTier,omitempty"`
	CapabilityMatrix map[string]any `json:"capabilityMatrix,omitempty"`
	CallbackPaths    []string       `json:"callbackPaths,omitempty"`
	Tenants          []string       `json:"tenants,omitempty"`
	MetadataSource   string         `json:"metadataSource"`
}

// BridgeCommandPlugin mirrors a Bridge-side core/plugin manifest.
type BridgeCommandPlugin struct {
	ID         string   `json:"id"`
	Version    string   `json:"version"`
	Commands   []string `json:"commands"`
	Tenants    []string `json:"tenants,omitempty"`
	SourcePath string   `json:"sourcePath,omitempty"`
}
```

Extend `BridgeRegistration`:

```go
type BridgeRegistration struct {
	BridgeID         string            `json:"bridgeId"`
	Platform         string            `json:"platform"`
	Transport        string            `json:"transport"`
	ProjectIDs       []string          `json:"projectIds,omitempty"`
	Capabilities     map[string]bool   `json:"capabilities,omitempty"`
	CapabilityMatrix map[string]any    `json:"capabilityMatrix,omitempty"`
	CallbackPaths    []string          `json:"callbackPaths,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Tenants          []string          `json:"tenants,omitempty"`
	TenantManifest   []TenantBinding   `json:"tenantManifest,omitempty"`

	Providers      []BridgeProvider      `json:"providers,omitempty"`
	CommandPlugins []BridgeCommandPlugin `json:"commandPlugins,omitempty"`
}
```

Extend `BridgeInstance` with the same two fields.

- [ ] **Step 2: Verify the bridge client still builds**

```bash
cd src-im-bridge && go build ./client/...
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add src-im-bridge/client/agentforge.go
git commit -m "feat(im-bridge-client): add Providers/CommandPlugins to BridgeRegistration"
```

---

## Task C3: Add `Registry.Snapshot()` and the bridge-side inventory type

**Files:**
- Create: `src-im-bridge/core/plugin/inventory.go`
- Create: `src-im-bridge/core/plugin/inventory_test.go`
- Modify: `src-im-bridge/core/plugin/plugin.go`

- [ ] **Step 1: Write the failing test**

Create `src-im-bridge/core/plugin/inventory_test.go`:

```go
package plugin

import (
	"testing"
	"time"
)

func TestRegistry_Snapshot_Empty(t *testing.T) {
	r := NewRegistry("")
	snap := r.Snapshot()
	if len(snap) != 0 {
		t.Errorf("Snapshot() len = %d, want 0", len(snap))
	}
}

func TestRegistry_Snapshot_StableOrder(t *testing.T) {
	r := NewRegistry("")
	r.plugins = map[string]*Loaded{
		"@beta/cmd": {
			Manifest: Manifest{
				ID:      "@beta/cmd",
				Version: "1.0.0",
				Commands: []CommandEntry{
					{Slash: "/beta"},
				},
				Tenants: []string{"acme"},
			},
			Path:     "/tmp/beta/plugin.yaml",
			LoadedAt: time.Now(),
		},
		"@alpha/cmd": {
			Manifest: Manifest{
				ID:      "@alpha/cmd",
				Version: "0.1.0",
				Commands: []CommandEntry{
					{Slash: "/alpha"},
					{Slash: "/gamma"},
				},
			},
			Path: "/tmp/alpha/plugin.yaml",
		},
	}

	snap := r.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("len = %d, want 2", len(snap))
	}
	if snap[0].ID != "@alpha/cmd" {
		t.Errorf("snap[0].ID = %q, want @alpha/cmd", snap[0].ID)
	}
	if len(snap[0].Commands) != 2 || snap[0].Commands[0] != "/alpha" {
		t.Errorf("snap[0].Commands = %v", snap[0].Commands)
	}
	if snap[1].ID != "@beta/cmd" {
		t.Errorf("snap[1].ID = %q, want @beta/cmd", snap[1].ID)
	}
	if len(snap[1].Tenants) != 1 || snap[1].Tenants[0] != "acme" {
		t.Errorf("snap[1].Tenants = %v", snap[1].Tenants)
	}
}

func TestRegistry_Snapshot_CloneTenants(t *testing.T) {
	r := NewRegistry("")
	r.plugins = map[string]*Loaded{
		"p": {
			Manifest: Manifest{
				ID:       "p",
				Version:  "0.0.1",
				Commands: []CommandEntry{{Slash: "/p"}},
				Tenants:  []string{"t1"},
			},
		},
	}
	snap := r.Snapshot()
	snap[0].Tenants[0] = "mutated"
	if r.plugins["p"].Manifest.Tenants[0] == "mutated" {
		t.Error("Snapshot() leaked tenants slice reference")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-im-bridge && go test ./core/plugin/ -run TestRegistry_Snapshot -v
```

Expected: compile error — `Snapshot` undefined, `IMBridgeCommandPlugin` undefined.

- [ ] **Step 3: Create `src-im-bridge/core/plugin/inventory.go`**

```go
package plugin

// IMBridgeCommandPlugin mirrors the backend model IMBridgeCommandPlugin in
// wire format. The bridge package depends only on core, so we declare the
// struct locally and serialize through json tags that match backend JSON.
type IMBridgeCommandPlugin struct {
	ID         string   `json:"id"`
	Version    string   `json:"version"`
	Commands   []string `json:"commands"`
	Tenants    []string `json:"tenants,omitempty"`
	SourcePath string   `json:"sourcePath,omitempty"`
}
```

- [ ] **Step 4: Add `Registry.Snapshot()` in `plugin.go`**

Near the bottom of `src-im-bridge/core/plugin/plugin.go` (after `Plugins()` returns), add:

```go
// Snapshot returns a stable-ordered inventory of every loaded manifest.
// Intended for control-plane reporting (bridge inventory snapshot). The
// returned slice and its string slices are deep copies — callers may
// mutate freely without affecting the registry.
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

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd src-im-bridge && go test ./core/plugin/ -run TestRegistry_Snapshot -v
```

Expected: 3 sub-tests PASS.

- [ ] **Step 6: Commit**

```bash
git add src-im-bridge/core/plugin/inventory.go src-im-bridge/core/plugin/inventory_test.go src-im-bridge/core/plugin/plugin.go
git commit -m "feat(im-bridge): add Registry.Snapshot for bridge inventory reporting"
```

---

## Task C4: `cmd/bridge/inventory.go` — build the multi-provider registration payload

**Files:**
- Create: `src-im-bridge/cmd/bridge/inventory.go`
- Create: `src-im-bridge/cmd/bridge/inventory_test.go`

- [ ] **Step 1: Write the failing test**

Create `src-im-bridge/cmd/bridge/inventory_test.go`:

```go
package main

import (
	"testing"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/core/plugin"
)

func TestBuildRegistrationInventory_MultiProvider(t *testing.T) {
	providers := []*activeProvider{
		{
			Descriptor: providerDescriptor{
				ID: "feishu",
				Metadata: core.PlatformMetadata{
					Source: "feishu",
					Capabilities: core.PlatformCapabilities{
						ReadinessTier: core.ReadinessTierFullNativeLifecycle,
					},
				},
			},
			TransportMode: core.TransportModeLive,
			Tenants:       []string{"acme"},
		},
		{
			Descriptor: providerDescriptor{
				ID: "slack",
				Metadata: core.PlatformMetadata{
					Source: "slack",
					Capabilities: core.PlatformCapabilities{
						ReadinessTier: "",
					},
				},
			},
			TransportMode: core.TransportModeStub,
			Tenants:       []string{"beta"},
		},
	}

	reg := plugin.NewRegistry("")

	inv := buildRegistrationInventory(providers, reg)

	if len(inv.Providers) != 2 {
		t.Fatalf("Providers len = %d, want 2", len(inv.Providers))
	}
	if inv.Providers[0].ID != "feishu" {
		t.Errorf("Providers[0].ID = %q, want feishu", inv.Providers[0].ID)
	}
	if inv.Providers[0].Transport != "live" {
		t.Errorf("Providers[0].Transport = %q, want live", inv.Providers[0].Transport)
	}
	if inv.Providers[0].ReadinessTier != "full_native_lifecycle" {
		t.Errorf("Providers[0].ReadinessTier = %q", inv.Providers[0].ReadinessTier)
	}
	if inv.Providers[0].Tenants[0] != "acme" {
		t.Errorf("Providers[0].Tenants = %v", inv.Providers[0].Tenants)
	}
	if inv.Providers[0].MetadataSource != "builtin" {
		t.Errorf("Providers[0].MetadataSource = %q, want builtin", inv.Providers[0].MetadataSource)
	}
	if inv.Providers[1].ID != "slack" || inv.Providers[1].Transport != "stub" {
		t.Errorf("Providers[1] = %+v", inv.Providers[1])
	}
	if len(inv.CommandPlugins) != 0 {
		t.Errorf("CommandPlugins len = %d, want 0", len(inv.CommandPlugins))
	}
}

func TestBuildRegistrationInventory_NilPluginRegistry(t *testing.T) {
	providers := []*activeProvider{{
		Descriptor: providerDescriptor{
			ID:       "feishu",
			Metadata: core.PlatformMetadata{Source: "feishu"},
		},
		TransportMode: core.TransportModeStub,
	}}
	inv := buildRegistrationInventory(providers, nil)
	if len(inv.CommandPlugins) != 0 {
		t.Errorf("nil registry should yield 0 command plugins, got %d", len(inv.CommandPlugins))
	}
}

// TestBuildRegistrationInventory_WireShape guards the serialized JSON
// matches the backend model exactly.
func TestBuildRegistrationInventory_WireShape(t *testing.T) {
	providers := []*activeProvider{{
		Descriptor: providerDescriptor{
			ID: "slack",
			Metadata: core.PlatformMetadata{
				Source:       "slack",
				Capabilities: core.PlatformCapabilities{SupportsRichMessages: true},
			},
		},
		TransportMode: "live",
		Tenants:       []string{"acme"},
	}}
	inv := buildRegistrationInventory(providers, nil)
	// Confirm the struct is assignable to the exported client.BridgeRegistration
	// payload field without intermediate transformation.
	var reg client.BridgeRegistration
	reg.Providers = inv.Providers
	reg.CommandPlugins = inv.CommandPlugins
	if len(reg.Providers) != 1 {
		t.Errorf("wire assignment dropped providers: %v", reg.Providers)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-im-bridge && go test ./cmd/bridge/ -run TestBuildRegistrationInventory -v
```

Expected: compile error — `buildRegistrationInventory`, `RegistrationInventory` undefined.

- [ ] **Step 3: Create `src-im-bridge/cmd/bridge/inventory.go`**

```go
package main

import (
	"context"
	"fmt"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/core/plugin"
)

// RegistrationInventory is the result of scanning the Bridge's active
// providers and loaded command plugins into the shapes expected by
// client.BridgeRegistration.Providers / CommandPlugins.
type RegistrationInventory struct {
	Providers      []client.BridgeProvider
	CommandPlugins []client.BridgeCommandPlugin
}

// buildRegistrationInventory assembles the multi-provider + command-plugin
// snapshot the orchestrator displays. The activeProvider slice is the full
// list of providers this Bridge process is hosting; pluginReg may be nil
// when IM_BRIDGE_PLUGIN_DIR is unset.
func buildRegistrationInventory(providers []*activeProvider, pluginReg *plugin.Registry) RegistrationInventory {
	inv := RegistrationInventory{}
	for _, p := range providers {
		if p == nil || p.Platform == nil {
			continue
		}
		md := p.Metadata()
		inv.Providers = append(inv.Providers, client.BridgeProvider{
			ID:               md.Source,
			Transport:        p.TransportMode,
			ReadinessTier:    string(md.Capabilities.ReadinessTier),
			CapabilityMatrix: md.Capabilities.Matrix(),
			CallbackPaths:    collectCallbackPaths(p),
			Tenants:          append([]string(nil), p.Tenants...),
			MetadataSource:   "builtin",
		})
	}
	if pluginReg != nil {
		for _, p := range pluginReg.Snapshot() {
			inv.CommandPlugins = append(inv.CommandPlugins, client.BridgeCommandPlugin{
				ID:         p.ID,
				Version:    p.Version,
				Commands:   append([]string(nil), p.Commands...),
				Tenants:    append([]string(nil), p.Tenants...),
				SourcePath: p.SourcePath,
			})
		}
	}
	return inv
}

// collectCallbackPaths reuses the Platform's optional callbackPathProvider
// interface (already used by bridgeRuntimeControl.Start) so inventory
// reporting and register payload stay in sync.
func collectCallbackPaths(p *activeProvider) []string {
	out := []string{"/im/notify", "/im/send"}
	if prov, ok := p.Platform.(callbackPathProvider); ok {
		for _, path := range prov.CallbackPaths() {
			if path != "" {
				out = append(out, path)
			}
		}
	}
	return out
}

// registerBridgeInventory calls /im/bridge/register once for this Bridge
// process with the full provider and command-plugin inventory. It replaces
// the per-RuntimeControl Start() registration that overwrote itself in
// multi-provider deployments.
func registerBridgeInventory(ctx context.Context, cl *client.AgentForgeClient, bridgeID string, cfg *config, providers []*activeProvider, pluginReg *plugin.Registry) error {
	if cl == nil || bridgeID == "" || len(providers) == 0 {
		return nil
	}
	primary := providers[0]
	md := primary.Metadata()
	inv := buildRegistrationInventory(providers, pluginReg)

	tenantIDs := append([]string(nil), primary.Tenants...)
	tenantManifest := buildTenantManifestFromProviders(providers)

	registration := client.BridgeRegistration{
		BridgeID:       bridgeID,
		Platform:       md.Source,
		Transport:      primary.TransportMode,
		ProjectIDs:     []string{cfg.ProjectID},
		Tenants:        tenantIDs,
		TenantManifest: tenantManifest,
		Capabilities: map[string]bool{
			"supports_deferred_reply":  md.Capabilities.SupportsDeferredReply,
			"supports_rich_messages":   md.Capabilities.SupportsRichMessages,
			"requires_public_callback": md.Capabilities.RequiresPublicCallback,
			"supports_mentions":        md.Capabilities.SupportsMentions,
			"supports_slash_commands":  md.Capabilities.SupportsSlashCommands,
		},
		CapabilityMatrix: md.Capabilities.Matrix(),
		CallbackPaths:    collectCallbackPaths(primary),
		Metadata: map[string]string{
			"platform_name":  primary.Platform.Name(),
			"provider_id":    primary.Descriptor.ID,
			"readiness_tier": string(md.Capabilities.ReadinessTier),
		},
		Providers:      inv.Providers,
		CommandPlugins: inv.CommandPlugins,
	}
	if preferredMode := string(md.Capabilities.PreferredAsyncUpdateMode); preferredMode != "" {
		registration.Metadata["preferred_async_update_mode"] = preferredMode
	}
	if fallbackMode := string(md.Capabilities.FallbackAsyncUpdateMode); fallbackMode != "" {
		registration.Metadata["fallback_async_update_mode"] = fallbackMode
	}

	if _, err := cl.RegisterBridge(ctx, registration); err != nil {
		return fmt.Errorf("register bridge inventory: %w", err)
	}
	return nil
}

// buildTenantManifestFromProviders consolidates the per-provider tenant
// slices into a single []client.TenantBinding for the top-level payload.
// Duplicate (id, projectId) pairs are suppressed.
func buildTenantManifestFromProviders(providers []*activeProvider) []client.TenantBinding {
	seen := map[string]struct{}{}
	var out []client.TenantBinding
	for _, p := range providers {
		for _, tid := range p.Tenants {
			key := tid
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			// ProjectID is not carried on activeProvider per-tenant; leave
			// empty. The orchestrator resolves projectId separately.
			out = append(out, client.TenantBinding{ID: tid})
		}
	}
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd src-im-bridge && go test ./cmd/bridge/ -run TestBuildRegistrationInventory -v
```

Expected: 3 sub-tests PASS.

- [ ] **Step 5: Commit**

```bash
git add src-im-bridge/cmd/bridge/inventory.go src-im-bridge/cmd/bridge/inventory_test.go
git commit -m "feat(im-bridge): assemble multi-provider registration inventory"
```

---

## Task C5: Hoist registration out of bridgeRuntimeControl + call it once from main.go

**Files:**
- Modify: `src-im-bridge/cmd/bridge/control_plane.go`
- Modify: `src-im-bridge/cmd/bridge/main.go`

- [ ] **Step 1: Remove the inline registration from `bridgeRuntimeControl.Start`**

In `src-im-bridge/cmd/bridge/control_plane.go`, locate `Start` (line 66–131) and delete the block that constructs `registration` and calls `c.client.RegisterBridge`. Start becomes:

```go
func (c *bridgeRuntimeControl) Start(ctx context.Context) error {
	if c == nil || c.client == nil || c.provider == nil || c.provider.Platform == nil {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(2)
	go func() {
		defer c.wg.Done()
		c.heartbeatLoop(runCtx)
	}()
	go func() {
		defer c.wg.Done()
		c.controlPlaneLoop(runCtx)
	}()
	return nil
}
```

Remove the now-unused `metadata`, `tenantIDs`, `tenantManifest`, and callback-path assembly code — the top-level registrant owns it now. Keep `SetTenants` since it still supplies tenant IDs for heartbeat metadata (if any caller still uses that path).

- [ ] **Step 2: Call `registerBridgeInventory` once from `main.go`**

In `src-im-bridge/cmd/bridge/main.go`, locate the loop that creates each `RuntimeControl` and `Start`s it (around line 652). Before the loop, add a single top-level registration call:

```go
// Register the Bridge process with the orchestrator once, carrying the
// full multi-provider + command-plugin inventory. Must happen before the
// per-provider heartbeat / WS goroutines start so the control plane has
// the up-to-date capability matrix for delivery routing.
if err := registerBridgeInventory(context.Background(), apiClient, bridgeID, cfg, extractActiveProviders(bindings), pluginRegistry); err != nil {
	log.WithField("component", "main").WithError(err).Fatal("Bridge registration failed")
}
```

Add a helper in the same file:

```go
func extractActiveProviders(bindings []*providerBinding) []*activeProvider {
	out := make([]*activeProvider, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, b.Provider)
	}
	return out
}
```

- [ ] **Step 3: Ensure graceful shutdown still calls `UnregisterBridge`**

In `main.go`'s shutdown block (around line 788), unregistration is called for every `RuntimeControl`. Since registration is now a one-shot, change this to a single top-level unregister. Replace:

```go
if b.RuntimeControl != nil {
    _ = b.RuntimeControl.Stop(context.Background())
}
```

…with the existing loop keeping `Stop` for per-RuntimeControl cleanup (heartbeat + WS), plus a single top-level call after the loop:

```go
_ = apiClient.UnregisterBridge(context.Background(), bridgeID)
```

Inside `bridgeRuntimeControl.Stop`, remove the `c.client.UnregisterBridge` call — unregistration is now a main.go responsibility. Stop becomes heartbeat+WS teardown only:

```go
func (c *bridgeRuntimeControl) Stop(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	return nil
}
```

- [ ] **Step 4: Update `cmd/bridge/control_plane_test.go`**

The existing tests intercept `/api/v1/im/bridge/register` traffic originating from `bridgeRuntimeControl.Start`. Since registration is now driven by `registerBridgeInventory`, update the fixtures accordingly:

1. Inspect each test in `cmd/bridge/control_plane_test.go` that asserts a registration request is made during `Start()`. That assertion is obsolete — remove it or move it to a new `inventory_integration_test.go` that exercises `registerBridgeInventory` directly.
2. Keep tests that cover heartbeat/WS consumption; those are unchanged.

Concretely, the 4 `case "/api/v1/im/bridge/register":` branches in `cmd/bridge/control_plane_test.go` are still valid as server-side stubs (they handle the call from `registerBridgeInventory` when the test drives the full boot path). If a test previously called `control.Start()` and expected a register round-trip, adapt it to call `registerBridgeInventory` explicitly.

- [ ] **Step 5: Run all Bridge cmd tests**

```bash
cd src-im-bridge && go test ./cmd/bridge/ -v
```

Expected: all tests PASS. If a pre-existing test breaks, follow the Step 4 adaptation — do not weaken the test expectations.

- [ ] **Step 6: Commit**

```bash
git add src-im-bridge/cmd/bridge/control_plane.go src-im-bridge/cmd/bridge/main.go src-im-bridge/cmd/bridge/control_plane_test.go
git commit -m "refactor(im-bridge): hoist bridge registration to a single process-level call"
```

---

## Task C6: Re-register triggers on SIGHUP / plugin-reload / reconcile

**Files:**
- Modify: `src-im-bridge/cmd/bridge/main.go`
- Modify: `src-im-bridge/cmd/bridge/hotreload_unix.go`
- Modify: `src-im-bridge/cmd/bridge/hotreload_windows.go`

- [ ] **Step 1: Expose a re-register callback the hotreload paths can call**

In `src-im-bridge/cmd/bridge/main.go`, define a closure alongside the one-shot register call:

```go
bridgeCtx, bridgeCancel := context.WithCancel(context.Background())
defer bridgeCancel()

activeProviders := extractActiveProviders(bindings)

reregister := func() {
	if err := registerBridgeInventory(bridgeCtx, apiClient, bridgeID, cfg, activeProviders, pluginRegistry); err != nil {
		log.WithField("component", "main").WithError(err).Warn("re-registration failed")
	}
}

if err := registerBridgeInventory(bridgeCtx, apiClient, bridgeID, cfg, activeProviders, pluginRegistry); err != nil {
	log.WithField("component", "main").WithError(err).Fatal("Bridge registration failed")
}
```

Pass `reregister` to the hotreload signal handler (Unix) and to any polling watcher that already calls `plugin.Registry.ReloadAll`. The Windows handler that already exists does nothing special; do the same there.

- [ ] **Step 2: Trigger re-register on tenants reload**

Locate the SIGHUP handler in `src-im-bridge/cmd/bridge/hotreload_unix.go` that reloads tenants.yaml and the plugin registry. Append:

```go
// After a successful SIGHUP reconcile, refresh the backend's view of
// tenants and command-plugin inventory.
reregister()
```

- [ ] **Step 3: Trigger re-register after a successful `plugin.Registry.ReloadAll`**

In `main.go`, the plugin registry's watcher is started via `pluginRegistry.StartWatcher(pluginCtx, 30*time.Second)`. Instead of relying on the watcher's internal call to `ReloadAll`, wrap it:

Add a small helper in `core/plugin/plugin.go` that lets callers supply an `OnReload` callback (optional, called only when a reload actually produced changes):

```go
// StartWatcherWithCallback is identical to StartWatcher but invokes fn
// after every successful ReloadAll. fn may be nil.
func (r *Registry) StartWatcherWithCallback(ctx context.Context, interval time.Duration, fn func()) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := r.ReloadAll(); err == nil && fn != nil {
					fn()
				}
			}
		}
	}()
}
```

In `main.go`, replace `pluginRegistry.StartWatcher(pluginCtx, 30*time.Second)` with:

```go
pluginRegistry.StartWatcherWithCallback(pluginCtx, 30*time.Second, reregister)
```

- [ ] **Step 4: Trigger re-register after a HotReloader.Reconcile that applied changes**

Locate the hotreload path that calls `HotReloader.Reconcile` on each provider. After the loop completes, if any provider returned `ReconcileResult.Applied` with non-empty fields, call `reregister()` once.

- [ ] **Step 5: Run Bridge tests**

```bash
cd src-im-bridge && go test ./...
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add src-im-bridge/cmd/bridge/main.go src-im-bridge/cmd/bridge/hotreload_unix.go src-im-bridge/cmd/bridge/hotreload_windows.go src-im-bridge/core/plugin/plugin.go
git commit -m "feat(im-bridge): re-register inventory on SIGHUP / plugin reload / reconcile"
```

---

## Task C7: Orchestrator-side — copy new fields + synthesize legacy Providers[0]

**Files:**
- Modify: `src-go/internal/service/im_control_plane.go`

- [ ] **Step 1: Copy the new fields into `IMBridgeInstance` in `RegisterBridge`**

Locate `RegisterBridge` in `src-go/internal/service/im_control_plane.go` (line 163). Extend the `record := &model.IMBridgeInstance{...}` assignment to include:

```go
record := &model.IMBridgeInstance{
	BridgeID:         strings.TrimSpace(req.BridgeID),
	Platform:         normalizePlatform(req.Platform),
	Transport:        strings.TrimSpace(req.Transport),
	ProjectIDs:       dedupeStrings(req.ProjectIDs),
	Capabilities:     cloneBoolMap(req.Capabilities),
	CapabilityMatrix: cloneAnyMap(req.CapabilityMatrix),
	CallbackPaths:    dedupeStrings(req.CallbackPaths),
	Metadata:         cloneStringMap(req.Metadata),
	Tenants:          dedupeStrings(req.Tenants),
	TenantManifest:   cloneTenantManifest(req.TenantManifest),
	Providers:        cloneProviders(req.Providers),
	CommandPlugins:   cloneCommandPlugins(req.CommandPlugins),
	Status:           "online",
}
```

- [ ] **Step 2: Add the clone helpers**

At the bottom of `im_control_plane.go` (near the other `clone*` helpers), add:

```go
func cloneProviders(in []model.IMBridgeProvider) []model.IMBridgeProvider {
	if len(in) == 0 {
		return nil
	}
	out := make([]model.IMBridgeProvider, len(in))
	for i, p := range in {
		out[i] = model.IMBridgeProvider{
			ID:               p.ID,
			Transport:        p.Transport,
			ReadinessTier:    p.ReadinessTier,
			CapabilityMatrix: cloneAnyMap(p.CapabilityMatrix),
			CallbackPaths:    dedupeStrings(p.CallbackPaths),
			Tenants:          dedupeStrings(p.Tenants),
			MetadataSource:   p.MetadataSource,
		}
	}
	return out
}

func cloneCommandPlugins(in []model.IMBridgeCommandPlugin) []model.IMBridgeCommandPlugin {
	if len(in) == 0 {
		return nil
	}
	out := make([]model.IMBridgeCommandPlugin, len(in))
	for i, p := range in {
		out[i] = model.IMBridgeCommandPlugin{
			ID:         p.ID,
			Version:    p.Version,
			Commands:   append([]string(nil), p.Commands...),
			Tenants:    append([]string(nil), p.Tenants...),
			SourcePath: p.SourcePath,
		}
	}
	return out
}
```

- [ ] **Step 3: Synthesize `Providers[0]` for legacy bridges in `GetBridgeStatus`**

Locate `GetBridgeStatus`. Before returning, enrich each bridge record that has `Providers == nil`:

```go
for _, bridge := range status.Bridges {
	if len(bridge.Providers) == 0 && bridge.Platform != "" {
		bridge.Providers = []model.IMBridgeProvider{{
			ID:               bridge.Platform,
			Transport:        bridge.Transport,
			ReadinessTier:    bridge.Metadata["readiness_tier"],
			CapabilityMatrix: bridge.CapabilityMatrix,
			CallbackPaths:    bridge.CallbackPaths,
			Tenants:          bridge.Tenants,
			MetadataSource:   "builtin",
		}}
	}
}
```

The exact field access path depends on the existing `GetBridgeStatus` return shape — verify it before writing the loop. If `status.Bridges` is a `[]*model.IMBridgeInstance`, mutate in place. If it is a DTO wrapper, mutate that struct's inner slice.

- [ ] **Step 4: Update `cloneBridgeInstance`**

Locate `cloneBridgeInstance` near the top/bottom of the file. Ensure it copies the two new fields:

```go
return &model.IMBridgeInstance{
	// ... existing field copies ...
	Providers:      cloneProviders(src.Providers),
	CommandPlugins: cloneCommandPlugins(src.CommandPlugins),
}
```

- [ ] **Step 5: Run existing tests to check for regressions**

```bash
cd src-go && go test ./internal/service/ -run TestIMControlPlane -v
```

Expected: all pre-existing IM control-plane tests PASS.

- [ ] **Step 6: Commit**

```bash
git add src-go/internal/service/im_control_plane.go
git commit -m "feat(im): copy Providers/CommandPlugins through RegisterBridge + synthesize for legacy bridges"
```

---

## Task C8: Orchestrator test coverage for multi-provider + command plugins

**Files:**
- Modify: `src-go/internal/service/im_control_plane_test.go`

- [ ] **Step 1: Add a multi-provider registration test**

Append to `src-go/internal/service/im_control_plane_test.go`:

```go
func TestRegisterBridge_MultiProviderInventory(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL: time.Minute,
	})

	req := &IMBridgeRegisterRequest{
		BridgeID:  "bridge-multi",
		Platform:  "feishu",
		Transport: "live",
		Providers: []model.IMBridgeProvider{
			{
				ID:             "feishu",
				Transport:      "live",
				ReadinessTier:  "full_native_lifecycle",
				Tenants:        []string{"acme"},
				MetadataSource: "builtin",
			},
			{
				ID:             "slack",
				Transport:      "stub",
				Tenants:        []string{"beta"},
				MetadataSource: "builtin",
			},
		},
		CommandPlugins: []model.IMBridgeCommandPlugin{
			{
				ID:       "@acme/jira",
				Version:  "1.0.0",
				Commands: []string{"/jira"},
				Tenants:  []string{"acme"},
			},
		},
	}

	if _, err := control.RegisterBridge(context.Background(), req); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}

	status, err := control.GetBridgeStatus(context.Background())
	if err != nil {
		t.Fatalf("GetBridgeStatus error: %v", err)
	}
	if len(status.Bridges) != 1 {
		t.Fatalf("Bridges len = %d, want 1", len(status.Bridges))
	}
	bridge := status.Bridges[0]
	if len(bridge.Providers) != 2 {
		t.Fatalf("Providers len = %d, want 2", len(bridge.Providers))
	}
	if bridge.Providers[0].ID != "feishu" || bridge.Providers[1].ID != "slack" {
		t.Errorf("provider ids = %q, %q", bridge.Providers[0].ID, bridge.Providers[1].ID)
	}
	if len(bridge.CommandPlugins) != 1 || bridge.CommandPlugins[0].ID != "@acme/jira" {
		t.Errorf("command plugins = %v", bridge.CommandPlugins)
	}
}

func TestGetBridgeStatus_LegacyBridgeSynthesizesProvider(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL: time.Minute,
	})

	req := &IMBridgeRegisterRequest{
		BridgeID:         "bridge-legacy",
		Platform:         "slack",
		Transport:        "live",
		CapabilityMatrix: map[string]any{"supportsRichMessages": true},
		CallbackPaths:    []string{"/im/notify", "/im/send"},
		Tenants:          []string{"acme"},
		Metadata:         map[string]string{"readiness_tier": "native_send_with_fallback"},
	}
	if _, err := control.RegisterBridge(context.Background(), req); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}

	status, err := control.GetBridgeStatus(context.Background())
	if err != nil {
		t.Fatalf("GetBridgeStatus error: %v", err)
	}
	if len(status.Bridges) != 1 {
		t.Fatalf("Bridges len = %d", len(status.Bridges))
	}
	providers := status.Bridges[0].Providers
	if len(providers) != 1 {
		t.Fatalf("synthesized Providers len = %d, want 1", len(providers))
	}
	if providers[0].ID != "slack" || providers[0].Transport != "live" {
		t.Errorf("synthesized provider = %+v", providers[0])
	}
	if providers[0].ReadinessTier != "native_send_with_fallback" {
		t.Errorf("synthesized ReadinessTier = %q", providers[0].ReadinessTier)
	}
}
```

- [ ] **Step 2: Run the new tests**

```bash
cd src-go && go test ./internal/service/ -run "TestRegisterBridge_MultiProviderInventory|TestGetBridgeStatus_LegacyBridgeSynthesizesProvider" -v
```

Expected: both PASS. Adjust the exact `status.Bridges[0]` field path if `GetBridgeStatus` returns a different shape than assumed (inspect the response struct before running).

- [ ] **Step 3: Commit**

```bash
git add src-go/internal/service/im_control_plane_test.go
git commit -m "test(im): multi-provider and legacy-synthesis registration cases"
```

---

## Task C9: Frontend `BridgeInventoryPanel` component + plugins page integration

**Files:**
- Create: `components/im/bridge-inventory-panel.tsx`
- Create: `components/im/bridge-inventory-panel.test.tsx`
- Modify: `app/(dashboard)/plugins/page.tsx`

- [ ] **Step 1: Write the failing render test**

Create `components/im/bridge-inventory-panel.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { BridgeInventoryPanel } from "./bridge-inventory-panel";

const sampleBridges = [
  {
    bridgeId: "bridge-abc",
    platform: "feishu",
    status: "online",
    providers: [
      {
        id: "feishu",
        transport: "live",
        readinessTier: "full_native_lifecycle",
        capabilityMatrix: { supportsRichMessages: true, supportsAttachments: true },
        tenants: ["acme"],
        metadataSource: "builtin",
      },
      {
        id: "slack",
        transport: "stub",
        tenants: ["beta"],
        metadataSource: "builtin",
      },
    ],
    commandPlugins: [
      { id: "@acme/jira", version: "1.0.0", commands: ["/jira"], tenants: ["acme"] },
    ],
  },
];

describe("BridgeInventoryPanel", () => {
  it("renders providers with readiness tier and tenant badges", () => {
    render(<BridgeInventoryPanel bridges={sampleBridges} />);
    expect(screen.getByText("feishu")).toBeInTheDocument();
    expect(screen.getByText("full_native_lifecycle")).toBeInTheDocument();
    expect(screen.getByText("acme")).toBeInTheDocument();
    expect(screen.getByText("slack")).toBeInTheDocument();
  });

  it("renders command plugins with command list", () => {
    render(<BridgeInventoryPanel bridges={sampleBridges} />);
    expect(screen.getByText("@acme/jira")).toBeInTheDocument();
    expect(screen.getByText("/jira")).toBeInTheDocument();
  });

  it("renders empty state when no bridges are online", () => {
    render(<BridgeInventoryPanel bridges={[]} />);
    expect(screen.getByText(/no IM bridges online/i)).toBeInTheDocument();
  });

  it("dims offline bridges", () => {
    const offline = [{ ...sampleBridges[0], status: "offline" }];
    render(<BridgeInventoryPanel bridges={offline} />);
    const card = screen.getByTestId("bridge-card-bridge-abc");
    expect(card.className).toMatch(/opacity-/);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
pnpm test components/im/bridge-inventory-panel.test.tsx
```

Expected: fail — module not found.

- [ ] **Step 3: Implement `components/im/bridge-inventory-panel.tsx`**

```tsx
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export interface BridgeProvider {
  id: string;
  transport: string;
  readinessTier?: string;
  capabilityMatrix?: Record<string, unknown>;
  callbackPaths?: string[];
  tenants?: string[];
  metadataSource: string;
}

export interface BridgeCommandPlugin {
  id: string;
  version: string;
  commands: string[];
  tenants?: string[];
  sourcePath?: string;
}

export interface BridgeInventoryEntry {
  bridgeId: string;
  platform: string;
  status: string;
  providers?: BridgeProvider[];
  commandPlugins?: BridgeCommandPlugin[];
}

export interface BridgeInventoryPanelProps {
  bridges: BridgeInventoryEntry[];
}

export function BridgeInventoryPanel({ bridges }: BridgeInventoryPanelProps) {
  if (bridges.length === 0) {
    return (
      <Card>
        <CardContent className="py-10 text-center text-sm text-muted-foreground">
          No IM bridges online. Start <code>src-im-bridge</code> to populate this panel.
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      {bridges.map((bridge) => (
        <Card
          key={bridge.bridgeId}
          data-testid={`bridge-card-${bridge.bridgeId}`}
          className={cn(bridge.status !== "online" && "opacity-60")}
        >
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-base">
              {bridge.bridgeId}
              <span className="ml-2 text-xs font-normal text-muted-foreground">
                ({bridge.status})
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <section>
              <h3 className="mb-2 text-sm font-semibold">Providers</h3>
              <ul className="space-y-2">
                {(bridge.providers ?? []).map((p) => (
                  <li key={p.id} className="rounded-md border p-3 text-sm">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{p.id}</span>
                      <Badge variant="outline">{p.transport}</Badge>
                      {p.readinessTier && <Badge variant="secondary">{p.readinessTier}</Badge>}
                    </div>
                    {(p.tenants?.length ?? 0) > 0 && (
                      <div className="mt-2 flex flex-wrap gap-1">
                        {p.tenants!.map((t) => (
                          <Badge key={t} variant="outline">{t}</Badge>
                        ))}
                      </div>
                    )}
                    {p.capabilityMatrix && (
                      <details className="mt-2 text-xs text-muted-foreground">
                        <summary className="cursor-pointer">Capability matrix</summary>
                        <pre className="mt-1 overflow-auto">{JSON.stringify(p.capabilityMatrix, null, 2)}</pre>
                      </details>
                    )}
                  </li>
                ))}
              </ul>
            </section>
            <section>
              <h3 className="mb-2 text-sm font-semibold">Command Plugins</h3>
              {(bridge.commandPlugins?.length ?? 0) === 0 ? (
                <p className="text-xs text-muted-foreground">None loaded.</p>
              ) : (
                <ul className="space-y-2">
                  {bridge.commandPlugins!.map((cp) => (
                    <li key={cp.id} className="rounded-md border p-3 text-sm">
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{cp.id}</span>
                        <Badge variant="outline">v{cp.version}</Badge>
                      </div>
                      <div className="mt-2 flex flex-wrap gap-1">
                        {cp.commands.map((cmd) => (
                          <Badge key={cmd} variant="secondary">{cmd}</Badge>
                        ))}
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
```

- [ ] **Step 4: Integrate the panel into the plugins page**

Modify `app/(dashboard)/plugins/page.tsx`. Locate the existing render tree. Insert a new section (after the main plugin card list):

```tsx
import { BridgeInventoryPanel } from "@/components/im/bridge-inventory-panel";
// ...

// Within the page body, after existing content:
<section className="mt-8">
  <h2 className="mb-3 text-lg font-semibold">IM Bridge Providers</h2>
  <p className="mb-4 text-sm text-muted-foreground">
    Read-only view of providers and command plugins loaded by every online IM Bridge
    process. Configuration is owned by the Bridge's environment; use this view for audit.
  </p>
  <BridgeInventoryPanel bridges={bridges} />
</section>
```

`bridges` must be hydrated from the existing IM bridge status endpoint. If the page is a Client Component using a Zustand store, add a selector like `useIMStore((s) => s.bridges)`. If it is a Server Component fetching via `fetch`, call `GET /api/v1/im/bridge/status` and pass the response through. The exact plumbing follows the page's current data-fetching pattern — inspect the file first.

- [ ] **Step 5: Run tests to verify they pass**

```bash
pnpm test components/im/bridge-inventory-panel.test.tsx
pnpm test app/\(dashboard\)/plugins/page.test.tsx
```

Expected: all PASS. Fix any import path or component shape differences surfaced by the existing `page.test.tsx`.

- [ ] **Step 6: Commit**

```bash
git add components/im/bridge-inventory-panel.tsx components/im/bridge-inventory-panel.test.tsx app/\(dashboard\)/plugins/page.tsx
git commit -m "feat(frontend): add IM Bridge inventory panel to plugins page"
```

---

## Task C10: Final integration check

**Files:** none created; runs full test suites.

- [ ] **Step 1: Backend Go tests**

```bash
cd src-go && go test ./...
```

Expected: all PASS.

- [ ] **Step 2: Bridge Go tests**

```bash
cd src-im-bridge && go test ./...
```

Expected: all PASS, including `TestAllBuiltinProvidersRegistered`.

- [ ] **Step 3: Frontend tests**

```bash
pnpm test
```

Expected: all PASS.

- [ ] **Step 4: Plugin workflow scripts**

```bash
pnpm exec vitest run scripts/plugin/
```

Expected: all PASS.

- [ ] **Step 5: Type check**

```bash
pnpm exec tsc --noEmit
```

Expected: no errors.

- [ ] **Step 6: Lint**

```bash
pnpm lint
```

Expected: no errors.

- [ ] **Step 7: Confirm no IM Bridge protocol duplication**

Sanity check against the spec's non-duplication guarantees:

```bash
# No new WS message types:
grep -rn "bridge.inventory.snapshot\|bridge_inventory_snapshot" src-go/ src-im-bridge/ components/ app/
# Expected: empty output.

# No new bridge-only HTTP endpoint:
grep -rn '/api/v1/im/bridges/\|/api/v1/im/inventory' src-go/ app/ components/
# Expected: empty output.

# No new migration touching bridge inventory:
ls src-go/migrations/*.sql | xargs grep -l "im_bridge_inventory\|bridge_inventory"
# Expected: empty output.
```

- [ ] **Step 8: Final commit (if anything remains uncommitted)**

```bash
git status
# If unstaged changes exist, review and commit them with a concise message referencing this plan.
```

---

# Rollback Notes

Each phase is a distinct commit range:
- Phase A commits: A1 → A4.
- Phase B commits: B1 → B4.
- Phase C commits: C1 → C10.

To roll back:
- Phase C rollback is safe (additive fields + UI panel); revert its commits only.
- Phase B rollback requires reverting the directory rename + restoring the four TS test fixtures + builtin bundle entry.
- Phase A rollback requires restoring the full `providerDescriptors()` switch in `platform_registry.go` and removing blank imports — do this only if the factory registry surfaces a critical defect.

No database schema changes, no migrations to reverse.

---

# Spec Coverage Check

| Spec section | Plan task(s) |
|--------------|--------------|
| §3.1 Core Provider Registry primitives | A1 |
| §3.2 Per-provider registration | A3 |
| §3.3 `platform_registry.go` contraction | A4 |
| §3.4 `ProviderEnv` implementation | A2 |
| §3.5 New-provider workflow | (documented in this plan; no task — validated by A4's guard test) |
| §3.6 Risks & tradeoffs | (addressed across A1–A4) |
| §3.7 Tests | A1 (registry), A2 (env), A4 (guard) |
| §4.1 Extend `IMBridgeRegisterRequest` | C1 + C2 |
| §4.2 Bridge-side assembly | C4 |
| §4.3 `Registry.Snapshot()` | C3 |
| §4.4 Re-registration triggers | C6 |
| §4.5 Orchestrator-side handling | C7 |
| §4.6 Frontend panel | C9 |
| §4.7 Duplication audit | C10 Step 7 |
| §4.8 Risks & tradeoffs | (addressed across C5–C7) |
| §4.9 Tests | C3, C4, C8, C9 |
| §5.1–5.4 feishu-adapter rename | B1–B4 |
| §6 Implementation order | Phases A → B → C |
| §7 What does not change | Tasks touch only the files listed in each task's file map |
| §8 Out-of-scope | No task |
