package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/jr/lazyhosts/internal/state"
)

// Renderer handles all view rendering.
type Renderer struct {
	width  int
	height int
}

// NewRenderer creates a renderer with terminal dimensions.
func NewRenderer(width, height int) *Renderer {
	return &Renderer{width: width, height: height}
}

// SetSize updates the terminal dimensions.
func (r *Renderer) SetSize(width, height int) {
	r.width = width
	r.height = height
}

// Render produces the full UI view from application state.
func (r *Renderer) Render(s *state.AppState) string {
	if r.width < 40 || r.height < 10 {
		return "Terminal too small. Please resize."
	}

	header := r.renderHeader()
	statusBar := r.renderStatusBar(s)

	// Available height for main panels (subtract header, status bar, borders)
	contentHeight := r.height - 5

	leftWidth := r.width*2/5 - 4
	rightWidth := r.width*3/5 - 4

	if leftWidth < 15 {
		leftWidth = 15
	}
	if rightWidth < 20 {
		rightWidth = 20
	}

	leftPanel := r.renderProfileList(s, leftWidth, contentHeight)
	rightPanel := r.renderDetailPanel(s, rightWidth, contentHeight)

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	view := lipgloss.JoinVertical(lipgloss.Left, header, mainView, statusBar)

	// Overlays (highest priority last)
	if s.ShowHelp {
		view = r.overlayHelp()
	}
	if s.ShowConfirm {
		view = r.overlayConfirm(s)
	}
	if s.IsInInputMode() {
		view = r.overlayInput(s)
	}

	return view
}

func (r *Renderer) renderHeader() string {
	title := HeaderStyle.Render("  lazyhosts")
	subtitle := HelpDescStyle.Render(" — hostctl profile manager")
	return lipgloss.JoinHorizontal(lipgloss.Center, title, subtitle)
}

func (r *Renderer) renderProfileList(s *state.AppState, width, height int) string {
	var title string
	if s.SearchMode {
		title = TitleStyle.Render(fmt.Sprintf(" Profiles [/%s]", s.SearchQuery))
	} else {
		title = TitleStyle.Render(" Profiles")
	}

	var items []string
	items = append(items, title)
	items = append(items, "")

	if s.Loading {
		items = append(items, LoadingStyle.Render("  Loading..."))
	} else if len(s.Profiles) == 0 {
		items = append(items, HelpDescStyle.Render("  No profiles found"))
	} else {
		indices := s.VisibleIndices()
		if len(indices) == 0 && s.SearchMode {
			items = append(items, HelpDescStyle.Render("  No matches"))
		}
		for i, idx := range indices {
			p := s.Profiles[idx]
			var statusIcon string
			var nameStyle lipgloss.Style
			if p.Enabled {
				statusIcon = EnabledStyle.Render("●")
				nameStyle = EnabledStyle
			} else {
				statusIcon = DisabledStyle.Render("○")
				nameStyle = DisabledStyle
			}

			entryInfo := fmt.Sprintf("(%d)", p.Entries)
			label := fmt.Sprintf(" %s %s %s", statusIcon, nameStyle.Render(p.Name), HelpDescStyle.Render(entryInfo))

			if i == s.Cursor {
				label = SelectedItemStyle.Width(width - 2).Render(
					fmt.Sprintf(" %s %s %s", statusIcon, p.Name, entryInfo),
				)
			}

			items = append(items, label)
		}
	}

	content := strings.Join(items, "\n")

	return ActivePanelStyle.
		Width(width).
		Height(height).
		Render(content)
}

func (r *Renderer) renderDetailPanel(s *state.AppState, width, height int) string {
	title := TitleStyle.Render(" Details")
	var sections []string
	sections = append(sections, title)
	sections = append(sections, "")

	// Active profiles
	active := s.ActiveProfiles()
	sections = append(sections, DetailLabelStyle.Render("  Active Profiles:"))
	if len(active) == 0 {
		sections = append(sections, HelpDescStyle.Render("    None"))
	} else {
		for _, name := range active {
			sections = append(sections, EnabledStyle.Render(fmt.Sprintf("    ● %s", name)))
		}
	}
	sections = append(sections, "")

	// Last switch
	if !s.LastSwitchTime.IsZero() {
		sections = append(sections, DetailLabelStyle.Render("  Last Switch:"))
		sections = append(sections, DetailValueStyle.Render(
			fmt.Sprintf("    %s", s.LastSwitchTime.Format("2006-01-02 15:04:05")),
		))
		sections = append(sections, "")
	}

	// Selected profile detail
	sel := s.SelectedProfile()
	if sel != nil && s.ProfileDetail != "" {
		sections = append(sections, DetailLabelStyle.Render(fmt.Sprintf("  Profile: %s", sel.Name)))
		sections = append(sections, "")
		for _, line := range strings.Split(s.ProfileDetail, "\n") {
			sections = append(sections, HelpDescStyle.Render("  "+line))
		}
		sections = append(sections, "")
	}

	// Hosts preview
	if s.HostsPreview != "" {
		sections = append(sections, DetailLabelStyle.Render("  /etc/hosts preview:"))
		sections = append(sections, "")
		for _, line := range strings.Split(s.HostsPreview, "\n") {
			sections = append(sections, HelpDescStyle.Render("  "+line))
		}
	}

	// Sudo status
	sections = append(sections, "")
	if s.HasSudo {
		sections = append(sections, EnabledStyle.Render("  ✓ sudo available"))
	} else {
		sections = append(sections, ErrorStyle.Render("  ✗ sudo not available (run: sudo -v)"))
	}

	content := strings.Join(sections, "\n")

	return PanelStyle.
		Width(width).
		Height(height).
		Render(content)
}

