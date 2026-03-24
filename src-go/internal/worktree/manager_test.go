package worktree_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/react-go-quick-starter/server/internal/worktree"
)

func TestManager_PrepareCreatesAndReusesCanonicalWorkspace(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	repoBasePath := filepath.Join(baseDir, "repos")
	worktreeBasePath := filepath.Join(baseDir, "worktrees")
	projectSlug := "demo"
	taskID := "task-1"

	repoPath := filepath.Join(repoBasePath, projectSlug)
	initGitRepo(t, repoPath)

	manager := worktree.NewManager(worktreeBasePath, repoBasePath, 5)

	if got := manager.Path(projectSlug, taskID); got != filepath.Join(worktreeBasePath, projectSlug, taskID) {
		t.Fatalf("Path() = %q, want %q", got, filepath.Join(worktreeBasePath, projectSlug, taskID))
	}
	if got := manager.Branch(taskID); got != "agent/"+taskID {
		t.Fatalf("Branch() = %q, want %q", got, "agent/"+taskID)
	}

	allocation, err := manager.Prepare(ctx, projectSlug, taskID)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if allocation.Path != manager.Path(projectSlug, taskID) {
		t.Fatalf("Prepare() path = %q, want %q", allocation.Path, manager.Path(projectSlug, taskID))
	}
	if allocation.Branch != manager.Branch(taskID) {
		t.Fatalf("Prepare() branch = %q, want %q", allocation.Branch, manager.Branch(taskID))
	}
	if allocation.Reused {
		t.Fatal("Prepare() marked first allocation as reused")
	}

	inspection, err := manager.Inspect(ctx, projectSlug, taskID)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if !inspection.Managed || inspection.Stale {
		t.Fatalf("Inspect() = %+v, want managed healthy workspace", inspection)
	}

	reused, err := manager.Prepare(ctx, projectSlug, taskID)
	if err != nil {
		t.Fatalf("second Prepare() error = %v", err)
	}
	if !reused.Reused {
		t.Fatal("second Prepare() did not mark allocation as reused")
	}
	if reused.Path != allocation.Path || reused.Branch != allocation.Branch {
		t.Fatalf("reused allocation = %+v, want same path/branch as %+v", reused, allocation)
	}
}

func TestManager_PrepareRejectsCapacityAndConflicts(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	repoBasePath := filepath.Join(baseDir, "repos")
	worktreeBasePath := filepath.Join(baseDir, "worktrees")
	projectSlug := "demo"

	repoPath := filepath.Join(repoBasePath, projectSlug)
	initGitRepo(t, repoPath)

	manager := worktree.NewManager(worktreeBasePath, repoBasePath, 1)

	if _, err := manager.Prepare(ctx, projectSlug, "task-1"); err != nil {
		t.Fatalf("Prepare(task-1) error = %v", err)
	}
	if _, err := manager.Prepare(ctx, projectSlug, "task-2"); !errors.Is(err, worktree.ErrCapacityReached) {
		t.Fatalf("Prepare(task-2) error = %v, want ErrCapacityReached", err)
	}

	otherBaseDir := t.TempDir()
	otherRepoBasePath := filepath.Join(otherBaseDir, "repos")
	otherWorktreeBasePath := filepath.Join(otherBaseDir, "worktrees")
	otherRepoPath := filepath.Join(otherRepoBasePath, projectSlug)
	initGitRepo(t, otherRepoPath)

	conflictManager := worktree.NewManager(otherWorktreeBasePath, otherRepoBasePath, 5)
	conflictPath := conflictManager.Path(projectSlug, "task-3")
	if err := os.MkdirAll(conflictPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", conflictPath, err)
	}
	if _, err := conflictManager.Prepare(ctx, projectSlug, "task-3"); !errors.Is(err, worktree.ErrPathConflict) {
		t.Fatalf("Prepare(task-3) error = %v, want ErrPathConflict", err)
	}
}

