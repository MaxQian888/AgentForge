package memory

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxTokens           = 4096
	defaultContextTokenBudget  = 1024
	defaultEntryImportance     = 0.5
	defaultIntegratedEntryKind = "instruction"
)

type ShortTermMemory struct {
	mu     sync.Mutex
	config Config
	scopes map[string]*scopeState
	now    func() time.Time
}

type scopeState struct {
	entries     []Entry
	totalTokens int
}

func NewShortTermMemory(config Config) *ShortTermMemory {
	if config.MaxTokens <= 0 {
		config.MaxTokens = defaultMaxTokens
	}
	if config.DefaultContextTokens <= 0 {
		config.DefaultContextTokens = min(config.MaxTokens, defaultContextTokenBudget)
	}
	if config.EvictionPolicy == "" {
		config.EvictionPolicy = EvictionPolicyLRU
	}
	if config.TokenEstimator == nil {
		config.TokenEstimator = defaultTokenEstimator
	}
	return &ShortTermMemory{
		config: config,
		scopes: make(map[string]*scopeState),
		now:    func() time.Time { return time.Now().UTC() },
	}
}

func (m *ShortTermMemory) Store(input StoreInput) (Entry, error) {
	scope := strings.TrimSpace(input.Scope)
	if scope == "" {
		return Entry{}, ErrScopeRequired
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return Entry{}, ErrContentRequired
	}

	tokenCount := m.config.TokenEstimator(content)
	if tokenCount <= 0 {
		tokenCount = 1
	}
	if tokenCount > m.config.MaxTokens {
		return Entry{}, fmt.Errorf("%w: %d > %d", ErrEntryExceedsBudget, tokenCount, m.config.MaxTokens)
	}

	now := m.now()
	entry := Entry{
		ID:             strings.TrimSpace(input.ID),
		Scope:          scope,
		Kind:           strings.TrimSpace(input.Kind),
		Content:        content,
		Importance:     normalizeImportance(input.Importance),
		TokenCount:     tokenCount,
		Metadata:       cloneStringMap(input.Metadata),
		CreatedAt:      now,
		LastAccessedAt: now,
	}
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("%s-%d", scope, now.UnixNano())
	}
	if entry.Kind == "" {
		entry.Kind = defaultIntegratedEntryKind
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	scopeState := m.ensureScope(scope)
	scopeState.entries = append(scopeState.entries, entry)
	scopeState.totalTokens += entry.TokenCount
	m.evictIfNeededLocked(scopeState)
	return cloneEntry(entry), nil
}

func (m *ShortTermMemory) Context(scope string, tokenBudget int) ([]Entry, error) {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return nil, ErrScopeRequired
	}
	if tokenBudget <= 0 {
		tokenBudget = m.config.DefaultContextTokens
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	scopeState, ok := m.scopes[scope]
	if !ok || len(scopeState.entries) == 0 {
		return []Entry{}, nil
	}

	result := make([]Entry, 0, len(scopeState.entries))
	usedTokens := 0
	now := m.now()
	for index := len(scopeState.entries) - 1; index >= 0; index-- {
		entry := scopeState.entries[index]
		if usedTokens+entry.TokenCount > tokenBudget {
			continue
		}
		scopeState.entries[index].LastAccessedAt = now
		entry.LastAccessedAt = now
		result = append(result, cloneEntry(entry))
		usedTokens += entry.TokenCount
	}

	reverseEntries(result)
	return result, nil
}

func (m *ShortTermMemory) Snapshot(scope string) (Snapshot, bool) {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return Snapshot{}, false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	scopeState, ok := m.scopes[scope]
	if !ok {
		return Snapshot{}, false
	}
	snapshot := Snapshot{
		Scope:       scope,
		Entries:     cloneEntries(scopeState.entries),
		TotalTokens: scopeState.totalTokens,
	}
	return snapshot, true
}

func (m *ShortTermMemory) ensureScope(scope string) *scopeState {
	state := m.scopes[scope]
	if state == nil {
		state = &scopeState{entries: make([]Entry, 0)}
		m.scopes[scope] = state
	}
	return state
}

func (m *ShortTermMemory) evictIfNeededLocked(scopeState *scopeState) {
	for scopeState.totalTokens > m.config.MaxTokens && len(scopeState.entries) > 0 {
		evictIndex := m.selectEvictionIndex(scopeState.entries)
		scopeState.totalTokens -= scopeState.entries[evictIndex].TokenCount
		scopeState.entries = append(scopeState.entries[:evictIndex], scopeState.entries[evictIndex+1:]...)
	}
}

func (m *ShortTermMemory) selectEvictionIndex(entries []Entry) int {
	switch m.config.EvictionPolicy {
	case EvictionPolicyImportance:
		return selectImportanceEviction(entries)
	default:
		return selectLRUEviction(entries)
	}
}

func selectLRUEviction(entries []Entry) int {
	evictIndex := 0
	for index := 1; index < len(entries); index++ {
		current := entries[index]
		candidate := entries[evictIndex]
		if current.LastAccessedAt.Before(candidate.LastAccessedAt) ||
			(current.LastAccessedAt.Equal(candidate.LastAccessedAt) && current.CreatedAt.Before(candidate.CreatedAt)) {
			evictIndex = index
		}
	}
	return evictIndex
}

func selectImportanceEviction(entries []Entry) int {
	evictIndex := 0
	for index := 1; index < len(entries); index++ {
		current := entries[index]
		candidate := entries[evictIndex]
		switch {
		case current.Importance < candidate.Importance:
			evictIndex = index
		case current.Importance == candidate.Importance && current.LastAccessedAt.Before(candidate.LastAccessedAt):
			evictIndex = index
		case current.Importance == candidate.Importance &&
			current.LastAccessedAt.Equal(candidate.LastAccessedAt) &&
			current.CreatedAt.Before(candidate.CreatedAt):
			evictIndex = index
		}
	}
	return evictIndex
}

func defaultTokenEstimator(text string) int {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return 0
	}
	return len(fields)
}

func normalizeImportance(value float64) float64 {
	if value <= 0 {
		return defaultEntryImportance
	}
	return value
}

func cloneEntries(entries []Entry) []Entry {
	cloned := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		cloned = append(cloned, cloneEntry(entry))
	}
	return cloned
}

func cloneEntry(entry Entry) Entry {
	cloned := entry
	cloned.Metadata = cloneStringMap(entry.Metadata)
	return cloned
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func reverseEntries(entries []Entry) {
	for left, right := 0, len(entries)-1; left < right; left, right = left+1, right-1 {
		entries[left], entries[right] = entries[right], entries[left]
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
