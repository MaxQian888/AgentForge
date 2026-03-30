package i18n

import (
	"slices"
	"testing"
)

func TestNewLocalizerInitializesBundleAndUsesRequestedLanguage(t *testing.T) {
	previous := Bundle
	Bundle = nil
	t.Cleanup(func() {
		Bundle = previous
	})

	localizer := NewLocalizer("  zh-CN  ", "en")
	if Bundle == nil {
		t.Fatal("expected NewLocalizer to initialize Bundle")
	}

	got := Localize(localizer, MsgInternalError)
	want := "\u670d\u52a1\u5668\u5185\u90e8\u9519\u8bef"
	if got != want {
		t.Fatalf("Localize(zh-CN) = %q, want %q", got, want)
	}
}

func TestLocalizeFallsBackToEnglishWhenLocalizerIsNil(t *testing.T) {
	previous := Bundle
	Bundle = nil
	t.Cleanup(func() {
		Bundle = previous
	})

	got := Localize(nil, MsgInvalidProjectID)
	want := "invalid project ID"
	if got != want {
		t.Fatalf("Localize(nil) = %q, want %q", got, want)
	}
}

func TestLocalizeReturnsMessageIDWhenTranslationIsMissing(t *testing.T) {
	previous := Bundle
	Bundle = nil
	t.Cleanup(func() {
		Bundle = previous
	})

	const missingID = "UnknownMessageID"
	if got := Localize(NewLocalizer("en"), missingID); got != missingID {
		t.Fatalf("Localize(missing) = %q, want %q", got, missingID)
	}
}

func TestNormalizeLangsTrimsWhitespaceAndDefaultsToEnglish(t *testing.T) {
	if got := normalizeLangs([]string{" ", "\t", "\n"}); !slices.Equal(got, []string{DefaultLocale}) {
		t.Fatalf("normalizeLangs(blank) = %v, want [%s]", got, DefaultLocale)
	}

	got := normalizeLangs([]string{"  zh-CN  ", "", " en "})
	want := []string{"zh-CN", "en"}
	if !slices.Equal(got, want) {
		t.Fatalf("normalizeLangs(mixed) = %v, want %v", got, want)
	}
}
