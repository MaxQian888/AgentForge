// Package bridge provides an HTTP client for the TypeScript Agent SDK bridge.
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
	log "github.com/sirupsen/logrus"
)

const (
	bridgeExecutePath             = "/bridge/execute"
	bridgeStatusPathTemplate      = "/bridge/status/:id"
	bridgeCancelPath              = "/bridge/cancel"
	bridgePausePath               = "/bridge/pause"
	bridgeResumePath              = "/bridge/resume"
	bridgeHealthPath              = "/bridge/health"
	bridgePoolPath                = "/bridge/pool"
	bridgeRuntimeCatalogPath      = "/bridge/runtimes"
	bridgeDecomposePath           = "/bridge/decompose"
	bridgeGeneratePath            = "/bridge/generate"
	bridgeClassifyIntentPath      = "/bridge/classify-intent"
	bridgeReviewPath              = "/bridge/review"
	bridgePluginRegisterPath      = "/bridge/plugins/register"
	bridgePluginRefreshMCPPattern = "/bridge/plugins/%s/mcp/refresh"
)

// ThinkingConfig controls extended thinking for the agent runtime.
type ThinkingConfig struct {
	Enabled      bool `json:"enabled"`
	BudgetTokens int  `json:"budget_tokens,omitempty"`
}

// StructuredOutputSchema defines a JSON schema for structured agent output.
type StructuredOutputSchema struct {
	Type   string         `json:"type"`
	Schema map[string]any `json:"schema"`
}

// HookDefinition declares a single hook trigger.
type HookDefinition struct {
	Hook    string         `json:"hook"`
	Matcher map[string]any `json:"matcher,omitempty"`
}

// HooksConfig groups hook triggers and their callback.
type HooksConfig struct {
	Hooks       []HookDefinition `json:"hooks"`
	CallbackURL string           `json:"callback_url,omitempty"`
	TimeoutMS   int              `json:"timeout_ms,omitempty"`
}

// Attachment represents an image or file attachment for an agent run.
type Attachment struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	MimeType string `json:"mime_type,omitempty"`
}

// AgentDefinition defines a sub-agent that can be spawned during execution.
type AgentDefinition struct {
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Tools       []string `json:"tools,omitempty"`
	Model       string   `json:"model,omitempty"`
}

// ExecuteRequest is sent to the bridge to start an agent session.
type ExecuteRequest struct {
	TaskID                 string                     `json:"task_id"`
	SessionID              string                     `json:"session_id"`
	MemberID               string                     `json:"member_id,omitempty"`
	TeamID                 string                     `json:"team_id,omitempty"`
	TeamRole               string                     `json:"team_role,omitempty"`
	Runtime                string                     `json:"runtime,omitempty"`
	Provider               string                     `json:"provider,omitempty"`
	Model                  string                     `json:"model,omitempty"`
	Prompt                 string                     `json:"prompt"`
	WorktreePath           string                     `json:"worktree_path"`
	BranchName             string                     `json:"branch_name"`
	SystemPrompt           string                     `json:"system_prompt,omitempty"`
	MaxTurns               int                        `json:"max_turns,omitempty"`
	BudgetUSD              float64                    `json:"budget_usd"`
	WarnThreshold          float64                    `json:"warn_threshold,omitempty"`
	AllowedTools           []string                   `json:"allowed_tools,omitempty"`
	DisallowedTools        []string                   `json:"disallowed_tools,omitempty"`
	PermissionMode         string                     `json:"permission_mode,omitempty"`
	RoleConfig             *RoleConfig                `json:"role_config,omitempty"`
	ThinkingConfig         *ThinkingConfig            `json:"thinking_config,omitempty"`
	OutputSchema           *StructuredOutputSchema    `json:"output_schema,omitempty"`
	HooksConfig            *HooksConfig               `json:"hooks_config,omitempty"`
	HookCallbackURL        string                     `json:"hook_callback_url,omitempty"`
	HookTimeoutMS          int                        `json:"hook_timeout_ms,omitempty"`
	Attachments            []Attachment               `json:"attachments,omitempty"`
	FileCheckpointing      *bool                      `json:"file_checkpointing,omitempty"`
	Agents                 map[string]AgentDefinition `json:"agents,omitempty"`
	FallbackModel          string                     `json:"fallback_model,omitempty"`
	AdditionalDirectories  []string                   `json:"additional_directories,omitempty"`
	IncludePartialMessages *bool                      `json:"include_partial_messages,omitempty"`
	ToolPermissionCallback *bool                      `json:"tool_permission_callback,omitempty"`
	WebSearch              *bool                      `json:"web_search,omitempty"`
	Env                    map[string]string          `json:"env,omitempty"`
	TeamContext            string                     `json:"team_context,omitempty"`
}

