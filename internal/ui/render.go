package ui

import (
	"fmt"
	"runtime"
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
	// Budget total lines to prevent overflow. height = usable lines inside panel border.
	budget := height - 2 // leave margin
	if budget < 10 {
		budget = 10
	}

	title := TitleStyle.Render(" Details")
	var sections []string
	sections = append(sections, title)
	sections = append(sections, "")
	used := 2

	// Active profiles
	active := s.ActiveProfiles()
	sections = append(sections, DetailLabelStyle.Render("  Active Profiles:"))
	used++
	if len(active) == 0 {
		sections = append(sections, HelpDescStyle.Render("    None"))
		used++
	} else {
		for _, name := range active {
			if used >= budget-6 {
				break
			}
			sections = append(sections, EnabledStyle.Render(fmt.Sprintf("    ● %s", name)))
			used++
		}
	}
	sections = append(sections, "")
	used++

	// Last switch
	if !s.LastSwitchTime.IsZero() && used < budget-6 {
		sections = append(sections, DetailLabelStyle.Render("  Last Switch:"))
		sections = append(sections, DetailValueStyle.Render(
			fmt.Sprintf("    %s", s.LastSwitchTime.Format("2006-01-02 15:04:05")),
		))
		sections = append(sections, "")
		used += 3
	}

	// Selected profile detail — use remaining budget, split between detail and preview
	sel := s.SelectedProfile()
	remaining := budget - used - 4 // reserve 4 lines for sudo + spacing
	if remaining < 4 {
		remaining = 4
	}

	if sel != nil && s.ProfileDetail != "" {
		detailBudget := remaining * 2 / 3
		if detailBudget < 3 {
			detailBudget = 3
		}

		sections = append(sections, DetailLabelStyle.Render(fmt.Sprintf("  Profile: %s (%d entries)", sel.Name, sel.Entries)))
		sections = append(sections, "")
		used += 2

		detailLines := strings.Split(s.ProfileDetail, "\n")
		for i, line := range detailLines {
			if i >= detailBudget {
				sections = append(sections, HelpDescStyle.Render(fmt.Sprintf("  ... and %d more", len(detailLines)-i)))
				used++
				break
			}
			sections = append(sections, HelpDescStyle.Render("  "+line))
			used++
		}
		sections = append(sections, "")
		used++

		remaining = budget - used - 3
	}

	// Hosts preview — use whatever remains
	if s.HostsPreview != "" && remaining > 2 {
		hostsLabel := "/etc/hosts preview:"
		if runtime.GOOS == "windows" {
			hostsLabel = "hosts file preview:"
		}
		sections = append(sections, DetailLabelStyle.Render("  "+hostsLabel))
		sections = append(sections, "")
		used += 2

		previewBudget := remaining - 2
		if previewBudget < 2 {
			previewBudget = 2
		}
		previewLines := strings.Split(s.HostsPreview, "\n")
		for i, line := range previewLines {
			if i >= previewBudget {
				sections = append(sections, HelpDescStyle.Render(fmt.Sprintf("  ... (%d more lines)", len(previewLines)-i)))
				used++
				break
			}
			sections = append(sections, HelpDescStyle.Render("  "+line))
			used++
		}
	}

	// Privilege status
	sections = append(sections, "")
	if s.HasSudo {
		sections = append(sections, EnabledStyle.Render("  ✓ elevated privileges available"))
	} else {
		if runtime.GOOS == "windows" {
			sections = append(sections, ErrorStyle.Render("  ✗ not administrator (run terminal as admin)"))
		} else {
			sections = append(sections, ErrorStyle.Render("  ✗ sudo not available (run: sudo -v)"))
		}
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
		left = HelpDescStyle.Render("  ? help  q quit  / search  a add  e edit  d disable  R rename  c copy  I batch-ip  x delete  i import")
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
	case state.InputRenameName:
		title = "Rename Profile"
	case state.InputCopyName:
		title = "Copy Profile"
	case state.InputBatchIP:
		title = "Batch Change IP"
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
