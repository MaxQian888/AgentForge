package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLegacyLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewLegacyRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !rl.AllowLegacy("user:1") {
			t.Fatalf("request %d denied, expected allowed", i+1)
		}
	}
}

func TestLegacyLimiter_DeniesOverLimit(t *testing.T) {
	rl := NewLegacyRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		rl.AllowLegacy("user:1")
	}
	if rl.AllowLegacy("user:1") {
		t.Fatal("4th request allowed, expected denied")
	}
}

func TestLegacyLimiter_ResetsAfterWindow(t *testing.T) {
	fakeNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rl := NewLegacyRateLimiter(2, time.Minute)
	rl.setNow(func() time.Time { return fakeNow })

	rl.AllowLegacy("user:1")
	rl.AllowLegacy("user:1")
	if rl.AllowLegacy("user:1") {
		t.Fatal("3rd request allowed, expected denied")
	}

	fakeNow = fakeNow.Add(61 * time.Second)
	if !rl.AllowLegacy("user:1") {
		t.Fatal("request after window denied, expected allowed")
	}
}

func TestLegacyLimiter_IsolatesByKey(t *testing.T) {
	rl := NewLegacyRateLimiter(1, time.Minute)
	if !rl.AllowLegacy("user:1") {
		t.Fatal("user:1 denied")
	}
	if rl.AllowLegacy("user:1") {
		t.Fatal("user:1 second request allowed")
	}
	if !rl.AllowLegacy("user:2") {
		t.Fatal("user:2 denied")
	}
}

func TestPolicyAllow_RejectsAtThreshold(t *testing.T) {
	rl := NewRateLimiter([]RateLimitPolicy{{
		ID:         "test",
		Dimensions: []RateDimension{DimUser},
		Rate:       2,
		Window:     time.Minute,
	}})
	scope := Scope{User: "alice"}
	for i := 0; i < 2; i++ {
		d, err := rl.Allow(context.Background(), scope)
		if err != nil || !d.Allowed {
			t.Fatalf("call %d: allowed=%v err=%v", i+1, d.Allowed, err)
		}
	}
	d, _ := rl.Allow(context.Background(), scope)
	if d.Allowed {
		t.Fatalf("3rd call should be rejected")
	}
	if d.Policy != "test" {
		t.Fatalf("policy = %s, want test", d.Policy)
	}
	if d.RetryAfterSec != 60 {
		t.Fatalf("retry_after = %d, want 60", d.RetryAfterSec)
	}
}

func TestPolicyAllow_ActionClassFilterApplies(t *testing.T) {
	rl := NewRateLimiter([]RateLimitPolicy{{
		ID:          "writes-only",
		Dimensions:  []RateDimension{DimUser, DimActionClass},
		Rate:        1,
		Window:      time.Minute,
		ActionClass: ActionClassWrite,
	}})

	// reads should not count against this policy
	for i := 0; i < 5; i++ {
		d, _ := rl.Allow(context.Background(), Scope{User: "alice", ActionClass: ActionClassRead})
		if !d.Allowed {
			t.Fatalf("read %d wrongly rejected", i)
		}
	}
	// first write OK, second rejected
	d1, _ := rl.Allow(context.Background(), Scope{User: "alice", ActionClass: ActionClassWrite})
	if !d1.Allowed {
		t.Fatalf("first write rejected")
	}
	d2, _ := rl.Allow(context.Background(), Scope{User: "alice", ActionClass: ActionClassWrite})
	if d2.Allowed {
		t.Fatalf("second write admitted")
	}
}

func TestPolicyAllow_IsolatesByDimensionValue(t *testing.T) {
	rl := NewRateLimiter([]RateLimitPolicy{{
		ID:         "per-user",
		Dimensions: []RateDimension{DimUser},
		Rate:       1,
		Window:     time.Minute,
	}})
	d, _ := rl.Allow(context.Background(), Scope{User: "alice"})
	if !d.Allowed {
		t.Fatalf("alice first rejected")
	}
	d, _ = rl.Allow(context.Background(), Scope{User: "alice"})
	if d.Allowed {
		t.Fatalf("alice second admitted")
	}
	d, _ = rl.Allow(context.Background(), Scope{User: "bob"})
	if !d.Allowed {
		t.Fatalf("bob first rejected")
	}
}

func TestPolicyAllow_MissingDimensionSkipsPolicy(t *testing.T) {
	rl := NewRateLimiter([]RateLimitPolicy{{
		ID:         "needs-tenant",
		Dimensions: []RateDimension{DimTenant, DimUser},
		Rate:       1,
		Window:     time.Minute,
	}})
	// Without Tenant, policy is skipped → any number of calls admitted.
	for i := 0; i < 10; i++ {
		d, err := rl.Allow(context.Background(), Scope{User: "alice"})
		if err != nil || !d.Allowed {
			t.Fatalf("call %d wrongly rejected (Tenant absent)", i)
		}
	}
}

func TestPolicyAllow_Concurrent(t *testing.T) {
	rl := NewRateLimiter([]RateLimitPolicy{{
		ID:         "gate",
		Dimensions: []RateDimension{DimUser},
		Rate:       5,
		Window:     time.Minute,
	}})
	var admitted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, _ := rl.Allow(context.Background(), Scope{User: "alice"})
			if d.Allowed {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := admitted.Load(); got != 5 {
		t.Fatalf("admitted = %d, want 5", got)
	}
}

func TestParsePolicies_JSONOverride(t *testing.T) {
	raw := `[
		{"id":"tighter","dimensions":["user"],"rate":2,"window":"30s"},
		{"id":"loose","dimensions":["chat"],"rate":100,"window":"1m"}
	]`
	policies, err := ParsePolicies(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(policies) != 2 {
		t.Fatalf("policies = %d, want 2", len(policies))
	}
	if policies[0].Rate != 2 || policies[0].Window != 30*time.Second {
		t.Fatalf("tighter misparsed: %+v", policies[0])
	}
}

func TestParsePolicies_RejectsInvalid(t *testing.T) {
	bad := []string{
		`[{"id":"","dimensions":["user"],"rate":1,"window":"1m"}]`,
		`[{"id":"x","dimensions":["user"],"rate":0,"window":"1m"}]`,
		`[{"id":"x","dimensions":["user"],"rate":1,"window":"invalid"}]`,
	}
	for _, raw := range bad {
		if _, err := ParsePolicies(raw); err == nil {
			t.Fatalf("expected error for: %s", raw)
		}
	}
}

func TestDefaultPolicies_ContainsCoreSet(t *testing.T) {
	p := DefaultPolicies()
	names := map[string]bool{}
	for _, pol := range p {
		names[pol.ID] = true
	}
	for _, want := range []string{"session-default", "write-action", "destructive-action", "per-chat"} {
		if !names[want] {
			t.Fatalf("default policies missing %s", want)
		}
	}
}

func TestActionClassForCommand(t *testing.T) {
	cases := map[string]RateActionClass{
		"":         ActionClassRead,
		"/help":    ActionClassRead,
		"/task":    ActionClassWrite,
		"/agent":   ActionClassWrite,
		"/tools":   ActionClassDestructive,
		"/unknown": ActionClassRead,
	}
	for cmd, want := range cases {
		if got := ActionClassForCommand(cmd); got != want {
			t.Fatalf("ActionClassForCommand(%q) = %s, want %s", cmd, got, want)
		}
	}
}
