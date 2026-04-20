package qcservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/agentforge/server/internal/workflow/nodetypes"
	"github.com/agentforge/server/plugins/qianchuan-ads/strategy"
	"github.com/google/uuid"
)

// qianchuanStrategyRepo is the narrow seam the service consumes. Defined here
// so tests can mock without dragging GORM in.
type qianchuanStrategyRepo interface {
	Insert(ctx context.Context, s *strategy.QianchuanStrategy) error
	GetByID(ctx context.Context, id uuid.UUID) (*strategy.QianchuanStrategy, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, includeSystem bool) ([]*strategy.QianchuanStrategy, error)
	UpdateDraft(ctx context.Context, id uuid.UUID, description, yamlSource, parsedSpec string) error
	SetStatus(ctx context.Context, id uuid.UUID, status string) error
	DeleteDraft(ctx context.Context, id uuid.UUID) error
	MaxVersion(ctx context.Context, projectID *uuid.UUID, name string) (int, error)
}

// QianchuanStrategyService is the orchestration layer for the strategy
// library: parse → validate → persist + status transitions + dry-run TestRun.
type QianchuanStrategyService struct {
	repo qianchuanStrategyRepo
}

func NewQianchuanStrategyService(repo qianchuanStrategyRepo) *QianchuanStrategyService {
	return &QianchuanStrategyService{repo: repo}
}

// Common service-layer errors. Handlers translate these into HTTP statuses.
var (
	// ErrStrategyImmutable is returned when a write targets a published or
	// archived row that this endpoint cannot modify.
	ErrStrategyImmutable = errors.New("strategy is immutable in current status")
	// ErrStrategyInvalidTransition is returned when the requested status
	// transition is not legal (e.g. archived -> anything).
	ErrStrategyInvalidTransition = errors.New("invalid status transition")
	// ErrStrategySystemReadOnly is returned when a write targets a system
	// seed (project_id IS NULL).
	ErrStrategySystemReadOnly = errors.New("system strategies are read-only")
)

// QianchuanStrategyCreateInput is the payload accepted by Create. ProjectID may be nil only
// for the seed loader; HTTP handlers always set it.
type QianchuanStrategyCreateInput struct {
	ProjectID  *uuid.UUID
	YAMLSource string
	CreatedBy  uuid.UUID
}

// Create parses the YAML, validates it, and persists a new draft row.
// The version starts at max(version)+1 for the (project, name) tuple.
func (s *QianchuanStrategyService) Create(ctx context.Context, in QianchuanStrategyCreateInput) (*strategy.QianchuanStrategy, error) {
	strat, parsed, err := strategy.Parse(in.YAMLSource)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return nil, fmt.Errorf("encode parsed spec: %w", err)
	}
	maxV, err := s.repo.MaxVersion(ctx, in.ProjectID, strat.Name)
	if err != nil {
		return nil, err
	}
	row := &strategy.QianchuanStrategy{
		ProjectID:   in.ProjectID,
		Name:        strat.Name,
		Description: strat.Description,
		YAMLSource:  in.YAMLSource,
		ParsedSpec:  string(encoded),
		Version:     maxV + 1,
		Status:      strategy.StatusDraft,
		CreatedBy:   in.CreatedBy,
	}
	if err := s.repo.Insert(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

// Update re-parses the YAML and overwrites a draft row. Returns
// ErrStrategyImmutable if the row is not in draft.
func (s *QianchuanStrategyService) Update(ctx context.Context, id uuid.UUID, yamlSource string) (*strategy.QianchuanStrategy, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.IsSystem() {
		return nil, ErrStrategySystemReadOnly
	}
	if existing.Status != strategy.StatusDraft {
		return nil, ErrStrategyImmutable
	}
	strat, parsed, err := strategy.Parse(yamlSource)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return nil, fmt.Errorf("encode parsed spec: %w", err)
	}
	if strat.Name != existing.Name {
		return nil, fmt.Errorf("cannot rename a strategy after creation (was %q, got %q)", existing.Name, strat.Name)
	}
	if err := s.repo.UpdateDraft(ctx, id, strat.Description, yamlSource, string(encoded)); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Publish flips draft -> published. Idempotent rejects when status != draft.
func (s *QianchuanStrategyService) Publish(ctx context.Context, id uuid.UUID) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.IsSystem() {
		return ErrStrategySystemReadOnly
	}
	if existing.Status != strategy.StatusDraft {
		return ErrStrategyInvalidTransition
	}
	return s.repo.SetStatus(ctx, id, strategy.StatusPublished)
}

