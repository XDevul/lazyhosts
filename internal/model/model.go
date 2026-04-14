package model

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jr/lazyhosts/internal/hostctl"
	"github.com/jr/lazyhosts/internal/state"
	"github.com/jr/lazyhosts/internal/ui"
)

// Messages for async command results.
type profilesLoadedMsg struct{ result hostctl.Result }
type profileDetailMsg struct{ result hostctl.Result }
type hostsPreviewMsg struct {
	preview string
	err     error
}
type commandDoneMsg struct {
	result hostctl.Result
	action string
}
type sudoCheckMsg struct{ hasSudo bool }
type clearStatusMsg struct{}
type profileEntriesMsg struct {
	entries string
	err     error
}

// Model is the main Bubble Tea model.
type Model struct {
	state    *state.AppState
	keys     ui.KeyMap
	renderer *ui.Renderer
	ready    bool
}

// New creates a new Model.
func New() Model {
	return Model{
		state:    state.NewAppState(),
		keys:     ui.DefaultKeyMap(),
		renderer: ui.NewRenderer(80, 24),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadProfiles,
		m.loadHostsPreview,
		m.checkSudo,
	)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.renderer.SetSize(msg.Width, msg.Height)
		m.ready = true
		return m, nil

	case profilesLoadedMsg:
		m.state.Loading = false
		m.state.LastRefreshAt = msg.result.ExecutedAt
		if msg.result.Error != nil {
			m.state.SetStatus(msg.result.Error.Error(), true)
			return m, m.clearStatusAfter(5 * time.Second)
		}
		m.state.Profiles = msg.result.Profiles
		m.state.ClampCursor()
		m.state.ClearStatus()
		return m, m.loadSelectedDetail

	case profileDetailMsg:
		if msg.result.Error == nil {
			m.state.ProfileDetail = msg.result.Output
		}
		return m, nil

	case hostsPreviewMsg:
		if msg.err == nil {
			m.state.HostsPreview = msg.preview
		}
		return m, nil

	case commandDoneMsg:
		m.state.Loading = false
		if msg.result.Error != nil {
			m.state.SetStatus(msg.result.Error.Error(), true)
			return m, m.clearStatusAfter(5 * time.Second)
		}
		m.state.LastSwitchTime = msg.result.ExecutedAt
		m.state.SetStatus(msg.action+" successful", false)
		return m, tea.Batch(
			m.loadProfiles,
			m.loadHostsPreview,
			m.clearStatusAfter(3*time.Second),
		)

	case profileEntriesMsg:
		if msg.err != nil {
			m.state.SetStatus(msg.err.Error(), true)
			m.state.ResetInput()
			return m, m.clearStatusAfter(5 * time.Second)
		}
		m.state.TextBuffer = msg.entries
		m.state.TextCursorRow = 0
		m.state.TextCursorCol = 0
		return m, nil

	case sudoCheckMsg:
		m.state.HasSudo = msg.hasSudo
		return m, nil

	case clearStatusMsg:
		m.state.ClearStatus()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Input mode takes priority
	if m.state.IsInInputMode() {
		return m.handleInputKey(msg)
	}

	// Search mode
	if m.state.SearchMode {
		return m.handleSearchKey(msg)
	}

	// Confirm dialog
	if m.state.ShowConfirm {
		return m.handleConfirmKey(msg)
	}

	// Help popup
	if m.state.ShowHelp {
		if key.Matches(msg, m.keys.Help) || key.Matches(msg, m.keys.Escape) || key.Matches(msg, m.keys.Quit) {
			m.state.ShowHelp = false
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.state.ShowHelp = true

	case key.Matches(msg, m.keys.Up):
		if m.state.Cursor > 0 {
			m.state.Cursor--
			return m, m.loadSelectedDetail
		}

	case key.Matches(msg, m.keys.Down):
		visible := m.state.VisibleIndices()
		if m.state.Cursor < len(visible)-1 {
			m.state.Cursor++
			return m, m.loadSelectedDetail
		}

	case key.Matches(msg, m.keys.Enter):
		sel := m.state.SelectedProfile()
		if sel != nil {
			m.state.ShowConfirm = true
			m.state.ConfirmAction = "Enable"
			m.state.ConfirmTarget = sel.Name
		}

	case key.Matches(msg, m.keys.Disable):
		sel := m.state.SelectedProfile()
		if sel != nil && sel.Enabled {
			m.state.ShowConfirm = true
			m.state.ConfirmAction = "Disable"
			m.state.ConfirmTarget = sel.Name
		}

	case key.Matches(msg, m.keys.Add):
		m.state.InputMode = state.InputAddName
		m.state.InputLabel = "New profile name:"
		m.state.InputBuffer = ""

	case key.Matches(msg, m.keys.Import):
		m.state.InputMode = state.InputImportName
		m.state.InputLabel = "Profile name to import as:"
		m.state.InputBuffer = ""

	case key.Matches(msg, m.keys.Edit):
		sel := m.state.SelectedProfile()
		if sel != nil && sel.Name != "default" {
			m.state.InputMode = state.InputEditEntries
			m.state.InputLabel = "Editing profile: " + sel.Name
			m.state.EditTarget = sel.Name
			m.state.TextBuffer = "Loading..."
			return m, m.loadProfileEntries(sel.Name)
		} else if sel != nil && sel.Name == "default" {
			m.state.SetStatus("default profile is managed by /etc/hosts directly, not editable via hostctl", true)
			return m, m.clearStatusAfter(3 * time.Second)
		}

	case key.Matches(msg, m.keys.Delete):
		sel := m.state.SelectedProfile()
		if sel != nil && sel.Name != "default" {
			m.state.ShowConfirm = true
			m.state.ConfirmAction = "Delete"
			m.state.ConfirmTarget = sel.Name
		} else if sel != nil && sel.Name == "default" {
			m.state.SetStatus("default profile cannot be deleted", true)
			return m, m.clearStatusAfter(3 * time.Second)
		}

	case key.Matches(msg, m.keys.Rename):
		sel := m.state.SelectedProfile()
		if sel != nil && sel.Name != "default" {
			m.state.InputMode = state.InputRenameName
			m.state.InputLabel = "Rename '" + sel.Name + "' to:"
			m.state.EditTarget = sel.Name
			m.state.InputBuffer = ""
		} else if sel != nil && sel.Name == "default" {
			m.state.SetStatus("default profile cannot be renamed", true)
			return m, m.clearStatusAfter(3 * time.Second)
		}

	case key.Matches(msg, m.keys.Reload):
		m.state.Loading = true
		m.state.SetStatus("Reloading...", false)
		return m, tea.Batch(m.loadProfiles, m.loadHostsPreview, m.checkSudo)

	case key.Matches(msg, m.keys.Search):
		m.state.SearchMode = true
		m.state.SearchQuery = ""
	}

	return m, nil
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Escape always cancels
	if key.Matches(msg, m.keys.Escape) {
		m.state.ResetInput()
		return m, nil
	}

	// Multi-line text editor modes
	if m.state.IsTextMode() {
		return m.handleTextEditorKey(msg)
	}

	// Single-line input modes
	switch msg.Type {
	case tea.KeyEnter:
		return m.submitSingleLineInput()

	case tea.KeyBackspace:
		if len(m.state.InputBuffer) > 0 {
			m.state.InputBuffer = m.state.InputBuffer[:len(m.state.InputBuffer)-1]
		}

	case tea.KeySpace:
		m.state.InputBuffer += " "

	case tea.KeyRunes:
		m.state.InputBuffer += string(msg.Runes)
	}

	return m, nil
}

func (m Model) submitSingleLineInput() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.state.InputBuffer)
	if value == "" {
		m.state.SetStatus("Input cannot be empty", true)
		return m, m.clearStatusAfter(3 * time.Second)
	}

	switch m.state.InputMode {
	case state.InputAddName:
		// Check if profile already exists
		for _, p := range m.state.Profiles {
			if strings.EqualFold(p.Name, value) {
				m.state.SetStatus("Profile '"+value+"' already exists", true)
				return m, m.clearStatusAfter(3 * time.Second)
			}
		}
		m.state.EditTarget = value
		m.state.InputMode = state.InputAddEntries
		m.state.InputLabel = "Add entries for '" + value + "'  (IP DOMAIN per line, Ctrl+S to save):"
		m.state.InputBuffer = ""
		m.state.TextBuffer = ""

	case state.InputImportName:
		// Check if profile already exists
		for _, p := range m.state.Profiles {
			if strings.EqualFold(p.Name, value) {
				m.state.SetStatus("Profile '"+value+"' already exists", true)
				return m, m.clearStatusAfter(3 * time.Second)
			}
		}
		m.state.EditTarget = value
		m.state.InputMode = state.InputImportPath
		m.state.InputLabel = "File path to import for '" + value + "':"
		m.state.InputBuffer = ""

	case state.InputImportPath:
		name := m.state.EditTarget
		filePath := value
		m.state.ResetInput()
		m.state.Loading = true
		m.state.SetStatus("Importing profile...", false)
		return m, m.importProfile(name, filePath)

	case state.InputRenameName:
		// Check if new name already exists
		for _, p := range m.state.Profiles {
			if strings.EqualFold(p.Name, value) {
				m.state.SetStatus("Profile '"+value+"' already exists", true)
				return m, m.clearStatusAfter(3 * time.Second)
			}
		}
		oldName := m.state.EditTarget
		newName := value
		m.state.ResetInput()
		m.state.Loading = true
		m.state.SetStatus("Renaming '"+oldName+"' to '"+newName+"'...", false)
		return m, m.renameProfile(oldName, newName)
	}

	return m, nil
}

