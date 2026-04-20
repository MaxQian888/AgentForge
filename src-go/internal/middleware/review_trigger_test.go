package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentforge/server/internal/middleware"
	"github.com/labstack/echo/v4"
)

func TestReviewTriggerAuthMiddleware_ServiceToken(t *testing.T) {
	e := echo.New()
	bl := newMockBlacklist()
	mw := middleware.ReviewTriggerAuthMiddleware(testSecret, bl, "agentforge-review-token")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/trigger", nil)
	req.Header.Set("Authorization", "Bearer agentforge-review-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusAccepted, "ok")
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestReviewTriggerAuthMiddleware_JWTFallback(t *testing.T) {
	e := echo.New()
	bl := newMockBlacklist()
	mw := middleware.ReviewTriggerAuthMiddleware(testSecret, bl, "agentforge-review-token")

	token := createToken(testSecret, "user-1", "review@example.com", "jti-review", time.Now().Add(15*time.Minute))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/trigger", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := mw(func(c echo.Context) error {
		claims, err := middleware.GetClaims(c)
		if err != nil {
			t.Fatalf("expected JWT claims, got error: %v", err)
		}
		if claims.Email != "review@example.com" {
			t.Fatalf("expected review@example.com, got %s", claims.Email)
		}
		return c.String(http.StatusAccepted, "ok")
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

type noopBlacklist struct{}

func (noopBlacklist) IsBlacklisted(context.Context, string) (bool, error) { return false, nil }
