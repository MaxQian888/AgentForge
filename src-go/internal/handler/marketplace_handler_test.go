package handler_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/handler"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	rolepkg "github.com/agentforge/server/internal/role"
	"github.com/agentforge/server/internal/service"
	"github.com/labstack/echo/v4"
)

func newMarketplaceHandlerForTest(
	t *testing.T,
	repo service.PluginRegistry,
	pluginsDir string,
	rolesDir string,
	marketURL string,
) *handler.MarketplaceHandler {
	t.Helper()
	if repo == nil {
		repo = repository.NewPluginRegistryRepository()
	}
	svc := service.NewPluginService(repo, handlerRuntimeClient{}, nil, pluginsDir)
	return handler.NewMarketplaceHandler(svc, marketURL, pluginsDir, rolesDir)
}

func newMarketplaceMetadataServer(t *testing.T) *httptest.Server {
	t.Helper()
	pluginArtifact := buildZipArchive(t, map[string]string{
		"repo-search/manifest.yaml": `apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: repo-search
  name: Repo Search
  version: 1.0.0
  description: Search the repository
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: [dist/index.js]
source:
  type: marketplace
`,
	})
	roleArtifact := buildZipArchive(t, map[string]string{
		"role.yaml": `apiVersion: agentforge/v1
kind: Role
metadata:
  id: role-item
  name: Marketplace Role
  version: 1.0.0
  description: Installed from marketplace
  author: AgentForge
identity:
  role: Marketplace Role
  goal: Help with delivery
  backstory: Installed by marketplace
capabilities:
  languages: [TypeScript]
  frameworks: [Next.js]
knowledge:
  repositories: []
  documents: []
  patterns: []
security:
  allowed_paths: []
  denied_paths: []
  max_budget_usd: 5
  require_review: true
`,
	})
	skillArtifact := buildZipArchive(t, map[string]string{
		"SKILL.md": `---
name: skill-item
description: Marketplace skill
---

# Marketplace Skill
`,
		"references/usage.md": "# Usage\n",
	})
	invalidRoleArtifact := buildZipArchive(t, map[string]string{
		"README.md": "# missing role manifest\n",
	})

	digest := sha256.Sum256(pluginArtifact)
	roleDigest := sha256.Sum256(roleArtifact)
	skillDigest := sha256.Sum256(skillArtifact)
	invalidRoleDigest := sha256.Sum256(invalidRoleArtifact)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/items/plugin-item", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":             "plugin-item",
			"type":           "plugin",
			"name":           "Plugin Item",
			"slug":           "plugin-item",
			"latest_version": "2.0.0",
		})
	})
	mux.HandleFunc("/api/v1/items/role-item", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":             "role-item",
			"type":           "role",
			"name":           "Role Item",
			"slug":           "role-item",
			"latest_version": "1.0.0",
		})
	})
	mux.HandleFunc("/api/v1/items/skill-item", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":             "skill-item",
			"type":           "skill",
			"name":           "Skill Item",
			"slug":           "skill-item",
			"latest_version": "1.0.0",
		})
	})
	mux.HandleFunc("/api/v1/items/invalid-role-item", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":             "invalid-role-item",
			"type":           "role",
			"name":           "Invalid Role Item",
			"slug":           "invalid-role-item",
			"latest_version": "1.0.0",
		})
	})
	mux.HandleFunc("/api/v1/items/plugin-item/versions/1.0.0/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Digest", hex.EncodeToString(digest[:]))
		_, _ = w.Write(pluginArtifact)
	})
	mux.HandleFunc("/api/v1/items/role-item/versions/1.0.0/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Digest", hex.EncodeToString(roleDigest[:]))
		_, _ = w.Write(roleArtifact)
	})
	mux.HandleFunc("/api/v1/items/skill-item/versions/1.0.0/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Digest", hex.EncodeToString(skillDigest[:]))
		_, _ = w.Write(skillArtifact)
	})
	mux.HandleFunc("/api/v1/items/invalid-role-item/versions/1.0.0/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Digest", hex.EncodeToString(invalidRoleDigest[:]))
		_, _ = w.Write(invalidRoleArtifact)
	})

	return httptest.NewServer(mux)
}

