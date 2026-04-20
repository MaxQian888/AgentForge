package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

var (
	ErrTaskNotFound             = errors.New("task not found")
	ErrTaskAlreadyDecomposed    = errors.New("task already has child tasks")
	ErrInvalidTaskDecomposition = errors.New("invalid task decomposition")
	ErrCyclicDependency         = errors.New("subtask dependencies contain a cycle")
	ErrOverlappingWriteScopes   = errors.New("subtasks have overlapping write scopes")
)

type TaskDecompositionRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	HasChildren(ctx context.Context, parentID uuid.UUID) (bool, error)
	CreateChildren(ctx context.Context, inputs []model.TaskChildInput) ([]*model.Task, error)
}

// DecompositionContext carries code-level context for the bridge to use during decomposition.
type DecompositionContext struct {
	RelevantFiles      []string `json:"relevantFiles,omitempty"`
	FunctionSignatures []string `json:"functionSignatures,omitempty"`
	TestCoverage       []string `json:"testCoverage,omitempty"`
}

// FewShotExample represents a past successful decomposition used as a few-shot prompt.
type FewShotExample struct {
	ParentTitle   string   `json:"parentTitle"`
	SubtaskTitles []string `json:"subtaskTitles"`
}

type BridgeDecomposeRequest struct {
	TaskID         string                `json:"task_id"`
	Title          string                `json:"title"`
	Description    string                `json:"description"`
	Priority       string                `json:"priority"`
	Provider       string                `json:"provider,omitempty"`
	Model          string                `json:"model,omitempty"`
	CodeContext    *DecompositionContext `json:"codeContext,omitempty"`
	FewShotHistory []FewShotExample      `json:"fewShotHistory,omitempty"`
	WaveMode       bool                  `json:"waveMode,omitempty"`
}

type BridgeDecomposeSubtask struct {
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Priority      string   `json:"priority"`
	ExecutionMode string   `json:"executionMode"`
	Dependencies  []int    `json:"dependencies,omitempty"`
	WriteScope    []string `json:"writeScope,omitempty"`
	RoleHint      string   `json:"roleHint,omitempty"`
	Confidence    float64  `json:"confidence,omitempty"`
}

type BridgeDecomposeResponse struct {
	Summary    string                   `json:"summary"`
	Subtasks   []BridgeDecomposeSubtask `json:"subtasks"`
	Confidence float64                  `json:"confidence,omitempty"`
}

type TaskDecompositionBridge interface {
	DecomposeTask(ctx context.Context, req BridgeDecomposeRequest) (*BridgeDecomposeResponse, error)
}

type TaskDecompositionService struct {
	repo        TaskDecompositionRepository
	bridge      TaskDecompositionBridge
	memorySvc   *MemoryService
	projectRepo AgentProjectRepository
}

func NewTaskDecompositionService(repo TaskDecompositionRepository, bridge TaskDecompositionBridge) *TaskDecompositionService {
	return &TaskDecompositionService{repo: repo, bridge: bridge}
}

// SetMemoryService sets the optional memory service for few-shot history.
func (s *TaskDecompositionService) SetMemoryService(memorySvc *MemoryService) {
	s.memorySvc = memorySvc
}

// SetProjectRepository sets the optional project repository for code context.
func (s *TaskDecompositionService) SetProjectRepository(projectRepo AgentProjectRepository) {
	s.projectRepo = projectRepo
}

