package core

import (
	"testing"
)

func TestAllowlist_EmptyAdmitsAll(t *testing.T) {
	al := NewCommandAllowlist("")
	if al.Enabled() {
		t.Fatalf("empty allowlist should be disabled")
	}
	if !al.Permit("feishu", "/anything") {
		t.Fatalf("disabled allowlist should admit everything")
	}
}

func TestAllowlist_WildcardAdmitsAllForPlatform(t *testing.T) {
	al := NewCommandAllowlist("slack:/*")
	if !al.Permit("slack", "/task") {
		t.Fatalf("slack:/* should admit /task")
	}
	if al.Permit("feishu", "/task") {
		t.Fatalf("slack-only rule should deny feishu")
	}
}

func TestAllowlist_PlatformWildcardAdmitsAllPlatforms(t *testing.T) {
	al := NewCommandAllowlist("*:/help")
	if !al.Permit("slack", "/help") {
		t.Fatalf("*:/help should admit slack /help")
	}
	if !al.Permit("feishu", "/help") {
		t.Fatalf("*:/help should admit feishu /help")
	}
	if al.Permit("feishu", "/task") {
		t.Fatalf("should deny commands not listed")
	}
}

func TestAllowlist_DenyTakesPrecedence(t *testing.T) {
	al := NewCommandAllowlist("slack:/*,!slack:/tools")
	if !al.Permit("slack", "/task") {
		t.Fatalf("slack:/* should admit /task")
	}
	if al.Permit("slack", "/tools") {
		t.Fatalf("deny rule should beat allow wildcard")
	}
}

func TestAllowlist_MultipleExplicit(t *testing.T) {
	al := NewCommandAllowlist("feishu:/task,feishu:/help,slack:/help")
	if !al.Permit("feishu", "/task") {
		t.Fatalf("feishu /task should be permitted")
	}
	if al.Permit("feishu", "/agent") {
		t.Fatalf("unlisted command should be denied")
	}
	if !al.Permit("slack", "/help") {
		t.Fatalf("slack /help should be permitted")
	}
	if al.Permit("slack", "/task") {
		t.Fatalf("unlisted slack command should be denied")
	}
}

func TestAllowlist_CaseInsensitivePlatform(t *testing.T) {
	al := NewCommandAllowlist("Slack:/task")
	if !al.Permit("slack", "/task") {
		t.Fatalf("platform comparison should be case-insensitive")
	}
}

func TestAllowlist_IgnoresMalformedEntries(t *testing.T) {
	al := NewCommandAllowlist(",slack,slack:/ok,bad_entry")
	if !al.Permit("slack", "/ok") {
		t.Fatalf("valid entry should still work after malformed neighbors")
	}
	if al.Permit("slack", "/nope") {
		t.Fatalf("unlisted command still denied")
	}
}
