package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentforge/server/internal/config"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// --- Mock repositories with configurable errors ---

type mockUserRepo struct {
	users     map[string]*model.User
	createErr error
	getErr    error // non-ErrNoRows error to inject
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[string]*model.User)}
}

func (m *mockUserRepo) Create(_ context.Context, user *model.User) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.users[user.Email] = user
	return nil
}

func (m *mockUserRepo) GetByEmail(_ context.Context, email string) (*model.User, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	u, ok := m.users[email]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *mockUserRepo) UpdateName(_ context.Context, id uuid.UUID, name string) error {
	for _, u := range m.users {
		if u.ID == id {
			u.Name = name
			return nil
		}
	}
	return repository.ErrNotFound
}

func (m *mockUserRepo) UpdatePassword(_ context.Context, id uuid.UUID, hashedPassword string) error {
	for _, u := range m.users {
		if u.ID == id {
			u.Password = hashedPassword
			return nil
		}
	}
	return repository.ErrNotFound
}

type mockCacheRepo struct {
	refreshTokens map[string]string
	blacklist     map[string]bool
	blacklistErr  error
	setErr        error
	getErr        error
	deleteErr     error
}

func newMockCacheRepo() *mockCacheRepo {
	return &mockCacheRepo{
		refreshTokens: make(map[string]string),
		blacklist:     make(map[string]bool),
	}
}

func (m *mockCacheRepo) SetRefreshToken(_ context.Context, userID, token string, _ time.Duration) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.refreshTokens[userID] = token
	return nil
}

func (m *mockCacheRepo) GetRefreshToken(_ context.Context, userID string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	t, ok := m.refreshTokens[userID]
	if !ok {
		return "", repository.ErrNotFound
	}
	return t, nil
}

func (m *mockCacheRepo) DeleteRefreshToken(_ context.Context, userID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.refreshTokens, userID)
	return nil
}

func (m *mockCacheRepo) BlacklistToken(_ context.Context, jti string, _ time.Duration) error {
	if m.blacklistErr != nil {
		return m.blacklistErr
	}
	m.blacklist[jti] = true
	return nil
}

func (m *mockCacheRepo) IsBlacklisted(_ context.Context, jti string) (bool, error) {
	return m.blacklist[jti], nil
}

// --- Helpers ---

func testConfig() *config.Config {
	return &config.Config{
		JWTSecret:     "test-secret-at-least-32-characters-long",
		JWTAccessTTL:  15 * time.Minute,
		JWTRefreshTTL: 7 * 24 * time.Hour,
		Env:           "test",
	}
}

