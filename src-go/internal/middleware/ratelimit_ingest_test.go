// src-go/internal/middleware/ratelimit_ingest_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	mw "github.com/agentforge/server/internal/middleware"
)

func TestIngestRateLimit_AllowsUnderLimit(t *testing.T) {
	e := echo.New()
	e.Use(mw.IngestRateLimit(5, 5)) // 5 req/s, burst 5
	e.POST("/x", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("req %d: want 204, got %d", i, rec.Code)
		}
	}
}

func TestIngestRateLimit_Rejects429OnBurst(t *testing.T) {
	e := echo.New()
	e.Use(mw.IngestRateLimit(1, 1))
	e.POST("/x", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = "10.0.0.2:1234"
		e.ServeHTTP(rec, req)
		if i == 0 && rec.Code != http.StatusNoContent {
			t.Fatalf("first req: want 204, got %d", rec.Code)
		}
		if i >= 1 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("req %d: want 429, got %d", i, rec.Code)
		}
	}
}

func TestIngestRateLimit_PerSourceIsolation(t *testing.T) {
	e := echo.New()
	e.Use(mw.IngestRateLimit(1, 1))
	e.POST("/x", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	recA := httptest.NewRecorder()
	reqA := httptest.NewRequest(http.MethodPost, "/x", nil)
	reqA.RemoteAddr = "10.0.0.3:1234"
	e.ServeHTTP(recA, reqA)

	recB := httptest.NewRecorder()
	reqB := httptest.NewRequest(http.MethodPost, "/x", nil)
	reqB.RemoteAddr = "10.0.0.4:1234"
	e.ServeHTTP(recB, reqB)

	if recA.Code != http.StatusNoContent || recB.Code != http.StatusNoContent {
		t.Fatalf("each IP gets its own bucket: A=%d B=%d", recA.Code, recB.Code)
	}
}
