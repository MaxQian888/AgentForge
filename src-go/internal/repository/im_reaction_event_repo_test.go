package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func openIMReactionEventRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	schema := []string{
		`CREATE TABLE im_reaction_events (
			id TEXT PRIMARY KEY,
			platform TEXT NOT NULL,
			chat_id TEXT NOT NULL,
			message_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			emoji TEXT NOT NULL,
			event_type TEXT NOT NULL,
			raw_payload TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,
	}

	for _, stmt := range schema {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create im_reaction_events schema: %v", err)
		}
	}

	return db
}

func TestIMReactionEventRepository_RecordRoundTrip(t *testing.T) {
	db := openIMReactionEventRepoTestDB(t)
	repo := NewIMReactionEventRepository(db)

	event := &model.IMReactionEvent{
		Platform:   "feishu",
		ChatID:     "chat-1",
		MessageID:  "om_abc",
		UserID:     "ou_reactor",
		Emoji:      "THUMBSUP",
		EventType:  model.IMReactionEventTypeCreated,
		RawPayload: []byte(`{"a":1}`),
	}
	if err := repo.Record(context.Background(), event); err != nil {
		t.Fatalf("Record error: %v", err)
	}
	if event.ID == uuid.Nil {
		t.Fatal("expected Record to set ID")
	}
	if event.CreatedAt.IsZero() {
		t.Fatal("expected Record to set CreatedAt")
	}

	stored, err := repo.ListByMessage(context.Background(), "om_abc", 10)
	if err != nil {
		t.Fatalf("ListByMessage error: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("len(stored)=%d", len(stored))
	}
	if stored[0].Emoji != "THUMBSUP" {
		t.Errorf("Emoji = %q", stored[0].Emoji)
	}
	var raw map[string]int
	if err := json.Unmarshal(stored[0].RawPayload, &raw); err != nil || raw["a"] != 1 {
		t.Errorf("raw payload round-trip failed: raw=%v err=%v", raw, err)
	}
	if time.Since(stored[0].CreatedAt) > time.Minute {
		t.Errorf("CreatedAt unexpected: %v", stored[0].CreatedAt)
	}
}

func TestIMReactionEventRepository_ValidatesEventType(t *testing.T) {
	db := openIMReactionEventRepoTestDB(t)
	repo := NewIMReactionEventRepository(db)

	err := repo.Record(context.Background(), &model.IMReactionEvent{
		Platform:  "feishu",
		MessageID: "om_abc",
		EventType: "invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid event_type")
	}
}
