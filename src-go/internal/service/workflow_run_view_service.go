package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

// Engine discriminators surfaced in the unified workflow-run view. These are
// wire-stable strings; the frontend branches on them to select the right
// detail component, and the filter surface accepts them verbatim.
const (
	UnifiedRunEngineDAG    = "dag"
	UnifiedRunEnginePlugin = "plugin"
)

// Canonical normalized run statuses emitted by the unified view. Engine-native
// values are translated to these via normalizePluginRunStatus /
// normalizeDAGExecutionStatus; unmapped values become "unknown".
const (
	UnifiedRunStatusPending   = "pending"
	UnifiedRunStatusRunning   = "running"
	UnifiedRunStatusPaused    = "paused"
	UnifiedRunStatusCompleted = "completed"
	UnifiedRunStatusFailed    = "failed"
	UnifiedRunStatusCancelled = "cancelled"
	UnifiedRunStatusUnknown   = "unknown"
)

// UnifiedRunWorkflowRef identifies a run's originating workflow. For DAG runs
// the ID is the WorkflowDefinition.ID (UUID); for plugin runs it is the
// plugin's string ID. Name is best-effort — empty when the workflow/plugin
// has been deleted.
type UnifiedRunWorkflowRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// UnifiedRunTriggeredBy describes how the run was initiated. Kind is one of
// "trigger" | "manual" | "sub_workflow" | "task". Ref carries the secondary
// identifier (trigger id, parent execution id, task id) as a string.
type UnifiedRunTriggeredBy struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref,omitempty"`
}

// UnifiedRunParentLink surfaces the parent↔child linkage for runs started as
// a sub_workflow child. nil when the run has no parent.
type UnifiedRunParentLink struct {
	ParentExecutionID string `json:"parentExecutionId"`
	ParentNodeID      string `json:"parentNodeId"`
}

// UnifiedRunRow is the canonical row shape returned by the unified list API
// and the fan-out WS channel. Keeps the frontend list code engine-agnostic.
type UnifiedRunRow struct {
	Engine           string                `json:"engine"`
	RunID            string                `json:"runId"`
	WorkflowRef      UnifiedRunWorkflowRef `json:"workflowRef"`
	Status           string                `json:"status"`
	StartedAt        string                `json:"startedAt,omitempty"`
	CompletedAt      string                `json:"completedAt,omitempty"`
	ActingEmployeeID string                `json:"actingEmployeeId,omitempty"`
	TriggeredBy      UnifiedRunTriggeredBy `json:"triggeredBy"`
	ParentLink       *UnifiedRunParentLink `json:"parentLink,omitempty"`
}

// UnifiedRunSummary cross-engine counts returned alongside each list page so
// the workspace tab header can render badges without a second request.
type UnifiedRunSummary struct {
	Running int `json:"running"`
	Paused  int `json:"paused"`
	Failed  int `json:"failed"`
}

// UnifiedRunListFilter mirrors the query surface defined by the spec. The
// handler layer parses URL parameters into this shape and the service layer
// pushes each dimension to the engine-native repo before merging.
type UnifiedRunListFilter struct {
	Engine           string      // "" | "dag" | "plugin"
	Statuses         []string    // canonical statuses; "" means no filter
	ActingEmployeeID *uuid.UUID
	TriggeredByKind  string // "" | "trigger" | "manual" | "sub_workflow"
	TriggerID        *uuid.UUID
	StartedAfter     *time.Time
	StartedBefore    *time.Time
}

// UnifiedRunListResult is the full list-API payload: rows + next cursor +
// cross-engine summary counts. Callers serialize this directly.
type UnifiedRunListResult struct {
	Rows       []UnifiedRunRow   `json:"rows"`
	NextCursor string            `json:"nextCursor,omitempty"`
	Summary    UnifiedRunSummary `json:"summary"`
}

// UnifiedRunDetail combines the shared envelope with the engine-native body
// for a single run. Body is the raw response shape existing per-engine read
// endpoints already return so existing detail UI components can consume it
// untouched.
type UnifiedRunDetail struct {
	Row  UnifiedRunRow `json:"row"`
	Body any           `json:"body"`
}

// --- Service dependencies (duck-typed interfaces for test injection) ---

