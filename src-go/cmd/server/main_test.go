package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agentforge/server/internal/worktree"
)

type fakeStartupSweepManager struct {
	inventoryByProject map[string][]*worktree.Inventory
	cleanedByProject   map[string][]worktree.Inspection
	inventoryCalls     map[string]int
	gcCalls            []string
	gcErrByProject     map[string]error
}

func (m *fakeStartupSweepManager) Inventory(_ context.Context, projectSlug string) (*worktree.Inventory, error) {
	call := m.inventoryCalls[projectSlug]
	m.inventoryCalls[projectSlug] = call + 1
	series := m.inventoryByProject[projectSlug]
	if len(series) == 0 {
		return &worktree.Inventory{ProjectSlug: projectSlug}, nil
	}
	if call >= len(series) {
		last := *series[len(series)-1]
		return &last, nil
	}
	cloned := *series[call]
	return &cloned, nil
}

func (m *fakeStartupSweepManager) GarbageCollectAll(_ context.Context, projectSlug string) ([]worktree.Inspection, error) {
	m.gcCalls = append(m.gcCalls, projectSlug)
	if err := m.gcErrByProject[projectSlug]; err != nil {
		return nil, err
	}
	cleaned := m.cleanedByProject[projectSlug]
	out := make([]worktree.Inspection, len(cleaned))
	copy(out, cleaned)
	return out, nil
}

func TestCollectStartupWorktreeProjectsSortsDirectoriesOnly(t *testing.T) {
	baseDir := t.TempDir()

	for _, name := range []string{"zeta", "alpha", "beta"} {
		if err := os.MkdirAll(filepath.Join(baseDir, name), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(baseDir, "README.txt"), []byte("ignore"), 0o600); err != nil {
		t.Fatalf("WriteFile(file) error = %v", err)
	}

	got, err := collectStartupWorktreeProjects(baseDir)
	if err != nil {
		t.Fatalf("collectStartupWorktreeProjects() error = %v", err)
	}

	want := []string{"alpha", "beta", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectStartupWorktreeProjects() = %v, want %v", got, want)
	}
}

func TestCollectStartupWorktreeProjectsIgnoresMissingBasePath(t *testing.T) {
	got, err := collectStartupWorktreeProjects(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("collectStartupWorktreeProjects() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("collectStartupWorktreeProjects() = %v, want empty result for missing base path", got)
	}
}

func TestSummarizeStartupWorktreeProjectTracksBeforeAndAfterState(t *testing.T) {
	manager := &fakeStartupSweepManager{
		inventoryByProject: map[string][]*worktree.Inventory{
			"demo": {
				{ProjectSlug: "demo", Total: 3, Managed: 1, Stale: 2},
				{ProjectSlug: "demo", Total: 1, Managed: 1, Stale: 0},
			},
		},
		cleanedByProject: map[string][]worktree.Inspection{
			"demo": {
				{ProjectSlug: "demo", TaskID: "task-stale-1", Stale: true},
				{ProjectSlug: "demo", TaskID: "task-stale-2", Stale: true},
			},
		},
		inventoryCalls: map[string]int{},
		gcErrByProject: map[string]error{},
	}

	report, err := summarizeStartupWorktreeProject(context.Background(), manager, "demo")
	if err != nil {
		t.Fatalf("summarizeStartupWorktreeProject() error = %v", err)
	}

	if report.ProjectSlug != "demo" {
		t.Fatalf("report.ProjectSlug = %q, want demo", report.ProjectSlug)
	}
	if report.StaleBefore != 2 || report.ManagedBefore != 1 || report.TotalBefore != 3 {
		t.Fatalf("report before counts = %+v, want stale=2 managed=1 total=3", report)
	}
	if report.Cleaned != 2 {
		t.Fatalf("report.Cleaned = %d, want 2", report.Cleaned)
	}
	if report.StaleAfter != 0 || report.ManagedAfter != 1 || report.TotalAfter != 1 {
		t.Fatalf("report after counts = %+v, want stale=0 managed=1 total=1", report)
	}
	if !reflect.DeepEqual(manager.gcCalls, []string{"demo"}) {
		t.Fatalf("GarbageCollectAll() calls = %v, want [demo]", manager.gcCalls)
	}
}

func TestSummarizeStartupWorktreeProjectReturnsCleanupError(t *testing.T) {
	manager := &fakeStartupSweepManager{
		inventoryByProject: map[string][]*worktree.Inventory{
			"demo": {{ProjectSlug: "demo", Total: 1, Managed: 0, Stale: 1}},
		},
		inventoryCalls: map[string]int{},
		gcErrByProject: map[string]error{"demo": errors.New("boom")},
	}

	_, err := summarizeStartupWorktreeProject(context.Background(), manager, "demo")
	if err == nil {
		t.Fatal("summarizeStartupWorktreeProject() error = nil, want cleanup error")
	}
}
