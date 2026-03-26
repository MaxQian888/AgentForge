package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestNewCustomFieldRepository(t *testing.T) {
	repo := NewCustomFieldRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil CustomFieldRepository")
	}
}

func TestCustomFieldRepositoryCreateDefinitionNilDB(t *testing.T) {
	repo := NewCustomFieldRepository(nil)
	err := repo.CreateDefinition(context.Background(), &model.CustomFieldDefinition{ID: uuid.New(), ProjectID: uuid.New()})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("CreateDefinition() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestCustomFieldRepositoryRoundTripDefinitionsAndValues(t *testing.T) {
	ctx := context.Background()
	repo := NewCustomFieldRepository(openFoundationRepoTestDB(t, &customFieldDefinitionRecord{}, &customFieldValueRecord{}))

	projectID := uuid.New()
	taskID := uuid.New()
	fieldID := uuid.New()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)

	definition := &model.CustomFieldDefinition{
		ID:        fieldID,
		ProjectID: projectID,
		Name:      "Priority",
		FieldType: model.CustomFieldTypeSelect,
		Options:   `["P0","P1","P2"]`,
		SortOrder: 2,
		Required:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateDefinition(ctx, definition); err != nil {
		t.Fatalf("CreateDefinition() error = %v", err)
	}

	definitions, err := repo.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("len(definitions) = %d, want 1", len(definitions))
	}
	if definitions[0].Name != "Priority" || definitions[0].Options != `["P0","P1","P2"]` {
		t.Fatalf("unexpected definition: %+v", definitions[0])
	}

	value := &model.CustomFieldValue{
		ID:         uuid.New(),
		TaskID:     taskID,
		FieldDefID: fieldID,
		Value:      `"P1"`,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := repo.SetValue(ctx, value); err != nil {
		t.Fatalf("SetValue() error = %v", err)
	}

	value.Value = `"P0"`
	if err := repo.SetValue(ctx, value); err != nil {
		t.Fatalf("SetValue() update error = %v", err)
	}

	values, err := repo.ListValuesByTask(ctx, taskID)
	if err != nil {
		t.Fatalf("ListValuesByTask() error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("len(values) = %d, want 1", len(values))
	}
	if values[0].Value != `"P0"` {
		t.Fatalf("values[0].Value = %s, want %s", values[0].Value, `"P0"`)
	}

	if err := repo.ClearValue(ctx, taskID, fieldID); err != nil {
		t.Fatalf("ClearValue() error = %v", err)
	}
	values, err = repo.ListValuesByTask(ctx, taskID)
	if err != nil {
		t.Fatalf("ListValuesByTask() after clear error = %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("len(values) after clear = %d, want 0", len(values))
	}
}