type RoleConfig struct {
	RoleID             string                               `json:"role_id"`
	Name               string                               `json:"name"`
	Role               string                               `json:"role"`
	Goal               string                               `json:"goal"`
	Backstory          string                               `json:"backstory"`
	SystemPrompt       string                               `json:"system_prompt"`
	AllowedTools       []string                             `json:"allowed_tools"`
	Tools              []string                             `json:"tools,omitempty"`
	PluginBindings     []model.RoleToolPluginBinding        `json:"plugin_bindings,omitempty"`
	KnowledgeContext   string                               `json:"knowledge_context,omitempty"`
	OutputFilters      []string                             `json:"output_filters,omitempty"`
	MaxBudgetUsd       float64                              `json:"max_budget_usd"`
	MaxTurns           int                                  `json:"max_turns"`
	PermissionMode     string                               `json:"permission_mode"`
	BlockedTools       []string                             `json:"blocked_tools,omitempty"`
	FilePermissions    *RoleFilePerms                       `json:"file_permissions,omitempty"`
	NetworkPermissions *RoleNetworkPerms                    `json:"network_permissions,omitempty"`
	LoadedSkills       []model.RoleExecutionSkill           `json:"loaded_skills,omitempty"`
	AvailableSkills    []model.RoleExecutionSkill           `json:"available_skills,omitempty"`
	SkillDiagnostics   []model.RoleExecutionSkillDiagnostic `json:"skill_diagnostics,omitempty"`
}

// RoleFilePerms defines file path access restrictions for a role.
type RoleFilePerms struct {
	AllowedPatterns []string `json:"allowed_patterns,omitempty"`
	BlockedPatterns []string `json:"blocked_patterns,omitempty"`
}

// RoleNetworkPerms defines network access restrictions for a role.
type RoleNetworkPerms struct {
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	Blocked        bool     `json:"blocked,omitempty"`
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
	RoleID         string  `json:"role_id,omitempty"`
	TeamID         string  `json:"team_id,omitempty"`
	TeamRole       string  `json:"team_role,omitempty"`
}

type PoolSummaryResponse struct {
	Active          int   `json:"active"`
	Max             int   `json:"max"`
	WarmTotal       int   `json:"warm_total"`
	WarmAvailable   int   `json:"warm_available"`
	WarmReuseHits   int   `json:"warm_reuse_hits"`
	ColdStarts      int   `json:"cold_starts"`
	LastReconcileAt int64 `json:"last_reconcile_at"`
	Degraded        bool  `json:"degraded"`
}

type HealthResponse struct {
	Status       string `json:"status"`
	ActiveAgents int    `json:"active_agents"`
	MaxAgents    int    `json:"max_agents"`
	UptimeMS     int64  `json:"uptime_ms"`
}

type ToolDefinition struct {
	PluginID    string         `json:"plugin_id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

type ToolsListResponse struct {
	Tools []ToolDefinition `json:"tools"`
}

type RuntimeCatalogResponse struct {
	DefaultRuntime string                   `json:"default_runtime"`
	Runtimes       []RuntimeCatalogEntryDTO `json:"runtimes"`
}

type RuntimeCatalogEntryDTO struct {
	Key                     string                         `json:"key"`
	Label                   string                         `json:"label"`
	DefaultProvider         string                         `json:"default_provider"`
	CompatibleProviders     []string                       `json:"compatible_providers"`
	DefaultModel            string                         `json:"default_model"`
	ModelOptions            []string                       `json:"model_options,omitempty"`
	Available               bool                           `json:"available"`
	Diagnostics             []RuntimeDiagnosticDTO         `json:"diagnostics"`
	SupportedFeatures       []string                       `json:"supported_features,omitempty"`
	InteractionCapabilities RuntimeInteractionCapabilities `json:"interaction_capabilities,omitempty"`
	Agents                  []string                       `json:"agents,omitempty"`
	Skills                  []string                       `json:"skills,omitempty"`
	Providers               []RuntimeCatalogProviderDTO    `json:"providers,omitempty"`
	LaunchContract          *RuntimeLaunchContractDTO      `json:"launch_contract,omitempty"`
	Lifecycle               *RuntimeLifecycleDTO           `json:"lifecycle,omitempty"`
}

type RuntimeDiagnosticDTO struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
}

type RuntimeCapabilityDescriptor struct {
	State                 string   `json:"state"`
	ReasonCode            string   `json:"reason_code,omitempty"`
	Message               string   `json:"message,omitempty"`
	RequiresRequestFields []string `json:"requires_request_fields,omitempty"`
}

type RuntimeInteractionCapabilities map[string]map[string]RuntimeCapabilityDescriptor

type RuntimeCatalogProviderDTO struct {
	Provider     string   `json:"provider"`
	Connected    bool     `json:"connected"`
	DefaultModel string   `json:"default_model,omitempty"`
	ModelOptions []string `json:"model_options,omitempty"`
	AuthRequired bool     `json:"auth_required,omitempty"`
	AuthMethods  []string `json:"auth_methods,omitempty"`
}

type RuntimeLaunchContractDTO struct {
	PromptTransport        string   `json:"prompt_transport"`
	OutputMode             string   `json:"output_mode"`
	SupportedOutputModes   []string `json:"supported_output_modes,omitempty"`
	SupportedApprovalModes []string `json:"supported_approval_modes,omitempty"`
	AdditionalDirectories  bool     `json:"additional_directories"`
	EnvOverrides           bool     `json:"env_overrides"`
}

type RuntimeLifecycleDTO struct {
	Stage              string `json:"stage"`
	SunsetAt           string `json:"sunset_at,omitempty"`
	ReplacementRuntime string `json:"replacement_runtime,omitempty"`
	Message            string `json:"message,omitempty"`
}

type DecomposeRequest struct {
	TaskID      string `json:"task_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
	Context     any    `json:"context,omitempty"`
}

