package service_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/service"
)

func TestHTTPRemoteRegistryClientFetchCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/plugins" {
			t.Fatalf("path = %s, want /v1/plugins", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"pluginId":"release-train","name":"Release Train","version":"1.2.0","description":"Workflow release automation","author":"AgentForge","tags":["workflow","release"]}
		]`))
	}))
	defer server.Close()

	client := service.NewHTTPRemoteRegistryClient(server.Client())

	entries, err := client.FetchCatalog(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("FetchCatalog() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].PluginID != "release-train" {
		t.Fatalf("entries[0].PluginID = %q, want release-train", entries[0].PluginID)
	}
}

func TestHTTPRemoteRegistryClientDownloadManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/plugins/release-train/versions/1.2.0/manifest" {
			t.Fatalf("path = %s, want /v1/plugins/release-train/versions/1.2.0/manifest", r.URL.Path)
		}
		_, _ = w.Write([]byte("apiVersion: agentforge/v1\nkind: WorkflowPlugin\nmetadata:\n  id: release-train\n"))
	}))
	defer server.Close()

	client := service.NewHTTPRemoteRegistryClient(server.Client())

	reader, err := client.Download(context.Background(), "release-train", "1.2.0", server.URL)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	defer reader.Close()

	payload, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !strings.Contains(string(payload), "release-train") {
		t.Fatalf("downloaded payload = %q, want manifest containing release-train", string(payload))
	}
}