func (r *Renderer) renderStatusBar(s *state.AppState) string {
	var left string
	if s.StatusMessage != "" {
		if s.IsError {
			left = ErrorStyle.Render("  " + s.StatusMessage)
		} else {
			left = SuccessStyle.Render("  " + s.StatusMessage)
		}
	} else {
		left = HelpDescStyle.Render("  ? help  q quit  / search  a add  e edit  i import")
	}

	return StatusBarStyle.Width(r.width).Render(left)
}

func (r *Renderer) overlayHelp() string {
	entries := HelpEntries()
	var lines []string
	lines = append(lines, HeaderStyle.Render("Keyboard Shortcuts"))
	lines = append(lines, "")
	for _, e := range entries {
		if e[0] == "" && e[1] == "" {
			lines = append(lines, "")
			continue
		}
		if e[0] == "" {
			lines = append(lines, "  "+DetailLabelStyle.Render(e[1]))
			continue
		}
		line := fmt.Sprintf("  %s  %s",
			HelpKeyStyle.Width(10).Render(e[0]),
			HelpDescStyle.Render(e[1]),
		)
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = append(lines, HelpDescStyle.Render("  Press ? or Esc to close"))

	popup := HelpPopupStyle.Render(strings.Join(lines, "\n"))

	return lipgloss.Place(
		r.width, r.height,
		lipgloss.Center, lipgloss.Center,
		popup,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func (r *Renderer) overlayConfirm(s *state.AppState) string {
	msg := fmt.Sprintf("%s profile '%s'?", s.ConfirmAction, s.ConfirmTarget)
	lines := []string{
		HeaderStyle.Render("Confirm"),
		"",
		DetailValueStyle.Render("  " + msg),
		"",
		fmt.Sprintf("  %s / %s",
			HelpKeyStyle.Render("y (yes)"),
			HelpKeyStyle.Render("n (no)"),
		),
	}

	popup := DialogStyle.Render(strings.Join(lines, "\n"))

	return lipgloss.Place(
		r.width, r.height,
		lipgloss.Center, lipgloss.Center,
		popup,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func (r *Renderer) overlayInput(s *state.AppState) string {
	popupWidth := r.width * 3 / 4
	if popupWidth > 80 {
		popupWidth = 80
	}
	if popupWidth < 40 {
		popupWidth = 40
	}

	var lines []string

	// Title based on mode
	var title string
	switch s.InputMode {
	case state.InputAddName:
		title = "Add New Profile"
	case state.InputAddEntries:
		title = "Add Profile Entries"
	case state.InputImportName:
		title = "Import Profile"
	case state.InputImportPath:
		title = "Import From File"
	case state.InputEditEntries:
		title = "Edit Profile"
	}
	lines = append(lines, HeaderStyle.Render(title))
	lines = append(lines, "")
	lines = append(lines, DetailLabelStyle.Render(s.InputLabel))
	lines = append(lines, "")

	if s.IsTextMode() {
		// Multi-line text editor
		editorLines := strings.Split(s.TextBuffer, "\n")
		editorHeight := r.height / 2
		if editorHeight < 5 {
			editorHeight = 5
		}
		if editorHeight > 20 {
			editorHeight = 20
		}

		// Show lines with cursor
		for i, line := range editorLines {
			if i >= editorHeight {
				lines = append(lines, HelpDescStyle.Render(fmt.Sprintf("  ... (%d more lines)", len(editorLines)-i)))
				break
			}

			lineNum := HelpDescStyle.Render(fmt.Sprintf("%3d ", i+1))
			if i == s.TextCursorRow {
				col := s.TextCursorCol
				lineRuneLen := utf8.RuneCountInString(line)
				if col > lineRuneLen {
					col = lineRuneLen
				}
				runes := []rune(line)
				before := string(runes[:col])
				after := string(runes[col:])
				cursor := "█"
				displayLine := lineNum + EnabledStyle.Render(before+cursor+after)
				lines = append(lines, displayLine)
			} else {
				lines = append(lines, lineNum+DetailValueStyle.Render(line))
			}
		}

		// If buffer is empty, show placeholder with cursor
		if s.TextBuffer == "" || s.TextBuffer == "Loading..." {
			if s.TextBuffer == "Loading..." {
				lines = append(lines, LoadingStyle.Render("  Loading entries..."))
			} else {
				lines = append(lines, HelpDescStyle.Render("  1 ")+EnabledStyle.Render("█"))
				lines = append(lines, HelpDescStyle.Render("  (format: IP DOMAIN, one per line)"))
			}
		}

		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  %s save    %s cancel",
			HelpKeyStyle.Render("Ctrl+S"),
			HelpKeyStyle.Render("Esc"),
		))
	} else {
		// Single-line input
		inputDisplay := s.InputBuffer + "█"
		lines = append(lines, "  "+SearchStyle.Render("> ")+DetailValueStyle.Render(inputDisplay))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  %s confirm    %s cancel",
			HelpKeyStyle.Render("Enter"),
			HelpKeyStyle.Render("Esc"),
		))
	}

	popup := InputPopupStyle.Width(popupWidth).Render(strings.Join(lines, "\n"))

	return lipgloss.Place(
		r.width, r.height,
		lipgloss.Center, lipgloss.Center,
		popup,
		lipgloss.WithWhitespaceChars(" "),
	)
}
