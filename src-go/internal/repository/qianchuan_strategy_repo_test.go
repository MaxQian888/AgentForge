package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/qianchuan/strategy"
)

func newTestStrategyRepo(t *testing.T) *QianchuanStrategyRepository {
	return NewQianchuanStrategyRepository(openFoundationRepoTestDB(t, &qianchuanStrategyRecord{}))
}

func TestQianchuanStrategyRepoInsertAndGet(t *testing.T) {
	ctx := context.Background()
	repo := newTestStrategyRepo(t)
	pid := uuid.New()
	row := &strategy.QianchuanStrategy{
		ProjectID:   &pid,
		Name:        "test-1",
		YAMLSource:  "name: x",
		ParsedSpec:  `{"schema_version":1}`,
		Version:     1,
		Status:      strategy.StatusDraft,
		Description: "hello",
		CreatedBy:   uuid.New(),
	}
	if err := repo.Insert(ctx, row); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if row.ID == uuid.Nil {
		t.Fatal("expected ID to be assigned")
	}

	got, err := repo.GetByID(ctx, row.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "test-1" || got.Description != "hello" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.ProjectID == nil || *got.ProjectID != pid {
		t.Errorf("project id round-trip: got %v want %v", got.ProjectID, pid)
	}
}

func TestQianchuanStrategyRepoListByProjectIncludesSystem(t *testing.T) {
	ctx := context.Background()
	repo := newTestStrategyRepo(t)

	pidA := uuid.New()
	pidB := uuid.New()

	mustInsert := func(s *strategy.QianchuanStrategy) {
		if err := repo.Insert(ctx, s); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	mustInsert(&strategy.QianchuanStrategy{ProjectID: &pidA, Name: "a", YAMLSource: "a", ParsedSpec: "{}", Version: 1, Status: strategy.StatusDraft, CreatedBy: uuid.New()})
	mustInsert(&strategy.QianchuanStrategy{ProjectID: &pidB, Name: "b", YAMLSource: "b", ParsedSpec: "{}", Version: 1, Status: strategy.StatusDraft, CreatedBy: uuid.New()})
	mustInsert(&strategy.QianchuanStrategy{ProjectID: nil, Name: "system:s", YAMLSource: "s", ParsedSpec: "{}", Version: 1, Status: strategy.StatusPublished, CreatedBy: uuid.New()})

	listA, err := repo.ListByProject(ctx, pidA, false)
	if err != nil {
		t.Fatalf("list excluding system: %v", err)
	}
	if len(listA) != 1 || listA[0].Name != "a" {
		t.Errorf("excluding system: %+v", listA)
	}

	listAWithSys, err := repo.ListByProject(ctx, pidA, true)
	if err != nil {
		t.Fatalf("list including system: %v", err)
	}
	if len(listAWithSys) != 2 {
		t.Fatalf("including system: got %d want 2", len(listAWithSys))
	}
	names := map[string]bool{}
	for _, s := range listAWithSys {
		names[s.Name] = true
	}
	if !names["a"] || !names["system:s"] {
		t.Errorf("expected both a and system:s, got %+v", names)
	}
	// Other project's row must not appear under pidA.
	if names["b"] {
		t.Errorf("project A list leaked project B's row")
	}
}

func TestQianchuanStrategyRepoMaxVersion(t *testing.T) {
	ctx := context.Background()
	repo := newTestStrategyRepo(t)
	pid := uuid.New()

	if v, err := repo.MaxVersion(ctx, &pid, "x"); err != nil || v != 0 {
		t.Fatalf("max version on empty: v=%d err=%v", v, err)
	}
	if err := repo.Insert(ctx, &strategy.QianchuanStrategy{
		ProjectID: &pid, Name: "x", YAMLSource: "y", ParsedSpec: "{}",
		Version: 1, Status: strategy.StatusPublished, CreatedBy: uuid.New(),
	}); err != nil {
		t.Fatalf("insert v1: %v", err)
	}
	if err := repo.Insert(ctx, &strategy.QianchuanStrategy{
		ProjectID: &pid, Name: "x", YAMLSource: "y", ParsedSpec: "{}",
		Version: 3, Status: strategy.StatusDraft, CreatedBy: uuid.New(),
	}); err != nil {
		t.Fatalf("insert v3: %v", err)
	}
	v, err := repo.MaxVersion(ctx, &pid, "x")
	if err != nil {
		t.Fatalf("max version: %v", err)
	}
	if v != 3 {
		t.Errorf("max version: got %d want 3", v)
	}
}

func TestQianchuanStrategyRepoUpdateDraftAndStatus(t *testing.T) {
	ctx := context.Background()
	repo := newTestStrategyRepo(t)
	pid := uuid.New()
	row := &strategy.QianchuanStrategy{
		ProjectID: &pid, Name: "x", YAMLSource: "old", ParsedSpec: "{}",
		Version: 1, Status: strategy.StatusDraft, CreatedBy: uuid.New(),
	}
	if err := repo.Insert(ctx, row); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := repo.UpdateDraft(ctx, row.ID, "desc2", "new", `{"v":1}`); err != nil {
		t.Fatalf("UpdateDraft: %v", err)
	}
	got, _ := repo.GetByID(ctx, row.ID)
	if got.YAMLSource != "new" || got.Description != "desc2" {
		t.Errorf("update applied incorrectly: %+v", got)
	}

	if err := repo.SetStatus(ctx, row.ID, strategy.StatusPublished); err != nil {
		t.Fatalf("SetStatus published: %v", err)
	}
	got, _ = repo.GetByID(ctx, row.ID)
	if got.Status != strategy.StatusPublished {
		t.Fatalf("status not set: %+v", got)
	}

	// Update on a non-draft row must surface ErrNotFound.
	err := repo.UpdateDraft(ctx, row.ID, "desc3", "x", "{}")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateDraft on published: got %v want ErrNotFound", err)
	}

	// Delete draft after we transitioned to published also fails.
	err = repo.DeleteDraft(ctx, row.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteDraft on published: got %v want ErrNotFound", err)
	}
}

func TestQianchuanStrategyRepoFindByProjectAndName(t *testing.T) {
	ctx := context.Background()
	repo := newTestStrategyRepo(t)
	pid := uuid.New()

	for v := 1; v <= 3; v++ {
		if err := repo.Insert(ctx, &strategy.QianchuanStrategy{
			ProjectID: &pid, Name: "x", YAMLSource: "y", ParsedSpec: "{}",
			Version: v, Status: strategy.StatusDraft, CreatedBy: uuid.New(),
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	rows, err := repo.FindByProjectAndName(ctx, &pid, "x")
	if err != nil {
		t.Fatalf("FindByProjectAndName: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len: got %d want 3", len(rows))
	}
	for i, r := range rows {
		if r.Version != i+1 {
			t.Errorf("rows[%d].Version: got %d want %d", i, r.Version, i+1)
		}
	}
}
