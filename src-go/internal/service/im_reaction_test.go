package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeReactionStore struct {
	mu        sync.Mutex
	events    []model.IMReactionEvent
	bindings  []model.IMReactionShortcutBinding
	shortcut  *model.IMReactionShortcutBinding
	resolveBy func(platform, messageID, emojiCode string) *model.IMReactionShortcutBinding
}

func (f *fakeReactionStore) RecordReaction(_ context.Context, event *model.IMReactionEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, *event)
	return nil
}

func (f *fakeReactionStore) BindShortcut(_ context.Context, b *model.IMReactionShortcutBinding) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindings = append(f.bindings, *b)
	f.shortcut = b
	return nil
}

func (f *fakeReactionStore) ResolveShortcut(_ context.Context, platform, messageID, emojiCode string) (*model.IMReactionShortcutBinding, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.resolveBy != nil {
		return f.resolveBy(platform, messageID, emojiCode), nil
	}
	return f.shortcut, nil
}

func TestIMService_HandleReaction_RecordsEventAndRoutesShortcut(t *testing.T) {
	store := &fakeReactionStore{}
	svc := NewIMService("http://notify", "slack")
	svc.SetReactionStore(store)
	svc.SetReviewTrigger(&nopReviewTrigger{})
	store.shortcut = &model.IMReactionShortcutBinding{
		ReviewID:  "rev-7",
		Outcome:   "approve",
		EmojiCode: "thumbs_up",
	}

	req := &model.IMReactionRequest{
		Platform:  "slack",
		ChatID:    "C1",
		MessageID: "M1",
		UserID:    "U1",
		EmojiCode: "thumbs_up",
		RawEmoji:  "+1",
		ReactedAt: time.Unix(1_700_000_000, 0).UTC(),
	}
	if err := svc.HandleReaction(context.Background(), req); err != nil {
		t.Fatalf("HandleReaction: %v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.events) != 1 {
		t.Fatalf("events = %d, want 1", len(store.events))
	}
	if store.events[0].Emoji != "thumbs_up" {
		t.Fatalf("emoji = %q", store.events[0].Emoji)
	}
}

func TestIMService_BindReactionShortcut_PersistsBinding(t *testing.T) {
	store := &fakeReactionStore{}
	svc := NewIMService("http://notify", "slack")
	svc.SetReactionStore(store)

	err := svc.BindReactionShortcut(context.Background(), &model.IMReactionShortcutBinding{
		ReviewID:  "rev-9",
		Outcome:   "approve",
		EmojiCode: "thumbs_up",
		Platform:  "slack",
	})
	if err != nil {
		t.Fatalf("BindReactionShortcut: %v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(store.bindings))
	}
	if store.bindings[0].ReviewID != "rev-9" {
		t.Fatalf("review_id = %q", store.bindings[0].ReviewID)
	}
}

type nopReviewTrigger struct{}

func (*nopReviewTrigger) Trigger(_ context.Context, _ *model.TriggerReviewRequest) (*model.Review, error) {
	return &model.Review{}, nil
}
