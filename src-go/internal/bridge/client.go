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
)

// ExecuteRequest is sent to the bridge to start an agent session.
type ExecuteRequest struct {
	TaskID    string  `json:"taskId"`
	MemberID  string  `json:"memberId"`
	Provider  string  `json:"provider"`
	Model     string  `json:"model"`
	Prompt    string  `json:"prompt"`
	Worktree  string  `json:"worktree"`
	BudgetUsd float64 `json:"budgetUsd"`
}

// ExecuteResponse is returned after an agent is started.
type ExecuteResponse struct {
	SessionID string `json:"sessionId"`
}

// StatusResponse holds agent run status from the bridge.
type StatusResponse struct {
	Status       string  `json:"status"`
	CostUsd      float64 `json:"costUsd"`
	TurnCount    int     `json:"turnCount"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/execute", bytes.NewReader(body))
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/status/"+taskID, nil)
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

// Cancel sends a cancel request to the bridge.
func (c *Client) Cancel(ctx context.Context, taskID, reason string) error {
	payload, _ := json.Marshal(map[string]string{"reason": reason})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/cancel/"+taskID, bytes.NewReader(payload))
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

// Health checks if the bridge is reachable.
func (c *Client) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
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
