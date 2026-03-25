package core

import "testing"

func TestParseActionReference_ParsesTrimmedActionAndEntity(t *testing.T) {
	action, entityID, ok := ParseActionReference("  act:approve:review-1  ")
	if !ok {
		t.Fatal("expected action reference to parse")
	}
	if action != "approve" || entityID != "review-1" {
		t.Fatalf("got action=%q entityID=%q", action, entityID)
	}
}

func TestParseActionReference_RejectsInvalidShapes(t *testing.T) {
	cases := []string{
		"",
		"link:https://example.test",
		"approve:review-1",
		"act:approve",
		"act::review-1",
		"act:approve:",
	}

	for _, raw := range cases {
		if action, entityID, ok := ParseActionReference(raw); ok {
			t.Fatalf("ParseActionReference(%q) = (%q, %q, true), want invalid", raw, action, entityID)
		}
	}
}