func TestMarketplaceHandler_InstallReturnsUnavailableWhenMarketplaceURLMissing(t *testing.T) {
	rootDir := t.TempDir()
	h := newMarketplaceHandlerForTest(
		t,
		nil,
		filepath.Join(rootDir, "plugins"),
		filepath.Join(rootDir, "roles"),
		"",
	)
	e := echo.New()
	body := bytes.NewBufferString(`{"item_id":"plugin-item","version":"1.0.0"}`)
	req := httptest.NewRequest(http.MethodPost, "/marketplace/install", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	if err := h.Install(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}

	var payload struct {
		ErrorCode string `json:"errorCode"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode install payload: %v", err)
	}
	if payload.ErrorCode != "marketplace_unconfigured" {
		t.Fatalf("errorCode = %q, want marketplace_unconfigured", payload.ErrorCode)
	}
}

func TestMarketplaceHandler_InstallAndConsumptionReportTypedStates(t *testing.T) {
	repo := repository.NewPluginRegistryRepository()
	rootDir := t.TempDir()
	pluginsDir := filepath.Join(rootDir, "plugins")
	rolesDir := filepath.Join(rootDir, "roles")
	server := newMarketplaceMetadataServer(t)
	defer server.Close()

	record := &model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindTool,
			Metadata: model.PluginMetadata{
				ID:      "legacy-search",
				Name:    "Legacy Search",
				Version: "2.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:   model.PluginRuntimeMCP,
				Transport: "stdio",
				Command:   "node",
			},
			Source: model.PluginSource{
				Type:    model.PluginSourceMarketplace,
				Catalog: "installed-plugin-item",
				Ref:     "2.0.0",
				Path:    "C:/plugins/repo-search",
			},
		},
		LifecycleState: model.PluginStateInstalled,
		RuntimeHost:    model.PluginHostTSBridge,
	}
	if err := repo.Save(context.Background(), record); err != nil {
		t.Fatalf("save installed marketplace plugin: %v", err)
	}

	h := newMarketplaceHandlerForTest(t, repo, pluginsDir, rolesDir, server.URL)
	e := echo.New()

	for _, tc := range []struct {
		name           string
		itemID         string
		wantStatusCode int
		wantType       string
		wantStatus     string
		wantSurface    string
		wantErrorCode  string
		wantInstalled  bool
	}{
		{
			name:           "role installs into roles workspace",
			itemID:         "role-item",
			wantStatusCode: http.StatusOK,
			wantType:       "role",
			wantStatus:     "installed",
			wantSurface:    "roles-workspace",
			wantErrorCode:  "",
			wantInstalled:  true,
		},
		{
			name:           "skill installs into role skill catalog",
			itemID:         "skill-item",
			wantStatusCode: http.StatusOK,
			wantType:       "skill",
			wantStatus:     "installed",
			wantSurface:    "role-skill-catalog",
			wantErrorCode:  "",
			wantInstalled:  true,
		},
		{
			name:           "plugin installs materialize into the plugin control plane",
			itemID:         "plugin-item",
			wantStatusCode: http.StatusOK,
			wantType:       "plugin",
			wantStatus:     "installed",
			wantSurface:    "plugin-management-panel",
			wantErrorCode:  "",
			wantInstalled:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{
				"item_id": tc.itemID,
				"version": "1.0.0",
			})
			req := httptest.NewRequest(http.MethodPost, "/marketplace/install", bytes.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			if err := h.Install(e.NewContext(req, rec)); err != nil {
				t.Fatalf("Install() error = %v", err)
			}
			if rec.Code != tc.wantStatusCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatusCode)
			}

			var payload struct {
				ErrorCode string `json:"errorCode"`
				Item      struct {
					ItemID          string `json:"itemId"`
					ItemType        string `json:"itemType"`
					Status          string `json:"status"`
					ConsumerSurface string `json:"consumerSurface"`
					Installed       bool   `json:"installed"`
				} `json:"item"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode install payload: %v", err)
			}
			if payload.ErrorCode != tc.wantErrorCode {
				t.Fatalf("errorCode = %q, want %q", payload.ErrorCode, tc.wantErrorCode)
			}
			if payload.Item.ItemID != tc.itemID {
				t.Fatalf("itemId = %q, want %q", payload.Item.ItemID, tc.itemID)
			}
			if payload.Item.ItemType != tc.wantType {
				t.Fatalf("itemType = %q, want %q", payload.Item.ItemType, tc.wantType)
			}
			if payload.Item.Status != tc.wantStatus {
				t.Fatalf("status = %q, want %q", payload.Item.Status, tc.wantStatus)
			}
			if payload.Item.ConsumerSurface != tc.wantSurface {
				t.Fatalf("consumerSurface = %q, want %q", payload.Item.ConsumerSurface, tc.wantSurface)
			}
			if payload.Item.Installed != tc.wantInstalled {
				t.Fatalf("installed = %v, want %v", payload.Item.Installed, tc.wantInstalled)
			}
		})
	}

	if _, err := rolepkg.NewFileStore(rolesDir).Get("role-item"); err != nil {
		t.Fatalf("expected installed marketplace role to be discoverable, got error %v", err)
	}
	skillEntries, err := rolepkg.DiscoverSkillCatalog(filepath.Join(rootDir, "skills"))
	if err != nil {
		t.Fatalf("discover installed marketplace skills: %v", err)
	}
	foundSkill := false
	for _, entry := range skillEntries {
		if entry.Path == "skills/skill-item" {
			foundSkill = true
			break
		}
	}
	if !foundSkill {
		t.Fatalf("expected installed marketplace skill to be discoverable, got %+v", skillEntries)
	}

	req := httptest.NewRequest(http.MethodGet, "/marketplace/consumption", nil)
	rec := httptest.NewRecorder()
	if err := h.Consumption(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Consumption() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("consumption status = %d, want 200", rec.Code)
	}

	var payload struct {
		Items []struct {
			ItemID          string `json:"itemId"`
			ItemType        string `json:"itemType"`
			Version         string `json:"version"`
			Status          string `json:"status"`
			ConsumerSurface string `json:"consumerSurface"`
			Installed       bool   `json:"installed"`
			RecordID        string `json:"recordId"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode consumption payload: %v", err)
	}

	byID := make(map[string]struct {
		ItemType        string
		Version         string
		Status          string
		ConsumerSurface string
		Installed       bool
		RecordID        string
	}, len(payload.Items))
	for _, item := range payload.Items {
		byID[item.ItemID] = struct {
			ItemType        string
			Version         string
			Status          string
			ConsumerSurface string
			Installed       bool
			RecordID        string
		}{
			ItemType:        item.ItemType,
			Version:         item.Version,
			Status:          item.Status,
			ConsumerSurface: item.ConsumerSurface,
			Installed:       item.Installed,
			RecordID:        item.RecordID,
		}
	}

	if item, ok := byID["installed-plugin-item"]; !ok {
		t.Fatal("expected installed marketplace plugin state to be returned")
	} else {
		if item.ItemType != "plugin" {
			t.Fatalf("installed itemType = %q, want plugin", item.ItemType)
		}
		if item.Status != "installed" {
			t.Fatalf("installed status = %q, want installed", item.Status)
		}
		if !item.Installed {
			t.Fatal("expected installed marketplace plugin to report installed=true")
		}
		if item.RecordID != "legacy-search" {
			t.Fatalf("recordId = %q, want legacy-search", item.RecordID)
		}
	}

	if item, ok := byID["role-item"]; !ok {
		t.Fatal("expected installed role state to be returned")
	} else if item.Status != "installed" {
		t.Fatalf("role status = %q, want installed", item.Status)
	}

	if item, ok := byID["skill-item"]; !ok {
		t.Fatal("expected installed skill state to be returned")
	} else if item.Status != "installed" {
		t.Fatalf("skill status = %q, want installed", item.Status)
	}

	if item, ok := byID["plugin-item"]; !ok {
		t.Fatal("expected installed plugin install state to be returned")
	} else if item.Status != "installed" {
		t.Fatalf("plugin status = %q, want installed", item.Status)
	} else if item.RecordID != "repo-search" {
		t.Fatalf("plugin recordId = %q, want repo-search", item.RecordID)
	}
}

func TestMarketplaceHandler_InstallRejectsInvalidRoleArtifactBeforeDiscoveryChanges(t *testing.T) {
	rootDir := t.TempDir()
	server := newMarketplaceMetadataServer(t)
	defer server.Close()

	h := newMarketplaceHandlerForTest(
		t,
		nil,
		filepath.Join(rootDir, "plugins"),
		filepath.Join(rootDir, "roles"),
		server.URL,
	)
	e := echo.New()
	body := bytes.NewBufferString(`{"item_id":"invalid-role-item","version":"1.0.0"}`)
	req := httptest.NewRequest(http.MethodPost, "/marketplace/install", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	if err := h.Install(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var payload struct {
		ErrorCode string `json:"errorCode"`
		Item      struct {
			Status          string `json:"status"`
			ConsumerSurface string `json:"consumerSurface"`
		} `json:"item"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode invalid role payload: %v", err)
	}
	if payload.ErrorCode != "marketplace_invalid_artifact" {
		t.Fatalf("errorCode = %q, want marketplace_invalid_artifact", payload.ErrorCode)
	}
	if payload.Item.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", payload.Item.Status)
	}
	if payload.Item.ConsumerSurface != "roles-workspace" {
		t.Fatalf("consumerSurface = %q, want roles-workspace", payload.Item.ConsumerSurface)
	}
	if _, err := os.Stat(filepath.Join(rootDir, "roles", "invalid-role-item")); !os.IsNotExist(err) {
		t.Fatalf("expected no extracted invalid role directory, got err=%v", err)
	}
}

