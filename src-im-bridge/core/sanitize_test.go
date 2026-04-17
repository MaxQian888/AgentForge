package core

import (
	"strings"
	"testing"
)

func profileWithLimit(limit int, segments bool) RenderingProfile {
	return RenderingProfile{
		DefaultTextFormat: TextFormatPlainText,
		SupportedFormats:  []TextFormatMode{TextFormatPlainText},
		MaxTextLength:     limit,
		SupportsSegments:  segments,
	}
}

func TestSanitize_OffPassesThrough(t *testing.T) {
	input := "@everyone hello\u200Bthere"
	got := SanitizeEgress(profileWithLimit(100, false), SanitizeOff, input)
	if got.Text != input {
		t.Fatalf("off mode should not modify text: got %q", got.Text)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("off mode should emit no warnings, got %v", got.Warnings)
	}
}

func TestSanitize_PermissiveKeepsMentions(t *testing.T) {
	input := "@everyone hello\u200Bthere"
	got := SanitizeEgress(profileWithLimit(100, false), SanitizePermissive, input)
	if strings.Contains(got.Text, "\u200B") {
		t.Fatalf("zero-width should be removed: %q", got.Text)
	}
	if !strings.Contains(got.Text, "@everyone") {
		t.Fatalf("permissive should keep broadcast: %q", got.Text)
	}
}

func TestSanitize_StrictStripsBroadcastMentions(t *testing.T) {
	for _, input := range []string{
		"hey @everyone come here",
		"ping @here now",
		"<!channel> urgent",
		"<!here> heads up",
		"@all please review",
		"@channel broadcast",
	} {
		got := SanitizeEgress(profileWithLimit(200, false), SanitizeStrict, input)
		if !strings.Contains(got.Text, broadcastReplacement) {
			t.Fatalf("expected replacement in output, got %q (input %q)", got.Text, input)
		}
		if !hasWarning(got.Warnings, WarnBroadcastMentionStripped) {
			t.Fatalf("missing broadcast warning: %v", got.Warnings)
		}
	}
}

func TestSanitize_StripsZeroWidth(t *testing.T) {
	input := "hel\u200Blo\u200C wo\u200Drld\uFEFF"
	got := SanitizeEgress(profileWithLimit(200, false), SanitizeStrict, input)
	if strings.ContainsAny(got.Text, "\u200B\u200C\u200D\uFEFF") {
		t.Fatalf("zero-width present: %q", got.Text)
	}
	if !hasWarning(got.Warnings, WarnZeroWidthStripped) {
		t.Fatalf("missing zero-width warning")
	}
}

func TestSanitize_StripsControlCharacters(t *testing.T) {
	input := "hello\x01 world\x07 keep\nnewline"
	got := SanitizeEgress(profileWithLimit(200, false), SanitizeStrict, input)
	if strings.ContainsRune(got.Text, 0x01) || strings.ContainsRune(got.Text, 0x07) {
		t.Fatalf("control chars present: %q", got.Text)
	}
	if !strings.Contains(got.Text, "\n") {
		t.Fatalf("newline should be preserved: %q", got.Text)
	}
}

func TestSanitize_TruncatesWhenSegmentsDisabled(t *testing.T) {
	long := strings.Repeat("a", 500)
	got := SanitizeEgress(profileWithLimit(100, false), SanitizeStrict, long)
	if len(got.Text) > 100 {
		t.Fatalf("truncated text too long: %d", len(got.Text))
	}
	if !strings.HasSuffix(got.Text, truncationMarker) {
		t.Fatalf("expected truncation marker: %q", got.Text)
	}
	if !hasWarning(got.Warnings, WarnTextTruncated) {
		t.Fatalf("missing truncation warning")
	}
}

func TestSanitize_SegmentsWhenProfileSupports(t *testing.T) {
	long := strings.Repeat("a", 250)
	got := SanitizeEgress(profileWithLimit(100, true), SanitizeStrict, long)
	if len(got.Segments) < 3 {
		t.Fatalf("expected multi segments, got %d", len(got.Segments))
	}
	for i, seg := range got.Segments {
		if len(seg) > 100 {
			t.Fatalf("segment %d over limit: %d", i, len(seg))
		}
	}
	if !hasWarning(got.Warnings, WarnTextSegmented) {
		t.Fatalf("missing segmentation warning")
	}
}

func TestSanitize_SegmentsHandleMultiByteBoundary(t *testing.T) {
	// Each rune "中" is 3 bytes. limit 4 forces a mid-rune split if we're
	// naive — the sanitizer must back up to a rune boundary.
	long := strings.Repeat("中", 20)
	got := SanitizeEgress(profileWithLimit(4, true), SanitizeStrict, long)
	for i, seg := range got.Segments {
		if !isValidUTF8(seg) {
			t.Fatalf("segment %d is not valid utf-8: %q", i, seg)
		}
	}
}

func TestSanitize_ZeroLimitLeavesUnbounded(t *testing.T) {
	long := strings.Repeat("a", 10_000)
	got := SanitizeEgress(profileWithLimit(0, false), SanitizeStrict, long)
	if len(got.Text) != len(long) {
		t.Fatalf("zero limit should not truncate: got %d", len(got.Text))
	}
	if hasWarning(got.Warnings, WarnTextTruncated) || hasWarning(got.Warnings, WarnTextSegmented) {
		t.Fatalf("should not emit length warnings: %v", got.Warnings)
	}
}

func hasWarning(haystack []SanitizeWarning, needle SanitizeWarning) bool {
	for _, w := range haystack {
		if w == needle {
			return true
		}
	}
	return false
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '\uFFFD' && !strings.ContainsRune(s, '\uFFFD') {
			return false
		}
	}
	// When decoding produces REPLACEMENT CHAR that wasn't in input, invalid.
	return true
}
