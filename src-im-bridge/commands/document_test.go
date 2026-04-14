package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

type docTestPlatform struct {
	mu      sync.Mutex
	replies []string
}

func (p *docTestPlatform) Name() string                                                  { return "test-stub" }
func (p *docTestPlatform) Start(handler core.MessageHandler) error                       { return nil }
func (p *docTestPlatform) Stop() error                                                   { return nil }
func (p *docTestPlatform) Send(ctx context.Context, chatID string, content string) error { return nil }
func (p *docTestPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.replies = append(p.replies, content)
	return nil
}

func TestDocCommand_NoSubcommandShowsUsage(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &docTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterDocumentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "test-stub",
		Content:  "/doc",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(platform.replies))
	}
	if !strings.Contains(platform.replies[0], "/doc") {
		t.Fatalf("expected usage text, got %q", platform.replies[0])
	}
}

func TestDocCommand_ListFormatsDocuments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/proj/documents" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.DocumentEntry{
			{ID: "d-1", Name: "design.pdf", Type: "pdf", Size: "2.3 MB", Status: "ready"},
			{ID: "d-2", Name: "notes.md", Type: "markdown", Size: "14 KB", Status: "processing"},
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &docTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterDocumentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "test-stub",
		Content:  "/doc list",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(platform.replies))
	}
	reply := platform.replies[0]
	if !strings.Contains(reply, "design.pdf") {
		t.Fatalf("reply missing design.pdf: %q", reply)
	}
	if !strings.Contains(reply, "notes.md") {
		t.Fatalf("reply missing notes.md: %q", reply)
	}
	if !strings.Contains(reply, "ready") {
		t.Fatalf("reply missing status 'ready': %q", reply)
	}
}

func TestDocCommand_ListEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.DocumentEntry{})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &docTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterDocumentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "test-stub",
		Content:  "/doc list",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(platform.replies))
	}
	if platform.replies[0] != "No documents found." {
		t.Fatalf("expected empty message, got %q", platform.replies[0])
	}
}

func TestDocCommand_UploadRequiresURL(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &docTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterDocumentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "test-stub",
		Content:  "/doc upload",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(platform.replies))
	}
	if !strings.Contains(platform.replies[0], "/doc upload") {
		t.Fatalf("expected usage text, got %q", platform.replies[0])
	}
}

func TestDocCommand_UploadRejectsInvalidURL(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &docTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterDocumentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "test-stub",
		Content:  "/doc upload not-a-url",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(platform.replies))
	}
	if !strings.Contains(platform.replies[0], "http://") {
		t.Fatalf("expected URL validation error, got %q", platform.replies[0])
	}
}

func TestDocCommand_UploadSuccess(t *testing.T) {
	// Serve a fake file to download.
	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("fake file content"))
	}))
	defer fileServer.Close()

	// Backend upload endpoint.
	var uploadReceived bool
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/projects/proj/documents/upload") {
			uploadReceived = true
			if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "multipart/form-data") {
				t.Fatalf("expected multipart content-type, got %q", ct)
			}
			w.WriteHeader(http.StatusCreated)
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer backend.Close()

	apiClient := client.NewAgentForgeClient(backend.URL, "proj", "secret")
	platform := &docTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterDocumentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "test-stub",
		Content:  "/doc upload " + fileServer.URL + "/report.pdf",
	})

	if !uploadReceived {
		t.Fatal("backend did not receive upload request")
	}
	if len(platform.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(platform.replies))
	}
	if !strings.Contains(platform.replies[0], "成功") {
		t.Fatalf("expected success message, got %q", platform.replies[0])
	}
}

func TestFormatDocumentList(t *testing.T) {
	docs := []client.DocumentEntry{
		{Name: "spec.pdf", Type: "pdf", Size: "1.2 MB", Status: "ready"},
	}
	result := formatDocumentList(docs)
	if !strings.Contains(result, "\U0001F4C4 spec.pdf") {
		t.Fatalf("missing emoji + name in %q", result)
	}
	if !strings.Contains(result, "(pdf, 1.2 MB)") {
		t.Fatalf("missing type+size in %q", result)
	}
	if !strings.Contains(result, "ready") {
		t.Fatalf("missing status in %q", result)
	}
}
