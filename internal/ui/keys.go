package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the application.
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Disable key.Binding
	Reload  key.Binding
	Quit    key.Binding
	Help    key.Binding
	Search  key.Binding
	Escape  key.Binding
	Yes     key.Binding
	No      key.Binding
	Add     key.Binding
	Import  key.Binding
	Edit    key.Binding
	Delete  key.Binding
	Rename  key.Binding
}

// DefaultKeyMap returns the default set of key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "enable profile"),
		),
		Disable: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "disable profile"),
		),
		Reload: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "reload profiles"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search profiles"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Yes: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "confirm"),
		),
		No: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "cancel"),
		),
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add profile"),
		),
		Import: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "import profile from file"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit profile"),
		),
		Delete: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "delete profile"),
		),
		Rename: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "rename profile"),
		),
	}
}

// HelpEntries returns keybindings formatted for the help popup.
func HelpEntries() [][]string {
	return [][]string{
		{"↑ / k", "Move up"},
		{"↓ / j", "Move down"},
		{"Enter", "Enable selected profile"},
		{"d", "Disable selected profile"},
		{"a", "Add new profile"},
		{"e", "Edit selected profile"},
		{"R", "Rename selected profile"},
		{"x", "Delete selected profile"},
		{"i", "Import profile from file"},
		{"r", "Reload profile list"},
		{"/", "Search / filter profiles"},
		{"Esc", "Cancel / close dialog"},
		{"?", "Toggle this help"},
		{"q", "Quit"},
		{"", ""},
		{"", "In editor:"},
		{"Ctrl+S", "Save & submit"},
		{"Esc", "Cancel"},
	}
}
