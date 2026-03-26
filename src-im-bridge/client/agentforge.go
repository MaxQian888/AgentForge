package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/agentforge/im-bridge/core"
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

// CreateTask creates a new task via the AgentForge API.
func (c *AgentForgeClient) CreateTask(ctx context.Context, title, description string) (*Task, error) {
	body := map[string]string{"title": title, "description": description, "project_id": c.projectID}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/tasks", body)
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
	path := fmt.Sprintf("/api/v1/tasks?project_id=%s", c.projectID)
	if filter != "" {
		path += "&status=" + filter
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

// --- Agent operations ---

// SpawnAgent spawns an AI agent for a task.
func (c *AgentForgeClient) SpawnAgent(ctx context.Context, taskID string) (*TaskDispatchResponse, error) {
	body := map[string]string{"taskId": taskID}
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

// ListProjectMembers returns project members that can be used for command-side assignee resolution.
func (c *AgentForgeClient) ListProjectMembers(ctx context.Context) ([]Member, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/members", c.projectID), nil)
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
	return &status, nil
}

// --- Cost operations ---

// GetCostStats returns cost statistics for the project.
func (c *AgentForgeClient) GetCostStats(ctx context.Context) (*CostStats, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/costs", c.projectID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}
	var stats CostStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &stats, nil
}

// --- Review operations ---

// TriggerReview triggers a code review for a PR URL.
func (c *AgentForgeClient) TriggerReview(ctx context.Context, prURL string) (*Review, error) {
	body := map[string]string{"prUrl": prURL, "projectId": c.projectID}
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
	path := fmt.Sprintf("/api/v1/projects/%s/sprints?status=active", c.projectID)
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
	// Step 1: create a task from the prompt.
	task, err := c.CreateTask(ctx, prompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	// Step 2: spawn an agent for the task.
	result, err := c.SpawnAgent(ctx, task.ID)
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

func (c *AgentForgeClient) HeartbeatBridge(ctx context.Context, bridgeID string) (*BridgeHeartbeat, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/im/bridge/heartbeat", map[string]string{
		"bridgeId": bridgeID,
	})
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

// AgentRun represents an AI agent execution.
type AgentRun struct {
	ID       string  `json:"id"`
	TaskID   string  `json:"taskId"`
	MemberID string  `json:"memberId,omitempty"`
	Status   string  `json:"status"`
	CostUsd  float64 `json:"costUsd"`
}

type DispatchOutcome struct {
	Status string    `json:"status"`
	Reason string    `json:"reason,omitempty"`
	Run    *AgentRun `json:"run,omitempty"`
}

type TaskDispatchResponse struct {
	Task     Task            `json:"task"`
	Dispatch DispatchOutcome `json:"dispatch"`
}

type Member struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	IsActive bool   `json:"isActive"`
}

// PoolStatus represents the agent pool status.
type PoolStatus struct {
	ActiveAgents int `json:"active_agents"`
	MaxAgents    int `json:"max_agents"`
}

// CostStats represents project cost statistics.
type CostStats struct {
	TotalUsd   float64 `json:"total_usd"`
	BudgetUsd  float64 `json:"budget_usd"`
	DailyUsd   float64 `json:"daily_usd"`
	WeeklyUsd  float64 `json:"weekly_usd"`
	MonthlyUsd float64 `json:"monthly_usd"`
}

// Review represents an AgentForge code review.
type Review struct {
	ID             string  `json:"id"`
	TaskID         string  `json:"taskId"`
	PRURL          string  `json:"prUrl"`
	Status         string  `json:"status"`
	RiskLevel      string  `json:"riskLevel"`
	Summary        string  `json:"summary"`
	Recommendation string  `json:"recommendation"`
	CostUSD        float64 `json:"costUsd"`
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

type BridgeRegistration struct {
	BridgeID         string            `json:"bridgeId"`
	Platform         string            `json:"platform"`
	Transport        string            `json:"transport"`
	ProjectIDs       []string          `json:"projectIds,omitempty"`
	Capabilities     map[string]bool   `json:"capabilities,omitempty"`
	CapabilityMatrix map[string]any    `json:"capabilityMatrix,omitempty"`
	CallbackPaths    []string          `json:"callbackPaths,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

type BridgeInstance struct {
	BridgeID         string         `json:"bridgeId"`
	Platform         string         `json:"platform"`
	Transport        string         `json:"transport"`
	CapabilityMatrix map[string]any `json:"capabilityMatrix,omitempty"`
	LastSeenAt       string         `json:"lastSeenAt"`
	ExpiresAt        string         `json:"expiresAt"`
	Status           string         `json:"status"`
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
