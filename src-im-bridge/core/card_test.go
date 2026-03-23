package core

import "testing"

func TestCardBuilder_AppendsFieldsAndButtons(t *testing.T) {
	card := NewCard().
		SetTitle("Bridge rollout").
		AddField("状态", "triaged").
		AddField("负责人", "Alice").
		AddButton("查看详情", "link:/tasks/1").
		AddPrimaryButton("分配", "act:assign:1").
		AddDangerButton("关闭", "act:close:1")

	if card.Title != "Bridge rollout" {
		t.Fatalf("title = %q", card.Title)
	}
	if len(card.Fields) != 2 {
		t.Fatalf("fields = %+v", card.Fields)
	}
	if len(card.Buttons) != 3 {
		t.Fatalf("buttons = %+v", card.Buttons)
	}
	if card.Buttons[0].Style != "default" || card.Buttons[1].Style != "primary" || card.Buttons[2].Style != "danger" {
		t.Fatalf("button styles = %+v", card.Buttons)
	}
}