func TestManager_ReleaseRepairsMissingWorktreeAndGarbageCollectsStaleState(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	repoBasePath := filepath.Join(baseDir, "repos")
	worktreeBasePath := filepath.Join(baseDir, "worktrees")
	projectSlug := "demo"
	taskID := "task-1"

	repoPath := filepath.Join(repoBasePath, projectSlug)
	initGitRepo(t, repoPath)

	manager := worktree.NewManager(worktreeBasePath, repoBasePath, 5)

	allocation, err := manager.Prepare(ctx, projectSlug, taskID)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if branchExists(t, repoPath, allocation.Branch) != true {
		t.Fatalf("expected branch %q to exist after Prepare()", allocation.Branch)
	}

	if err := os.RemoveAll(allocation.Path); err != nil {
		t.Fatalf("RemoveAll(%q) error = %v", allocation.Path, err)
	}

	inspection, err := manager.Inspect(ctx, projectSlug, taskID)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if !inspection.Stale {
		t.Fatalf("Inspect() = %+v, want stale workspace", inspection)
	}

	if err := manager.Release(ctx, projectSlug, taskID); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if branchExists(t, repoPath, allocation.Branch) {
		t.Fatalf("expected branch %q to be removed by Release()", allocation.Branch)
	}

	if _, err := manager.Prepare(ctx, projectSlug, taskID); err != nil {
		t.Fatalf("Prepare() after Release() error = %v", err)
	}

	staleTaskID := "task-stale"
	staleBranch := manager.Branch(staleTaskID)
	runGit(t, repoPath, "branch", staleBranch, "main")

	staleInspection, err := manager.Inspect(ctx, projectSlug, staleTaskID)
	if err != nil {
		t.Fatalf("Inspect(stale) error = %v", err)
	}
	if !staleInspection.Stale {
		t.Fatalf("Inspect(stale) = %+v, want stale branch-only state", staleInspection)
	}

	if err := manager.GarbageCollect(ctx, projectSlug, staleTaskID); err != nil {
		t.Fatalf("GarbageCollect() error = %v", err)
	}
	if branchExists(t, repoPath, staleBranch) {
		t.Fatalf("expected branch %q to be removed by GarbageCollect()", staleBranch)
	}
}

func TestManager_GarbageCollectAllRepairsProjectWideStaleManagedState(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	repoBasePath := filepath.Join(baseDir, "repos")
	worktreeBasePath := filepath.Join(baseDir, "worktrees")
	projectSlug := "demo"

	repoPath := filepath.Join(repoBasePath, projectSlug)
	initGitRepo(t, repoPath)

	manager := worktree.NewManager(worktreeBasePath, repoBasePath, 5)

	healthyTaskID := "task-healthy"
	if _, err := manager.Prepare(ctx, projectSlug, healthyTaskID); err != nil {
		t.Fatalf("Prepare(healthy) error = %v", err)
	}

	stalePathTaskID := "task-stale-path"
	stalePathAllocation, err := manager.Prepare(ctx, projectSlug, stalePathTaskID)
	if err != nil {
		t.Fatalf("Prepare(stale-path) error = %v", err)
	}
	if err := os.RemoveAll(stalePathAllocation.Path); err != nil {
		t.Fatalf("RemoveAll(%q) error = %v", stalePathAllocation.Path, err)
	}

	staleBranchTaskID := "task-stale-branch"
	runGit(t, repoPath, "branch", manager.Branch(staleBranchTaskID), "main")

	cleaned, err := manager.GarbageCollectAll(ctx, projectSlug)
	if err != nil {
		t.Fatalf("GarbageCollectAll() error = %v", err)
	}
	if len(cleaned) != 2 {
		t.Fatalf("GarbageCollectAll() cleaned %d items, want 2 (%+v)", len(cleaned), cleaned)
	}
	if branchExists(t, repoPath, manager.Branch(staleBranchTaskID)) {
		t.Fatalf("expected branch %q to be removed by GarbageCollectAll()", manager.Branch(staleBranchTaskID))
	}

	healthyInspection, err := manager.Inspect(ctx, projectSlug, healthyTaskID)
	if err != nil {
		t.Fatalf("Inspect(healthy) error = %v", err)
	}
	if !healthyInspection.Managed || healthyInspection.Stale {
		t.Fatalf("Inspect(healthy) = %+v, want healthy managed workspace", healthyInspection)
	}

	stalePathInspection, err := manager.Inspect(ctx, projectSlug, stalePathTaskID)
	if err != nil {
		t.Fatalf("Inspect(stale-path) error = %v", err)
	}
	if stalePathInspection.Managed || stalePathInspection.Stale || stalePathInspection.Exists {
		t.Fatalf("Inspect(stale-path) after GC = %+v, want cleared state", stalePathInspection)
	}
}

