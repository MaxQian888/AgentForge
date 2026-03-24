// Package bridge provides an HTTP client for the TypeScript Agent SDK bridge.
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
)

// ExecuteRequest is sent to the bridge to start an agent session.
type ExecuteRequest struct {
	TaskID         string      `json:"task_id"`
	SessionID      string      `json:"session_id"`
	MemberID       string      `json:"member_id,omitempty"`
	Runtime        string      `json:"runtime,omitempty"`
	Provider       string      `json:"provider,omitempty"`
	Model          string      `json:"model,omitempty"`
	Prompt         string      `json:"prompt"`
	WorktreePath   string      `json:"worktree_path"`
	BranchName     string      `json:"branch_name"`
	SystemPrompt   string      `json:"system_prompt,omitempty"`
	MaxTurns       int         `json:"max_turns,omitempty"`
	BudgetUSD      float64     `json:"budget_usd"`
	AllowedTools   []string    `json:"allowed_tools,omitempty"`
	PermissionMode string      `json:"permission_mode,omitempty"`
	RoleConfig     *RoleConfig `json:"role_config,omitempty"`
}

type RoleConfig struct {
	RoleID         string   `json:"role_id"`
	Name           string   `json:"name"`
	Role           string   `json:"role"`
	Goal           string   `json:"goal"`
	Backstory      string   `json:"backstory"`
	SystemPrompt   string   `json:"system_prompt"`
	AllowedTools   []string `json:"allowed_tools"`
	MaxBudgetUsd   float64  `json:"max_budget_usd"`
	MaxTurns       int      `json:"max_turns"`
	PermissionMode string   `json:"permission_mode"`
}

// ExecuteResponse is returned after an agent is started.
type ExecuteResponse struct {
	SessionID string `json:"session_id"`
}

type PauseResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}

type ResumeResponse struct {
	SessionID string `json:"session_id"`
	Resumed   bool   `json:"resumed"`
}

// StatusResponse holds agent run status from the bridge.
type StatusResponse struct {
	TaskID         string  `json:"task_id"`
	State          string  `json:"state"`
	TurnNumber     int     `json:"turn_number"`
	LastTool       string  `json:"last_tool"`
	LastActivityMS int64   `json:"last_activity_ms"`
	SpentUSD       float64 `json:"spent_usd"`
	Runtime        string  `json:"runtime"`
	Provider       string  `json:"provider"`
	Model          string  `json:"model"`
}

type RuntimeCatalogResponse struct {
	DefaultRuntime string                   `json:"default_runtime"`
	Runtimes       []RuntimeCatalogEntryDTO `json:"runtimes"`
}

type RuntimeCatalogEntryDTO struct {
	Key                 string                 `json:"key"`
	Label               string                 `json:"label"`
	DefaultProvider     string                 `json:"default_provider"`
	CompatibleProviders []string               `json:"compatible_providers"`
	DefaultModel        string                 `json:"default_model"`
	Available           bool                   `json:"available"`
	Diagnostics         []RuntimeDiagnosticDTO `json:"diagnostics"`
}

type RuntimeDiagnosticDTO struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
}

type DecomposeRequest struct {
	TaskID      string `json:"task_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
}

type DecomposeSubtask struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	Priority      string `json:"priority"`
	ExecutionMode string `json:"executionMode"`
}

type DecomposeResponse struct {
	Summary  string             `json:"summary"`
	Subtasks []DecomposeSubtask `json:"subtasks"`
}

type ReviewRequest struct {
	ReviewID    string   `json:"review_id"`
	TaskID      string   `json:"task_id"`
	PRURL       string   `json:"pr_url"`
	PRNumber    int      `json:"pr_number"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Diff        string   `json:"diff"`
	Dimensions  []string `json:"dimensions"`
}

type ReviewResponse struct {
	RiskLevel      string                `json:"risk_level"`
	Findings       []model.ReviewFinding `json:"findings"`
	Summary        string                `json:"summary"`
	Recommendation string                `json:"recommendation"`
	CostUSD        float64               `json:"cost_usd"`
}

// Client is an HTTP client for the TypeScript bridge service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new bridge client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Execute starts an agent session via the bridge.
func (c *Client) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal execute request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bridge/execute", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bridge execute: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bridge execute returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result ExecuteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode execute response: %w", err)
	}
	return &result, nil
}

// GetStatus queries the bridge for agent run status.
func (c *Client) GetStatus(ctx context.Context, taskID string) (*StatusResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/bridge/status/"+taskID, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bridge get status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bridge status returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode status response: %w", err)
	}
	return &result, nil
}

func (c *Client) GetRuntimeCatalog(ctx context.Context) (*RuntimeCatalogResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/bridge/runtimes", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bridge get runtime catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bridge runtime catalog returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result RuntimeCatalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode runtime catalog response: %w", err)
	}
	return &result, nil
}

// Cancel sends a cancel request to the bridge.
func (c *Client) Cancel(ctx context.Context, taskID, reason string) error {
	payload, _ := json.Marshal(map[string]string{"task_id": taskID, "reason": reason})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bridge/cancel", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("bridge cancel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bridge cancel returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// Pause requests the bridge to pause an active runtime while preserving resumable state.
func (c *Client) Pause(ctx context.Context, taskID, reason string) (*PauseResponse, error) {
	payload, _ := json.Marshal(map[string]string{"task_id": taskID, "reason": reason})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bridge/pause", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bridge pause: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bridge pause returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result PauseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode pause response: %w", err)
	}
	return &result, nil
}

