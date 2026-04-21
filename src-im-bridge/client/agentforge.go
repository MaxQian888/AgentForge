package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/agentforge/im-bridge/core"
)

var errProjectScopeNotConfigured = errors.New(
	"当前未设置 project。先用 /project list 查看项目，再用 /project set <project-id|slug> 选择项目。",
)

// AgentForgeClient communicates with the AgentForge Go backend API.
type AgentForgeClient struct {
	baseURL     string
	projectID   string
	apiKey      string
	client      *http.Client
	imSource    string
	bridgeID    string
	replyTarget *core.ReplyTarget
}

// NewAgentForgeClient creates a new API client.
func NewAgentForgeClient(baseURL, projectID, apiKey string) *AgentForgeClient {
	return &AgentForgeClient{
		baseURL:   baseURL,
		projectID: projectID,
		apiKey:    apiKey,
		client:    &http.Client{Timeout: 30 * time.Second},
		imSource:  "feishu",
	}
}

// WithSource returns a shallow copy that tags outbound requests with the given IM source.
func (c *AgentForgeClient) WithSource(source string) *AgentForgeClient {
	clone := *c
	if normalized := core.NormalizePlatformName(source); normalized != "" {
		clone.imSource = normalized
	}
	return &clone
}

// WithPlatform returns a shallow copy that tags outbound requests with the
// normalized metadata source of the given platform.
func (c *AgentForgeClient) WithPlatform(platform core.Platform) *AgentForgeClient {
	return c.WithSource(core.MetadataForPlatform(platform).Source)
}

// WithProjectScope returns a shallow copy using the given project scope.
func (c *AgentForgeClient) WithProjectScope(projectID string) *AgentForgeClient {
	clone := *c
	clone.projectID = strings.TrimSpace(projectID)
	return &clone
}

// SetProjectScope updates the current project scope in-place.
func (c *AgentForgeClient) SetProjectScope(projectID string) {
	c.projectID = strings.TrimSpace(projectID)
}

// ProjectScope returns the currently configured project scope.
func (c *AgentForgeClient) ProjectScope() string {
	return strings.TrimSpace(c.projectID)
}

// WithBridgeContext returns a shallow copy tagged with the runtime bridge id and
// the current reply target for asynchronous progress delivery.
func (c *AgentForgeClient) WithBridgeContext(bridgeID string, replyTarget *core.ReplyTarget) *AgentForgeClient {
	clone := *c
	if strings.TrimSpace(bridgeID) != "" {
		clone.bridgeID = strings.TrimSpace(bridgeID)
	}
	if replyTarget != nil {
		targetCopy := *replyTarget
		if replyTarget.Metadata != nil {
			targetCopy.Metadata = make(map[string]string, len(replyTarget.Metadata))
			for key, value := range replyTarget.Metadata {
				targetCopy.Metadata[key] = value
			}
		}
		clone.replyTarget = &targetCopy
	} else {
		clone.replyTarget = nil
	}
	return &clone
}

// --- Task operations ---

type CreateTaskInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
}

// CreateTask creates a new task via the AgentForge API.
func (c *AgentForgeClient) CreateTask(ctx context.Context, title, description string) (*Task, error) {
	return c.CreateTaskWithInput(ctx, CreateTaskInput{
		Title:       title,
		Description: description,
		Priority:    "medium",
	})
}

// CreateTaskWithInput creates a new task via the AgentForge API with explicit payload fields.
func (c *AgentForgeClient) CreateTaskWithInput(ctx context.Context, input CreateTaskInput) (*Task, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	body := map[string]string{
		"title":       input.Title,
		"description": input.Description,
		"priority":    strings.TrimSpace(input.Priority),
	}
	if strings.TrimSpace(body["priority"]) == "" {
		body["priority"] = "medium"
	}
	resp, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/tasks", projectID), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var task Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &task, nil
}

// ListTasks lists tasks, optionally filtered by status.
func (c *AgentForgeClient) ListTasks(ctx context.Context, filter string) ([]Task, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/api/v1/projects/%s/tasks", projectID)
	if filter != "" {
		path = fmt.Sprintf("%s?status=%s", path, filter)
	}
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var tasks []Task
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return tasks, nil
}

// GetTask retrieves a single task by ID.
func (c *AgentForgeClient) GetTask(ctx context.Context, taskID string) (*Task, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/tasks/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var task Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &task, nil
}

