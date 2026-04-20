package qianchuan

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openOAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&OAuthState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRepo_CreateAndLookup(t *testing.T) {
	db := openOAuthTestDB(t)
	repo := NewOAuthStateRepo(db)
	ctx := context.Background()

	dn := "Test Store"
	empID := uuid.New()
	state := &OAuthState{
		StateToken:       uuid.New(),
		ProjectID:        uuid.New(),
		RedirectURI:      "http://localhost:7777/api/v1/qianchuan/oauth/callback",
		InitiatedBy:      uuid.New(),
		DisplayName:      &dn,
		ActingEmployeeID: &empID,
		ExpiresAt:        time.Now().UTC().Add(10 * time.Minute),
	}
	require.NoError(t, repo.Create(ctx, state))

	got, err := repo.Lookup(ctx, state.StateToken)
	require.NoError(t, err)
	assert.Equal(t, state.ProjectID, got.ProjectID)
	assert.Equal(t, state.RedirectURI, got.RedirectURI)
	assert.Equal(t, state.InitiatedBy, got.InitiatedBy)
	assert.Equal(t, *state.DisplayName, *got.DisplayName)
	assert.Equal(t, *state.ActingEmployeeID, *got.ActingEmployeeID)
	assert.Nil(t, got.ConsumedAt)
}

func TestRepo_LookupConsumed(t *testing.T) {
	db := openOAuthTestDB(t)
	repo := NewOAuthStateRepo(db)
	ctx := context.Background()

	state := &OAuthState{
		StateToken:  uuid.New(),
		ProjectID:   uuid.New(),
		RedirectURI: "http://localhost/cb",
		InitiatedBy: uuid.New(),
		ExpiresAt:   time.Now().UTC().Add(10 * time.Minute),
	}
	require.NoError(t, repo.Create(ctx, state))
	require.NoError(t, repo.MarkConsumed(ctx, state.StateToken))

	_, err := repo.Lookup(ctx, state.StateToken)
	assert.ErrorIs(t, err, ErrStateConsumed)
}

func TestRepo_LookupExpired(t *testing.T) {
	db := openOAuthTestDB(t)
	repo := NewOAuthStateRepo(db)
	ctx := context.Background()

	state := &OAuthState{
		StateToken:  uuid.New(),
		ProjectID:   uuid.New(),
		RedirectURI: "http://localhost/cb",
		InitiatedBy: uuid.New(),
		ExpiresAt:   time.Now().UTC().Add(-1 * time.Second),
	}
	require.NoError(t, repo.Create(ctx, state))

	_, err := repo.Lookup(ctx, state.StateToken)
	assert.ErrorIs(t, err, ErrStateExpired)
}

func TestRepo_LookupMissing(t *testing.T) {
	db := openOAuthTestDB(t)
	repo := NewOAuthStateRepo(db)
	ctx := context.Background()

	_, err := repo.Lookup(ctx, uuid.New())
	assert.ErrorIs(t, err, ErrStateNotFound)
}

func TestRepo_DeleteExpired(t *testing.T) {
	db := openOAuthTestDB(t)
	repo := NewOAuthStateRepo(db)
	ctx := context.Background()

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		s := &OAuthState{
			StateToken:  uuid.New(),
			ProjectID:   uuid.New(),
			RedirectURI: "http://localhost/cb",
			InitiatedBy: uuid.New(),
			ExpiresAt:   now.Add(-time.Duration(i+1) * time.Minute),
		}
		require.NoError(t, repo.Create(ctx, s))
	}
	fresh := &OAuthState{
		StateToken:  uuid.New(),
		ProjectID:   uuid.New(),
		RedirectURI: "http://localhost/cb",
		InitiatedBy: uuid.New(),
		ExpiresAt:   now.Add(10 * time.Minute),
	}
	require.NoError(t, repo.Create(ctx, fresh))

	deleted, err := repo.DeleteExpired(ctx, now)
	require.NoError(t, err)
	assert.Equal(t, int64(3), deleted)

	got, err := repo.Lookup(ctx, fresh.StateToken)
	require.NoError(t, err)
	assert.Equal(t, fresh.StateToken, got.StateToken)
}
