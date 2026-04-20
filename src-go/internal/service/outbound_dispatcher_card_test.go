package service_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	eb "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/ws"
)

func TestDispatcher_DefaultCard_SuccessAndFailure(t *testing.T) {
	type capturedSend struct {
		body []byte
	}

	cases := []struct {
		name       string
		status     string
		errMessage string
		extraMeta  map[string]any
		assertCard func(t *testing.T, card map[string]any, execID string)
	}{
		{
			name:   "success card",
			status: model.WorkflowExecStatusCompleted,
			extraMeta: map[string]any{
				"final_output": "echo 工作流完成",
			},
			assertCard: func(t *testing.T, card map[string]any, execID string) {
				if card["status"] != "success" {
					t.Fatalf("status = %v want success", card["status"])
				}
				if !strings.Contains(card["title"].(string), "echo-workflow") {
					t.Fatalf("title = %v", card["title"])
				}
				if !strings.Contains(card["summary"].(string), "echo 工作流完成") {
					t.Fatalf("summary = %v", card["summary"])
				}
				fields := card["fields"].([]any)
				if len(fields) == 0 {
					t.Fatal("expected at least Run field")
				}
				lastField := fields[len(fields)-1].(map[string]any)
				if lastField["label"] != "Run" || lastField["value"] != execID {
					t.Fatalf("Run field: %v", lastField)
				}
				actions := card["actions"].([]any)
				if len(actions) != 1 {
					t.Fatalf("actions = %v", actions)
				}
				url := actions[0].(map[string]any)["url"].(string)
				if !strings.Contains(url, execID) {
					t.Fatalf("view url missing exec id: %s", url)
				}
			},
		},
		{
			name:       "failed card",
			status:     model.WorkflowExecStatusFailed,
			errMessage: "downstream service 401",
			extraMeta: map[string]any{
				"failed_node": "http_call_1",
			},
			assertCard: func(t *testing.T, card map[string]any, execID string) {
				if card["status"] != "failed" {
					t.Fatalf("status = %v want failed", card["status"])
				}
				if !strings.Contains(card["title"].(string), "执行失败") {
					t.Fatalf("title = %v", card["title"])
				}
				if !strings.Contains(card["summary"].(string), "401") {
					t.Fatalf("summary = %v", card["summary"])
				}
				labels := []string{}
				for _, f := range card["fields"].([]any) {
					labels = append(labels, f.(map[string]any)["label"].(string))
				}
				hasFailedNode := false
				hasRun := false
				for _, l := range labels {
					if l == "失败节点" {
						hasFailedNode = true
					}
					if l == "Run" {
						hasRun = true
					}
				}
				if !hasFailedNode || !hasRun {
					t.Fatalf("expected 失败节点 + Run fields, got %v", labels)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cap := &capturedSend{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				cap.body = body
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"status":"sent"}`))
			}))
			defer srv.Close()

			meta := map[string]any{"reply_target": map[string]any{"platform": "feishu", "chat_id": "c"}}
			for k, v := range c.extraMeta {
				meta[k] = v
			}
			exec := mkExec(t, c.status, meta)
			exec.ErrorMessage = c.errMessage

			d := service.NewOutboundDispatcher(&fakeExecRepo{exec: exec}, srv.URL, "https://fe.example", nil)
			d.SetRetryDelays(0, 0, 0)
			d.SetWorkflowLoader(&fakeWorkflowLoader{name: "echo-workflow"})

			payload, _ := json.Marshal(map[string]any{
				"executionId": exec.ID.String(),
				"workflowId":  exec.WorkflowID.String(),
				"status":      c.status,
			})
			ev := eb.NewEvent(ws.EventWorkflowExecutionCompleted, "core", "project:"+exec.ProjectID.String())
			ev.Payload = payload
			d.Observe(context.Background(), ev, &eb.PipelineCtx{})

			deadline := time.Now().Add(1 * time.Second)
			for time.Now().Before(deadline) && len(cap.body) == 0 {
				time.Sleep(10 * time.Millisecond)
			}
			if len(cap.body) == 0 {
				t.Fatal("dispatcher did not POST to bridge")
			}

			var sent map[string]any
			if err := json.Unmarshal(cap.body, &sent); err != nil {
				t.Fatalf("decode posted body: %v\n%s", err, cap.body)
			}
			card := sent["card"].(map[string]any)
			c.assertCard(t, card, exec.ID.String())
		})
	}
}

type fakeWorkflowLoader struct {
	name string
}

func (f *fakeWorkflowLoader) GetWorkflow(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error) {
	return &model.WorkflowDefinition{ID: id, Name: f.name}, nil
}
