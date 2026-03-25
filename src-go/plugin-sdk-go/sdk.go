package pluginsdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	ABIVersion       = "v1"
	ABIVersionNumber = uint64(1)
)

type Capability struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type Descriptor struct {
	APIVersion   string       `json:"apiVersion,omitempty"`
	Kind         string       `json:"kind,omitempty"`
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Version      string       `json:"version,omitempty"`
	Runtime      string       `json:"runtime,omitempty"`
	ABIVersion   string       `json:"abiVersion"`
	Description  string       `json:"description,omitempty"`
	Capabilities []Capability `json:"capabilities,omitempty"`
}

type Invocation struct {
	Operation string         `json:"operation"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type Result struct {
	Data map[string]any `json:"data,omitempty"`
}

type RuntimeError struct {
	Code    string         `json:"code,omitempty"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type Plugin interface {
	Describe(ctx *Context) (*Descriptor, error)
	Init(ctx *Context) error
	Health(ctx *Context) (*Result, error)
	Invoke(ctx *Context, invocation Invocation) (*Result, error)
}

type Context struct {
	config       map[string]any
	operation    string
	capabilities []string
}

func (c *Context) ConfigValue(key string) any {
	if c == nil || c.config == nil {
		return nil
	}
	return c.config[key]
}

func (c *Context) ConfigString(key string) string {
	value := c.ConfigValue(key)
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func (c *Context) Operation() string {
	if c == nil {
		return ""
	}
	return c.operation
}

func (c *Context) AllowedCapabilities() []string {
	if c == nil || len(c.capabilities) == 0 {
		return nil
	}
	values := make([]string, len(c.capabilities))
	copy(values, c.capabilities)
	return values
}

func (c *Context) CapabilityAllowed(name string) bool {
	if c == nil || len(c.capabilities) == 0 {
		return false
	}
	for _, capability := range c.capabilities {
		if capability == name {
			return true
		}
	}
	return false
}

func (c *Context) Log(level, message string) {
	entry := map[string]any{
		"ts":      time.Now().UTC().Format(time.RFC3339),
		"level":   level,
		"message": message,
	}
	_ = json.NewEncoder(os.Stderr).Encode(entry)
}

type Runtime struct {
	plugin Plugin
}

type envelope struct {
	OK        bool           `json:"ok"`
	Operation string         `json:"operation"`
	Data      map[string]any `json:"data,omitempty"`
	Error     *RuntimeError  `json:"error,omitempty"`
}

func NewRuntime(plugin Plugin) *Runtime {
	return &Runtime{plugin: plugin}
}

func Success(data map[string]any) *Result {
	if data == nil {
		data = map[string]any{}
	}
	return &Result{Data: data}
}

func NewRuntimeError(code, message string) *RuntimeError {
	return &RuntimeError{
		Code:    code,
		Message: message,
	}
}

func (e *RuntimeError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *RuntimeError) WithDetail(key string, value any) *RuntimeError {
	if e == nil {
		return nil
	}
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

func (r *Runtime) ABIVersion() uint64 {
	return ABIVersionNumber
}

func (r *Runtime) Run() uint32 {
	operation := os.Getenv("AGENTFORGE_OPERATION")
	ctx := &Context{
		config:       parseMapEnv("AGENTFORGE_CONFIG"),
		operation:    operation,
		capabilities: parseListEnv("AGENTFORGE_CAPABILITIES"),
	}
	payload := parseMapEnv("AGENTFORGE_PAYLOAD")

	var (
		data map[string]any
		err  error
	)

	switch operation {
	case "describe":
		var descriptor *Descriptor
		descriptor, err = r.plugin.Describe(ctx)
		if err == nil {
			data, err = normalizeEnvelopeData(descriptor)
		}
	case "init":
		err = r.plugin.Init(ctx)
		if err == nil {
			data = map[string]any{"status": "initialized"}
		}
	case "health":
		var result *Result
		result, err = r.plugin.Health(ctx)
		if err == nil {
			data = normalizeResult(result)
		}
	default:
		var result *Result
		result, err = r.plugin.Invoke(ctx, Invocation{
			Operation: operation,
			Payload:   payload,
		})
		if err == nil {
			data = normalizeResult(result)
		}
	}

	if err != nil {
		runtimeError := AsRuntimeError(err)
		_ = json.NewEncoder(os.Stdout).Encode(envelope{
			OK:        false,
			Operation: operation,
			Error:     runtimeError,
		})
		return 1
	}

	_ = json.NewEncoder(os.Stdout).Encode(envelope{
		OK:        true,
		Operation: operation,
		Data:      data,
	})
	return 0
}

func ExportABIVersion(runtime *Runtime) uint64 {
	if runtime == nil {
		return 0
	}
	return runtime.ABIVersion()
}

func ExportRun(runtime *Runtime) uint32 {
	if runtime == nil {
		return 1
	}
	return runtime.Run()
}

func Autorun(runtime *Runtime) {
	if ShouldAutorun() {
		os.Exit(int(ExportRun(runtime)))
	}
}

func ShouldAutorun() bool {
	enabled, err := strconv.ParseBool(os.Getenv("AGENTFORGE_AUTORUN"))
	return err == nil && enabled
}

func parseMapEnv(name string) map[string]any {
	raw := os.Getenv(name)
	if raw == "" {
		return map[string]any{}
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return map[string]any{}
	}
	return decoded
}

func parseListEnv(name string) []string {
	raw := os.Getenv(name)
	if raw == "" {
		return nil
	}
	var decoded []string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil
	}
	return decoded
}

func normalizeResult(result *Result) map[string]any {
	if result == nil || result.Data == nil {
		return map[string]any{}
	}
	return result.Data
}

func normalizeEnvelopeData(value any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func AsRuntimeError(err error) *RuntimeError {
	if err == nil {
		return nil
	}
	var runtimeError *RuntimeError
	if errors.As(err, &runtimeError) && runtimeError != nil {
		return runtimeError
	}
	return &RuntimeError{
		Message: err.Error(),
	}
}