// runeLen returns the number of runes (characters) in a string.
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// runeSlice returns a substring by rune index [start:end].
func runeSlice(s string, start, end int) string {
	runes := []rune(s)
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}

func (m Model) handleTextEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlS {
		return m.submitTextEditor()
	}

	lines := strings.Split(m.state.TextBuffer, "\n")
	row := m.state.TextCursorRow
	col := m.state.TextCursorCol

	// Clamp row/col to valid range
	if row >= len(lines) {
		row = len(lines) - 1
	}
	if row < 0 {
		row = 0
	}
	if len(lines) > 0 && col > runeLen(lines[row]) {
		col = runeLen(lines[row])
	}

	switch msg.Type {
	case tea.KeyEnter:
		m.editorInsertNewline(lines, row, col)
	case tea.KeyBackspace:
		m.editorDeleteBefore(lines, row, col)
	case tea.KeyUp:
		m.editorMoveUp(lines)
	case tea.KeyDown:
		m.editorMoveDown(lines)
	case tea.KeyLeft:
		m.editorMoveLeft()
	case tea.KeyRight:
		m.editorMoveRight(lines, row)
	case tea.KeySpace:
		m.editorInsertRunes(lines, row, col, []rune{' '})
	case tea.KeyRunes:
		m.editorInsertRunes(lines, row, col, msg.Runes)
	}

	return m, nil
}