// Archive flips published -> archived.
func (s *QianchuanStrategyService) Archive(ctx context.Context, id uuid.UUID) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.IsSystem() {
		return ErrStrategySystemReadOnly
	}
	if existing.Status != strategy.StatusPublished {
		return ErrStrategyInvalidTransition
	}
	return s.repo.SetStatus(ctx, id, strategy.StatusArchived)
}

// Delete hard-deletes a draft row.
func (s *QianchuanStrategyService) Delete(ctx context.Context, id uuid.UUID) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.IsSystem() {
		return ErrStrategySystemReadOnly
	}
	if existing.Status != strategy.StatusDraft {
		return ErrStrategyImmutable
	}
	return s.repo.DeleteDraft(ctx, id)
}

// Get returns a single row by ID.
func (s *QianchuanStrategyService) Get(ctx context.Context, id uuid.UUID) (*strategy.QianchuanStrategy, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns a project's strategies plus the system seeds.
func (s *QianchuanStrategyService) List(ctx context.Context, projectID uuid.UUID) ([]*strategy.QianchuanStrategy, error) {
	return s.repo.ListByProject(ctx, projectID, true)
}

// TestRunResult is the dry-run output: the actions a single tick would emit
// for the given snapshot, plus any rules that fired.
type TestRunResult struct {
	FiredRules []string                `json:"fired_rules"`
	Actions    []TestRunResolvedAction `json:"actions"`
}

// TestRunResolvedAction mirrors strategy.ParsedAction with the ad_id_expr
// resolved against the snapshot for FE display.
type TestRunResolvedAction struct {
	Rule   string         `json:"rule"`
	Type   string         `json:"type"`
	AdID   string         `json:"ad_id,omitempty"`
	Params map[string]any `json:"params"`
}

// TestRun evaluates each rule's condition against the supplied snapshot and
// returns the resulting actions. No persistence, no policy gate — this is
// the FE's "what would this strategy do right now" preview.
func (s *QianchuanStrategyService) TestRun(ctx context.Context, id uuid.UUID, snapshot map[string]any) (*TestRunResult, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var spec strategy.ParsedSpec
	if err := json.Unmarshal([]byte(row.ParsedSpec), &spec); err != nil {
		return nil, fmt.Errorf("decode parsed spec: %w", err)
	}

	store := map[string]any{"snapshot": snapshot}
	out := &TestRunResult{}
	for _, rule := range spec.Rules {
		if !evalRuleCondition(rule.ConditionRaw, store) {
			continue
		}
		out.FiredRules = append(out.FiredRules, rule.Name)
		for _, a := range rule.Actions {
			resolved := TestRunResolvedAction{
				Rule:   rule.Name,
				Type:   a.Type,
				Params: a.Params,
			}
			if a.AdIDExpr != "" {
				val := nodetypes.EvaluateExpression(a.AdIDExpr, store)
				resolved.AdID = fmt.Sprintf("%v", val)
			}
			out.Actions = append(out.Actions, resolved)
		}
	}
	return out, nil
}

// evalRuleCondition wraps EvaluateExpression with a truthy-predicate. v1
// semantics: bool true; non-zero number; non-empty non-"false" string;
// non-nil/non-empty maps and slices.
func evalRuleCondition(expr string, store map[string]any) bool {
	val := nodetypes.EvaluateExpression(expr, store)
	switch v := val.(type) {
	case nil:
		return false
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	case string:
		if v == "" || v == "false" {
			return false
		}
		return true
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return true
	}
}
