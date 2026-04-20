package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestNewEntityLinkRepository(t *testing.T) {
	repo := NewEntityLinkRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil EntityLinkRepository")
	}
}

func TestEntityLinkRepositoryCreateNilDB(t *testing.T) {
	repo := NewEntityLinkRepository(nil)
	err := repo.Create(context.Background(), &model.EntityLink{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		SourceID:  uuid.New(),
		TargetID:  uuid.New(),
		CreatedBy: uuid.New(),
	})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestEntityLinkRecordRoundTrip(t *testing.T) {
	anchorBlockID := "block-42"
	deletedAt := mustParseTime(t, "2026-03-26T10:16:00Z")
	link := &model.EntityLink{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		SourceType:    model.EntityTypeTask,
		SourceID:      uuid.New(),
		TargetType:    model.EntityTypeWikiPage,
		TargetID:      uuid.New(),
		LinkType:      model.EntityLinkTypeRequirement,
		AnchorBlockID: &anchorBlockID,
		CreatedBy:     uuid.New(),
		CreatedAt:     mustParseTime(t, "2026-03-26T10:15:00Z"),
		DeletedAt:     &deletedAt,
	}

	record := newEntityLinkRecord(link)
	result := record.toModel()

	if result.ID != link.ID || result.ProjectID != link.ProjectID {
		t.Fatalf("round trip ids mismatch: got %+v want %+v", result, link)
	}
	if result.SourceType != link.SourceType || result.TargetType != link.TargetType || result.LinkType != link.LinkType {
		t.Fatalf("round trip types mismatch: got %+v want %+v", result, link)
	}
	if result.AnchorBlockID == nil || *result.AnchorBlockID != anchorBlockID {
		t.Fatalf("AnchorBlockID = %v, want %q", result.AnchorBlockID, anchorBlockID)
	}
	if result.DeletedAt == nil || !result.DeletedAt.Equal(deletedAt) {
		t.Fatalf("DeletedAt = %v, want %v", result.DeletedAt, deletedAt)
	}
}

func TestEntityLinkRepositoryCreateListDelete(t *testing.T) {
	ctx := context.Background()
	repo := NewEntityLinkRepository(openEntityLinkRepoTestDB(t))

	projectID := uuid.New()
	sourceID := uuid.New()
	targetID := uuid.New()
	createdBy := uuid.New()

	link := &model.EntityLink{
		ID:         uuid.New(),
		ProjectID:  projectID,
		SourceType: model.EntityTypeTask,
		SourceID:   sourceID,
		TargetType: model.EntityTypeWikiPage,
		TargetID:   targetID,
		LinkType:   model.EntityLinkTypeRequirement,
		CreatedBy:  createdBy,
	}

	if err := repo.Create(ctx, link); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	bySource, err := repo.ListBySource(ctx, projectID, model.EntityTypeTask, sourceID)
	if err != nil {
		t.Fatalf("ListBySource() error = %v", err)
	}
	if len(bySource) != 1 || bySource[0].ID != link.ID {
		t.Fatalf("ListBySource() = %+v, want [%s]", bySource, link.ID)
	}

	byTarget, err := repo.ListByTarget(ctx, projectID, model.EntityTypeWikiPage, targetID)
	if err != nil {
		t.Fatalf("ListByTarget() error = %v", err)
	}
	if len(byTarget) != 1 || byTarget[0].ID != link.ID {
		t.Fatalf("ListByTarget() = %+v, want [%s]", byTarget, link.ID)
	}

	if err := repo.Delete(ctx, link.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	afterDelete, err := repo.ListBySource(ctx, projectID, model.EntityTypeTask, sourceID)
	if err != nil {
		t.Fatalf("ListBySource() after delete error = %v", err)
	}
	if len(afterDelete) != 0 {
		t.Fatalf("len(ListBySource() after delete) = %d, want 0", len(afterDelete))
	}
}

func TestEntityLinkRepositoryUpsertAndDeleteMentionLinks(t *testing.T) {
	ctx := context.Background()
	repo := NewEntityLinkRepository(openEntityLinkRepoTestDB(t))

	projectID := uuid.New()
	sourceID := uuid.New()
	createdBy := uuid.New()
	targetA := model.EntityLinkTarget{EntityType: model.EntityTypeTask, EntityID: uuid.New()}
	targetB := model.EntityLinkTarget{EntityType: model.EntityTypeWikiPage, EntityID: uuid.New()}

	if err := repo.UpsertMentionLinks(ctx, projectID, model.EntityTypeTask, sourceID, createdBy, []model.EntityLinkTarget{targetA, targetB}); err != nil {
		t.Fatalf("UpsertMentionLinks() error = %v", err)
	}
	if err := repo.UpsertMentionLinks(ctx, projectID, model.EntityTypeTask, sourceID, createdBy, []model.EntityLinkTarget{targetA, targetB}); err != nil {
		t.Fatalf("UpsertMentionLinks() second pass error = %v", err)
	}

	links, err := repo.ListBySource(ctx, projectID, model.EntityTypeTask, sourceID)
	if err != nil {
		t.Fatalf("ListBySource() error = %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("len(ListBySource()) = %d, want 2", len(links))
	}
	for _, link := range links {
		if link.LinkType != model.EntityLinkTypeMention {
			t.Fatalf("LinkType = %q, want %q", link.LinkType, model.EntityLinkTypeMention)
		}
	}

	if err := repo.DeleteMentionLinksForSource(ctx, projectID, model.EntityTypeTask, sourceID); err != nil {
		t.Fatalf("DeleteMentionLinksForSource() error = %v", err)
	}

	afterDelete, err := repo.ListBySource(ctx, projectID, model.EntityTypeTask, sourceID)
	if err != nil {
		t.Fatalf("ListBySource() after mention delete error = %v", err)
	}
	if len(afterDelete) != 0 {
		t.Fatalf("len(ListBySource() after mention delete) = %d, want 0", len(afterDelete))
	}
}

func openEntityLinkRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(&entityLinkRecord{}); err != nil {
		t.Fatalf("migrate entity links table: %v", err)
	}
	return db
}

func mustParseTime(t *testing.T, raw string) time.Time {
	t.Helper()

	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse time %q: %v", raw, err)
	}
	return value
}
