package liveartifact

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// scopeAssetProject is a WS-router sentinel: the router substitutes it
// with the hosting asset's actual project_id before matching hub
// events. Kept lowercase/file-scoped to avoid clashing with a sibling
// cost_summary projector that declares its own copy; §9.4 will
// consolidate. See the live-artifact-projection spec.
const scopeAssetProject = "$asset_project"

// taskListReader is the narrow slice of *repository.TaskRepository
// this projector depends on. Repository-package isolation keeps the
// projector unit-testable.
type taskListReader interface {
	List(ctx context.Context, projectID uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error)
}

// savedViewReader covers the saved-view lookup path. Optional: pass
// nil and the projector returns StatusDegraded for saved_view_id refs.
type savedViewReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.SavedView, error)
}

// TaskGroupProjector renders a filtered task list as a BlockNote
// fragment for embedding inside wiki pages.
type TaskGroupProjector struct {
	tasks      taskListReader
	savedViews savedViewReader // may be nil
}

// NewTaskGroupProjector constructs the projector. savedViews may be
// nil; when nil, saved-view refs degrade gracefully.
func NewTaskGroupProjector(tasks taskListReader, savedViews savedViewReader) *TaskGroupProjector {
	return &TaskGroupProjector{tasks: tasks, savedViews: savedViews}
}

// Kind reports the discriminator this projector handles.
func (p *TaskGroupProjector) Kind() LiveArtifactKind { return KindTaskGroup }

// RequiredRole reports the minimum role tier.
func (p *TaskGroupProjector) RequiredRole() Role { return RoleViewer }

// taskGroupFilter is the inline filter shape inside target_ref.filter.
type taskGroupFilter struct {
	SavedViewID string `json:"saved_view_id,omitempty"`
	Status      string `json:"status,omitempty"`
	Assignee    string `json:"assignee,omitempty"`
	Tag         string `json:"tag,omitempty"`
	SprintID    string `json:"sprint_id,omitempty"`
	MilestoneID string `json:"milestone_id,omitempty"`
}

// taskGroupTargetRef is the full target_ref shape.
type taskGroupTargetRef struct {
	Kind   string          `json:"kind"`
	Filter taskGroupFilter `json:"filter"`
}

// taskGroupViewOpts is the view_opts shape.
type taskGroupViewOpts struct {
	PageSize *int   `json:"page_size,omitempty"`
	Sort     string `json:"sort,omitempty"`
}

const (
	defaultPageSize        = 50
	maxPageSize            = 50
	defaultTaskGroupSort   = "updated_at_desc"
	taskGroupTTL           = 30 * time.Second
	savedViewUnsupportedMsg = "saved views not yet supported"
)

// allowedSorts maps projector sort tokens to repository Sort strings.
// Unknown values fall back to the default.
var allowedSorts = map[string]string{
	"updated_at_desc": "updated_at desc",
	"status":          "status asc",
	"priority":        "priority asc",
	"due_date":        "planned_end_at asc",
}

