// Package worktree manages git worktrees for isolated agent execution.
package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrCapacityReached = errors.New("managed worktree capacity reached")
	ErrPathConflict    = errors.New("managed worktree path conflict")
	ErrStaleState      = errors.New("managed worktree stale state")
)

// Allocation describes the managed workspace selected for a task.
type Allocation struct {
	ProjectSlug string
	TaskID      string
	Branch      string
	Path        string
	Reused      bool
}

// Inspection describes the current managed workspace state for a task.
type Inspection struct {
	ProjectSlug string
	TaskID      string
	Branch      string
	Path        string
	Exists      bool
	Managed     bool
	Stale       bool
	Reason      string
}

// Inventory summarizes managed worktree state for a project.
type Inventory struct {
	ProjectSlug string
	Total       int
	Managed     int
	Stale       int
	Entries     []Inspection
}

type worktreeEntry struct {
	Path   string
	Branch string
}

// Manager creates and removes git worktrees.
type Manager struct {
	basePath     string // base directory for worktrees
	repoBasePath string // base directory for cloned repositories
	maxActive    int
}

// NewManager creates a worktree manager.
func NewManager(basePath, repoBasePath string, maxActive ...int) *Manager {
	manager := &Manager{basePath: basePath, repoBasePath: repoBasePath}
	if len(maxActive) > 0 {
		manager.maxActive = maxActive[0]
	}
	return manager
}

// Branch returns the canonical managed branch for a task.
func (m *Manager) Branch(taskID string) string {
	return "agent/" + taskID
}

// Prepare returns the canonical managed workspace for a task.
func (m *Manager) Prepare(ctx context.Context, projectSlug, taskID string) (*Allocation, error) {
	inspection, err := m.Inspect(ctx, projectSlug, taskID)
	if err != nil {
		return nil, err
	}
	if inspection.Managed && !inspection.Stale {
		return &Allocation{
			ProjectSlug: projectSlug,
			TaskID:      taskID,
			Branch:      inspection.Branch,
			Path:        inspection.Path,
			Reused:      true,
		}, nil
	}
	if inspection.Stale {
		return nil, fmt.Errorf("%w: %s", ErrStaleState, inspection.Reason)
	}
	if inspection.Exists {
		return nil, fmt.Errorf("%w: %s", ErrPathConflict, inspection.Reason)
	}

	entries, err := m.listEntries(ctx, projectSlug)
	if err != nil {
		return nil, err
	}
	if m.maxActive > 0 && m.countManagedEntries(projectSlug, entries) >= m.maxActive {
		return nil, fmt.Errorf("%w: max_active=%d", ErrCapacityReached, m.maxActive)
	}

	worktreeBase := filepath.Join(m.basePath, projectSlug)
	if err := os.MkdirAll(worktreeBase, 0o755); err != nil {
		return nil, fmt.Errorf("create worktree base dir: %w", err)
	}

	branch := m.Branch(taskID)
	worktreePath := m.Path(projectSlug, taskID)
	repoPath := m.repoPath(projectSlug)

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branch, worktreePath)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return &Allocation{
		ProjectSlug: projectSlug,
		TaskID:      taskID,
		Branch:      branch,
		Path:        worktreePath,
	}, nil
}

// Release removes the managed workspace and canonical branch for a task.
func (m *Manager) Release(ctx context.Context, projectSlug, taskID string) error {
	inspection, err := m.Inspect(ctx, projectSlug, taskID)
	if err != nil {
		return err
	}

	entries, err := m.listEntries(ctx, projectSlug)
	if err != nil {
		return err
	}
	pathEntry := m.findEntryByPath(entries, inspection.Path)

	if pathEntry != nil && inspection.Exists {
		if err := m.gitCommand(ctx, projectSlug, "worktree", "remove", "--force", inspection.Path); err != nil {
			return err
		}
	} else if inspection.Exists {
		if err := os.RemoveAll(inspection.Path); err != nil {
			return fmt.Errorf("remove worktree dir: %w", err)
		}
	}

	if err := m.prune(ctx, projectSlug); err != nil {
		return err
	}
	if err := os.RemoveAll(inspection.Path); err != nil {
		return fmt.Errorf("remove worktree dir: %w", err)
	}

	if exists, err := m.branchExists(ctx, projectSlug, inspection.Branch); err != nil {
		return err
	} else if exists {
		if err := m.gitCommand(ctx, projectSlug, "branch", "-D", inspection.Branch); err != nil {
			return err
		}
	}

	return nil
}

// GarbageCollect repairs stale state for a managed workspace.
func (m *Manager) GarbageCollect(ctx context.Context, projectSlug, taskID string) error {
	inspection, err := m.Inspect(ctx, projectSlug, taskID)
	if err != nil {
		return err
	}
	if !inspection.Stale {
		return nil
	}
	return m.Release(ctx, projectSlug, taskID)
}

