package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// RateDimension identifies one axis against which rate limit policies can
// bucket events. Policies are built by combining dimensions; each dimension
// that the policy names MUST have a non-empty value in the Scope for the
// policy to apply, otherwise the policy is skipped for that request.
type RateDimension string

const (
	DimTenant      RateDimension = "tenant"
	DimChat        RateDimension = "chat"
	DimUser        RateDimension = "user"
	DimCommand     RateDimension = "command"      // e.g. "/task"
	DimActionClass RateDimension = "action_class" // "read"|"write"|"destructive"
	DimBridge      RateDimension = "bridge"
)

// RateActionClass bins commands by blast radius so destructive operations
// can carry their own quota independent of reads/writes.
type RateActionClass string

const (
	ActionClassRead        RateActionClass = "read"
	ActionClassWrite       RateActionClass = "write"
	ActionClassDestructive RateActionClass = "destructive"
)

// RateLimitPolicy describes one rate-limit rule. A RateLimiter evaluates
// all applicable policies for each event; the first policy that rejects
// stops evaluation.
type RateLimitPolicy struct {
	ID          string          `json:"id"`
	Dimensions  []RateDimension `json:"dimensions"`
	Rate        int             `json:"rate"`
	Window      time.Duration   `json:"window"`
	ActionClass RateActionClass `json:"actionClass,omitempty"` // optional filter; empty matches all
	Description string          `json:"description,omitempty"`
}

// Scope is the per-request context presented to the limiter. Fields left
// empty disable policies whose Dimensions require them.
type Scope struct {
	Tenant      string
	Chat        string
	User        string
	Command     string
	ActionClass RateActionClass
	Bridge      string
}

// RateDecision is the outcome of a rate-limit check.
type RateDecision struct {
	Allowed       bool
	Policy        string
	RetryAfterSec int
}

// DefaultPolicies preserves the legacy 20/min per (chat,user) envelope and
// adds write/destructive class limiters matching the design.
func DefaultPolicies() []RateLimitPolicy {
	return []RateLimitPolicy{
		{
			ID:          "session-default",
			Dimensions:  []RateDimension{DimChat, DimUser},
			Rate:        20,
			Window:      time.Minute,
			Description: "per-session legacy limit (preserves historical 20/min)",
		},
		{
			ID:          "write-action",
			Dimensions:  []RateDimension{DimUser, DimActionClass},
			Rate:        10,
			Window:      time.Minute,
			ActionClass: ActionClassWrite,
			Description: "per-user write commands",
		},
		{
			ID:          "destructive-action",
			Dimensions:  []RateDimension{DimUser, DimActionClass},
			Rate:        3,
			Window:      time.Minute,
			ActionClass: ActionClassDestructive,
			Description: "per-user destructive commands",
		},
		{
			ID:          "per-chat",
			Dimensions:  []RateDimension{DimChat},
			Rate:        60,
			Window:      time.Minute,
			Description: "aggregate chat-wide gate",
		},
	}
}

// ParsePolicies decodes a JSON array of policies; used by operators to
// override the default set via IM_RATE_POLICY.
func ParsePolicies(raw string) ([]RateLimitPolicy, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	type wire struct {
		ID          string          `json:"id"`
		Dimensions  []RateDimension `json:"dimensions"`
		Rate        int             `json:"rate"`
		Window      string          `json:"window"`
		ActionClass RateActionClass `json:"actionClass,omitempty"`
		Description string          `json:"description,omitempty"`
	}
	var items []wire
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("parse IM_RATE_POLICY: %w", err)
	}
	out := make([]RateLimitPolicy, 0, len(items))
	for _, item := range items {
		if item.ID == "" {
			return nil, errors.New("policy id is required")
		}
		if item.Rate <= 0 {
			return nil, fmt.Errorf("policy %s: rate must be positive", item.ID)
		}
		window, err := time.ParseDuration(item.Window)
		if err != nil || window <= 0 {
			return nil, fmt.Errorf("policy %s: invalid window %q", item.ID, item.Window)
		}
		out = append(out, RateLimitPolicy{
			ID:          item.ID,
			Dimensions:  item.Dimensions,
			Rate:        item.Rate,
			Window:      window,
			ActionClass: item.ActionClass,
			Description: item.Description,
		})
	}
	return out, nil
}

// compositeKey assembles a stable hash from dimension values. Returns ""
// if the policy cannot be applied because a required dimension is empty.
func compositeKey(policy RateLimitPolicy, scope Scope) string {
	if policy.ActionClass != "" && scope.ActionClass != policy.ActionClass {
		return ""
	}
	dims := append([]RateDimension(nil), policy.Dimensions...)
	sort.SliceStable(dims, func(i, j int) bool { return dims[i] < dims[j] })
	parts := make([]string, 0, len(dims)+1)
	parts = append(parts, string(policy.ID))
	for _, d := range dims {
		var v string
		switch d {
		case DimTenant:
			v = scope.Tenant
		case DimChat:
			v = scope.Chat
		case DimUser:
			v = scope.User
		case DimCommand:
			v = scope.Command
		case DimActionClass:
			v = string(scope.ActionClass)
		case DimBridge:
			v = scope.Bridge
		default:
			v = ""
		}
		if v == "" {
			return ""
		}
		parts = append(parts, string(d)+"="+v)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:16])
}
