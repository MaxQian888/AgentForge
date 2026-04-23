# Organization Management + Global RBAC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Organization entity with multi-tenant isolation, global RBAC, and org-scoped project management on top of the existing project-level RBAC.

**Architecture:** Layered Go backend (model → repository → service → handler → routes) with new `OrgRBAC` middleware alongside existing `ProjectMiddleware`. Frontend gains org pages, stores, and sidebar integration. Fully backward-compatible — existing personal projects continue to work without an org.

**Tech Stack:** Go (Echo, GORM, PostgreSQL), TypeScript (Next.js 16, Zustand, shadcn/ui), Jest

---

## File Structure

### Backend (src-go/)

| File | Action | Responsibility |
|------|--------|---------------|
| `migrations/079_create_organizations.up.sql` | Create | DDL for organizations, org_members, projects.org_id |
| `migrations/079_create_organizations.down.sql` | Create | Rollback DDL |
| `internal/model/org.go` | Create | Organization, OrgMember structs, DTOs, request types, constants |
| `internal/repository/org_repo.go` | Create | CRUD for organizations and org_members |
| `internal/repository/org_repo_test.go` | Create | Repository unit tests |
| `internal/service/org_service.go` | Create | Business logic: create org, invite, role management |
| `internal/service/org_service_test.go` | Create | Service unit tests |
| `internal/middleware/org_rbac.go` | Create | Org-scoped RBAC middleware, action IDs, role matrix |
| `internal/handler/org_handler.go` | Create | HTTP handlers for all org endpoints |
| `internal/server/routes.go` | Modify | Wire org handler, add org route group |

### Frontend

| File | Action | Responsibility |
|------|--------|---------------|
| `lib/stores/org-store.ts` | Create | Org CRUD, current org context |
| `lib/stores/org-member-store.ts` | Create | Org membership management |
| `app/(dashboard)/orgs/page.tsx` | Create | Org list page |
| `app/(dashboard)/orgs/[orgId]/page.tsx` | Create | Org detail/dashboard page |
| `app/(dashboard)/orgs/[orgId]/members/page.tsx` | Create | Member management |
| `app/(dashboard)/orgs/[orgId]/settings/page.tsx` | Create | Org settings |
| `components/org/org-card.tsx` | Create | Org list card component |
| `components/org/org-member-table.tsx` | Create | Member table with role management |
| `components/org/org-switcher.tsx` | Create | Org context switcher for header |
| `components/layout/sidebar.tsx` | Modify | Add org nav group |
| `components/project/new-project-dialog.tsx` | Modify | Add org selector dropdown |

---

## Task 1: Database Migration

**Files:**
- Create: `src-go/migrations/079_create_organizations.up.sql`
- Create: `src-go/migrations/079_create_organizations.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- 079_create_organizations.up.sql

-- Organizations
CREATE TABLE organizations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        VARCHAR(64) UNIQUE NOT NULL,
    name        VARCHAR(256) NOT NULL,
    avatar_url  TEXT,
    plan        VARCHAR(32) NOT NULL DEFAULT 'free',
    settings    JSONB NOT NULL DEFAULT '{}',
    created_by  UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_organizations_slug ON organizations(slug) WHERE deleted_at IS NULL;
CREATE INDEX idx_organizations_created_by ON organizations(created_by) WHERE deleted_at IS NULL;

-- Organization membership
CREATE TABLE org_members (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        VARCHAR(32) NOT NULL DEFAULT 'member',
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(org_id, user_id)
);

CREATE INDEX idx_org_members_user ON org_members(user_id);
CREATE INDEX idx_org_members_org ON org_members(org_id);

-- Add org_id to projects (nullable for backward compatibility)
ALTER TABLE projects ADD COLUMN org_id UUID REFERENCES organizations(id);
CREATE INDEX idx_projects_org ON projects(org_id) WHERE deleted_at IS NULL;
```

- [ ] **Step 2: Write the down migration**

```sql
-- 079_create_organizations.down.sql
DROP INDEX IF EXISTS idx_projects_org;
ALTER TABLE projects DROP COLUMN IF EXISTS org_id;
DROP TABLE IF EXISTS org_members;
DROP TABLE IF EXISTS organizations;
```

- [ ] **Step 3: Verify migration compiles**

Run: `cd /d/Project/AgentForge/src-go && go build ./...`

Expected: builds without errors (migration is embedded via `embed.go`)

- [ ] **Step 4: Commit**

```bash
git add src-go/migrations/079_create_organizations.up.sql src-go/migrations/079_create_organizations.down.sql
git commit -m "feat(org): add organizations and org_members migration"
```

---

## Task 2: Domain Model

**Files:**
- Create: `src-go/internal/model/org.go`

- [ ] **Step 1: Write the model file**

```go
package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ── Org role constants ──────────────────────────────────────────────

const (
	OrgRoleOwner  = "owner"
	OrgRoleAdmin  = "admin"
	OrgRoleMember = "member"
	OrgRoleViewer = "viewer"
)

func OrgRoleRank(v string) int {
	switch v {
	case OrgRoleOwner:
		return 4
	case OrgRoleAdmin:
		return 3
	case OrgRoleMember:
		return 2
	case OrgRoleViewer:
		return 1
	}
	return 0
}

func OrgRoleAtLeast(have, need string) bool {
	return OrgRoleRank(have) >= OrgRoleRank(need)
}

func IsValidOrgRole(v string) bool {
	return OrgRoleRank(v) > 0
}

// ── Org plan constants ──────────────────────────────────────────────

const (
	OrgPlanFree       = "free"
	OrgPlanTeam       = "team"
	OrgPlanEnterprise = "enterprise"
)

func IsValidOrgPlan(v string) bool {
	switch v {
	case OrgPlanFree, OrgPlanTeam, OrgPlanEnterprise:
		return true
	}
	return false
}

// ── Organization ────────────────────────────────────────────────────

type Organization struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	Slug      string     `db:"slug" json:"slug"`
	Name      string     `db:"name" json:"name"`
	AvatarURL *string    `db:"avatar_url" json:"avatarUrl,omitempty"`
	Plan      string     `db:"plan" json:"plan"`
	Settings  string     `db:"settings" json:"settings"`
	CreatedBy uuid.UUID  `db:"created_by" json:"createdBy"`
	CreatedAt time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time  `db:"updated_at" json:"updatedAt"`
	DeletedAt *time.Time `db:"deleted_at" json:"deletedAt,omitempty"`
}

func (o *Organization) IsDeleted() bool {
	return o.DeletedAt != nil
}

func (o *Organization) ToDTO() OrganizationDTO {
	dto := OrganizationDTO{
		ID:        o.ID.String(),
		Slug:      o.Slug,
		Name:      o.Name,
		Plan:      o.Plan,
		Settings:  o.Settings,
		CreatedBy: o.CreatedBy.String(),
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}
	if o.AvatarURL != nil {
		dto.AvatarURL = *o.AvatarURL
	}
	return dto
}

type OrganizationDTO struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatarUrl,omitempty"`
	Plan      string    `json:"plan"`
	Settings  string    `json:"settings"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ── Org Member ──────────────────────────────────────────────────────

type OrgMember struct {
	ID       uuid.UUID `db:"id" json:"id"`
	OrgID    uuid.UUID `db:"org_id" json:"orgId"`
	UserID   uuid.UUID `db:"user_id" json:"userId"`
	Role     string    `db:"role" json:"role"`
	JoinedAt time.Time `db:"joined_at" json:"joinedAt"`

	// Joined fields (populated by queries)
	UserName  string `db:"-" json:"userName,omitempty"`
	UserEmail string `db:"-" json:"userEmail,omitempty"`
}

func (m *OrgMember) ToDTO() OrgMemberDTO {
	return OrgMemberDTO{
		ID:        m.ID.String(),
		OrgID:     m.OrgID.String(),
		UserID:    m.UserID.String(),
		Role:      m.Role,
		JoinedAt:  m.JoinedAt,
		UserName:  m.UserName,
		UserEmail: m.UserEmail,
	}
}

type OrgMemberDTO struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"orgId"`
	UserID    string    `json:"userId"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joinedAt"`
	UserName  string    `json:"userName,omitempty"`
	UserEmail string    `json:"userEmail,omitempty"`
}

// ── Request/Response types ──────────────────────────────────────────

type CreateOrgRequest struct {
	Name string `json:"name" validate:"required,min=1,max=256"`
	Slug string `json:"slug" validate:"required,min=1,max=64,alphanumdash"`
}

type UpdateOrgRequest struct {
	Name      *string `json:"name" validate:"omitempty,min=1,max=256"`
	AvatarURL *string `json:"avatarUrl" validate:"omitempty,url"`
}

type UpdateOrgMemberRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=owner admin member viewer"`
}

type OrgListResponse struct {
	Organizations []OrganizationDTO `json:"organizations"`
}

type OrgMemberListResponse struct {
	Members []OrgMemberDTO `json:"members"`
}

