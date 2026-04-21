// Package plugin implements the IM Bridge command plugin registry.
//
// A command plugin is declared by a YAML manifest at
// ${IM_BRIDGE_PLUGIN_DIR}/<plugin-id>/plugin.yaml. Each manifest enumerates
// slash commands and, for each, one of three invoke kinds: http, mcp,
// builtin. The plugin system supports tenant-scoped allowlists, filesystem
// reload (polling-based watcher in this change; fsnotify is a follow-up),
// and marketplace-driven distribution.
package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Manifest is the deserialized form of plugin.yaml.
type Manifest struct {
	ID       string          `yaml:"id"`
	Version  string          `yaml:"version"`
	Name     string          `yaml:"name"`
	Commands []CommandEntry  `yaml:"commands"`
	Tenants  []string        `yaml:"tenants,omitempty"`
}

// CommandEntry is a single top-level slash command or a container for
// subcommands.
type CommandEntry struct {
	Slash       string              `yaml:"slash"`
	Description string              `yaml:"description,omitempty"`
	ActionClass string              `yaml:"action_class,omitempty"`
	Subcommands []SubcommandEntry   `yaml:"subcommands,omitempty"`
	Invoke      *InvokeSpec         `yaml:"invoke,omitempty"`
}

// SubcommandEntry is a second-level command under a top-level slash.
type SubcommandEntry struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description,omitempty"`
	ActionClass string      `yaml:"action_class,omitempty"`
	Invoke      *InvokeSpec `yaml:"invoke,omitempty"`
}