type GenerateRequest struct {
	Prompt       string  `json:"prompt"`
	SystemPrompt string  `json:"system_prompt,omitempty"`
	Provider     string  `json:"provider,omitempty"`
	Model        string  `json:"model,omitempty"`
	MaxTokens    int     `json:"max_tokens,omitempty"`
	Temperature  float64 `json:"temperature,omitempty"`
}

type GenerateUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type GenerateResponse struct {
	Text  string        `json:"text"`
	Usage GenerateUsage `json:"usage"`
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
	ReviewID      string                `json:"review_id"`
	TaskID        string                `json:"task_id"`
	PRURL         string                `json:"pr_url"`
	PRNumber      int                   `json:"pr_number"`
	Title         string                `json:"title"`
	Description   string                `json:"description"`
	Diff          string                `json:"diff"`
	Dimensions    []string              `json:"dimensions"`
	TriggerEvent  string                `json:"trigger_event,omitempty"`
	ChangedFiles  []string              `json:"changed_files,omitempty"`
	ReviewPlugins []ReviewPluginRequest `json:"review_plugins,omitempty"`
}

type ReviewPluginRequest struct {
	PluginID     string   `json:"plugin_id"`
	Name         string   `json:"name"`
	Entrypoint   string   `json:"entrypoint,omitempty"`
	SourceType   string   `json:"source_type,omitempty"`
	Transport    string   `json:"transport,omitempty"`
	Command      string   `json:"command,omitempty"`
	Args         []string `json:"args,omitempty"`
	URL          string   `json:"url,omitempty"`
	Events       []string `json:"events,omitempty"`
	FilePatterns []string `json:"file_patterns,omitempty"`
	OutputFormat string   `json:"output_format,omitempty"`
}

