package model

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jr/lazyhosts/internal/state"
)

// paste feeds a bracketed-paste key message into the multi-line editor.
func paste(m Model, text string) Model {
	updated, _ := m.handleTextEditorKey(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(text),
		Paste: true,
	})
	return updated.(Model)
}

func TestEditorPasteMultiline(t *testing.T) {
	m := New()
	m = paste(m, "192.168.4.172 eip.local\r\n192.168.4.172 chat.local\r\n")

	want := "192.168.4.172 eip.local\n192.168.4.172 chat.local\n"
	if m.state.TextBuffer != want {
		t.Fatalf("TextBuffer = %q, want %q", m.state.TextBuffer, want)
	}
	if m.state.TextCursorRow != 2 || m.state.TextCursorCol != 0 {
		t.Errorf("cursor = (%d,%d), want (2,0)", m.state.TextCursorRow, m.state.TextCursorCol)
	}
}

func TestEditorPasteIntoExistingLine(t *testing.T) {
	m := New()
	m.state.TextBuffer = "10.0.0.1 a.dev\n10.0.0.9 z.dev"
	m.state.TextCursorRow = 0
	m.state.TextCursorCol = 14 // end of first line

	m = paste(m, "\n10.0.0.2 b.dev")

	want := "10.0.0.1 a.dev\n10.0.0.2 b.dev\n10.0.0.9 z.dev"
	if m.state.TextBuffer != want {
		t.Fatalf("TextBuffer = %q, want %q", m.state.TextBuffer, want)
	}
	if m.state.TextCursorRow != 1 || m.state.TextCursorCol != 14 {
		t.Errorf("cursor = (%d,%d), want (1,14)", m.state.TextCursorRow, m.state.TextCursorCol)
	}
}

func TestEditorPasteStripsControlChars(t *testing.T) {
	m := New()
	m = paste(m, "10.0.0.1\x00 a.dev\x07")

	if m.state.TextBuffer != "10.0.0.1 a.dev" {
		t.Fatalf("TextBuffer = %q, want %q", m.state.TextBuffer, "10.0.0.1 a.dev")
	}
}

func TestEditorTypingSingleRune(t *testing.T) {
	m := New()
	m.state.TextBuffer = "10.0.0.1 a.de"
	m.state.TextCursorRow = 0
	m.state.TextCursorCol = 13

	updated, _ := m.handleTextEditorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	m = updated.(Model)

	if m.state.TextBuffer != "10.0.0.1 a.dev" {
		t.Fatalf("TextBuffer = %q", m.state.TextBuffer)
	}
	if m.state.TextCursorCol != 14 {
		t.Errorf("cursor col = %d, want 14", m.state.TextCursorCol)
	}
}

// pressKey feeds a key into whatever dialog is currently open.
func pressKey(m Model, msg tea.KeyMsg) Model {
	updated, _ := m.handleKey(msg)
	return updated.(Model)
}

func TestAddFlowPasteThenName(t *testing.T) {
	m := New()

	// 'a' must open the editor directly, not the name field.
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if m.state.InputMode != state.InputAddEntries {
		t.Fatalf("after 'a' InputMode = %v, want InputAddEntries", m.state.InputMode)
	}

	m = paste(m, "# header\n192.168.4.172 eip.local chat.local\n\n192.168.4.172 vllm.local # note\n")
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.state.InputMode != state.InputAddName {
		t.Fatalf("after Ctrl+S InputMode = %v, want InputAddName", m.state.InputMode)
	}
	want := "192.168.4.172 eip.local\n192.168.4.172 chat.local\n192.168.4.172 vllm.local"
	if m.state.PendingEntries != want {
		t.Fatalf("PendingEntries = %q, want %q", m.state.PendingEntries, want)
	}
	if !strings.Contains(m.state.InputLabel, "3 entries") {
		t.Errorf("InputLabel = %q, want it to mention 3 entries", m.state.InputLabel)
	}
}

func TestNameFieldKeepsFirstLineOfPaste(t *testing.T) {
	m := New()
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = paste(m, "10.0.0.1 a.dev\n")
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlS})

	// A stray multi-line paste into the name field must not swallow the block.
	m = pressKey(m, tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("dgxspark\n192.168.4.172 eip.local\n192.168.4.172 chat.local"),
		Paste: true,
	})
	if m.state.InputBuffer != "dgxspark" {
		t.Fatalf("InputBuffer = %q, want %q", m.state.InputBuffer, "dgxspark")
	}
}

func TestAddFlowRejectsEntrylessBuffer(t *testing.T) {
	m := New()
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = paste(m, "# only comments\n# nothing usable\n")
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.state.InputMode != state.InputAddEntries {
		t.Errorf("InputMode = %v, want to stay in the editor", m.state.InputMode)
	}
	if !m.state.IsError {
		t.Error("expected an error status")
	}
}

func TestEditFlowStillSavesDirectly(t *testing.T) {
	m := New()
	m.state.InputMode = state.InputEditEntries
	m.state.EditTarget = "dev"
	m.state.TextBuffer = "10.0.0.1 a.dev"

	updated, cmd := m.handleTextEditorKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("expected an update command")
	}
	if m.state.InputMode != state.InputNone {
		t.Errorf("InputMode = %v, want InputNone after saving an edit", m.state.InputMode)
	}
}

func TestAddFlowFoldsUppercaseName(t *testing.T) {
	m := New()
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = paste(m, "192.168.4.172 eip.ai-stack.dgxspark\n")
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlS})

	m = pressKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("EAS")})
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	if !strings.Contains(m.state.StatusMessage, "'eas'") {
		t.Fatalf("StatusMessage = %q, want it to name the lowercased profile", m.state.StatusMessage)
	}
	if !strings.Contains(m.state.StatusMessage, "lowercased from 'EAS'") {
		t.Errorf("StatusMessage = %q, want it to explain the fold", m.state.StatusMessage)
	}
	if m.state.IsError {
		t.Error("folding a name must not be reported as an error")
	}
}

func TestAddFlowKeepsDialogOnBadName(t *testing.T) {
	m := New()
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = paste(m, "192.168.4.172 eip.ai-stack.dgxspark\n")
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlS})

	// A space is not a legal profile name: the entries must survive the retry.
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("my")})
	m = pressKey(m, tea.KeyMsg{Type: tea.KeySpace})
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("stack")})
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	if !m.state.IsError {
		t.Fatal("expected an error status for a name with a space")
	}
	if m.state.InputMode != state.InputAddName {
		t.Errorf("InputMode = %v, want to stay on the name prompt", m.state.InputMode)
	}
	if m.state.PendingEntries != "192.168.4.172 eip.ai-stack.dgxspark" {
		t.Errorf("PendingEntries = %q, entries were lost", m.state.PendingEntries)
	}
}
