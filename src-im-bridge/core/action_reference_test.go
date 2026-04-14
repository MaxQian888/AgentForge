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

func TestActionReference_BuildAndParseMetadata(t *testing.T) {
	raw := BuildActionReference("create-task", "project-1", map[string]string{
		"title":    "Follow up",
		"body":     "Created from IM card",
		"priority": "high",
	})

	action, entityID, metadata, ok := ParseActionReferenceWithMetadata(raw)
	if !ok {
		t.Fatalf("expected action reference %q to parse", raw)
	}
	if action != "create-task" || entityID != "project-1" {
		t.Fatalf("got action=%q entityID=%q", action, entityID)
	}
	if metadata["title"] != "Follow up" || metadata["body"] != "Created from IM card" || metadata["priority"] != "high" {
		t.Fatalf("metadata = %+v", metadata)
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
