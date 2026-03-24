package pluginsdk

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	ABIVersion       = "v1"
	ABIVersionNumber = uint64(1)
)

type Plugin interface {
	Describe(ctx *Context) (map[string]any, error)
	Init(ctx *Context) error
	Health(ctx *Context) (map[string]any, error)
	Invoke(ctx *Context, operation string, payload map[string]any) (map[string]any, error)
}

type Context struct {
	config map[string]any
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
	Error     *envelopeError `json:"error,omitempty"`
}

type envelopeError struct {
	Message string `json:"message"`
}

func NewRuntime(plugin Plugin) *Runtime {
	return &Runtime{plugin: plugin}
}

func (r *Runtime) ABIVersion() uint64 {
	return ABIVersionNumber
}

func (r *Runtime) Run() uint32 {
	ctx := &Context{config: parseMapEnv("AGENTFORGE_CONFIG")}
	operation := os.Getenv("AGENTFORGE_OPERATION")
	payload := parseMapEnv("AGENTFORGE_PAYLOAD")

	var (
		data map[string]any
		err  error
	)

	switch operation {
	case "describe":
		data, err = r.plugin.Describe(ctx)
	case "init":
		err = r.plugin.Init(ctx)
		if err == nil {
			data = map[string]any{"status": "initialized"}
		}
	case "health":
		data, err = r.plugin.Health(ctx)
	default:
		data, err = r.plugin.Invoke(ctx, operation, payload)
	}

	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(envelope{
			OK:        false,
			Operation: operation,
			Error: &envelopeError{
				Message: err.Error(),
			},
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
