package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/react-go-quick-starter/server/internal/model"
)

// imReactionStoreAdapter bridges service.IMReactionStore to the repository
// seams. Shortcut bindings are held in-memory pending a durable table.
type imReactionStoreAdapter struct {
	repo      imReactionEventRepo
	mu        sync.RWMutex
	shortcuts map[string]*model.IMReactionShortcutBinding
}

// imReactionEventRepo is the narrow slice of IMReactionEventRepository that
// the reaction store needs. Defined here to avoid pulling a concrete package
// dependency at the service layer.
type imReactionEventRepo interface {
	Record(ctx context.Context, event *model.IMReactionEvent) error
}

// NewIMReactionStoreAdapter wires the reaction event repository into the
// service-level IMReactionStore seam. Shortcut bindings are in-memory for
// now.
func NewIMReactionStoreAdapter(repo imReactionEventRepo) IMReactionStore {
	return &imReactionStoreAdapter{
		repo:      repo,
		shortcuts: make(map[string]*model.IMReactionShortcutBinding),
	}
}

func (a *imReactionStoreAdapter) RecordReaction(ctx context.Context, event *model.IMReactionEvent) error {
	if a == nil || a.repo == nil {
		return fmt.Errorf("reaction repo unavailable")
	}
	return a.repo.Record(ctx, event)
}

func (a *imReactionStoreAdapter) BindShortcut(_ context.Context, binding *model.IMReactionShortcutBinding) error {
	if a == nil || binding == nil {
		return fmt.Errorf("binding is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shortcuts[shortcutKey(binding.Platform, shortcutMessageID(binding), binding.EmojiCode)] = binding
	return nil
}

func (a *imReactionStoreAdapter) ResolveShortcut(_ context.Context, platform, messageID, emojiCode string) (*model.IMReactionShortcutBinding, error) {
	if a == nil {
		return nil, nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if binding, ok := a.shortcuts[shortcutKey(platform, messageID, emojiCode)]; ok {
		return binding, nil
	}
	return nil, nil
}

func shortcutKey(platform, messageID, emojiCode string) string {
	return strings.ToLower(strings.TrimSpace(platform)) + "|" + strings.TrimSpace(messageID) + "|" + strings.TrimSpace(emojiCode)
}

func shortcutMessageID(b *model.IMReactionShortcutBinding) string {
	if b == nil || b.ReplyTarget == nil {
		return ""
	}
	if id := strings.TrimSpace(b.ReplyTarget.MessageID); id != "" {
		return id
	}
	return strings.TrimSpace(b.ReplyTarget.ThreadID)
}