type ReviewExecutionResult struct {
	Dimension   string `json:"dimension"`
	SourceType  string `json:"source_type,omitempty"`
	PluginID    string `json:"plugin_id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Status      string `json:"status"`
	Summary     string `json:"summary"`
	Error       string `json:"error,omitempty"`
}

type ReviewResponse struct {
	RiskLevel        string                  `json:"risk_level"`
	Findings         []model.ReviewFinding   `json:"findings"`
	DimensionResults []ReviewExecutionResult `json:"dimension_results,omitempty"`
	Summary          string                  `json:"summary"`
	Recommendation   string                  `json:"recommendation"`
	CostUSD          float64                 `json:"cost_usd"`
}

// Client is an HTTP client for the TypeScript bridge service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func (c *Client) requestLogFields(method, path string) log.Fields {
	return log.Fields{
		"method":  method,
		"path":    path,
		"baseUrl": c.baseURL,
	}
}

func logBridgeRequestResult(fields log.Fields, start time.Time, status int, err error, successMessage, failureMessage string) {
	fields["durationMs"] = time.Since(start).Milliseconds()
	if status > 0 {
		fields["status"] = status
	}
	if err != nil {
		log.WithFields(fields).WithError(err).Warn(failureMessage)
		return
	}
	log.WithFields(fields).Info(successMessage)
}

func bridgeUpstreamError(path string, status int, respBody []byte) error {
	body := strings.TrimSpace(string(respBody))
	if body == "" {
		return fmt.Errorf("bridge upstream %s returned %d", path, status)
	}
	return fmt.Errorf("bridge upstream %s returned %d: %s", path, status, body)
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
	start := time.Now()
	fields := c.requestLogFields(http.MethodPost, bridgeExecutePath)
	fields["taskId"] = req.TaskID
	fields["sessionId"] = req.SessionID
	fields["runtime"] = req.Runtime
	fields["provider"] = req.Provider
	fields["model"] = req.Model

	body, err := json.Marshal(req)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge execute marshal failed")
		return nil, fmt.Errorf("marshal execute request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+bridgeExecutePath, bytes.NewReader(body))
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge execute request creation failed")
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge execute request failed")
		return nil, fmt.Errorf("bridge execute: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logBridgeRequestResult(fields, start, resp.StatusCode, fmt.Errorf("bridge execute returned non-OK status"), "", "bridge execute returned non-OK status")
		return nil, fmt.Errorf("bridge execute returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result ExecuteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logBridgeRequestResult(fields, start, resp.StatusCode, err, "", "bridge execute decode failed")
		return nil, fmt.Errorf("decode execute response: %w", err)
	}
	logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge execute completed", "")
	return &result, nil
}

// GetStatus queries the bridge for agent run status.
func (c *Client) GetStatus(ctx context.Context, taskID string) (*StatusResponse, error) {
	start := time.Now()
	fields := c.requestLogFields(http.MethodGet, bridgeStatusPathTemplate)
	fields["taskId"] = taskID

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/bridge/status/"+taskID, nil)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge status request creation failed")
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge status request failed")
		return nil, fmt.Errorf("bridge get status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logBridgeRequestResult(fields, start, resp.StatusCode, fmt.Errorf("bridge status returned non-OK status"), "", "bridge status returned non-OK status")
		return nil, bridgeUpstreamError(bridgeStatusPathTemplate, resp.StatusCode, respBody)
	}

	var result StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logBridgeRequestResult(fields, start, resp.StatusCode, err, "", "bridge status decode failed")
		return nil, fmt.Errorf("decode status response: %w", err)
	}
	fields["state"] = result.State
	fields["turnNumber"] = result.TurnNumber
	logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge status fetched", "")
	return &result, nil
}

func (c *Client) GetPoolSummary(ctx context.Context) (*PoolSummaryResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+bridgePoolPath, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bridge get pool summary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bridge pool summary returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result PoolSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode pool summary response: %w", err)
	}
	return &result, nil
}

func (c *Client) GetPool(ctx context.Context) (*PoolSummaryResponse, error) {
	return c.GetPoolSummary(ctx)
}

func (c *Client) GetRuntimeCatalog(ctx context.Context) (*RuntimeCatalogResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+bridgeRuntimeCatalogPath, nil)
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
		return nil, bridgeUpstreamError(bridgeRuntimeCatalogPath, resp.StatusCode, respBody)
	}

	var result RuntimeCatalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode runtime catalog response: %w", err)
	}
	return &result, nil
}

// Cancel sends a cancel request to the bridge.
func (c *Client) Cancel(ctx context.Context, taskID, reason string) error {
	start := time.Now()
	fields := c.requestLogFields(http.MethodPost, bridgeCancelPath)
	fields["taskId"] = taskID

	payload, _ := json.Marshal(map[string]string{"task_id": taskID, "reason": reason})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+bridgeCancelPath, bytes.NewReader(payload))
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge cancel request creation failed")
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge cancel request failed")
		return fmt.Errorf("bridge cancel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logBridgeRequestResult(fields, start, resp.StatusCode, fmt.Errorf("bridge cancel returned non-OK status"), "", "bridge cancel returned non-OK status")
		return fmt.Errorf("bridge cancel returned %d: %s", resp.StatusCode, string(respBody))
	}
	logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge cancel completed", "")
	return nil
}

// Pause requests the bridge to pause an active runtime while preserving resumable state.
func (c *Client) Pause(ctx context.Context, taskID, reason string) (*PauseResponse, error) {
	start := time.Now()
	fields := c.requestLogFields(http.MethodPost, bridgePausePath)
	fields["taskId"] = taskID

	payload, _ := json.Marshal(map[string]string{"task_id": taskID, "reason": reason})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+bridgePausePath, bytes.NewReader(payload))
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge pause request creation failed")
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge pause request failed")
		return nil, fmt.Errorf("bridge pause: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logBridgeRequestResult(fields, start, resp.StatusCode, fmt.Errorf("bridge pause returned non-OK status"), "", "bridge pause returned non-OK status")
		return nil, fmt.Errorf("bridge pause returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result PauseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logBridgeRequestResult(fields, start, resp.StatusCode, err, "", "bridge pause decode failed")
		return nil, fmt.Errorf("decode pause response: %w", err)
	}
	fields["sessionId"] = result.SessionID
	fields["state"] = result.Status
	logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge pause completed", "")
	return &result, nil
}

// Resume requests the bridge to resume a runtime from a persisted snapshot.
func (c *Client) Resume(ctx context.Context, req ExecuteRequest) (*ResumeResponse, error) {
	start := time.Now()
	fields := c.requestLogFields(http.MethodPost, bridgeResumePath)
	fields["taskId"] = req.TaskID
	fields["sessionId"] = req.SessionID
	fields["runtime"] = req.Runtime
	fields["provider"] = req.Provider
	fields["model"] = req.Model

	body, err := json.Marshal(req)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge resume marshal failed")
		return nil, fmt.Errorf("marshal resume request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+bridgeResumePath, bytes.NewReader(body))
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge resume request creation failed")
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge resume request failed")
		return nil, fmt.Errorf("bridge resume: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logBridgeRequestResult(fields, start, resp.StatusCode, fmt.Errorf("bridge resume returned non-OK status"), "", "bridge resume returned non-OK status")
		return nil, bridgeUpstreamError(bridgeResumePath, resp.StatusCode, respBody)
	}

	var result ResumeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logBridgeRequestResult(fields, start, resp.StatusCode, err, "", "bridge resume decode failed")
		return nil, fmt.Errorf("decode resume response: %w", err)
	}
	fields["resumed"] = result.Resumed
	logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge resume completed", "")
	return &result, nil
}

// Health checks if the bridge is reachable.
func (c *Client) Health(ctx context.Context) error {
	start := time.Now()
	fields := c.requestLogFields(http.MethodGet, bridgeHealthPath)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+bridgeHealthPath, nil)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge health request creation failed")
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge health request failed")
		return fmt.Errorf("bridge health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logBridgeRequestResult(fields, start, resp.StatusCode, fmt.Errorf("bridge health returned non-OK status"), "", "bridge health returned non-OK status")
		return fmt.Errorf("bridge unhealthy: status %d", resp.StatusCode)
	}
	logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge health check passed", "")
	return nil
}

func (c *Client) GetHealth(ctx context.Context) (*HealthResponse, error) {
	var result HealthResponse
	if err := c.doJSONRequest(ctx, http.MethodGet, bridgeHealthPath, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DecomposeTask requests a lightweight task decomposition from the bridge.
func (c *Client) DecomposeTask(ctx context.Context, req DecomposeRequest) (*DecomposeResponse, error) {
	start := time.Now()
	fields := c.requestLogFields(http.MethodPost, bridgeDecomposePath)
	fields["taskId"] = req.TaskID
	fields["provider"] = req.Provider
	fields["model"] = req.Model

	body, err := json.Marshal(req)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge decompose marshal failed")
		return nil, fmt.Errorf("marshal decompose request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+bridgeDecomposePath, bytes.NewReader(body))
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge decompose request creation failed")
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge decompose request failed")
		return nil, fmt.Errorf("bridge decompose: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logBridgeRequestResult(fields, start, resp.StatusCode, fmt.Errorf("bridge decompose returned non-OK status"), "", "bridge decompose returned non-OK status")
		return nil, bridgeUpstreamError(bridgeDecomposePath, resp.StatusCode, respBody)
	}

	var result DecomposeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logBridgeRequestResult(fields, start, resp.StatusCode, err, "", "bridge decompose decode failed")
		return nil, fmt.Errorf("decode decompose response: %w", err)
	}
	fields["subtaskCount"] = len(result.Subtasks)
	logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge decompose completed", "")
	return &result, nil
}

func (c *Client) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	start := time.Now()
	fields := c.requestLogFields(http.MethodPost, bridgeGeneratePath)
	fields["provider"] = req.Provider
	fields["model"] = req.Model

	body, err := json.Marshal(req)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge generate marshal failed")
		return nil, fmt.Errorf("marshal generate request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+bridgeGeneratePath, bytes.NewReader(body))
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge generate request creation failed")
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge generate request failed")
		return nil, fmt.Errorf("bridge generate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logBridgeRequestResult(fields, start, resp.StatusCode, fmt.Errorf("bridge generate returned non-OK status"), "", "bridge generate returned non-OK status")
		return nil, bridgeUpstreamError(bridgeGeneratePath, resp.StatusCode, respBody)
	}

	var result GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logBridgeRequestResult(fields, start, resp.StatusCode, err, "", "bridge generate decode failed")
		return nil, fmt.Errorf("decode generate response: %w", err)
	}
	fields["inputTokens"] = result.Usage.InputTokens
	fields["outputTokens"] = result.Usage.OutputTokens
	logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge generate completed", "")
	return &result, nil
}

// --- Conversation management ---

// ForkRequest forks a conversation at a specific message.
type ForkRequest struct {
	TaskID    string `json:"task_id"`
	MessageID string `json:"message_id,omitempty"`
}

// ForkResponse is returned after a fork operation.
type ForkResponse struct {
	NewTaskID  string `json:"new_task_id"`
	Continuity any    `json:"continuity,omitempty"`
}

// Fork forks a conversation at a given message checkpoint.
func (c *Client) Fork(ctx context.Context, req ForkRequest) (*ForkResponse, error) {
	var result ForkResponse
	if err := c.doJSONRequest(ctx, http.MethodPost, "/bridge/fork", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RollbackRequest rolls back an agent runtime.
type RollbackRequest struct {
	TaskID       string `json:"task_id"`
	CheckpointID string `json:"checkpoint_id,omitempty"`
	Turns        int    `json:"turns,omitempty"`
}

// Rollback rolls back a runtime to a checkpoint or by N turns.
func (c *Client) Rollback(ctx context.Context, req RollbackRequest) error {
	return c.doJSONRequest(ctx, http.MethodPost, "/bridge/rollback", req, nil)
}

// RevertRequest reverts a specific message.
type RevertRequest struct {
	TaskID    string `json:"task_id"`
	MessageID string `json:"message_id"`
}

// Revert reverts a specific message in the conversation.
func (c *Client) Revert(ctx context.Context, req RevertRequest) error {
	return c.doJSONRequest(ctx, http.MethodPost, "/bridge/revert", req, nil)
}

// UnrevertRequest undoes a previous revert.
type UnrevertRequest struct {
	TaskID string `json:"task_id"`
}

// Unrevert undoes a previous message revert.
func (c *Client) Unrevert(ctx context.Context, req UnrevertRequest) error {
	return c.doJSONRequest(ctx, http.MethodPost, "/bridge/unrevert", req, nil)
}

// DiffResponse holds the diff output from an agent runtime.
type DiffResponse struct {
	Diff string `json:"diff,omitempty"`
}

// GetDiff retrieves the diff of changes made by an agent runtime.
func (c *Client) GetDiff(ctx context.Context, taskID string) (*DiffResponse, error) {
	var result DiffResponse
	if err := c.doJSONRequest(ctx, http.MethodGet, "/bridge/diff/"+taskID, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// MessagesResponse holds conversation messages from an agent runtime.
type MessagesResponse struct {
	Messages []any `json:"messages,omitempty"`
}

// GetMessages retrieves the conversation history for a task.
func (c *Client) GetMessages(ctx context.Context, taskID string) (*MessagesResponse, error) {
	var result MessagesResponse
	if err := c.doJSONRequest(ctx, http.MethodGet, "/bridge/messages/"+taskID, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CommandRequest sends a command to a running agent runtime.
type CommandRequest struct {
	TaskID    string `json:"task_id"`
	Command   string `json:"command"`
	Arguments string `json:"arguments,omitempty"`
}

// ExecuteCommand sends a runtime command to a running agent.
func (c *Client) ExecuteCommand(ctx context.Context, req CommandRequest) error {
	return c.doJSONRequest(ctx, http.MethodPost, "/bridge/command", req, nil)
}

type ShellRequest struct {
	TaskID  string `json:"task_id"`
	Command string `json:"command"`
	Agent   string `json:"agent,omitempty"`
	Model   string `json:"model,omitempty"`
}

type ShellResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (c *Client) ExecuteShell(ctx context.Context, req ShellRequest) (*ShellResponse, error) {
	var result ShellResponse
	if err := c.doJSONRequest(ctx, http.MethodPost, "/bridge/shell", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Runtime control ---

// Interrupt interrupts a running agent without cancelling.
func (c *Client) Interrupt(ctx context.Context, taskID string) error {
	return c.doJSONRequest(ctx, http.MethodPost, "/bridge/interrupt", map[string]string{"task_id": taskID}, nil)
}

type ThinkingBudgetRequest struct {
	TaskID            string `json:"task_id"`
	MaxThinkingTokens *int   `json:"max_thinking_tokens"`
}

func (c *Client) SetThinkingBudget(ctx context.Context, req ThinkingBudgetRequest) error {
	return c.doJSONRequest(ctx, http.MethodPost, "/bridge/thinking", req, nil)
}

// ModelSwitchRequest switches the model of a running agent.
type ModelSwitchRequest struct {
	TaskID string `json:"task_id"`
	Model  string `json:"model"`
}

// SwitchModel switches the model for a running agent session.
func (c *Client) SwitchModel(ctx context.Context, req ModelSwitchRequest) error {
	return c.doJSONRequest(ctx, http.MethodPost, "/bridge/model", req, nil)
}

// PermissionResponsePayload is submitted to resolve a pending permission request.
type PermissionResponsePayload struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

// PermissionResponse submits a permission decision to the bridge.
func (c *Client) PermissionResponse(ctx context.Context, requestID string, payload PermissionResponsePayload) error {
	return c.doJSONRequest(ctx, http.MethodPost, "/bridge/permission-response/"+requestID, payload, nil)
}

func (c *Client) GetMCPStatus(ctx context.Context, taskID string) ([]map[string]any, error) {
	var result []map[string]any
	if err := c.doJSONRequest(ctx, http.MethodGet, "/bridge/mcp-status/"+taskID, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) StartOpenCodeProviderAuth(ctx context.Context, provider string, payload map[string]any) (map[string]any, error) {
	var result map[string]any
	if err := c.doJSONRequest(ctx, http.MethodPost, "/bridge/opencode/provider-auth/"+provider+"/start", payload, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) CompleteOpenCodeProviderAuth(ctx context.Context, requestID string, payload map[string]any) (map[string]any, error) {
	var result map[string]any
	if err := c.doJSONRequest(ctx, http.MethodPost, "/bridge/opencode/provider-auth/"+requestID+"/complete", payload, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetActive returns the list of all active agent runtimes.
func (c *Client) GetActive(ctx context.Context) ([]StatusResponse, error) {
	var result []StatusResponse
	if err := c.doJSONRequest(ctx, http.MethodGet, "/bridge/active", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// --- Plugin lifecycle ---

// PluginListResponse holds the list of plugins.
type PluginListResponse struct {
	Plugins []any `json:"plugins"`
}

// ListPlugins returns all registered plugins.
func (c *Client) ListPlugins(ctx context.Context) (*PluginListResponse, error) {
	var result PluginListResponse
	if err := c.doJSONRequest(ctx, http.MethodGet, "/bridge/plugins", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// EnablePlugin enables a registered plugin.
func (c *Client) EnablePlugin(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	record, err := c.doPluginRequest(ctx, http.MethodPost, "/bridge/plugins/"+pluginID+"/enable", nil)
	if err != nil {
		return nil, err
	}
	return pluginRuntimeStatusFromRecord(record), nil
}

// DisablePlugin disables a registered plugin.
func (c *Client) DisablePlugin(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	record, err := c.doPluginRequest(ctx, http.MethodPost, "/bridge/plugins/"+pluginID+"/disable", nil)
	if err != nil {
		return nil, err
	}
	return pluginRuntimeStatusFromRecord(record), nil
}

// ClassifyIntentRequest is sent to the bridge for NLU intent classification.
type ClassifyIntentRequest struct {
	Text       string   `json:"text"`
	UserID     string   `json:"user_id"`
	ProjectID  string   `json:"project_id"`
	Candidates []string `json:"candidates,omitempty"`
	Context    any      `json:"context,omitempty"`
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
	start := time.Now()
	fields := c.requestLogFields(http.MethodPost, bridgeClassifyIntentPath)
	fields["userId"] = req.UserID
	fields["projectId"] = req.ProjectID

	body, err := json.Marshal(req)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge classify intent marshal failed")
		return nil, fmt.Errorf("marshal classify intent request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+bridgeClassifyIntentPath, bytes.NewReader(body))
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge classify intent request creation failed")
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge classify intent request failed")
		return nil, fmt.Errorf("bridge classify intent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logBridgeRequestResult(fields, start, resp.StatusCode, fmt.Errorf("bridge classify intent returned non-OK status"), "", "bridge classify intent returned non-OK status")
		return nil, bridgeUpstreamError(bridgeClassifyIntentPath, resp.StatusCode, respBody)
	}

	var result ClassifyIntentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logBridgeRequestResult(fields, start, resp.StatusCode, err, "", "bridge classify intent decode failed")
		return nil, fmt.Errorf("decode classify intent response: %w", err)
	}
	fields["intent"] = result.Intent
	fields["command"] = result.Command
	fields["confidence"] = result.Confidence
	logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge classify intent completed", "")
	return &result, nil
}

// Review executes a Layer 2 deep review via the bridge.
func (c *Client) Review(ctx context.Context, req ReviewRequest) (*ReviewResponse, error) {
	start := time.Now()
	fields := c.requestLogFields(http.MethodPost, bridgeReviewPath)
	fields["reviewId"] = req.ReviewID
	fields["taskId"] = req.TaskID
	fields["dimensionCount"] = len(req.Dimensions)
	fields["pluginCount"] = len(req.ReviewPlugins)

	body, err := json.Marshal(req)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge review marshal failed")
		return nil, fmt.Errorf("marshal review request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+bridgeReviewPath, bytes.NewReader(body))
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge review request creation failed")
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge review request failed")
		return nil, fmt.Errorf("bridge review: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logBridgeRequestResult(fields, start, resp.StatusCode, fmt.Errorf("bridge review returned non-OK status"), "", "bridge review returned non-OK status")
		return nil, fmt.Errorf("bridge review returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result ReviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logBridgeRequestResult(fields, start, resp.StatusCode, err, "", "bridge review decode failed")
		return nil, fmt.Errorf("decode review response: %w", err)
	}
	fields["riskLevel"] = result.RiskLevel
	fields["findingCount"] = len(result.Findings)
	logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge review completed", "")
	return &result, nil
}

func (c *Client) RegisterToolPlugin(ctx context.Context, manifest model.PluginManifest) (*model.PluginRuntimeStatus, error) {
	payload, err := json.Marshal(map[string]any{"manifest": manifest})
	if err != nil {
		return nil, fmt.Errorf("marshal tool plugin manifest: %w", err)
	}

	record, err := c.doPluginRequest(ctx, http.MethodPost, bridgePluginRegisterPath, payload)
	if err != nil {
		return nil, err
	}
	return pluginRuntimeStatusFromRecord(record), nil
}

func (c *Client) ListTools(ctx context.Context) (*ToolsListResponse, error) {
	var result ToolsListResponse
	if err := c.doJSONRequest(ctx, http.MethodGet, "/bridge/tools", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) InstallTool(ctx context.Context, manifest model.PluginManifest) (*model.PluginRecord, error) {
	var result model.PluginRecord
	if err := c.doJSONRequest(ctx, http.MethodPost, "/bridge/tools/install", map[string]any{"manifest": manifest}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UninstallTool(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	var result model.PluginRecord
	if err := c.doJSONRequest(ctx, http.MethodPost, "/bridge/tools/uninstall", map[string]any{"plugin_id": pluginID}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) RestartTool(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	var result model.PluginRecord
	if err := c.doJSONRequest(ctx, http.MethodPost, "/bridge/tools/"+pluginID+"/restart", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ActivateToolPlugin(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	record, err := c.doPluginRequest(ctx, http.MethodPost, "/bridge/plugins/"+pluginID+"/activate", nil)
	if err != nil {
		return nil, err
	}
	return pluginRuntimeStatusFromRecord(record), nil
}

// DisableToolPlugin asks the TS bridge to disconnect the MCP transport
// for the given plugin and release the child process. The Go control
// plane calls this during Disable/Deactivate/Uninstall so the child
// never outlives its registry record.
func (c *Client) DisableToolPlugin(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	record, err := c.doPluginRequest(ctx, http.MethodPost, "/bridge/plugins/"+pluginID+"/disable", nil)
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

func (c *Client) RefreshToolPluginMCPSurface(ctx context.Context, pluginID string) (*model.PluginMCPRefreshResult, error) {
	record, err := c.doPluginRequest(ctx, http.MethodPost, fmt.Sprintf(bridgePluginRefreshMCPPattern, pluginID), nil)
	if err != nil {
		return nil, err
	}
	return pluginMCPSurfaceFromRecord(record), nil
}

func (c *Client) InvokeToolPluginMCPTool(ctx context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	if args == nil {
		args = map[string]any{}
	}
	var result model.PluginMCPToolCallResult
	if err := c.doJSONRequest(ctx, http.MethodPost, "/bridge/plugins/"+pluginID+"/mcp/tools/call", map[string]any{
		"tool_name": toolName,
		"arguments": args,
	}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ReadToolPluginMCPResource(ctx context.Context, pluginID, uri string) (*model.PluginMCPResourceReadResult, error) {
	var result model.PluginMCPResourceReadResult
	if err := c.doJSONRequest(ctx, http.MethodPost, "/bridge/plugins/"+pluginID+"/mcp/resources/read", map[string]any{
		"uri": uri,
	}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetToolPluginMCPPrompt(ctx context.Context, pluginID, name string, args map[string]string) (*model.PluginMCPPromptResult, error) {
	payload := map[string]any{"name": name}
	if args != nil {
		payload["arguments"] = args
	}
	var result model.PluginMCPPromptResult
	if err := c.doJSONRequest(ctx, http.MethodPost, "/bridge/plugins/"+pluginID+"/mcp/prompts/get", payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type pluginRecordResponse struct {
	Metadata struct {
		ID string `json:"id"`
	} `json:"metadata"`
	LifecycleState        model.PluginLifecycleState         `json:"lifecycle_state"`
	RuntimeHost           model.PluginRuntimeHost            `json:"runtime_host"`
	LastHealthAt          *time.Time                         `json:"last_health_at,omitempty"`
	LastError             string                             `json:"last_error,omitempty"`
	RestartCount          int                                `json:"restart_count"`
	ResolvedSourcePath    string                             `json:"resolved_source_path,omitempty"`
	RuntimeMetadata       *model.PluginRuntimeMetadata       `json:"runtime_metadata,omitempty"`
	MCPCapabilitySnapshot *model.PluginMCPCapabilitySnapshot `json:"mcp_capability_snapshot,omitempty"`
}

func (c *Client) doPluginRequest(ctx context.Context, method, path string, payload []byte) (*pluginRecordResponse, error) {
	start := time.Now()
	fields := c.requestLogFields(method, path)
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge plugin request creation failed")
		return nil, fmt.Errorf("create plugin request: %w", err)
	}
	if payload != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge plugin request failed")
		return nil, fmt.Errorf("plugin request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		logBridgeRequestResult(fields, start, resp.StatusCode, fmt.Errorf("bridge plugin request returned non-OK status"), "", "bridge plugin request returned non-OK status")
		return nil, fmt.Errorf("plugin request %s %s returned %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	var result pluginRecordResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logBridgeRequestResult(fields, start, resp.StatusCode, err, "", "bridge plugin response decode failed")
		return nil, fmt.Errorf("decode plugin response: %w", err)
	}
	fields["pluginId"] = result.Metadata.ID
	fields["lifecycleState"] = result.LifecycleState
	logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge plugin request completed", "")
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

func pluginMCPSurfaceFromRecord(record *pluginRecordResponse) *model.PluginMCPRefreshResult {
	if record == nil {
		return nil
	}

	snapshot := model.PluginMCPCapabilitySnapshot{}
	if record.MCPCapabilitySnapshot != nil {
		snapshot = *record.MCPCapabilitySnapshot
	} else if record.RuntimeMetadata != nil && record.RuntimeMetadata.MCP != nil {
		snapshot.Transport = record.RuntimeMetadata.MCP.Transport
		snapshot.LastDiscoveryAt = record.RuntimeMetadata.MCP.LastDiscoveryAt
		snapshot.ToolCount = record.RuntimeMetadata.MCP.ToolCount
		snapshot.ResourceCount = record.RuntimeMetadata.MCP.ResourceCount
		snapshot.PromptCount = record.RuntimeMetadata.MCP.PromptCount
		snapshot.LatestInteraction = record.RuntimeMetadata.MCP.LatestInteraction
	}

	return &model.PluginMCPRefreshResult{
		PluginID:        record.Metadata.ID,
		LifecycleState:  record.LifecycleState,
		RuntimeHost:     record.RuntimeHost,
		RuntimeMetadata: record.RuntimeMetadata,
		Snapshot:        snapshot,
	}
}

func (c *Client) doJSONRequest(ctx context.Context, method, path string, payload any, out any) error {
	start := time.Now()
	fields := c.requestLogFields(method, path)
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			logBridgeRequestResult(fields, start, 0, err, "", "bridge JSON request marshal failed")
			return fmt.Errorf("marshal request payload: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge JSON request creation failed")
		return fmt.Errorf("create request: %w", err)
	}
	if payload != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logBridgeRequestResult(fields, start, 0, err, "", "bridge JSON request failed")
		return fmt.Errorf("bridge request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		logBridgeRequestResult(fields, start, resp.StatusCode, fmt.Errorf("bridge JSON request returned non-OK status"), "", "bridge JSON request returned non-OK status")
		return bridgeUpstreamError(path, resp.StatusCode, respBody)
	}

	if out == nil {
		logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge JSON request completed", "")
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		logBridgeRequestResult(fields, start, resp.StatusCode, err, "", "bridge JSON response decode failed")
		return fmt.Errorf("decode bridge response: %w", err)
	}
	logBridgeRequestResult(fields, start, resp.StatusCode, nil, "bridge JSON request completed", "")
	return nil
}
