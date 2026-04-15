package eventbus

import (
	"fmt"
	"strings"
)

type Address struct {
	Scheme string
	Name   string
	Raw    string
}

var validSchemes = map[string]struct{}{
	"agent":    {},
	"role":     {},
	"task":     {},
	"project":  {},
	"team":     {},
	"workflow": {},
	"plugin":   {},
	"skill":    {},
	"user":     {},
	"channel":  {},
}

func ParseAddress(s string) (Address, error) {
	if s == "" {
		return Address{}, fmt.Errorf("address: empty")
	}
	if s == "core" {
		return Address{Scheme: "core", Raw: s}, nil
	}
	idx := strings.IndexByte(s, ':')
	if idx <= 0 || idx == len(s)-1 {
		return Address{}, fmt.Errorf("address: %q malformed", s)
	}
	scheme, rest := s[:idx], s[idx+1:]
	if _, ok := validSchemes[scheme]; !ok {
		return Address{}, fmt.Errorf("address: unknown scheme %q", scheme)
	}
	return Address{Scheme: scheme, Name: rest, Raw: s}, nil
}

// ChannelScope returns the inner address embedded in a channel:xxx URI,
// e.g. channel:task:7b2e -> (task:7b2e, true).
func (a Address) ChannelScope() (Address, bool) {
	if a.Scheme != "channel" {
		return Address{}, false
	}
	inner, err := ParseAddress(a.Name)
	if err != nil {
		return Address{}, false
	}
	return inner, true
}

func MakeChannel(scope, name string) string {
	return "channel:" + scope + ":" + name
}

func MakeAgent(runID string) string   { return "agent:" + runID }
func MakeTask(taskID string) string   { return "task:" + taskID }
func MakeProject(pid string) string   { return "project:" + pid }
func MakeTeam(tid string) string      { return "team:" + tid }
func MakeWorkflow(wid string) string  { return "workflow:" + wid }
func MakePlugin(pid string) string    { return "plugin:" + pid }
func MakeUser(uid string) string      { return "user:" + uid }
