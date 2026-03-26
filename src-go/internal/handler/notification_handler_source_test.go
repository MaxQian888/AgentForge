package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotificationHandler_ExposesMarkAllReadHandler(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("notification_handler.go"))
	if err != nil {
		t.Fatalf("ReadFile(notification_handler.go) error = %v", err)
	}

	source := string(content)
	if !strings.Contains(source, "func (h *NotificationHandler) MarkAllRead") {
		t.Fatal("expected NotificationHandler to expose MarkAllRead")
	}
}
