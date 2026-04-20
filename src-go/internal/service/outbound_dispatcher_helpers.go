package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
)

// dispatcherReplyTarget is the wire shape the trigger handler writes into
// system_metadata["reply_target"]. It mirrors the IM Bridge's
// core.ReplyTarget but the dispatcher only consumes platform + chat_id +
// thread_id + message_id.
type dispatcherReplyTarget struct {
	Platform  string `json:"platform"`
	ChatID    string `json:"chat_id"`
	ChannelID string `json:"channel_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
	MessageID string `json:"message_id,omitempty"`
}

func decodeSystemMetadata(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func decodeReplyTarget(raw any) *dispatcherReplyTarget {
	if raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var t dispatcherReplyTarget
	if err := json.Unmarshal(b, &t); err != nil {
		return nil
	}
	if strings.TrimSpace(t.Platform) == "" || (strings.TrimSpace(t.ChatID) == "" && strings.TrimSpace(t.ChannelID) == "") {
		return nil
	}
	if t.ChatID == "" {
		t.ChatID = t.ChannelID
	}
	return &t
}

// providerNeutralCard mirrors the spec §8 wire shape produced for the
// IM Bridge `card` field. We only need to emit valid JSON; the bridge owns
// rendering.
type providerNeutralCard struct {
	Title   string                  `json:"title"`
	Status  string                  `json:"status,omitempty"`
	Summary string                  `json:"summary,omitempty"`
	Fields  []map[string]string     `json:"fields,omitempty"`
	Actions []providerNeutralAction `json:"actions,omitempty"`
	Footer  string                  `json:"footer,omitempty"`
}

type providerNeutralAction struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Style string `json:"style,omitempty"`
	Type  string `json:"type"`
	URL   string `json:"url,omitempty"`
}

func (d *OutboundDispatcher) buildDefaultCard(ctx context.Context, exec *model.WorkflowExecution, status string, sm map[string]any) providerNeutralCard {
	workflowName := d.lookupWorkflowName(ctx, exec)
	title := workflowName
	cardStatus := "info"
	summary := ""
	fields := []map[string]string{
		{"label": "Run", "value": exec.ID.String()},
	}

	switch status {
	case model.WorkflowExecStatusCompleted:
		cardStatus = "success"
		if sm != nil {
			if final, ok := sm["final_output"].(string); ok && strings.TrimSpace(final) != "" {
				summary = strings.TrimSpace(final)
			}
		}
		if summary == "" {
			summary = "工作流执行完成"
		}
	case model.WorkflowExecStatusFailed:
		cardStatus = "failed"
		title = workflowName + " 执行失败"
		if msg := strings.TrimSpace(exec.ErrorMessage); msg != "" {
			summary = msg
		} else {
			summary = "工作流执行失败"
		}
		if sm != nil {
			if node, ok := sm["failed_node"].(string); ok && strings.TrimSpace(node) != "" {
				fields = append([]map[string]string{{"label": "失败节点", "value": node}}, fields...)
			}
		}
	}

	footer := time.Now().UTC().Format(time.RFC3339)

	actions := []providerNeutralAction{}
	if d.feBaseURL != "" {
		actions = append(actions, providerNeutralAction{
			ID:    "view",
			Label: "查看详情",
			Type:  "url",
			URL:   fmt.Sprintf("%s/runs/%s", d.feBaseURL, exec.ID.String()),
		})
	}

	return providerNeutralCard{
		Title:   title,
		Status:  cardStatus,
		Summary: summary,
		Fields:  fields,
		Actions: actions,
		Footer:  footer,
	}
}

func (d *OutboundDispatcher) lookupWorkflowName(ctx context.Context, exec *model.WorkflowExecution) string {
	fallback := exec.WorkflowID.String()
	if d.wfRepo == nil {
		return fallback
	}
	wf, err := d.wfRepo.GetByID(ctx, exec.WorkflowID)
	if err != nil || wf == nil {
		return fallback
	}
	if name := strings.TrimSpace(wf.Name); name != "" {
		return name
	}
	return fallback
}
