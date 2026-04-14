package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterRoutes_WiresReviewDocWriteback(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("routes.go"))
	if err != nil {
		t.Fatalf("ReadFile(routes.go) error = %v", err)
	}
	source := string(content)
	if !strings.Contains(source, "reviewSvc.WithDocWriteback(") {
		t.Fatal("expected RegisterRoutes to wire review doc writeback repositories into reviewSvc")
	}
}

func TestRegisterRoutes_WiresNotificationMarkAllRead(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("routes.go"))
	if err != nil {
		t.Fatalf("ReadFile(routes.go) error = %v", err)
	}
	source := string(content)
	if !strings.Contains(source, `protected.PUT("/notifications/read-all", notifH.MarkAllRead)`) {
		t.Fatal("expected RegisterRoutes to wire notification mark-all-read endpoint")
	}
}

func TestRegisterRoutes_WiresBridgeAPISurface(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("routes.go"))
	if err != nil {
		t.Fatalf("ReadFile(routes.go) error = %v", err)
	}
	source := string(content)
	expectedRoutes := []string{
		`protected.GET("/bridge/health", bridgeHealthH.Get)`,
		`protected.GET("/bridge/pool", bridgePoolH.Get)`,
		`protected.GET("/bridge/runtimes", bridgeRuntimeCatalogH.Get)`,
		`protected.GET("/bridge/tools", bridgeToolsH.List)`,
		`protected.POST("/bridge/tools/install", bridgeToolsH.Install)`,
		`protected.POST("/bridge/tools/uninstall", bridgeToolsH.Uninstall)`,
		`protected.POST("/bridge/tools/:id/restart", bridgeToolsH.Restart)`,
		`protected.POST("/ai/decompose", bridgeAIH.Decompose)`,
		`protected.POST("/ai/generate", bridgeAIH.Generate)`,
		`protected.POST("/ai/classify-intent", bridgeAIH.ClassifyIntent)`,
		`protected.POST("/bridge/shell", bridgeConvH.ExecuteShell)`,
		`protected.POST("/bridge/thinking", bridgeConvH.SetThinkingBudget)`,
		`protected.GET("/bridge/mcp-status/:task_id", bridgeConvH.GetMCPStatus)`,
		`protected.POST("/bridge/opencode/provider-auth/:provider/start", bridgeConvH.StartOpenCodeProviderAuth)`,
		`protected.POST("/bridge/opencode/provider-auth/:request_id/complete", bridgeConvH.CompleteOpenCodeProviderAuth)`,
	}
	for _, route := range expectedRoutes {
		if !strings.Contains(source, route) {
			t.Fatalf("expected RegisterRoutes to contain %s", route)
		}
	}
}

func TestRegisterRoutes_WiresDispatchPreflightRoute(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("routes.go"))
	if err != nil {
		t.Fatalf("ReadFile(routes.go) error = %v", err)
	}
	source := string(content)
	if !strings.Contains(source, `projectGroup.GET("/dispatch/preflight", dispatchPreflightH.Get)`) {
		t.Fatal("expected RegisterRoutes to wire project dispatch preflight endpoint")
	}
}

func TestRegisterRoutes_WiresDispatchObservabilityRoutes(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("routes.go"))
	if err != nil {
		t.Fatalf("ReadFile(routes.go) error = %v", err)
	}
	source := string(content)
	expected := []string{
		`projectGroup.GET("/dispatch/stats", dispatchStatsH.Get)`,
		`protected.GET("/tasks/:tid/dispatch/history", dispatchHistoryH.Get)`,
	}
	for _, route := range expected {
		if !strings.Contains(source, route) {
			t.Fatalf("expected RegisterRoutes to contain %s", route)
		}
	}
}

func TestRegisterRoutes_DoesNotWireInstructionIntrospectionRoutes(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("routes.go"))
	if err != nil {
		t.Fatalf("ReadFile(routes.go) error = %v", err)
	}
	source := string(content)
	unexpected := []string{
		`projectGroup.GET("/instructions/pending", instructionH.ListPending)`,
		`projectGroup.GET("/instructions/history", instructionH.ListHistory)`,
		`projectGroup.GET("/instructions/metrics", instructionH.ListMetrics)`,
	}
	for _, route := range unexpected {
		if strings.Contains(source, route) {
			t.Fatalf("expected RegisterRoutes not to contain %s", route)
		}
	}
}

func TestRegisterRoutes_WiresMemoryExplorerRoutes(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("routes.go"))
	if err != nil {
		t.Fatalf("ReadFile(routes.go) error = %v", err)
	}
	source := string(content)
	expectedRoutes := []string{
		`projectGroup.POST("/memory", memoryH.Store)`,
		`projectGroup.GET("/memory", memoryH.Search)`,
		`projectGroup.GET("/memory/stats", memoryH.Stats)`,
		`projectGroup.GET("/memory/export", memoryH.Export)`,
		`projectGroup.POST("/memory/bulk-delete", memoryH.BulkDelete)`,
		`projectGroup.POST("/memory/cleanup", memoryH.Cleanup)`,
		`projectGroup.GET("/memory/:mid", memoryH.Get)`,
		`projectGroup.DELETE("/memory/:mid", memoryH.Delete)`,
	}
	for _, route := range expectedRoutes {
		if !strings.Contains(source, route) {
			t.Fatalf("expected RegisterRoutes to contain %s", route)
		}
	}
}
