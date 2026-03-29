package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/config"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
	"golang.org/x/crypto/bcrypt"
)

// --- Test validator ---

type testValidator struct {
	validator *validator.Validate
}

func (tv *testValidator) Validate(i interface{}) error {
	return tv.validator.Struct(i)
}

// --- Mock repositories ---

type mockUserRepo struct {
	users map[string]*model.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[string]*model.User)}
}

func (m *mockUserRepo) Create(_ context.Context, user *model.User) error {
	m.users[user.Email] = user
	return nil
}

func (m *mockUserRepo) GetByEmail(_ context.Context, email string) (*model.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByID(_ context.Context, id uuid.UUID) (*model.User, error) {
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

const testSecret = "test-secret-at-least-32-characters-long"

func testConfig() *config.Config {
	return &config.Config{
		JWTSecret:     testSecret,
		JWTAccessTTL:  15 * time.Minute,
		JWTRefreshTTL: 7 * 24 * time.Hour,
		Env:           "test",
	}
}

func setupEcho() *echo.Echo {
	e := echo.New()
	e.Validator = &testValidator{validator: validator.New()}
	return e
}

func setupAuthHandlerWithMocks() (*handler.AuthHandler, *mockUserRepo, *mockCacheRepo) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	cfg := testConfig()
	authSvc := service.NewAuthService(userRepo, cacheRepo, cfg)
	h := handler.NewAuthHandler(authSvc, cfg.JWTAccessTTL)
	return h, userRepo, cacheRepo
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

func createJWTToken(secret, userID, email, jti string, expiresAt time.Time) string {
	claims := &service.Claims{
		UserID: userID,
		Email:  email,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	return token
}

func setJWTClaims(c echo.Context, userID, email, jti string, expiresAt time.Time) {
	claims := &service.Claims{
		UserID: userID,
		Email:  email,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}
	c.Set(middleware.JWTContextKey, claims)
}

// --- Register Tests ---

func TestRegisterHandler_InvalidBody(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Register(c)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRegisterHandler_ValidationError(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Register(c)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestRegisterHandler_Success(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	body := `{"email":"new@example.com","password":"password123","name":"New User"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Register(c)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}

	var resp model.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if resp.User.Email != "new@example.com" {
		t.Errorf("expected email new@example.com, got %s", resp.User.Email)
	}
}

func TestRegisterHandler_DuplicateEmail(t *testing.T) {
	e := setupEcho()
	h, userRepo, _ := setupAuthHandlerWithMocks()
	seedUser(userRepo, "dup@example.com", "password", "Existing")

	body := `{"email":"dup@example.com","password":"password123","name":"Dup"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Register(c)
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

// --- Login Tests ---

func TestLoginHandler_InvalidBody(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Login(c)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestLoginHandler_ValidationError(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Login(c)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestLoginHandler_Success(t *testing.T) {
	e := setupEcho()
	h, userRepo, _ := setupAuthHandlerWithMocks()
	seedUser(userRepo, "login@example.com", "correct-pass", "Login User")

	body := `{"email":"login@example.com","password":"correct-pass"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Login(c)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	e := setupEcho()
	h, userRepo, _ := setupAuthHandlerWithMocks()
	seedUser(userRepo, "login@example.com", "correct-pass", "Login User")

	body := `{"email":"login@example.com","password":"wrong-pass"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Login(c)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestLoginHandler_UserNotFound(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	body := `{"email":"nobody@example.com","password":"password"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Login(c)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// --- Refresh Tests ---

func TestRefreshHandler_InvalidBody(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Refresh(c)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRefreshHandler_ValidationError(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"refreshToken":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Refresh(c)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestRefreshHandler_InvalidToken(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	body := `{"refreshToken":"invalid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Refresh(c)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRefreshHandler_Success(t *testing.T) {
	e := setupEcho()
	h, userRepo, _ := setupAuthHandlerWithMocks()

	// Register a user first to get valid tokens
	registerBody := `{"email":"refresh@example.com","password":"password123","name":"Refresh User"}`
	req1 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(registerBody))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)
	_ = h.Register(c1)

	var registerResp model.AuthResponse
	_ = json.Unmarshal(rec1.Body.Bytes(), &registerResp)

	// We need to make sure the user is findable by ID for refresh
	for _, u := range userRepo.users {
		if u.Email == "refresh@example.com" {
			break
		}
	}

	// Now refresh
	refreshBody := `{"refreshToken":"` + registerResp.RefreshToken + `"}`
	req2 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(refreshBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	_ = h.Refresh(c2)

	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rec2.Code, rec2.Body.String())
	}
}

func TestRefreshHandler_CacheUnavailable(t *testing.T) {
	e := setupEcho()
	h, userRepo, cacheRepo := setupAuthHandlerWithMocks()

	registerBody := `{"email":"refresh-cache@example.com","password":"password123","name":"Refresh User"}`
	req1 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(registerBody))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)
	_ = h.Register(c1)

	var registerResp model.AuthResponse
	_ = json.Unmarshal(rec1.Body.Bytes(), &registerResp)

	cacheRepo.getErr = repository.ErrCacheUnavailable
	for _, u := range userRepo.users {
		if u.Email == "refresh-cache@example.com" {
			break
		}
	}

	refreshBody := `{"refreshToken":"` + registerResp.RefreshToken + `"}`
	req2 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(refreshBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	_ = h.Refresh(c2)

	if rec2.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec2.Code)
	}
}

// --- Logout Tests ---

func TestLogoutHandler_NoClaims(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Logout(c)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestLogoutHandler_Success(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	setJWTClaims(c, uuid.New().String(), "test@example.com", "jti-logout", time.Now().Add(15*time.Minute))

	_ = h.Logout(c)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestLogoutHandler_ExpiredToken(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Token already expired — remaining TTL should be clamped to 0
	setJWTClaims(c, uuid.New().String(), "test@example.com", "jti-expired", time.Now().Add(-1*time.Hour))

	_ = h.Logout(c)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- GetMe Tests ---

func TestLogoutHandler_ServiceError(t *testing.T) {
	e := setupEcho()
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	cacheRepo.blacklistErr = repository.ErrCacheUnavailable
	cfg := testConfig()
	authSvc := service.NewAuthService(userRepo, cacheRepo, cfg)
	h := handler.NewAuthHandler(authSvc, cfg.JWTAccessTTL)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	setJWTClaims(c, uuid.New().String(), "test@example.com", "jti-fail", time.Now().Add(15*time.Minute))

	_ = h.Logout(c)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestGetMeHandler_NoClaims(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.GetMe(c)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestGetMeHandler_Success(t *testing.T) {
	e := setupEcho()
	h, userRepo, _ := setupAuthHandlerWithMocks()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	user := seedUser(userRepo, "me@example.com", "password123", "Current User")
	setJWTClaims(c, user.ID.String(), "stale@example.com", "jti-me", time.Now().Add(15*time.Minute))

	_ = h.GetMe(c)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var dto model.UserDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &dto)
	if dto.ID != user.ID.String() {
		t.Errorf("expected ID %s, got %s", user.ID.String(), dto.ID)
	}
	if dto.Email != "me@example.com" {
		t.Errorf("expected email me@example.com, got %s", dto.Email)
	}
	if dto.Name != "Current User" {
		t.Errorf("expected Current User, got %s", dto.Name)
	}
}

func TestGetMeHandler_UserMissing(t *testing.T) {
	e := setupEcho()
	h, _, _ := setupAuthHandlerWithMocks()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	setJWTClaims(c, uuid.New().String(), "missing@example.com", "jti-missing", time.Now().Add(15*time.Minute))

	_ = h.GetMe(c)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