// unifiedExecutionRepo is the narrow DAG execution surface the unified view
// needs. Implemented by *repository.WorkflowExecutionRepository.
type unifiedExecutionRepo interface {
	ListByProjectFiltered(ctx context.Context, projectID uuid.UUID, filter repository.WorkflowExecutionListFilter, limit int) ([]*model.WorkflowExecution, error)
	GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error)
}

// unifiedPluginRunRepo is the narrow plugin run surface the unified view
// needs. Implemented by *repository.WorkflowPluginRunRepository.
type unifiedPluginRunRepo interface {
	ListByProject(ctx context.Context, projectID uuid.UUID, filter repository.WorkflowPluginRunListFilter, limit int) ([]*model.WorkflowPluginRun, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowPluginRun, error)
}

// unifiedDefinitionRepo resolves workflow names for DAG rows. Implemented by
// *repository.WorkflowDefinitionRepository.
type unifiedDefinitionRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error)
}

// unifiedNodeExecutionRepo is optional; when wired the detail endpoint returns
// node executions alongside the DAG body for parity with the legacy endpoint.
type unifiedNodeExecutionRepo interface {
	ListNodeExecutions(ctx context.Context, executionID uuid.UUID) ([]*model.WorkflowNodeExecution, error)
}

// unifiedParentLinkRepo surfaces parent↔child linkage. Optional — when nil,
// parentLink is always nil on returned rows.
type unifiedParentLinkRepo interface {
	GetByChild(ctx context.Context, engineKind string, childRunID uuid.UUID) (*model.WorkflowRunParentLink, error)
}

// WorkflowRunViewService composes DAG executions and legacy plugin runs into
// the single cross-engine read surface consumed by the workflow workspace UI.
// The service owns filter pushdown, status normalization, cursor pagination
// and the workflow-name lookup cache.
type WorkflowRunViewService struct {
	execRepo   unifiedExecutionRepo
	pluginRepo unifiedPluginRunRepo
	defRepo    unifiedDefinitionRepo
	nodeRepo   unifiedNodeExecutionRepo
	linkRepo   unifiedParentLinkRepo
}

// NewWorkflowRunViewService constructs the service with its required deps.
// nodeRepo and linkRepo are wired separately via With* setters so tests can
// skip them.
func NewWorkflowRunViewService(execRepo unifiedExecutionRepo, pluginRepo unifiedPluginRunRepo, defRepo unifiedDefinitionRepo) *WorkflowRunViewService {
	return &WorkflowRunViewService{execRepo: execRepo, pluginRepo: pluginRepo, defRepo: defRepo}
}

// WithNodeRepo wires the node execution repo. Safe to omit; the detail
// endpoint will simply skip the `nodeExecutions` field.
func (s *WorkflowRunViewService) WithNodeRepo(r unifiedNodeExecutionRepo) *WorkflowRunViewService {
	s.nodeRepo = r
	return s
}

// WithParentLinkRepo wires the parent↔child link repo so rows carry
// parentLink metadata when applicable.
func (s *WorkflowRunViewService) WithParentLinkRepo(r unifiedParentLinkRepo) *WorkflowRunViewService {
	s.linkRepo = r
	return s
}

// --- Status normalization ---

// normalizeDAGExecutionStatus maps a DAG workflow_executions.status value to
// the canonical vocabulary. The DAG engine's native statuses already match
// the canonical set; unknown values are surfaced as "unknown".
func normalizeDAGExecutionStatus(s string) string {
	switch s {
	case model.WorkflowExecStatusPending:
		return UnifiedRunStatusPending
	case model.WorkflowExecStatusRunning:
		return UnifiedRunStatusRunning
	case model.WorkflowExecStatusPaused:
		return UnifiedRunStatusPaused
	case model.WorkflowExecStatusCompleted:
		return UnifiedRunStatusCompleted
	case model.WorkflowExecStatusFailed:
		return UnifiedRunStatusFailed
	case model.WorkflowExecStatusCancelled:
		return UnifiedRunStatusCancelled
	default:
		return UnifiedRunStatusUnknown
	}
}

