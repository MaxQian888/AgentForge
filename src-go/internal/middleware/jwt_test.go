package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
)

const testSecret = "test-secret-at-least-32-characters-long"

// --- Mock blacklist ---

type mockBlacklist struct {
	blacklisted map[string]bool
}

func newMockBlacklist() *mockBlacklist {
	return &mockBlacklist{blacklisted: make(map[string]bool)}
}

func (m *mockBlacklist) IsBlacklisted(_ context.Context, jti string) (bool, error) {
	return m.blacklisted[jti], nil
}

type errorBlacklist struct{}

func (m *errorBlacklist) IsBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, repository.ErrCacheUnavailable
}

// --- Helpers ---

func createToken(secret, userID, email, jti string, expiresAt time.Time) string {
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

func TestJWTMiddleware_ValidToken(t *testing.T) {
	e := echo.New()
	bl := newMockBlacklist()
	mw := middleware.JWTMiddleware(testSecret, bl)

	token := createToken(testSecret, uuid.New().String(), "test@example.com", "jti-1", time.Now().Add(15*time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := mw(func(c echo.Context) error {
		claims, err := middleware.GetClaims(c)
		if err != nil {
			t.Fatalf("GetClaims error: %v", err)
		}
		if claims.Email != "test@example.com" {
			t.Errorf("expected email test@example.com, got %s", claims.Email)
		}
		return c.String(http.StatusOK, "ok")
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestJWTMiddleware_MissingHeader(t *testing.T) {
	e := echo.New()
	bl := newMockBlacklist()
	mw := middleware.JWTMiddleware(testSecret, bl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestJWTMiddleware_ExpiredToken(t *testing.T) {
	e := echo.New()
	bl := newMockBlacklist()
	mw := middleware.JWTMiddleware(testSecret, bl)

	token := createToken(testSecret, uuid.New().String(), "test@example.com", "jti-2", time.Now().Add(-1*time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestJWTMiddleware_BlacklistedToken(t *testing.T) {
	e := echo.New()
	bl := newMockBlacklist()
	bl.blacklisted["revoked-jti"] = true
	mw := middleware.JWTMiddleware(testSecret, bl)

	token := createToken(testSecret, uuid.New().String(), "test@example.com", "revoked-jti", time.Now().Add(15*time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestJWTMiddleware_InvalidSignature(t *testing.T) {
	e := echo.New()
	bl := newMockBlacklist()
	mw := middleware.JWTMiddleware(testSecret, bl)

	token := createToken("wrong-secret-that-is-long-enough", uuid.New().String(), "test@example.com", "jti-3", time.Now().Add(15*time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestJWTMiddleware_BlacklistError(t *testing.T) {
	e := echo.New()
	bl := &errorBlacklist{}
	mw := middleware.JWTMiddleware(testSecret, bl)

	token := createToken(testSecret, uuid.New().String(), "test@example.com", "jti-err", time.Now().Add(15*time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestJWTMiddleware_NotBearerPrefix(t *testing.T) {
	e := echo.New()
	bl := newMockBlacklist()
	mw := middleware.JWTMiddleware(testSecret, bl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "NotBearer token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestGetClaims_NoClaims(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	claims, err := middleware.GetClaims(c)
	if err == nil {
		t.Error("expected error when no claims in context")
	}
	if claims != nil {
		t.Error("expected nil claims")
	}
}

func TestGetClaims_WrongType(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.JWTContextKey, "not-claims-type")

	claims, err := middleware.GetClaims(c)
	if err == nil {
		t.Error("expected error when claims have wrong type")
	}
	if claims != nil {
		t.Error("expected nil claims")
	}
}