// Resume requests the bridge to resume a runtime from a persisted snapshot.
func (c *Client) Resume(ctx context.Context, req ExecuteRequest) (*ResumeResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal resume request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bridge/resume", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bridge resume: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bridge resume returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result ResumeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode resume response: %w", err)
	}
	return &result, nil
}

// Health checks if the bridge is reachable.
func (c *Client) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/bridge/health", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("bridge health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bridge unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

// DecomposeTask requests a lightweight task decomposition from the bridge.
func (c *Client) DecomposeTask(ctx context.Context, req DecomposeRequest) (*DecomposeResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal decompose request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bridge/decompose", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bridge decompose: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bridge decompose returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result DecomposeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode decompose response: %w", err)
	}
	return &result, nil
}

// ClassifyIntentRequest is sent to the bridge for NLU intent classification.
type ClassifyIntentRequest struct {
	Text      string `json:"text"`
	UserID    string `json:"user_id"`
	ProjectID string `json:"project_id"`
}

// ClassifyIntentResponse is the NLU result from the bridge.
type ClassifyIntentResponse struct {
	Intent     string  `json:"intent"`
	Command    string  `json:"command"`
	Args       string  `json:"args"`
	Confidence float64 `json:"confidence"`
	Reply      string  `json:"reply,omitempty"`
}

// ClassifyIntent sends a natural language text to the bridge for intent classification.
func (c *Client) ClassifyIntent(ctx context.Context, req ClassifyIntentRequest) (*ClassifyIntentResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal classify intent request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bridge/classify-intent", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bridge classify intent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bridge classify intent returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result ClassifyIntentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode classify intent response: %w", err)
	}
	return &result, nil
}

// Review executes a Layer 2 deep review via the bridge.
func (c *Client) Review(ctx context.Context, req ReviewRequest) (*ReviewResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal review request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bridge/review", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bridge review: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bridge review returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result ReviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode review response: %w", err)
	}
	return &result, nil
}

func (c *Client) RegisterToolPlugin(ctx context.Context, manifest model.PluginManifest) (*model.PluginRuntimeStatus, error) {
	payload, err := json.Marshal(map[string]any{"manifest": manifest})
	if err != nil {
		return nil, fmt.Errorf("marshal tool plugin manifest: %w", err)
	}

	record, err := c.doPluginRequest(ctx, http.MethodPost, "/bridge/plugins/register", payload)
	if err != nil {
		return nil, err
	}
	return pluginRuntimeStatusFromRecord(record), nil
}

func (c *Client) ActivateToolPlugin(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	record, err := c.doPluginRequest(ctx, http.MethodPost, "/bridge/plugins/"+pluginID+"/activate", nil)
	if err != nil {
		return nil, err
	}
	return pluginRuntimeStatusFromRecord(record), nil
}

func (c *Client) CheckToolPluginHealth(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	record, err := c.doPluginRequest(ctx, http.MethodGet, "/bridge/plugins/"+pluginID+"/health", nil)
	if err != nil {
		return nil, err
	}
	return pluginRuntimeStatusFromRecord(record), nil
}

func (c *Client) RestartToolPlugin(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	record, err := c.doPluginRequest(ctx, http.MethodPost, "/bridge/plugins/"+pluginID+"/restart", nil)
	if err != nil {
		return nil, err
	}
	return pluginRuntimeStatusFromRecord(record), nil
}

type pluginRecordResponse struct {
	Metadata struct {
		ID string `json:"id"`
	} `json:"metadata"`
	LifecycleState     model.PluginLifecycleState   `json:"lifecycle_state"`
	RuntimeHost        model.PluginRuntimeHost      `json:"runtime_host"`
	LastHealthAt       *time.Time                   `json:"last_health_at,omitempty"`
	LastError          string                       `json:"last_error,omitempty"`
	RestartCount       int                          `json:"restart_count"`
	ResolvedSourcePath string                       `json:"resolved_source_path,omitempty"`
	RuntimeMetadata    *model.PluginRuntimeMetadata `json:"runtime_metadata,omitempty"`
}

func (c *Client) doPluginRequest(ctx context.Context, method, path string, payload []byte) (*pluginRecordResponse, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create plugin request: %w", err)
	}
	if payload != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("plugin request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("plugin request %s %s returned %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	var result pluginRecordResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode plugin response: %w", err)
	}
	return &result, nil
}

func pluginRuntimeStatusFromRecord(record *pluginRecordResponse) *model.PluginRuntimeStatus {
	if record == nil {
		return nil
	}

	return &model.PluginRuntimeStatus{
		PluginID:           record.Metadata.ID,
		Host:               record.RuntimeHost,
		LifecycleState:     record.LifecycleState,
		LastHealthAt:       record.LastHealthAt,
		LastError:          record.LastError,
		RestartCount:       record.RestartCount,
		ResolvedSourcePath: record.ResolvedSourcePath,
		RuntimeMetadata:    record.RuntimeMetadata,
	}
}