func (m *Model) editorInsertNewline(lines []string, row, col int) {
	if len(lines) == 0 {
		m.state.TextBuffer = "\n"
		m.state.TextCursorRow = 1
		m.state.TextCursorCol = 0
		return
	}
	before := runeSlice(lines[row], 0, col)
	after := runeSlice(lines[row], col, runeLen(lines[row]))
	lines[row] = before
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:row+1]...)
	newLines = append(newLines, after)
	if row+1 < len(lines) {
		newLines = append(newLines, lines[row+1:]...)
	}
	m.state.TextBuffer = strings.Join(newLines, "\n")
	m.state.TextCursorRow = row + 1
	m.state.TextCursorCol = 0
}

func (m *Model) editorDeleteBefore(lines []string, row, col int) {
	if len(lines) == 0 {
		return
	}
	if col > 0 {
		line := lines[row]
		lines[row] = runeSlice(line, 0, col-1) + runeSlice(line, col, runeLen(line))
		m.state.TextBuffer = strings.Join(lines, "\n")
		m.state.TextCursorCol = col - 1
	} else if row > 0 {
		prevLen := runeLen(lines[row-1])
		lines[row-1] = lines[row-1] + lines[row]
		newLines := make([]string, 0, len(lines)-1)
		newLines = append(newLines, lines[:row]...)
		newLines = append(newLines, lines[row+1:]...)
		m.state.TextBuffer = strings.Join(newLines, "\n")
		m.state.TextCursorRow = row - 1
		m.state.TextCursorCol = prevLen
	}
}

func (m *Model) editorMoveUp(lines []string) {
	if m.state.TextCursorRow > 0 {
		m.state.TextCursorRow--
		if m.state.TextCursorCol > runeLen(lines[m.state.TextCursorRow]) {
			m.state.TextCursorCol = runeLen(lines[m.state.TextCursorRow])
		}
	}
}

func (m *Model) editorMoveDown(lines []string) {
	if m.state.TextCursorRow < len(lines)-1 {
		m.state.TextCursorRow++
		if m.state.TextCursorCol > runeLen(lines[m.state.TextCursorRow]) {
			m.state.TextCursorCol = runeLen(lines[m.state.TextCursorRow])
		}
	}
}

func (m *Model) editorMoveLeft() {
	if m.state.TextCursorCol > 0 {
		m.state.TextCursorCol--
	}
}

func (m *Model) editorMoveRight(lines []string, row int) {
	if len(lines) > 0 && m.state.TextCursorCol < runeLen(lines[row]) {
		m.state.TextCursorCol++
	}
}

func (m *Model) editorInsertRunes(lines []string, row, col int, runes []rune) {
	insertion := string(runes)
	if len(lines) == 0 {
		m.state.TextBuffer = insertion
		m.state.TextCursorCol = runeLen(insertion)
		return
	}
	line := lines[row]
	lines[row] = runeSlice(line, 0, col) + insertion + runeSlice(line, col, runeLen(line))
	m.state.TextBuffer = strings.Join(lines, "\n")
	m.state.TextCursorCol = col + runeLen(insertion)
}

