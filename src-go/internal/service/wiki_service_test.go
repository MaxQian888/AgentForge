package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/ws"
	"gorm.io/gorm"
)

type stubWikiSpaceRepo struct {
	spaces map[uuid.UUID]*model.WikiSpace
}

func (r *stubWikiSpaceRepo) Create(_ context.Context, space *model.WikiSpace) error {
	cloned := *space
	if r.spaces == nil {
		r.spaces = make(map[uuid.UUID]*model.WikiSpace)
	}
	r.spaces[space.ID] = &cloned
	return nil
}

func (r *stubWikiSpaceRepo) GetByProjectID(_ context.Context, projectID uuid.UUID) (*model.WikiSpace, error) {
	for _, space := range r.spaces {
		if space.ProjectID == projectID && space.DeletedAt == nil {
			cloned := *space
			return &cloned, nil
		}
	}
	return nil, errors.New("space not found")
}

func (r *stubWikiSpaceRepo) GetByID(_ context.Context, id uuid.UUID) (*model.WikiSpace, error) {
	space, ok := r.spaces[id]
	if !ok || space.DeletedAt != nil {
		return nil, errors.New("space not found")
	}
	cloned := *space
	return &cloned, nil
}

func (r *stubWikiSpaceRepo) Delete(_ context.Context, id uuid.UUID) error {
	space, ok := r.spaces[id]
	if !ok {
		return errors.New("space not found")
	}
	now := time.Now().UTC()
	space.DeletedAt = &now
	return nil
}

type stubWikiPageRepo struct {
	pages map[uuid.UUID]*model.WikiPage
}

func (r *stubWikiPageRepo) Create(_ context.Context, page *model.WikiPage) error {
	cloned := cloneWikiPage(page)
	if r.pages == nil {
		r.pages = make(map[uuid.UUID]*model.WikiPage)
	}
	r.pages[page.ID] = cloned
	return nil
}

func (r *stubWikiPageRepo) GetByID(_ context.Context, id uuid.UUID) (*model.WikiPage, error) {
	page, ok := r.pages[id]
	if !ok || page.DeletedAt != nil {
		return nil, errors.New("page not found")
	}
	return cloneWikiPage(page), nil
}

func (r *stubWikiPageRepo) Update(_ context.Context, page *model.WikiPage) error {
	if _, ok := r.pages[page.ID]; !ok {
		return errors.New("page not found")
	}
	r.pages[page.ID] = cloneWikiPage(page)
	return nil
}

func (r *stubWikiPageRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	page, ok := r.pages[id]
	if !ok {
		return errors.New("page not found")
	}
	now := time.Now().UTC()
	page.DeletedAt = &now
	page.UpdatedAt = now
	return nil
}

func (r *stubWikiPageRepo) ListTree(_ context.Context, spaceID uuid.UUID) ([]*model.WikiPage, error) {
	pages := make([]*model.WikiPage, 0)
	for _, page := range r.pages {
		if page.SpaceID == spaceID && page.DeletedAt == nil {
			pages = append(pages, cloneWikiPage(page))
		}
	}
	sort.Slice(pages, func(i, j int) bool {
		if pages[i].Path == pages[j].Path {
			if pages[i].SortOrder == pages[j].SortOrder {
				return pages[i].CreatedAt.Before(pages[j].CreatedAt)
			}
			return pages[i].SortOrder < pages[j].SortOrder
		}
		return pages[i].Path < pages[j].Path
	})
	return pages, nil
}

func (r *stubWikiPageRepo) ListByParent(_ context.Context, spaceID uuid.UUID, parentID *uuid.UUID) ([]*model.WikiPage, error) {
	pages := make([]*model.WikiPage, 0)
	for _, page := range r.pages {
		if page.SpaceID != spaceID || page.DeletedAt != nil {
			continue
		}
		if parentID == nil && page.ParentID == nil {
			pages = append(pages, cloneWikiPage(page))
			continue
		}
		if parentID != nil && page.ParentID != nil && *page.ParentID == *parentID {
			pages = append(pages, cloneWikiPage(page))
		}
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].SortOrder < pages[j].SortOrder })
	return pages, nil
}

