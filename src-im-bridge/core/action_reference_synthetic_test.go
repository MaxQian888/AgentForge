package core

import "testing"

func TestSyntheticActionNames_AreClosedEnum(t *testing.T) {
	want := []string{
		"react",
		"select",
		"multi_select",
		"date_pick",
		"overflow",
		"toggle",
		"input_submit",
		"form_submit",
	}
	got := []string{
		ActionNameReact,
		ActionNameSelect,
		ActionNameMultiSelect,
		ActionNameDatePick,
		ActionNameOverflow,
		ActionNameToggle,
		ActionNameInputSubmit,
		ActionNameFormSubmit,
	}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, len(want)=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got=%q want=%q", i, got[i], want[i])
		}
	}
}