// Project runs the projection. See LiveArtifactProjector for contract.
func (p *TaskGroupProjector) Project(
	ctx context.Context,
	principal model.PrincipalContext,
	projectID uuid.UUID,
	targetRef json.RawMessage,
	viewOpts json.RawMessage,
) (ProjectionResult, error) {
	now := time.Now().UTC()

	if !PrincipalHasRole(principal, p.RequiredRole()) {
		return ProjectionResult{Status: StatusForbidden, ProjectedAt: now}, nil
	}

	ref, err := parseTaskGroupTargetRef(targetRef)
	if err != nil {
		return ProjectionResult{
			Status:      StatusDegraded,
			ProjectedAt: now,
			Diagnostics: err.Error(),
		}, nil
	}

	pageSize, sortToken := parseTaskGroupViewOpts(viewOpts)

	// Saved-view path.
	if ref.Filter.SavedViewID != "" {
		if p.savedViews == nil {
			// TODO(§6 follow-up): wire saved-view translation once the
			// view config schema stabilises.
			return ProjectionResult{
				Status:      StatusDegraded,
				ProjectedAt: now,
				Diagnostics: savedViewUnsupportedMsg,
			}, nil
		}
		// Even with a reader, translation isn't implemented yet. Degrade.
		return ProjectionResult{
			Status:      StatusDegraded,
			ProjectedAt: now,
			Diagnostics: savedViewUnsupportedMsg,
		}, nil
	}

	query := model.TaskListQuery{
		Status:     ref.Filter.Status,
		AssigneeID: ref.Filter.Assignee,
		SprintID:   ref.Filter.SprintID,
		Page:       1,
		Limit:      pageSize,
		Sort:       allowedSorts[sortToken],
	}

	// Fetch tasks.
	tasks, total, err := p.tasks.List(ctx, projectID, query)
	if err != nil {
		return ProjectionResult{
			Status:      StatusDegraded,
			ProjectedAt: now,
			Diagnostics: err.Error(),
		}, nil
	}

	// Post-filters: tag (against task Labels) and milestone_id.
	// TaskListQuery has no Tag or MilestoneID fields; applied here.
	if ref.Filter.Tag != "" {
		tasks = filterByTag(tasks, ref.Filter.Tag)
	}
	if ref.Filter.MilestoneID != "" {
		tasks = filterByMilestone(tasks, ref.Filter.MilestoneID)
	}

	returned := len(tasks)
	if returned > pageSize {
		tasks = tasks[:pageSize]
		returned = pageSize
	}

	// When post-filters changed the count we cannot trust the original
	// total. Fall back to the in-memory count.
	if ref.Filter.Tag != "" || ref.Filter.MilestoneID != "" {
		total = returned
	}

	fragment, err := renderTaskGroupBlocks(ref.Filter, tasks, total, returned)
	if err != nil {
		return ProjectionResult{
			Status:      StatusDegraded,
			ProjectedAt: now,
			Diagnostics: err.Error(),
		}, nil
	}

	ttl := taskGroupTTL
	return ProjectionResult{
		Status:      StatusOK,
		Projection:  fragment,
		ProjectedAt: now,
		TTLHint:     &ttl,
	}, nil
}

// Subscribe declares hub event topics that should re-trigger a
// projection of a task_group block.
//
// Scope narrowing: project_id is always substituted by the router via
// scopeAssetProject. sprint_id and assignee narrow further when the
// inline filter has them. tag and milestone_id are intentionally not
// included in scope: task events do not reliably carry those keys.
func (p *TaskGroupProjector) Subscribe(targetRef json.RawMessage) []EventTopic {
	empty := []EventTopic{}
	ref, err := parseTaskGroupTargetRef(targetRef)
	if err != nil {
		return empty
	}
	scope := map[string]string{"project_id": scopeAssetProject}
	if ref.Filter.SprintID != "" {
		scope["sprint_id"] = ref.Filter.SprintID
	}
	if ref.Filter.Assignee != "" {
		scope["assignee_id"] = ref.Filter.Assignee
	}
	events := []string{
		"task.created",
		"task.updated",
		"task.deleted",
		"task.transitioned",
		"task.assigned",
	}
	out := make([]EventTopic, 0, len(events))
	for _, name := range events {
		out = append(out, EventTopic{Event: name, Scope: copyScopeMap(scope)})
	}
	return out
}

// --- helpers ---

func parseTaskGroupTargetRef(raw json.RawMessage) (taskGroupTargetRef, error) {
	var ref taskGroupTargetRef
	if len(raw) == 0 {
		return ref, fmt.Errorf("target_ref missing")
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		return ref, fmt.Errorf("target_ref invalid: %w", err)
	}
	if ref.Kind != "" && ref.Kind != string(KindTaskGroup) {
		return ref, fmt.Errorf("target_ref kind %q not task_group", ref.Kind)
	}
	hasSavedView := ref.Filter.SavedViewID != ""
	hasInline := ref.Filter.Status != "" ||
		ref.Filter.Assignee != "" ||
		ref.Filter.Tag != "" ||
		ref.Filter.SprintID != "" ||
		ref.Filter.MilestoneID != ""
	if hasSavedView == hasInline {
		// Both set, or both empty.
		return ref, fmt.Errorf("filter must have exactly one of saved_view_id or inline fields")
	}
	if hasSavedView {
		if _, err := uuid.Parse(ref.Filter.SavedViewID); err != nil {
			return ref, fmt.Errorf("saved_view_id invalid: %w", err)
		}
	}
	return ref, nil
}