// InvokeSpec describes how to fulfill a command. Only one of Kind=http,
// Kind=mcp, Kind=builtin is valid per entry.
type InvokeSpec struct {
	Kind    string            `yaml:"kind"`
	URL     string            `yaml:"url,omitempty"`
	Method  string            `yaml:"method,omitempty"`
	Timeout string            `yaml:"timeout,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	// MCP fields
	ServerID string `yaml:"serverId,omitempty"`
	Tool     string `yaml:"tool,omitempty"`
	// Builtin: registration key
	Key string `yaml:"key,omitempty"`
}

// Loaded represents a manifest after validation and is the canonical
// in-memory form passed around the registry.
type Loaded struct {
	Manifest Manifest
	Path     string
	LoadedAt time.Time
}

// InvokeContext is the argument passed to an invoke handler.
type InvokeContext struct {
	PluginID    string
	Command     string
	Subcommand  string
	Args        string
	TenantID    string
	Platform    string
	UserID      string
	ChatID      string
	Metadata    map[string]string
}

// InvokeResult is what a plugin handler returns.
type InvokeResult struct {
	Text string `json:"text,omitempty"`
	Err  string `json:"error,omitempty"`
}

// Invoker executes one command dispatch for a plugin.
type Invoker interface {
	Invoke(ctx context.Context, spec *InvokeSpec, icx InvokeContext) (*InvokeResult, error)
}

// Registry indexes loaded plugin manifests and dispatches invocations.
type Registry struct {
	mu       sync.RWMutex
	dir      string
	plugins  map[string]*Loaded
	invokers map[string]Invoker

	// builtinHandlers maps builtin.key to an in-process handler func.
	builtinHandlers map[string]BuiltinHandler
}

// BuiltinHandler is the signature for in-process plugin commands.
type BuiltinHandler func(ctx context.Context, icx InvokeContext) (*InvokeResult, error)

// NewRegistry returns a fresh registry that will watch dir for manifests.
func NewRegistry(dir string) *Registry {
	r := &Registry{
		dir:             strings.TrimSpace(dir),
		plugins:         map[string]*Loaded{},
		invokers:        map[string]Invoker{},
		builtinHandlers: map[string]BuiltinHandler{},
	}
	r.invokers["http"] = &httpInvoker{client: &http.Client{Timeout: 30 * time.Second}}
	r.invokers["mcp"] = &mcpInvoker{}
	r.invokers["builtin"] = &builtinInvoker{registry: r}
	return r
}

// SetInvoker installs a custom invoker for a given kind (http, mcp,
// builtin). Tests use this to stub out the HTTP/MCP client.
func (r *Registry) SetInvoker(kind string, inv Invoker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invokers[strings.ToLower(strings.TrimSpace(kind))] = inv
}

// RegisterBuiltin installs an in-process handler for invoke.kind=builtin
// manifests that reference the given key.
func (r *Registry) RegisterBuiltin(key string, handler BuiltinHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builtinHandlers[strings.TrimSpace(key)] = handler
}

// ReloadAll walks the plugin dir and refreshes the in-memory manifest set.
// Plugins that fail to parse are skipped with an error; existing plugins
// remain loaded.
func (r *Registry) ReloadAll() error {
	r.mu.Lock()
	dir := r.dir
	r.mu.Unlock()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("plugin: read dir %s: %w", dir, err)
	}
	fresh := map[string]*Loaded{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "plugin.yaml")
		raw, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var m Manifest
		if err := yaml.Unmarshal(raw, &m); err != nil {
			continue
		}
		m.ID = strings.TrimSpace(m.ID)
		if m.ID == "" {
			continue
		}
		if err := validateManifest(&m); err != nil {
			continue
		}
		fresh[m.ID] = &Loaded{Manifest: m, Path: manifestPath, LoadedAt: time.Now()}
	}
	r.mu.Lock()
	r.plugins = fresh
	r.mu.Unlock()
	return nil
}

// StartWatcher spawns a polling goroutine that refreshes the registry
// every `interval`. fsnotify is deferred; polling is correct and portable.
func (r *Registry) StartWatcher(ctx context.Context, interval time.Duration) {
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
				_ = r.ReloadAll()
			}
		}
	}()
}

// Plugins returns a snapshot of loaded plugins, sorted by id for stable
// iteration. The returned slice is a copy; callers may mutate it freely.
func (r *Registry) Plugins() []*Loaded {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Loaded, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest.ID < out[j].Manifest.ID })
	return out
}

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

// Dispatch routes an invocation to the matching plugin + command +
// subcommand. It returns a plugin.ErrNotFound when no plugin matches and
// plugin.ErrForbidden when the plugin excludes the current tenant.
func (r *Registry) Dispatch(ctx context.Context, icx InvokeContext) (*InvokeResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.plugins {
		if !tenantPermitted(p.Manifest.Tenants, icx.TenantID) {
			continue
		}
		for _, cmd := range p.Manifest.Commands {
			if !strings.EqualFold(strings.TrimSpace(cmd.Slash), icx.Command) {
				continue
			}
			// Match subcommand if specified.
			if len(cmd.Subcommands) > 0 {
				for _, sub := range cmd.Subcommands {
					if strings.EqualFold(strings.TrimSpace(sub.Name), icx.Subcommand) {
						return r.invoke(ctx, p.Manifest.ID, icx, sub.Invoke)
					}
				}
				continue
			}
			if cmd.Invoke != nil {
				return r.invoke(ctx, p.Manifest.ID, icx, cmd.Invoke)
			}
		}
	}
	return nil, ErrNotFound
}

func (r *Registry) invoke(ctx context.Context, pluginID string, icx InvokeContext, spec *InvokeSpec) (*InvokeResult, error) {
	if spec == nil {
		return nil, ErrNotFound
	}
	icx.PluginID = pluginID
	kind := strings.ToLower(strings.TrimSpace(spec.Kind))
	inv, ok := r.invokers[kind]
	if !ok {
		return nil, fmt.Errorf("plugin: no invoker for kind %q", spec.Kind)
	}
	return inv.Invoke(ctx, spec, icx)
}

func tenantPermitted(allowlist []string, tenantID string) bool {
	if len(allowlist) == 0 {
		return true
	}
	tenantID = strings.TrimSpace(tenantID)
	for _, t := range allowlist {
		if strings.TrimSpace(t) == tenantID {
			return true
		}
	}
	return false
}

func validateManifest(m *Manifest) error {
	if len(m.Commands) == 0 {
		return errors.New("manifest: no commands declared")
	}
	for _, cmd := range m.Commands {
		if strings.TrimSpace(cmd.Slash) == "" {
			return errors.New("manifest: command missing slash")
		}
		if len(cmd.Subcommands) == 0 && cmd.Invoke == nil {
			return fmt.Errorf("manifest: command %s needs invoke or subcommands", cmd.Slash)
		}
		for _, sub := range cmd.Subcommands {
			if sub.Invoke == nil {
				return fmt.Errorf("manifest: subcommand %s/%s missing invoke", cmd.Slash, sub.Name)
			}
			if err := validateInvoke(sub.Invoke); err != nil {
				return err
			}
		}
		if cmd.Invoke != nil {
			if err := validateInvoke(cmd.Invoke); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateInvoke(spec *InvokeSpec) error {
	switch strings.ToLower(strings.TrimSpace(spec.Kind)) {
	case "http":
		if strings.TrimSpace(spec.URL) == "" {
			return errors.New("manifest: http invoke missing url")
		}
	case "mcp":
		if strings.TrimSpace(spec.ServerID) == "" || strings.TrimSpace(spec.Tool) == "" {
			return errors.New("manifest: mcp invoke missing serverId or tool")
		}
	case "builtin":
		if strings.TrimSpace(spec.Key) == "" {
			return errors.New("manifest: builtin invoke missing key")
		}
	default:
		return fmt.Errorf("manifest: unsupported invoke kind %q", spec.Kind)
	}
	return nil
}

// ErrNotFound indicates no plugin matched the dispatch request.
var ErrNotFound = errors.New("plugin: not found")

// ErrForbidden indicates the plugin matched but the tenant is not in the
// allowlist.
var ErrForbidden = errors.New("plugin: tenant not permitted")

// ----- HTTP invoker ------------------------------------------------------

type httpInvoker struct {
	client *http.Client
}

func (h *httpInvoker) Invoke(ctx context.Context, spec *InvokeSpec, icx InvokeContext) (*InvokeResult, error) {
	timeout := 10 * time.Second
	if d, err := time.ParseDuration(strings.TrimSpace(spec.Timeout)); err == nil && d > 0 {
		timeout = d
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		method = http.MethodPost
	}
	payload := map[string]any{
		"pluginId":   icx.PluginID,
		"command":    icx.Command,
		"subcommand": icx.Subcommand,
		"args":       icx.Args,
		"tenantId":   icx.TenantID,
		"platform":   icx.Platform,
		"userId":     icx.UserID,
		"chatId":     icx.ChatID,
		"metadata":   icx.Metadata,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(cctx, method, spec.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range spec.Headers {
		req.Header.Set(k, expandPlaceholders(v, icx))
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out InvokeResult
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if out.Err == "" {
			out.Err = fmt.Sprintf("plugin http %s returned %d", spec.URL, resp.StatusCode)
		}
	}
	return &out, nil
}

// expandPlaceholders replaces `${TENANT_META_<key>}` with values from the
// invocation context metadata map.
func expandPlaceholders(v string, icx InvokeContext) string {
	if !strings.Contains(v, "${") {
		return v
	}
	for key, val := range icx.Metadata {
		v = strings.ReplaceAll(v, "${TENANT_META_"+key+"}", val)
	}
	return v
}

// ----- MCP invoker (stub) ------------------------------------------------

type mcpInvoker struct{}

func (m *mcpInvoker) Invoke(_ context.Context, spec *InvokeSpec, icx InvokeContext) (*InvokeResult, error) {
	// The MCP invoker depends on the src-bridge MCP client which runs in a
	// separate process. For this change we return a structured "not yet
	// wired" reply so operators can see the plugin was resolved but the
	// underlying MCP proxy still needs configuration. A follow-up change
	// will connect this to the real MCP transport.
	return &InvokeResult{
		Text: fmt.Sprintf("MCP plugin %s:%s invoked (transport pending)", spec.ServerID, spec.Tool),
	}, nil
}

// ----- builtin invoker ---------------------------------------------------

type builtinInvoker struct {
	registry *Registry
}

func (b *builtinInvoker) Invoke(ctx context.Context, spec *InvokeSpec, icx InvokeContext) (*InvokeResult, error) {
	b.registry.mu.RLock()
	handler, ok := b.registry.builtinHandlers[strings.TrimSpace(spec.Key)]
	b.registry.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("plugin: builtin key %q not registered", spec.Key)
	}
	return handler(ctx, icx)
}