// DeleteTask deletes a task by ID.
func (c *AgentForgeClient) DeleteTask(ctx context.Context, taskID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/tasks/"+strings.TrimSpace(taskID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

// AssignTask assigns a task to the given assignee.
func (c *AgentForgeClient) AssignTask(ctx context.Context, taskID, assigneeID, assigneeType string) (*TaskDispatchResponse, error) {
	body := map[string]string{
		"assigneeId":   assigneeID,
		"assigneeType": assigneeType,
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/tasks/"+taskID+"/assign", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var result TaskDispatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// DecomposeTask triggers AI task decomposition for an existing task.
func (c *AgentForgeClient) DecomposeTask(ctx context.Context, taskID string) (*TaskDecompositionResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/tasks/"+taskID+"/decompose", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var result TaskDecompositionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (c *AgentForgeClient) DecomposeTaskViaBridge(ctx context.Context, taskID, provider, model string) (*BridgeTaskDecompositionResponse, error) {
	task, err := c.GetTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("load task: %w", err)
	}

	body := map[string]string{
		"task_id":     task.ID,
		"title":       task.Title,
		"description": task.Description,
		"priority":    normalizeTaskPriority(task.Priority),
	}
	if strings.TrimSpace(provider) != "" {
		body["provider"] = strings.TrimSpace(provider)
	}
	if strings.TrimSpace(model) != "" {
		body["model"] = strings.TrimSpace(model)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/ai/decompose", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var result BridgeTaskDecompositionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	result.ParentTask = *task
	return &result, nil
}

// --- Agent operations ---

// SpawnAgent spawns an AI agent for a task.
func (c *AgentForgeClient) SpawnAgent(ctx context.Context, taskID string) (*TaskDispatchResponse, error) {
	return c.SpawnAgentWithOptions(ctx, taskID, AgentSpawnOptions{})
}

// SpawnAgentWithOptions spawns an AI agent for a task with optional runtime overrides.
func (c *AgentForgeClient) SpawnAgentWithOptions(ctx context.Context, taskID string, options AgentSpawnOptions) (*TaskDispatchResponse, error) {
	body := map[string]any{"taskId": taskID}
	if trimmed := strings.TrimSpace(options.Runtime); trimmed != "" {
		body["runtime"] = trimmed
	}
	if trimmed := strings.TrimSpace(options.Provider); trimmed != "" {
		body["provider"] = trimmed
	}
	if trimmed := strings.TrimSpace(options.Model); trimmed != "" {
		body["model"] = trimmed
	}
	if trimmed := strings.TrimSpace(options.RoleID); trimmed != "" {
		body["roleId"] = trimmed
	}
	if options.MaxBudgetUsd > 0 {
		body["maxBudgetUsd"] = options.MaxBudgetUsd
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/agents/spawn", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var result TaskDispatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (c *AgentForgeClient) ListProjects(ctx context.Context) ([]Project, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/projects", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var projects []Project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return projects, nil
}

func (c *AgentForgeClient) GetProject(ctx context.Context, projectID string) (*Project, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/projects/"+strings.TrimSpace(projectID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var project Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &project, nil
}

func (c *AgentForgeClient) UpdateProject(ctx context.Context, projectID string, input ProjectUpdateInput) (*Project, error) {
	resp, err := c.doRequest(ctx, http.MethodPut, "/api/v1/projects/"+strings.TrimSpace(projectID), input)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var project Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &project, nil
}

type CreateProjectInput struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	RepoURL     string `json:"repoUrl,omitempty"`
}

type CreateMemberInput struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Role        string   `json:"role,omitempty"`
	Status      string   `json:"status,omitempty"`
	Email       string   `json:"email,omitempty"`
	IMPlatform  string   `json:"imPlatform,omitempty"`
	IMUserID    string   `json:"imUserId,omitempty"`
	AgentConfig string   `json:"agentConfig,omitempty"`
	Skills      []string `json:"skills,omitempty"`
}

func (c *AgentForgeClient) CreateProject(ctx context.Context, input CreateProjectInput) (*Project, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/projects", input)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.readError(resp)
	}
	var project Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &project, nil
}

func (c *AgentForgeClient) DeleteProject(ctx context.Context, projectID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/projects/"+strings.TrimSpace(projectID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

func (c *AgentForgeClient) CreateMember(ctx context.Context, input CreateMemberInput) (*Member, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/projects/"+projectID+"/members", input)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.readError(resp)
	}
	var member Member
	if err := json.NewDecoder(resp.Body).Decode(&member); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &member, nil
}

func (c *AgentForgeClient) DeleteMember(ctx context.Context, memberID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/members/"+strings.TrimSpace(memberID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

// ListProjectMembers returns project members that can be used for command-side assignee resolution.
func (c *AgentForgeClient) ListProjectMembers(ctx context.Context) ([]Member, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/members", projectID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var members []Member
	if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return members, nil
}

// GetAgentPoolStatus returns the current agent pool status.
func (c *AgentForgeClient) GetAgentPoolStatus(ctx context.Context) (*PoolStatus, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/agents/pool", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var status PoolStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if status.ActiveAgents == 0 && status.Active != 0 {
		status.ActiveAgents = status.Active
	}
	if status.MaxAgents == 0 && status.Max != 0 {
		status.MaxAgents = status.Max
	}
	return &status, nil
}

func (c *AgentForgeClient) GetBridgePoolStatus(ctx context.Context) (*BridgePoolStatus, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/bridge/pool", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var status BridgePoolStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &status, nil
}

func (c *AgentForgeClient) GetBridgeHealth(ctx context.Context) (*BridgeHealthStatus, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/bridge/health", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var status BridgeHealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &status, nil
}

func (c *AgentForgeClient) GetBridgeRuntimes(ctx context.Context) (*BridgeRuntimeCatalog, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/bridge/runtimes", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var catalog BridgeRuntimeCatalog
	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &catalog, nil
}

func (c *AgentForgeClient) ListBridgeTools(ctx context.Context) ([]BridgeTool, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/bridge/tools", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var payload struct {
		Tools []BridgeTool `json:"tools"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return payload.Tools, nil
}

func (c *AgentForgeClient) InstallBridgeTool(ctx context.Context, manifestURL string) (*BridgePluginRecord, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/bridge/tools/install", map[string]string{
		"manifest_url": manifestURL,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var record BridgePluginRecord
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &record, nil
}

func (c *AgentForgeClient) UninstallBridgeTool(ctx context.Context, pluginID string) (*BridgePluginRecord, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/bridge/tools/uninstall", map[string]string{
		"plugin_id": pluginID,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var record BridgePluginRecord
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &record, nil
}

func (c *AgentForgeClient) RestartBridgeTool(ctx context.Context, pluginID string) (*BridgePluginRecord, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/bridge/tools/"+pluginID+"/restart", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var record BridgePluginRecord
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &record, nil
}

func (c *AgentForgeClient) GenerateTaskAI(ctx context.Context, prompt, provider, model string) (*TaskAIGenerateResponse, error) {
	body := map[string]string{"prompt": prompt}
	if strings.TrimSpace(provider) != "" {
		body["provider"] = strings.TrimSpace(provider)
	}
	if strings.TrimSpace(model) != "" {
		body["model"] = strings.TrimSpace(model)
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/ai/generate", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var result TaskAIGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (c *AgentForgeClient) ClassifyTaskAI(ctx context.Context, text string, candidates []string) (*TaskAIClassifyResponse, error) {
	body := map[string]any{"text": text}
	if len(candidates) > 0 {
		body["candidates"] = candidates
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/ai/classify-intent", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var result TaskAIClassifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (c *AgentForgeClient) ClassifyMentionIntent(ctx context.Context, req MentionIntentRequest) (*TaskAIClassifyResponse, error) {
	body := map[string]any{"text": req.Text, "project_id": c.projectID}
	if strings.TrimSpace(req.UserID) != "" {
		body["user_id"] = strings.TrimSpace(req.UserID)
	}
	if len(req.Candidates) > 0 {
		body["candidates"] = req.Candidates
	}
	if req.Context != nil {
		body["context"] = req.Context
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/ai/classify-intent", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var result TaskAIClassifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// GetAgentRun returns a single agent run summary.
func (c *AgentForgeClient) GetAgentRun(ctx context.Context, runID string) (*AgentRunSummary, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/agents/"+runID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var run AgentRunSummary
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &run, nil
}

func (c *AgentForgeClient) PauseAgentRun(ctx context.Context, runID string) (*AgentRunSummary, error) {
	return c.agentRunLifecycleAction(ctx, runID, "pause")
}

func (c *AgentForgeClient) ResumeAgentRun(ctx context.Context, runID string) (*AgentRunSummary, error) {
	return c.agentRunLifecycleAction(ctx, runID, "resume")
}

func (c *AgentForgeClient) KillAgentRun(ctx context.Context, runID string) (*AgentRunSummary, error) {
	return c.agentRunLifecycleAction(ctx, runID, "kill")
}

func (c *AgentForgeClient) agentRunLifecycleAction(ctx context.Context, runID, action string) (*AgentRunSummary, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/agents/%s/%s", runID, action), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var run AgentRunSummary
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &run, nil
}

// TransitionTaskStatus moves a task to a new workflow status.
func (c *AgentForgeClient) TransitionTaskStatus(ctx context.Context, taskID, status string) (*Task, error) {
	body := map[string]string{"status": status}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/tasks/"+taskID+"/transition", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var task Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &task, nil
}

// ListQueueEntries returns project-scoped queue entries.
func (c *AgentForgeClient) ListQueueEntries(ctx context.Context, status string) ([]QueueEntry, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		query.Set("status", trimmed)
	}
	path := fmt.Sprintf("/api/v1/projects/%s/queue", projectID)
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var entries []QueueEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return entries, nil
}

// CancelQueueEntry cancels a queued admission entry.
func (c *AgentForgeClient) CancelQueueEntry(ctx context.Context, entryID, reason string) (*QueueEntry, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		query.Set("reason", trimmed)
	}
	path := fmt.Sprintf("/api/v1/projects/%s/queue/%s", projectID, entryID)
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var entry QueueEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &entry, nil
}

// SearchProjectMemory searches project-scoped memory entries.
func (c *AgentForgeClient) SearchProjectMemory(ctx context.Context, queryText string, limit int) ([]MemoryEntry, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("query", strings.TrimSpace(queryText))
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/memory?%s", projectID, query.Encode()), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var entries []MemoryEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return entries, nil
}

// StoreProjectMemoryNote stores a lightweight project-scoped operator note.
func (c *AgentForgeClient) StoreProjectMemoryNote(ctx context.Context, key, content string) (*MemoryEntry, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"scope":          "project",
		"category":       "episodic",
		"key":            key,
		"content":        content,
		"metadata":       `{"kind":"operator_note","editable":true,"tags":[]}`,
		"relevanceScore": 0.5,
	}
	resp, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/memory", projectID), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.readError(resp)
	}
	var entry MemoryEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &entry, nil
}

// --- Cost operations ---

// GetCostStats returns cost statistics for the project.
func (c *AgentForgeClient) GetCostStats(ctx context.Context) (*CostStats, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/stats/cost?projectId=%s", projectID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var summary costSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	totalUsd := firstNonNilFloat(summary.TotalCostUsd, summary.LegacyTotalUsd)
	budgetUsd := summary.LegacyBudgetUsd
	if summary.BudgetSummary != nil {
		budgetUsd = summary.BudgetSummary.Allocated
	}
	dailyUsd := summary.LegacyDailyUsd
	weeklyUsd := summary.LegacyWeeklyUsd
	monthlyUsd := summary.LegacyMonthlyUsd
	if summary.PeriodRollups != nil {
		dailyUsd = summary.TodayCostUsd(dailyUsd)
		weeklyUsd = summary.Last7DaysCostUsd(weeklyUsd)
		monthlyUsd = summary.Last30DaysCostUsd(monthlyUsd)
	}
	return &CostStats{
		TotalUsd:   totalUsd,
		BudgetUsd:  budgetUsd,
		DailyUsd:   dailyUsd,
		WeeklyUsd:  weeklyUsd,
		MonthlyUsd: monthlyUsd,
	}, nil
}

// --- Review operations ---

// TriggerReview triggers a code review for a PR URL.
func (c *AgentForgeClient) TriggerReview(ctx context.Context, prURL string) (*Review, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	body := map[string]string{"prUrl": prURL, "projectId": projectID}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/reviews/trigger", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var review Review
	if err := json.NewDecoder(resp.Body).Decode(&review); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &review, nil
}

// TriggerStandaloneDeepReview triggers a detached deep review for a PR URL.
func (c *AgentForgeClient) TriggerStandaloneDeepReview(ctx context.Context, prURL string) (*Review, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	body := map[string]string{"prUrl": prURL, "projectId": projectID, "trigger": "manual"}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/reviews/trigger", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var review Review
	if err := json.NewDecoder(resp.Body).Decode(&review); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &review, nil
}

// ApproveReview approves a review that is waiting for human action.
func (c *AgentForgeClient) ApproveReview(ctx context.Context, reviewID string, comment string) (*Review, error) {
	body := map[string]string{"comment": comment}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/reviews/"+reviewID+"/approve", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var review Review
	if err := json.NewDecoder(resp.Body).Decode(&review); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &review, nil
}

// RequestChangesReview requests changes for a pending_human review.
func (c *AgentForgeClient) RequestChangesReview(ctx context.Context, reviewID string, comment string) (*Review, error) {
	body := map[string]string{"comment": comment}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/reviews/"+reviewID+"/request-changes", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, c.readError(resp)
	}
	var review Review
	if err := json.NewDecoder(resp.Body).Decode(&review); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &review, nil
}

// GetReview retrieves a review by ID.
func (c *AgentForgeClient) GetReview(ctx context.Context, reviewID string) (*Review, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/reviews/"+reviewID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var review Review
	if err := json.NewDecoder(resp.Body).Decode(&review); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &review, nil
}

// --- Sprint operations ---

// GetCurrentSprint returns the active sprint for the project.
func (c *AgentForgeClient) GetCurrentSprint(ctx context.Context) (*Sprint, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/api/v1/projects/%s/sprints?status=active", projectID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var sprints []Sprint
	if err := json.NewDecoder(resp.Body).Decode(&sprints); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(sprints) == 0 {
		return nil, fmt.Errorf("no active sprint found")
	}
	return &sprints[0], nil
}

// GetSprintBurndown returns burndown metrics for a sprint.
func (c *AgentForgeClient) GetSprintBurndown(ctx context.Context, sprintID string) (*SprintMetrics, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/sprints/"+sprintID+"/burndown", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var metrics SprintMetrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &metrics, nil
}

// --- Quick Agent Run ---

// QuickAgentRun creates a task and spawns an agent in one step.
func (c *AgentForgeClient) QuickAgentRun(ctx context.Context, prompt string) (*TaskDispatchResponse, error) {
	return c.QuickAgentRunWithOptions(ctx, prompt, AgentSpawnOptions{})
}

type AgentSpawnOptions struct {
	Runtime      string
	Provider     string
	Model        string
	RoleID       string
	MaxBudgetUsd float64
}

// QuickAgentRunWithOptions creates a task and spawns an agent in one step with optional runtime overrides.
func (c *AgentForgeClient) QuickAgentRunWithOptions(ctx context.Context, prompt string, options AgentSpawnOptions) (*TaskDispatchResponse, error) {
	// Step 1: create a task from the prompt.
	task, err := c.CreateTask(ctx, prompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	// Step 2: spawn an agent for the task.
	result, err := c.SpawnAgentWithOptions(ctx, task.ID, options)
	if err != nil {
		return nil, fmt.Errorf("spawn agent: %w", err)
	}
	return result, nil
}

// GetAgentLogs retrieves recent log entries for an agent run.
func (c *AgentForgeClient) GetAgentLogs(ctx context.Context, runID string) ([]AgentLogEntry, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/agents/"+runID+"/logs", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var logs []AgentLogEntry
	if err := json.NewDecoder(resp.Body).Decode(&logs); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return logs, nil
}

// --- NLU ---

// SendNLU sends a natural language message to the intent endpoint.
func (c *AgentForgeClient) SendNLU(ctx context.Context, text, userID string) (string, error) {
	body := map[string]string{"text": text, "user_id": userID, "project_id": c.projectID}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/intent", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", c.readError(resp)
	}
	var result struct {
		Reply string `json:"reply"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return result.Reply, nil
}

func (c *AgentForgeClient) HandleIMAction(ctx context.Context, req IMActionRequest) (*IMActionResponse, error) {
	if strings.TrimSpace(req.Platform) == "" {
		req.Platform = c.imSource
	}
	if strings.TrimSpace(req.BridgeID) == "" {
		req.BridgeID = c.bridgeID
	}
	if req.ReplyTarget == nil {
		req.ReplyTarget = c.replyTarget
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/im/action", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.readError(resp)
	}

	var result IMActionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// TaskAttachmentRef references an attachment staged by the IM Bridge and
// asks the backend to persist the reference against a task. The backend is
// expected to materialize the staged file into persistent storage.
type TaskAttachmentRef struct {
	StagedID string            `json:"staged_id,omitempty"`
	URL      string            `json:"url,omitempty"`
	Kind     string            `json:"kind,omitempty"`
	Filename string            `json:"filename,omitempty"`
	MimeType string            `json:"mime_type,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// AttachToTask persists an attachment reference against a task.
func (c *AgentForgeClient) AttachToTask(ctx context.Context, taskID string, ref TaskAttachmentRef) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("task id is required")
	}
	resp, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%s/attachments", url.PathEscape(taskID)), ref)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return c.readError(resp)
	}
	return nil
}

// ReactionEvent is the payload shape shipped to the Go backend's
// POST /api/v1/im/reactions endpoint. Field names match the Go handler.
type ReactionEvent struct {
	Platform    string            `json:"platform"`
	ChatID      string            `json:"chat_id,omitempty"`
	MessageID   string            `json:"message_id,omitempty"`
	UserID      string            `json:"user_id,omitempty"`
	EmojiCode   string            `json:"emoji_code,omitempty"`
	RawEmoji    string            `json:"raw_emoji,omitempty"`
	ReactedAt   time.Time         `json:"reacted_at"`
	Removed     bool              `json:"removed,omitempty"`
	ReplyTarget *core.ReplyTarget `json:"reply_target,omitempty"`
	BridgeID    string            `json:"bridge_id,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ReactionShortcutBinding tells the backend to treat a specific unified
// emoji code on a specific reply target as a review decision shortcut.
type ReactionShortcutBinding struct {
	ReviewID    string            `json:"review_id"`
	Outcome     string            `json:"outcome"`
	EmojiCode   string            `json:"emoji_code"`
	Platform    string            `json:"platform,omitempty"`
	BridgeID    string            `json:"bridge_id,omitempty"`
	ReplyTarget *core.ReplyTarget `json:"reply_target,omitempty"`
}

// BindReviewReactionShortcut creates a binding so the backend turns a
// reaction event (via /api/v1/im/reactions) into a review approve /
// request-changes action.
func (c *AgentForgeClient) BindReviewReactionShortcut(ctx context.Context, reviewID, outcome, emojiCode string, target *core.ReplyTarget) error {
	reviewID = strings.TrimSpace(reviewID)
	if reviewID == "" {
		return errors.New("review id is required")
	}
	binding := ReactionShortcutBinding{
		ReviewID:    reviewID,
		Outcome:     strings.TrimSpace(outcome),
		EmojiCode:   strings.TrimSpace(emojiCode),
		Platform:    c.imSource,
		BridgeID:    c.bridgeID,
		ReplyTarget: target,
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/im/reactions/shortcuts", binding)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return c.readError(resp)
	}
	return nil
}

// PostReaction persists an inbound reaction event on the backend. The backend
// uses it to drive review/approval shortcuts and audit trails.
func (c *AgentForgeClient) PostReaction(ctx context.Context, event ReactionEvent) error {
	if strings.TrimSpace(event.Platform) == "" {
		event.Platform = c.imSource
	}
	if strings.TrimSpace(event.BridgeID) == "" {
		event.BridgeID = c.bridgeID
	}
	if event.ReactedAt.IsZero() {
		event.ReactedAt = time.Now().UTC()
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/im/reactions", event)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return c.readError(resp)
	}
	return nil
}

func (c *AgentForgeClient) RegisterBridge(ctx context.Context, req BridgeRegistration) (*BridgeInstance, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/im/bridge/register", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var instance BridgeInstance
	if err := json.NewDecoder(resp.Body).Decode(&instance); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &instance, nil
}

func (c *AgentForgeClient) HeartbeatBridge(ctx context.Context, bridgeID string, metadata ...map[string]string) (*BridgeHeartbeat, error) {
	body := map[string]any{
		"bridgeId": bridgeID,
	}
	if len(metadata) > 0 && len(metadata[0]) > 0 {
		body["metadata"] = metadata[0]
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/im/bridge/heartbeat", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var heartbeat BridgeHeartbeat
	if err := json.NewDecoder(resp.Body).Decode(&heartbeat); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &heartbeat, nil
}

func (c *AgentForgeClient) UnregisterBridge(ctx context.Context, bridgeID string) error {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/im/bridge/unregister", map[string]string{
		"bridgeId": bridgeID,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

func (c *AgentForgeClient) BindActionContext(ctx context.Context, binding IMActionBinding) error {
	if binding.Platform == "" {
		binding.Platform = c.imSource
	}
	if binding.BridgeID == "" {
		binding.BridgeID = c.bridgeID
	}
	if binding.ReplyTarget == nil {
		binding.ReplyTarget = c.replyTarget
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/im/bridge/bind", binding)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

// --- Document operations ---

// DocumentEntry represents a project document returned from the API.
type DocumentEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Size   string `json:"size"`
	Status string `json:"status"`
}

// ListDocuments lists documents for the current project.
func (c *AgentForgeClient) ListDocuments(ctx context.Context) ([]DocumentEntry, error) {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return nil, err
	}
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/documents", projectID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var docs []DocumentEntry
	if err := json.NewDecoder(resp.Body).Decode(&docs); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return docs, nil
}

// UploadDocumentFromURL downloads a file from the given URL and uploads it
// to the project documents endpoint as a multipart form.
func (c *AgentForgeClient) UploadDocumentFromURL(ctx context.Context, fileURL string) error {
	projectID, err := c.requireProjectScope()
	if err != nil {
		return err
	}

	// Download the file from the remote URL.
	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	dlResp, err := c.client.Do(dlReq)
	if err != nil {
		return fmt.Errorf("download file: %w", err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", dlResp.StatusCode)
	}

	// Derive file name from URL path.
	fileName := fileURL
	if idx := strings.LastIndex(fileURL, "/"); idx >= 0 && idx+1 < len(fileURL) {
		fileName = fileURL[idx+1:]
	}
	if qIdx := strings.Index(fileName, "?"); qIdx >= 0 {
		fileName = fileName[:qIdx]
	}
	if fileName == "" {
		fileName = "upload"
	}

	// Build multipart body.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, dlResp.Body); err != nil {
		return fmt.Errorf("copy file data: %w", err)
	}
	writer.Close()

	uploadURL := fmt.Sprintf("%s/api/v1/projects/%s/documents/upload", c.baseURL, projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("X-IM-Source", c.imSource)
	if c.bridgeID != "" {
		req.Header.Set("X-IM-Bridge-ID", c.bridgeID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return c.readError(resp)
	}
	return nil
}

// --- Helpers ---

func (c *AgentForgeClient) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("X-IM-Source", c.imSource)
	if c.bridgeID != "" {
		req.Header.Set("X-IM-Bridge-ID", c.bridgeID)
	}
	if c.replyTarget != nil {
		if encoded, err := json.Marshal(c.replyTarget); err == nil {
			req.Header.Set("X-IM-Reply-Target", string(encoded))
		}
	}
	return c.client.Do(req)
}

func (c *AgentForgeClient) readError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
}

func normalizeTaskPriority(priority string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "critical", "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(priority))
	default:
		return "medium"
	}
}

func (c *AgentForgeClient) requireProjectScope() (string, error) {
	projectID := strings.TrimSpace(c.projectID)
	if projectID == "" {
		return "", errProjectScopeNotConfigured
	}
	return projectID, nil
}

// --- Types ---

// Task represents an AgentForge task.
type Task struct {
	ID           string  `json:"id"`
	ProjectID    string  `json:"projectId"`
	ParentID     *string `json:"parentId,omitempty"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	Status       string  `json:"status"`
	Priority     string  `json:"priority"`
	AssigneeID   string  `json:"assigneeId,omitempty"`
	AssigneeType string  `json:"assigneeType,omitempty"`
	AssigneeName string  `json:"assigneeName"`
	SpentUsd     float64 `json:"spentUsd"`
	BudgetUsd    float64 `json:"budgetUsd"`
	PRUrl        string  `json:"prUrl"`
}

// TaskDecompositionItem represents one created subtask returned from decomposition.
type TaskDecompositionItem = Task

// TaskDecompositionResponse is returned after a task is decomposed.
type TaskDecompositionResponse struct {
	ParentTask Task                    `json:"parentTask"`
	Summary    string                  `json:"summary"`
	Subtasks   []TaskDecompositionItem `json:"subtasks"`
}

type BridgeTaskDecompositionItem struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	Priority      string `json:"priority"`
	ExecutionMode string `json:"executionMode"`
}

type BridgeTaskDecompositionResponse struct {
	ParentTask Task                          `json:"-"`
	Summary    string                        `json:"summary"`
	Subtasks   []BridgeTaskDecompositionItem `json:"subtasks"`
}

// AgentRun represents an AI agent execution.
type AgentRun struct {
	ID       string  `json:"id"`
	TaskID   string  `json:"taskId"`
	MemberID string  `json:"memberId,omitempty"`
	Status   string  `json:"status"`
	CostUsd  float64 `json:"costUsd"`
}

type AgentRunSummary struct {
	ID             string  `json:"id"`
	TaskID         string  `json:"taskId"`
	TaskTitle      string  `json:"taskTitle"`
	MemberID       string  `json:"memberId,omitempty"`
	Status         string  `json:"status"`
	Runtime        string  `json:"runtime"`
	Provider       string  `json:"provider"`
	Model          string  `json:"model"`
	CostUsd        float64 `json:"costUsd"`
	CanResume      bool    `json:"canResume"`
	LastActivityAt string  `json:"lastActivityAt"`
}

type DispatchOutcome struct {
	Status         string                 `json:"status"`
	Reason         string                 `json:"reason,omitempty"`
	Runtime        string                 `json:"runtime,omitempty"`
	Provider       string                 `json:"provider,omitempty"`
	Model          string                 `json:"model,omitempty"`
	RoleID         string                 `json:"roleId,omitempty"`
	GuardrailType  string                 `json:"guardrailType,omitempty"`
	GuardrailScope string                 `json:"guardrailScope,omitempty"`
	BudgetWarning  *DispatchBudgetWarning `json:"budgetWarning,omitempty"`
	Queue          *QueueEntry            `json:"queue,omitempty"`
	Run            *AgentRun              `json:"run,omitempty"`
}

type TaskDispatchResponse struct {
	Task     Task            `json:"task"`
	Dispatch DispatchOutcome `json:"dispatch"`
}

type DispatchBudgetWarning struct {
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

type Member struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Role       string   `json:"role"`
	Status     string   `json:"status"`
	IMPlatform string   `json:"imPlatform,omitempty"`
	IMUserID   string   `json:"imUserId,omitempty"`
	Skills     []string `json:"skills,omitempty"`
	IsActive   bool     `json:"isActive"`
}

type BridgeTool struct {
	PluginID    string `json:"plugin_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type BridgePluginMetadata struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type BridgePluginRecord struct {
	Metadata       BridgePluginMetadata `json:"metadata"`
	LifecycleState string               `json:"lifecycle_state"`
	RestartCount   int                  `json:"restart_count"`
}

type BridgePoolStatus struct {
	Active        int  `json:"active"`
	Max           int  `json:"max"`
	WarmTotal     int  `json:"warm_total"`
	WarmAvailable int  `json:"warm_available"`
	Degraded      bool `json:"degraded"`
}

type BridgeHealthPool struct {
	Active    int `json:"active"`
	Available int `json:"available"`
	Warm      int `json:"warm"`
}

type BridgeHealthStatus struct {
	Status string           `json:"status"`
	Pool   BridgeHealthPool `json:"pool"`
}

type BridgeRuntimeEntry struct {
	Key             string `json:"key"`
	Label           string `json:"label"`
	DefaultProvider string `json:"default_provider"`
	DefaultModel    string `json:"default_model"`
	Available       bool   `json:"available"`
	Diagnostics     []ProjectRuntimeDiagnostic `json:"diagnostics,omitempty"`
}

type BridgeRuntimeCatalog struct {
	DefaultRuntime string               `json:"default_runtime"`
	Runtimes       []BridgeRuntimeEntry `json:"runtimes"`
}

type TaskAIUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type TaskAIGenerateResponse struct {
	Text  string      `json:"text"`
	Usage TaskAIUsage `json:"usage"`
}

type TaskAIClassifyResponse struct {
	Intent     string  `json:"intent"`
	Command    string  `json:"command"`
	Args       string  `json:"args"`
	Confidence float64 `json:"confidence"`
	Reply      string  `json:"reply,omitempty"`
}

type CodingAgentSelection struct {
	Runtime  string `json:"runtime,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type ProjectSettings struct {
	CodingAgent CodingAgentSelection `json:"codingAgent"`
}

type ProjectSettingsPatch struct {
	CodingAgent *CodingAgentSelection `json:"codingAgent,omitempty"`
}

type ProjectUpdateInput struct {
	Name          *string              `json:"name,omitempty"`
	Description   *string              `json:"description,omitempty"`
	RepoURL       *string              `json:"repoUrl,omitempty"`
	DefaultBranch *string              `json:"defaultBranch,omitempty"`
	Settings      *ProjectSettingsPatch `json:"settings,omitempty"`
}

type ProjectRuntimeDiagnostic struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
}

type ProjectRuntimeProvider struct {
	Provider     string   `json:"provider"`
	Connected    bool     `json:"connected"`
	DefaultModel string   `json:"defaultModel,omitempty"`
	ModelOptions []string `json:"modelOptions,omitempty"`
	AuthRequired bool     `json:"authRequired,omitempty"`
	AuthMethods  []string `json:"authMethods,omitempty"`
}

type ProjectCodingAgentRuntime struct {
	Runtime             string                   `json:"runtime"`
	Label               string                   `json:"label"`
	DefaultProvider     string                   `json:"defaultProvider"`
	CompatibleProviders []string                 `json:"compatibleProviders"`
	DefaultModel        string                   `json:"defaultModel"`
	ModelOptions        []string                 `json:"modelOptions,omitempty"`
	Available           bool                     `json:"available"`
	Diagnostics         []ProjectRuntimeDiagnostic `json:"diagnostics,omitempty"`
	Providers           []ProjectRuntimeProvider `json:"providers,omitempty"`
}

type ProjectCodingAgentCatalog struct {
	DefaultRuntime   string                    `json:"defaultRuntime"`
	DefaultSelection CodingAgentSelection      `json:"defaultSelection"`
	Runtimes         []ProjectCodingAgentRuntime `json:"runtimes"`
}

type Project struct {
	ID                 string                    `json:"id"`
	Name               string                    `json:"name"`
	Slug               string                    `json:"slug"`
	Description        string                    `json:"description"`
	RepoURL            string                    `json:"repoUrl,omitempty"`
	DefaultBranch      string                    `json:"defaultBranch,omitempty"`
	Settings           ProjectSettings           `json:"settings"`
	CodingAgentCatalog *ProjectCodingAgentCatalog `json:"codingAgentCatalog,omitempty"`
}

type MentionIntentRequest struct {
	Text       string   `json:"text"`
	UserID     string   `json:"user_id,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
	Context    any      `json:"context,omitempty"`
}

// PoolStatus represents the agent pool status.
type PoolStatus struct {
	ActiveAgents    int `json:"active_agents"`
	MaxAgents       int `json:"max_agents"`
	Active          int `json:"active"`
	Max             int `json:"max"`
	Available       int `json:"available"`
	PausedResumable int `json:"pausedResumable"`
	Queued          int `json:"queued"`
}

type QueueEntry struct {
	EntryID    string  `json:"entryId"`
	ProjectID  string  `json:"projectId"`
	TaskID     string  `json:"taskId"`
	MemberID   string  `json:"memberId"`
	Status     string  `json:"status"`
	Reason     string  `json:"reason"`
	Runtime    string  `json:"runtime"`
	Provider   string  `json:"provider"`
	Model      string  `json:"model"`
	RoleID     string  `json:"roleId,omitempty"`
	Priority   int     `json:"priority"`
	BudgetUSD  float64 `json:"budgetUsd"`
	GuardrailType       string  `json:"guardrailType,omitempty"`
	GuardrailScope      string  `json:"guardrailScope,omitempty"`
	RecoveryDisposition string  `json:"recoveryDisposition,omitempty"`
	AgentRunID *string `json:"agentRunId,omitempty"`
}

type MemoryEntry struct {
	ID             string  `json:"id"`
	ProjectID      string  `json:"projectId"`
	Scope          string  `json:"scope"`
	RoleID         string  `json:"roleId"`
	Category       string  `json:"category"`
	Kind           string  `json:"kind,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Editable       bool    `json:"editable"`
	Key            string  `json:"key"`
	Content        string  `json:"content"`
	Metadata       string  `json:"metadata"`
	RelevanceScore float64 `json:"relevanceScore"`
	AccessCount    int     `json:"accessCount"`
	CreatedAt      string  `json:"createdAt"`
}

// CostStats represents project cost statistics.
type CostStats struct {
	TotalUsd   float64 `json:"total_usd"`
	BudgetUsd  float64 `json:"budget_usd"`
	DailyUsd   float64 `json:"daily_usd"`
	WeeklyUsd  float64 `json:"weekly_usd"`
	MonthlyUsd float64 `json:"monthly_usd"`
}

type costSummaryResponse struct {
	TotalCostUsd   *float64 `json:"totalCostUsd"`
	LegacyTotalUsd float64  `json:"total_usd"`
	BudgetSummary  *struct {
		Allocated float64 `json:"allocated"`
	} `json:"budgetSummary"`
	LegacyBudgetUsd float64 `json:"budget_usd"`
	PeriodRollups   *struct {
		Today struct {
			CostUsd float64 `json:"costUsd"`
		} `json:"today"`
		Last7Days struct {
			CostUsd float64 `json:"costUsd"`
		} `json:"last7Days"`
		Last30Days struct {
			CostUsd float64 `json:"costUsd"`
		} `json:"last30Days"`
	} `json:"periodRollups"`
	LegacyDailyUsd   float64 `json:"daily_usd"`
	LegacyWeeklyUsd  float64 `json:"weekly_usd"`
	LegacyMonthlyUsd float64 `json:"monthly_usd"`
}

func (c *costSummaryResponse) TodayCostUsd(fallback float64) float64 {
	if c == nil || c.PeriodRollups == nil {
		return fallback
	}
	return c.PeriodRollups.Today.CostUsd
}

func (c *costSummaryResponse) Last7DaysCostUsd(fallback float64) float64 {
	if c == nil || c.PeriodRollups == nil {
		return fallback
	}
	return c.PeriodRollups.Last7Days.CostUsd
}

func (c *costSummaryResponse) Last30DaysCostUsd(fallback float64) float64 {
	if c == nil || c.PeriodRollups == nil {
		return fallback
	}
	return c.PeriodRollups.Last30Days.CostUsd
}

func firstNonNilFloat(value *float64, fallback float64) float64 {
	if value != nil {
		return *value
	}
	return fallback
}

// Review represents an AgentForge code review.
type ReviewFinding struct {
	ID         string   `json:"id,omitempty"`
	Category   string   `json:"category,omitempty"`
	Severity   string   `json:"severity"`
	File       string   `json:"file,omitempty"`
	Line       int      `json:"line,omitempty"`
	Message    string   `json:"message"`
	Suggestion string   `json:"suggestion,omitempty"`
	Sources    []string `json:"sources,omitempty"`
	Dismissed  bool     `json:"dismissed,omitempty"`
}

type Review struct {
	ID             string          `json:"id"`
	TaskID         string          `json:"taskId"`
	PRURL          string          `json:"prUrl"`
	Status         string          `json:"status"`
	RiskLevel      string          `json:"riskLevel"`
	Findings       []ReviewFinding `json:"findings,omitempty"`
	Summary        string          `json:"summary"`
	Recommendation string          `json:"recommendation"`
	CostUSD        float64         `json:"costUsd"`
}

// Sprint represents an AgentForge sprint.
type Sprint struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	StartDate      string  `json:"startDate"`
	EndDate        string  `json:"endDate"`
	Status         string  `json:"status"`
	TotalBudgetUsd float64 `json:"totalBudgetUsd"`
	SpentUsd       float64 `json:"spentUsd"`
}

// SprintMetrics represents burndown metrics for a sprint.
type SprintMetrics struct {
	Sprint          Sprint          `json:"sprint"`
	PlannedTasks    int             `json:"plannedTasks"`
	CompletedTasks  int             `json:"completedTasks"`
	RemainingTasks  int             `json:"remainingTasks"`
	CompletionRate  float64         `json:"completionRate"`
	VelocityPerWeek float64         `json:"velocityPerWeek"`
	Burndown        []BurndownPoint `json:"burndown"`
}

// BurndownPoint represents a single data point in a burndown chart.
type BurndownPoint struct {
	Date           string `json:"date"`
	RemainingTasks int    `json:"remainingTasks"`
	CompletedTasks int    `json:"completedTasks"`
}

// AgentLogEntry represents a single log entry from an agent run.
type AgentLogEntry struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Content   string `json:"content"`
}

// BridgeProvider describes one IM provider active on this Bridge process.
// Serialization is identical to the backend model.IMBridgeProvider.
type BridgeProvider struct {
	ID               string         `json:"id"`
	Transport        string         `json:"transport"`
	ReadinessTier    string         `json:"readinessTier,omitempty"`
	CapabilityMatrix map[string]any `json:"capabilityMatrix,omitempty"`
	CallbackPaths    []string       `json:"callbackPaths,omitempty"`
	Tenants          []string       `json:"tenants,omitempty"`
	MetadataSource   string         `json:"metadataSource"`
}

// BridgeCommandPlugin mirrors a Bridge-side core/plugin manifest.
type BridgeCommandPlugin struct {
	ID         string   `json:"id"`
	Version    string   `json:"version"`
	Commands   []string `json:"commands"`
	Tenants    []string `json:"tenants,omitempty"`
	SourcePath string   `json:"sourcePath,omitempty"`
}

type BridgeRegistration struct {
	BridgeID         string            `json:"bridgeId"`
	Platform         string            `json:"platform"`
	Transport        string            `json:"transport"`
	ProjectIDs       []string          `json:"projectIds,omitempty"`
	Capabilities     map[string]bool   `json:"capabilities,omitempty"`
	CapabilityMatrix map[string]any    `json:"capabilityMatrix,omitempty"`
	CallbackPaths    []string          `json:"callbackPaths,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	// Tenants this provider serves on this bridge. Empty in legacy single-
	// tenant mode; backend treats empty as the default tenant derived from
	// ProjectIDs[0] for compatibility.
	Tenants []string `json:"tenants,omitempty"`
	// TenantManifest enumerates every tenant declared on this bridge with
	// its backend projectId so the control plane can index by (bridgeId,
	// providerId, tenantId) without a separate lookup.
	TenantManifest []TenantBinding     `json:"tenantManifest,omitempty"`
	Providers      []BridgeProvider      `json:"providers,omitempty"`
	CommandPlugins []BridgeCommandPlugin `json:"commandPlugins,omitempty"`
}

// TenantBinding is the registration-time declaration of a tenant hosted
// by this bridge process.
type TenantBinding struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
}

type BridgeInstance struct {
	BridgeID         string            `json:"bridgeId"`
	Platform         string            `json:"platform"`
	Transport        string            `json:"transport"`
	CapabilityMatrix map[string]any    `json:"capabilityMatrix,omitempty"`
	LastSeenAt       string            `json:"lastSeenAt"`
	ExpiresAt        string            `json:"expiresAt"`
	Status           string            `json:"status"`
	Providers      []BridgeProvider      `json:"providers,omitempty"`
	CommandPlugins []BridgeCommandPlugin `json:"commandPlugins,omitempty"`
}

type BridgeHeartbeat struct {
	BridgeID   string `json:"bridgeId"`
	LastSeenAt string `json:"lastSeenAt"`
	ExpiresAt  string `json:"expiresAt"`
	Status     string `json:"status"`
}

type IMActionBinding struct {
	BridgeID    string            `json:"bridgeId"`
	Platform    string            `json:"platform"`
	ProjectID   string            `json:"projectId,omitempty"`
	TaskID      string            `json:"taskId,omitempty"`
	RunID       string            `json:"runId,omitempty"`
	ReviewID    string            `json:"reviewId,omitempty"`
	ReplyTarget *core.ReplyTarget `json:"replyTarget,omitempty"`
}

type IMActionRequest struct {
	Platform    string            `json:"platform"`
	Action      string            `json:"action"`
	EntityID    string            `json:"entityId"`
	ChannelID   string            `json:"channelId"`
	UserID      string            `json:"userId,omitempty"`
	BridgeID    string            `json:"bridgeId,omitempty"`
	ReplyTarget *core.ReplyTarget `json:"replyTarget,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type IMActionResponse struct {
	Result        string                     `json:"result"`
	Success       bool                       `json:"success"`
	Status        string                     `json:"status,omitempty"`
	Task          *Task                      `json:"task,omitempty"`
	Dispatch      *DispatchOutcome           `json:"dispatch,omitempty"`
	Decomposition *TaskDecompositionResponse `json:"decomposition,omitempty"`
	Review        *Review                    `json:"review,omitempty"`
	ReplyTarget   *core.ReplyTarget          `json:"replyTarget,omitempty"`
	Metadata      map[string]string          `json:"metadata,omitempty"`
	Structured    *core.StructuredMessage    `json:"structured,omitempty"`
	Native        *core.NativeMessage        `json:"native,omitempty"`
}

// TriggerIMEventRequest is the payload shape for POST /api/v1/triggers/im/events.
// It mirrors the Go handler's imEventRequest struct (src-go/internal/handler/trigger_handler.go).
type TriggerIMEventRequest struct {
	Platform    string            `json:"platform"`
	Command     string            `json:"command"`
	Content     string            `json:"content,omitempty"`
	Args        []any             `json:"args,omitempty"`
	ChatID      string            `json:"chatId,omitempty"`
	ThreadID    string            `json:"threadId,omitempty"`
	UserID      string            `json:"userId,omitempty"`
	UserName    string            `json:"userName,omitempty"`
	TenantID    string            `json:"tenantId,omitempty"`
	MessageID   string            `json:"messageId,omitempty"`
	ReplyTarget *core.ReplyTarget `json:"replyTarget,omitempty"`
	Extra       map[string]any    `json:"extra,omitempty"`
}

// TriggerIMEventResponse is the 202/404 JSON body.
type TriggerIMEventResponse struct {
	Started int    `json:"started"`
	Message string `json:"message,omitempty"`
}

// TriggerIMEvent posts a normalized IM event to the backend router.
// Returns the count of executions started and an error. Callers distinguish
// "no matching trigger" (404, started=0) from transport errors — a 404 is
// returned with a nil error and Started=0, so callers can reply to the user
// accordingly rather than treating it as an error.
func (c *AgentForgeClient) TriggerIMEvent(ctx context.Context, req TriggerIMEventRequest) (*TriggerIMEventResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/triggers/im/events", req)
	if err != nil {
		return nil, fmt.Errorf("trigger im event: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted, http.StatusNotFound:
		var decoded TriggerIMEventResponse
		if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
			return nil, fmt.Errorf("decode trigger response: %w", err)
		}
		return &decoded, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("trigger im event: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}