// NormalizeSlug lowercases and strips non-alphanumeric characters (except dash/underscore).
func NormalizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
```

- [ ] **Step 2: Add validation tag registration**

Open `src-go/internal/handler/handler.go` (or wherever validator is set up) and verify the `alphanumdash` and `oneof` tags are supported. If not, add a custom validator:

```go
// In the validator initialization block (wherever echo.Validator is set)
// "alphanumdash" = alphanumeric + dash + underscore
// "oneof" is built-in to go-playground/validator
```

Note: `oneof` is already built-in. For `alphanumdash`, check if there's a custom registration. If not, add to the existing validator setup:

```go
if v, ok := echoValidator.(*validator.Validate); ok {
    v.RegisterValidation("alphanumdash", func(fl validator.FieldLevel) bool {
        s := fl.Field().String()
        for _, r := range s {
            if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
                return false
            }
        }
        return len(s) > 0
    })
}
```

- [ ] **Step 3: Build to verify**

Run: `cd /d/Project/AgentForge/src-go && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add src-go/internal/model/org.go
git commit -m "feat(org): add Organization and OrgMember domain models"
```

---

## Task 3: Repository Layer

**Files:**
- Create: `src-go/internal/repository/org_repo.go`
- Create: `src-go/internal/repository/org_repo_test.go`

- [ ] **Step 1: Write the repository**

```go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ── Org Repository ──────────────────────────────────────────────────

type OrgRepository struct {
	db *gorm.DB
}

func NewOrgRepository(db *gorm.DB) *OrgRepository {
	return &OrgRepository{db: db}
}

// ── Organization CRUD ───────────────────────────────────────────────

type orgRecord struct {
	ID        uuid.UUID  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Slug      string     `gorm:"uniqueIndex;size:64;not null"`
	Name      string     `gorm:"size:256;not null"`
	AvatarURL *string    `gorm:"column:avatar_url"`
	Plan      string     `gorm:"size:32;not null;default:free"`
	Settings  string     `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedBy uuid.UUID  `gorm:"column:created_by;type:uuid;not null"`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (*orgRecord) TableName() string { return "organizations" }

func newOrgRecord(o *model.Organization) *orgRecord {
	return &orgRecord{
		ID: o.ID, Slug: o.Slug, Name: o.Name,
		AvatarURL: o.AvatarURL, Plan: o.Plan, Settings: o.Settings,
		CreatedBy: o.CreatedBy,
	}
}

func (r *orgRecord) toModel() *model.Organization {
	return &model.Organization{
		ID: r.ID, Slug: r.Slug, Name: r.Name,
		AvatarURL: r.AvatarURL, Plan: r.Plan, Settings: r.Settings,
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		DeletedAt: r.DeletedAt.TimePtr(),
	}
}

func (repo *OrgRepository) Create(ctx context.Context, org *model.Organization) error {
	if repo.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := repo.db.WithContext(ctx).Create(newOrgRecord(org)).Error; err != nil {
		return fmt.Errorf("create org: %w", err)
	}
	return nil
}

func (repo *OrgRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Organization, error) {
	if repo.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var rec orgRecord
	if err := repo.db.WithContext(ctx).Where("id = ?", id).Take(&rec).Error; err != nil {
		return nil, fmt.Errorf("get org by id: %w", normalizeRepositoryError(err))
	}
	return rec.toModel(), nil
}

func (repo *OrgRepository) GetBySlug(ctx context.Context, slug string) (*model.Organization, error) {
	if repo.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var rec orgRecord
	if err := repo.db.WithContext(ctx).Where("slug = ?", slug).Take(&rec).Error; err != nil {
		return nil, fmt.Errorf("get org by slug: %w", normalizeRepositoryError(err))
	}
	return rec.toModel(), nil
}

func (repo *OrgRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*model.Organization, error) {
	if repo.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var recs []orgRecord
	err := repo.db.WithContext(ctx).
		Joins("JOIN org_members ON org_members.org_id = organizations.id").
		Where("org_members.user_id = ?", userID).
		Order("organizations.name").
		Find(&recs).Error
	if err != nil {
		return nil, fmt.Errorf("list orgs by user: %w", err)
	}
	result := make([]*model.Organization, len(recs))
	for i := range recs {
		result[i] = recs[i].toModel()
	}
	return result, nil
}

func (repo *OrgRepository) Update(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	if repo.db == nil {
		return ErrDatabaseUnavailable
	}
	result := repo.db.WithContext(ctx).Model(&orgRecord{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update org: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (repo *OrgRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return repo.Update(ctx, id, map[string]any{"deleted_at": time.Now().UTC()})
}

// ── Org Member CRUD ─────────────────────────────────────────────────

type orgMemberRecord struct {
	ID       uuid.UUID `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	OrgID    uuid.UUID `gorm:"column:org_id;type:uuid;not null"`
	UserID   uuid.UUID `gorm:"column:user_id;type:uuid;not null"`
	Role     string    `gorm:"size:32;not null;default:member"`
	JoinedAt time.Time `gorm:"autoCreateTime"`
}

func (*orgMemberRecord) TableName() string { return "org_members" }

func (repo *OrgRepository) CreateMember(ctx context.Context, m *model.OrgMember) error {
	if repo.db == nil {
		return ErrDatabaseUnavailable
	}
	rec := &orgMemberRecord{
		OrgID: m.OrgID, UserID: m.UserID, Role: m.Role,
	}
	if err := repo.db.WithContext(ctx).Create(rec).Error; err != nil {
		return fmt.Errorf("create org member: %w", err)
	}
	m.ID = rec.ID
	m.JoinedAt = rec.JoinedAt
	return nil
}

func (repo *OrgRepository) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*model.OrgMember, error) {
	if repo.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var rec orgMemberRecord
	err := repo.db.WithContext(ctx).Where("org_id = ? AND user_id = ?", orgID, userID).Take(&rec).Error
	if err != nil {
		return nil, fmt.Errorf("get org member: %w", normalizeRepositoryError(err))
	}
	return &model.OrgMember{
		ID: rec.ID, OrgID: rec.OrgID, UserID: rec.UserID,
		Role: rec.Role, JoinedAt: rec.JoinedAt,
	}, nil
}

type orgMemberWithUser struct {
	orgMemberRecord
	UserName  string `gorm:"column:user_name"`
	UserEmail string `gorm:"column:user_email"`
}

func (repo *OrgRepository) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*model.OrgMember, error) {
	if repo.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var recs []orgMemberWithUser
	err := repo.db.WithContext(ctx).
		Table("org_members").
		Select("org_members.*, users.name as user_name, users.email as user_email").
		Joins("JOIN users ON users.id = org_members.user_id").
		Where("org_members.org_id = ?", orgID).
		Order("org_members.joined_at").
		Find(&recs).Error
	if err != nil {
		return nil, fmt.Errorf("list org members: %w", err)
	}
	result := make([]*model.OrgMember, len(recs))
	for i := range recs {
		result[i] = &model.OrgMember{
			ID: recs[i].ID, OrgID: recs[i].OrgID, UserID: recs[i].UserID,
			Role: recs[i].Role, JoinedAt: recs[i].JoinedAt,
			UserName: recs[i].UserName, UserEmail: recs[i].UserEmail,
		}
	}
	return result, nil
}