func (r *stubWikiPageRepo) MovePage(_ context.Context, id uuid.UUID, parentID *uuid.UUID, path string, sortOrder int) error {
	page, ok := r.pages[id]
	if !ok {
		return errors.New("page not found")
	}
	page.ParentID = cloneUUIDPointer(parentID)
	page.Path = path
	page.SortOrder = sortOrder
	page.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *stubWikiPageRepo) UpdateSortOrder(_ context.Context, id uuid.UUID, sortOrder int) error {
	page, ok := r.pages[id]
	if !ok {
		return errors.New("page not found")
	}
	page.SortOrder = sortOrder
	page.UpdatedAt = time.Now().UTC()
	return nil
}

type stubPageVersionRepo struct {
	versions map[uuid.UUID]*model.PageVersion
}

func (r *stubPageVersionRepo) Create(_ context.Context, version *model.PageVersion) error {
	cloned := *version
	if r.versions == nil {
		r.versions = make(map[uuid.UUID]*model.PageVersion)
	}
	r.versions[version.ID] = &cloned
	return nil
}

func (r *stubPageVersionRepo) ListByPageID(_ context.Context, pageID uuid.UUID) ([]*model.PageVersion, error) {
	versions := make([]*model.PageVersion, 0)
	for _, version := range r.versions {
		if version.PageID == pageID {
			cloned := *version
			versions = append(versions, &cloned)
		}
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].VersionNumber > versions[j].VersionNumber })
	return versions, nil
}

func (r *stubPageVersionRepo) GetByID(_ context.Context, id uuid.UUID) (*model.PageVersion, error) {
	version, ok := r.versions[id]
	if !ok {
		return nil, errors.New("version not found")
	}
	cloned := *version
	return &cloned, nil
}

type stubPageCommentRepo struct {
	comments map[uuid.UUID]*model.PageComment
}

func (r *stubPageCommentRepo) Create(_ context.Context, comment *model.PageComment) error {
	cloned := clonePageComment(comment)
	if r.comments == nil {
		r.comments = make(map[uuid.UUID]*model.PageComment)
	}
	r.comments[comment.ID] = cloned
	return nil
}

func (r *stubPageCommentRepo) ListByPageID(_ context.Context, pageID uuid.UUID) ([]*model.PageComment, error) {
	comments := make([]*model.PageComment, 0)
	for _, comment := range r.comments {
		if comment.PageID == pageID && comment.DeletedAt == nil {
			comments = append(comments, clonePageComment(comment))
		}
	}
	sort.Slice(comments, func(i, j int) bool { return comments[i].CreatedAt.Before(comments[j].CreatedAt) })
	return comments, nil
}

func (r *stubPageCommentRepo) Update(_ context.Context, comment *model.PageComment) error {
	if _, ok := r.comments[comment.ID]; !ok {
		return errors.New("comment not found")
	}
	r.comments[comment.ID] = clonePageComment(comment)
	return nil
}

func (r *stubPageCommentRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	comment, ok := r.comments[id]
	if !ok {
		return errors.New("comment not found")
	}
	now := time.Now().UTC()
	comment.DeletedAt = &now
	comment.UpdatedAt = now
	return nil
}

type stubPageFavoriteRepo struct {
	favorites map[string]*model.PageFavorite
}

func (r *stubPageFavoriteRepo) Add(_ context.Context, pageID, userID uuid.UUID) error {
	if r.favorites == nil {
		r.favorites = make(map[string]*model.PageFavorite)
	}
	key := pageID.String() + ":" + userID.String()
	r.favorites[key] = &model.PageFavorite{PageID: pageID, UserID: userID, CreatedAt: time.Now().UTC()}
	return nil
}

func (r *stubPageFavoriteRepo) Remove(_ context.Context, pageID, userID uuid.UUID) error {
	delete(r.favorites, pageID.String()+":"+userID.String())
	return nil
}

