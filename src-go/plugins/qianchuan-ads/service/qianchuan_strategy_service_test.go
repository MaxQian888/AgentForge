package qcservice_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/agentforge/server/internal/repository"
	qcservice "github.com/agentforge/server/plugins/qianchuan-ads/service"
	"github.com/agentforge/server/plugins/qianchuan-ads/strategy"
	"github.com/google/uuid"
)

const validYAML = `
name: my-strategy
triggers:
  schedule: 1m
inputs:
  - metric: cost
    dimensions: [ad_id]
    window: 1m
rules:
  - name: heartbeat
    condition: "true"
    actions:
      - type: notify_im
        target: {}
        params:
          channel: default
          template: "tick"
`

// fakeStrategyRepo is an in-memory mock of the qianchuanStrategyRepo interface.
type fakeStrategyRepo struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*strategy.QianchuanStrategy
}

func newFakeRepo() *fakeStrategyRepo {
	return &fakeStrategyRepo{rows: map[uuid.UUID]*strategy.QianchuanStrategy{}}
}

func (f *fakeStrategyRepo) Insert(_ context.Context, s *strategy.QianchuanStrategy) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	cp := *s
	f.rows[cp.ID] = &cp
	return nil
}

func (f *fakeStrategyRepo) GetByID(_ context.Context, id uuid.UUID) (*strategy.QianchuanStrategy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *row
	return &cp, nil
}