func (s *TaskDecompositionService) Decompose(ctx context.Context, taskID uuid.UUID) (*model.TaskDecompositionResponse, error) {
	parent, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return nil, ErrTaskNotFound
	}

	hasChildren, err := s.repo.HasChildren(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("check child tasks: %w", err)
	}
	if hasChildren {
		return nil, ErrTaskAlreadyDecomposed
	}

	if s.bridge == nil {
		return nil, fmt.Errorf("bridge client unavailable")
	}

	// Gather code context from project metadata.
	codeCtx := s.gatherCodeContext(ctx, parent.ProjectID)

	// Fetch few-shot examples from memory.
	fewShot := s.fetchFewShotHistory(ctx, parent.ProjectID, parent.Title)

	result, err := s.bridge.DecomposeTask(ctx, BridgeDecomposeRequest{
		TaskID:         parent.ID.String(),
		Title:          parent.Title,
		Description:    parent.Description,
		Priority:       normalizeTaskPriority(parent.Priority, "medium"),
		CodeContext:    codeCtx,
		FewShotHistory: fewShot,
		WaveMode:       true,
	})
	if err != nil {
		return nil, fmt.Errorf("bridge decompose task: %w", err)
	}
	if err := validateTaskDecomposition(result); err != nil {
		return nil, err
	}

	// Validate dependency DAG (cycle detection).
	if err := validateSubtaskDAG(result.Subtasks); err != nil {
		return nil, err
	}

	// Validate disjoint write scopes.
	if err := validateDisjointWriteScopes(result.Subtasks); err != nil {
		return nil, err
	}

	lowConfidence := result.Confidence > 0 && result.Confidence < 0.6

	inputs := make([]model.TaskChildInput, 0, len(result.Subtasks))
	for i, subtask := range result.Subtasks {
		labels := withExecutionModeLabel(parent.Labels, subtask.ExecutionMode)

		// Store dependency metadata as labels.
		for _, depIdx := range subtask.Dependencies {
			labels = append(labels, fmt.Sprintf("dep:%d", depIdx))
		}
		for _, scope := range subtask.WriteScope {
			labels = append(labels, fmt.Sprintf("scope:%s", scope))
		}
		if subtask.RoleHint != "" {
			labels = append(labels, fmt.Sprintf("role:%s", subtask.RoleHint))
		}

		// If overall response confidence is low, flag for human review.
		if lowConfidence {
			labels = append(labels, "needs-human-review")
		}

		_ = i // index available for future use

		inputs = append(inputs, model.TaskChildInput{
			ParentID:    parent.ID,
			ProjectID:   parent.ProjectID,
			SprintID:    parent.SprintID,
			ReporterID:  parent.ReporterID,
			Title:       strings.TrimSpace(subtask.Title),
			Description: strings.TrimSpace(subtask.Description),
			Priority:    normalizeTaskPriority(subtask.Priority, normalizeTaskPriority(parent.Priority, "medium")),
			Labels:      labels,
			BudgetUSD:   0,
		})
	}

	children, err := s.repo.CreateChildren(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("create child tasks: %w", err)
	}

	// Record successful decomposition to memory for future few-shot examples.
	s.recordDecompositionToMemory(ctx, parent, result)

	response := &model.TaskDecompositionResponse{
		ParentTask: parent.ToDTO(),
		Summary:    strings.TrimSpace(result.Summary),
		Subtasks:   make([]model.TaskDTO, 0, len(children)),
	}
	for _, child := range children {
		response.Subtasks = append(response.Subtasks, child.ToDTO())
	}
	return response, nil
}

// validateSubtaskDAG checks that subtask dependencies form a valid DAG using Kahn's algorithm.
func validateSubtaskDAG(subtasks []BridgeDecomposeSubtask) error {
	n := len(subtasks)
	if n == 0 {
		return nil
	}

	// Build in-degree counts and adjacency list.
	inDegree := make([]int, n)
	adj := make([][]int, n)
	for i := range adj {
		adj[i] = []int{}
	}

	for i, st := range subtasks {
		for _, dep := range st.Dependencies {
			if dep < 0 || dep >= n || dep == i {
				return fmt.Errorf("%w: subtask %d has invalid dependency index %d", ErrCyclicDependency, i, dep)
			}
			adj[dep] = append(adj[dep], i)
			inDegree[i]++
		}
	}

	// Kahn's algorithm: start with all nodes that have zero in-degree.
	queue := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++
		for _, neighbor := range adj[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if visited != n {
		return ErrCyclicDependency
	}
	return nil
}

// validateDisjointWriteScopes ensures no two subtasks write to overlapping paths.
// A path is considered overlapping if one is a prefix of the other (directory containment)
// or they are identical.
func validateDisjointWriteScopes(subtasks []BridgeDecomposeSubtask) error {
	// Collect all (subtaskIndex, path) pairs.
	type scopeEntry struct {
		subtaskIdx int
		path       string
	}
	var entries []scopeEntry
	for i, st := range subtasks {
		for _, p := range st.WriteScope {
			normalized := strings.TrimSuffix(strings.TrimSpace(p), "/")
			if normalized != "" {
				entries = append(entries, scopeEntry{subtaskIdx: i, path: normalized})
			}
		}
	}

	// Check all pairs for overlap.
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].subtaskIdx == entries[j].subtaskIdx {
				continue
			}
			if pathsOverlap(entries[i].path, entries[j].path) {
				return fmt.Errorf("%w: subtask %d path %q overlaps with subtask %d path %q",
					ErrOverlappingWriteScopes,
					entries[i].subtaskIdx, entries[i].path,
					entries[j].subtaskIdx, entries[j].path)
			}
		}
	}
	return nil
}

