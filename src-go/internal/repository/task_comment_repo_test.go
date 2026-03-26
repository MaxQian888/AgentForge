package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

func TestNewTaskCommentRepository(t *testing.T) {
	repo := NewTaskCommentRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil TaskCommentRepository")
	}
}

func TestTaskCommentRepositoryCreateNilDB(t *testing.T) {
	repo := NewTaskCommentRepository(nil)
	err := repo.Create(context.Background(), &model.TaskComment{
		ID:        uuid.New(),
		TaskID:    uuid.New(),
		CreatedBy: uuid.New(),
	})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestTaskCommentRecordRoundTrip(t *testing.T) {
	parentID := uuid.New()
	resolvedAt := mustParseTaskCommentTime(t, "2026-03-26T12:10:00Z")
	deletedAt := mustParseTaskCommentTime(t, "2026-03-26T12:20:00Z")
	comment := &model.TaskComment{
		ID:              uuid.New(),
		TaskID:          uuid.New(),
		ParentCommentID: &parentID,
		Body:            "Looks good.",
		Mentions:        []string{"alice", "bob"},
		ResolvedAt:      &resolvedAt,
		CreatedBy:       uuid.New(),
		CreatedAt:       mustParseTaskCommentTime(t, "2026-03-26T12:00:00Z"),
		UpdatedAt:       mustParseTaskCommentTime(t, "2026-03-26T12:05:00Z"),
		DeletedAt:       &deletedAt,
	}

	record := newTaskCommentRecord(comment)
	result, err := record.toModel()
	if err != nil {
		t.Fatalf("toModel() error = %v", err)
	}

	if result.ID != comment.ID || result.TaskID != comment.TaskID {
		t.Fatalf("round trip ids mismatch: got %+v want %+v", result, comment)
	}
	if len(result.Mentions) != 2 || result.Mentions[0] != "alice" || result.Mentions[1] != "bob" {
		t.Fatalf("Mentions = %v, want [alice bob]", result.Mentions)
	}
	if result.ResolvedAt == nil || !result.ResolvedAt.Equal(resolvedAt) {
		t.Fatalf("ResolvedAt = %v, want %v", result.ResolvedAt, resolvedAt)
	}
	if result.DeletedAt == nil || !result.DeletedAt.Equal(deletedAt) {
		t.Fatalf("DeletedAt = %v, want %v", result.DeletedAt, deletedAt)
	}
}

func TestTaskCommentRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := NewTaskCommentRepository(openTaskCommentRepoTestDB(t))

	taskID := uuid.New()
	comment := &model.TaskComment{
		ID:        uuid.New(),
		TaskID:    taskID,
		Body:      "Initial comment",
		Mentions:  []string{"alice"},
		CreatedBy: uuid.New(),
	}

	if err := repo.Create(ctx, comment); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	loaded, err := repo.GetByID(ctx, comment.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if loaded.Body != "Initial comment" {
		t.Fatalf("Body = %q, want Initial comment", loaded.Body)
	}

	loaded.Body = "Updated comment"
	loaded.Mentions = []string{"alice", "bob"}
	resolvedAt := mustParseTaskCommentTime(t, "2026-03-26T12:30:00Z")
	loaded.ResolvedAt = &resolvedAt
	if err := repo.Update(ctx, loaded); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	updated, err := repo.GetByID(ctx, comment.ID)
	if err != nil {
		t.Fatalf("GetByID() after update error = %v", err)
	}
	if updated.Body != "Updated comment" {
		t.Fatalf("Body after update = %q, want Updated comment", updated.Body)
	}
	if len(updated.Mentions) != 2 {
		t.Fatalf("len(Mentions) = %d, want 2", len(updated.Mentions))
	}
	if updated.ResolvedAt == nil || !updated.ResolvedAt.Equal(resolvedAt) {
		t.Fatalf("ResolvedAt after update = %v, want %v", updated.ResolvedAt, resolvedAt)
	}

	comments, err := repo.ListByTaskID(ctx, taskID)
	if err != nil {
		t.Fatalf("ListByTaskID() error = %v", err)
	}
	if len(comments) != 1 || comments[0].ID != comment.ID {
		t.Fatalf("ListByTaskID() = %+v, want [%s]", comments, comment.ID)
	}

	if err := repo.SoftDelete(ctx, comment.ID); err != nil {
		t.Fatalf("SoftDelete() error = %v", err)
	}

	deleted, err := repo.GetByID(ctx, comment.ID)
	if err != nil {
		t.Fatalf("GetByID() after delete error = %v", err)
	}
	if deleted.DeletedAt == nil {
		t.Fatal("expected DeletedAt to be populated after soft delete")
	}
}

func openTaskCommentRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(&taskCommentRecord{}); err != nil {
		t.Fatalf("migrate task comments table: %v", err)
	}
	return db
}

func mustParseTaskCommentTime(t *testing.T, raw string) time.Time {
	t.Helper()

	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse time %q: %v", raw, err)
	}
	return value
}
