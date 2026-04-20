package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentforge/server/internal/handler"
	"github.com/agentforge/server/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func setupNotificationDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	if err := db.Exec(`
		CREATE TABLE notifications (
			id TEXT PRIMARY KEY,
			target_id TEXT NOT NULL,
			type TEXT,
			title TEXT,
			body TEXT,
			data TEXT,
			is_read BOOLEAN NOT NULL DEFAULT FALSE,
			channel TEXT,
			sent BOOLEAN NOT NULL DEFAULT FALSE,
			created_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create notifications table error = %v", err)
	}

	return db
}

func insertNotificationRow(t *testing.T, db *gorm.DB, id, targetID uuid.UUID, isRead bool) {
	t.Helper()

	if err := db.Exec(
		`INSERT INTO notifications (id, target_id, type, title, body, data, is_read, channel, sent, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(),
		targetID.String(),
		"task_progress_warning",
		"Task warning",
		"Task warning body",
		"",
		isRead,
		"in_app",
		false,
		time.Now().UTC(),
	).Error; err != nil {
		t.Fatalf("insert notification error = %v", err)
	}
}

func TestNotificationHandler_MarkAllReadRequiresClaims(t *testing.T) {
	e := setupEcho()
	h := handler.NewNotificationHandler(repository.NewNotificationRepository(setupNotificationDB(t)))

	req := httptest.NewRequest(http.MethodPut, "/notifications/read-all", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.MarkAllRead(c); err != nil {
		t.Fatalf("MarkAllRead() error = %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNotificationHandler_MarkAllReadMarksCurrentUsersUnreadNotifications(t *testing.T) {
	db := setupNotificationDB(t)
	repo := repository.NewNotificationRepository(db)
	h := handler.NewNotificationHandler(repo)
	e := setupEcho()

	targetID := uuid.New()
	otherTargetID := uuid.New()
	insertNotificationRow(t, db, uuid.New(), targetID, false)
	insertNotificationRow(t, db, uuid.New(), targetID, true)
	insertNotificationRow(t, db, uuid.New(), otherTargetID, false)

	req := httptest.NewRequest(http.MethodPut, "/notifications/read-all", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setJWTClaims(c, targetID.String(), "notify@example.com", "jti-notify", time.Now().Add(15*time.Minute))

	if err := h.MarkAllRead(c); err != nil {
		t.Fatalf("MarkAllRead() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var remainingUnreadForTarget int64
	if err := db.Table("notifications").
		Where("target_id = ? AND is_read = ?", targetID.String(), false).
		Count(&remainingUnreadForTarget).Error; err != nil {
		t.Fatalf("count target unread error = %v", err)
	}
	if remainingUnreadForTarget != 0 {
		t.Fatalf("remainingUnreadForTarget = %d, want 0", remainingUnreadForTarget)
	}

	var remainingUnreadForOther int64
	if err := db.Table("notifications").
		Where("target_id = ? AND is_read = ?", otherTargetID.String(), false).
		Count(&remainingUnreadForOther).Error; err != nil {
		t.Fatalf("count other unread error = %v", err)
	}
	if remainingUnreadForOther != 1 {
		t.Fatalf("remainingUnreadForOther = %d, want 1", remainingUnreadForOther)
	}
}