func (m Model) submitTextEditor() (tea.Model, tea.Cmd) {
	content := strings.TrimSpace(m.state.TextBuffer)
	if content == "" || content == "Loading..." {
		m.state.SetStatus("No entries to save", true)
		return m, m.clearStatusAfter(3 * time.Second)
	}

	name := m.state.EditTarget
	mode := m.state.InputMode
	m.state.ResetInput()
	m.state.Loading = true

	switch mode {
	case state.InputAddEntries:
		m.state.SetStatus("Adding profile '"+name+"'...", false)
		return m, m.addProfile(name, content)
	case state.InputEditEntries:
		m.state.SetStatus("Updating profile '"+name+"'...", false)
		return m, m.updateProfile(name, content)
	}

	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.state.SearchMode = false
		m.state.SearchQuery = ""
		m.state.FilteredIdx = nil
		m.state.ClampCursor()
		return m, nil

	case msg.Type == tea.KeyEnter:
		m.state.SearchMode = false
		return m, nil

	case msg.Type == tea.KeyBackspace:
		if len(m.state.SearchQuery) > 0 {
			m.state.SearchQuery = m.state.SearchQuery[:len(m.state.SearchQuery)-1]
			m.applyFilter()
		}
		return m, nil

	default:
		if msg.Type == tea.KeyRunes {
			m.state.SearchQuery += string(msg.Runes)
			m.applyFilter()
		}
		return m, nil
	}
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Yes):
		action := m.state.ConfirmAction
		target := m.state.ConfirmTarget
		m.state.ShowConfirm = false
		m.state.Loading = true
		m.state.SetStatus("Executing...", false)

		switch action {
		case "Enable":
			return m, m.enableProfile(target)
		case "Delete":
			return m, m.deleteProfile(target)
		default:
			return m, m.disableProfile(target)
		}

	case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Escape):
		m.state.ShowConfirm = false
		return m, nil
	}
	return m, nil
}

func (m *Model) applyFilter() {
	query := strings.ToLower(m.state.SearchQuery)
	m.state.FilteredIdx = nil
	for i, p := range m.state.Profiles {
		if strings.Contains(strings.ToLower(p.Name), query) {
			m.state.FilteredIdx = append(m.state.FilteredIdx, i)
		}
	}
	m.state.Cursor = 0
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}
	return m.renderer.Render(m.state)
}

// Async commands

func (m Model) loadProfiles() tea.Msg {
	result := hostctl.ListProfiles()
	return profilesLoadedMsg{result: result}
}

func (m Model) loadSelectedDetail() tea.Msg {
	sel := m.state.SelectedProfile()
	if sel == nil {
		return profileDetailMsg{}
	}
	result := hostctl.ShowProfile(sel.Name)
	return profileDetailMsg{result: result}
}

func (m Model) loadHostsPreview() tea.Msg {
	preview, err := hostctl.HostsPreview(15)
	return hostsPreviewMsg{preview: preview, err: err}
}

func (m Model) loadProfileEntries(name string) tea.Cmd {
	return func() tea.Msg {
		entries, err := hostctl.GetProfileEntries(name)
		return profileEntriesMsg{entries: entries, err: err}
	}
}

func (m Model) enableProfile(name string) tea.Cmd {
	return func() tea.Msg {
		result := hostctl.EnableProfile(name)
		return commandDoneMsg{result: result, action: "Enable " + name}
	}
}

func (m Model) disableProfile(name string) tea.Cmd {
	return func() tea.Msg {
		result := hostctl.DisableProfile(name)
		return commandDoneMsg{result: result, action: "Disable " + name}
	}
}

func (m Model) addProfile(name string, entries string) tea.Cmd {
	return func() tea.Msg {
		result := hostctl.AddProfile(name, entries)
		return commandDoneMsg{result: result, action: "Add " + name}
	}
}

func (m Model) importProfile(name string, filePath string) tea.Cmd {
	return func() tea.Msg {
		result := hostctl.ImportProfile(name, filePath)
		return commandDoneMsg{result: result, action: "Import " + name}
	}
}

func (m Model) updateProfile(name string, entries string) tea.Cmd {
	return func() tea.Msg {
		result := hostctl.UpdateProfile(name, entries)
		return commandDoneMsg{result: result, action: "Update " + name}
	}
}

func (m Model) deleteProfile(name string) tea.Cmd {
	return func() tea.Msg {
		result := hostctl.RemoveProfile(name)
		return commandDoneMsg{result: result, action: "Delete " + name}
	}
}

func (m Model) renameProfile(oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		result := hostctl.RenameProfile(oldName, newName)
		return commandDoneMsg{result: result, action: "Rename " + oldName + " → " + newName}
	}
}

func (m Model) checkSudo() tea.Msg {
	return sudoCheckMsg{hasSudo: hostctl.HasElevatedPrivilege()}
}

func (m Model) clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}
