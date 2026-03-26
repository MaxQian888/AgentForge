package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeCustomFieldRepo struct {
	definitionsByProject map[uuid.UUID][]*model.CustomFieldDefinition
	definitionsByID      map[uuid.UUID]*model.CustomFieldDefinition
	valuesByTask         map[uuid.UUID][]*model.CustomFieldValue
	updatedDefinitions   []*model.CustomFieldDefinition
	createdDefinition    *model.CustomFieldDefinition
	setValueInput        *model.CustomFieldValue
	deletedDefinitionID  uuid.UUID
	clearValueTaskID     uuid.UUID
	clearValueFieldID    uuid.UUID
}

func (f *fakeCustomFieldRepo) CreateDefinition(_ context.Context, definition *model.CustomFieldDefinition) error {
	f.createdDefinition = definition
	if f.definitionsByID == nil {
		f.definitionsByID = map[uuid.UUID]*model.CustomFieldDefinition{}
	}
	f.definitionsByID[definition.ID] = definition
	f.definitionsByProject[definition.ProjectID] = append(f.definitionsByProject[definition.ProjectID], definition)
	return nil
}

func (f *fakeCustomFieldRepo) GetDefinition(_ context.Context, id uuid.UUID) (*model.CustomFieldDefinition, error) {
	if def, ok := f.definitionsByID[id]; ok {
		return def, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeCustomFieldRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]*model.CustomFieldDefinition, error) {
	return append([]*model.CustomFieldDefinition(nil), f.definitionsByProject[projectID]...), nil
}

func (f *fakeCustomFieldRepo) UpdateDefinition(_ context.Context, definition *model.CustomFieldDefinition) error {
	f.updatedDefinitions = append(f.updatedDefinitions, definition)
	f.definitionsByID[definition.ID] = definition
	return nil
}

func (f *fakeCustomFieldRepo) DeleteDefinition(_ context.Context, id uuid.UUID) error {
	f.deletedDefinitionID = id
	return nil
}

func (f *fakeCustomFieldRepo) SetValue(_ context.Context, value *model.CustomFieldValue) error {
	f.setValueInput = value
	f.valuesByTask[value.TaskID] = []*model.CustomFieldValue{value}
	return nil
}

func (f *fakeCustomFieldRepo) ClearValue(_ context.Context, taskID uuid.UUID, fieldDefID uuid.UUID) error {
	f.clearValueTaskID = taskID
	f.clearValueFieldID = fieldDefID
	delete(f.valuesByTask, taskID)
	return nil
}

func (f *fakeCustomFieldRepo) ListValuesByTask(_ context.Context, taskID uuid.UUID) ([]*model.CustomFieldValue, error) {
	return append([]*model.CustomFieldValue(nil), f.valuesByTask[taskID]...), nil
}

func TestCustomFieldServiceCreateFieldAssignsIDAndSortOrder(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeCustomFieldRepo{
		definitionsByProject: map[uuid.UUID][]*model.CustomFieldDefinition{
			projectID: {{
				ID:        uuid.New(),
				ProjectID: projectID,
				Name:      "Existing",
				SortOrder: 3,
			}},
		},
		definitionsByID: map[uuid.UUID]*model.CustomFieldDefinition{},
		valuesByTask:    map[uuid.UUID][]*model.CustomFieldValue{},
	}

	service := NewCustomFieldService(repo)
	definition := &model.CustomFieldDefinition{
		ProjectID: projectID,
		Name:      "Priority",
		FieldType: model.CustomFieldTypeSelect,
		Options:   `["P0","P1"]`,
	}
	if err := service.CreateField(context.Background(), definition); err != nil {
		t.Fatalf("CreateField() error = %v", err)
	}
	if definition.ID == uuid.Nil {
		t.Fatal("expected CreateField to assign an ID")
	}
	if definition.SortOrder != 4 {
		t.Fatalf("definition.SortOrder = %d, want 4", definition.SortOrder)
	}
}

func TestCustomFieldServiceReorderFieldsUpdatesSortOrder(t *testing.T) {
	projectID := uuid.New()
	first := &model.CustomFieldDefinition{ID: uuid.New(), ProjectID: projectID, SortOrder: 1}
	second := &model.CustomFieldDefinition{ID: uuid.New(), ProjectID: projectID, SortOrder: 2}
	repo := &fakeCustomFieldRepo{
		definitionsByProject: map[uuid.UUID][]*model.CustomFieldDefinition{projectID: {first, second}},
		definitionsByID:      map[uuid.UUID]*model.CustomFieldDefinition{first.ID: first, second.ID: second},
		valuesByTask:         map[uuid.UUID][]*model.CustomFieldValue{},
	}
	service := NewCustomFieldService(repo)

	if err := service.ReorderFields(context.Background(), projectID, []uuid.UUID{second.ID, first.ID}); err != nil {
		t.Fatalf("ReorderFields() error = %v", err)
	}
	if len(repo.updatedDefinitions) != 2 {
		t.Fatalf("len(updatedDefinitions) = %d, want 2", len(repo.updatedDefinitions))
	}
	if repo.updatedDefinitions[0].SortOrder != 1 || repo.updatedDefinitions[1].SortOrder != 2 {
		t.Fatalf("unexpected reorder results: %+v", repo.updatedDefinitions)
	}
}

func TestCustomFieldServiceValidateRequiredFields(t *testing.T) {
	projectID := uuid.New()
	requiredID := uuid.New()
	repo := &fakeCustomFieldRepo{
		definitionsByProject: map[uuid.UUID][]*model.CustomFieldDefinition{
			projectID: {{
				ID:        requiredID,
				ProjectID: projectID,
				Name:      "Risk",
				Required:  true,
			}},
		},
		definitionsByID: map[uuid.UUID]*model.CustomFieldDefinition{},
		valuesByTask:    map[uuid.UUID][]*model.CustomFieldValue{},
	}
	service := NewCustomFieldService(repo)

	if err := service.ValidateRequiredFields(context.Background(), projectID, map[uuid.UUID]string{}); err == nil {
		t.Fatal("expected missing required field error")
	}
	if err := service.ValidateRequiredFields(context.Background(), projectID, map[uuid.UUID]string{requiredID: `"High"`}); err != nil {
		t.Fatalf("ValidateRequiredFields() with value error = %v", err)
	}
}

func TestCustomFieldServiceSetAndClearValue(t *testing.T) {
	fieldID := uuid.New()
	taskID := uuid.New()
	repo := &fakeCustomFieldRepo{
		definitionsByProject: map[uuid.UUID][]*model.CustomFieldDefinition{},
		definitionsByID: map[uuid.UUID]*model.CustomFieldDefinition{
			fieldID: {ID: fieldID, Name: "Priority"},
		},
		valuesByTask: map[uuid.UUID][]*model.CustomFieldValue{},
	}
	service := NewCustomFieldService(repo)

	if err := service.SetValue(context.Background(), &model.CustomFieldValue{TaskID: taskID, FieldDefID: fieldID, Value: `"P0"`}); err != nil {
		t.Fatalf("SetValue() error = %v", err)
	}
	if repo.setValueInput == nil || repo.setValueInput.ID == uuid.Nil {
		t.Fatal("expected SetValue to assign an ID")
	}
	if err := service.ClearValue(context.Background(), taskID, fieldID); err != nil {
		t.Fatalf("ClearValue() error = %v", err)
	}
	if repo.clearValueTaskID != taskID || repo.clearValueFieldID != fieldID {
		t.Fatalf("unexpected clear call task=%s field=%s", repo.clearValueTaskID, repo.clearValueFieldID)
	}
}
