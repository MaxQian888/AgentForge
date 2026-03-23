package server

import (
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/react-go-quick-starter/server/internal/config"
	"github.com/react-go-quick-starter/server/internal/handler"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/version"
)

func RegisterRoutes(
	e *echo.Echo,
	cfg *config.Config,
	authSvc *service.AuthService,
	cache *repository.CacheRepository,
	projectRepo *repository.ProjectRepository,
	memberRepo *repository.MemberRepository,
	sprintRepo *repository.SprintRepository,
	taskRepo *repository.TaskRepository,
	agentRunRepo *repository.AgentRunRepository,
	notifRepo *repository.NotificationRepository,
) {
	jwtMw := appMiddleware.JWTMiddleware(cfg.JWTSecret, cache)

	// Health
	healthH := handler.NewHealthHandler(version.Version, version.Commit, version.BuildDate, cfg.Env)
	e.GET("/health", healthH.Health)

	// v1 group
	v1 := e.Group("/api/v1")
	v1.GET("/health", healthH.HealthV1)

	// Auth routes (public, rate-limited)
	authH := handler.NewAuthHandler(authSvc, cfg.JWTAccessTTL)
	authRateLimiter := echomiddleware.RateLimiter(echomiddleware.NewRateLimiterMemoryStore(20))
	auth := v1.Group("/auth")
	auth.POST("/register", authH.Register, authRateLimiter)
	auth.POST("/login", authH.Login, authRateLimiter)
	auth.POST("/refresh", authH.Refresh)
	auth.POST("/logout", authH.Logout, jwtMw)

	// User routes (protected)
	users := v1.Group("/users", jwtMw)
	users.GET("/me", authH.GetMe)

	// WebSocket
	wsH := handler.NewWSHandler(cfg.JWTSecret)
	e.GET("/ws", wsH.HandleWS)

	// --- New resource handlers ---
	projectH := handler.NewProjectHandler(projectRepo)
	memberH := handler.NewMemberHandler(memberRepo)
	sprintH := handler.NewSprintHandler(sprintRepo)
	taskH := handler.NewTaskHandler(taskRepo)
	agentH := handler.NewAgentHandler(agentRunRepo)
	notifH := handler.NewNotificationHandler(notifRepo)
	costH := handler.NewCostHandler(agentRunRepo)
	roleH := handler.NewRoleHandler(cfg.RolesDir)

	// JWT protected routes
	protected := v1.Group("", jwtMw)

	// Projects
	protected.POST("/projects", projectH.Create)
	protected.GET("/projects", projectH.List)
	protected.GET("/projects/:id", projectH.Get)
	protected.PUT("/projects/:id", projectH.Update)

	// Project-scoped routes
	projectMw := appMiddleware.ProjectMiddleware(projectRepo)
	projectGroup := protected.Group("/projects/:pid", projectMw)
	projectGroup.POST("/members", memberH.Create)
	projectGroup.GET("/members", memberH.List)
	projectGroup.POST("/tasks", taskH.Create)
	projectGroup.GET("/tasks", taskH.List)
	projectGroup.POST("/sprints", sprintH.Create)
	projectGroup.GET("/sprints", sprintH.List)

	// Task operations (not project-scoped, task ID is unique)
	protected.GET("/tasks/:id", taskH.Get)
	protected.PUT("/tasks/:id", taskH.Update)
	protected.DELETE("/tasks/:id", taskH.Delete)
	protected.POST("/tasks/:id/transition", taskH.Transition)
	protected.POST("/tasks/:id/assign", taskH.Assign)

	// Member update
	protected.PUT("/members/:id", memberH.Update)

	// Agents
	protected.POST("/agents/spawn", agentH.Spawn)
	protected.GET("/agents", agentH.List)
	protected.GET("/agents/:id", agentH.Get)
	protected.POST("/agents/:id/pause", agentH.Pause)
	protected.POST("/agents/:id/resume", agentH.Resume)
	protected.POST("/agents/:id/kill", agentH.Kill)

	// Notifications
	protected.GET("/notifications", notifH.List)
	protected.PUT("/notifications/:id/read", notifH.MarkRead)

	// Cost
	protected.GET("/stats/cost", costH.GetStats)

	// Roles
	protected.GET("/roles", roleH.List)
	protected.GET("/roles/:id", roleH.Get)
	protected.POST("/roles", roleH.Create)
	protected.PUT("/roles/:id", roleH.Update)
}
