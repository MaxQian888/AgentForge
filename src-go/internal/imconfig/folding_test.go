package imconfig

import "testing"

func TestFoldingModeFor_KnownPlatforms(t *testing.T) {
	cases := []struct {
		platform string
		want     FoldingMode
	}{
		{"feishu", FoldingModeNested},
		{"FEISHU", FoldingModeNested},
		{"slack", FoldingModeNested},
		{"telegram", FoldingModeNested},
		{"discord", FoldingModeNested},
		{"wecom", FoldingModeNested},
		{"qq", FoldingModeFrontendOnly},
		{"QQ", FoldingModeFrontendOnly},
		{"qqbot", FoldingModeFrontendOnly},
		{"QQBOT", FoldingModeFrontendOnly},
	}
	for _, tc := range cases {
		t.Run(tc.platform, func(t *testing.T) {
			got := FoldingModeFor(tc.platform)
			if got != tc.want {
				t.Errorf("FoldingModeFor(%q) = %q, want %q", tc.platform, got, tc.want)
			}
		})
	}
}

func TestFoldingModeFor_UnknownPlatformReturnsFlat(t *testing.T) {
	cases := []string{"", "dingtalk", "email", "sms", "unknown", "  "}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			got := FoldingModeFor(p)
			if got != FoldingModeFlat {
				t.Errorf("FoldingModeFor(%q) = %q, want flat", p, got)
			}
		})
	}
}

func TestFoldingModeFor_CaseInsensitive(t *testing.T) {
	platforms := []string{"Feishu", "FEISHU", "feishu", " feishu "}
	for _, p := range platforms {
		t.Run(p, func(t *testing.T) {
			got := FoldingModeFor(p)
			if got != FoldingModeNested {
				t.Errorf("FoldingModeFor(%q) = %q, want nested", p, got)
			}
		})
	}
}
