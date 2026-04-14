package state

import (
	"time"

	"github.com/jr/lazyhosts/internal/hostctl"
)

// InputMode represents the current input dialog state.
type InputMode int

const (
	InputNone         InputMode = iota
	InputAddName                // entering profile name for add
	InputAddEntries             // entering host entries for add
	InputImportName             // entering profile name for import
	InputImportPath             // entering file path for import
	InputEditEntries            // editing entries for existing profile
	InputRenameName             // entering new name for rename
)

// AppState holds all application state, separated from UI concerns.
type AppState struct {
	Profiles       []hostctl.Profile
	Cursor         int
	HostsPreview   string
	ProfileDetail  string
	LastSwitchTime time.Time
	LastRefreshAt  time.Time
	StatusMessage  string
	IsError        bool
	Loading        bool
	ShowHelp       bool
	ShowConfirm    bool
	ConfirmAction  string
	ConfirmTarget  string
	SearchMode     bool
	SearchQuery    string
	FilteredIdx    []int
	HasSudo        bool

	// Input dialog state
	InputMode     InputMode
	InputLabel    string // label shown above the input field
	InputBuffer   string // single-line text buffer
	TextBuffer    string // multi-line text buffer
	TextCursorRow int    // cursor row in multi-line editor
	TextCursorCol int    // cursor col in multi-line editor
	EditTarget    string // profile name being edited
}

// NewAppState creates a fresh application state.
func NewAppState() *AppState {
	return &AppState{
		Cursor: 0,
	}
}

// SelectedProfile returns the currently highlighted profile, or nil.
func (s *AppState) SelectedProfile() *hostctl.Profile {
	indices := s.VisibleIndices()
	if len(indices) == 0 {
		return nil
	}
	if s.Cursor < 0 || s.Cursor >= len(indices) {
		return nil
	}
	return &s.Profiles[indices[s.Cursor]]
}

// VisibleIndices returns the indices of profiles matching the current filter.
func (s *AppState) VisibleIndices() []int {
	if s.SearchMode && len(s.FilteredIdx) > 0 {
		return s.FilteredIdx
	}
	if s.SearchMode && s.SearchQuery != "" {
		return s.FilteredIdx
	}
	idx := make([]int, len(s.Profiles))
	for i := range s.Profiles {
		idx[i] = i
	}
	return idx
}

// ActiveProfiles returns the names of all enabled profiles.
func (s *AppState) ActiveProfiles() []string {
	var active []string
	for _, p := range s.Profiles {
		if p.Enabled {
			active = append(active, p.Name)
		}
	}
	return active
}

// ClampCursor ensures cursor is within valid range.
func (s *AppState) ClampCursor() {
	visible := s.VisibleIndices()
	if len(visible) == 0 {
		s.Cursor = 0
		return
	}
	if s.Cursor < 0 {
		s.Cursor = 0
	}
	if s.Cursor >= len(visible) {
		s.Cursor = len(visible) - 1
	}
}

// SetStatus sets a status message with error flag.
func (s *AppState) SetStatus(msg string, isError bool) {
	s.StatusMessage = msg
	s.IsError = isError
}

// ClearStatus clears the status message.
func (s *AppState) ClearStatus() {
	s.StatusMessage = ""
	s.IsError = false
}

// ResetInput clears all input dialog state.
func (s *AppState) ResetInput() {
	s.InputMode = InputNone
	s.InputLabel = ""
	s.InputBuffer = ""
	s.TextBuffer = ""
	s.TextCursorRow = 0
	s.TextCursorCol = 0
	s.EditTarget = ""
}

// IsInInputMode returns true if any input dialog is active.
func (s *AppState) IsInInputMode() bool {
	return s.InputMode != InputNone
}

// IsTextMode returns true if we're in a multi-line text editing mode.
func (s *AppState) IsTextMode() bool {
	return s.InputMode == InputAddEntries || s.InputMode == InputEditEntries
}