func (r *stubPageFavoriteRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]*model.PageFavorite, error) {
	favorites := make([]*model.PageFavorite, 0)
	for _, favorite := range r.favorites {
		if favorite.UserID == userID {
			cloned := *favorite
			favorites = append(favorites, &cloned)
		}
	}
	sort.Slice(favorites, func(i, j int) bool { return favorites[i].CreatedAt.After(favorites[j].CreatedAt) })
	return favorites, nil
}

func (r *stubPageFavoriteRepo) ListByPage(_ context.Context, pageID uuid.UUID) ([]*model.PageFavorite, error) {
	favorites := make([]*model.PageFavorite, 0)
	for _, favorite := range r.favorites {
		if favorite.PageID == pageID {
			cloned := *favorite
			favorites = append(favorites, &cloned)
		}
	}
	return favorites, nil
}

type stubWikiIMNotifier struct {
	requests []*model.IMNotifyRequest
}

func (s *stubWikiIMNotifier) Notify(_ context.Context, req *model.IMNotifyRequest) error {
	if req == nil {
		return nil
	}
	cloned := *req
	if req.Metadata != nil {
		cloned.Metadata = map[string]string{}
		for key, value := range req.Metadata {
			cloned.Metadata[key] = value
		}
	}
	s.requests = append(s.requests, &cloned)
	return nil
}

type stubIMEventChannelResolver struct {
	channels []*model.IMChannel
	err      error
}

func (s *stubIMEventChannelResolver) ResolveChannelsForEvent(_ context.Context, eventType string, platform string, channelID string) ([]*model.IMChannel, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := make([]*model.IMChannel, 0, len(s.channels))
	for _, channel := range s.channels {
		if channel == nil {
			continue
		}
		cloned := *channel
		result = append(result, &cloned)
	}
	return result, nil
}

type stubPageRecentAccessRepo struct {
	accesses map[string]*model.PageRecentAccess
}

func (r *stubPageRecentAccessRepo) Touch(_ context.Context, pageID, userID uuid.UUID, accessedAt time.Time) error {
	if r.accesses == nil {
		r.accesses = make(map[string]*model.PageRecentAccess)
	}
	r.accesses[pageID.String()+":"+userID.String()] = &model.PageRecentAccess{
		PageID:     pageID,
		UserID:     userID,
		AccessedAt: accessedAt,
	}
	return nil
}

func (r *stubPageRecentAccessRepo) ListByUser(_ context.Context, userID uuid.UUID, limit int) ([]*model.PageRecentAccess, error) {
	accesses := make([]*model.PageRecentAccess, 0)
	for _, access := range r.accesses {
		if access.UserID == userID {
			cloned := *access
			accesses = append(accesses, &cloned)
		}
	}
	sort.Slice(accesses, func(i, j int) bool { return accesses[i].AccessedAt.After(accesses[j].AccessedAt) })
	if limit > 0 && len(accesses) > limit {
		accesses = accesses[:limit]
	}
	return accesses, nil
}

type stubWikiBroadcaster struct {
	events []*ws.Event
}

func (b *stubWikiBroadcaster) BroadcastEvent(event *ws.Event) {
	cloned := *event
	b.events = append(b.events, &cloned)
}

type stubMentionLinkSyncer struct {
	projectID  uuid.UUID
	sourceType string
	sourceID   uuid.UUID
	createdBy  uuid.UUID
	content    string
	calls      int
}

func (s *stubMentionLinkSyncer) SyncMentionLinksForSource(_ context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID, createdBy uuid.UUID, content string) error {
	s.projectID = projectID
	s.sourceType = sourceType
	s.sourceID = sourceID
	s.createdBy = createdBy
	s.content = content
	s.calls++
	return nil
}

