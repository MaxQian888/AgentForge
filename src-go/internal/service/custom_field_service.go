package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type customFieldRepository interface {
	CreateDefinition(ctx context.Context, definition *model.CustomFieldDefinition) error
	GetDefinition(ctx context.Context, id uuid.UUID) (*model.CustomFieldDefinition, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.CustomFieldDefinition, error)
	UpdateDefinition(ctx context.Context, definition *model.CustomFieldDefinition) error
	DeleteDefinition(ctx context.Context, id uuid.UUID) error
	SetValue(ctx context.Context, value *model.CustomFieldValue) error
	ClearValue(ctx context.Context, taskID uuid.UUID, fieldDefID uuid.UUID) error
	ListValuesByTask(ctx context.Context, taskID uuid.UUID) ([]*model.CustomFieldValue, error)
}

type CustomFieldService struct {
	repo customFieldRepository
	now  func() time.Time
}

func NewCustomFieldService(repo customFieldRepository) *CustomFieldService {
	return &CustomFieldService{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *CustomFieldService) CreateField(ctx context.Context, definition *model.CustomFieldDefinition) error {
	if definition.ID == uuid.Nil {
		definition.ID = uuid.New()
	}
	existing, err := s.repo.ListByProject(ctx, definition.ProjectID)
	if err != nil {
		return fmt.Errorf("list custom fields: %w", err)
	}
	if definition.SortOrder == 0 {
		maxSortOrder := 0
		for _, item := range existing {
			if item.SortOrder > maxSortOrder {
				maxSortOrder = item.SortOrder
			}
		}
		definition.SortOrder = maxSortOrder + 1
	}
	now := s.now()
	if definition.CreatedAt.IsZero() {
		definition.CreatedAt = now
	}
	definition.UpdatedAt = now
	return s.repo.CreateDefinition(ctx, definition)
}

func (s *CustomFieldService) GetField(ctx context.Context, id uuid.UUID) (*model.CustomFieldDefinition, error) {
	return s.repo.GetDefinition(ctx, id)
}

func (s *CustomFieldService) ListFields(ctx context.Context, projectID uuid.UUID) ([]*model.CustomFieldDefinition, error) {
	return s.repo.ListByProject(ctx, projectID)
}

func (s *CustomFieldService) UpdateField(ctx context.Context, definition *model.CustomFieldDefinition) error {
	definition.UpdatedAt = s.now()
	return s.repo.UpdateDefinition(ctx, definition)
}

func (s *CustomFieldService) DeleteField(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteDefinition(ctx, id)
}

func (s *CustomFieldService) ReorderFields(ctx context.Context, projectID uuid.UUID, orderedIDs []uuid.UUID) error {
	definitions, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("list custom fields: %w", err)
	}
	byID := make(map[uuid.UUID]*model.CustomFieldDefinition, len(definitions))
	for _, definition := range definitions {
		byID[definition.ID] = definition
	}
	for index, id := range orderedIDs {
		definition, ok := byID[id]
		if !ok {
			return fmt.Errorf("custom field %s not found in project", id)
		}
		definition.SortOrder = index + 1
		definition.UpdatedAt = s.now()
		if err := s.repo.UpdateDefinition(ctx, definition); err != nil {
			return fmt.Errorf("update custom field order: %w", err)
		}
	}
	return nil
}

func (s *CustomFieldService) SetValue(ctx context.Context, value *model.CustomFieldValue) error {
	if _, err := s.repo.GetDefinition(ctx, value.FieldDefID); err != nil {
		return fmt.Errorf("load custom field definition: %w", err)
	}
	if value.ID == uuid.Nil {
		value.ID = uuid.New()
	}
	now := s.now()
	if value.CreatedAt.IsZero() {
		value.CreatedAt = now
	}
	value.UpdatedAt = now
	return s.repo.SetValue(ctx, value)
}

func (s *CustomFieldService) ClearValue(ctx context.Context, taskID uuid.UUID, fieldDefID uuid.UUID) error {
	return s.repo.ClearValue(ctx, taskID, fieldDefID)
}

func (s *CustomFieldService) GetValuesForTask(ctx context.Context, taskID uuid.UUID) ([]*model.CustomFieldValue, error) {
	return s.repo.ListValuesByTask(ctx, taskID)
}

func (s *CustomFieldService) ValidateRequiredFields(ctx context.Context, projectID uuid.UUID, values map[uuid.UUID]string) error {
	definitions, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("list custom fields: %w", err)
	}
	missing := make([]string, 0)
	for _, definition := range definitions {
		if !definition.Required {
			continue
		}
		raw := strings.TrimSpace(values[definition.ID])
		if raw == "" || raw == "null" || raw == `""` {
			missing = append(missing, definition.Name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required custom fields: %s", strings.Join(missing, ", "))
	}
	return nil
}
