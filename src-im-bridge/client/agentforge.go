package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agentforge/im-bridge/core"
)

// AgentForgeClient communicates with the AgentForge Go backend API.
type AgentForgeClient struct {
	baseURL   string
	projectID string
	apiKey    string
	client    *http.Client
	imSource  string
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

// --- Task operations ---

// CreateTask creates a new task via the AgentForge API.
func (c *AgentForgeClient) CreateTask(ctx context.Context, title, description string) (*Task, error) {
	body := map[string]string{"title": title, "description": description, "project_id": c.projectID}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/tasks", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
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
func (c *AgentForgeClient) AssignTask(ctx context.Context, taskID, assignee string) (*Task, error) {
	body := map[string]string{"assignee": assignee}
	resp, err := c.doRequest(ctx, http.MethodPatch, "/api/v1/tasks/"+taskID+"/assign", body)
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

// DecomposeTask triggers AI task decomposition for an existing task.
func (c *AgentForgeClient) DecomposeTask(ctx context.Context, taskID string) (*TaskDecompositionResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/tasks/"+taskID+"/decompose", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
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
func (c *AgentForgeClient) SpawnAgent(ctx context.Context, taskID string) (*AgentRun, error) {
	body := map[string]string{"task_id": taskID}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/agents/spawn", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.readError(resp)
	}
	var run AgentRun
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &run, nil
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
	AssigneeName string  `json:"assignee_name"`
	SpentUsd     float64 `json:"spent_usd"`
	BudgetUsd    float64 `json:"budget_usd"`
	PRUrl        string  `json:"pr_url"`
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
	ID      string  `json:"id"`
	TaskID  string  `json:"task_id"`
	Status  string  `json:"status"`
	CostUsd float64 `json:"cost_usd"`
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