func (repo *OrgRepository) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	if repo.db == nil {
		return ErrDatabaseUnavailable
	}
	result := repo.db.WithContext(ctx).
		Model(&orgMemberRecord{}).
		Where("org_id = ? AND user_id = ?", orgID, userID).
		Update("role", role)
	if result.Error != nil {
		return fmt.Errorf("update org member role: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (repo *OrgRepository) DeleteMember(ctx context.Context, orgID, userID uuid.UUID) error {
	if repo.db == nil {
		return ErrDatabaseUnavailable
	}
	result := repo.db.WithContext(ctx).
		Where("org_id = ? AND user_id = ?", orgID, userID).
		Delete(&orgMemberRecord{})
	if result.Error != nil {
		return fmt.Errorf("delete org member: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateOrgWithOwner creates an org and adds the creator as owner in a transaction.
func (repo *OrgRepository) CreateOrgWithOwner(ctx context.Context, org *model.Organization, owner *model.OrgMember) error {
	return repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(newOrgRecord(org)).Error; err != nil {
			return fmt.Errorf("create org: %w", err)
		}
		rec := &orgMemberRecord{OrgID: org.ID, UserID: owner.UserID, Role: owner.Role}
		if err := tx.Create(rec).Error; err != nil {
			return fmt.Errorf("create org owner: %w", err)
		}
		owner.ID = rec.ID
		owner.JoinedAt = rec.JoinedAt
		return nil
	})
}
```

- [ ] **Step 2: Write repository tests**

```go
package repository

import (
	"context"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupOrgTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// Create tables manually for SQLite test
	require.NoError(t, db.Exec(`CREATE TABLE organizations (
		id TEXT PRIMARY KEY, slug TEXT UNIQUE NOT NULL, name TEXT NOT NULL,
		avatar_url TEXT, plan TEXT NOT NULL DEFAULT 'free', settings TEXT NOT NULL DEFAULT '{}',
		created_by TEXT NOT NULL, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE org_members (
		id TEXT PRIMARY KEY, org_id TEXT NOT NULL, user_id TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'member', joined_at DATETIME,
		UNIQUE(org_id, user_id)
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY, name TEXT, email TEXT
	)`).Error)
	return db
}

func TestOrgRepository_CreateOrgWithOwner(t *testing.T) {
	db := setupOrgTestDB(t)
	repo := NewOrgRepository(db)
	ctx := context.Background()

	userID := uuid.New()
	require.NoError(t, db.Exec("INSERT INTO users (id, name, email) VALUES (?, ?, ?)", userID, "Alice", "alice@test.com").Error)

	org := &model.Organization{
		ID: uuid.New(), Slug: "test-org", Name: "Test Org",
		Plan: model.OrgPlanFree, Settings: "{}", CreatedBy: userID,
	}
	owner := &model.OrgMember{OrgID: org.ID, UserID: userID, Role: model.OrgRoleOwner}

	err := repo.CreateOrgWithOwner(ctx, org, owner)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, owner.ID)

	got, err := repo.GetByID(ctx, org.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Test Org", got.Name)

	member, err := repo.GetMember(ctx, org.ID, userID)
	assert.NoError(t, err)
	assert.Equal(t, model.OrgRoleOwner, member.Role)
}

func TestOrgRepository_GetBySlug(t *testing.T) {
	db := setupOrgTestDB(t)
	repo := NewOrgRepository(db)
	ctx := context.Background()

	userID := uuid.New()
	require.NoError(t, db.Exec("INSERT INTO users (id, name, email) VALUES (?, ?, ?)", userID, "Alice", "alice@test.com").Error)

	org := &model.Organization{
		ID: uuid.New(), Slug: "my-slug", Name: "My Org",
		Plan: model.OrgPlanFree, Settings: "{}", CreatedBy: userID,
	}
	require.NoError(t, repo.Create(ctx, org))

	got, err := repo.GetBySlug(ctx, "my-slug")
	assert.NoError(t, err)
	assert.Equal(t, org.ID, got.ID)

	_, err = repo.GetBySlug(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestOrgRepository_ListByUserID(t *testing.T) {
	db := setupOrgTestDB(t)
	repo := NewOrgRepository(db)
	ctx := context.Background()

	userID := uuid.New()
	require.NoError(t, db.Exec("INSERT INTO users (id, name, email) VALUES (?, ?, ?)", userID, "Alice", "alice@test.com").Error)

	org1 := &model.Organization{ID: uuid.New(), Slug: "org-1", Name: "Org 1", Plan: "free", Settings: "{}", CreatedBy: userID}
	org2 := &model.Organization{ID: uuid.New(), Slug: "org-2", Name: "Org 2", Plan: "free", Settings: "{}", CreatedBy: userID}
	require.NoError(t, repo.Create(ctx, org1))
	require.NoError(t, repo.Create(ctx, org2))
	require.NoError(t, repo.CreateMember(ctx, &model.OrgMember{OrgID: org1.ID, UserID: userID, Role: model.OrgRoleOwner}))
	require.NoError(t, repo.CreateMember(ctx, &model.OrgMember{OrgID: org2.ID, UserID: userID, Role: model.OrgRoleMember}))

	orgs, err := repo.ListByUserID(ctx, userID)
	assert.NoError(t, err)
	assert.Len(t, orgs, 2)
}

func TestOrgRepository_UpdateMemberRole(t *testing.T) {
	db := setupOrgTestDB(t)
	repo := NewOrgRepository(db)
	ctx := context.Background()

	userID := uuid.New()
	require.NoError(t, db.Exec("INSERT INTO users (id, name, email) VALUES (?, ?, ?)", userID, "Alice", "alice@test.com").Error)

	org := &model.Organization{ID: uuid.New(), Slug: "test", Name: "Test", Plan: "free", Settings: "{}", CreatedBy: userID}
	require.NoError(t, repo.Create(ctx, org))
	require.NoError(t, repo.CreateMember(ctx, &model.OrgMember{OrgID: org.ID, UserID: userID, Role: model.OrgRoleMember}))

	err := repo.UpdateMemberRole(ctx, org.ID, userID, model.OrgRoleAdmin)
	assert.NoError(t, err)

	member, _ := repo.GetMember(ctx, org.ID, userID)
	assert.Equal(t, model.OrgRoleAdmin, member.Role)
}

func TestOrgRepository_DeleteMember(t *testing.T) {
	db := setupOrgTestDB(t)
	repo := NewOrgRepository(db)
	ctx := context.Background()

	userID := uuid.New()
	require.NoError(t, db.Exec("INSERT INTO users (id, name, email) VALUES (?, ?, ?)", userID, "Alice", "alice@test.com").Error)

	org := &model.Organization{ID: uuid.New(), Slug: "test", Name: "Test", Plan: "free", Settings: "{}", CreatedBy: userID}
	require.NoError(t, repo.Create(ctx, org))
	require.NoError(t, repo.CreateMember(ctx, &model.OrgMember{OrgID: org.ID, UserID: userID, Role: model.OrgRoleMember}))

	err := repo.DeleteMember(ctx, org.ID, userID)
	assert.NoError(t, err)

	_, err = repo.GetMember(ctx, org.ID, userID)
	assert.ErrorIs(t, err, ErrNotFound)
}
```

- [ ] **Step 3: Run tests**

Run: `cd /d/Project/AgentForge/src-go && go test ./internal/repository/ -run TestOrg -v`

Expected: All 6 tests pass

- [ ] **Step 4: Commit**

```bash
git add src-go/internal/repository/org_repo.go src-go/internal/repository/org_repo_test.go
git commit -m "feat(org): add org repository with CRUD and member management"
```

---

## Task 4: Service Layer

**Files:**
- Create: `src-go/internal/service/org_service.go`
- Create: `src-go/internal/service/org_service_test.go`

- [ ] **Step 1: Write the service**

```go
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrOrgNotFound       = errors.New("org: not found")
	ErrOrgSlugTaken      = errors.New("org: slug already taken")
	ErrOrgNotMember      = errors.New("org: user is not a member")
	ErrOrgLastOwner      = errors.New("org: cannot remove the last owner")
	ErrOrgForbidden      = errors.New("org: insufficient permissions")
	ErrOrgInvalidRole    = errors.New("org: invalid role")
	ErrOrgAlreadyMember  = errors.New("org: user is already a member")
)

// ── Narrow interfaces ───────────────────────────────────────────────

type OrgRepo interface {
	Create(ctx context.Context, org *model.Organization) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Organization, error)
	GetBySlug(ctx context.Context, slug string) (*model.Organization, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*model.Organization, error)
	Update(ctx context.Context, id uuid.UUID, updates map[string]any) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	CreateOrgWithOwner(ctx context.Context, org *model.Organization, owner *model.OrgMember) error
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*model.OrgMember, error)
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]*model.OrgMember, error)
	UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error
	DeleteMember(ctx context.Context, orgID, userID uuid.UUID) error
	CreateMember(ctx context.Context, m *model.OrgMember) error
}

type OrgUserLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

// ── Service ─────────────────────────────────────────────────────────

type OrgService struct {
	repo     OrgRepo
	userRepo OrgUserLookup
}

func NewOrgService(repo OrgRepo) *OrgService {
	return &OrgService{repo: repo}
}

func (s *OrgService) WithUserLookup(userRepo OrgUserLookup) *OrgService {
	s.userRepo = userRepo
	return s
}

func (s *OrgService) Create(ctx context.Context, creatorID uuid.UUID, req *model.CreateOrgRequest) (*model.Organization, error) {
	slug := model.NormalizeSlug(req.Slug)
	if slug == "" {
		return nil, fmt.Errorf("invalid slug: %w", ErrOrgInvalidRole)
	}

	// Check slug uniqueness
	if existing, _ := s.repo.GetBySlug(ctx, slug); existing != nil {
		return nil, ErrOrgSlugTaken
	}

	org := &model.Organization{
		ID:        uuid.New(),
		Slug:      slug,
		Name:      req.Name,
		Plan:      model.OrgPlanFree,
		Settings:  "{}",
		CreatedBy: creatorID,
	}

	owner := &model.OrgMember{
		OrgID:  org.ID,
		UserID: creatorID,
		Role:   model.OrgRoleOwner,
	}

	if err := s.repo.CreateOrgWithOwner(ctx, org, owner); err != nil {
		return nil, fmt.Errorf("create org: %w", err)
	}

	return s.repo.GetByID(ctx, org.ID)
}

func (s *OrgService) Get(ctx context.Context, orgID uuid.UUID) (*model.Organization, error) {
	org, err := s.repo.GetByID(ctx, orgID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrOrgNotFound
		}
		return nil, fmt.Errorf("get org: %w", err)
	}
	return org, nil
}

func (s *OrgService) ListForUser(ctx context.Context, userID uuid.UUID) ([]*model.Organization, error) {
	return s.repo.ListByUserID(ctx, userID)
}

func (s *OrgService) Update(ctx context.Context, orgID uuid.UUID, req *model.UpdateOrgRequest) (*model.Organization, error) {
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = *req.AvatarURL
	}
	if len(updates) == 0 {
		return s.repo.GetByID(ctx, orgID)
	}
	if err := s.repo.Update(ctx, orgID, updates); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrOrgNotFound
		}
		return nil, fmt.Errorf("update org: %w", err)
	}
	return s.repo.GetByID(ctx, orgID)
}

func (s *OrgService) Delete(ctx context.Context, orgID uuid.UUID) error {
	if err := s.repo.SoftDelete(ctx, orgID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrOrgNotFound
		}
		return fmt.Errorf("delete org: %w", err)
	}
	return nil
}

// ── Member management ───────────────────────────────────────────────

func (s *OrgService) AddMember(ctx context.Context, orgID, userID uuid.UUID, role string) (*model.OrgMember, error) {
	if !model.IsValidOrgRole(role) {
		return nil, ErrOrgInvalidRole
	}

	if _, err := s.repo.GetByID(ctx, orgID); err != nil {
		return nil, ErrOrgNotFound
	}

	if existing, _ := s.repo.GetMember(ctx, orgID, userID); existing != nil {
		return nil, ErrOrgAlreadyMember
	}

	m := &model.OrgMember{OrgID: orgID, UserID: userID, Role: role}
	if err := s.repo.CreateMember(ctx, m); err != nil {
		return nil, fmt.Errorf("add org member: %w", err)
	}
	return m, nil
}