// GarbageCollectAll repairs stale managed worktree state discovered for a project.
func (m *Manager) GarbageCollectAll(ctx context.Context, projectSlug string) ([]Inspection, error) {
	taskIDs, err := m.managedTaskIDs(ctx, projectSlug)
	if err != nil {
		return nil, err
	}

	cleaned := make([]Inspection, 0)
	for _, taskID := range taskIDs {
		inspection, err := m.Inspect(ctx, projectSlug, taskID)
		if err != nil {
			return nil, err
		}
		if !inspection.Stale {
			continue
		}
		if err := m.Release(ctx, projectSlug, taskID); err != nil {
			return nil, err
		}
		cleaned = append(cleaned, *inspection)
	}

	return cleaned, nil
}

// Inventory reports project-wide managed worktree visibility for operator-safe inspection.
func (m *Manager) Inventory(ctx context.Context, projectSlug string) (*Inventory, error) {
	taskIDs, err := m.managedTaskIDs(ctx, projectSlug)
	if err != nil {
		return nil, err
	}
	sort.Strings(taskIDs)

	inventory := &Inventory{
		ProjectSlug: projectSlug,
		Entries:     make([]Inspection, 0, len(taskIDs)),
	}

	for _, taskID := range taskIDs {
		inspection, err := m.Inspect(ctx, projectSlug, taskID)
		if err != nil {
			return nil, err
		}
		inventory.Entries = append(inventory.Entries, *inspection)
		if inspection.Managed && !inspection.Stale {
			inventory.Managed++
		}
		if inspection.Stale {
			inventory.Stale++
		}
	}
	inventory.Total = len(inventory.Entries)

	return inventory, nil
}

// Inspect reports the current state of the managed workspace for a task.
func (m *Manager) Inspect(ctx context.Context, projectSlug, taskID string) (*Inspection, error) {
	entries, err := m.listEntries(ctx, projectSlug)
	if err != nil {
		return nil, err
	}

	inspection := &Inspection{
		ProjectSlug: projectSlug,
		TaskID:      taskID,
		Branch:      m.Branch(taskID),
		Path:        m.Path(projectSlug, taskID),
	}

	if info, err := os.Stat(inspection.Path); err == nil && info.IsDir() {
		inspection.Exists = true
	}

	pathEntry := m.findEntryByPath(entries, inspection.Path)
	branchEntry := m.findEntryByBranch(entries, inspection.Branch)
	branchExists, err := m.branchExists(ctx, projectSlug, inspection.Branch)
	if err != nil {
		return nil, err
	}

	switch {
	case pathEntry != nil && pathEntry.Branch == m.branchRef(inspection.Branch) && inspection.Exists:
		inspection.Managed = true
	case pathEntry != nil && pathEntry.Branch == m.branchRef(inspection.Branch) && !inspection.Exists:
		inspection.Stale = true
		inspection.Reason = "managed git metadata exists but workspace directory is missing"
	case pathEntry != nil && pathEntry.Branch != m.branchRef(inspection.Branch):
		inspection.Stale = true
		inspection.Reason = "canonical workspace path is attached to a different branch"
	case branchEntry != nil && normalizePath(branchEntry.Path) != normalizePath(inspection.Path):
		inspection.Stale = true
		inspection.Reason = "managed branch is attached to a different workspace path"
	case branchExists && !inspection.Exists:
		inspection.Stale = true
		inspection.Reason = "managed branch exists without a canonical workspace"
	case branchExists && inspection.Exists:
		inspection.Stale = true
		inspection.Reason = "workspace and branch exist without matching git worktree metadata"
	case inspection.Exists:
		inspection.Reason = "canonical workspace path exists but is not a managed git worktree"
	}

	return inspection, nil
}

// Create creates a new git worktree for a task on a new branch.
func (m *Manager) Create(ctx context.Context, projectSlug, taskID, branchName string) (string, error) {
	if branchName != "" && branchName != m.Branch(taskID) {
		return "", fmt.Errorf("%w: expected %s, got %s", ErrPathConflict, m.Branch(taskID), branchName)
	}
	allocation, err := m.Prepare(ctx, projectSlug, taskID)
	if err != nil {
		return "", err
	}
	return allocation.Path, nil
}

// Remove removes a git worktree.
func (m *Manager) Remove(ctx context.Context, projectSlug, taskID string) error {
	return m.Release(ctx, projectSlug, taskID)
}

