package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type stubSecretResolver struct {
	valueByName map[string]string
	err         error
}

func (s *stubSecretResolver) Resolve(_ context.Context, _ uuid.UUID, name string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	val, ok := s.valueByName[name]
	if !ok {
		return "", fmt.Errorf("secret not found: %s", name)
	}
	return val, nil
}

type stubDataStoreMerger struct{ wrote map[string]any }

func (s *stubDataStoreMerger) MergeNodeResult(_ context.Context, _ uuid.UUID, nodeID string, payload map[string]any) error {
	if s.wrote == nil {
		s.wrote = map[string]any{}
	}
	s.wrote[nodeID] = payload
	return nil
}

func TestApplyExecuteHTTPCall_SecretResolutionAnd2xx(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	ds := &stubDataStoreMerger{}
	a := &EffectApplier{
		SecretResolver:  &stubSecretResolver{valueByName: map[string]string{"GITHUB_TOKEN": "tok-abc"}},
		DataStoreMerger: ds,
	}

	payload := ExecuteHTTPCallPayload{
		Method: "GET", URL: srv.URL, TimeoutSeconds: 5,
		Headers:   map[string]string{"Authorization": "Bearer {{secrets.GITHUB_TOKEN}}"},
		ProjectID: uuid.New().String(),
	}
	raw, _ := json.Marshal(payload)
	exec := &model.WorkflowExecution{ID: uuid.New(), ProjectID: uuid.New()}
	node := &model.WorkflowNode{ID: "http-1"}
	if err := a.applyExecuteHTTPCall(context.Background(), exec, node, raw); err != nil {
		t.Fatalf("applyExecuteHTTPCall: %v", err)
	}
	if gotAuth != "Bearer tok-abc" {
		t.Fatalf("server saw Authorization=%q, want Bearer tok-abc", gotAuth)
	}
	out := ds.wrote["http-1"].(map[string]any)
	if out["status"].(int) != 200 {
		t.Errorf("status = %v, want 200", out["status"])
	}
}

func TestApplyExecuteHTTPCall_Non2xxFailsByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	a := &EffectApplier{
		SecretResolver: &stubSecretResolver{valueByName: map[string]string{}}, DataStoreMerger: &stubDataStoreMerger{},
	}
	raw, _ := json.Marshal(ExecuteHTTPCallPayload{
		Method: "GET", URL: srv.URL, TimeoutSeconds: 5, ProjectID: uuid.New().String(),
	})
	err := a.applyExecuteHTTPCall(context.Background(), &model.WorkflowExecution{}, &model.WorkflowNode{ID: "h"}, raw)
	if err == nil || err.Error() == "" {
		t.Fatalf("expected error, got nil")
	}
}

func TestApplyExecuteHTTPCall_TreatAsSuccessAllows401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	ds := &stubDataStoreMerger{}
	a := &EffectApplier{SecretResolver: &stubSecretResolver{valueByName: map[string]string{}}, DataStoreMerger: ds}
	raw, _ := json.Marshal(ExecuteHTTPCallPayload{
		Method: "GET", URL: srv.URL, TimeoutSeconds: 5,
		TreatAsSuccess: []int{401}, ProjectID: uuid.New().String(),
	})
	if err := a.applyExecuteHTTPCall(context.Background(), &model.WorkflowExecution{}, &model.WorkflowNode{ID: "h"}, raw); err != nil {
		t.Fatalf("treat_as_success should not fail: %v", err)
	}
	out := ds.wrote["h"].(map[string]any)
	if out["status"].(int) != 401 {
		t.Errorf("status = %v", out["status"])
	}
}