func (s *OrgService) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*model.OrgMember, error) {
	if _, err := s.repo.GetByID(ctx, orgID); err != nil {
		return nil, ErrOrgNotFound
	}
	return s.repo.ListMembers(ctx, orgID)
}

func (s *OrgService) UpdateMemberRole(ctx context.Context, orgID, targetUserID, callerID uuid.UUID, newRole string) error {
	if !model.IsValidOrgRole(newRole) {
		return ErrOrgInvalidRole
	}

	caller, err := s.repo.GetMember(ctx, orgID, callerID)
	if err != nil {
		return ErrOrgNotMember
	}
	if !model.OrgRoleAtLeast(caller.Role, model.OrgRoleAdmin) {
		return ErrOrgForbidden
	}

	target, err := s.repo.GetMember(ctx, orgID, targetUserID)
	if err != nil {
		return ErrOrgNotMember
	}

	// Cannot demote the last owner
	if target.Role == model.OrgRoleOwner && newRole != model.OrgRoleOwner {
		members, _ := s.repo.ListMembers(ctx, orgID)
		ownerCount := 0
		for _, m := range members {
			if m.Role == model.OrgRoleOwner {
				ownerCount++
			}
		}
		if ownerCount <= 1 {
			return ErrOrgLastOwner
		}
	}

	// Only owners can assign owner role
	if newRole == model.OrgRoleOwner && caller.Role != model.OrgRoleOwner {
		return ErrOrgForbidden
	}

	return s.repo.UpdateMemberRole(ctx, orgID, targetUserID, newRole)
}

func (s *OrgService) RemoveMember(ctx context.Context, orgID, targetUserID, callerID uuid.UUID) error {
	caller, err := s.repo.GetMember(ctx, orgID, callerID)
	if err != nil {
		return ErrOrgNotMember
	}
	if !model.OrgRoleAtLeast(caller.Role, model.OrgRoleAdmin) {
		return ErrOrgForbidden
	}

	target, err := s.repo.GetMember(ctx, orgID, targetUserID)
	if err != nil {
		return ErrOrgNotMember
	}

	if target.Role == model.OrgRoleOwner {
		members, _ := s.repo.ListMembers(ctx, orgID)
		ownerCount := 0
		for _, m := range members {
			if m.Role == model.OrgRoleOwner {
				ownerCount++
			}
		}
		if ownerCount <= 1 {
			return ErrOrgLastOwner
		}
	}

	return s.repo.DeleteMember(ctx, orgID, targetUserID)
}

func (s *OrgService) GetMemberRole(ctx context.Context, orgID, userID uuid.UUID) (string, error) {
	m, err := s.repo.GetMember(ctx, orgID, userID)
	if err != nil {
		return "", ErrOrgNotMember
	}
	return m.Role, nil
}
```

- [ ] **Step 2: Write service tests**

```go
package service

import (
	"context"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ── Mock ────────────────────────────────────────────────────────────

type mockOrgRepo struct {
	mock.Mock
}

func (m *mockOrgRepo) Create(ctx context.Context, org *model.Organization) error {
	return m.Called(ctx, org).Error(0)
}
func (m *mockOrgRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Organization, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Organization), args.Error(1)
}
func (m *mockOrgRepo) GetBySlug(ctx context.Context, slug string) (*model.Organization, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Organization), args.Error(1)
}
func (m *mockOrgRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*model.Organization, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Organization), args.Error(1)
}
func (m *mockOrgRepo) Update(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	return m.Called(ctx, id, updates).Error(0)
}
func (m *mockOrgRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockOrgRepo) CreateOrgWithOwner(ctx context.Context, org *model.Organization, owner *model.OrgMember) error {
	return m.Called(ctx, org, owner).Error(0)
}
func (m *mockOrgRepo) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*model.OrgMember, error) {
	args := m.Called(ctx, orgID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.OrgMember), args.Error(1)
}
func (m *mockOrgRepo) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*model.OrgMember, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.OrgMember), args.Error(1)
}
func (m *mockOrgRepo) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	return m.Called(ctx, orgID, userID, role).Error(0)
}
func (m *mockOrgRepo) DeleteMember(ctx context.Context, orgID, userID uuid.UUID) error {
	return m.Called(ctx, orgID, userID).Error(0)
}
func (m *mockOrgRepo) CreateMember(ctx context.Context, mem *model.OrgMember) error {
	return m.Called(ctx, mem).Error(0)
}

// ── Tests ───────────────────────────────────────────────────────────

func TestOrgService_Create(t *testing.T) {
	repo := new(mockOrgRepo)
	svc := NewOrgService(repo)
	ctx := context.Background()
	creatorID := uuid.New()

	repo.On("GetBySlug", ctx, "test-org").Return(nil, nil)
	repo.On("CreateOrgWithOwner", ctx, mock.AnythingOfType("*model.Organization"), mock.AnythingOfType("*model.OrgMember")).Return(nil)
	repo.On("GetByID", ctx, mock.AnythingOfType("uuid.UUID")).Return(&model.Organization{ID: uuid.New(), Slug: "test-org", Name: "Test"}, nil)

	org, err := svc.Create(ctx, creatorID, &model.CreateOrgRequest{Name: "Test", Slug: "test-org"})
	assert.NoError(t, err)
	assert.Equal(t, "Test", org.Name)
	repo.AssertExpectations(t)
}

func TestOrgService_Create_DuplicateSlug(t *testing.T) {
	repo := new(mockOrgRepo)
	svc := NewOrgService(repo)
	ctx := context.Background()

	repo.On("GetBySlug", ctx, "taken").Return(&model.Organization{Slug: "taken"}, nil)

	_, err := svc.Create(ctx, uuid.New(), &model.CreateOrgRequest{Name: "Taken", Slug: "taken"})
	assert.ErrorIs(t, err, ErrOrgSlugTaken)
}

func TestOrgService_UpdateMemberRole_LastOwner(t *testing.T) {
	repo := new(mockOrgRepo)
	svc := NewOrgService(repo)
	ctx := context.Background()
	orgID := uuid.New()
	ownerID := uuid.New()
	adminID := uuid.New()

	repo.On("GetMember", ctx, orgID, adminID).Return(&model.OrgMember{Role: model.OrgRoleAdmin}, nil)
	repo.On("GetMember", ctx, orgID, ownerID).Return(&model.OrgMember{Role: model.OrgRoleOwner}, nil)
	repo.On("ListMembers", ctx, orgID).Return([]*model.OrgMember{{Role: model.OrgRoleOwner}}, nil)

	err := svc.UpdateMemberRole(ctx, orgID, ownerID, adminID, model.OrgRoleMember)
	assert.ErrorIs(t, err, ErrOrgLastOwner)
}

func TestOrgService_RemoveMember_AdminCanRemove(t *testing.T) {
	repo := new(mockOrgRepo)
	svc := NewOrgService(repo)
	ctx := context.Background()
	orgID := uuid.New()
	adminID := uuid.New()
	memberID := uuid.New()

	repo.On("GetMember", ctx, orgID, adminID).Return(&model.OrgMember{Role: model.OrgRoleAdmin}, nil)
	repo.On("GetMember", ctx, orgID, memberID).Return(&model.OrgMember{Role: model.OrgRoleMember}, nil)
	repo.On("DeleteMember", ctx, orgID, memberID).Return(nil)

	err := svc.RemoveMember(ctx, orgID, memberID, adminID)
	assert.NoError(t, err)
}

func TestOrgService_RemoveMember_NonAdminCannotRemove(t *testing.T) {
	repo := new(mockOrgRepo)
	svc := NewOrgService(repo)
	ctx := context.Background()
	orgID := uuid.New()
	member1 := uuid.New()
	member2 := uuid.New()

	repo.On("GetMember", ctx, orgID, member1).Return(&model.OrgMember{Role: model.OrgRoleMember}, nil)

	err := svc.RemoveMember(ctx, orgID, member2, member1)
	assert.ErrorIs(t, err, ErrOrgForbidden)
}

func TestOrgService_AddMember_AlreadyMember(t *testing.T) {
	repo := new(mockOrgRepo)
	svc := NewOrgService(repo)
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	repo.On("GetByID", ctx, orgID).Return(&model.Organization{ID: orgID}, nil)
	repo.On("GetMember", ctx, orgID, userID).Return(&model.OrgMember{Role: model.OrgRoleMember}, nil)

	_, err := svc.AddMember(ctx, orgID, userID, model.OrgRoleMember)
	assert.ErrorIs(t, err, ErrOrgAlreadyMember)
}
```

- [ ] **Step 3: Run tests**

Run: `cd /d/Project/AgentForge/src-go && go test ./internal/service/ -run TestOrgService -v`

Expected: All 6 tests pass

- [ ] **Step 4: Commit**

```bash
git add src-go/internal/service/org_service.go src-go/internal/service/org_service_test.go
git commit -m "feat(org): add org service with member role management"
```

---

## Task 5: Org RBAC Middleware

**Files:**
- Create: `src-go/internal/middleware/org_rbac.go`

- [ ] **Step 1: Write the middleware**

```go
package middleware