func TestWikiServiceCreateUpdateMoveDeletePageLifecycle(t *testing.T) {
	ctx := context.Background()
	projectID := uuid.New()
	spaceID := uuid.New()
	rootID := uuid.New()
	rootPath := "/" + rootID.String()

	pages := &stubWikiPageRepo{pages: map[uuid.UUID]*model.WikiPage{
		rootID: {
			ID:          rootID,
			SpaceID:     spaceID,
			Title:       "Root",
			Content:     `[]`,
			ContentText: "Root",
			Path:        rootPath,
			SortOrder:   0,
			CreatedAt:   time.Date(2026, 3, 26, 16, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 3, 26, 16, 0, 0, 0, time.UTC),
		},
	}}
	broadcaster := &stubWikiBroadcaster{}
	svc := NewWikiService(nil, pages, nil, nil, nil, nil, broadcaster)

	child, err := svc.CreatePage(ctx, projectID, spaceID, "Child", &rootID, `[{"type":"paragraph"}]`, uuidPtr(uuid.New()))
	if err != nil {
		t.Fatalf("CreatePage() error = %v", err)
	}
	if child.Path != rootPath+"/"+child.ID.String() {
		t.Fatalf("child path = %q", child.Path)
	}

	tree, err := svc.GetPageTree(ctx, spaceID)
	if err != nil {
		t.Fatalf("GetPageTree() error = %v", err)
	}
	if len(tree) != 2 {
		t.Fatalf("len(tree) = %d, want 2", len(tree))
	}

	expected := child.UpdatedAt
	updated, err := svc.UpdatePage(ctx, projectID, child.ID, "Child Updated", `[{"type":"heading"}]`, "Heading", uuidPtr(uuid.New()), &expected, nil)
	if err != nil {
		t.Fatalf("UpdatePage() error = %v", err)
	}
	if updated.Title != "Child Updated" || updated.ContentText != "Heading" {
		t.Fatalf("updated page = %+v", updated)
	}
	stale := updated.UpdatedAt.Add(-time.Nanosecond)
	if _, err := svc.UpdatePage(ctx, projectID, child.ID, "stale", `[]`, "", nil, &stale, nil); !errors.Is(err, ErrWikiPageConflict) {
		t.Fatalf("UpdatePage() stale error = %v, want %v", err, ErrWikiPageConflict)
	}

	if _, err := svc.MovePage(ctx, projectID, rootID, &child.ID, 0); !errors.Is(err, ErrWikiCircularMove) {
		t.Fatalf("MovePage() circular error = %v, want %v", err, ErrWikiCircularMove)
	}
	moved, err := svc.MovePage(ctx, projectID, child.ID, nil, 3)
	if err != nil {
		t.Fatalf("MovePage() error = %v", err)
	}
	if moved.ParentID != nil || moved.Path != "/"+child.ID.String() {
		t.Fatalf("moved page = %+v", moved)
	}

	if err := svc.DeletePage(ctx, projectID, rootID); err != nil {
		t.Fatalf("DeletePage() error = %v", err)
	}
	if pages.pages[rootID].DeletedAt == nil {
		t.Fatal("expected root page to be soft deleted")
	}

	if len(broadcaster.events) < 4 {
		t.Fatalf("broadcasted events = %d, want at least 4", len(broadcaster.events))
	}
	if broadcaster.events[0].Type != ws.EventWikiPageCreated {
		t.Fatalf("first event type = %q, want %q", broadcaster.events[0].Type, ws.EventWikiPageCreated)
	}
}

