package core

import "testing"

func TestNormalizePlatformName_TrimsLowercasesAndRemovesStubSuffix(t *testing.T) {
	tests := map[string]string{
		" Slack-Stub ": "slack",
		"FEISHU":       "feishu",
		"dingtalk":     "dingtalk",
		"   ":          "",
	}

	for input, want := range tests {
		if got := NormalizePlatformName(input); got != want {
			t.Fatalf("NormalizePlatformName(%q) = %q, want %q", input, got, want)
		}
	}
}
