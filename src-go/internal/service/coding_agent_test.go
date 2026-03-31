package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestResolveProjectCodingAgentSelectionSupportsAdditionalBackends(t *testing.T) {
	t.Parallel()

	project := &model.Project{
		ID:       uuid.New(),
		Name:     "AgentForge",
		Slug:     "agentforge",
		Settings: "{}",
	}

	tests := []struct {
		name     string
		runtime  string
		provider string
		wantModel string
	}{
		{
			name:      "cursor",
			runtime:   "cursor",
			provider:  "cursor",
			wantModel: "claude-sonnet-4-20250514",
		},
		{
			name:      "gemini",
			runtime:   "gemini",
			provider:  "google",
			wantModel: "gemini-2.5-pro",
		},
		{
			name:      "qoder",
			runtime:   "qoder",
			provider:  "qoder",
			wantModel: "auto",
		},
		{
			name:      "iflow",
			runtime:   "iflow",
			provider:  "iflow",
			wantModel: "Qwen3-Coder",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			selection, err := ResolveProjectCodingAgentSelection(project, tt.runtime, tt.provider, "")
			if err != nil {
				t.Fatalf("ResolveProjectCodingAgentSelection() error = %v", err)
			}

			if selection.Runtime != tt.runtime {
				t.Fatalf("runtime = %q, want %q", selection.Runtime, tt.runtime)
			}
			if selection.Provider != tt.provider {
				t.Fatalf("provider = %q, want %q", selection.Provider, tt.provider)
			}
			if selection.Model != tt.wantModel {
				t.Fatalf("model = %q, want %q", selection.Model, tt.wantModel)
			}
		})
	}
}

func TestDefaultCodingAgentCatalogIncludesAdditionalBackends(t *testing.T) {
	t.Parallel()

	catalog := DefaultCodingAgentCatalog(model.CodingAgentSelection{
		Runtime:  "cursor",
		Provider: "cursor",
		Model:    "claude-sonnet-4-20250514",
	})

	if catalog == nil {
		t.Fatal("expected coding agent catalog")
	}

	if len(catalog.Runtimes) < 7 {
		t.Fatalf("runtime count = %d, want at least 7", len(catalog.Runtimes))
	}

	runtimeByKey := make(map[string]model.CodingAgentRuntimeOptionDTO, len(catalog.Runtimes))
	for _, runtime := range catalog.Runtimes {
		runtimeByKey[runtime.Runtime] = runtime
	}

	for _, key := range []string{"claude_code", "codex", "opencode", "cursor", "gemini", "qoder", "iflow"} {
		if _, ok := runtimeByKey[key]; !ok {
			t.Fatalf("expected runtime %q in default catalog", key)
		}
	}

	if got := runtimeByKey["gemini"].DefaultProvider; got != "google" {
		t.Fatalf("gemini default provider = %q, want google", got)
	}
	if got := runtimeByKey["qoder"].DefaultModel; got != "auto" {
		t.Fatalf("qoder default model = %q, want auto", got)
	}
	if got := runtimeByKey["iflow"].DefaultModel; got != "Qwen3-Coder" {
		t.Fatalf("iflow default model = %q, want Qwen3-Coder", got)
	}
}
