package main

import (
	"sort"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

// TestAllBuiltinProvidersRegistered verifies every in-tree provider
// package is blank-imported from main.go and its init() has run. If a new
// provider package is added without an accompanying `import _ "..."` in
// main.go, this test fails loudly in CI with the missing id.
func TestAllBuiltinProvidersRegistered(t *testing.T) {
	expected := []string{
		"dingtalk",
		"discord",
		"email",
		"feishu",
		"qq",
		"qqbot",
		"slack",
		"telegram",
		"wechat",
		"wecom",
	}
	sort.Strings(expected)

	got := map[string]bool{}
	for _, f := range core.RegisteredProviders() {
		got[f.ID] = true
	}

	var missing []string
	for _, id := range expected {
		if !got[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		t.Errorf("providers not registered (likely missing blank import in main.go): %v", missing)
	}
}