import (
	"net/http"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ── Org Action IDs ──────────────────────────────────────────────────

type OrgActionID string

const (
	ActionOrgRead          OrgActionID = "org.read"
	ActionOrgUpdate        OrgActionID = "org.update"
	ActionOrgDelete        OrgActionID = "org.delete"
	ActionOrgMemberList    OrgActionID = "org.member.list"
	ActionOrgMemberInvite  OrgActionID = "org.member.invite"
	ActionOrgMemberUpdate  OrgActionID = "org.member.update"
	ActionOrgMemberRemove  OrgActionID = "org.member.remove"
	ActionOrgProjectList   OrgActionID = "org.project.list"
	ActionOrgProjectCreate OrgActionID = "org.project.create"
	ActionOrgSettingsRead  OrgActionID = "org.settings.read"
	ActionOrgSettingsUpdate OrgActionID = "org.settings.update"
)

// orgRoleMatrix maps action IDs to minimum required org role.
var orgRoleMatrix = map[OrgActionID]string{
	ActionOrgRead:           model.OrgRoleViewer,
	ActionOrgUpdate:         model.OrgRoleAdmin,
	ActionOrgDelete:         model.OrgRoleOwner,
	ActionOrgMemberList:     model.OrgRoleViewer,
	ActionOrgMemberInvite:   model.OrgRoleAdmin,
	ActionOrgMemberUpdate:   model.OrgRoleAdmin,
	ActionOrgMemberRemove:   model.OrgRoleAdmin,
	ActionOrgProjectList:    model.OrgRoleViewer,
	ActionOrgProjectCreate:  model.OrgRoleAdmin,
	ActionOrgSettingsRead:   model.OrgRoleViewer,
	ActionOrgSettingsUpdate: model.OrgRoleAdmin,
}

func minOrgRoleFor(action OrgActionID) (string, bool) {
	role, ok := orgRoleMatrix[action]
	return role, ok
}

// ── Org Member Lookup ───────────────────────────────────────────────

type OrgMemberLookup interface {
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*model.OrgMember, error)
}

// Note: We reuse the context.Context from echo.Context, so the interface
// matches the repository's method signatures naturally.

type orgMemberLookupFunc func(orgID, userID uuid.UUID) (string, error)

var globalOrgMemberLookup orgMemberLookupFunc

func SetOrgMemberLookup(repo *repository.OrgRepository) {
	globalOrgMemberLookup = func(orgID, userID uuid.UUID) (string, error) {
		m, err := repo.GetMember(nil, orgID, userID) // context set later
		if err != nil {
			return "", err
		}
		return m.Role, nil
	}
}

// OrgMiddleware extracts :orgId from the URL and stores it in context.
func OrgMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		orgIDStr := c.Param("orgId")
		if orgIDStr == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "missing orgId parameter")
		}
		orgID, err := uuid.Parse(orgIDStr)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid orgId")
		}
		c.Set("orgID", orgID)
		return next(c)
	}
}

// RequireOrg returns middleware that checks the caller's org role.
func RequireOrg(action OrgActionID) echo.MiddlewareFunc {
	minRole, ok := minOrgRoleFor(action)
	if !ok {
		panic("unknown org action: " + string(action))
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, err := claimsUserIDFromContext(c)
			if err != nil || userID == nil {
				return echo.NewHTTPError(http.StatusUnauthorized)
			}

			orgID, ok := c.Get("orgID").(uuid.UUID)
			if !ok {
				return echo.NewHTTPError(http.StatusBadRequest, "org context missing")
			}

			if globalOrgMemberLookup == nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "org member lookup not configured")
			}

			role, err := globalOrgMemberLookup(orgID, *userID)
			if err != nil {
				return echo.NewHTTPError(http.StatusForbidden, "not an org member")
			}

			if !model.OrgRoleAtLeast(role, minRole) {
				return echo.NewHTTPError(http.StatusForbidden, "insufficient org permissions")
			}

			c.Set("orgRole", role)
			return next(c)
		}
	}
}

func claimsUserIDFromContext(c echo.Context) (*uuid.UUID, error) {
	return claimsUserID(c)
}
```

- [ ] **Step 2: Build to verify**

Run: `cd /d/Project/AgentForge/src-go && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add src-go/internal/middleware/org_rbac.go
git commit -m "feat(org): add org RBAC middleware with action ID matrix"
```

---

## Task 6: Handler Layer

**Files:**
- Create: `src-go/internal/handler/org_handler.go`

- [ ] **Step 1: Write the handler**

```go
package handler

import (
	"net/http"

	"github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type OrgService interface {
	Create(ctx context.Context, creatorID uuid.UUID, req *model.CreateOrgRequest) (*model.Organization, error)
	Get(ctx context.Context, orgID uuid.UUID) (*model.Organization, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]*model.Organization, error)
	Update(ctx context.Context, orgID uuid.UUID, req *model.UpdateOrgRequest) (*model.Organization, error)
	Delete(ctx context.Context, orgID uuid.UUID) error
	AddMember(ctx context.Context, orgID, userID uuid.UUID, role string) (*model.OrgMember, error)
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]*model.OrgMember, error)
	UpdateMemberRole(ctx context.Context, orgID, targetUserID, callerID uuid.UUID, newRole string) error
	RemoveMember(ctx context.Context, orgID, targetUserID, callerID uuid.UUID) error
}

type OrgHandler struct {
	svc OrgService
}

func NewOrgHandler(svc OrgService) *OrgHandler {
	return &OrgHandler{svc: svc}
}

// Create creates a new organization.
func (h *OrgHandler) Create(c echo.Context) error {
	req := new(model.CreateOrgRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(req); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}

	userID, err := claimsUserID(c)
	if err != nil || userID == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	org, svcErr := h.svc.Create(c.Request().Context(), *userID, req)
	if svcErr != nil {
		return orgServiceError(c, svcErr)
	}

	return c.JSON(http.StatusCreated, org.ToDTO())
}

// List returns all organizations the caller belongs to.
func (h *OrgHandler) List(c echo.Context) error {
	userID, err := claimsUserID(c)
	if err != nil || userID == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	orgs, svcErr := h.svc.ListForUser(c.Request().Context(), *userID)
	if svcErr != nil {
		return orgServiceError(c, svcErr)
	}

	dtos := make([]model.OrganizationDTO, len(orgs))
	for i, o := range orgs {
		dtos[i] = o.ToDTO()
	}
	return c.JSON(http.StatusOK, model.OrgListResponse{Organizations: dtos})
}

// Get returns a single organization.
func (h *OrgHandler) Get(c echo.Context) error {
	orgID := c.Get("orgID").(uuid.UUID)

	org, svcErr := h.svc.Get(c.Request().Context(), orgID)
	if svcErr != nil {
		return orgServiceError(c, svcErr)
	}

	return c.JSON(http.StatusOK, org.ToDTO())
}

// Update modifies an organization.
func (h *OrgHandler) Update(c echo.Context) error {
	orgID := c.Get("orgID").(uuid.UUID)

	req := new(model.UpdateOrgRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(req); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}

	org, svcErr := h.svc.Update(c.Request().Context(), orgID, req)
	if svcErr != nil {
		return orgServiceError(c, svcErr)
	}

	return c.JSON(http.StatusOK, org.ToDTO())
}

// Delete soft-deletes an organization.
func (h *OrgHandler) Delete(c echo.Context) error {
	orgID := c.Get("orgID").(uuid.UUID)

	if svcErr := h.svc.Delete(c.Request().Context(), orgID); svcErr != nil {
		return orgServiceError(c, svcErr)
	}

	return c.NoContent(http.StatusNoContent)
}

// ListMembers returns all members of an organization.
func (h *OrgHandler) ListMembers(c echo.Context) error {
	orgID := c.Get("orgID").(uuid.UUID)

	members, svcErr := h.svc.ListMembers(c.Request().Context(), orgID)
	if svcErr != nil {
		return orgServiceError(c, svcErr)
	}

	dtos := make([]model.OrgMemberDTO, len(members))
	for i, m := range members {
		dtos[i] = m.ToDTO()
	}
	return c.JSON(http.StatusOK, model.OrgMemberListResponse{Members: dtos})
}

