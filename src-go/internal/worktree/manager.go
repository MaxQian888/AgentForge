// Package worktree manages git worktrees for isolated agent execution.
package worktree

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Manager creates and removes git worktrees.
type Manager struct {
	basePath     string // base directory for worktrees
	repoBasePath string // base directory for cloned repositories
}

// NewManager creates a worktree manager.
func NewManager(basePath, repoBasePath string) *Manager {
	return &Manager{basePath: basePath, repoBasePath: repoBasePath}
}

// Create creates a new git worktree for a task on a new branch.
func (m *Manager) Create(ctx context.Context, projectSlug, taskID, branchName string) (string, error) {
	worktreeBase := filepath.Join(m.basePath, projectSlug)
	worktreePath := filepath.Join(worktreeBase, taskID)
	repoPath := filepath.Join(m.repoBasePath, projectSlug)

	// Ensure base directory exists.
	if err := os.MkdirAll(worktreeBase, 0o755); err != nil {
		return "", fmt.Errorf("create worktree base dir: %w", err)
	}

	// Create worktree with new branch.
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branchName, worktreePath)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return worktreePath, nil
}

// Remove removes a git worktree.
func (m *Manager) Remove(ctx context.Context, projectSlug, taskID string) error {
	worktreePath := filepath.Join(m.basePath, projectSlug, taskID)
	repoPath := filepath.Join(m.repoBasePath, projectSlug)

	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// List returns all current worktrees.
func (m *Manager) List(ctx context.Context, projectSlug string) ([]string, error) {
	repoPath := filepath.Join(m.repoBasePath, projectSlug)
	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var paths []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, strings.TrimPrefix(line, "worktree "))
		}
	}
	return paths, nil
}

// Exists checks if a worktree for a task exists.
func (m *Manager) Exists(projectSlug, taskID string) bool {
	worktreePath := filepath.Join(m.basePath, projectSlug, taskID)
	info, err := os.Stat(worktreePath)
	return err == nil && info.IsDir()
}

// Path returns the worktree path for a task.
func (m *Manager) Path(projectSlug, taskID string) string {
	return filepath.Join(m.basePath, projectSlug, taskID)
}