func seedUser(repo *mockUserRepo, email, password, name string) *model.User {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	u := &model.User{
		ID:        uuid.New(),
		Email:     email,
		Password:  string(hash),
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.users[email] = u
	return u
}

// --- Register Tests ---

func TestRegister_Success(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	resp, err := svc.Register(context.Background(), &model.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if resp.User.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", resp.User.Email)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	seedUser(userRepo, "dup@example.com", "password", "Existing")

	_, err := svc.Register(context.Background(), &model.RegisterRequest{
		Email:    "dup@example.com",
		Password: "password123",
		Name:     "Duplicate",
	})
	if err != service.ErrEmailAlreadyExists {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestRegister_GetByEmailDBError(t *testing.T) {
	userRepo := newMockUserRepo()
	userRepo.getErr = errors.New("db connection lost")
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	_, err := svc.Register(context.Background(), &model.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, userRepo.getErr) {
		t.Errorf("expected wrapped db error, got %v", err)
	}
}

func TestRegister_CreateError(t *testing.T) {
	userRepo := newMockUserRepo()
	userRepo.createErr = errors.New("insert failed")
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	_, err := svc.Register(context.Background(), &model.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRegister_CacheSetError(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	cacheRepo.setErr = errors.New("redis write failed")
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	_, err := svc.Register(context.Background(), &model.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test",
	})
	if err == nil {
		t.Fatal("expected error from cache set failure")
	}
}

// --- Login Tests ---

func TestLogin_Success(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	seedUser(userRepo, "login@example.com", "correct-password", "Login User")

	resp, err := svc.Login(context.Background(), &model.LoginRequest{
		Email:    "login@example.com",
		Password: "correct-password",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	seedUser(userRepo, "login@example.com", "correct-password", "Login User")

	_, err := svc.Login(context.Background(), &model.LoginRequest{
		Email:    "login@example.com",
		Password: "wrong-password",
	})
	if err != service.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_NonExistentUser(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	_, err := svc.Login(context.Background(), &model.LoginRequest{
		Email:    "nobody@example.com",
		Password: "whatever",
	})
	if err != service.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_DBError(t *testing.T) {
	userRepo := newMockUserRepo()
	userRepo.getErr = errors.New("db timeout")
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	_, err := svc.Login(context.Background(), &model.LoginRequest{
		Email:    "test@example.com",
		Password: "password",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Refresh Tests ---

func TestRefresh_Success(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	resp, err := svc.Register(context.Background(), &model.RegisterRequest{
		Email:    "refresh@example.com",
		Password: "password123",
		Name:     "Refresh User",
	})
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	newResp, err := svc.Refresh(context.Background(), resp.RefreshToken)
	if err != nil {
		t.Fatalf("refresh error: %v", err)
	}
	if newResp.AccessToken == "" {
		t.Error("expected non-empty new access token")
	}
	if newResp.AccessToken == resp.AccessToken {
		t.Error("expected new access token to differ from old")
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	_, err := svc.Refresh(context.Background(), "invalid-token")
	if err != service.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestRefresh_TokenNotInCache(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	// Register then clear the cache to simulate missing refresh token
	resp, err := svc.Register(context.Background(), &model.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test",
	})
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	// Clear cache
	for k := range cacheRepo.refreshTokens {
		delete(cacheRepo.refreshTokens, k)
	}

	_, err = svc.Refresh(context.Background(), resp.RefreshToken)
	if err != service.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestRefresh_CacheUnavailable(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	resp, err := svc.Register(context.Background(), &model.RegisterRequest{
		Email:    "cache@example.com",
		Password: "password123",
		Name:     "Cache User",
	})
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	cacheRepo.getErr = repository.ErrCacheUnavailable

	_, err = svc.Refresh(context.Background(), resp.RefreshToken)
	if !errors.Is(err, repository.ErrCacheUnavailable) {
		t.Errorf("expected ErrCacheUnavailable, got %v", err)
	}
}

// --- Logout Tests ---

func TestLogout_Success(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	resp, err := svc.Register(context.Background(), &model.RegisterRequest{
		Email:    "logout@example.com",
		Password: "password123",
		Name:     "Logout User",
	})
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	err = svc.Logout(context.Background(), resp.User.ID, "test-jti", 15*time.Minute)
	if err != nil {
		t.Fatalf("logout error: %v", err)
	}

	blacklisted, _ := cacheRepo.IsBlacklisted(context.Background(), "test-jti")
	if !blacklisted {
		t.Error("expected JTI to be blacklisted after logout")
	}
}

func TestLogout_BlacklistError(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	cacheRepo.blacklistErr = errors.New("redis error")
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	err := svc.Logout(context.Background(), "user-id", "jti", 15*time.Minute)
	if err == nil {
		t.Fatal("expected error from blacklist failure")
	}
}

func TestLogout_DeleteRefreshTokenError(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	cacheRepo.deleteErr = repository.ErrCacheUnavailable
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	err := svc.Logout(context.Background(), "user-id", "jti", 15*time.Minute)
	if !errors.Is(err, repository.ErrCacheUnavailable) {
		t.Fatalf("expected ErrCacheUnavailable, got %v", err)
	}
}

func TestGetCurrentUser_Success(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	user := seedUser(userRepo, "me@example.com", "password123", "Current User")

	dto, err := svc.GetCurrentUser(context.Background(), user.ID.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "Current User" {
		t.Fatalf("expected Current User, got %s", dto.Name)
	}
}

func TestGetCurrentUser_UserMissing(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	_, err := svc.GetCurrentUser(context.Background(), uuid.New().String())
	if err != service.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestChangePassword_BlacklistsCurrentTokenAndClearsRefreshToken(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	user := seedUser(userRepo, "change@example.com", "old-password", "Changer")
	cacheRepo.refreshTokens[user.ID.String()] = "refresh-token"

	err := svc.ChangePassword(context.Background(), user.ID.String(), "old-password", "new-password-123", "current-jti", 15*time.Minute)
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if !cacheRepo.blacklist["current-jti"] {
		t.Fatal("expected current access token to be blacklisted")
	}
	if _, ok := cacheRepo.refreshTokens[user.ID.String()]; ok {
		t.Fatal("expected refresh token to be deleted after password change")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("new-password-123")) != nil {
		t.Fatal("expected stored password hash to be updated")
	}
}

func TestChangePassword_BlacklistFailure(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	cacheRepo.blacklistErr = repository.ErrCacheUnavailable
	svc := service.NewAuthService(userRepo, cacheRepo, testConfig())

	user := seedUser(userRepo, "change@example.com", "old-password", "Changer")

	err := svc.ChangePassword(context.Background(), user.ID.String(), "old-password", "new-password-123", "current-jti", 15*time.Minute)
	if !errors.Is(err, repository.ErrCacheUnavailable) {
		t.Fatalf("expected ErrCacheUnavailable, got %v", err)
	}
}