// normalizePluginRunStatus maps a plugin run's native WorkflowRunStatus to the
// canonical vocabulary. Kept as a separate table so plugin-specific variants
// can be added without touching the DAG path.
func normalizePluginRunStatus(s model.WorkflowRunStatus) string {
	switch s {
	case model.WorkflowRunStatusPending:
		return UnifiedRunStatusPending
	case model.WorkflowRunStatusRunning:
		return UnifiedRunStatusRunning
	case model.WorkflowRunStatusPaused:
		return UnifiedRunStatusPaused
	case model.WorkflowRunStatusCompleted:
		return UnifiedRunStatusCompleted
	case model.WorkflowRunStatusFailed:
		return UnifiedRunStatusFailed
	case model.WorkflowRunStatusCancelled:
		return UnifiedRunStatusCancelled
	default:
		return UnifiedRunStatusUnknown
	}
}

// denormalizeStatusesForDAG translates canonical status filters back to the
// native DAG vocabulary (which happens to match), skipping "unknown" since no
// native row carries that value.
func denormalizeStatusesForDAG(statuses []string) []string {
	if len(statuses) == 0 {
		return nil
	}
	out := make([]string, 0, len(statuses))
	for _, s := range statuses {
		if s == UnifiedRunStatusUnknown || s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

// denormalizeStatusesForPlugin translates canonical status filters to the
// native plugin vocabulary (also 1:1 today).
func denormalizeStatusesForPlugin(statuses []string) []model.WorkflowRunStatus {
	if len(statuses) == 0 {
		return nil
	}
	out := make([]model.WorkflowRunStatus, 0, len(statuses))
	for _, s := range statuses {
		if s == UnifiedRunStatusUnknown || s == "" {
			continue
		}
		out = append(out, model.WorkflowRunStatus(s))
	}
	return out
}

// --- Cursor encoding ---

// unifiedRunCursor is the opaque pagination token encoded into nextCursor.
// Compound on (StartedAt, RunID) so ties at StartedAt resolve deterministically
// by RunID and inserts landing with the same StartedAt don't duplicate rows.
type unifiedRunCursor struct {
	StartedAt time.Time `json:"t"`
	RunID     string    `json:"r"`
}

func encodeCursor(c unifiedRunCursor) string {
	raw, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(s string) (unifiedRunCursor, error) {
	if s == "" {
		return unifiedRunCursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return unifiedRunCursor{}, fmt.Errorf("decode cursor: %w", err)
	}
	var c unifiedRunCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return unifiedRunCursor{}, fmt.Errorf("unmarshal cursor: %w", err)
	}
	return c, nil
}

// --- ListRuns ---

// ListRuns returns a page of unified rows for the project, applying filter
// pushdown per engine, merging in descending (startedAt, runId) order, and
// emitting the next cursor when more rows exist. The summary counts cross
// both engines and are always scoped to the filter except for the status
// dimension (running/paused/failed are absolute counts across every row that
// matches every other filter).
func (s *WorkflowRunViewService) ListRuns(ctx context.Context, projectID uuid.UUID, filter UnifiedRunListFilter, cursor string, limit int) (*UnifiedRunListResult, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	cur, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}

	wantDAG := filter.Engine == "" || filter.Engine == UnifiedRunEngineDAG
	wantPlugin := filter.Engine == "" || filter.Engine == UnifiedRunEnginePlugin

	// Fetch budget: 2*limit from each engine so we can merge and truncate.
	fetchLimit := limit * 2
	if fetchLimit < limit+1 {
		fetchLimit = limit + 1
	}

	var dagRuns []*model.WorkflowExecution
	if wantDAG && s.execRepo != nil {
		dagFilter := repository.WorkflowExecutionListFilter{
			Statuses:         denormalizeStatusesForDAG(filter.Statuses),
			ActingEmployeeID: filter.ActingEmployeeID,
			TriggerID:        filter.TriggerID,
			TriggeredByKind:  filter.TriggeredByKind,
			StartedAfter:     filter.StartedAfter,
			StartedBefore:    filter.StartedBefore,
		}
		dagRuns, err = s.execRepo.ListByProjectFiltered(ctx, projectID, dagFilter, fetchLimit)
		if err != nil {
			return nil, fmt.Errorf("unified list dag: %w", err)
		}
	}

	var pluginRuns []*model.WorkflowPluginRun
	if wantPlugin && s.pluginRepo != nil {
		pluginFilter := repository.WorkflowPluginRunListFilter{
			Statuses:         denormalizeStatusesForPlugin(filter.Statuses),
			ActingEmployeeID: filter.ActingEmployeeID,
			TriggerID:        filter.TriggerID,
			TriggeredByKind:  filter.TriggeredByKind,
			StartedAfter:     filter.StartedAfter,
			StartedBefore:    filter.StartedBefore,
		}
		pluginRuns, err = s.pluginRepo.ListByProject(ctx, projectID, pluginFilter, fetchLimit)
		if err != nil {
			return nil, fmt.Errorf("unified list plugin: %w", err)
		}
	}

	rows := make([]UnifiedRunRow, 0, len(dagRuns)+len(pluginRuns))
	for _, exec := range dagRuns {
		rows = append(rows, s.rowFromDAG(ctx, exec))
	}
	for _, run := range pluginRuns {
		rows = append(rows, s.rowFromPlugin(ctx, run))
	}

	sort.SliceStable(rows, func(i, j int) bool {
		ti := parseStartedAt(rows[i].StartedAt)
		tj := parseStartedAt(rows[j].StartedAt)
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return rows[i].RunID > rows[j].RunID
	})

	// Apply cursor: include only rows strictly after the cursor position.
	if !cur.StartedAt.IsZero() || cur.RunID != "" {
		filtered := rows[:0]
		for _, r := range rows {
			t := parseStartedAt(r.StartedAt)
			if t.After(cur.StartedAt) {
				continue // cursor expects rows strictly older than the anchor
			}
			if t.Equal(cur.StartedAt) && r.RunID >= cur.RunID {
				continue
			}
			filtered = append(filtered, r)
		}
		rows = filtered
	}

	summary := summarizeRows(rows)

	var nextCursor string
	if len(rows) > limit {
		anchor := rows[limit-1]
		nextCursor = encodeCursor(unifiedRunCursor{
			StartedAt: parseStartedAt(anchor.StartedAt),
			RunID:     anchor.RunID,
		})
		rows = rows[:limit]
	}

	return &UnifiedRunListResult{
		Rows:       rows,
		NextCursor: nextCursor,
		Summary:    summary,
	}, nil
}

// --- GetRun ---

// GetRun returns the unified detail for a single run keyed on (engine, runID).
// The shared envelope is always populated; the engine-native body mirrors the
// existing per-engine read endpoints. Unknown engines and cross-project reads
// return ErrNotFound rather than leaking the other engine's state.
func (s *WorkflowRunViewService) GetRun(ctx context.Context, projectID uuid.UUID, engine string, runID uuid.UUID) (*UnifiedRunDetail, error) {
	switch strings.ToLower(engine) {
	case UnifiedRunEngineDAG:
		return s.getDAG(ctx, projectID, runID)
	case UnifiedRunEnginePlugin:
		return s.getPlugin(ctx, projectID, runID)
	default:
		return nil, fmt.Errorf("unified run detail: unknown engine %q", engine)
	}
}

func (s *WorkflowRunViewService) getDAG(ctx context.Context, projectID uuid.UUID, runID uuid.UUID) (*UnifiedRunDetail, error) {
	if s.execRepo == nil {
		return nil, fmt.Errorf("unified run detail: dag exec repo not configured")
	}
	exec, err := s.execRepo.GetExecution(ctx, runID)
	if err != nil {
		return nil, err
	}
	if exec.ProjectID != projectID {
		return nil, repository.ErrNotFound
	}
	row := s.rowFromDAG(ctx, exec)
	body := map[string]any{"execution": exec.ToDTO()}
	if s.nodeRepo != nil {
		nodes, nErr := s.nodeRepo.ListNodeExecutions(ctx, exec.ID)
		if nErr == nil {
			dtos := make([]model.WorkflowNodeExecutionDTO, len(nodes))
			for i, n := range nodes {
				dtos[i] = n.ToDTO()
			}
			body["nodeExecutions"] = dtos
		}
	}
	return &UnifiedRunDetail{Row: row, Body: body}, nil
}

func (s *WorkflowRunViewService) getPlugin(ctx context.Context, projectID uuid.UUID, runID uuid.UUID) (*UnifiedRunDetail, error) {
	if s.pluginRepo == nil {
		return nil, fmt.Errorf("unified run detail: plugin run repo not configured")
	}
	run, err := s.pluginRepo.GetByID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.ProjectID != projectID {
		return nil, repository.ErrNotFound
	}
	row := s.rowFromPlugin(ctx, run)
	return &UnifiedRunDetail{Row: row, Body: run}, nil
}

// --- Row conversion ---

// rowFromDAG converts a DAG WorkflowExecution to the canonical row shape.
// Workflow name is best-effort via the definition repo; if the definition has
// been deleted, name is empty but the id is still surfaced.
func (s *WorkflowRunViewService) rowFromDAG(ctx context.Context, exec *model.WorkflowExecution) UnifiedRunRow {
	row := UnifiedRunRow{
		Engine: UnifiedRunEngineDAG,
		RunID:  exec.ID.String(),
		WorkflowRef: UnifiedRunWorkflowRef{
			ID: exec.WorkflowID.String(),
		},
		Status: normalizeDAGExecutionStatus(exec.Status),
	}
	if s.defRepo != nil {
		if def, err := s.defRepo.GetByID(ctx, exec.WorkflowID); err == nil && def != nil {
			row.WorkflowRef.Name = def.Name
		}
	}
	if exec.StartedAt != nil {
		row.StartedAt = exec.StartedAt.UTC().Format(time.RFC3339Nano)
	} else {
		row.StartedAt = exec.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	if exec.CompletedAt != nil {
		row.CompletedAt = exec.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	if exec.ActingEmployeeID != nil {
		row.ActingEmployeeID = exec.ActingEmployeeID.String()
	}
	if exec.TriggeredBy != nil {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "trigger", Ref: exec.TriggeredBy.String()}
	} else if exec.TaskID != nil {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "task", Ref: exec.TaskID.String()}
	} else {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "manual"}
	}
	if s.linkRepo != nil {
		if link, err := s.linkRepo.GetByChild(ctx, model.SubWorkflowEngineDAG, exec.ID); err == nil && link != nil {
			row.ParentLink = &UnifiedRunParentLink{
				ParentExecutionID: link.ParentExecutionID.String(),
				ParentNodeID:      link.ParentNodeID,
			}
			row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "sub_workflow", Ref: link.ParentExecutionID.String()}
		}
	}
	return row
}