func TestWikiServiceUpdatePageSyncsMentionLinks(t *testing.T) {
	ctx := context.Background()
	projectID := uuid.New()
	pageID := uuid.New()
	actorID := uuid.New()
	pages := &stubWikiPageRepo{pages: map[uuid.UUID]*model.WikiPage{
		pageID: {
			ID:          pageID,
			SpaceID:     uuid.New(),
			Title:       "Docs",
			Content:     `[{"type":"paragraph","content":"draft"}]`,
			ContentText: "draft",
			Path:        "/" + pageID.String(),
			SortOrder:   0,
			CreatedAt:   time.Date(2026, 3, 26, 19, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 3, 26, 19, 0, 0, 0, time.UTC),
		},
	}}
	linkSyncer := &stubMentionLinkSyncer{}
	svc := NewWikiService(nil, pages, nil, nil, nil, nil, &stubWikiBroadcaster{}).WithEntityLinkSyncer(linkSyncer)

	expected := pages.pages[pageID].UpdatedAt
	updated, err := svc.UpdatePage(ctx, projectID, pageID, "Docs", "See [[task-"+uuid.New().String()+"]]", "See mention", &actorID, &expected, nil)
	if err != nil {
		t.Fatalf("UpdatePage() error = %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated page")
	}
	if linkSyncer.calls != 1 {
		t.Fatalf("link sync calls = %d, want 1", linkSyncer.calls)
	}
	if linkSyncer.projectID != projectID || linkSyncer.sourceType != model.EntityTypeWikiPage || linkSyncer.sourceID != pageID || linkSyncer.createdBy != actorID {
		t.Fatalf("link sync args = %+v", linkSyncer)
	}
	if linkSyncer.content != updated.Content {
		t.Fatalf("link sync content = %q, want %q", linkSyncer.content, updated.Content)
	}
}

