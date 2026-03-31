package memory

import (
	"errors"
	"time"
)

type EvictionPolicy string

const (
	EvictionPolicyLRU        EvictionPolicy = "lru"
	EvictionPolicyImportance EvictionPolicy = "importance"
)

var (
	ErrScopeRequired      = errors.New("memory scope is required")
	ErrContentRequired    = errors.New("memory content is required")
	ErrEntryExceedsBudget = errors.New("memory entry exceeds configured token budget")
)

type TokenEstimator func(text string) int

type Config struct {
	MaxTokens            int
	DefaultContextTokens int
	EvictionPolicy       EvictionPolicy
	TokenEstimator       TokenEstimator
}

type StoreInput struct {
	Scope      string
	ID         string
	Kind       string
	Content    string
	Importance float64
	Metadata   map[string]string
}

type Entry struct {
	ID             string            `json:"id"`
	Scope          string            `json:"scope"`
	Kind           string            `json:"kind,omitempty"`
	Content        string            `json:"content"`
	Importance     float64           `json:"importance"`
	TokenCount     int               `json:"tokenCount"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"createdAt"`
	LastAccessedAt time.Time         `json:"lastAccessedAt"`
}

type Snapshot struct {
	Scope       string  `json:"scope"`
	Entries     []Entry `json:"entries"`
	TotalTokens int     `json:"totalTokens"`
}
