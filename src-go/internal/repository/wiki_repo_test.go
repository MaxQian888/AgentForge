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

func TestNewWikiRepositories(t *testing.T) {
	if NewWikiSpaceRepository(nil) == nil {
		t.Fatal("expected non-nil WikiSpaceRepository")
	}
	if NewWikiPageRepository(nil) == nil {
		t.Fatal("expected non-nil WikiPageRepository")
	}
	if NewPageVersionRepository(nil) == nil {
		t.Fatal("expected non-nil PageVersionRepository")
	}
	if NewPageCommentRepository(nil) == nil {
		t.Fatal("expected non-nil PageCommentRepository")
	}
	if NewPageFavoriteRepository(nil) == nil {
		t.Fatal("expected non-nil PageFavoriteRepository")
	}
	if NewPageRecentAccessRepository(nil) == nil {
		t.Fatal("expected non-nil PageRecentAccessRepository")
	}
}

func TestWikiRepositoriesNilDB(t *testing.T) {
	ctx := context.Background()
	spaceRepo := NewWikiSpaceRepository(nil)
	pageRepo := NewWikiPageRepository(nil)
	versionRepo := NewPageVersionRepository(nil)
	commentRepo := NewPageCommentRepository(nil)
	favoriteRepo := NewPageFavoriteRepository(nil)
	recentRepo := NewPageRecentAccessRepository(nil)

	if err := spaceRepo.Create(ctx, &model.WikiSpace{ID: uuid.New(), ProjectID: uuid.New()}); err != ErrDatabaseUnavailable {
		t.Fatalf("WikiSpaceRepository.Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
	if _, err := spaceRepo.GetByProjectID(ctx, uuid.New()); err != ErrDatabaseUnavailable {
		t.Fatalf("WikiSpaceRepository.GetByProjectID() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
	if err := pageRepo.Create(ctx, &model.WikiPage{ID: uuid.New(), SpaceID: uuid.New()}); err != ErrDatabaseUnavailable {
		t.Fatalf("WikiPageRepository.Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
	if err := versionRepo.Create(ctx, &model.PageVersion{ID: uuid.New(), PageID: uuid.New()}); err != ErrDatabaseUnavailable {
		t.Fatalf("PageVersionRepository.Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
	if err := commentRepo.Create(ctx, &model.PageComment{ID: uuid.New(), PageID: uuid.New()}); err != ErrDatabaseUnavailable {
		t.Fatalf("PageCommentRepository.Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
	if err := favoriteRepo.Add(ctx, uuid.New(), uuid.New()); err != ErrDatabaseUnavailable {
		t.Fatalf("PageFavoriteRepository.Add() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
	if err := recentRepo.Touch(ctx, uuid.New(), uuid.New(), time.Now().UTC()); err != ErrDatabaseUnavailable {
		t.Fatalf("PageRecentAccessRepository.Touch() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestWikiSpaceRepositoryCreateGetDelete(t *testing.T) {
	ctx := context.Background()
	db := openWikiRepoTestDB(t)
	repo := NewWikiSpaceRepository(db)

	space := &model.WikiSpace{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		CreatedAt: time.Date(2026, 3, 26, 11, 0, 0, 0, time.UTC),
	}

	if err := repo.Create(ctx, space); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	stored, err := repo.GetByProjectID(ctx, space.ProjectID)
	if err != nil {
		t.Fatalf("GetByProjectID() error = %v", err)
	}
	if stored.ID != space.ID {
		t.Fatalf("stored space id = %s, want %s", stored.ID, space.ID)
	}

	if err := repo.Delete(ctx, space.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := repo.GetByProjectID(ctx, space.ProjectID); err != ErrNotFound {
		t.Fatalf("GetByProjectID() after delete error = %v, want %v", err, ErrNotFound)
	}
}

func TestWikiPageRepositoryCreateListMoveUpdateAndSoftDelete(t *testing.T) {
	ctx := context.Background()
	db := openWikiRepoTestDB(t)
	repo := NewWikiPageRepository(db)

	spaceID := uuid.New()
	root := &model.WikiPage{
		ID:          uuid.New(),
		SpaceID:     spaceID,
		Title:       "Root",
		Content:     `[]`,
		ContentText: "Root",
		Path:        "/" + uuid.NewString(),
		SortOrder:   0,
		CreatedAt:   time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
	}
	child := &model.WikiPage{
		ID:          uuid.New(),
		SpaceID:     spaceID,
		ParentID:    &root.ID,
		Title:       "Child",
		Content:     `[{"type":"paragraph"}]`,
		ContentText: "Child",
		Path:        root.Path + "/" + uuid.NewString(),
		SortOrder:   1,
		CreatedAt:   time.Date(2026, 3, 26, 12, 1, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 3, 26, 12, 1, 0, 0, time.UTC),
	}

	if err := repo.Create(ctx, root); err != nil {
		t.Fatalf("Create(root) error = %v", err)
	}
	if err := repo.Create(ctx, child); err != nil {
		t.Fatalf("Create(child) error = %v", err)
	}

	pages, err := repo.ListTree(ctx, spaceID)
	if err != nil {
		t.Fatalf("ListTree() error = %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("len(ListTree()) = %d, want 2", len(pages))
	}

	children, err := repo.ListByParent(ctx, spaceID, &root.ID)
	if err != nil {
		t.Fatalf("ListByParent() error = %v", err)
	}
	if len(children) != 1 || children[0].ID != child.ID {
		t.Fatalf("ListByParent() = %+v, want child %s", children, child.ID)
	}

	child.Title = "Child Updated"
	child.ContentText = "Child Updated"
	child.UpdatedAt = time.Date(2026, 3, 26, 12, 2, 0, 0, time.UTC)
	if err := repo.Update(ctx, child); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if err := repo.MovePage(ctx, child.ID, nil, "/"+child.ID.String(), 0); err != nil {
		t.Fatalf("MovePage() error = %v", err)
	}
	if err := repo.UpdateSortOrder(ctx, root.ID, 2); err != nil {
		t.Fatalf("UpdateSortOrder() error = %v", err)
	}

	storedChild, err := repo.GetByID(ctx, child.ID)
	if err != nil {
		t.Fatalf("GetByID(child) error = %v", err)
	}
	if storedChild.ParentID != nil {
		t.Fatalf("stored child parent_id = %v, want nil", storedChild.ParentID)
	}
	if storedChild.Path != "/"+child.ID.String() {
		t.Fatalf("stored child path = %q, want %q", storedChild.Path, "/"+child.ID.String())
	}
	if storedChild.Title != "Child Updated" {
		t.Fatalf("stored child title = %q, want %q", storedChild.Title, "Child Updated")
	}

	storedRoot, err := repo.GetByID(ctx, root.ID)
	if err != nil {
		t.Fatalf("GetByID(root) error = %v", err)
	}
	if storedRoot.SortOrder != 2 {
		t.Fatalf("stored root sort order = %d, want 2", storedRoot.SortOrder)
	}

	if err := repo.SoftDelete(ctx, child.ID); err != nil {
		t.Fatalf("SoftDelete() error = %v", err)
	}
	if _, err := repo.GetByID(ctx, child.ID); err != ErrNotFound {
		t.Fatalf("GetByID() after SoftDelete error = %v, want %v", err, ErrNotFound)
	}
}

func TestPageVersionRepositoryCreateListAndGet(t *testing.T) {
	ctx := context.Background()
	db := openWikiRepoTestDB(t)
	repo := NewPageVersionRepository(db)

	pageID := uuid.New()
	v1 := &model.PageVersion{
		ID:            uuid.New(),
		PageID:        pageID,
		VersionNumber: 1,
		Name:          "v1",
		Content:       `[]`,
		CreatedAt:     time.Date(2026, 3, 26, 13, 0, 0, 0, time.UTC),
	}
	v2 := &model.PageVersion{
		ID:            uuid.New(),
		PageID:        pageID,
		VersionNumber: 2,
		Name:          "v2",
		Content:       `[{"type":"paragraph"}]`,
		CreatedAt:     time.Date(2026, 3, 26, 13, 5, 0, 0, time.UTC),
	}

	if err := repo.Create(ctx, v1); err != nil {
		t.Fatalf("Create(v1) error = %v", err)
	}
	if err := repo.Create(ctx, v2); err != nil {
		t.Fatalf("Create(v2) error = %v", err)
	}

	versions, err := repo.ListByPageID(ctx, pageID)
	if err != nil {
		t.Fatalf("ListByPageID() error = %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("len(ListByPageID()) = %d, want 2", len(versions))
	}
	if versions[0].VersionNumber != 2 || versions[1].VersionNumber != 1 {
		t.Fatalf("version order = [%d %d], want [2 1]", versions[0].VersionNumber, versions[1].VersionNumber)
	}

	stored, err := repo.GetByID(ctx, v1.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.Name != "v1" {
		t.Fatalf("stored version name = %q, want %q", stored.Name, "v1")
	}
}

func TestPageCommentRepositoryCreateListUpdateAndSoftDelete(t *testing.T) {
	ctx := context.Background()
	db := openWikiRepoTestDB(t)
	repo := NewPageCommentRepository(db)

	pageID := uuid.New()
	rootComment := &model.PageComment{
		ID:        uuid.New(),
		PageID:    pageID,
		Body:      "root",
		Mentions:  `[]`,
		CreatedAt: time.Date(2026, 3, 26, 14, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 26, 14, 0, 0, 0, time.UTC),
	}
	reply := &model.PageComment{
		ID:              uuid.New(),
		PageID:          pageID,
		ParentCommentID: &rootComment.ID,
		AnchorBlockID:   stringPointer("block-1"),
		Body:            "reply",
		Mentions:        `["alice"]`,
		CreatedAt:       time.Date(2026, 3, 26, 14, 1, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2026, 3, 26, 14, 1, 0, 0, time.UTC),
	}

	if err := repo.Create(ctx, rootComment); err != nil {
		t.Fatalf("Create(rootComment) error = %v", err)
	}
	if err := repo.Create(ctx, reply); err != nil {
		t.Fatalf("Create(reply) error = %v", err)
	}

	comments, err := repo.ListByPageID(ctx, pageID)
	if err != nil {
		t.Fatalf("ListByPageID() error = %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("len(ListByPageID()) = %d, want 2", len(comments))
	}

	now := time.Date(2026, 3, 26, 14, 2, 0, 0, time.UTC)
	reply.Body = "reply updated"
	reply.ResolvedAt = &now
	reply.UpdatedAt = now
	if err := repo.Update(ctx, reply); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	updated, err := repo.ListByPageID(ctx, pageID)
	if err != nil {
		t.Fatalf("ListByPageID() after update error = %v", err)
	}
	if updated[1].Body != "reply updated" {
		t.Fatalf("updated reply body = %q, want %q", updated[1].Body, "reply updated")
	}
	if updated[1].ResolvedAt == nil || !updated[1].ResolvedAt.Equal(now) {
		t.Fatalf("updated reply resolved_at = %v, want %v", updated[1].ResolvedAt, now)
	}

	if err := repo.SoftDelete(ctx, reply.ID); err != nil {
		t.Fatalf("SoftDelete() error = %v", err)
	}
	remaining, err := repo.ListByPageID(ctx, pageID)
	if err != nil {
		t.Fatalf("ListByPageID() after delete error = %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != rootComment.ID {
		t.Fatalf("remaining comments = %+v, want only root comment", remaining)
	}
}

func TestFavoriteAndRecentAccessRepositories(t *testing.T) {
	ctx := context.Background()
	db := openWikiRepoTestDB(t)
	favoriteRepo := NewPageFavoriteRepository(db)
	recentRepo := NewPageRecentAccessRepository(db)

	userID := uuid.New()
	pageID := uuid.New()
	otherPageID := uuid.New()

	if err := favoriteRepo.Add(ctx, pageID, userID); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := favoriteRepo.Add(ctx, pageID, userID); err != nil {
		t.Fatalf("Add() duplicate error = %v", err)
	}

	favorites, err := favoriteRepo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(favorites) != 1 || favorites[0].PageID != pageID {
		t.Fatalf("favorites = %+v, want one favorite for page %s", favorites, pageID)
	}
	byPage, err := favoriteRepo.ListByPage(ctx, pageID)
	if err != nil {
		t.Fatalf("ListByPage() error = %v", err)
	}
	if len(byPage) != 1 || byPage[0].UserID != userID {
		t.Fatalf("favorites by page = %+v, want one favorite for user %s", byPage, userID)
	}

	if err := favoriteRepo.Remove(ctx, pageID, userID); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	favorites, err = favoriteRepo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser() after remove error = %v", err)
	}
	if len(favorites) != 0 {
		t.Fatalf("favorites after remove = %d, want 0", len(favorites))
	}

	firstAccess := time.Date(2026, 3, 26, 15, 0, 0, 0, time.UTC)
	secondAccess := firstAccess.Add(5 * time.Minute)
	if err := recentRepo.Touch(ctx, pageID, userID, firstAccess); err != nil {
		t.Fatalf("Touch(first) error = %v", err)
	}
	if err := recentRepo.Touch(ctx, otherPageID, userID, secondAccess); err != nil {
		t.Fatalf("Touch(second) error = %v", err)
	}
	if err := recentRepo.Touch(ctx, pageID, userID, secondAccess.Add(5*time.Minute)); err != nil {
		t.Fatalf("Touch(update existing) error = %v", err)
	}

	recent, err := recentRepo.ListByUser(ctx, userID, 10)
	if err != nil {
		t.Fatalf("ListByUser() recent error = %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("len(recent) = %d, want 2", len(recent))
	}
	if recent[0].PageID != pageID {
		t.Fatalf("most recent page = %s, want %s", recent[0].PageID, pageID)
	}
	if !recent[0].AccessedAt.After(recent[1].AccessedAt) {
		t.Fatalf("recent order incorrect: %v then %v", recent[0].AccessedAt, recent[1].AccessedAt)
	}
}

func openWikiRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(
		&wikiSpaceRecord{},
		&wikiPageRecord{},
		&pageVersionRecord{},
		&pageCommentRecord{},
		&pageFavoriteRecord{},
		&pageRecentAccessRecord{},
	); err != nil {
		t.Fatalf("migrate wiki records: %v", err)
	}
	return db
}

func stringPointer(value string) *string {
	return &value
}