func TestWikiServiceUpdatePageRollsBackWhenMentionSyncFails(t *testing.T) {
	db := openWikiServiceTxTestDB(t)
	pageID := uuid.New()
	projectID := uuid.New()
	spaceID := uuid.New()
	actorID := uuid.New()
	updatedAt := time.Date(2026, 3, 26, 22, 0, 0, 0, time.UTC)

	if err := db.Exec(`CREATE TABLE wiki_pages (
		id TEXT PRIMARY KEY,
		space_id TEXT NOT NULL,
		parent_id TEXT,
		title TEXT,
		content TEXT,
		content_text TEXT,
		path TEXT,
		sort_order INTEGER,
		is_template BOOLEAN,
		template_category TEXT,
		is_system BOOLEAN,
		is_pinned BOOLEAN,
		created_by TEXT,
		updated_by TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create wiki_pages table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE entity_links (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		source_type TEXT NOT NULL,
		source_id TEXT NOT NULL,
		target_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		link_type TEXT NOT NULL,
		anchor_block_id TEXT,
		created_by TEXT NOT NULL,
		created_at DATETIME,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create entity_links table: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO wiki_pages (id, space_id, title, content, content_text, path, sort_order, is_template, template_category, is_system, is_pinned, created_by, updated_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pageID.String(),
		spaceID.String(),
		"Doc",
		"before",
		"before",
		"/"+pageID.String(),
		0,
		false,
		nil,
		false,
		false,
		actorID.String(),
		actorID.String(),
		updatedAt,
		updatedAt,
	).Error; err != nil {
		t.Fatalf("insert wiki page: %v", err)
	}

	pageRepo := repository.NewWikiPageRepository(db)
	syncer := &failingMentionLinkSyncer{
		base: NewEntityLinkService(repository.NewEntityLinkRepository(db), nil, nil),
		fail: true,
	}
	svc := NewWikiService(nil, pageRepo, nil, nil, nil, nil, &stubWikiBroadcaster{}).WithEntityLinkSyncer(syncer)

	if _, err := svc.UpdatePage(context.Background(), projectID, pageID, "Doc updated", "after [[task-"+uuid.New().String()+"]]", "after", &actorID, &updatedAt, nil); err == nil {
		t.Fatal("expected UpdatePage() to fail when mention sync fails")
	}

	page, err := pageRepo.GetByID(context.Background(), pageID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if page.Content != "before" || page.Title != "Doc" {
		t.Fatalf("page after rollback = %+v", page)
	}
}

func openWikiServiceTxTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	return db
}

func TestWikiServiceCreateAndRestoreVersion(t *testing.T) {
	ctx := context.Background()
	projectID := uuid.New()
	pageID := uuid.New()
	spaceID := uuid.New()
	actorID := uuid.New()
	pageRepo := &stubWikiPageRepo{pages: map[uuid.UUID]*model.WikiPage{
		pageID: {
			ID:          pageID,
			SpaceID:     spaceID,
			Title:       "ADR",
			Content:     `[{"type":"paragraph","content":"draft"}]`,
			ContentText: "draft",
			Path:        "/" + pageID.String(),
			SortOrder:   0,
			CreatedAt:   time.Date(2026, 3, 26, 17, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 3, 26, 17, 0, 0, 0, time.UTC),
		},
	}}
	versionRepo := &stubPageVersionRepo{}
	svc := NewWikiService(nil, pageRepo, versionRepo, nil, nil, nil, &stubWikiBroadcaster{})

	version, err := svc.CreateVersion(ctx, projectID, pageID, "Initial", &actorID)
	if err != nil {
		t.Fatalf("CreateVersion() error = %v", err)
	}
	if version.VersionNumber != 1 {
		t.Fatalf("version number = %d, want 1", version.VersionNumber)
	}

	pageRepo.pages[pageID].Content = `[{"type":"paragraph","content":"published"}]`
	pageRepo.pages[pageID].ContentText = "published"
	version2, err := svc.CreateVersion(ctx, projectID, pageID, "Published", &actorID)
	if err != nil {
		t.Fatalf("CreateVersion(second) error = %v", err)
	}
	if version2.VersionNumber != 2 {
		t.Fatalf("second version number = %d, want 2", version2.VersionNumber)
	}

	restoredPage, restoreVersion, err := svc.RestoreVersion(ctx, projectID, pageID, version.ID, &actorID)
	if err != nil {
		t.Fatalf("RestoreVersion() error = %v", err)
	}
	if restoredPage.Content != version.Content {
		t.Fatalf("restored content = %q, want %q", restoredPage.Content, version.Content)
	}
	if !strings.Contains(restoreVersion.Name, "Restored from v1") {
		t.Fatalf("restore version name = %q", restoreVersion.Name)
	}
}

func TestWikiServiceCommentTemplateAndFavoriteFlows(t *testing.T) {
	ctx := context.Background()
	projectID := uuid.New()
	spaceID := uuid.New()
	pageID := uuid.New()
	userID := uuid.New()
	pages := &stubWikiPageRepo{pages: map[uuid.UUID]*model.WikiPage{
		pageID: {
			ID:          pageID,
			SpaceID:     spaceID,
			Title:       "PRD",
			Content:     `[{"type":"paragraph","content":"hello"}]`,
			ContentText: "hello",
			Path:        "/" + pageID.String(),
			SortOrder:   0,
			CreatedAt:   time.Date(2026, 3, 26, 18, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 3, 26, 18, 0, 0, 0, time.UTC),
		},
	}}
	comments := &stubPageCommentRepo{}
	favorites := &stubPageFavoriteRepo{}
	recent := &stubPageRecentAccessRepo{}
	broadcaster := &stubWikiBroadcaster{}
	spaces := &stubWikiSpaceRepo{spaces: map[uuid.UUID]*model.WikiSpace{
		spaceID: {
			ID:        spaceID,
			ProjectID: projectID,
			CreatedAt: time.Date(2026, 3, 26, 18, 0, 0, 0, time.UTC),
		},
	}}
	svc := NewWikiService(spaces, pages, &stubPageVersionRepo{}, comments, favorites, recent, broadcaster)

	comment, err := svc.CreateComment(ctx, projectID, pageID, "please review", stringPtr("block-a"), nil, &userID, `["alice"]`)
	if err != nil {
		t.Fatalf("CreateComment() error = %v", err)
	}
	resolved, err := svc.ResolveComment(ctx, projectID, pageID, comment.ID)
	if err != nil {
		t.Fatalf("ResolveComment() error = %v", err)
	}
	if resolved.ResolvedAt == nil {
		t.Fatal("expected comment to be resolved")
	}
	reopened, err := svc.ReopenComment(ctx, projectID, pageID, comment.ID)
	if err != nil {
		t.Fatalf("ReopenComment() error = %v", err)
	}
	if reopened.ResolvedAt != nil {
		t.Fatal("expected comment to be reopened")
	}

	seeded, err := svc.SeedBuiltInTemplates(ctx, projectID, spaceID)
	if err != nil {
		t.Fatalf("SeedBuiltInTemplates() error = %v", err)
	}
	if len(seeded) != 7 {
		t.Fatalf("seeded templates = %d, want 7", len(seeded))
	}

	template, err := svc.CreateTemplateFromPage(ctx, projectID, pageID, "PRD Template", "prd", &userID)
	if err != nil {
		t.Fatalf("CreateTemplateFromPage() error = %v", err)
	}
	if !template.IsTemplate || template.IsSystem {
		t.Fatalf("template flags = %+v", template)
	}

	created, err := svc.CreatePageFromTemplate(ctx, projectID, spaceID, template.ID, nil, "New Doc", &userID)
	if err != nil {
		t.Fatalf("CreatePageFromTemplate() error = %v", err)
	}
	if created.Title != "New Doc" || created.Content != template.Content {
		t.Fatalf("created from template = %+v", created)
	}

	if err := svc.AddFavorite(ctx, pageID, userID); err != nil {
		t.Fatalf("AddFavorite() error = %v", err)
	}
	if err := svc.TouchRecentAccess(ctx, pageID, userID); err != nil {
		t.Fatalf("TouchRecentAccess() error = %v", err)
	}
	if err := svc.SetPinned(ctx, projectID, pageID, true, &userID); err != nil {
		t.Fatalf("SetPinned() error = %v", err)
	}

	favoriteList, err := svc.ListFavorites(ctx, userID)
	if err != nil {
		t.Fatalf("ListFavorites() error = %v", err)
	}
	if len(favoriteList) != 1 {
		t.Fatalf("favoriteList len = %d, want 1", len(favoriteList))
	}
	recentList, err := svc.ListRecentAccess(ctx, userID, 10)
	if err != nil {
		t.Fatalf("ListRecentAccess() error = %v", err)
	}
	if len(recentList) != 1 {
		t.Fatalf("recentList len = %d, want 1", len(recentList))
	}
	if !pages.pages[pageID].IsPinned {
		t.Fatal("expected page to be pinned")
	}

	if err := svc.DeleteComment(ctx, projectID, pageID, comment.ID); err != nil {
		t.Fatalf("DeleteComment() error = %v", err)
	}
	commentList, err := svc.ListComments(ctx, pageID)
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if len(commentList) != 0 {
		t.Fatalf("commentList len = %d, want 0", len(commentList))
	}
	foundResolved := false
	for _, event := range broadcaster.events {
		if event.Type == ws.EventWikiCommentResolved {
			foundResolved = true
			break
		}
	}
	if !foundResolved {
		t.Fatalf("expected %q in broadcasted events, got %#v", ws.EventWikiCommentResolved, broadcaster.events)
	}
}

func TestWikiServiceForwardIMEventUsesConfiguredChannelRoutingBeforeFallback(t *testing.T) {
	ctx := context.Background()
	projectID := uuid.New()
	spaceID := uuid.New()
	pageID := uuid.New()
	actorID := uuid.New()
	pageRepo := &stubWikiPageRepo{pages: map[uuid.UUID]*model.WikiPage{
		pageID: {
			ID:          pageID,
			SpaceID:     spaceID,
			Title:       "ADR",
			Content:     `[{"type":"paragraph","content":"draft"}]`,
			ContentText: "draft",
			Path:        "/" + pageID.String(),
			SortOrder:   0,
			CreatedAt:   time.Date(2026, 3, 26, 17, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 3, 26, 17, 0, 0, 0, time.UTC),
		},
	}}
	versionRepo := &stubPageVersionRepo{}
	notifier := &stubWikiIMNotifier{}
	resolver := &stubIMEventChannelResolver{
		channels: []*model.IMChannel{{
			Platform:  "slack",
			Name:      "Docs",
			ChannelID: "C-docs",
			Events:    []string{"wiki.version.published"},
			Active:    true,
		}},
	}
	svc := NewWikiService(nil, pageRepo, versionRepo, nil, nil, nil, &stubWikiBroadcaster{}).
		WithIMForwarder(notifier, "feishu", "legacy-chat").
		WithIMChannelResolver(resolver)

	if _, err := svc.CreateVersion(ctx, projectID, pageID, "Published", &actorID); err != nil {
		t.Fatalf("CreateVersion() error = %v", err)
	}

	if len(notifier.requests) != 1 {
		t.Fatalf("notify requests = %+v, want 1", notifier.requests)
	}
	if notifier.requests[0].Platform != "slack" || notifier.requests[0].ChannelID != "C-docs" {
		t.Fatalf("notify request = %+v", notifier.requests[0])
	}
	if notifier.requests[0].Event != model.NotificationTypeWikiVersionPublished {
		t.Fatalf("event = %q", notifier.requests[0].Event)
	}
}

func TestWikiServiceForwardIMEventFallsBackWhenNoConfiguredChannelMatches(t *testing.T) {
	ctx := context.Background()
	projectID := uuid.New()
	spaceID := uuid.New()
	pageID := uuid.New()
	actorID := uuid.New()
	pageRepo := &stubWikiPageRepo{pages: map[uuid.UUID]*model.WikiPage{
		pageID: {
			ID:          pageID,
			SpaceID:     spaceID,
			Title:       "ADR",
			Content:     `[{"type":"paragraph","content":"draft"}]`,
			ContentText: "draft",
			Path:        "/" + pageID.String(),
			SortOrder:   0,
			CreatedAt:   time.Date(2026, 3, 26, 17, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 3, 26, 17, 0, 0, 0, time.UTC),
		},
	}}
	versionRepo := &stubPageVersionRepo{}
	notifier := &stubWikiIMNotifier{}
	svc := NewWikiService(nil, pageRepo, versionRepo, nil, nil, nil, &stubWikiBroadcaster{}).
		WithIMForwarder(notifier, "feishu", "legacy-chat").
		WithIMChannelResolver(&stubIMEventChannelResolver{})

	if _, err := svc.CreateVersion(ctx, projectID, pageID, "Published", &actorID); err != nil {
		t.Fatalf("CreateVersion() error = %v", err)
	}

	if len(notifier.requests) != 1 {
		t.Fatalf("notify requests = %+v, want 1", notifier.requests)
	}
	if notifier.requests[0].Platform != "feishu" || notifier.requests[0].ChannelID != "legacy-chat" {
		t.Fatalf("notify request = %+v", notifier.requests[0])
	}
}

func cloneWikiPage(page *model.WikiPage) *model.WikiPage {
	if page == nil {
		return nil
	}
	cloned := *page
	cloned.ParentID = testCloneUUIDPointer(page.ParentID)
	cloned.CreatedBy = testCloneUUIDPointer(page.CreatedBy)
	cloned.UpdatedBy = testCloneUUIDPointer(page.UpdatedBy)
	cloned.DeletedAt = testCloneTimePointer(page.DeletedAt)
	return &cloned
}

func clonePageComment(comment *model.PageComment) *model.PageComment {
	if comment == nil {
		return nil
	}
	cloned := *comment
	cloned.AnchorBlockID = testCloneStringPointer(comment.AnchorBlockID)
	cloned.ParentCommentID = testCloneUUIDPointer(comment.ParentCommentID)
	cloned.CreatedBy = testCloneUUIDPointer(comment.CreatedBy)
	cloned.ResolvedAt = testCloneTimePointer(comment.ResolvedAt)
	cloned.DeletedAt = testCloneTimePointer(comment.DeletedAt)
	return &cloned
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	return &id
}

func stringPtr(value string) *string {
	return &value
}

func testCloneUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func testCloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func testCloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
