package main

import (
	"fmt"
	"net/smtp"
	"strings"

	pluginsdk "github.com/agentforge/server/plugin-sdk-go"
)

type emailPlugin struct{}

func (emailPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		APIVersion:  "agentforge/v1",
		Kind:        "IntegrationPlugin",
		ID:          "email-adapter",
		Name:        "Email Adapter",
		Version:     "0.1.0",
		Runtime:     "wasm",
		ABIVersion:  pluginsdk.ABIVersion,
		Description: "Sends outbound email via SMTP. Falls back to a stub mode (sent=false, stub=true) when smtp_host is unconfigured — useful for tests and dry-runs.",
		Capabilities: []pluginsdk.Capability{
			{Name: "health", Description: "Report plugin health"},
			{Name: "send_email", Description: "Send a single email message via SMTP"},
		},
	}, nil
}

func (emailPlugin) Init(ctx *pluginsdk.Context) error { return nil }

func (emailPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
	return pluginsdk.Success(map[string]any{"ok": true}), nil
}

func (p emailPlugin) Invoke(ctx *pluginsdk.Context, inv pluginsdk.Invocation) (*pluginsdk.Result, error) {
	switch inv.Operation {
	case "health":
		return pluginsdk.Success(map[string]any{"ok": true}), nil
	case "send_email":
		return p.sendEmail(inv.Payload)
	default:
		return nil, pluginsdk.NewRuntimeError("unsupported_operation",
			fmt.Sprintf("unsupported operation %s", inv.Operation)).
			WithDetail("operation", inv.Operation)
	}
}

func (emailPlugin) sendEmail(payload map[string]any) (*pluginsdk.Result, error) {
	to, _ := payload["to"].(string)
	subject, _ := payload["subject"].(string)
	body, _ := payload["body"].(string)
	smtpHost, _ := payload["smtp_host"].(string)
	smtpPort, _ := payload["smtp_port"].(string)
	from, _ := payload["from"].(string)
	username, _ := payload["username"].(string)
	password, _ := payload["password"].(string)

	if strings.TrimSpace(to) == "" {
		return nil, pluginsdk.NewRuntimeError("invalid_argument", "send_email: 'to' address is required")
	}
	if strings.TrimSpace(subject) == "" {
		return nil, pluginsdk.NewRuntimeError("invalid_argument", "send_email: 'subject' is required")
	}

	// Stub mode: no SMTP configured. The wasip1 runtime currently lacks
	// raw socket support, so production deployments that need real
	// outbound mail must run this plugin in a host that proxies SMTP, or
	// wait for wasi-sockets. Stub mode lets the rest of the notification
	// path run end-to-end during tests and dry-runs.
	if smtpHost == "" {
		return pluginsdk.Success(map[string]any{
			"sent": false,
			"stub": true,
			"to":   to,
		}), nil
	}

	if smtpPort == "" {
		smtpPort = "587"
	}
	if from == "" {
		from = username
	}

	addr := smtpHost + ":" + smtpPort
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body)

	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, smtpHost)
	}

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		return nil, pluginsdk.NewRuntimeError("smtp_error", "send_email: "+err.Error())
	}

	return pluginsdk.Success(map[string]any{
		"sent": true,
		"to":   to,
	}), nil
}

var runtime = pluginsdk.NewRuntime(emailPlugin{})

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 { return pluginsdk.ExportABIVersion(runtime) }

//go:wasmexport agentforge_run
func agentforgeRun() uint32 { return pluginsdk.ExportRun(runtime) }

func main() {
	pluginsdk.Autorun(runtime)
}