// List returns all current worktrees.
func (m *Manager) List(ctx context.Context, projectSlug string) ([]string, error) {
	entries, err := m.listEntries(ctx, projectSlug)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, normalizePath(entry.Path))
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

func (m *Manager) repoPath(projectSlug string) string {
	return filepath.Join(m.repoBasePath, projectSlug)
}

func (m *Manager) branchRef(branch string) string {
	return "refs/heads/" + branch
}

func (m *Manager) countManagedEntries(projectSlug string, entries []worktreeEntry) int {
	count := 0
	projectBase := normalizePath(filepath.Join(m.basePath, projectSlug))
	for _, entry := range entries {
		if strings.HasPrefix(normalizePath(entry.Path), projectBase+"/") && strings.HasPrefix(entry.Branch, "refs/heads/agent/") {
			count++
		}
	}
	return count
}

func (m *Manager) findEntryByPath(entries []worktreeEntry, path string) *worktreeEntry {
	normalized := normalizePath(path)
	for _, entry := range entries {
		if normalizePath(entry.Path) == normalized {
			cloned := entry
			return &cloned
		}
	}
	return nil
}

func (m *Manager) findEntryByBranch(entries []worktreeEntry, branch string) *worktreeEntry {
	branchRef := m.branchRef(branch)
	for _, entry := range entries {
		if entry.Branch == branchRef {
			cloned := entry
			return &cloned
		}
	}
	return nil
}

func (m *Manager) listEntries(ctx context.Context, projectSlug string) ([]worktreeEntry, error) {
	output, err := m.gitOutput(ctx, projectSlug, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var entries []worktreeEntry
	var current *worktreeEntry
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				entries = append(entries, *current)
				current = nil
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current != nil {
				entries = append(entries, *current)
			}
			current = &worktreeEntry{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch ") && current != nil:
			current.Branch = strings.TrimPrefix(line, "branch ")
		}
	}
	if current != nil {
		entries = append(entries, *current)
	}

	return entries, nil
}

func (m *Manager) managedTaskIDs(ctx context.Context, projectSlug string) ([]string, error) {
	seen := map[string]struct{}{}
	taskIDs := make([]string, 0)

	entries, err := m.listEntries(ctx, projectSlug)
	if err != nil {
		return nil, err
	}
	projectBase := normalizePath(filepath.Join(m.basePath, projectSlug))
	for _, entry := range entries {
		entryPath := normalizePath(entry.Path)
		if !strings.HasPrefix(entryPath, projectBase+"/") || !strings.HasPrefix(entry.Branch, "refs/heads/agent/") {
			continue
		}
		taskID := strings.TrimPrefix(filepath.Base(entryPath), ".")
		if _, ok := seen[taskID]; !ok {
			seen[taskID] = struct{}{}
			taskIDs = append(taskIDs, taskID)
		}
	}

	worktreeBase := filepath.Join(m.basePath, projectSlug)
	if dirEntries, err := os.ReadDir(worktreeBase); err == nil {
		for _, entry := range dirEntries {
			if !entry.IsDir() {
				continue
			}
			taskID := entry.Name()
			if _, ok := seen[taskID]; !ok {
				seen[taskID] = struct{}{}
				taskIDs = append(taskIDs, taskID)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read worktree base dir: %w", err)
	}

	branches, err := m.listManagedBranches(ctx, projectSlug)
	if err != nil {
		return nil, err
	}
	for _, branch := range branches {
		taskID := strings.TrimPrefix(branch, "agent/")
		if _, ok := seen[taskID]; !ok {
			seen[taskID] = struct{}{}
			taskIDs = append(taskIDs, taskID)
		}
	}

	return taskIDs, nil
}

func (m *Manager) listManagedBranches(ctx context.Context, projectSlug string) ([]string, error) {
	output, err := m.gitOutput(ctx, projectSlug, "for-each-ref", "--format=%(refname:short)", "refs/heads/agent")
	if err != nil {
		return nil, fmt.Errorf("list managed branches: %w", err)
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		branches = append(branches, line)
	}
	return branches, nil
}

func (m *Manager) prune(ctx context.Context, projectSlug string) error {
	return m.gitCommand(ctx, projectSlug, "worktree", "prune", "--expire", "now")
}

func (m *Manager) branchExists(ctx context.Context, projectSlug, branch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", m.branchRef(branch))
	cmd.Dir = m.repoPath(projectSlug)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, fmt.Errorf("git show-ref: %w", err)
}

func (m *Manager) gitOutput(ctx context.Context, projectSlug string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.repoPath(projectSlug)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	return string(output), nil
}

func (m *Manager) gitCommand(ctx context.Context, projectSlug string, args ...string) error {
	_, err := m.gitOutput(ctx, projectSlug, args...)
	return err
}

func normalizePath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}
