package server

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/agentforge/marketplace/internal/config"
	"github.com/agentforge/marketplace/internal/handler"
	"github.com/agentforge/marketplace/internal/middleware"
)

// RegisterRoutes wires all HTTP routes onto the Echo instance.
func RegisterRoutes(
	e *echo.Echo,
	cfg *config.Config,
	itemH *handler.ItemHandler,
	versionH *handler.VersionHandler,
	reviewH *handler.ReviewHandler,
	adminH *handler.AdminHandler,
) {
	jwtMw := middleware.JWTMiddleware(cfg.JWTSecret)

	// Health
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Public read routes
	v1 := e.Group("/api/v1")
	v1.GET("/items", itemH.List)
	v1.GET("/items/featured", itemH.Featured)  // must be before /:id
	v1.GET("/items/search", itemH.Search)      // must be before /:id
	v1.GET("/items/:id", itemH.Get)
	v1.GET("/items/:id/versions", versionH.List)
	v1.GET("/items/:id/reviews", reviewH.List)

	// Authenticated write routes
	auth := v1.Group("", jwtMw)
	auth.POST("/items", itemH.Publish)
	auth.PATCH("/items/:id", itemH.UpdateMeta)
	auth.DELETE("/items/:id", itemH.Delete)
	auth.POST("/items/:id/versions", versionH.Upload)
	auth.POST("/items/:id/versions/:ver/yank", versionH.Yank)
	auth.GET("/items/:id/versions/:ver/download", versionH.Download)
	auth.POST("/items/:id/reviews", reviewH.Upsert)
	auth.DELETE("/items/:id/reviews/me", reviewH.DeleteMine)

	// Admin routes
	adminGroup := e.Group("/admin", jwtMw)
	adminGroup.POST("/items/:id/feature", adminH.Feature)
	adminGroup.POST("/items/:id/verify", adminH.Verify)
}
