package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"
)

// HTTPCallHandler implements the "http_call" node type. It is a pure
// handler: it captures the templated config into an EffectExecuteHTTPCall
// effect. The applier is what resolves {{secrets.X}}, dials the network,
// and writes the response into dataStore.
type HTTPCallHandler struct{}

var allowedMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
}

func (HTTPCallHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	cfg := req.Config
	method := upperString(cfg["method"])
	if method == "" {
		method = "GET"
	}
	if !allowedMethods[method] {
		return nil, fmt.Errorf("http_call: unsupported method %q", method)
	}
	url, _ := cfg["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("http_call: url is required")
	}
	timeout := 30
	if v, ok := cfg["timeout_seconds"].(float64); ok && v > 0 {
		timeout = int(v)
	}
	if timeout > 300 {
		timeout = 300
	}

	headers := stringMap(cfg["headers"])
	urlQuery := stringMap(cfg["url_query"])
	body, _ := cfg["body"].(string)
	treatAsSuccess := intSlice(cfg["treat_as_success"])

	payload := ExecuteHTTPCallPayload{
		Method:         method,
		URL:            url,
		Headers:        headers,
		URLQuery:       urlQuery,
		Body:           body,
		TimeoutSeconds: timeout,
		TreatAsSuccess: treatAsSuccess,
		ProjectID:      req.ProjectID.String(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	return &NodeExecResult{
		Effects: []Effect{{Kind: EffectExecuteHTTPCall, Payload: raw}},
	}, nil
}

func (HTTPCallHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type":"object",
  "required":["url"],
  "properties":{
    "method":{"type":"string","enum":["GET","POST","PUT","PATCH","DELETE"]},
    "url":{"type":"string"},
    "headers":{"type":"object","additionalProperties":{"type":"string"}},
    "url_query":{"type":"object","additionalProperties":{"type":"string"}},
    "body":{"type":"string"},
    "timeout_seconds":{"type":"number","minimum":1,"maximum":300},
    "treat_as_success":{"type":"array","items":{"type":"integer"}}
  }
}`)
}

func (HTTPCallHandler) Capabilities() []EffectKind {
	return []EffectKind{EffectExecuteHTTPCall}
}

// ── helpers ──

func upperString(v any) string {
	s, _ := v.(string)
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		out = append(out, c)
	}
	return string(out)
}

func stringMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, val := range m {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	return out
}

func intSlice(v any) []int {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]int, 0, len(arr))
	for _, x := range arr {
		switch n := x.(type) {
		case float64:
			out = append(out, int(n))
		case int:
			out = append(out, n)
		}
	}
	return out
}
