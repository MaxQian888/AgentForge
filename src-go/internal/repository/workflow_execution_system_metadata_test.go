package repository

import (
	"encoding/json"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestWorkflowExecutionRecord_SystemMetadataRoundTrip(t *testing.T) {
	meta := json.RawMessage(`{"reply_target":{"provider":"feishu","chat_id":"oc_x","thread_id":"r_y"},"im_dispatched":true}`)
	rec := workflowExecutionRecord{
		ID:             uuid.New(),
		WorkflowID:     uuid.New(),
		ProjectID:      uuid.New(),
		Status:         model.WorkflowExecStatusRunning,
		CurrentNodes:   newRawJSON(json.RawMessage("[]"), "[]"),
		Context:        newRawJSON(json.RawMessage("{}"), "{}"),
		DataStore:      newRawJSON(json.RawMessage("{}"), "{}"),
		SystemMetadata: newRawJSON(meta, "{}"),
	}
	m := rec.toModel()
	if string(m.SystemMetadata) != string(meta) {
		t.Fatalf("system_metadata round-trip mismatch:\n  got:  %s\n  want: %s", string(m.SystemMetadata), string(meta))
	}
}