func (f *fakeStrategyRepo) ListByProject(_ context.Context, pid uuid.UUID, includeSystem bool) ([]*strategy.QianchuanStrategy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []*strategy.QianchuanStrategy{}
	for _, r := range f.rows {
		if r.ProjectID != nil && *r.ProjectID == pid {
			cp := *r
			out = append(out, &cp)
			continue
		}
		if includeSystem && r.ProjectID == nil {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeStrategyRepo) UpdateDraft(_ context.Context, id uuid.UUID, desc, yamlSource, parsedSpec string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[id]
	if !ok || row.Status != strategy.StatusDraft {
		return repository.ErrNotFound
	}
	row.Description = desc
	row.YAMLSource = yamlSource
	row.ParsedSpec = parsedSpec
	return nil
}

func (f *fakeStrategyRepo) SetStatus(_ context.Context, id uuid.UUID, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[id]
	if !ok {
		return repository.ErrNotFound
	}
	row.Status = status
	return nil
}

func (f *fakeStrategyRepo) DeleteDraft(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[id]
	if !ok || row.Status != strategy.StatusDraft {
		return repository.ErrNotFound
	}
	delete(f.rows, id)
	return nil
}

func (f *fakeStrategyRepo) MaxVersion(_ context.Context, projectID *uuid.UUID, name string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	max := 0
	for _, r := range f.rows {
		if r.Name != name {
			continue
		}
		if (projectID == nil && r.ProjectID != nil) || (projectID != nil && r.ProjectID == nil) {
			continue
		}
		if projectID != nil && r.ProjectID != nil && *projectID != *r.ProjectID {
			continue
		}
		if r.Version > max {
			max = r.Version
		}
	}
	return max, nil
}

func TestQianchuanServiceCreate(t *testing.T) {
	ctx := context.Background()
	svc := qcservice.NewQianchuanStrategyService(newFakeRepo())
	pid := uuid.New()
	row, err := svc.Create(ctx, qcservice.QianchuanStrategyCreateInput{
		ProjectID:  &pid,
		YAMLSource: validYAML,
		CreatedBy:  uuid.New(),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if row.Version != 1 {
		t.Errorf("first version: got %d want 1", row.Version)
	}
	if row.Status != strategy.StatusDraft {
		t.Errorf("status: got %q want draft", row.Status)
	}
}

func TestQianchuanServiceCreateBumpsVersion(t *testing.T) {
	ctx := context.Background()
	svc := qcservice.NewQianchuanStrategyService(newFakeRepo())
	pid := uuid.New()
	user := uuid.New()
	for v := 1; v <= 3; v++ {
		row, err := svc.Create(ctx, qcservice.QianchuanStrategyCreateInput{
			ProjectID:  &pid,
			YAMLSource: validYAML,
			CreatedBy:  user,
		})
		if err != nil {
			t.Fatalf("Create v%d: %v", v, err)
		}
		if row.Version != v {
			t.Errorf("Create v%d: got %d", v, row.Version)
		}
	}
}

func TestQianchuanServiceUpdateRejectedOnPublished(t *testing.T) {
	ctx := context.Background()
	svc := qcservice.NewQianchuanStrategyService(newFakeRepo())
	pid := uuid.New()
	row, _ := svc.Create(ctx, qcservice.QianchuanStrategyCreateInput{ProjectID: &pid, YAMLSource: validYAML, CreatedBy: uuid.New()})
	if err := svc.Publish(ctx, row.ID); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	_, err := svc.Update(ctx, row.ID, validYAML)
	if !errors.Is(err, qcservice.ErrStrategyImmutable) {
		t.Errorf("expected ErrStrategyImmutable, got %v", err)
	}
}

func TestQianchuanServicePublishTwiceRejected(t *testing.T) {
	ctx := context.Background()
	svc := qcservice.NewQianchuanStrategyService(newFakeRepo())
	pid := uuid.New()
	row, _ := svc.Create(ctx, qcservice.QianchuanStrategyCreateInput{ProjectID: &pid, YAMLSource: validYAML, CreatedBy: uuid.New()})
	if err := svc.Publish(ctx, row.ID); err != nil {
		t.Fatalf("Publish 1: %v", err)
	}
	err := svc.Publish(ctx, row.ID)
	if !errors.Is(err, qcservice.ErrStrategyInvalidTransition) {
		t.Errorf("Publish 2: got %v want ErrStrategyInvalidTransition", err)
	}
}

func TestQianchuanServiceArchiveOnlyFromPublished(t *testing.T) {
	ctx := context.Background()
	svc := qcservice.NewQianchuanStrategyService(newFakeRepo())
	pid := uuid.New()
	row, _ := svc.Create(ctx, qcservice.QianchuanStrategyCreateInput{ProjectID: &pid, YAMLSource: validYAML, CreatedBy: uuid.New()})
	err := svc.Archive(ctx, row.ID)
	if !errors.Is(err, qcservice.ErrStrategyInvalidTransition) {
		t.Errorf("archive on draft: got %v want ErrStrategyInvalidTransition", err)
	}
	_ = svc.Publish(ctx, row.ID)
	if err := svc.Archive(ctx, row.ID); err != nil {
		t.Fatalf("archive on published: %v", err)
	}
}

func TestQianchuanServiceDeleteOnlyDraft(t *testing.T) {
	ctx := context.Background()
	svc := qcservice.NewQianchuanStrategyService(newFakeRepo())
	pid := uuid.New()
	row, _ := svc.Create(ctx, qcservice.QianchuanStrategyCreateInput{ProjectID: &pid, YAMLSource: validYAML, CreatedBy: uuid.New()})
	if err := svc.Delete(ctx, row.ID); err != nil {
		t.Fatalf("delete draft: %v", err)
	}

	row2, _ := svc.Create(ctx, qcservice.QianchuanStrategyCreateInput{ProjectID: &pid, YAMLSource: validYAML, CreatedBy: uuid.New()})
	_ = svc.Publish(ctx, row2.ID)
	err := svc.Delete(ctx, row2.ID)
	if !errors.Is(err, qcservice.ErrStrategyImmutable) {
		t.Errorf("delete published: got %v want ErrStrategyImmutable", err)
	}
}

func TestQianchuanServiceEditAfterPublishRequiresNewCreate(t *testing.T) {
	ctx := context.Background()
	svc := qcservice.NewQianchuanStrategyService(newFakeRepo())
	pid := uuid.New()
	user := uuid.New()
	row, _ := svc.Create(ctx, qcservice.QianchuanStrategyCreateInput{ProjectID: &pid, YAMLSource: validYAML, CreatedBy: user})
	_ = svc.Publish(ctx, row.ID)

	// Subsequent Create with the same name returns a fresh draft at version=2.
	row2, err := svc.Create(ctx, qcservice.QianchuanStrategyCreateInput{ProjectID: &pid, YAMLSource: validYAML, CreatedBy: user})
	if err != nil {
		t.Fatalf("Create v2: %v", err)
	}
	if row2.Version != 2 {
		t.Errorf("expected v2, got %d", row2.Version)
	}
	if row2.Status != strategy.StatusDraft {
		t.Errorf("expected draft, got %q", row2.Status)
	}
	if row2.ID == row.ID {
		t.Errorf("expected new row")
	}
}

func TestQianchuanServiceTestRunHeartbeatRule(t *testing.T) {
	ctx := context.Background()
	svc := qcservice.NewQianchuanStrategyService(newFakeRepo())
	pid := uuid.New()
	row, _ := svc.Create(ctx, qcservice.QianchuanStrategyCreateInput{ProjectID: &pid, YAMLSource: validYAML, CreatedBy: uuid.New()})
	res, err := svc.TestRun(ctx, row.ID, map[string]any{"metrics": map[string]any{"cost": 12.5}})
	if err != nil {
		t.Fatalf("TestRun: %v", err)
	}
	if len(res.FiredRules) != 1 || res.FiredRules[0] != "heartbeat" {
		t.Errorf("fired rules: got %+v", res.FiredRules)
	}
	if len(res.Actions) != 1 || res.Actions[0].Type != "notify_im" {
		t.Errorf("actions: got %+v", res.Actions)
	}
}

func TestQianchuanServiceWriteRejectsSystemRow(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := qcservice.NewQianchuanStrategyService(repo)
	// Insert a system row directly.
	sysRow := &strategy.QianchuanStrategy{
		Name: "system:x", YAMLSource: validYAML, ParsedSpec: "{}",
		Version: 1, Status: strategy.StatusPublished, CreatedBy: uuid.New(),
	}
	_ = repo.Insert(ctx, sysRow)

	if err := svc.Publish(ctx, sysRow.ID); !errors.Is(err, qcservice.ErrStrategySystemReadOnly) {
		t.Errorf("publish system: got %v", err)
	}
	if err := svc.Archive(ctx, sysRow.ID); !errors.Is(err, qcservice.ErrStrategySystemReadOnly) {
		t.Errorf("archive system: got %v", err)
	}
	if err := svc.Delete(ctx, sysRow.ID); !errors.Is(err, qcservice.ErrStrategySystemReadOnly) {
		t.Errorf("delete system: got %v", err)
	}
	if _, err := svc.Update(ctx, sysRow.ID, validYAML); !errors.Is(err, qcservice.ErrStrategySystemReadOnly) {
		t.Errorf("update system: got %v", err)
	}
}

func TestQianchuanServiceCreateBubblesParseErrors(t *testing.T) {
	ctx := context.Background()
	svc := qcservice.NewQianchuanStrategyService(newFakeRepo())
	pid := uuid.New()
	_, err := svc.Create(ctx, qcservice.QianchuanStrategyCreateInput{
		ProjectID: &pid, YAMLSource: "not: [valid", CreatedBy: uuid.New(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var spe *strategy.StrategyParseError
	if !errors.As(err, &spe) {
		t.Errorf("expected *StrategyParseError, got %T", err)
	}
}
