package nodetypes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// applyExecuteHTTPCall resolves {{secrets.X}} templates in the payload's
// headers / url / url_query / body fields, dials the URL, and writes the
// response into dataStore[nodeID].
//
// Plaintext secret values exist only inside this function's scope. They
// are NEVER copied into the request log, error message, or dataStore.
func (a *EffectApplier) applyExecuteHTTPCall(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error {
	if a.SecretResolver == nil {
		return fmt.Errorf("http_call: SecretResolver is not configured")
	}
	if a.DataStoreMerger == nil {
		return fmt.Errorf("http_call: DataStoreMerger is not configured")
	}
	var p ExecuteHTTPCallPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	projectID, err := uuid.Parse(p.ProjectID)
	if err != nil {
		projectID = exec.ProjectID
	}

	// Resolve every templated string.
	urlResolved, err := resolveSecretTemplates(ctx, a.SecretResolver, projectID, p.URL)
	if err != nil {
		return fmt.Errorf("http_call:secret_resolve url: %w", err)
	}
	bodyResolved, err := resolveSecretTemplates(ctx, a.SecretResolver, projectID, p.Body)
	if err != nil {
		return fmt.Errorf("http_call:secret_resolve body: %w", err)
	}
	headersResolved := make(map[string]string, len(p.Headers))
	for k, v := range p.Headers {
		r, err := resolveSecretTemplates(ctx, a.SecretResolver, projectID, v)
		if err != nil {
			return fmt.Errorf("http_call:secret_resolve header %q: %w", k, err)
		}
		headersResolved[k] = r
	}
	queryResolved := make(map[string]string, len(p.URLQuery))
	for k, v := range p.URLQuery {
		r, err := resolveSecretTemplates(ctx, a.SecretResolver, projectID, v)
		if err != nil {
			return fmt.Errorf("http_call:secret_resolve query %q: %w", k, err)
		}
		queryResolved[k] = r
	}

	// Append url_query as URL params.
	if len(queryResolved) > 0 {
		parsed, perr := url.Parse(urlResolved)
		if perr != nil {
			return fmt.Errorf("http_call: invalid url after resolve")
		}
		q := parsed.Query()
		for k, v := range queryResolved {
			q.Set(k, v)
		}
		parsed.RawQuery = q.Encode()
		urlResolved = parsed.String()
	}

	// Build request.
	var bodyReader io.Reader
	if bodyResolved != "" {
		bodyReader = bytes.NewReader([]byte(bodyResolved))
	}
	req, err := http.NewRequestWithContext(ctx, p.Method, urlResolved, bodyReader)
	if err != nil {
		return fmt.Errorf("http_call: build request: %w", err)
	}
	for k, v := range headersResolved {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: time.Duration(p.TimeoutSeconds) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if ue, ok := err.(interface{ Timeout() bool }); ok && ue.Timeout() {
			return fmt.Errorf("http_call:timeout")
		}
		return fmt.Errorf("http_call: dial: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	// 2xx is always success. Otherwise treat_as_success whitelist.
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !ok {
		for _, s := range p.TreatAsSuccess {
			if s == resp.StatusCode {
				ok = true
				break
			}
		}
	}
	if !ok {
		return fmt.Errorf("http_call:non_2xx_status (got %d)", resp.StatusCode)
	}

	// Write response into dataStore.
	var bodyVal any = string(respBody)
	if isJSONContentType(resp.Header.Get("Content-Type")) && json.Valid(respBody) {
		var parsed any
		_ = json.Unmarshal(respBody, &parsed)
		bodyVal = parsed
	}
	result := map[string]any{
		"status":  resp.StatusCode,
		"headers": flattenHeaders(resp.Header),
		"body":    bodyVal,
	}
	if err := a.DataStoreMerger.MergeNodeResult(ctx, exec.ID, node.ID, result); err != nil {
		return fmt.Errorf("http_call: merge result: %w", err)
	}

	// Audit log: NEVER include the URL with query (might contain secrets).
	if a.AuditSink != nil {
		host := ""
		if u, _ := url.Parse(urlResolved); u != nil {
			host = u.Host
		}
		_ = a.AuditSink.Record(ctx, "http_call_executed", map[string]any{
			"executionId": exec.ID.String(),
			"nodeId":      node.ID,
			"method":      p.Method,
			"urlHost":     host,
			"status":      resp.StatusCode,
		})
	}
	return nil
}

// resolveSecretTemplates scans a string for {{secrets.NAME}} patterns and
// replaces each with the resolved plaintext from the SecretResolver.
func resolveSecretTemplates(ctx context.Context, resolver SecretResolver, projectID uuid.UUID, template string) (string, error) {
	if !strings.Contains(template, "{{secrets.") {
		return template, nil
	}
	result := template
	for {
		start := strings.Index(result, "{{secrets.")
		if start < 0 {
			break
		}
		end := strings.Index(result[start:], "}}")
		if end < 0 {
			break
		}
		end += start + 2
		// Extract the secret name: {{secrets.NAME}} -> NAME
		inner := result[start+len("{{secrets.") : end-2]
		plaintext, err := resolver.Resolve(ctx, projectID, inner)
		if err != nil {
			return "", fmt.Errorf("secret %q: %w", inner, err)
		}
		result = result[:start] + plaintext + result[end:]
	}
	return result, nil
}

func isJSONContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
	return ct == "application/json" || strings.HasSuffix(ct, "+json")
}

func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}
