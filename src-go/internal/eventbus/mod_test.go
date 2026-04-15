package eventbus

import "testing"

func TestMatchesType(t *testing.T) {
	cases := []struct {
		pat, typ string
		want     bool
	}{
		{"*", "anything.at.all", true},
		{"task.*", "task.created", true},
		{"task.*", "task", true},
		{"task.*", "tasks.created", false},
		{"task.created", "task.created", true},
		{"task.created", "task.updated", false},
		{"workflow.execution.*", "workflow.execution.started", true},
		{"workflow.execution.*", "workflow.other", false},
	}
	for _, c := range cases {
		got := MatchesType(c.pat, c.typ)
		if got != c.want {
			t.Errorf("MatchesType(%q,%q)=%v want %v", c.pat, c.typ, got, c.want)
		}
	}
}
