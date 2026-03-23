package worktree_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/react-go-quick-starter/server/internal/worktree"
)

func TestManagerCreateListRemoveAndPath(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	repoBasePath := filepath.Join(baseDir, "repos")
	worktreeBasePath := filepath.Join(baseDir, "worktrees")
	projectSlug := "demo"
	taskID := "task-1"
	branchName := "feature/task-1"

	repoPath := filepath.Join(repoBasePath, projectSlug)
	initGitRepo(t, repoPath)

	manager := worktree.NewManager(worktreeBasePath, repoBasePath)

	if got := manager.Path(projectSlug, taskID); got != filepath.Join(worktreeBasePath, projectSlug, taskID) {
		t.Fatalf("Path() = %q, want %q", got, filepath.Join(worktreeBasePath, projectSlug, taskID))
	}
	if manager.Exists(projectSlug, taskID) {
		t.Fatal("Exists() = true before Create(), want false")
	}

	worktreePath, err := manager.Create(ctx, projectSlug, taskID, branchName)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if worktreePath != manager.Path(projectSlug, taskID) {
		t.Fatalf("Create() path = %q, want %q", worktreePath, manager.Path(projectSlug, taskID))
	}
	if !manager.Exists(projectSlug, taskID) {
		t.Fatal("Exists() = false after Create(), want true")
	}

	paths, err := manager.List(ctx, projectSlug)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	slices.Sort(paths)
	normalizedWorktreePath := filepath.ToSlash(worktreePath)
	if !slices.Contains(paths, normalizedWorktreePath) {
		t.Fatalf("List() = %v, want to contain %q", paths, worktreePath)
	}

	if err := manager.Remove(ctx, projectSlug, taskID); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if manager.Exists(projectSlug, taskID) {
		t.Fatal("Exists() = true after Remove(), want false")
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