// pathsOverlap returns true if a is a prefix of b, b is a prefix of a, or they are equal.
// It uses directory-boundary-aware prefix checking (e.g., "src/api" does not overlap with "src/api-v2").
func pathsOverlap(a, b string) bool {
	if a == b {
		return true
	}
	// Check if a is a directory ancestor of b or vice versa.
	if strings.HasPrefix(b, a+"/") || strings.HasPrefix(a, b+"/") {
		return true
	}
	return false
}

// gatherCodeContext fetches project metadata to populate code context for the bridge request.
func (s *TaskDecompositionService) gatherCodeContext(ctx context.Context, projectID uuid.UUID) *DecompositionContext {
	if s.projectRepo == nil {
		return nil
	}

	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil || project == nil {
		return nil
	}

	// Populate context from project metadata; the bridge side handles actual code analysis.
	dc := &DecompositionContext{}
	if project.RepoURL != "" {
		dc.RelevantFiles = []string{project.RepoURL}
	}
	return dc
}

// fetchFewShotHistory queries memory for similar past decompositions.
func (s *TaskDecompositionService) fetchFewShotHistory(ctx context.Context, projectID uuid.UUID, taskTitle string) []FewShotExample {
	if s.memorySvc == nil {
		return nil
	}

	query := fmt.Sprintf("task-decomposition %s", taskTitle)
	memories, err := s.memorySvc.Search(ctx, projectID, query, 5)
	if err != nil || len(memories) == 0 {
		return nil
	}

	examples := make([]FewShotExample, 0, len(memories))
	for _, mem := range memories {
		var ex FewShotExample
		if err := json.Unmarshal([]byte(mem.Content), &ex); err != nil {
			continue
		}
		if ex.ParentTitle != "" && len(ex.SubtaskTitles) > 0 {
			examples = append(examples, ex)
		}
	}
	return examples
}

// recordDecompositionToMemory stores a successful decomposition as a few-shot example.
func (s *TaskDecompositionService) recordDecompositionToMemory(ctx context.Context, parent *model.Task, result *BridgeDecomposeResponse) {
	if s.memorySvc == nil {
		return
	}

	titles := make([]string, 0, len(result.Subtasks))
	for _, st := range result.Subtasks {
		titles = append(titles, strings.TrimSpace(st.Title))
	}

	example := FewShotExample{
		ParentTitle:   parent.Title,
		SubtaskTitles: titles,
	}
	content, err := json.Marshal(example)
	if err != nil {
		return
	}

	// Best-effort store; ignore errors.
	_, _ = s.memorySvc.Store(ctx, StoreMemoryInput{
		ProjectID:      parent.ProjectID,
		Scope:          model.MemoryScopeProject,
		Category:       model.MemoryCategoryProcedural,
		Key:            fmt.Sprintf("task-decomposition-%s", parent.ID.String()[:8]),
		Content:        string(content),
		RelevanceScore: 0.8,
	})
}

func validateTaskDecomposition(result *BridgeDecomposeResponse) error {
	if result == nil {
		return ErrInvalidTaskDecomposition
	}
	if strings.TrimSpace(result.Summary) == "" || len(result.Subtasks) == 0 {
		return ErrInvalidTaskDecomposition
	}
	for _, subtask := range result.Subtasks {
		if strings.TrimSpace(subtask.Title) == "" || strings.TrimSpace(subtask.Description) == "" {
			return ErrInvalidTaskDecomposition
		}
		if normalizeExecutionMode(subtask.ExecutionMode) == "" {
			return ErrInvalidTaskDecomposition
		}
	}
	return nil
}

func normalizeTaskPriority(priority string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "medium":
		return "medium"
	case "low":
		return "low"
	default:
		switch strings.ToLower(strings.TrimSpace(fallback)) {
		case "critical", "high", "medium", "low":
			return strings.ToLower(strings.TrimSpace(fallback))
		default:
			return "medium"
		}
	}
}

func normalizeExecutionMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "agent":
		return "agent"
	case "human":
		return "human"
	default:
		return ""
	}
}

func withExecutionModeLabel(labels []string, mode string) []string {
	result := make([]string, 0, len(labels)+1)
	for _, label := range labels {
		if strings.HasPrefix(label, "execution:") {
			continue
		}
		result = append(result, label)
	}

	normalizedMode := normalizeExecutionMode(mode)
	if normalizedMode != "" {
		result = append(result, "execution:"+normalizedMode)
	}

	return result
}
