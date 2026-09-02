package ui

import "testing"

func TestFitEditorLineClipsLongLine(t *testing.T) {
	line := "192.168.4.172 eip.ai-stack.local chat.ai-stack.local hireagent.ai-stack.local"

	visible, _ := fitEditorLine(line, -1, 20)
	if visible != "192.168.4.172 eip.a" {
		t.Fatalf("visible = %q", visible)
	}
}

func TestFitEditorLineCountsWideRunes(t *testing.T) {
	// Each CJK rune takes two cells, so 20 cells fit at most 9 of them plus one.
	visible, _ := fitEditorLine("平台與管理工具平台與管理工具", -1, 20)
	if len([]rune(visible)) != 9 {
		t.Fatalf("visible = %q (%d runes), want 9 runes", visible, len([]rune(visible)))
	}
}

func TestFitEditorLineKeepsCursorVisible(t *testing.T) {
	line := "192.168.4.172 dataverse.ai-stack.local elara.ai-stack.local"
	cursor := len([]rune(line)) // cursor sits past the right edge

	visible, col := fitEditorLine(line, cursor, 20)
	if col < 0 || col > len([]rune(visible)) {
		t.Fatalf("cursor col %d out of range for %q", col, visible)
	}
	if col != len([]rune(visible)) {
		t.Errorf("cursor col = %d, want it at the end (%d)", col, len([]rune(visible)))
	}
}

func TestFitEditorLineShortLineUnchanged(t *testing.T) {
	visible, col := fitEditorLine("10.0.0.1 a.dev", 5, 40)
	if visible != "10.0.0.1 a.dev" {
		t.Fatalf("visible = %q", visible)
	}
	if col != 5 {
		t.Errorf("cursor col = %d, want 5", col)
	}
}
