package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	appI18n "github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
)

func TestLocaleMiddleware_DefaultsToEnglishWhenHeaderMissing(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	called := false

	err := appMiddleware.Locale()(func(c echo.Context) error {
		called = true

		if got := appI18n.Localize(appMiddleware.GetLocalizer(c), appI18n.MsgInternalError); got != "internal server error" {
			t.Fatalf("localized message = %q, want %q", got, "internal server error")
		}

		return nil
	})(c)

	if err != nil {
		t.Fatalf("middleware returned error: %v", err)
	}

	if !called {
		t.Fatal("expected wrapped handler to be called")
	}
}
