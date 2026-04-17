package plugin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, dir, id, body string) string {
	t.Helper()
	pluginDir := filepath.Join(dir, id)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(pluginDir, "plugin.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestRegistryLoadsValidManifests(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "jira", `id: "@acme/jira"
version: "1.0.0"
name: "Jira"
commands:
  - slash: "/jira"
    subcommands:
      - name: "create"
        invoke:
          kind: http
          url: http://example.test/create
`)

	r := NewRegistry(dir)
	if err := r.ReloadAll(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	plugins := r.Plugins()
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Manifest.ID != "@acme/jira" {
		t.Fatalf("unexpected id: %s", plugins[0].Manifest.ID)
	}
}

func TestRegistryRejectsUnknownInvokeKind(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "bad", `id: "bad"
commands:
  - slash: "/bad"
    invoke:
      kind: "rocket"
`)

	r := NewRegistry(dir)
	_ = r.ReloadAll()
	if len(r.Plugins()) != 0 {
		t.Fatal("expected invalid manifest to be skipped")
	}
}

func TestDispatchHTTPInvoke(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"ok"}`))
	}))
	defer server.Close()

	writeManifest(t, dir, "echo", `id: "echo"
commands:
  - slash: "/echo"
    invoke:
      kind: http
      url: `+server.URL+`
`)
	r := NewRegistry(dir)
	if err := r.ReloadAll(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	res, err := r.Dispatch(context.Background(), InvokeContext{Command: "/echo"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if res.Text != "ok" {
		t.Fatalf("unexpected text: %q", res.Text)
	}
}

func TestDispatchBuiltin(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "task", `id: "task"
commands:
  - slash: "/task"
    invoke:
      kind: builtin
      key: builtin.task.list
`)
	r := NewRegistry(dir)
	called := false
	r.RegisterBuiltin("builtin.task.list", func(ctx context.Context, icx InvokeContext) (*InvokeResult, error) {
		called = true
		return &InvokeResult{Text: "listed"}, nil
	})
	if err := r.ReloadAll(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	res, err := r.Dispatch(context.Background(), InvokeContext{Command: "/task"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !called {
		t.Fatal("builtin not called")
	}
	if res.Text != "listed" {
		t.Fatalf("unexpected text: %q", res.Text)
	}
}

func TestDispatchTenantAllowlist(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "scoped", `id: "scoped"
tenants: [acme]
commands:
  - slash: "/scoped"
    invoke:
      kind: builtin
      key: scoped.cmd
`)
	r := NewRegistry(dir)
	r.RegisterBuiltin("scoped.cmd", func(ctx context.Context, icx InvokeContext) (*InvokeResult, error) {
		return &InvokeResult{Text: "ok"}, nil
	})
	if err := r.ReloadAll(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	// Wrong tenant → not found.
	if _, err := r.Dispatch(context.Background(), InvokeContext{Command: "/scoped", TenantID: "beta"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for beta, got %v", err)
	}
	// Right tenant → success.
	if _, err := r.Dispatch(context.Background(), InvokeContext{Command: "/scoped", TenantID: "acme"}); err != nil {
		t.Fatalf("expected acme dispatch success, got %v", err)
	}
}

func TestDispatchSubcommand(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "jira", `id: "jira"
commands:
  - slash: "/jira"
    subcommands:
      - name: create
        invoke:
          kind: builtin
          key: jira.create
      - name: link
        invoke:
          kind: builtin
          key: jira.link
`)
	r := NewRegistry(dir)
	created := ""
	r.RegisterBuiltin("jira.create", func(ctx context.Context, icx InvokeContext) (*InvokeResult, error) {
		created = "create-" + icx.Args
		return &InvokeResult{Text: created}, nil
	})
	r.RegisterBuiltin("jira.link", func(ctx context.Context, icx InvokeContext) (*InvokeResult, error) {
		return &InvokeResult{Text: "linked"}, nil
	})
	_ = r.ReloadAll()
	res, err := r.Dispatch(context.Background(), InvokeContext{Command: "/jira", Subcommand: "create", Args: "ISSUE-1"})
	if err != nil || res.Text != "create-ISSUE-1" {
		t.Fatalf("create dispatch: %v %v", res, err)
	}
}
