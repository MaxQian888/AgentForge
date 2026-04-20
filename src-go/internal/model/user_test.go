package model_test

import (
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestUser_ToDTO(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	user := &model.User{
		ID:        id,
		Email:     "test@example.com",
		Password:  "hashed-password",
		Name:      "Test User",
		CreatedAt: now,
		UpdatedAt: now,
	}

	dto := user.ToDTO()

	if dto.ID != id.String() {
		t.Errorf("expected ID %s, got %s", id.String(), dto.ID)
	}
	if dto.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", dto.Email)
	}
	if dto.Name != "Test User" {
		t.Errorf("expected name Test User, got %s", dto.Name)
	}
	if !dto.CreatedAt.Equal(now) {
		t.Errorf("expected CreatedAt %v, got %v", now, dto.CreatedAt)
	}
}

func TestUser_ToDTO_EmptyFields(t *testing.T) {
	user := &model.User{}
	dto := user.ToDTO()

	if dto.ID != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("expected zero UUID, got %s", dto.ID)
	}
	if dto.Email != "" {
		t.Errorf("expected empty email, got %s", dto.Email)
	}
}