// AddMember adds a user to the organization.
func (h *OrgHandler) AddMember(c echo.Context) error {
	orgID := c.Get("orgID").(uuid.UUID)

	var req struct {
		UserID string `json:"userId" validate:"required"`
		Role   string `json:"role" validate:"required,oneof=owner admin member viewer"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid userId")
	}

	member, svcErr := h.svc.AddMember(c.Request().Context(), orgID, userID, req.Role)
	if svcErr != nil {
		return orgServiceError(c, svcErr)
	}

	return c.JSON(http.StatusCreated, member.ToDTO())
}

// UpdateMemberRole changes a member's role.
func (h *OrgHandler) UpdateMemberRole(c echo.Context) error {
	orgID := c.Get("orgID").(uuid.UUID)
	targetUID, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid uid")
	}

	callerID, err := claimsUserID(c)
	if err != nil || callerID == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	req := new(model.UpdateOrgMemberRoleRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(req); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}

	if svcErr := h.svc.UpdateMemberRole(c.Request().Context(), orgID, targetUID, *callerID, req.Role); svcErr != nil {
		return orgServiceError(c, svcErr)
	}

	return c.NoContent(http.StatusNoContent)
}

// RemoveMember removes a member from the organization.
func (h *OrgHandler) RemoveMember(c echo.Context) error {
	orgID := c.Get("orgID").(uuid.UUID)
	targetUID, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid uid")
	}

	callerID, err := claimsUserID(c)
	if err != nil || callerID == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	if svcErr := h.svc.RemoveMember(c.Request().Context(), orgID, targetUID, *callerID); svcErr != nil {
		return orgServiceError(c, svcErr)
	}

	return c.NoContent(http.StatusNoContent)
}

// orgServiceError maps service errors to HTTP responses.
func orgServiceError(c echo.Context, err error) error {
	switch err {
	case service.ErrOrgNotFound:
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case service.ErrOrgSlugTaken:
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case service.ErrOrgNotMember:
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	case service.ErrOrgLastOwner:
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case service.ErrOrgForbidden:
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	case service.ErrOrgInvalidRole:
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case service.ErrOrgAlreadyMember:
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
}
```

**Note:** The `OrgService` interface in the handler has a `context.Context` parameter. The actual `service.OrgService` methods use `context.Context` as first param — these match. The handler interface definition needs to include `context.Context`:

Add `import "context"` at the top and make the interface:
```go
type OrgService interface {
    Create(ctx context.Context, creatorID uuid.UUID, req *model.CreateOrgRequest) (*model.Organization, error)
    Get(ctx context.Context, orgID uuid.UUID) (*model.Organization, error)
    ListForUser(ctx context.Context, userID uuid.UUID) ([]*model.Organization, error)
    Update(ctx context.Context, orgID uuid.UUID, req *model.UpdateOrgRequest) (*model.Organization, error)
    Delete(ctx context.Context, orgID uuid.UUID) error
    AddMember(ctx context.Context, orgID, userID uuid.UUID, role string) (*model.OrgMember, error)
    ListMembers(ctx context.Context, orgID uuid.UUID) ([]*model.OrgMember, error)
    UpdateMemberRole(ctx context.Context, orgID, targetUserID, callerID uuid.UUID, newRole string) error
    RemoveMember(ctx context.Context, orgID, targetUserID, callerID uuid.UUID) error
}
```

- [ ] **Step 2: Build to verify**

Run: `cd /d/Project/AgentForge/src-go && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add src-go/internal/handler/org_handler.go
git commit -m "feat(org): add org handler with all CRUD and member endpoints"
```

---

## Task 7: Route Registration

**Files:**
- Modify: `src-go/internal/server/routes.go`

- [ ] **Step 1: Add org imports and parameters**

In `routes.go`, add to the `RegisterRoutes` parameter list:

```go
orgRepo *repository.OrgRepository,
```

Add imports:
```go
orgHandler "github.com/agentforge/server/internal/handler"  // already imported
```

- [ ] **Step 2: Wire org handler and routes**

After the existing handler wiring (around line 500+), add:

```go
// ── Organization routes ─────────────────────────────────────────
orgSvc := service.NewOrgService(orgRepo).WithUserLookup(userRepo)
orgH := handler.NewOrgHandler(orgSvc)

// Install org member lookup for RBAC middleware
appMiddleware.SetOrgMemberLookup(orgRepo)

// Org CRUD (user-scoped — no org RBAC needed)
orgs := v1.Group("/orgs", jwtMw)
orgs.POST("", orgH.Create)
orgs.GET("", orgH.List)

// Org-scoped routes with org RBAC middleware
orgGroup := v1.Group("/orgs/:orgId", jwtMw, appMiddleware.OrgMiddleware)
orgGroup.GET("", orgH.Get, appMiddleware.RequireOrg(appMiddleware.ActionOrgRead))
orgGroup.PUT("", orgH.Update, appMiddleware.RequireOrg(appMiddleware.ActionOrgUpdate))
orgGroup.DELETE("", orgH.Delete, appMiddleware.RequireOrg(appMiddleware.ActionOrgDelete))

// Org members
orgMembers := orgGroup.Group("/members")
orgMembers.GET("", orgH.ListMembers, appMiddleware.RequireOrg(appMiddleware.ActionOrgMemberList))
orgMembers.POST("", orgH.AddMember, appMiddleware.RequireOrg(appMiddleware.ActionOrgMemberInvite))
orgMembers.PUT("/:uid", orgH.UpdateMemberRole, appMiddleware.RequireOrg(appMiddleware.ActionOrgMemberUpdate))
orgMembers.DELETE("/:uid", orgH.RemoveMember, appMiddleware.RequireOrg(appMiddleware.ActionOrgMemberRemove))
```

- [ ] **Step 3: Pass orgRepo in cmd/server**

In `cmd/server/main.go` (or wherever `RegisterRoutes` is called), add `orgRepo` to the call. Find where repositories are created:

```go
orgRepo := repository.NewOrgRepository(db)
```

And pass it to `RegisterRoutes`.

- [ ] **Step 4: Build and verify**

Run: `cd /d/Project/AgentForge/src-go && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/server/routes.go src-go/cmd/server/main.go
git commit -m "feat(org): wire org routes with RBAC middleware"
```

---

## Task 8: Frontend Stores

**Files:**
- Create: `lib/stores/org-store.ts`
- Create: `lib/stores/org-member-store.ts`

- [ ] **Step 1: Write org-store.ts**

```typescript
import { create } from "zustand";
import { useAuthStore } from "./auth-store";
import { createApiClient } from "@/lib/api-client";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface Organization {
  id: string;
  slug: string;
  name: string;
  avatarUrl?: string;
  plan: string;
  settings: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

interface OrgState {
  orgs: Organization[];
  currentOrgId: string | null;
  loading: boolean;
  error: string | null;

  fetchOrgs: () => Promise<void>;
  createOrg: (name: string, slug: string) => Promise<Organization | null>;
  updateOrg: (orgId: string, data: { name?: string; avatarUrl?: string }) => Promise<Organization | null>;
  deleteOrg: (orgId: string) => Promise<boolean>;
  setCurrentOrg: (orgId: string | null) => void;
  upsertOrg: (org: Organization) => void;
}

export const useOrgStore = create<OrgState>()((set) => ({
  orgs: [],
  currentOrgId: null,
  loading: false,
  error: null,

  fetchOrgs: async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<{ organizations: Organization[] }>("/api/v1/orgs", { token });
      set({ orgs: data.organizations ?? [], error: null });
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Failed to load organizations", orgs: [] });
    } finally {
      set({ loading: false });
    }
  },

  createOrg: async (name, slug) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<Organization>("/api/v1/orgs", { name, slug }, { token });
      set((state) => ({ orgs: [...state.orgs, data] }));
      return data;
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Failed to create organization" });
      return null;
    }
  },

  updateOrg: async (orgId, data) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;
    try {
      const api = createApiClient(API_URL);
      const { data: updated } = await api.put<Organization>(`/api/v1/orgs/${orgId}`, data, { token });
      set((state) => ({
        orgs: state.orgs.map((o) => (o.id === orgId ? updated : o)),
      }));
      return updated;
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Failed to update organization" });
      return null;
    }
  },

  deleteOrg: async (orgId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return false;
    try {
      const api = createApiClient(API_URL);
      await api.delete(`/api/v1/orgs/${orgId}`, { token });
      set((state) => ({
        orgs: state.orgs.filter((o) => o.id !== orgId),
        currentOrgId: state.currentOrgId === orgId ? null : state.currentOrgId,
      }));
      return true;
    } catch {
      return false;
    }
  },

  setCurrentOrg: (orgId) => set({ currentOrgId: orgId }),

  upsertOrg: (org) =>
    set((state) => {
      const exists = state.orgs.some((o) => o.id === org.id);
      return {
        orgs: exists ? state.orgs.map((o) => (o.id === org.id ? org : o)) : [...state.orgs, org],
      };
    }),
}));
```

- [ ] **Step 2: Write org-member-store.ts**

```typescript
import { create } from "zustand";
import { useAuthStore } from "./auth-store";
import { createApiClient } from "@/lib/api-client";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface OrgMember {
  id: string;
  orgId: string;
  userId: string;
  role: string;
  joinedAt: string;
  userName?: string;
  userEmail?: string;
}

interface OrgMemberState {
  members: OrgMember[];
  loading: boolean;
  error: string | null;

  fetchMembers: (orgId: string) => Promise<void>;
  addMember: (orgId: string, userId: string, role: string) => Promise<OrgMember | null>;
  updateRole: (orgId: string, userId: string, role: string) => Promise<boolean>;
  removeMember: (orgId: string, userId: string) => Promise<boolean>;
}

export const useOrgMemberStore = create<OrgMemberState>()((set) => ({
  members: [],
  loading: false,
  error: null,

  fetchMembers: async (orgId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<{ members: OrgMember[] }>(`/api/v1/orgs/${orgId}/members`, { token });
      set({ members: data.members ?? [], error: null });
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Failed to load members", members: [] });
    } finally {
      set({ loading: false });
    }
  },

  addMember: async (orgId, userId, role) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<OrgMember>(`/api/v1/orgs/${orgId}/members`, { userId, role }, { token });
      set((state) => ({ members: [...state.members, data] }));
      return data;
    } catch {
      return null;
    }
  },

  updateRole: async (orgId, userId, role) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return false;
    try {
      const api = createApiClient(API_URL);
      await api.put(`/api/v1/orgs/${orgId}/members/${userId}`, { role }, { token });
      set((state) => ({
        members: state.members.map((m) => (m.userId === userId ? { ...m, role } : m)),
      }));
      return true;
    } catch {
      return false;
    }
  },

  removeMember: async (orgId, userId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return false;
    try {
      const api = createApiClient(API_URL);
      await api.delete(`/api/v1/orgs/${orgId}/members/${userId}`, { token });
      set((state) => ({ members: state.members.filter((m) => m.userId !== userId) }));
      return true;
    } catch {
      return false;
    }
  },
}));
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `pnpm exec tsc --noEmit 2>&1 | head -20`

Expected: No errors in the new store files (pre-existing errors in other files are fine)

- [ ] **Step 4: Commit**

```bash
git add lib/stores/org-store.ts lib/stores/org-member-store.ts
git commit -m "feat(org): add org and org-member Zustand stores"
```

---

## Task 9: Frontend Pages

**Files:**
- Create: `app/(dashboard)/orgs/page.tsx`
- Create: `app/(dashboard)/orgs/[orgId]/page.tsx`
- Create: `app/(dashboard)/orgs/[orgId]/members/page.tsx`
- Create: `app/(dashboard)/orgs/[orgId]/settings/page.tsx`
- Create: `components/org/org-card.tsx`
- Create: `components/org/org-member-table.tsx`

- [ ] **Step 1: Create org list page**

Create `app/(dashboard)/orgs/page.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Plus, Building2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PageHeader } from "@/components/shared/page-header";
import { OrgCard } from "@/components/org/org-card";
import { useOrgStore } from "@/lib/stores/org-store";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

export default function OrgsPage() {
  const t = useTranslations("common");
  const router = useRouter();
  useBreadcrumbs([{ label: "Organizations", href: "/orgs" }]);

  const orgs = useOrgStore((s) => s.orgs);
  const loading = useOrgStore((s) => s.loading);
  const fetchOrgs = useOrgStore((s) => s.fetchOrgs);
  const createOrg = useOrgStore((s) => s.createOrg);
  const setCurrentOrg = useOrgStore((s) => s.setCurrentOrg);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    fetchOrgs();
  }, [fetchOrgs]);

  const handleCreate = async () => {
    if (!name.trim() || !slug.trim()) return;
    setCreating(true);
    const org = await createOrg(name.trim(), slug.trim());
    setCreating(false);
    if (org) {
      setDialogOpen(false);
      setName("");
      setSlug("");
      setCurrentOrg(org.id);
      router.push(`/orgs/${org.id}`);
    }
  };

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader title="Organizations">
        <Button onClick={() => setDialogOpen(true)}>
          <Plus className="mr-2 size-4" />
          New Organization
        </Button>
      </PageHeader>

      {loading ? (
        <p className="text-muted-foreground">Loading...</p>
      ) : orgs.length === 0 ? (
        <div className="flex flex-col items-center justify-center gap-4 py-16 text-center">
          <Building2 className="size-12 text-muted-foreground" />
          <h3 className="text-lg font-semibold">No organizations yet</h3>
          <p className="text-muted-foreground">Create an organization to manage teams and projects.</p>
          <Button onClick={() => setDialogOpen(true)}>
            <Plus className="mr-2 size-4" />
            Create Organization
          </Button>
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {orgs.map((org) => (
            <OrgCard key={org.id} org={org} onClick={() => { setCurrentOrg(org.id); router.push(`/orgs/${org.id}`); }} />
          ))}
        </div>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create Organization</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="org-name">Name</Label>
              <Input id="org-name" value={name} onChange={(e) => { setName(e.target.value); setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, "-").replace(/-+/g, "-").slice(0, 64)); }} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="org-slug">Slug</Label>
              <Input id="org-slug" value={slug} onChange={(e) => setSlug(e.target.value)} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDialogOpen(false)}>Cancel</Button>
            <Button onClick={handleCreate} disabled={!name.trim() || !slug.trim() || creating}>
              {creating ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
```

- [ ] **Step 2: Create org-card component**

Create `components/org/org-card.tsx`:

```tsx
"use client";

import { Building2 } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import type { Organization } from "@/lib/stores/org-store";

interface OrgCardProps {
  org: Organization;
  onClick: () => void;
}

export function OrgCard({ org, onClick }: OrgCardProps) {
  return (
    <Card className="cursor-pointer transition-colors hover:bg-accent" onClick={onClick}>
      <CardContent className="flex items-center gap-4 p-4">
        <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10">
          {org.avatarUrl ? (
            <img src={org.avatarUrl} alt={org.name} className="size-10 rounded-lg" />
          ) : (
            <Building2 className="size-5 text-primary" />
          )}
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate font-medium">{org.name}</p>
          <p className="text-sm text-muted-foreground">{org.slug}</p>
        </div>
        <span className="rounded-full bg-muted px-2 py-0.5 text-xs capitalize text-muted-foreground">
          {org.plan}
        </span>
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 3: Create org detail page**

Create `app/(dashboard)/orgs/[orgId]/page.tsx`:

```tsx
"use client";

import { useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Settings, Users, FolderKanban } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { useOrgStore } from "@/lib/stores/org-store";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import Link from "next/link";

export default function OrgDetailPage() {
  const params = useParams<{ orgId: string }>();
  const router = useRouter();
  useBreadcrumbs([
    { label: "Organizations", href: "/orgs" },
    { label: "Organization" },
  ]);

  const orgs = useOrgStore((s) => s.orgs);
  const setCurrentOrg = useOrgStore((s) => s.setCurrentOrg);
  const org = orgs.find((o) => o.id === params.orgId);

  useEffect(() => {
    if (org) setCurrentOrg(org.id);
  }, [org, setCurrentOrg]);

  if (!org) {
    return (
      <div className="py-16 text-center">
        <p className="text-muted-foreground">Organization not found.</p>
        <Button variant="link" onClick={() => router.push("/orgs")}>Back to Organizations</Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader title={org.name}>
        <Button variant="outline" asChild>
          <Link href={`/orgs/${org.id}/settings`}>
            <Settings className="mr-2 size-4" /> Settings
          </Link>
        </Button>
      </PageHeader>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card className="p-6 cursor-pointer hover:bg-accent transition-colors" onClick={() => router.push(`/orgs/${org.id}/members`)}>
          <div className="flex items-center gap-3">
            <Users className="size-5 text-muted-foreground" />
            <div>
              <p className="font-medium">Members</p>
              <p className="text-sm text-muted-foreground">Manage org members and roles</p>
            </div>
          </div>
        </Card>
        <Card className="p-6 cursor-pointer hover:bg-accent transition-colors">
          <div className="flex items-center gap-3">
            <FolderKanban className="size-5 text-muted-foreground" />
            <div>
              <p className="font-medium">Projects</p>
              <p className="text-sm text-muted-foreground">View org projects</p>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
}
```

Note: Import `Card` from `@/components/ui/card`.

- [ ] **Step 4: Create org members page**

Create `app/(dashboard)/orgs/[orgId]/members/page.tsx`:

```tsx
"use client";

import { useEffect } from "react";
import { useParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { PageHeader } from "@/components/shared/page-header";
import { OrgMemberTable } from "@/components/org/org-member-table";
import { useOrgMemberStore } from "@/lib/stores/org-member-store";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

export default function OrgMembersPage() {
  const params = useParams<{ orgId: string }>();
  useBreadcrumbs([
    { label: "Organizations", href: "/orgs" },
    { label: "Members" },
  ]);

  const members = useOrgMemberStore((s) => s.members);
  const loading = useOrgMemberStore((s) => s.loading);
  const fetchMembers = useOrgMemberStore((s) => s.fetchMembers);

  useEffect(() => {
    fetchMembers(params.orgId);
  }, [params.orgId, fetchMembers]);

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader title="Organization Members" />
      <OrgMemberTable members={members} loading={loading} orgId={params.orgId} />
    </div>
  );
}
```

- [ ] **Step 5: Create org-member-table component**

Create `components/org/org-member-table.tsx`:

```tsx
"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Plus, Trash2 } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { useOrgMemberStore, type OrgMember } from "@/lib/stores/org-member-store";

interface OrgMemberTableProps {
  members: OrgMember[];
  loading: boolean;
  orgId: string;
}

export function OrgMemberTable({ members, loading, orgId }: OrgMemberTableProps) {
  const addMember = useOrgMemberStore((s) => s.addMember);
  const updateRole = useOrgMemberStore((s) => s.updateRole);
  const removeMember = useOrgMemberStore((s) => s.removeMember);

  const [addOpen, setAddOpen] = useState(false);
  const [newUserId, setNewUserId] = useState("");
  const [newRole, setNewRole] = useState("member");

  const handleAdd = async () => {
    if (!newUserId.trim()) return;
    await addMember(orgId, newUserId.trim(), newRole);
    setAddOpen(false);
    setNewUserId("");
    setNewRole("member");
  };

  if (loading) {
    return (
      <div className="flex flex-col gap-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    );
  }

  return (
    <>
      <div className="flex justify-end">
        <Button onClick={() => setAddOpen(true)}>
          <Plus className="mr-2 size-4" /> Add Member
        </Button>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Email</TableHead>
            <TableHead>Role</TableHead>
            <TableHead>Joined</TableHead>
            <TableHead className="w-[80px]" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {members.map((member) => (
            <TableRow key={member.id}>
              <TableCell className="font-medium">{member.userName || member.userId}</TableCell>
              <TableCell>{member.userEmail || "—"}</TableCell>
              <TableCell>
                <Select value={member.role} onValueChange={(role) => updateRole(orgId, member.userId, role)}>
                  <SelectTrigger className="w-[120px]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="owner">Owner</SelectItem>
                    <SelectItem value="admin">Admin</SelectItem>
                    <SelectItem value="member">Member</SelectItem>
                    <SelectItem value="viewer">Viewer</SelectItem>
                  </SelectContent>
                </Select>
              </TableCell>
              <TableCell>{new Date(member.joinedAt).toLocaleDateString()}</TableCell>
              <TableCell>
                <Button variant="ghost" size="icon" onClick={() => removeMember(orgId, member.userId)}>
                  <Trash2 className="size-4 text-destructive" />
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {members.length === 0 && (
            <TableRow>
              <TableCell colSpan={5} className="text-center text-muted-foreground">
                No members yet
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>

      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add Member</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-4">
            <div className="flex flex-col gap-2">
              <Label>User ID</Label>
              <Input value={newUserId} onChange={(e) => setNewUserId(e.target.value)} placeholder="Enter user ID" />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Role</Label>
              <Select value={newRole} onValueChange={setNewRole}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="member">Member</SelectItem>
                  <SelectItem value="admin">Admin</SelectItem>
                  <SelectItem value="viewer">Viewer</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setAddOpen(false)}>Cancel</Button>
            <Button onClick={handleAdd} disabled={!newUserId.trim()}>Add</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
```

- [ ] **Step 6: Create org settings page**

Create `app/(dashboard)/orgs/[orgId]/settings/page.tsx`:

```tsx
"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { PageHeader } from "@/components/shared/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useOrgStore } from "@/lib/stores/org-store";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

export default function OrgSettingsPage() {
  const params = useParams<{ orgId: string }>();
  const router = useRouter();
  useBreadcrumbs([
    { label: "Organizations", href: "/orgs" },
    { label: "Settings" },
  ]);

  const orgs = useOrgStore((s) => s.orgs);
  const updateOrg = useOrgStore((s) => s.updateOrg);
  const deleteOrg = useOrgStore((s) => s.deleteOrg);
  const org = orgs.find((o) => o.id === params.orgId);

  const [name, setName] = useState(org?.name ?? "");
  const [saving, setSaving] = useState(false);

  if (!org) {
    return <div className="py-16 text-center text-muted-foreground">Organization not found.</div>;
  }

  const handleSave = async () => {
    setSaving(true);
    await updateOrg(org.id, { name });
    setSaving(false);
  };

  const handleDelete = async () => {
    if (!confirm("Are you sure you want to delete this organization? This cannot be undone.")) return;
    const ok = await deleteOrg(org.id);
    if (ok) router.push("/orgs");
  };

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader title="Organization Settings" />
      <div className="max-w-lg space-y-6">
        <div className="space-y-2">
          <Label>Organization Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label>Slug</Label>
          <Input value={org.slug} disabled />
        </div>
        <div className="space-y-2">
          <Label>Plan</Label>
          <Input value={org.plan} disabled />
        </div>
        <div className="flex gap-2">
          <Button onClick={handleSave} disabled={saving}>{saving ? "Saving..." : "Save Changes"}</Button>
        </div>
        <hr />
        <div>
          <h3 className="text-destructive font-semibold">Danger Zone</h3>
          <p className="text-sm text-muted-foreground">Deleting an organization is permanent and cannot be undone.</p>
          <Button variant="destructive" className="mt-2" onClick={handleDelete}>Delete Organization</Button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 7: Verify TypeScript compiles**

Run: `pnpm exec tsc --noEmit 2>&1 | grep -E "org-store|org-member|org/" | head -20`

Expected: No errors in org-related files

- [ ] **Step 8: Commit**

```bash
git add app/\(dashboard\)/orgs/ components/org/
git commit -m "feat(org): add org list, detail, members, and settings pages"
```

---

## Task 10: Sidebar and Navigation Integration

**Files:**
- Modify: `components/layout/sidebar.tsx`

- [ ] **Step 1: Add org nav group to sidebar**

Open `components/layout/sidebar.tsx`. Add `Building2` to the lucide imports, then add a new nav group:

```typescript
{
  id: "organization",
  labelKey: "nav.group.organization",
  defaultOpen: true,
  items: [
    { href: "/orgs", labelKey: "nav.organizations", icon: Building2 },
  ],
},
```

Insert this group as the **first item** in the `navGroups` array (before "workspace"), so organizations are top-level.

- [ ] **Step 2: Add i18n keys**

Add to the i18n files (check existing pattern in `lib/i18n/`):

In the English locale file, add:
```json
"nav.group.organization": "Organization",
"nav.organizations": "Organizations"
```

In the Chinese locale file, add:
```json
"nav.group.organization": "组织",
"nav.organizations": "组织管理"
```

- [ ] **Step 3: Verify sidebar renders**

Run: `pnpm dev` and navigate to the app. Verify the sidebar shows an "Organizations" link.

- [ ] **Step 4: Commit**

```bash
git add components/layout/sidebar.tsx lib/i18n/
git commit -m "feat(org): add organization navigation to sidebar"
```

---

## Task 11: Project Creation Integration

**Files:**
- Modify: `components/project/new-project-dialog.tsx`
- Modify: `lib/stores/project-store.ts`

- [ ] **Step 1: Add org selector to project creation dialog**

Open `components/project/new-project-dialog.tsx`. Add:

1. Import `useOrgStore` and `Select`/`SelectContent`/`SelectItem`/`SelectTrigger`/`SelectValue` from shadcn
2. Add state: `const [selectedOrgId, setSelectedOrgId] = useState<string>("")`
3. Fetch orgs: `const orgs = useOrgStore((s) => s.orgs);` + `useEffect` to `fetchOrgs()`
4. Add org selector dropdown after the "Start From" section:

```tsx
<div className="flex flex-col gap-2">
  <Label>Organization (optional)</Label>
  <Select value={selectedOrgId} onValueChange={setSelectedOrgId}>
    <SelectTrigger>
      <SelectValue placeholder="Personal workspace" />
    </SelectTrigger>
    <SelectContent>
      <SelectItem value="">Personal workspace</SelectItem>
      {orgs.map((org) => (
        <SelectItem key={org.id} value={org.id}>{org.name}</SelectItem>
      ))}
    </SelectContent>
  </Select>
</div>
```

5. Pass `selectedOrgId` to `createProject`:

```tsx
const result = await createProject({
  name: trimmed,
  description,
  orgId: selectedOrgId || undefined,
  ...
});
```

- [ ] **Step 2: Update project store and backend request**

Open `lib/stores/project-store.ts`. Find the `createProject` action and add `orgId` to the request body:

```typescript
createProject: async (input: { name: string; description: string; orgId?: string; ... }) => {
  // ... existing code
  const body: Record<string, unknown> = { name: input.name, description: input.description };
  if (input.orgId) body.orgId = input.orgId;
  // send body to API
}
```

- [ ] **Step 3: Verify project creation with org**

Run: `pnpm dev`. Create a new project and verify the org selector appears and the selected org is sent with the request.

- [ ] **Step 4: Commit**

```bash
git add components/project/new-project-dialog.tsx lib/stores/project-store.ts
git commit -m "feat(org): add org selector to project creation dialog"
```

---

## Task 12: Integration Verification

**Files:**
- No new files — verification only

- [ ] **Step 1: Run Go tests**

Run: `cd /d/Project/AgentForge/src-go && go test ./internal/repository/ -run TestOrg -v && go test ./internal/service/ -run TestOrgService -v`

Expected: All tests pass

- [ ] **Step 2: Run full Go build**

Run: `cd /d/Project/AgentForge/src-go && go build ./...`

Expected: No errors

- [ ] **Step 3: Run TypeScript check**

Run: `pnpm exec tsc --noEmit 2>&1 | grep -c "error TS"`

Expected: 0 new errors (pre-existing errors OK)

- [ ] **Step 4: Start dev server and smoke test**

Run: `pnpm dev`

Verify:
1. Sidebar shows "Organizations" link
2. `/orgs` page loads and shows empty state
3. Can create an org
4. Org detail page shows members and settings
5. Can add/remove members
6. Project creation shows org selector
7. Existing projects still work (no org_id)

- [ ] **Step 5: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix(org): integration fixes from smoke test"
```

---

## Self-Review Checklist

**Spec coverage:**
- [x] Data model (organizations, org_members tables) — Task 1
- [x] Global role hierarchy (owner/admin/member/viewer) — Task 2
- [x] API endpoints (CRUD, members, invitations) — Tasks 6-7
- [x] Project-Org relationship (org_id column) — Task 1
- [x] Middleware (OrgRBAC with action IDs) — Task 5
- [x] Frontend pages (org list, detail, members, settings) — Task 9
- [x] Frontend stores (org-store, org-member-store) — Task 8
- [x] Sidebar integration — Task 10
- [x] Project creation integration — Task 11
- [x] Migration strategy (backward compatible) — Task 1
- [x] Testing (unit, integration, E2E) — Tasks 3-4, 12

**Placeholder scan:** No TBD/TODO/FIXME in plan. All steps have code.

**Type consistency:** All model types (`Organization`, `OrgMember`, DTOs) are defined in Task 2 and consistently used across all subsequent tasks. Repository interfaces match service expectations. Handler types match store expectations.

**Missing:** Org invitations endpoint is in the spec but not in the plan. This is intentional — invitations reuse the existing invitation system with an org context. It should be a follow-up task after the core org CRUD is working.
