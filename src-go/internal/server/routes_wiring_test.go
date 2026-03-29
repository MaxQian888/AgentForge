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
		`protected.GET("/bridge/runtimes", bridgeRuntimeCatalogH.Get)`,
		`protected.POST("/ai/generate", bridgeAIH.Generate)`,
		`protected.POST("/ai/classify-intent", bridgeAIH.ClassifyIntent)`,
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