func TestManager_InventorySummarizesManagedAndStaleState(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	repoBasePath := filepath.Join(baseDir, "repos")
	worktreeBasePath := filepath.Join(baseDir, "worktrees")
	projectSlug := "demo"

	repoPath := filepath.Join(repoBasePath, projectSlug)
	initGitRepo(t, repoPath)

	manager := worktree.NewManager(worktreeBasePath, repoBasePath, 5)

	healthyTaskID := "task-healthy"
	if _, err := manager.Prepare(ctx, projectSlug, healthyTaskID); err != nil {
		t.Fatalf("Prepare(healthy) error = %v", err)
	}

	staleTaskID := "task-stale"
	staleAllocation, err := manager.Prepare(ctx, projectSlug, staleTaskID)
	if err != nil {
		t.Fatalf("Prepare(stale) error = %v", err)
	}
	if err := os.RemoveAll(staleAllocation.Path); err != nil {
		t.Fatalf("RemoveAll(%q) error = %v", staleAllocation.Path, err)
	}

	branchOnlyTaskID := "task-branch-only"
	runGit(t, repoPath, "branch", manager.Branch(branchOnlyTaskID), "main")

	inventory, err := manager.Inventory(ctx, projectSlug)
	if err != nil {
		t.Fatalf("Inventory() error = %v", err)
	}

	if inventory.ProjectSlug != projectSlug {
		t.Fatalf("Inventory().ProjectSlug = %q, want %q", inventory.ProjectSlug, projectSlug)
	}
	if inventory.Total != 3 {
		t.Fatalf("Inventory().Total = %d, want 3", inventory.Total)
	}
	if inventory.Managed != 1 {
		t.Fatalf("Inventory().Managed = %d, want 1", inventory.Managed)
	}
	if inventory.Stale != 2 {
		t.Fatalf("Inventory().Stale = %d, want 2", inventory.Stale)
	}
	if len(inventory.Entries) != 3 {
		t.Fatalf("len(Inventory().Entries) = %d, want 3", len(inventory.Entries))
	}

	if inventory.Entries[0].TaskID != branchOnlyTaskID || !inventory.Entries[0].Stale {
		t.Fatalf("Inventory().Entries[0] = %+v, want stale branch-only task first", inventory.Entries[0])
	}
	if inventory.Entries[1].TaskID != healthyTaskID || !inventory.Entries[1].Managed || inventory.Entries[1].Stale {
		t.Fatalf("Inventory().Entries[1] = %+v, want healthy managed task", inventory.Entries[1])
	}
	if inventory.Entries[2].TaskID != staleTaskID || !inventory.Entries[2].Stale {
		t.Fatalf("Inventory().Entries[2] = %+v, want stale missing-path task", inventory.Entries[2])
	}
}

func initGitRepo(t *testing.T, repoPath string) {
	t.Helper()

	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", repoPath, err)
	}

	runGit(t, repoPath, "init", "-b", "main")
	runGit(t, repoPath, "config", "user.name", "AgentForge Tests")
	runGit(t, repoPath, "config", "user.email", "tests@example.com")

	filePath := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(filePath, []byte("seed"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", filePath, err)
	}

	runGit(t, repoPath, "add", "README.md")
	runGit(t, repoPath, "commit", "-m", "init")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v error = %v\n%s", args, err, string(output))
	}
}

func branchExists(t *testing.T, dir, branch string) bool {
	t.Helper()

	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = dir
	err := cmd.Run()
	return err == nil
}
