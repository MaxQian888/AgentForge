package service

import "testing"

func TestShouldIgnoreBridgeTaskID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		taskID string
		want   bool
	}{
		{taskID: "__heartbeat__", want: true},
		{taskID: "__bridge__", want: true},
		{taskID: "  __heartbeat__  ", want: true},
		{taskID: "4d8f9e90-2d0f-4f1b-8c28-8a4f8f6d1a23", want: false},
		{taskID: "", want: false},
	}

	for _, tc := range cases {
		if got := shouldIgnoreBridgeTaskID(tc.taskID); got != tc.want {
			t.Fatalf("shouldIgnoreBridgeTaskID(%q) = %v, want %v", tc.taskID, got, tc.want)
		}
	}
}
