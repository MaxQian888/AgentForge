package core

import (
	"strings"
	"testing"
)

func TestResolveReplyPlan_HonorsOpenThreadPolicyWhenSupported(t *testing.T) {
	metadata := NormalizeMetadata(PlatformMetadata{Source: "slack"}, "slack")
	target := &ReplyTarget{Platform: "slack", ChatID: "C1", ThreadPolicy: ThreadPolicyOpen}

	plan := ResolveReplyPlan(metadata, target, "C1")
	if plan.Method != DeliveryMethodOpenThread {
		t.Fatalf("Method = %q, want open_thread", plan.Method)
	}
}

func TestResolveReplyPlan_DegradesToReplyWhenThreadUnsupported(t *testing.T) {
	metadata := NormalizeMetadata(PlatformMetadata{Source: "wecom"}, "wecom")
	target := &ReplyTarget{Platform: "wecom", ChatID: "C1", ThreadPolicy: ThreadPolicyOpen, UseReply: true}

	plan := ResolveReplyPlan(metadata, target, "C1")
	if plan.Method == DeliveryMethodOpenThread {
		t.Fatalf("wecom should not route to open_thread, got %q", plan.Method)
	}
	if !strings.Contains(plan.FallbackReason, "thread_open_unsupported") {
		t.Fatalf("fallback_reason = %q, want thread_open_unsupported", plan.FallbackReason)
	}
}

func TestApplyIsolatePrefix_AddsSessionMarker(t *testing.T) {
	target := &ReplyTarget{ThreadParentID: "abcdef12345678", ThreadPolicy: ThreadPolicyIsolate}
	got := applyIsolatePrefix(target, "hello world")
	if !strings.HasPrefix(got, "[session: ") {
		t.Fatalf("got = %q, want [session: ...] prefix", got)
	}
	if !strings.HasSuffix(got, "hello world") {
		t.Fatalf("got = %q, missing original content", got)
	}
	// Idempotent
	again := applyIsolatePrefix(target, got)
	if again != got {
		t.Fatalf("applyIsolatePrefix not idempotent: %q -> %q", got, again)
	}
}