func TestMarketplaceHandler_ConsumptionIncludesOfficialBuiltInSkills(t *testing.T) {
	rootDir := t.TempDir()
	pluginsDir := filepath.Join(rootDir, "plugins")
	rolesDir := filepath.Join(rootDir, "roles")
	skillsDir := filepath.Join(rootDir, "skills")
	if err := os.MkdirAll(filepath.Join(skillsDir, "react", "agents"), 0o755); err != nil {
		t.Fatalf("mkdir react agents: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(skillsDir, "testing"), 0o755); err != nil {
		t.Fatalf("mkdir testing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "react", "SKILL.md"), []byte(`---
name: React
description: Build React surfaces.
requires:
  - skills/typescript
tools:
  - code_editor
  - browser_preview
---

# React

Build React surfaces.
`), 0o644); err != nil {
		t.Fatalf("write react skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "react", "agents", "openai.yaml"), []byte(`interface:
  display_name: "AgentForge React"
  short_description: "Build React safely"
  default_prompt: "Use React skill"
`), 0o644); err != nil {
		t.Fatalf("write react agent config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "testing", "SKILL.md"), []byte(`---
name: Testing
description: Add verification coverage.
tools:
  - terminal
---

# Testing
`), 0o644); err != nil {
		t.Fatalf("write testing skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "builtin-bundle.yaml"), []byte(`skills:
  - id: react
    root: react
    category: frontend
    tags:
      - react
      - nextjs
    featured: true
    docsRef: docs/role-yaml.md
`), 0o644); err != nil {
		t.Fatalf("write built-in skill bundle: %v", err)
	}

	h := newMarketplaceHandlerForTest(t, nil, pluginsDir, rolesDir, "")
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/marketplace/consumption", nil)
	rec := httptest.NewRecorder()

	if err := h.Consumption(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Consumption() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("consumption status = %d, want 200", rec.Code)
	}

	var payload struct {
		Items []struct {
			ItemID          string `json:"itemId"`
			ItemType        string `json:"itemType"`
			Status          string `json:"status"`
			ConsumerSurface string `json:"consumerSurface"`
			Installed       bool   `json:"installed"`
			Used            bool   `json:"used"`
			LocalPath       string `json:"localPath"`
			Provenance      struct {
				SourceType string `json:"sourceType"`
			} `json:"provenance"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode built-in consumption payload: %v", err)
	}

	var builtIn *struct {
		ItemID          string `json:"itemId"`
		ItemType        string `json:"itemType"`
		Status          string `json:"status"`
		ConsumerSurface string `json:"consumerSurface"`
		Installed       bool   `json:"installed"`
		Used            bool   `json:"used"`
		LocalPath       string `json:"localPath"`
		Provenance      struct {
			SourceType string `json:"sourceType"`
		} `json:"provenance"`
	}
	for i := range payload.Items {
		if payload.Items[i].ItemID == "react" {
			builtIn = &payload.Items[i]
			break
		}
	}
	if builtIn == nil {
		t.Fatalf("expected built-in react skill consumption record, got %+v", payload.Items)
	}
	if builtIn.ItemType != "skill" {
		t.Fatalf("itemType = %q, want skill", builtIn.ItemType)
	}
	if builtIn.Status != "installed" {
		t.Fatalf("status = %q, want installed", builtIn.Status)
	}
	if builtIn.ConsumerSurface != "role-skill-catalog" {
		t.Fatalf("consumerSurface = %q, want role-skill-catalog", builtIn.ConsumerSurface)
	}
	if !builtIn.Installed {
		t.Fatal("expected built-in skill to report installed=true")
	}
	if builtIn.Used {
		t.Fatal("expected built-in skill to remain available-but-not-used")
	}
	if builtIn.Provenance.SourceType != "builtin" {
		t.Fatalf("sourceType = %q, want builtin", builtIn.Provenance.SourceType)
	}
	if builtIn.LocalPath != filepath.Join(skillsDir, "react") {
		t.Fatalf("localPath = %q, want %q", builtIn.LocalPath, filepath.Join(skillsDir, "react"))
	}
}

func TestMarketplaceHandler_ListBuiltInSkillsReturnsStructuredSkillPreview(t *testing.T) {
	rootDir := t.TempDir()
	pluginsDir := filepath.Join(rootDir, "plugins")
	rolesDir := filepath.Join(rootDir, "roles")
	skillsDir := filepath.Join(rootDir, "skills")
	if err := os.MkdirAll(filepath.Join(skillsDir, "react", "agents"), 0o755); err != nil {
		t.Fatalf("mkdir react agents: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "react", "SKILL.md"), []byte(`---
name: React
description: Build React surfaces.
requires:
  - skills/typescript
tools:
  - code_editor
  - browser_preview
---

# React

Build product-facing React surfaces.
`), 0o644); err != nil {
		t.Fatalf("write react skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "react", "agents", "openai.yaml"), []byte(`interface:
  display_name: "AgentForge React"
  short_description: "Build React safely"
  default_prompt: "Use React skill"
`), 0o644); err != nil {
		t.Fatalf("write react agent config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "builtin-bundle.yaml"), []byte(`skills:
  - id: react
    root: react
    category: frontend
    tags:
      - react
      - nextjs
    featured: true
    docsRef: docs/role-yaml.md
`), 0o644); err != nil {
		t.Fatalf("write built-in bundle: %v", err)
	}

	h := newMarketplaceHandlerForTest(t, nil, pluginsDir, rolesDir, "")
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/marketplace/built-in-skills", nil)
	rec := httptest.NewRecorder()

	if err := h.ListBuiltInSkills(e.NewContext(req, rec)); err != nil {
		t.Fatalf("ListBuiltInSkills() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload []struct {
		ID           string `json:"id"`
		SourceType   string `json:"sourceType"`
		Category     string `json:"category"`
		IsFeatured   bool   `json:"is_featured"`
		LocalPath    string `json:"localPath"`
		SkillPreview struct {
			CanonicalPath   string `json:"canonicalPath"`
			FrontmatterYAML string `json:"frontmatterYaml"`
			MarkdownBody    string `json:"markdownBody"`
			AgentConfigs    []struct {
				Path string `json:"path"`
				Yaml string `json:"yaml"`
			} `json:"agentConfigs"`
		} `json:"skillPreview"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode built-in skill payload: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("len(payload) = %d, want 1", len(payload))
	}
	if payload[0].ID != "react" {
		t.Fatalf("id = %q, want react", payload[0].ID)
	}
	if payload[0].SourceType != "builtin" {
		t.Fatalf("sourceType = %q, want builtin", payload[0].SourceType)
	}
	if payload[0].Category != "frontend" {
		t.Fatalf("category = %q, want frontend", payload[0].Category)
	}
	if !payload[0].IsFeatured {
		t.Fatal("expected built-in skill to be featured")
	}
	if payload[0].LocalPath != filepath.Join(skillsDir, "react") {
		t.Fatalf("localPath = %q, want %q", payload[0].LocalPath, filepath.Join(skillsDir, "react"))
	}
	if payload[0].SkillPreview.CanonicalPath != "skills/react" {
		t.Fatalf("canonicalPath = %q, want skills/react", payload[0].SkillPreview.CanonicalPath)
	}
	if !strings.Contains(payload[0].SkillPreview.FrontmatterYAML, "name: React") {
		t.Fatalf("frontmatterYAML = %q, want normalized frontmatter", payload[0].SkillPreview.FrontmatterYAML)
	}
	if !strings.Contains(payload[0].SkillPreview.MarkdownBody, "Build product-facing React surfaces.") {
		t.Fatalf("markdownBody = %q, want skill markdown body", payload[0].SkillPreview.MarkdownBody)
	}
	if len(payload[0].SkillPreview.AgentConfigs) != 1 {
		t.Fatalf("agentConfigs len = %d, want 1", len(payload[0].SkillPreview.AgentConfigs))
	}
	if payload[0].SkillPreview.AgentConfigs[0].Path != "agents/openai.yaml" {
		t.Fatalf("agent config path = %q, want agents/openai.yaml", payload[0].SkillPreview.AgentConfigs[0].Path)
	}
}

func TestMarketplaceHandler_UninstallRemovesRoleArtifactAndConsumptionState(t *testing.T) {
	repo := repository.NewPluginRegistryRepository()
	rootDir := t.TempDir()
	pluginsDir := filepath.Join(rootDir, "plugins")
	rolesDir := filepath.Join(rootDir, "roles")
	server := newMarketplaceMetadataServer(t)
	defer server.Close()

	h := newMarketplaceHandlerForTest(t, repo, pluginsDir, rolesDir, server.URL)
	e := echo.New()

	// First install a role.
	body, _ := json.Marshal(map[string]string{"item_id": "role-item", "version": "1.0.0"})
	req := httptest.NewRequest(http.MethodPost, "/marketplace/install", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	if err := h.Install(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("install status = %d, want 200", rec.Code)
	}

	// Verify role directory exists.
	if _, err := os.Stat(filepath.Join(rolesDir, "role-item")); err != nil {
		t.Fatalf("expected role directory to exist after install, got %v", err)
	}

	// Now uninstall.
	body, _ = json.Marshal(map[string]string{"item_id": "role-item", "item_type": "role"})
	req = httptest.NewRequest(http.MethodPost, "/marketplace/uninstall", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	if err := h.Uninstall(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("uninstall status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}

	// Verify role directory removed.
	if _, err := os.Stat(filepath.Join(rolesDir, "role-item")); !os.IsNotExist(err) {
		t.Fatalf("expected role directory to be removed after uninstall, got err=%v", err)
	}

	// Verify consumption state removed.
	marketplaceDir := filepath.Join(pluginsDir, "marketplace", "role-item")
	if _, err := os.Stat(marketplaceDir); !os.IsNotExist(err) {
		t.Fatalf("expected marketplace consumption dir to be removed after uninstall, got err=%v", err)
	}
}

func TestMarketplaceHandler_UninstallReturns404ForMissingItem(t *testing.T) {
	rootDir := t.TempDir()
	h := newMarketplaceHandlerForTest(t, nil, filepath.Join(rootDir, "plugins"), filepath.Join(rootDir, "roles"), "")
	e := echo.New()

	body, _ := json.Marshal(map[string]string{"item_id": "nonexistent", "item_type": "plugin"})
	req := httptest.NewRequest(http.MethodPost, "/marketplace/uninstall", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	if err := h.Uninstall(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestMarketplaceHandler_SideloadInstallsRoleFromZip(t *testing.T) {
	rootDir := t.TempDir()
	pluginsDir := filepath.Join(rootDir, "plugins")
	rolesDir := filepath.Join(rootDir, "roles")

	h := newMarketplaceHandlerForTest(t, nil, pluginsDir, rolesDir, "")
	e := echo.New()

	roleZip := buildZipArchive(t, map[string]string{
		"role.yaml": `apiVersion: agentforge/v1
kind: Role
metadata:
  id: sideload-test
  name: Sideload Test
  version: 1.0.0
  description: Sideloaded role
  author: Test
identity:
  role: Sideload Test
  goal: Testing
  backstory: Sideloaded
capabilities:
  languages: [Go]
  frameworks: []
knowledge:
  repositories: []
  documents: []
  patterns: []
security:
  allowed_paths: []
  denied_paths: []
  max_budget_usd: 5
  require_review: true
`,
	})

	// Build multipart body.
	var buf bytes.Buffer
	writer := newMultipartWriter(&buf)
	writer.WriteField("type", "role")
	part, _ := writer.CreateFormFile("file", "sideload-test.zip")
	_, _ = part.Write(roleZip)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/marketplace/sideload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	if err := h.Sideload(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Sideload() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("sideload status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}

	var payload model.MarketplaceInstallResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode sideload payload: %v", err)
	}
	if !payload.OK {
		t.Fatalf("expected ok=true, got %v", payload)
	}
	if payload.Item.ItemType != model.MarketplaceItemTypeRole {
		t.Fatalf("itemType = %q, want role", payload.Item.ItemType)
	}
	if !payload.Item.Installed {
		t.Fatal("expected sideloaded role to report installed=true")
	}

	// Verify role directory exists.
	if _, err := rolepkg.NewFileStore(rolesDir).Get("sideload-test"); err != nil {
		t.Fatalf("expected sideloaded role to be discoverable, got %v", err)
	}
}

func TestMarketplaceHandler_SideloadInstallsSkillFromZip(t *testing.T) {
	rootDir := t.TempDir()
	pluginsDir := filepath.Join(rootDir, "plugins")
	rolesDir := filepath.Join(rootDir, "roles")

	h := newMarketplaceHandlerForTest(t, nil, pluginsDir, rolesDir, "")
	e := echo.New()

	skillZip := buildZipArchive(t, map[string]string{
		"SKILL.md": `---
name: sideload-skill
description: Sideloaded skill
---

# Sideloaded Skill
`,
	})

	var buf bytes.Buffer
	writer := newMultipartWriter(&buf)
	writer.WriteField("type", "skill")
	part, _ := writer.CreateFormFile("file", "sideload-skill.zip")
	_, _ = part.Write(skillZip)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/marketplace/sideload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	if err := h.Sideload(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Sideload() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("sideload skill status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}

	var payload model.MarketplaceInstallResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode sideload payload: %v", err)
	}
	if !payload.OK {
		t.Fatalf("expected ok=true, got %v", payload)
	}
	if payload.Item.ItemType != model.MarketplaceItemTypeSkill {
		t.Fatalf("itemType = %q, want skill", payload.Item.ItemType)
	}
	if !payload.Item.Installed {
		t.Fatal("expected sideloaded skill to report installed=true")
	}
}

func TestMarketplaceHandler_UpdatesEndpointReturnsEmptyWhenNoMarketplaceURL(t *testing.T) {
	rootDir := t.TempDir()
	h := newMarketplaceHandlerForTest(t, nil, filepath.Join(rootDir, "plugins"), filepath.Join(rootDir, "roles"), "")
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/marketplace/updates", nil)
	rec := httptest.NewRecorder()
	if err := h.Updates(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Updates() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload struct {
		Items []handler.MarketplaceUpdateInfo `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode updates payload: %v", err)
	}
	if len(payload.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(payload.Items))
	}
}

func newMultipartWriter(buf *bytes.Buffer) *multipartWriter {
	w := multipart.NewWriter(buf)
	return &multipartWriter{Writer: w}
}

type multipartWriter struct {
	*multipart.Writer
}

func (w *multipartWriter) WriteField(name, value string) {
	_ = w.Writer.WriteField(name, value)
}

func (w *multipartWriter) CreateFormFile(fieldname, filename string) (io.Writer, error) {
	return w.Writer.CreateFormFile(fieldname, filename)
}

func buildZipArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}
