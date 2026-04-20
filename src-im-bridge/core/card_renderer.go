package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// RenderedPayload is the platform-specific JSON or text the bridge will hand
// to its existing send path. ContentType is platform-meaningful:
//   - "interactive" for Feishu cards
//   - "blocks" for Slack
//   - "actioncard" for DingTalk
//   - "text" for the universal fallback when no renderer is registered
type RenderedPayload struct {
	ContentType string
	Body        string
}

// CardRenderer renders a ProviderNeutralCard into a platform payload.
type CardRenderer func(ProviderNeutralCard) (RenderedPayload, error)

var (
	rendererMu sync.RWMutex
	renderers  = map[string]CardRenderer{}
)

// RegisterCardRenderer is called from each platform package's init().
func RegisterCardRenderer(platform string, fn CardRenderer) {
	rendererMu.Lock()
	defer rendererMu.Unlock()
	renderers[strings.ToLower(strings.TrimSpace(platform))] = fn
}

// UnregisterCardRenderer is test-only — used to scope a stub registration to
// a single test without corrupting the global table.
func UnregisterCardRenderer(platform string) {
	rendererMu.Lock()
	defer rendererMu.Unlock()
	delete(renderers, strings.ToLower(strings.TrimSpace(platform)))
}

// DispatchCard validates the card, then renders it via the platform-specific
// renderer (when registered) or the text fallback. It is safe for concurrent
// use; the registry is RWMutex-guarded.
func DispatchCard(card ProviderNeutralCard, target *ReplyTarget) (RenderedPayload, error) {
	if err := card.Validate(); err != nil {
		return RenderedPayload{}, fmt.Errorf("dispatch card: %w", err)
	}
	platform := ""
	if target != nil {
		platform = strings.ToLower(strings.TrimSpace(target.Platform))
	}
	if platform == "" {
		return RenderedPayload{}, errors.New("dispatch card: reply target platform is required")
	}
	rendererMu.RLock()
	fn, ok := renderers[platform]
	rendererMu.RUnlock()
	if ok {
		return fn(card)
	}
	return RenderTextFallback(card), nil
}

// RenderTextFallback produces a plain-text rendering used when no platform
// renderer is registered. The format is stable so that downstream IM
// platforms which only support plain text still see a coherent message.
func RenderTextFallback(c ProviderNeutralCard) RenderedPayload {
	var b strings.Builder
	b.WriteString(c.Title)
	b.WriteByte('\n')
	if c.Status != "" || c.Summary != "" {
		if c.Status != "" {
			b.WriteString("[")
			b.WriteString(strings.ToUpper(string(c.Status)))
			b.WriteString("]")
			if c.Summary != "" {
				b.WriteByte(' ')
			}
		}
		if c.Summary != "" {
			b.WriteString(c.Summary)
		}
		b.WriteByte('\n')
	}
	for _, f := range c.Fields {
		b.WriteString("[")
		b.WriteString(f.Label)
		b.WriteString("] ")
		b.WriteString(f.Value)
		b.WriteByte('\n')
	}
	if c.Footer != "" {
		b.WriteString(c.Footer)
		b.WriteByte('\n')
	}
	for _, a := range c.Actions {
		switch a.Type {
		case CardActionTypeURL:
			b.WriteString("[")
			b.WriteString(a.Label)
			b.WriteString("] ")
			b.WriteString(a.URL)
			b.WriteByte('\n')
		case CardActionTypeCallback:
			b.WriteString("[")
			b.WriteString(a.Label)
			b.WriteString("] (interactive button — open AgentForge to respond)\n")
		}
	}
	return RenderedPayload{ContentType: "text", Body: b.String()}
}