func parseTaskGroupViewOpts(raw json.RawMessage) (pageSize int, sortToken string) {
	pageSize = defaultPageSize
	sortToken = defaultTaskGroupSort
	if len(raw) == 0 {
		return
	}
	var opts taskGroupViewOpts
	if err := json.Unmarshal(raw, &opts); err != nil {
		return
	}
	if opts.PageSize != nil {
		if *opts.PageSize > 0 && *opts.PageSize <= maxPageSize {
			pageSize = *opts.PageSize
		} else if *opts.PageSize > maxPageSize {
			pageSize = maxPageSize
		}
	}
	if opts.Sort != "" {
		if _, ok := allowedSorts[opts.Sort]; ok {
			sortToken = opts.Sort
		}
	}
	return
}

func filterByTag(tasks []*model.Task, tag string) []*model.Task {
	out := make([]*model.Task, 0, len(tasks))
	for _, t := range tasks {
		for _, label := range t.Labels {
			if label == tag {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

func filterByMilestone(tasks []*model.Task, milestone string) []*model.Task {
	out := make([]*model.Task, 0, len(tasks))
	target, err := uuid.Parse(milestone)
	if err != nil {
		// Malformed id: yields no matches.
		return out
	}
	for _, t := range tasks {
		if t.MilestoneID != nil && *t.MilestoneID == target {
			out = append(out, t)
		}
	}
	return out
}

func renderTaskGroupBlocks(
	filter taskGroupFilter,
	tasks []*model.Task,
	total int,
	returned int,
) (json.RawMessage, error) {
	blocks := make([]map[string]any, 0, 4)

	title := "Tasks"
	if subtitle := filterSubtitle(filter); subtitle != "" {
		title = "Tasks · " + subtitle
	}
	blocks = append(blocks, headingBlock(3, title))

	if returned == 0 {
		blocks = append(blocks, paragraphBlock("No tasks match this filter."))
		return json.Marshal(blocks)
	}

	blocks = append(blocks, paragraphBlock(fmt.Sprintf("Showing %d/%d", returned, total)))

	// TODO(§10.5): replace this paragraph-per-row fallback with a
	// BlockNote table block once the frontend's table shape is
	// confirmed. Each row renders as
	// "• {title} — {status} · {assignee} · {priority} · {updated-human}".
	for _, t := range tasks {
		blocks = append(blocks, paragraphBlock(renderTaskRow(t)))
	}

	if total > returned {
		blocks = append(blocks, paragraphBlock(fmt.Sprintf(
			"… %d more matching tasks.", total-returned,
		)))
	}
	return json.Marshal(blocks)
}

func renderTaskRow(t *model.Task) string {
	assignee := "—"
	if t.AssigneeID != nil {
		assignee = t.AssigneeID.String()
	}
	return fmt.Sprintf(
		"• %s — %s · %s · %s · %s",
		t.Title,
		t.Status,
		assignee,
		t.Priority,
		humanAge(t.UpdatedAt),
	)
}

// humanAge returns a compact "x ago" string. Uses UTC and coarse
// buckets; we're rendering an informational cell, not a timestamp.
func humanAge(ts time.Time) string {
	if ts.IsZero() {
		return "—"
	}
	d := time.Since(ts)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// filterSubtitle builds a short human summary of an inline filter in a
// stable order ("status=..., assignee=...") for the heading.
func filterSubtitle(f taskGroupFilter) string {
	parts := map[string]string{}
	if f.Status != "" {
		parts["status"] = f.Status
	}
	if f.Assignee != "" {
		parts["assignee"] = f.Assignee
	}
	if f.Tag != "" {
		parts["tag"] = f.Tag
	}
	if f.SprintID != "" {
		parts["sprint"] = f.SprintID
	}
	if f.MilestoneID != "" {
		parts["milestone"] = f.MilestoneID
	}
	if len(parts) == 0 {
		return ""
	}
	keys := make([]string, 0, len(parts))
	for k := range parts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(parts[k])
	}
	return sb.String()
}

func copyScopeMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