// rowFromPlugin converts a WorkflowPluginRun to the canonical row shape.
func (s *WorkflowRunViewService) rowFromPlugin(ctx context.Context, run *model.WorkflowPluginRun) UnifiedRunRow {
	row := UnifiedRunRow{
		Engine: UnifiedRunEnginePlugin,
		RunID:  run.ID.String(),
		WorkflowRef: UnifiedRunWorkflowRef{
			ID:   run.PluginID,
			Name: run.PluginID,
		},
		Status:    normalizePluginRunStatus(run.Status),
		StartedAt: run.StartedAt.UTC().Format(time.RFC3339Nano),
	}
	if run.CompletedAt != nil {
		row.CompletedAt = run.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	if run.ActingEmployeeID != nil {
		row.ActingEmployeeID = run.ActingEmployeeID.String()
	}
	if run.TriggerID != nil {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "trigger", Ref: run.TriggerID.String()}
	} else if src, _ := run.Trigger["source"].(string); src == "workflow.trigger" {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "trigger"}
		if tid, _ := run.Trigger["triggerId"].(string); tid != "" {
			row.TriggeredBy.Ref = tid
		}
	} else if src == "sub_workflow" {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "sub_workflow"}
	} else if src == "task.trigger" {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "task"}
		if tid, _ := run.Trigger["taskId"].(string); tid != "" {
			row.TriggeredBy.Ref = tid
		}
	} else {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "manual"}
	}
	if s.linkRepo != nil {
		if link, err := s.linkRepo.GetByChild(ctx, model.SubWorkflowEnginePlugin, run.ID); err == nil && link != nil {
			row.ParentLink = &UnifiedRunParentLink{
				ParentExecutionID: link.ParentExecutionID.String(),
				ParentNodeID:      link.ParentNodeID,
			}
			row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "sub_workflow", Ref: link.ParentExecutionID.String()}
		}
	}
	return row
}

// summarizeRows counts canonical status values across the given rows.
func summarizeRows(rows []UnifiedRunRow) UnifiedRunSummary {
	var s UnifiedRunSummary
	for _, r := range rows {
		switch r.Status {
		case UnifiedRunStatusRunning:
			s.Running++
		case UnifiedRunStatusPaused:
			s.Paused++
		case UnifiedRunStatusFailed:
			s.Failed++
		}
	}
	return s
}

// parseStartedAt parses an RFC3339Nano timestamp string, returning zero time
// on parse failure. Used during merge-sort of cross-engine rows.
func parseStartedAt(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
