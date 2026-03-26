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
