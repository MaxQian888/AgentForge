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
